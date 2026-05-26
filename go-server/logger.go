package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sync"
)

var (
	logger     *zap.Logger
	loggerOnce sync.Once
)

// GetLogger returns the global zap logger instance.
func GetLogger() *zap.Logger {
	loggerOnce.Do(func() {
		cfg := zap.NewProductionConfig()
		cfg.Encoding = "console"
		cfg.EncoderConfig.TimeKey = "time"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.EncoderConfig.CallerKey = ""
		cfg.EncoderConfig.StacktraceKey = ""
		cfg.DisableStacktrace = true

		if GetSettings().Debug {
			cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		} else {
			cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		}

		var err error
		logger, err = cfg.Build()
		if err != nil {
			panic(err)
		}
	})
	return logger
}

// Sync flushes any buffered log entries.
func SyncLogger() {
	if logger != nil {
		_ = logger.Sync()
	}
}
