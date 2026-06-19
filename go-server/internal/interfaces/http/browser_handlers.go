package httpiface

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/domain/browser"
	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/interfaces/dto"
)

// firstNonEmpty returns the first non-empty string from the arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// browserHandlers wires the browser-executor HTTP endpoints: browser connection
// management plus the browser-data primitives (extract / screenshot /
// run_page_script / fetch_in_page) that forward commands to the extension.
type browserHandlers struct {
	deps Dependencies
}

// RegisterBrowserHandlers registers the browser endpoints on the protected group.
func RegisterBrowserHandlers(api *echo.Group, deps Dependencies) {
	h := &browserHandlers{deps: deps}

	api.GET("/browsers", h.list)
	api.POST("/browsers/register", h.register)
	api.POST("/browsers/kick", h.kick)
	api.POST("/browsers/ban", h.ban)
	api.POST("/browsers/unban", h.unban)

	api.POST("/extract", h.extract)
	api.POST("/screenshot", h.screenshot)
	api.POST("/run_page_script", h.runPageScript)
	api.POST("/fetch_in_page", h.fetchInPage)

	// Capture records (read-only history of stored extractions).
	api.GET("/captures/extract", h.listExtractCaptures)
}

// listExtractCaptures returns the stored extraction history (metadata only).
func (h *browserHandlers) listExtractCaptures(c echo.Context) error {
	limit, offset := pageParams(c, 50)
	rows, total, err := h.deps.DB.ListExtractCache(limit, offset)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	items := make([]dto.ExtractRecord, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.ExtractRecord{
			URL:       r.URL,
			Title:     r.Title,
			Chars:     len([]rune(r.Markdown)),
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": total})
}

// send resolves the target browser, forwards one command, and maps transport /
// extension failures to the standard error responses. On failure it has already
// written the response and returns ok=false, so callers just `return nil`.
func (h *browserHandlers) send(c echo.Context, browserName string, req *capture.BrowserRequest) (*capture.BrowserResponse, bool) {
	logger := h.deps.Logger

	connID, err := h.deps.WSManager.ResolveBrowserConnID(browserName)
	if err != nil {
		logger.Warn("Browser not found", zap.String("browser", browserName), zap.Error(err))
		_ = detailErr(c, http.StatusServiceUnavailable, err.Error())
		return nil, false
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), time.Duration(h.deps.Settings.APIExtractTimeout*float64(time.Second)))
	defer cancel()

	resp, err := h.deps.WSManager.SendMessage(ctx, req, connID)
	if err != nil {
		logger.Error("browser command failed", zap.String("command", req.Command), zap.Error(err))
		_ = detailErr(c, http.StatusGatewayTimeout, err.Error())
		return nil, false
	}
	if !resp.Success {
		logger.Error("Browser extension returned error", zap.String("error", resp.Error))
		_ = detailErr(c, http.StatusBadGateway, "Browser extension error: "+resp.Error)
		return nil, false
	}
	return resp, true
}

// --- browser connection management ---

func (h *browserHandlers) list(c echo.Context) error {
	regs := h.deps.BrowserRegistry.List()
	list := make([]browser.BrowserInfo, 0, len(regs))
	for _, reg := range regs {
		list = append(list, browser.BrowserInfo{
			ConnID: reg.ConnectID,
			Name:   reg.Name,
			Online: h.deps.WSManager.HasConnection(reg.ConnectID),
			Banned: reg.Banned,
		})
	}
	return c.JSON(http.StatusOK, dto.BrowserListResponse{Browsers: list, Count: len(list)})
}

func (h *browserHandlers) register(c echo.Context) error {
	var req dto.BrowserRegisterRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}

	reg, err := h.deps.BrowserRegistry.Register(req.ConnectID, req.Name)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, browserregistry.ErrBrowserRegistryConflict) {
			status = http.StatusConflict
		}
		h.deps.Logger.Warn("Browser registration failed", zap.Error(err))
		return detailErr(c, status, err.Error())
	}
	return c.JSON(http.StatusOK, dto.BrowserRegisterResponse{Success: true, Browser: reg})
}

