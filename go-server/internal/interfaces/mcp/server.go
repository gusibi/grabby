package mcpiface

import (
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"go-server/internal/config"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
)

// Deps carries everything the MCP tools need.
type Deps struct {
	WM       *browserws.WebSocketManager
	DB       *sqlite.Database
	Settings *config.Settings
	Logger   *zap.Logger
}

// Servers holds the two HTTP transports we expose for the same tool set.
type Servers struct {
	// Streamable is the current MCP HTTP transport, served at /mcp.
	Streamable *server.StreamableHTTPServer
	// SSE is the legacy transport, served at /mcp/sse (+ /mcp/message).
	SSE *server.SSEServer
}

// NewServer builds the MCP server and registers all tools. Tool groups live in
// their own files: browser_tools.go (screenshot/extract/fetch/script),
// twitter_tools.go, reddit_tools.go, xiaohongshu_tools.go and
// library_tools.go (stored items / sources / AI digests).
func NewServer(deps Deps) Servers {
	mcpSvr := server.NewMCPServer("Grabby", "1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	registerBrowserTools(mcpSvr, deps.WM, deps.Logger)
	registerTwitterTools(mcpSvr, deps.WM, deps.Logger)
	registerRedditTools(mcpSvr, deps.WM, deps.Logger)
	registerXiaohongshuTools(mcpSvr, deps.WM, deps.Logger)
	registerLibraryTools(mcpSvr, deps.DB, deps.Logger)

	return Servers{
		Streamable: server.NewStreamableHTTPServer(mcpSvr),
		// The SSE transport matches on absolute paths, so it must know it is
		// mounted under /mcp — without this /mcp/sse returns 404.
		SSE: server.NewSSEServer(mcpSvr, server.WithStaticBasePath("/mcp")),
	}
}
