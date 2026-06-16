package httpiface

import (
	"bytes"
	"context"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"
	"go.uber.org/zap"

	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
)

// contentHandlers wires the core content endpoints: health, items, sources, run
// logs and stats. Browser, Twitter and AI endpoints live in their own files.
type contentHandlers struct {
	deps Dependencies
}

// RegisterHandlers registers the core content endpoints on their groups.
func RegisterHandlers(openAPI, api *echo.Group, deps Dependencies) {
	h := &contentHandlers{deps: deps}

	openAPI.GET("/health", h.health)

	openAPI.GET("/items", h.listItems)
	openAPI.GET("/items/:id", h.getItem)
	api.POST("/items/:id/read", h.markItemRead)
	api.POST("/items/:id/star", h.starItem)

	api.GET("/sources", h.listSources)
	api.POST("/sources", h.createSource)
	api.PUT("/sources/:id", h.updateSource)
	api.DELETE("/sources/:id", h.deleteSource)
	api.POST("/sources/:id/toggle", h.toggleSource)
	api.POST("/sources/:id/run", h.runSource)

	api.GET("/logs", h.listLogs)
	openAPI.GET("/stats", h.stats)
}

func (h *contentHandlers) health(c echo.Context) error {
	browsers := h.deps.WSManager.GetBrowserList()
	return c.JSON(http.StatusOK, map[string]any{
		"status":            "ok",
		"browser_connected": len(browsers) > 0,
		"browser_count":     len(browsers),
		"browsers":          browsers,
		"timestamp":         time.Now(),
	})
}

// --- items ---

func (h *contentHandlers) listItems(c echo.Context) error {
	limit := 20
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}

	var starred *int
	if v := c.QueryParam("starred"); v != "" {
		s := 0
		if v == "1" || v == "true" {
			s = 1
		}
		starred = &s
	}

	var readStatus *int
	if v := c.QueryParam("read_status"); v != "" {
		if r, err := strconv.Atoi(v); err == nil {
			readStatus = &r
		}
	}

	items, nextCursor, err := h.deps.DB.GetScrapedItems(item.ItemsFilter{
		Category:       c.QueryParam("category"),
		SourceCategory: c.QueryParam("source_category"),
		Origin:         c.QueryParam("origin"),
		Q:              c.QueryParam("q"),
		Starred:        starred,
		ReadStatus:     readStatus,
		Cursor:         c.QueryParam("cursor"),
		Limit:          limit,
	})
	if err != nil {
		h.deps.Logger.Error("Failed to query items", zap.Error(err))
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "cursor": nextCursor})
}

func (h *contentHandlers) getItem(c echo.Context) error {
	id, err := paramInt64(c)
	if err != nil {
		return err
	}

	scrapedItem, dbErr := h.deps.DB.GetScrapedItem(id)
	if dbErr != nil {
		return detailErr(c, http.StatusInternalServerError, dbErr.Error())
	}
	if scrapedItem == nil {
		return detailErr(c, http.StatusNotFound, "Item not found")
	}

	htmlContent := ""
	if scrapedItem.Content != "" {
		trimmed := strings.TrimSpace(scrapedItem.Content)
		if strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, ">") {
			htmlContent = scrapedItem.Content
		} else {
			var buf bytes.Buffer
			if err := goldmark.Convert([]byte(scrapedItem.Content), &buf); err == nil {
				htmlContent = buf.String()
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]any{"item": scrapedItem, "html_content": htmlContent})
}

func (h *contentHandlers) markItemRead(c echo.Context) error {
	id, err := paramInt64(c)
	if err != nil {
		return err
	}
	var body struct {
		ReadStatus int `json:"read_status"`
	}
	if err := c.Bind(&body); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid body")
	}
	if err := h.deps.DB.MarkItemRead(id, body.ReadStatus); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *contentHandlers) starItem(c echo.Context) error {
	id, err := paramInt64(c)
	if err != nil {
		return err
	}
	var body struct {
		Starred int `json:"starred"`
	}
	if err := c.Bind(&body); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid body")
	}
	if err := h.deps.DB.ToggleItemStarred(id, body.Starred); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

// --- sources ---

func (h *contentHandlers) listSources(c echo.Context) error {
	list, err := h.deps.DB.GetSources()
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, list)
}

