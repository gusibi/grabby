package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WSConn wraps a gorilla websocket with safe concurrent access.
type WSConn struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	logger *zap.Logger
}

func NewWSConn(conn *websocket.Conn, logger *zap.Logger) *WSConn {
	return &WSConn{conn: conn, logger: logger}
}

func (w *WSConn) WriteJSON(v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(v)
}

func (w *WSConn) ReadJSON(v any) error {
	return w.conn.ReadJSON(v)
}

func (w *WSConn) Close() error {
	return w.conn.Close()
}

// BrowserInfo represents a connected browser instance.
type BrowserInfo struct {
	ConnID string `json:"conn_id"`
	Name   string `json:"name"`
}

// WebSocketManager manages active WebSocket connections and pending responses.
type WebSocketManager struct {
	mu                sync.RWMutex
	activeConnections map[string]*WSConn
	pendingResponses  map[string]chan *BrowserResponse
	browserNames      map[string]string // conn_id -> name
	logger            *zap.Logger
}

func NewWebSocketManager(logger *zap.Logger) *WebSocketManager {
	return &WebSocketManager{
		activeConnections: make(map[string]*WSConn),
		pendingResponses:  make(map[string]chan *BrowserResponse),
		browserNames:      make(map[string]string),
		logger:            logger,
	}
}

// Connect registers a new WebSocket connection.
func (wm *WebSocketManager) Connect(connID string, ws *WSConn) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.activeConnections[connID] = ws
	wm.logger.Info("WebSocket connection established", zap.String("conn_id", connID))
	wm.logger.Debug("Active connection count", zap.Int("count", len(wm.activeConnections)))
}

// Disconnect removes a WebSocket connection.
func (wm *WebSocketManager) Disconnect(connID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, ok := wm.activeConnections[connID]; ok {
		delete(wm.activeConnections, connID)
		wm.logger.Info("WebSocket connection closed", zap.String("conn_id", connID))
	}
	wm.logger.Debug("Active connection count", zap.Int("count", len(wm.activeConnections)))
}

// HasConnection checks if a connection exists.
func (wm *WebSocketManager) HasConnection(connID string) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	_, ok := wm.activeConnections[connID]
	return ok
}

// RegisterBrowserName associates a human-readable name with a connection.
func (wm *WebSocketManager) RegisterBrowserName(connID, name string) error {
	if name == "" {
		return errors.New("browser name is required")
	}
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for existingConnID, existingName := range wm.browserNames {
		if existingConnID != connID {
			if _, ok := wm.activeConnections[existingConnID]; ok && existingName == name {
				return fmt.Errorf("browser name already connected: %s", name)
			}
		}
	}
	wm.browserNames[connID] = name
	wm.logger.Info("Browser registered", zap.String("conn_id", connID), zap.String("name", name))
	return nil
}

// UnregisterBrowserName removes the name mapping for a connection.
func (wm *WebSocketManager) UnregisterBrowserName(connID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	delete(wm.browserNames, connID)
}

// GetBrowserList returns all connected browsers with their names.
func (wm *WebSocketManager) GetBrowserList() []BrowserInfo {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	list := make([]BrowserInfo, 0, len(wm.browserNames))
	for connID, name := range wm.browserNames {
		if _, ok := wm.activeConnections[connID]; ok {
			list = append(list, BrowserInfo{ConnID: connID, Name: name})
		}
	}
	return list
}

// IsBrowserNameActive checks whether a named browser is already connected.
func (wm *WebSocketManager) IsBrowserNameActive(name string) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	for connID, activeName := range wm.browserNames {
		if activeName == name {
			if _, ok := wm.activeConnections[connID]; ok {
				return true
			}
		}
	}
	return false
}

