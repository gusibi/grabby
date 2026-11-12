package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go-server/internal/config"
)

var (
	logger     *zap.Logger
	loggerOnce sync.Once
)

// LogFileEnv names the environment variable that mirrors every log line to a
// file. The desktop build sets it so a double-clicked .app (which has no
// terminal attached) still leaves a readable log behind.
const LogFileEnv = "GRABBY_LOG_FILE"

// SetupFileLogging mirrors logs to path in addition to stderr, and returns the
// closer for that file. It must be called before GetLogger; an empty path is a
// no-op. The standard library `log` package is redirected as well, because the
// desktop shell logs its own startup through it.
func SetupFileLogging(path string) io.Closer {
	if path == "" {
		return nil
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	_ = os.Setenv(LogFileEnv, path)
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return f
}

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

		if config.GetSettings().Debug {
			cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		} else {
			cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		}

		if path := os.Getenv(LogFileEnv); path != "" {
			cfg.OutputPaths = []string{"stderr", path}
			cfg.ErrorOutputPaths = []string{"stderr", path}
		}

		var err error
		logger, err = cfg.Build()
		if err != nil {
			panic(err)
		}
	})
	return logger
}

// SyncLogger flushes any buffered log entries.
func SyncLogger() {
	if logger != nil {
		_ = logger.Sync()
	}
}
