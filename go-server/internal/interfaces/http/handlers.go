package httpiface

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"go-server/internal/application/render"
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
	api.GET("/logs/:id", h.getLogDetail)
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

	var since time.Time
	if v := c.QueryParam("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}

	items, nextCursor, err := h.deps.DB.GetScrapedItems(item.ItemsFilter{
		Category:       c.QueryParam("category"),
		ContentType:    c.QueryParam("content_type"),
		SourceCategory: c.QueryParam("source_category"),
		SourceID:       c.QueryParam("source_id"),
		Origin:         c.QueryParam("origin"),
		Q:              c.QueryParam("q"),
		Starred:        starred,
		ReadStatus:     readStatus,
		Since:          since,
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

	htmlContent := render.Render(scrapedItem.Content)
	summaryHTML := render.Render(scrapedItem.Summary)

	return c.JSON(http.StatusOK, map[string]any{
		"item":         scrapedItem,
		"html_content": htmlContent,
		"summary_html": summaryHTML,
	})
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

// effectiveSourceCategory resolves the content type a source assigns to its
// items. An explicit default category wins; otherwise podcast/youtube sources
// imply their own type so the feed filters work without extra configuration.
func effectiveSourceCategory(src source.Source) string {
	if c := strings.TrimSpace(src.DefaultCategory); c != "" && c != "auto" {
		return c
	}
	switch src.Type {
	case "podcast", "youtube":
		return src.Type
	}
	return ""
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
	if src.DefaultCategory == "" || src.DefaultCategory == "auto" {
		if auto := effectiveSourceCategory(src); auto != "" {
			src.DefaultCategory = auto
		}
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

	oldCategory := effectiveSourceCategory(*src)
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
	if src.DefaultCategory == "" || src.DefaultCategory == "auto" {
		if auto := effectiveSourceCategory(*src); auto != "" {
			src.DefaultCategory = auto
		}
	}
	newCategory := effectiveSourceCategory(*src)

	if err := h.deps.DB.UpdateSource(*src); err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	// Reclassify already-imported items so switching a source to podcast (or
	// YouTube) takes effect immediately instead of only for future scrapes.
	if newCategory != "" && newCategory != oldCategory {
		if err := h.deps.DB.SetSourceItemsCategory(src.ID, newCategory); err != nil && h.deps.Logger != nil {
			h.deps.Logger.Warn("reclassify source items failed", zap.String("source", src.ID), zap.Error(err))
		}
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

	added, logID, err := h.deps.Scraper.RunSource(ctx, *src, "manual")
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	resp := map[string]any{"success": true, "items_added": added}
	if l, lerr := h.deps.DB.GetFetchLog(logID); lerr == nil && l != nil {
		resp["log_id"] = l.ID
		resp["status"] = l.Status
		resp["trigger"] = l.Trigger
		resp["items_found"] = l.ItemsFound
		resp["items_updated"] = l.ItemsUpdated
		resp["items_skipped"] = l.ItemsSkipped
		resp["items_failed"] = l.ItemsFailed
		resp["summary"] = l.Summary
	}
	return c.JSON(http.StatusOK, resp)
}

// --- logs & stats ---

func (h *contentHandlers) listLogs(c echo.Context) error {
	logs, err := h.deps.DB.GetFetchLogs(c.QueryParam("source_id"), 50)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, logs)
}

// getLogDetail returns one run plus every discovered/processed entry and the
// per-text results (feed body, original page, description, captions,
// transcript). It is the API behind the desktop fetch-log detail view.
func (h *contentHandlers) getLogDetail(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return detailErr(c, http.StatusBadRequest, "Invalid ID format")
	}
	log, err := h.deps.DB.GetFetchLog(id)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	if log == nil {
		return detailErr(c, http.StatusNotFound, "Fetch log not found")
	}
	items, err := h.deps.DB.GetFetchLogItems(id)
	if err != nil {
		return detailErr(c, http.StatusInternalServerError, err.Error())
	}
	if items == nil {
		items = []source.FetchLogItem{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"log":     log,
		"items":   items,
	})
}

func (h *contentHandlers) stats(c echo.Context) error {
	db := h.deps.DB
	totalCount, _ := db.CountItems()
	unreadCount, _ := db.CountUnreadItems()
	starredCount, _ := db.CountStarredItems()
	catCounts, _ := db.CountItemsByCategory()
	sourceCatUnread, _ := db.CountItemsBySourceCategory()
	allSourceCats, _ := db.GetDistinctSourceCategories()
	sourceCounts, _ := db.CountItemsBySource()
	now := time.Now()
	todayCount, _ := db.CountItemsSince(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))

	return c.JSON(http.StatusOK, map[string]any{
		"total_count":            totalCount,
		"unread_count":           unreadCount,
		"starred_count":          starredCount,
		"today_count":            todayCount,
		"categories":             catCounts,
		"source_categories":      allSourceCats,
		"source_category_unread": sourceCatUnread,
		"source_counts":          sourceCounts,
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

// RegisterStaticHandlers serves a bundled landing page, when the build has one.
// The headless server ships a small status page (web/index.html); the desktop
// app's window is native UI and ships no web frontend, so it passes an empty FS
// and no static route is registered.
func RegisterStaticHandlers(e *echo.Echo, staticFS embed.FS, logger *zap.Logger) {
	root := ""
	if _, err := fs.Stat(staticFS, "web/index.html"); err == nil {
		root = "web"
	}
	if root == "" {
		return
	}
	fsys, err := fs.Sub(staticFS, root)
	if err != nil {
		logger.Warn("Failed to load embedded web files", zap.Error(err))
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
