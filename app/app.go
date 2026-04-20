package app

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/health"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/repository"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/trap"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/usecase"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/graceful"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/redis"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/snmp"
	rds "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// App represents the main application structure that holds the HTTP router
// and manages the application lifecycle, including dependency initialization
// and server startup.
type App struct {
	router http.Handler
}

// New creates and returns a new instance of the App with initialized dependencies.
// It prepares the application for startup but does not start the server.
func New() *App {
	return &App{}
}

// Start initializes the application components, sets up connections to external
// services (Redis and SNMP), and starts the HTTP server. It handles graceful
// shutdown on context cancellation and ensures proper cleanup of resources.
func (a *App) Start(ctx context.Context) error {
	// Load configuration from environment variables (no config file needed).
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("failed to load config", zap.Error(err))
		return err
	}

	// Initialize Redis client.
	redisClient := redis.NewRedisClient(cfg)

	// Check Redis connection.
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to ping redis server", zap.Error(err))
	} else {
		logger.Info("redis server successfully connected")
	}

	// Close Redis client on shutdown.
	defer func(redisClient *rds.Client) {
		if err := redisClient.Close(); err != nil {
			logger.Error("failed to close redis client", zap.Error(err))
		}
	}(redisClient)

	// Initialize SNMP connection.
	snmpConn, err := snmp.SetupSnmpConnection(cfg)
	if err != nil {
		logger.Error("failed to setup snmp connection", zap.Error(err))
		return err
	}
	logger.Info("snmp server successfully connected")

	// Initialize repository (creates connection pool from seed connection).
	snmpRepo := repository.NewPonRepositoryWithConcurrency(snmpConn, cfg.SnmpCfg.MaxConcurrent)

	// Close all pool connections on shutdown.
	defer snmpRepo.Close()
	redisRepo := repository.NewOnuRedisRepo(redisClient)

	// Initialize usecase.
	onuUsecase := usecase.NewOnuUsecase(snmpRepo, redisRepo, cfg)

	// Initialize handler.
	onuHandler := handler.NewOnuHandler(onuUsecase)

	// Pre-warm cache in background.
	if cfg.CacheCfg.PreWarm {
		go onuUsecase.PreWarmCache(ctx)
	}

	// Start SNMP Trap listener if enabled.
	if cfg.TrapCfg.Enabled {
		var webhookClient *trap.WebhookClient
		if cfg.TrapCfg.WebhookURL != "" {
			formatter, finalURL := trap.NewFormatter(
				cfg.TrapCfg.WebhookURL,
				cfg.TrapCfg.WebhookType,
				cfg.TrapCfg.WebhookChatID,
			)
			webhookClient = trap.NewWebhookClient(
				finalURL,
				cfg.TrapCfg.WebhookRetries,
				cfg.TrapCfg.WebhookTimeout,
				formatter,
			)
			logger.Info("webhook_client_initialized",
				zap.String("platform", trap.DetectPlatform(cfg.TrapCfg.WebhookURL)),
				zap.String("url", finalURL))
		}

		var batcher *trap.Batcher
		if webhookClient != nil {
			intervals := trap.BuildIntervals(
				cfg.TrapCfg.CriticalInterval,
				cfg.TrapCfg.HighInterval,
				cfg.TrapCfg.MediumInterval,
				cfg.TrapCfg.LowInterval,
			)
			if len(intervals) > 0 {
				batcher = trap.NewBatcher(webhookClient, onuUsecase, intervals)
				batcher.HighThreshold = cfg.TrapCfg.RxPowerHighThreshold
				batcher.LowThreshold = cfg.TrapCfg.RxPowerLowThreshold
				batcher.RepeatIntervals = trap.BuildRepeatIntervals(
					cfg.TrapCfg.CriticalRepeat,
					cfg.TrapCfg.HighRepeat,
					cfg.TrapCfg.MediumRepeat,
					cfg.TrapCfg.LowRepeat,
				)
				go batcher.Start()
				defer func() { _ = batcher.Close() }()
			}
		}

		trapHandler := trap.NewHandler(webhookClient, batcher, onuUsecase)
		trapListener := trap.NewListener(trap.ListenerConfig{
			Port:      cfg.TrapCfg.Port,
			Community: cfg.TrapCfg.Community,
			OnEvent:   trapHandler.HandleEvent,
		})

		go func() {
			if err := trapListener.Start(); err != nil {
				logger.Error("snmp trap listener failed", zap.Error(err))
			}
		}()
		// Wait for listener to be ready.
		<-trapListener.Listening()
		logger.Info("snmp trap listener started", zap.Uint16("port", cfg.TrapCfg.Port))

		defer func() { _ = trapListener.Close() }()

		// Start RX Power monitor if enabled.
		if cfg.TrapCfg.PowerMonitor && webhookClient != nil {
			powerMonitor := trap.NewPowerMonitor(trap.PowerMonitorConfig{
				Interval:      time.Duration(cfg.TrapCfg.PowerMonitorInterval) * time.Second,
				Cron:          cfg.TrapCfg.PowerMonitorCron,
				Timezone:      cfg.TrapCfg.PowerMonitorTimezone,
				HighThreshold: cfg.TrapCfg.RxPowerHighThreshold,
				LowThreshold:  cfg.TrapCfg.RxPowerLowThreshold,
				Source:        cfg.SnmpCfg.IP,
			}, onuUsecase, webhookClient)
			if batcher != nil {
				powerMonitor.SetBatcher(batcher)
			}
			go powerMonitor.Start()
			defer func() { _ = powerMonitor.Close() }()
			logger.Info("rx power monitor started",
				zap.Float64("high_threshold", cfg.TrapCfg.RxPowerHighThreshold),
				zap.Float64("low_threshold", cfg.TrapCfg.RxPowerLowThreshold),
				zap.Int("interval_sec", cfg.TrapCfg.PowerMonitorInterval),
				zap.String("cron", cfg.TrapCfg.PowerMonitorCron),
				zap.String("timezone", cfg.TrapCfg.PowerMonitorTimezone),
			)
		}
	}

	// Register dependency probes for /readyz.
	// Redis ping is cached for 5s; SNMP reachability is cached for 30s since
	// it is a heavier check (goes over the wire to the OLT).
	checker := health.NewChecker(2 * time.Second)
	checker.Register("redis", 5*time.Second, func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	})
	checker.Register("snmp", 30*time.Second, func(_ context.Context) error {
		// Lightweight reachability check: acquire and release a pooled
		// connection. If the pool is exhausted or the seed connection is
		// broken, this fails — the readyz response will surface that.
		return snmpRepo.Ping()
	})

	// Initialize router with the health checker.
	a.router = loadRoutes(onuHandler, checker)

	// Start server.
	addr := os.Getenv("SERVER_PORT")
	if addr == "" {
		addr = "8081"
	}
	server := &http.Server{
		Addr:    ":" + addr,
		Handler: a.router,
	}

	logger.Info("application started", zap.String("addr", addr))

	// Graceful shutdown.
	return graceful.Shutdown(ctx, server)
}
