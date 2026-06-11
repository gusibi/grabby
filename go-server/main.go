package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"go-server/internal/bootstrap"
	"go-server/internal/config"
	"go-server/internal/logging"
)

//go:embed frontend/dist
var frontendFS embed.FS

func main() {
	settings := config.GetSettings()
	logger := logging.GetLogger()
	defer logging.SyncLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApp(settings, logger, frontendFS)
	if err != nil {
		logger.Fatal("Failed to initialize app", zap.Error(err))
	}
	if err := app.Run(ctx); err != nil {
		logger.Fatal("Server stopped with error", zap.Error(err))
	}
}
