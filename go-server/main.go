package main

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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yuin/goldmark"
	"go.uber.org/zap"
)

//go:embed frontend/dist
var frontendFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	settings := GetSettings()
	logger := GetLogger()
	defer SyncLogger()

	dbPath := getEnv("DB_PATH", "grabby.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Load AI settings from SQLite database (falling back to env)
	dbAISettings, err := db.LoadAISettings(settings.AISettings)
	if err != nil {
		logger.Error("Failed to load AI settings from database, using env/defaults", zap.Error(err))
	} else {
		settings.AISettings = dbAISettings
	}

	wsManager := NewWebSocketManager(logger)
	browserRegistry, err := NewBrowserRegistry("")
	if err != nil {
		logger.Fatal("Failed to load browser registry", zap.Error(err))
	}

	// --- Initialize AI Engine & Daily Manager ---
	aiEngine, err := NewAIEngine(settings.AISettings, db, logger)
	if err != nil {
		logger.Fatal("Failed to initialize AI Engine", zap.Error(err))
	}
	aiEngine.Start()
	defer aiEngine.Stop()

	dailyManager := NewAIDailyManager(db, aiEngine, logger)

	// --- Initialize Task Queue & Scheduler & Scrapers ---
	taskQueue := NewTaskQueue(wsManager, db, logger, 1, aiEngine) // default serial execution
	taskQueue.Start(context.Background())

	scraper := NewScraper(db, wsManager, taskQueue, logger, aiEngine)

	scheduler := NewScheduler(db, scraper, dailyManager, logger)
	if err := scheduler.Start(context.Background()); err != nil {
		logger.Fatal("Failed to start scheduler", zap.Error(err))
	}

	// --- HTTP Router ---
	mux := http.NewServeMux()

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
		resp := BrowserListResponse{
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

		var req BrowserRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"detail":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		browser, err := browserRegistry.Register(req.ConnectID, req.Name)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, ErrBrowserRegistryConflict) {
				status = http.StatusConflict
			}
			logger.Warn("Browser registration failed", zap.Error(err))
			http.Error(w, fmt.Sprintf(`{"detail":"%s"}`, err.Error()), status)
			return
		}

		resp := BrowserRegisterResponse{
			Success: true,
			Browser: browser,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// API Extract endpoint
	mux.HandleFunc("/api/extract", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ExtractAPIRequest
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

		resp, err := wsManager.SendMessage(ctx, &BrowserRequest{
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

		out := ExtractAPIResponse{
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

		var req ScreenshotAPIRequest
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

		resp, err := wsManager.SendMessage(ctx, &BrowserRequest{
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

		out := ScreenshotAPIResponse{
			Success:   true,
			URL:       firstNonEmpty(resp.Result.URL, req.URL),
			ImageData: resp.Result.ImageData,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	// --- NEW API Endpoints for Dashboard ---

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

		items, nextCursor, err := db.GetScrapedItems(ItemsFilter{
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

	// GET/POST /api/items/{id} and /api/items/{id}/read, /api/items/{id}/star
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
			item, err := db.GetScrapedItem(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if item == nil {
				http.Error(w, "Item not found", http.StatusNotFound)
				return
			}

			// Render Markdown to HTML in backend, or return HTML directly if already HTML
			htmlContent := ""
			if item.Content != "" {
				trimmed := strings.TrimSpace(item.Content)
				if strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, ">") {
					htmlContent = item.Content
				} else {
					var buf bytes.Buffer
					if err := goldmark.Convert([]byte(item.Content), &buf); err == nil {
						htmlContent = buf.String()
					}
				}
			}

			// Automatically mark as read on view
			_ = db.MarkItemRead(id, 1)

			resp := map[string]any{
				"item":         item,
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
			var form SourceForm
			if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}

			if form.ID == "" || form.Name == "" || form.Type == "" || form.URL == "" || form.Schedule == "" {
				http.Error(w, "Missing required fields", http.StatusBadRequest)
				return
			}

			src := Source{
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
			var form SourceForm
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

		var totalCount int
		var unreadCount int
		var starredCount int

		_ = db.db.QueryRow("SELECT COUNT(*) FROM scraped_items").Scan(&totalCount)
		_ = db.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE read_status = 0").Scan(&unreadCount)
		_ = db.db.QueryRow("SELECT COUNT(*) FROM scraped_items WHERE starred = 1").Scan(&starredCount)

		rows, err := db.db.Query("SELECT category, COUNT(*) FROM scraped_items GROUP BY category")
		catCounts := make(map[string]int)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cat string
				var count int
				if err := rows.Scan(&cat, &count); err == nil {
					catCounts[cat] = count
				}
			}
		}

		rows2, err2 := db.db.Query(`
			SELECT s.category, COUNT(*) 
			FROM scraped_items i 
			JOIN sources s ON i.source_id = s.id 
			WHERE i.read_status = 0 
			GROUP BY s.category
		`)
		sourceCatUnread := make(map[string]int)
		if err2 == nil {
			defer rows2.Close()
			for rows2.Next() {
				var cat string
				var count int
				if err := rows2.Scan(&cat, &count); err == nil {
					sourceCatUnread[cat] = count
				}
			}
		}

		rows3, err3 := db.db.Query("SELECT DISTINCT category FROM sources")
		var allSourceCats []string
		if err3 == nil {
			defer rows3.Close()
			for rows3.Next() {
				var cat string
				if err := rows3.Scan(&cat); err == nil && cat != "" {
					allSourceCats = append(allSourceCats, cat)
				}
			}
		}

		resp := map[string]any{
			"total_count":           totalCount,
			"unread_count":          unreadCount,
			"starred_count":         starredCount,
			"categories":            catCounts,
			"source_categories":     allSourceCats,
			"source_category_unread": sourceCatUnread,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// --- AI API Endpoints ---
	aiHandlers := NewAIHandlers(db, aiEngine, dailyManager, logger)
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

	// WebSocket endpoints
	mux.HandleFunc("/ws_browser", handleWebSocketBrowser(wsManager, browserRegistry, logger))
	mux.HandleFunc("/ws_command", handleWebSocketCommand(wsManager, settings, logger))

	// --- MCP Server ---
	mcpSvr := server.NewMCPServer("Grabby", "1.0.0")

	// Register screenshot tool
	screenshotTool := mcp.NewTool("screenshot",
		mcp.WithDescription("Capture a screenshot of a specified webpage"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL of the webpage to capture")),
		mcp.WithBoolean("fullPage", mcp.DefaultBool(false), mcp.Description("Whether to capture the full page")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional, uses default if not specified)")),
	)
	mcpSvr.AddTool(screenshotTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := parseArgs[ScreenshotParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		connID, err := wsManager.ResolveBrowserConnID(params.Browser)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Browser not available: %s", err.Error())), nil
		}

		logger.Info("Executing screenshot", zap.String("url", params.URL), zap.String("browser", params.Browser))
		resp, err := wsManager.SendMessage(ctx, &BrowserRequest{
			Source:   "mcp_client",
			Action:   "mcp_request",
			Command:  "capture",
			URL:      params.URL,
			FullPage: params.FullPage,
		}, connID)
		if err != nil {
			logger.Error("Screenshot failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("Screenshot failed: %s", err.Error())), nil
		}

		if resp.Result.ImageData != "" {
			logger.Info("Screenshot succeeded", zap.String("url", params.URL))
			return mcp.NewToolResultText(resp.Result.ImageData), nil
		}
		logger.Warn("Screenshot response missing image data", zap.Any("response", resp))
		return mcp.NewToolResultText(""), nil
	})

	// Register extract tool
	extractTool := mcp.NewTool("extract",
		mcp.WithDescription("Extract the content of a specified webpage and return it in Markdown format"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL of the webpage to extract content from")),
		mcp.WithString("browser", mcp.Description("Browser name to use (optional, uses default if not specified)")),
	)
	mcpSvr.AddTool(extractTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := parseArgs[ExtractParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		connID, err := wsManager.ResolveBrowserConnID(params.Browser)
		if err != nil {
			logger.Error("Browser not available", zap.String("browser", params.Browser))
			return mcp.NewToolResultError(fmt.Sprintf("Browser not available: %s", err.Error())), nil
		}

		logger.Info("Executing extract", zap.String("url", params.URL), zap.String("browser", params.Browser))
		resp, err := wsManager.SendMessage(ctx, &BrowserRequest{
			Source:  "mcp_client",
			Action:  "mcp_request",
			Command: "extract",
			URL:     params.URL,
		}, connID)
		if err != nil {
			logger.Error("Extract failed", zap.Error(err))
			return mcp.NewToolResultError(fmt.Sprintf("Extract failed: %s", err.Error())), nil
		}

		markdown := resp.Result.Content.MarkdownContent()
		if markdown != "" {
			logger.Info("Extract succeeded", zap.String("url", params.URL))
			return mcp.NewToolResultText(markdown), nil
		}
		logger.Warn("Extract response missing content", zap.Any("response", resp))
		return mcp.NewToolResultText(""), nil
	})

	// Register add tool
	addTool := mcp.NewTool("add",
		mcp.WithDescription("Calculate the sum of two numbers"),
		mcp.WithNumber("a", mcp.Required(), mcp.Description("First number")),
		mcp.WithNumber("b", mcp.Required(), mcp.Description("Second number")),
	)
	mcpSvr.AddTool(addTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := parseArgs[AddParams](req.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		result := params.A + params.B
		logger.Debug("Calculating sum", zap.Float64("a", params.A), zap.Float64("b", params.B), zap.Float64("result", result))
		return mcp.NewToolResultText(fmt.Sprintf("%v", result)), nil
	})

	// Register get_server_time tool
	timeTool := mcp.NewTool("get_server_time",
		mcp.WithDescription("Get the current server time"),
	)
	mcpSvr.AddTool(timeTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(time.Now().Format(time.RFC3339)), nil
	})

	// Mount MCP SSE server at /mcp
	sseSvr := server.NewSSEServer(mcpSvr)
	mux.Handle("/mcp/", sseSvr)

	// --- Serve Frontend Static Files & SPA Routing ---
	fsys, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		logger.Warn("Failed to load embedded frontend files", zap.Error(err))
	} else {
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
			// Do not intercept API, WS, or MCP routes
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws_browser" || r.URL.Path == "/ws_command" || strings.HasPrefix(r.URL.Path, "/mcp/") {
				http.NotFound(w, r)
				return
			}
			spa.ServeHTTP(w, r)
		})
	}

	// --- Start HTTP Server ---
	addr := fmt.Sprintf("%s:%d", settings.Host, settings.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful Shutdown Channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Graceful shutdown initiated...")

		// 1. Stop HTTP Server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown error", zap.Error(err))
		}

		// 2. Stop Cron scheduler
		cronCtx := scheduler.Stop()
		<-cronCtx.Done()

		// 3. Shutdown Task Queue
		taskQueue.Shutdown()

		// 3.5. Stop AI Engine
		aiEngine.Stop()

		// 4. Close all active WebSockets
		wsManager.CloseAll()

		// 5. Close DB
		if err := db.Close(); err != nil {
			logger.Error("Database close error", zap.Error(err))
		}

		logger.Info("Shutdown complete")
		os.Exit(0)
	}()

	logger.Info("Starting MCP server", zap.String("address", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed", zap.Error(err))
	}
}

// ---------- WebSocket Handlers ----------

func handleWebSocketBrowser(wm *WebSocketManager, registry *BrowserRegistry, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connID := r.URL.Query().Get("conn_id")
		browserName := r.URL.Query().Get("name")
		if connID == "" || browserName == "" {
			logger.Warn("WebSocket rejected: missing browser id or name", zap.String("conn_id", connID), zap.String("name", browserName))
			http.Error(w, "Missing conn_id or name", http.StatusForbidden)
			return
		}
		if !registry.Validate(connID, browserName) {
			logger.Warn("WebSocket rejected: browser is not registered", zap.String("conn_id", connID), zap.String("name", browserName))
			http.Error(w, "Browser is not registered", http.StatusForbidden)
			return
		}
		if wm.HasConnection(connID) {
			logger.Warn("WebSocket rejected: browser id already connected", zap.String("conn_id", connID))
			http.Error(w, "Browser id already connected", http.StatusConflict)
			return
		}
		if wm.IsBrowserNameActive(browserName) {
			logger.Warn("WebSocket rejected: browser name already connected", zap.String("name", browserName))
			http.Error(w, "Browser name already connected", http.StatusConflict)
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("WebSocket upgrade failed", zap.Error(err))
			return
		}

		conn := NewWSConn(ws, logger)
		wm.Connect(connID, conn)
		if err := wm.RegisterBrowserName(connID, browserName); err != nil {
			logger.Warn("WebSocket rejected after upgrade", zap.Error(err))
			_ = conn.WriteJSON(BrowserResponse{
				Type:    "auth_response",
				Success: false,
				Error:   err.Error(),
			})
			_ = conn.Close()
			wm.Disconnect(connID)
			return
		}
		defer func() {
			wm.UnregisterBrowserName(connID)
			wm.Disconnect(connID)
		}()

		_ = conn.WriteJSON(BrowserResponse{
			Type:      "auth_response",
			Success:   true,
			MessageID: "",
		})

		wm.ReadLoop(connID, conn)
	}
}

func handleWebSocketCommand(wm *WebSocketManager, settings *Settings, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connID := r.URL.Query().Get("conn_id")
		if connID == "" {
			connID = generateID()
		}
		connID = "ws_command:" + connID

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("WebSocket upgrade failed", zap.Error(err))
			return
		}

		conn := NewWSConn(ws, logger)
		wm.Connect(connID, conn)
		defer wm.Disconnect(connID)

		for {
			var cmd BrowserRequest
			if err := conn.ReadJSON(&cmd); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					logger.Warn("Command WebSocket read error", zap.String("conn_id", connID), zap.Error(err))
				}
				return
			}

			if cmd.Command == "" || cmd.URL == "" {
				logger.Warn("Invalid command format", zap.String("command", cmd.Command), zap.String("url", cmd.URL))
				_ = conn.WriteJSON(BrowserResponse{Error: "Invalid command format, requires 'command' and 'url'"})
				continue
			}

			if cmd.MessageID == "" {
				cmd.MessageID = generateID()
			}
			if cmd.Source == "" {
				cmd.Source = "ws_command"
			}
			if cmd.Action == "" {
				cmd.Action = cmd.Command
			}

			browserConnID, err := wm.ResolveBrowserConnID(cmd.Browser)
			if err != nil {
				logger.Error("Command target browser not available", zap.String("browser", cmd.Browser), zap.Error(err))
				_ = conn.WriteJSON(BrowserResponse{Error: err.Error()})
				continue
			}
			logger.Info("Forwarding command to browser", zap.String("command", cmd.Command), zap.String("url", cmd.URL), zap.String("browser", cmd.Browser))

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.WebsocketTimeout*float64(time.Second)))
			resp, err := wm.SendMessage(ctx, &cmd, browserConnID)
			cancel()

			if err != nil {
				logger.Error("Command execution failed", zap.Error(err))
				_ = conn.WriteJSON(BrowserResponse{Error: err.Error()})
				continue
			}

			logger.Info("Received response from browser, forwarding back")
			_ = conn.WriteJSON(resp)
		}
	}
}

// ---------- Helpers ----------

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
