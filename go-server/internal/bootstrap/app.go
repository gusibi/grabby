package bootstrap

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

func (a *App) Run(ctx context.Context) error {
	a.aiEngine.Start()
	defer a.aiEngine.Stop()

	a.taskQueue.Start(ctx)
	defer a.taskQueue.Shutdown()

	if err := a.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

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
	server := &http.Server{Addr: addr, Handler: router}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		a.logger.Info("Graceful shutdown initiated...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			a.logger.Error("HTTP server shutdown error", zap.Error(err))
		}

		cronCtx := a.scheduler.Stop()
		<-cronCtx.Done()

		a.taskQueue.Shutdown()
		a.aiEngine.Stop()
		a.wsManager.CloseAll()

		if err := a.db.Close(); err != nil {
			a.logger.Error("Database close error", zap.Error(err))
		}

		a.logger.Info("Shutdown complete")
		os.Exit(0)
	}()

	a.logger.Info("Starting server", zap.String("address", addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
