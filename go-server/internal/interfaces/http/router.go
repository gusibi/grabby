package httpiface

import (
	"embed"

	aiapp "go-server/internal/application/ai"
	"go-server/internal/application/scheduler"
	"go-server/internal/application/scraping"
	"go-server/internal/config"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
	mcpiface "go-server/internal/interfaces/mcp"
	wsiface "go-server/internal/interfaces/websocket"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// Dependencies holds all dependencies needed by HTTP handlers.
type Dependencies struct {
	Settings        *config.Settings
	Logger          *zap.Logger
	DB              *sqlite.Database
	WSManager       *browserws.WebSocketManager
	BrowserRegistry *browserregistry.BrowserRegistry
	AIEngine        *aiapp.AIEngine
	DailyManager    *aiapp.AIDailyManager
	TaskQueue       *scraping.TaskQueue
	Scraper         *scraping.Scraper
	Scheduler       *scheduler.Scheduler
	FrontendFS      embed.FS
}

// NewRouter creates and returns a fully-configured Echo router.
func NewRouter(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	auth := newAdminAuth(deps.Settings)
	openAPI := e.Group("/open/api")
	api := e.Group("/api")
	api.Use(auth.middleware)

	RegisterAuthHandlers(api, deps)
	RegisterHandlers(openAPI, api, deps)
	RegisterBrowserHandlers(api, deps)
	RegisterTwitterHandlers(api, deps)
	RegisterRedditHandlers(api, deps)
	RegisterXiaohongshuHandlers(api, deps)
	RegisterAIHandlers(openAPI, api, deps)
	e.GET("/ws_browser", wrapHandlerFunc(wsiface.HandleWebSocketBrowser(deps.WSManager, deps.BrowserRegistry, deps.Logger)))
	e.GET("/ws_command", wrapHandlerFunc(wsiface.HandleWebSocketCommand(deps.WSManager, deps.Settings, deps.Logger)))

	// MCP endpoints, token-protected so an agent can connect directly:
	//   POST/GET /mcp        -> streamable HTTP transport (current standard)
	//   GET      /mcp/sse    -> legacy SSE transport
	//   POST     /mcp/message-> legacy SSE message channel
	mcpSvrs := mcpiface.NewServer(mcpiface.Deps{
		WM:       deps.WSManager,
		DB:       deps.DB,
		Settings: deps.Settings,
		Logger:   deps.Logger,
	})
	mcpGroup := e.Group("/mcp", auth.tokenMiddleware)
	mcpGroup.Any("", echo.WrapHandler(mcpSvrs.Streamable))
	mcpGroup.Any("/", echo.WrapHandler(mcpSvrs.Streamable))
	mcpGroup.Any("/sse", echo.WrapHandler(mcpSvrs.SSE))
	mcpGroup.Any("/message", echo.WrapHandler(mcpSvrs.SSE))

	RegisterStaticHandlers(e, deps.FrontendFS, deps.Logger)
	return e
}
