// Package logger provides application-wide structured logging using zap.
//
// Design:
//   - JSON encoder in production, colored console encoder in development.
//   - ISO8601 UTC timestamps with millisecond precision.
//   - Required base fields (service, version) attached at init time so every
//     log line carries them automatically.
//   - WithRequestID helper for per-request loggers.
//
// This package is the project-wide replacement for zerolog. Import it as:
//
//	import "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
//
// Then call logger.Info / logger.Error / logger.WithRequestID etc.
package logger

import (
	"context"
	stdlog "log"
	"sync"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/reqctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	appLogger *zap.Logger
	once      sync.Once
)

// Init initializes the global application logger for the given environment and
// service metadata. Safe to call multiple times — only the first call takes effect.
//
// env:     "production" / "staging" / "development" / "dev"
// service: service identifier (e.g. "go-snmp-olt-zte-c320")
// version: build version string (usually from ldflags)
func Init(env, service, version string) {
	once.Do(func() {
		var cfg zap.Config
		if env == "development" || env == "dev" {
			cfg = zap.NewDevelopmentConfig()
			cfg.Encoding = "console"
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		} else {
			cfg = zap.NewProductionConfig()
			cfg.Encoding = "json"
			cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		}

		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.MessageKey = "message"
		cfg.EncoderConfig.LevelKey = "level"
		cfg.EncoderConfig.CallerKey = ""
		cfg.EncoderConfig.StacktraceKey = ""

		base, err := cfg.Build()
		if err != nil {
			stdlog.Fatalf("failed to init zap logger: %v", err)
		}
		appLogger = base.With(
			zap.String("service", service),
			zap.String("version", version),
		)
	})
}

// L returns the global application logger. If Init was not called yet, it
// initializes with safe production defaults so that library code never panics.
func L() *zap.Logger {
	if appLogger == nil {
		Init("production", "go-snmp-olt-zte-c320", "unknown")
	}
	return appLogger
}

// WithRequestID returns a child logger pre-populated with request_id extracted
// from the given context. Returns the base logger unchanged if no request ID
// is present.
func WithRequestID(ctx context.Context) *zap.Logger {
	reqID := reqctx.RequestIDFromContext(ctx)
	if reqID == "" {
		return L()
	}
	return L().With(zap.String("request_id", reqID))
}

// WithModule returns a child logger tagged with a module field. Use this in
// package init code or handler constructors to scope all logs to a subsystem.
func WithModule(module string) *zap.Logger {
	return L().With(zap.String("module", module))
}

// Info logs an info-level message with the global logger.
func Info(msg string, fields ...zap.Field) { L().Info(msg, fields...) }

// Debug logs a debug-level message with the global logger.
func Debug(msg string, fields ...zap.Field) { L().Debug(msg, fields...) }

// Warn logs a warning-level message with the global logger.
func Warn(msg string, fields ...zap.Field) { L().Warn(msg, fields...) }

// Error logs an error-level message with the global logger.
func Error(msg string, fields ...zap.Field) { L().Error(msg, fields...) }

// Fatal logs a fatal-level message and then calls os.Exit(1).
func Fatal(msg string, fields ...zap.Field) { L().Fatal(msg, fields...) }

// Sync flushes any buffered log entries. Call before process exit.
func Sync() error {
	if appLogger == nil {
		return nil
	}
	return appLogger.Sync()
}

// SetForTest replaces the global logger with a custom one. Intended for tests
// that need to capture log output. Returns a restore function that reverts to
// the previous logger.
//
// Usage:
//
//	buf := &bytes.Buffer{}
//	restore := logger.SetForTest(newTestLogger(buf))
//	defer restore()
func SetForTest(l *zap.Logger) (restore func()) {
	prev := appLogger
	appLogger = l
	return func() { appLogger = prev }
}