func (h *browserHandlers) kick(c echo.Context) error {
	connID, err := bindConnID(c)
	if err != nil {
		return err
	}
	if err := h.deps.WSManager.Kick(connID); err != nil {
		return detailErr(c, http.StatusNotFound, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *browserHandlers) ban(c echo.Context) error {
	connID, err := bindConnID(c)
	if err != nil {
		return err
	}
	// First, kick/disconnect the browser if it's currently connected.
	_ = h.deps.WSManager.Kick(connID)
	if err := h.deps.BrowserRegistry.Ban(connID); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *browserHandlers) unban(c echo.Context) error {
	connID, err := bindConnID(c)
	if err != nil {
		return err
	}
	if err := h.deps.BrowserRegistry.Unban(connID); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

// bindConnID binds and validates the `conn_id` field shared by kick/ban/unban.
// On a bad body it writes the error response and returns a non-nil error.
func bindConnID(c echo.Context) (string, error) {
	var req struct {
		ConnID string `json:"conn_id"`
	}
	if err := c.Bind(&req); err != nil {
		return "", detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.ConnID == "" {
		return "", detailErr(c, http.StatusBadRequest, "conn_id is required")
	}
	return req.ConnID, nil
}

// --- browser-data primitives ---

func (h *browserHandlers) extract(c echo.Context) error {
	var req dto.ExtractAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}

	// Read-through cache: return the stored extraction unless ?refresh=true.
	if !isTrue(c.QueryParam("refresh")) {
		if cached, err := h.deps.DB.GetExtractCache(req.URL); err == nil && cached != nil {
			return c.JSON(http.StatusOK, dto.ExtractAPIResponse{
				Success:  true,
				URL:      cached.URL,
				Title:    cached.Title,
				Markdown: cached.Markdown,
				Cached:   true,
			})
		}
	}

	resp, ok := h.send(c, req.Browser, &capture.BrowserRequest{
		Source:  "http_api",
		Action:  "mcp_request",
		Command: "extract",
		URL:     req.URL,
	})
	if !ok {
		return nil
	}

	url := firstNonEmpty(resp.Result.URL, req.URL)
	title := firstNonEmpty(resp.Result.Title, resp.Result.Content.Title)
	markdown := resp.Result.Content.MarkdownContent()

	// Cache keyed by the requested URL (req.URL) so the lookup above can hit even
	// when the page redirects to a different resolved URL. Only cache successful
	// extractions (non-empty markdown); skip failures so a later retry re-fetches.
	if markdown != "" {
		if err := h.deps.DB.SaveExtractCache(req.URL, title, markdown); err != nil {
			h.deps.Logger.Warn("failed to cache extract result", zap.String("url", req.URL), zap.Error(err))
		}
	}

	return c.JSON(http.StatusOK, dto.ExtractAPIResponse{
		Success:  true,
		URL:      url,
		Title:    title,
		Markdown: markdown,
		Cached:   false,
	})
}

func (h *browserHandlers) screenshot(c echo.Context) error {
	var req dto.ScreenshotAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}

	resp, ok := h.send(c, req.Browser, &capture.BrowserRequest{
		Source:   "http_api",
		Action:   "mcp_request",
		Command:  "capture",
		URL:      req.URL,
		FullPage: req.FullPage,
	})
	if !ok {
		return nil
	}
	return c.JSON(http.StatusOK, dto.ScreenshotAPIResponse{
		Success:   true,
		URL:       firstNonEmpty(resp.Result.URL, req.URL),
		ImageData: resp.Result.ImageData,
	})
}

// runPageScript runs the extension's whitelisted page-script primitive (plan §4.1).
func (h *browserHandlers) runPageScript(c echo.Context) error {
	var req dto.RunPageScriptAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.URL == "" || req.Script == "" {
		return detailErr(c, http.StatusBadRequest, "url and script are required")
	}

	resp, ok := h.send(c, req.Browser, &capture.BrowserRequest{
		Source:   "http_api",
		Action:   "mcp_request",
		Command:  "runPageScript",
		URL:      req.URL,
		Visible:  req.Visible,
		CloseTab: true,
		Params: map[string]any{
			"script": req.Script,
			"params": req.Params,
		},
	})
	if !ok {
		return nil
	}
	return c.JSON(http.StatusOK, dto.RunPageScriptAPIResponse{
		Success: true,
		URL:     firstNonEmpty(resp.Result.URL, req.URL),
		JSON:    resp.Result.JSON,
		Text:    resp.Result.Text,
	})
}

// fetchInPage runs the extension's in-page fetch primitive (plan §4.2): the
// request runs in the page MAIN world with its cookies/origin.
func (h *browserHandlers) fetchInPage(c echo.Context) error {
	var req dto.FetchInPageAPIRequest
	if err := c.Bind(&req); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid request body")
	}
	if req.URL == "" {
		return detailErr(c, http.StatusBadRequest, "url is required")
	}

	params := map[string]any{}
	if req.RequestURL != "" {
		params["requestUrl"] = req.RequestURL
	}
	if req.Method != "" {
		params["method"] = req.Method
	}
	if len(req.Headers) > 0 {
		params["headers"] = req.Headers
	}
	if req.Body != "" {
		params["body"] = req.Body
	}
	if req.Credentials != "" {
		params["credentials"] = req.Credentials
	}

	resp, ok := h.send(c, req.Browser, &capture.BrowserRequest{
		Source:   "http_api",
		Action:   "mcp_request",
		Command:  "fetchInPage",
		URL:      req.URL,
		Visible:  req.Visible,
		CloseTab: true,
		Params:   params,
	})
	if !ok {
		return nil
	}

	status := 0
	if resp.Result.JSON != nil {
		if s, ok := resp.Result.JSON["status"].(float64); ok {
			status = int(s)
		}
	}
	return c.JSON(http.StatusOK, dto.FetchInPageAPIResponse{
		Success: true,
		URL:     firstNonEmpty(resp.Result.URL, req.RequestURL, req.URL),
		Status:  status,
		Text:    resp.Result.Text,
	})
}