// ResolveBrowserConnID finds a connection ID by browser name.
// If name is empty, returns the default connection (first active or configured default).
func (wm *WebSocketManager) ResolveBrowserConnID(name string) (string, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	if len(wm.activeConnections) == 0 {
		return "", errors.New("no browser connections available")
	}

	// If no name specified, use the configured default browser name.
	if name == "" {
		defaultName := GetSettings().DefaultBrowser
		if defaultName != "" {
			for connID, n := range wm.browserNames {
				if n == defaultName {
					return connID, nil
				}
			}
			// Default configured but not found — fall through to first connection.
		}
		// Return the first active connection.
		for connID := range wm.browserNames {
			if _, ok := wm.activeConnections[connID]; ok {
				return connID, nil
			}
		}
		return "", errors.New("no browser connections available")
	}

	// Look up by name.
	for connID, n := range wm.browserNames {
		if n == name {
			if _, ok := wm.activeConnections[connID]; !ok {
				continue
			}
			return connID, nil
		}
	}
	return "", fmt.Errorf("browser '%s' not found", name)
}

// SendMessage sends a request to the target connection and waits for a response.
func (wm *WebSocketManager) SendMessage(ctx context.Context, req *BrowserRequest, targetConnID string) (*BrowserResponse, error) {
	wm.mu.RLock()
	ws, ok := wm.activeConnections[targetConnID]
	wm.mu.RUnlock()

	if !ok {
		return nil, errors.New("target WebSocket connection not found")
	}

	// Ensure message_id exists.
	if req.MessageID == "" {
		req.MessageID = generateID()
	}

	// Create a response channel.
	respCh := make(chan *BrowserResponse, 1)
	wm.mu.Lock()
	wm.pendingResponses[req.MessageID] = respCh
	wm.mu.Unlock()

	defer func() {
		wm.mu.Lock()
		delete(wm.pendingResponses, req.MessageID)
		wm.mu.Unlock()
	}()

	wm.logger.Debug("Sending message",
		zap.String("target_conn_id", targetConnID),
		zap.String("message_id", req.MessageID),
		zap.String("command", req.Command),
	)

	if err := ws.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for response with context timeout.
	timeout := time.Duration(GetSettings().WebsocketTimeout * float64(time.Second))
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		return nil, errors.New("waiting for response timed out")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleResponse routes a response to its pending request.
func (wm *WebSocketManager) HandleResponse(resp *BrowserResponse) {
	if resp.MessageID == "" {
		wm.logger.Warn("Response missing message_id", zap.Any("data", resp))
		return
	}

	wm.mu.Lock()
	ch, ok := wm.pendingResponses[resp.MessageID]
	wm.mu.Unlock()

	if !ok {
		wm.logger.Warn("No pending request for response", zap.String("message_id", resp.MessageID))
		return
	}

	select {
	case ch <- resp:
		wm.logger.Debug("Response delivered", zap.String("message_id", resp.MessageID))
	default:
		wm.logger.Warn("Response channel full, dropping", zap.String("message_id", resp.MessageID))
	}
}

// ReadLoop continuously reads messages from a connection and routes them.
func (wm *WebSocketManager) ReadLoop(connID string, ws *WSConn) {
	defer wm.Disconnect(connID)

	for {
		var resp BrowserResponse
		if err := ws.ReadJSON(&resp); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				wm.logger.Warn("WebSocket read error", zap.String("conn_id", connID), zap.Error(err))
			}
			return
		}

		// Messages with a message_id are treated as responses.
		if resp.MessageID != "" {
			wm.HandleResponse(&resp)
			continue
		}

		// Otherwise, log and ignore (heartbeat, auth, etc.).
		wm.logger.Debug("Received non-response message", zap.String("conn_id", connID), zap.Any("data", resp))
	}
}

var idCounter uint64
var idMu sync.Mutex

func generateID() string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounter++
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), idCounter)
}

// CloseAll closes all active WebSocket connections.
func (wm *WebSocketManager) CloseAll() {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for connID, conn := range wm.activeConnections {
		wm.logger.Info("Closing connection during shutdown", zap.String("conn_id", connID))
		_ = conn.Close()
	}
	// Clear maps
	wm.activeConnections = make(map[string]*WSConn)
	wm.browserNames = make(map[string]string)
}
