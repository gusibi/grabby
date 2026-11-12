package bootstrap

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"time"

	aiapp "go-server/internal/application/ai"
	"go-server/internal/application/scheduler"
	"go-server/internal/application/scraping"
	"go-server/internal/config"
	"go-server/internal/infrastructure/browserregistry"
	"go-server/internal/infrastructure/browserws"
	"go-server/internal/infrastructure/sqlite"
	httpiface "go-server/internal/interfaces/http"

	"go.uber.org/zap"
)

type App struct {
	settings        *config.Settings
	logger          *zap.Logger
	db              *sqlite.Database
	wsManager       *browserws.WebSocketManager
	browserRegistry *browserregistry.BrowserRegistry
	aiEngine        *aiapp.AIEngine
	dailyManager    *aiapp.AIDailyManager
	taskQueue       *scraping.TaskQueue
	scraper         *scraping.Scraper
	scheduler       *scheduler.Scheduler
	frontendFS      embed.FS
}

func NewApp(settings *config.Settings, logger *zap.Logger, frontendFS embed.FS) (*App, error) {
	dbPath := config.GetEnv("DB_PATH", "grabby.db")
	db, err := sqlite.NewDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	dbAISettings, err := db.LoadAISettings(settings.AISettings)
	if err != nil {
		logger.Error("Failed to load AI settings from database, using env/defaults", zap.Error(err))
	} else {
		settings.AISettings = dbAISettings
	}

	wsManager := browserws.NewWebSocketManager(logger)
	browserRegistry, err := browserregistry.NewBrowserRegistry("")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load browser registry: %w", err)
	}

	aiEngine, err := aiapp.NewAIEngine(settings.AISettings, db, logger)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize AI engine: %w", err)
	}

	dailyManager := aiapp.NewAIDailyManager(db, aiEngine, logger)
	taskQueue := scraping.NewTaskQueue(wsManager, db, logger, 1, aiEngine)
	scraper := scraping.NewScraper(db, wsManager, taskQueue, logger, aiEngine)
	schedulerInstance := scheduler.NewScheduler(db, scraper, dailyManager, logger)

	return &App{
		settings:        settings,
		logger:          logger,
		db:              db,
		wsManager:       wsManager,
		browserRegistry: browserRegistry,
		aiEngine:        aiEngine,
		dailyManager:    dailyManager,
		taskQueue:       taskQueue,
		scraper:         scraper,
		scheduler:       schedulerInstance,
		frontendFS:      frontendFS,
	}, nil
}

// Run starts the background workers and the HTTP/WebSocket/MCP server and
// blocks until ctx is cancelled or the server fails. Both the headless server
// entrypoint and the desktop shell call it, so the caller owns the process
// lifecycle: Run never installs signal handlers and never calls os.Exit.
func (a *App) Run(ctx context.Context) error {
	a.aiEngine.Start()
	a.taskQueue.Start(ctx)

	if err := a.scheduler.Start(ctx); err != nil {
		a.taskQueue.Shutdown()
		a.aiEngine.Stop()
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	defer func() {
		cronCtx := a.scheduler.Stop()
		<-cronCtx.Done()
		a.taskQueue.Shutdown()
		a.aiEngine.Stop()
		a.wsManager.CloseAll()
		if err := a.db.Close(); err != nil {
			a.logger.Error("Database close error", zap.Error(err))
		}
		a.logger.Info("Shutdown complete")
	}()

	router := httpiface.NewRouter(httpiface.Dependencies{
		Settings:        a.settings,
		Logger:          a.logger,
		DB:              a.db,
		WSManager:       a.wsManager,
		BrowserRegistry: a.browserRegistry,
		AIEngine:        a.aiEngine,
		DailyManager:    a.dailyManager,
		TaskQueue:       a.taskQueue,
		Scraper:         a.scraper,
		Scheduler:       a.scheduler,
		FrontendFS:      a.frontendFS,
	})

	addr := fmt.Sprintf("%s:%d", a.settings.Host, a.settings.Port)

	// ctx cancellation drives a graceful shutdown instead of a process signal.
	// The second case keeps the goroutine from lingering if the server stops
	// on its own (for example when the port is already taken).
	serverStopped := make(chan struct{})
	defer close(serverStopped)
	go func() {
		select {
		case <-ctx.Done():
			a.logger.Info("Graceful shutdown initiated...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := router.Shutdown(shutdownCtx); err != nil {
				a.logger.Error("HTTP server shutdown error", zap.Error(err))
			}
		case <-serverStopped:
		}
	}()

	a.logger.Info("Starting server", zap.String("address", addr))
	if err := router.Start(addr); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