func (h *contentHandlers) createSource(c echo.Context) error {
	var form source.SourceForm
	if err := c.Bind(&form); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid body")
	}
	if form.ID == "" || form.Name == "" || form.Type == "" || form.URL == "" || form.Schedule == "" {
		return detailErr(c, http.StatusBadRequest, "Missing required fields")
	}

	src := source.Source{
		ID:              form.ID,
		Name:            form.Name,
		Type:            form.Type,
		URL:             form.URL,
		Schedule:        form.Schedule,
		Enabled:         1,
		DefaultCategory: form.DefaultCategory,
		Config:          form.Config,
		Category:        form.Category,
	}
	if src.Config == "" {
		src.Config = "{}"
	}

	if err := h.deps.DB.InsertSource(src); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	h.deps.Scheduler.AddOrUpdateSource(src)

	return c.JSON(http.StatusOK, src)
}

func (h *contentHandlers) updateSource(c echo.Context) error {
	id := c.Param("id")
	var form source.SourceForm
	if err := c.Bind(&form); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid body")
	}

	src, err := h.deps.DB.GetSource(id)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	if src == nil {
		return detailErr(c, http.StatusNotFound, "Source not found")
	}

	src.Name = form.Name
	src.Type = form.Type
	src.URL = form.URL
	src.Schedule = form.Schedule
	src.DefaultCategory = form.DefaultCategory
	src.Config = form.Config
	src.Category = form.Category
	if src.Config == "" {
		src.Config = "{}"
	}

	if err := h.deps.DB.UpdateSource(*src); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	h.deps.Scheduler.AddOrUpdateSource(*src)

	return c.JSON(http.StatusOK, src)
}

func (h *contentHandlers) deleteSource(c echo.Context) error {
	id := c.Param("id")
	if err := h.deps.DB.DeleteSource(id); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	h.deps.Scheduler.RemoveSource(id)
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *contentHandlers) toggleSource(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		Enabled int `json:"enabled"`
	}
	if err := c.Bind(&body); err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid body")
	}
	if err := h.deps.DB.ToggleSource(id, body.Enabled); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	if src, err := h.deps.DB.GetSource(id); err == nil && src != nil {
		h.deps.Scheduler.AddOrUpdateSource(*src)
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *contentHandlers) runSource(c echo.Context) error {
	id := c.Param("id")
	src, err := h.deps.DB.GetSource(id)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	if src == nil {
		return detailErr(c, http.StatusNotFound, "Source not found")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
	defer cancel()

	added, err := h.deps.Scraper.ScrapeSource(ctx, *src)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true, "items_added": added})
}

// --- logs & stats ---

func (h *contentHandlers) listLogs(c echo.Context) error {
	logs, err := h.deps.DB.GetFetchLogs(c.QueryParam("source_id"), 50)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, logs)
}

func (h *contentHandlers) stats(c echo.Context) error {
	db := h.deps.DB
	totalCount, _ := db.CountItems()
	unreadCount, _ := db.CountUnreadItems()
	starredCount, _ := db.CountStarredItems()
	catCounts, _ := db.CountItemsByCategory()
	sourceCatUnread, _ := db.CountItemsBySourceCategory()
	allSourceCats, _ := db.GetDistinctSourceCategories()

	return c.JSON(http.StatusOK, map[string]any{
		"total_count":            totalCount,
		"unread_count":           unreadCount,
		"starred_count":          starredCount,
		"categories":             catCounts,
		"source_categories":      allSourceCats,
		"source_category_unread": sourceCatUnread,
	})
}

// paramInt64 parses the `:id` path param as int64. On failure it writes the
// error response and returns a non-nil error for the caller to return.
func paramInt64(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, detailErr(c, http.StatusBadRequest, "Invalid ID format")
	}
	return id, nil
}

// RegisterStaticHandlers serves the embedded frontend SPA. It legitimately uses
// net/http file serving, so it keeps the wrapHandlerFunc bridge.
func RegisterStaticHandlers(e *echo.Echo, frontendFS embed.FS, logger *zap.Logger) {
	fsys, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		logger.Warn("Failed to load embedded frontend files", zap.Error(err))
		return
	}
	fileServer := http.FileServer(http.FS(fsys))
	spa := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filePath := strings.TrimPrefix(r.URL.Path, "/")
		if filePath == "" {
			filePath = "index.html"
		}
		f, err := fsys.Open(filePath)
		if err != nil {
			indexFile, err := fsys.Open("index.html")
			if err != nil {
				http.Error(w, "index.html not found", http.StatusNotFound)
				return
			}
			defer indexFile.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.Copy(w, indexFile)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})

	e.GET("/*", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/open/api/") || r.URL.Path == "/ws_browser" || r.URL.Path == "/ws_command" || strings.HasPrefix(r.URL.Path, "/mcp/") {
			http.NotFound(w, r)
			return
		}
		spa.ServeHTTP(w, r)
	}))
}
