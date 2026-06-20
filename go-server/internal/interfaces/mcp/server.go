package mcpiface

import (
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"go-server/internal/config"
	"go-server/internal/infrastructure/browserws"
)

// NewServer builds the MCP SSE server and registers all tools. Tool groups live
// in their own files: browser_tools.go (screenshot/extract) and twitter_tools.go
// (twitter_search/timeline/likes).
func NewServer(wm *browserws.WebSocketManager, settings *config.Settings, logger *zap.Logger) *server.SSEServer {
	mcpSvr := server.NewMCPServer("Grabby", "1.0.0")

	registerBrowserTools(mcpSvr, wm, logger)
	registerTwitterTools(mcpSvr, wm, logger)
	registerRedditTools(mcpSvr, wm, logger)

	return server.NewSSEServer(mcpSvr)
}
