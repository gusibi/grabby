package websocket

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"go-server/internal/config"
	"go-server/internal/domain/capture"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleWebSocketBrowser(wm *browserws.WebSocketManager, registry *browserregistry.BrowserRegistry, logger *zap.Logger) http.HandlerFunc {
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

		conn := browserws.NewWSConn(ws, logger)
		wm.Connect(connID, conn)
		if err := wm.RegisterBrowserName(connID, browserName); err != nil {
			logger.Warn("WebSocket rejected after upgrade", zap.Error(err))
			_ = conn.WriteJSON(capture.BrowserResponse{
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

		_ = conn.WriteJSON(capture.BrowserResponse{
			Type:      "auth_response",
			Success:   true,
			MessageID: "",
		})

		wm.ReadLoop(connID, conn)
	}
}

func HandleWebSocketCommand(wm *browserws.WebSocketManager, settings *config.Settings, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connID := r.URL.Query().Get("conn_id")
		if connID == "" {
			connID = uuid.New().String()
		}
		connID = "ws_command:" + connID

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("WebSocket upgrade failed", zap.Error(err))
			return
		}

		conn := browserws.NewWSConn(ws, logger)
		wm.Connect(connID, conn)
		defer wm.Disconnect(connID)

		for {
			var cmd capture.BrowserRequest
			if err := conn.ReadJSON(&cmd); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					logger.Warn("Command WebSocket read error", zap.String("conn_id", connID), zap.Error(err))
				}
				return
			}

			if cmd.Command == "" || cmd.URL == "" {
				logger.Warn("Invalid command format", zap.String("command", cmd.Command), zap.String("url", cmd.URL))
				_ = conn.WriteJSON(capture.BrowserResponse{Error: "Invalid command format, requires 'command' and 'url'"})
				continue
			}

			if cmd.MessageID == "" {
				cmd.MessageID = uuid.New().String()
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
				_ = conn.WriteJSON(capture.BrowserResponse{Error: err.Error()})
				continue
			}
			logger.Info("Forwarding command to browser", zap.String("command", cmd.Command), zap.String("url", cmd.URL), zap.String("browser", cmd.Browser))

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.WebsocketTimeout*float64(time.Second)))
			resp, err := wm.SendMessage(ctx, &cmd, browserConnID)
			cancel()

			if err != nil {
				logger.Error("Command execution failed", zap.Error(err))
				_ = conn.WriteJSON(capture.BrowserResponse{Error: err.Error()})
				continue
			}

			logger.Info("Received response from browser, forwarding back")
			_ = conn.WriteJSON(resp)
		}
	}
}
