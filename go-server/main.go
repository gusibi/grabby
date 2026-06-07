package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	settings := GetSettings()
	logger := GetLogger()
	defer SyncLogger()

	wsManager := NewWebSocketManager(logger)
	browserRegistry, err := NewBrowserRegistry("")
	if err != nil {
		logger.Fatal("Failed to load browser registry", zap.Error(err))
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

	// Register add tool (demo)
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

	// --- Start HTTP Server ---
	addr := fmt.Sprintf("%s:%d", settings.Host, settings.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

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

		// Send auth confirmation.
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

		// Read commands from this connection and forward to browser.
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

			// Forward to browser.
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
