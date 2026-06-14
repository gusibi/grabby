package httpiface

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"
	"go.uber.org/zap"

	"go-server/internal/domain/capture"
	"go-server/internal/domain/item"
	"go-server/internal/domain/source"
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

func RegisterHandlers(openAPI, api *echo.Group, deps Dependencies) {
	db := deps.DB
	wsManager := deps.WSManager
	browserRegistry := deps.BrowserRegistry
	settings := deps.Settings
	logger := deps.Logger
	scheduler := deps.Scheduler
	scraper := deps.Scraper

	// Health check
	openAPI.GET("/health", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		browsers := wsManager.GetBrowserList()
		resp := map[string]any{
			"status":            "ok",
			"browser_connected": len(browsers) > 0,
			"browser_count":     len(browsers),
			"browsers":          browsers,
			"timestamp":         time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	// Browser list endpoint
	api.GET("/browsers", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		list := wsManager.GetBrowserList()
		resp := dto.BrowserListResponse{
			Browsers: list,
			Count:    len(list),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	// Browser registration endpoint
	api.POST("/browsers/register", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req dto.BrowserRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"detail":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		browserReg, err := browserRegistry.Register(req.ConnectID, req.Name)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, browserregistry.ErrBrowserRegistryConflict) {
				status = http.StatusConflict
			}
			logger.Warn("Browser registration failed", zap.Error(err))
			http.Error(w, fmt.Sprintf(`{"detail":"%s"}`, err.Error()), status)
			return
		}

		resp := dto.BrowserRegisterResponse{
			Success: true,
			Browser: browserReg,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	// Browser kick endpoint
	api.POST("/browsers/kick", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ConnID string `json:"conn_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"detail":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.ConnID == "" {
			http.Error(w, `{"detail":"conn_id is required"}`, http.StatusBadRequest)
			return
		}

		if err := wsManager.Kick(req.ConnID); err != nil {
			http.Error(w, fmt.Sprintf(`{"detail":"%s"}`, err.Error()), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))

	// API Extract endpoint
	api.POST("/extract", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req dto.ExtractAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"detail":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		browserConnID, err := wsManager.ResolveBrowserConnID(req.Browser)
		if err != nil {
			logger.Warn("Browser not found", zap.String("browser", req.Browser), zap.Error(err))
			http.Error(w, fmt.Sprintf(`{"detail":"%s"}`, err.Error()), http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(settings.APIExtractTimeout*float64(time.Second)))
		defer cancel()

		resp, err := wsManager.SendMessage(ctx, &capture.BrowserRequest{
			Source:  "http_api",
			Action:  "mcp_request",
			Command: "extract",
			URL:     req.URL,
		}, browserConnID)
		if err != nil {
			logger.Error("Extract operation failed", zap.Error(err))
			http.Error(w, fmt.Sprintf(`{"detail":"%s"}`, err.Error()), http.StatusGatewayTimeout)
			return
		}

		if !resp.Success {
			logger.Error("Browser extension returned error", zap.String("error", resp.Error))
			http.Error(w, fmt.Sprintf(`{"detail":"Browser extension error: %s"}`, resp.Error), http.StatusBadGateway)
			return
		}

		out := dto.ExtractAPIResponse{
			Success:  true,
			URL:      firstNonEmpty(resp.Result.URL, req.URL),
			Title:    firstNonEmpty(resp.Result.Title, resp.Result.Content.Title),
			Markdown: resp.Result.Content.MarkdownContent(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))

	// API Screenshot endpoint
	api.POST("/screenshot", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"detail":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req dto.ScreenshotAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"detail":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		browserConnID, err := wsManager.ResolveBrowserConnID(req.Browser)
		if err != nil {
			logger.Warn("Browser not found", zap.String("browser", req.Browser), zap.Error(err))
			http.Error(w, fmt.Sprintf(`{"detail":"%s"}`, err.Error()), http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(settings.APIExtractTimeout*float64(time.Second)))
		defer cancel()

		resp, err := wsManager.SendMessage(ctx, &capture.BrowserRequest{
			Source:   "http_api",
			Action:   "mcp_request",
			Command:  "capture",
			URL:      req.URL,
			FullPage: req.FullPage,
		}, browserConnID)
		if err != nil {
			logger.Error("Screenshot operation failed", zap.Error(err))
			http.Error(w, fmt.Sprintf(`{"detail":"%s"}`, err.Error()), http.StatusGatewayTimeout)
			return
		}

		if !resp.Success {
			logger.Error("Browser extension returned error", zap.String("error", resp.Error))
			http.Error(w, fmt.Sprintf(`{"detail":"Browser extension error: %s"}`, resp.Error), http.StatusBadGateway)
			return
		}

		out := dto.ScreenshotAPIResponse{
			Success:   true,
			URL:       firstNonEmpty(resp.Result.URL, req.URL),
			ImageData: resp.Result.ImageData,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))

	// GET /api/items
	openAPI.GET("/items", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		category := r.URL.Query().Get("category")
		sourceCategory := r.URL.Query().Get("source_category")
		origin := r.URL.Query().Get("origin")
		q := r.URL.Query().Get("q")
		cursor := r.URL.Query().Get("cursor")
		limit := 20
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if parsedLimit, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || parsedLimit != 1 || limit <= 0 {
				limit = 20
			}
		}

		var starred *int
		if sVal := r.URL.Query().Get("starred"); sVal != "" {
			sInt := 0
			if sVal == "1" || sVal == "true" {
				sInt = 1
			}
			starred = &sInt
		}

		var readStatus *int
		if rVal := r.URL.Query().Get("read_status"); rVal != "" {
			var rInt int
			if _, err := fmt.Sscanf(rVal, "%d", &rInt); err == nil {
				readStatus = &rInt
			}
		}

		items, nextCursor, err := db.GetScrapedItems(item.ItemsFilter{
			Category:       category,
			SourceCategory: sourceCategory,
			Origin:         origin,
			Q:              q,
			Starred:        starred,
			ReadStatus:     readStatus,
			Cursor:         cursor,
			Limit:          limit,
		})
		if err != nil {
			logger.Error("Failed to query items", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"items":  items,
			"cursor": nextCursor,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	// GET/POST /api/items/{id} and sub-actions
	itemHandler := func(w http.ResponseWriter, r *http.Request) {
		idStr, subAction, ok := pathIDAndSubAction(r.URL.Path, "items")
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		var id int64
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodGet && subAction == "" {
			scrapedItem, err := db.GetScrapedItem(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if scrapedItem == nil {
				http.Error(w, "Item not found", http.StatusNotFound)
				return
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

			resp := map[string]any{
				"item":         scrapedItem,
				"html_content": htmlContent,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodPost && subAction == "read" {
			var body struct {
				ReadStatus int `json:"read_status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}
			if err := db.MarkItemRead(id, body.ReadStatus); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}

		if r.Method == http.MethodPost && subAction == "star" {
			var body struct {
				Starred int `json:"starred"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}
			if err := db.ToggleItemStarred(id, body.Starred); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	openAPI.GET("/items/:id", wrapHandlerFunc(itemHandler))
	api.POST("/items/:id/read", wrapHandlerFunc(itemHandler))
	api.POST("/items/:id/star", wrapHandlerFunc(itemHandler))

	// GET/POST /api/sources
	api.GET("/sources", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			list, err := db.GetSources()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	api.POST("/sources", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var form source.SourceForm
			if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}

			if form.ID == "" || form.Name == "" || form.Type == "" || form.URL == "" || form.Schedule == "" {
				http.Error(w, "Missing required fields", http.StatusBadRequest)
				return
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

			if err := db.InsertSource(src); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			scheduler.AddOrUpdateSource(src)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(src)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	// PUT/DELETE/POST /api/sources/{id}/...
	sourceHandler := func(w http.ResponseWriter, r *http.Request) {
		id, subAction, ok := pathIDAndSubAction(r.URL.Path, "sources")
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodPut && subAction == "" {
			var form source.SourceForm
			if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}

			src, err := db.GetSource(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if src == nil {
				http.Error(w, "Source not found", http.StatusNotFound)
				return
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

			if err := db.UpdateSource(*src); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			scheduler.AddOrUpdateSource(*src)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(src)
			return
		}

		if r.Method == http.MethodDelete && subAction == "" {
			if err := db.DeleteSource(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			scheduler.RemoveSource(id)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}

		if r.Method == http.MethodPost && subAction == "toggle" {
			var body struct {
				Enabled int `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}

			if err := db.ToggleSource(id, body.Enabled); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			src, err := db.GetSource(id)
			if err == nil && src != nil {
				scheduler.AddOrUpdateSource(*src)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}

		if r.Method == http.MethodPost && subAction == "run" {
			src, err := db.GetSource(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if src == nil {
				http.Error(w, "Source not found", http.StatusNotFound)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			added, err := scraper.ScrapeSource(ctx, *src)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":     true,
				"items_added": added,
			})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	api.PUT("/sources/:id", wrapHandlerFunc(sourceHandler))
	api.DELETE("/sources/:id", wrapHandlerFunc(sourceHandler))
	api.POST("/sources/:id/toggle", wrapHandlerFunc(sourceHandler))
	api.POST("/sources/:id/run", wrapHandlerFunc(sourceHandler))

	// GET /api/logs
	api.GET("/logs", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sourceID := r.URL.Query().Get("source_id")
		logs, err := db.GetFetchLogs(sourceID, 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(logs)
	}))

	// GET /api/stats
	openAPI.GET("/stats", wrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		totalCount, _ := db.CountItems()
		unreadCount, _ := db.CountUnreadItems()
		starredCount, _ := db.CountStarredItems()

		catCounts, _ := db.CountItemsByCategory()
		sourceCatUnread, _ := db.CountItemsBySourceCategory()
		allSourceCats, _ := db.GetDistinctSourceCategories()

		resp := map[string]any{
			"total_count":            totalCount,
			"unread_count":           unreadCount,
			"starred_count":          starredCount,
			"categories":             catCounts,
			"source_categories":      allSourceCats,
			"source_category_unread": sourceCatUnread,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func pathIDAndSubAction(path, resource string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part != resource {
			continue
		}
		if len(parts) <= i+1 || parts[i+1] == "" {
			return "", "", false
		}
		subAction := ""
		if len(parts) > i+2 {
			subAction = parts[i+2]
		}
		return parts[i+1], subAction, true
	}
	return "", "", false
}

// RegisterAIHandlers registers all AI-related HTTP handlers.
func RegisterAIHandlers(openAPI, api *echo.Group, deps Dependencies) {
	aiHandlers := NewAIHandlers(deps.DB, deps.AIEngine, deps.DailyManager, deps.Logger)
	openAPI.GET("/ai/quality", wrapHandlerFunc(aiHandlers.HandleQuality))
	openAPI.GET("/ai/categories", wrapHandlerFunc(aiHandlers.HandleCategories))
	openAPI.GET("/ai/items", wrapHandlerFunc(aiHandlers.HandleItems))
	openAPI.GET("/ai/analysis/:id", wrapHandlerFunc(aiHandlers.HandleAnalysis))
	openAPI.GET("/ai/daily", wrapHandlerFunc(aiHandlers.HandleDaily))
	openAPI.GET("/ai/daily/list", wrapHandlerFunc(aiHandlers.HandleDailyList))
	openAPI.GET("/ai/daily/rss", wrapHandlerFunc(aiHandlers.HandleDailyRSS))
	openAPI.GET("/ai/stats", wrapHandlerFunc(aiHandlers.HandleStats))

	api.POST("/ai/daily/generate", wrapHandlerFunc(aiHandlers.HandleDailyGenerate))
	api.POST("/ai/reanalyze/:id", wrapHandlerFunc(aiHandlers.HandleReanalyze))
	api.GET("/ai/settings", wrapHandlerFunc(aiHandlers.HandleGetSettings))
	api.POST("/ai/settings", wrapHandlerFunc(aiHandlers.HandleSaveSettings))
	api.POST("/ai/test", wrapHandlerFunc(aiHandlers.HandleTestConnection))
	api.POST("/ai/start_eval", wrapHandlerFunc(aiHandlers.HandleStartEvaluation))
}

// RegisterStaticHandlers serves the embedded frontend SPA.
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
