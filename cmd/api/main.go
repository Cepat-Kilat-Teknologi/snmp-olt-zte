package main

import (
	"context"
	"os"
	_ "time/tzdata" // embed the IANA tz database so time.LoadLocation works in the distroless image (no OS tzdata)

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/app"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/buildinfo"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// Build info injected via ldflags:
//
//	go build -ldflags "-X main.version=3.0.0 -X main.commit=abcd123 -X main.buildTime=2026-04-12T10:00:00Z"
//
// Lowercase names match `-X main.version=` used in the Dockerfile build args.
var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

// fatal indirects logger.Fatal so the server-start failure path is testable
// without terminating the test process. Defaults to the real (os.Exit) behavior.
var fatal = logger.Fatal

func main() {
	// Propagate ldflags values to buildinfo so the rest of the app can read
	// them without importing main.
	buildinfo.Version = version
	buildinfo.Commit = commit
	buildinfo.BuildTime = buildTime

	// Initialize global zap logger before anything else so startup logs are structured.
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}
	logger.Init(env, "snmp-olt-zte", version)
	defer func() { _ = logger.Sync() }()

	// Load .env file if present (ignored in production when file doesn't exist).
	if err := godotenv.Load(); err != nil {
		logger.Debug("no .env file found, using environment variables")
	}

	// Note: `service` and `version` are already attached as base fields
	// inside logger.Init, so we only add the env/commit/build_time here
	// to avoid duplicate keys in the JSON output.
	logger.Info("starting application",
		zap.String("env", env),
		zap.String("commit", commit),
		zap.String("build_time", buildTime),
	)

	server := app.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		fatal("failed to start server", zap.Error(err))
	}
}
