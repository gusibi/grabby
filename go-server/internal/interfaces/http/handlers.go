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

func RegisterHandlers(mux *http.ServeMux, deps Dependencies) {
	db := deps.DB
	wsManager := deps.WSManager
	browserRegistry := deps.BrowserRegistry
	settings := deps.Settings
	logger := deps.Logger
	scheduler := deps.Scheduler
	scraper := deps.Scraper

	// Health check
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Browser list endpoint
	mux.HandleFunc("/api/browsers", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Browser registration endpoint
	mux.HandleFunc("/api/browsers/register", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Browser kick endpoint
	mux.HandleFunc("/api/browsers/kick", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// API Extract endpoint
	mux.HandleFunc("/api/extract", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// API Screenshot endpoint
	mux.HandleFunc("/api/screenshot", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// GET /api/items
	mux.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		category := r.URL.Query().Get("category")
		sourceCategory := r.URL.Query().Get("source_category")
		origin := r.URL.Query().Get("origin")
		q := r.URL.Query().Get("q")
		cursor := r.URL.Query().Get("cursor")

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
			Limit:          20,
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
	})

	// GET/POST /api/items/{id} and sub-actions
	mux.HandleFunc("/api/items/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		idStr := parts[3]
		var id int64
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		subAction := ""
		if len(parts) > 4 {
			subAction = parts[4]
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

			_ = db.MarkItemRead(id, 1)

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
	})

	// GET/POST /api/sources
	mux.HandleFunc("/api/sources", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// PUT/DELETE/POST /api/sources/{id}/...
	mux.HandleFunc("/api/sources/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		id := parts[3]
		subAction := ""
		if len(parts) > 4 {
			subAction = parts[4]
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
	})

	// GET /api/logs
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// GET /api/stats
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
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
	})
}

// RegisterAIHandlers registers all AI-related HTTP handlers.
func RegisterAIHandlers(mux *http.ServeMux, deps Dependencies) {
	aiHandlers := NewAIHandlers(deps.DB, deps.AIEngine, deps.DailyManager, deps.Logger)
	mux.HandleFunc("/api/ai/quality", aiHandlers.HandleQuality)
	mux.HandleFunc("/api/ai/categories", aiHandlers.HandleCategories)
	mux.HandleFunc("/api/ai/items", aiHandlers.HandleItems)
	mux.HandleFunc("/api/ai/analysis/", aiHandlers.HandleAnalysis)
	mux.HandleFunc("/api/ai/daily", aiHandlers.HandleDaily)
	mux.HandleFunc("/api/ai/daily/list", aiHandlers.HandleDailyList)
	mux.HandleFunc("/api/ai/daily/generate", aiHandlers.HandleDailyGenerate)
	mux.HandleFunc("/api/ai/daily/rss", aiHandlers.HandleDailyRSS)
	mux.HandleFunc("/api/ai/reanalyze/", aiHandlers.HandleReanalyze)
	mux.HandleFunc("/api/ai/stats", aiHandlers.HandleStats)
	mux.HandleFunc("/api/ai/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			aiHandlers.HandleGetSettings(w, r)
		} else if r.Method == http.MethodPost {
			aiHandlers.HandleSaveSettings(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/ai/test", aiHandlers.HandleTestConnection)
	mux.HandleFunc("/api/ai/start_eval", aiHandlers.HandleStartEvaluation)
}

// RegisterStaticHandlers serves the embedded frontend SPA.
func RegisterStaticHandlers(mux *http.ServeMux, frontendFS embed.FS, logger *zap.Logger) {
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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws_browser" || r.URL.Path == "/ws_command" || strings.HasPrefix(r.URL.Path, "/mcp/") {
			http.NotFound(w, r)
			return
		}
		spa.ServeHTTP(w, r)
	})
}
