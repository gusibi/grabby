package httpiface

import (
	"embed"
	"net/http"

	aiapp "go-server/internal/application/ai"
	"go-server/internal/application/scheduler"
	"go-server/internal/application/scraping"
	"go-server/internal/config"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
	mcpiface "go-server/internal/interfaces/mcp"
	wsiface "go-server/internal/interfaces/websocket"

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

// NewRouter creates and returns a fully-configured HTTP handler.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	RegisterHandlers(mux, deps)
	RegisterAIHandlers(mux, deps)
	mux.HandleFunc("/ws_browser", wsiface.HandleWebSocketBrowser(deps.WSManager, deps.BrowserRegistry, deps.Logger))
	mux.HandleFunc("/ws_command", wsiface.HandleWebSocketCommand(deps.WSManager, deps.Settings, deps.Logger))
	sseSvr := mcpiface.NewServer(deps.WSManager, deps.Settings, deps.Logger)
	mux.Handle("/mcp/", sseSvr)
	RegisterStaticHandlers(mux, deps.FrontendFS, deps.Logger)
	return mux
}
