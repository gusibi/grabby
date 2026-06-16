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
	RegisterAIHandlers(openAPI, api, deps)
	e.GET("/ws_browser", wrapHandlerFunc(wsiface.HandleWebSocketBrowser(deps.WSManager, deps.BrowserRegistry, deps.Logger)))
	e.GET("/ws_command", wrapHandlerFunc(wsiface.HandleWebSocketCommand(deps.WSManager, deps.Settings, deps.Logger)))
	sseSvr := mcpiface.NewServer(deps.WSManager, deps.Settings, deps.Logger)
	e.Any("/mcp/*", echo.WrapHandler(sseSvr))
	RegisterStaticHandlers(e, deps.FrontendFS, deps.Logger)
	return e
}
