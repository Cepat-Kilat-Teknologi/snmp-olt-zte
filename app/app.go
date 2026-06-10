package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/health"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/repository"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/trap"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/usecase"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/graceful"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/redis"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/snmp"
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

	redisRepo := repository.NewOnuRedisRepo(redisClient)

	// Build the multi-OLT registry: one SNMP pool + usecase + handler per OLT.
	// All OLTs share Redis; each usecase namespaces its cache keys by OLT id
	// (the default OLT keeps unprefixed keys for backward compatibility).
	type oltStack struct {
		olt  config.OLTRuntimeConfig
		uc   usecase.OnuUseCaseInterface
		repo repository.SnmpRepositoryInterface
	}
	var (
		oltRoutes      []oltRoute
		stacks         []oltStack
		defaultUsecase usecase.OnuUseCaseInterface
	)

	for _, olt := range cfg.OLTs {
		snmpConn, connErr := snmp.SetupSnmpConnectionWith(olt.Host, olt.Port, olt.Community)
		if connErr != nil {
			logger.Error("failed to setup snmp connection for olt",
				zap.String("olt_id", olt.ID), zap.String("host", olt.Host), zap.Error(connErr))
			continue // skip this OLT; the others still serve
		}
		snmpRepo := repository.NewPonRepositoryWithConcurrency(snmpConn, olt.MaxConcurrent, olt.UseWalk)
		defer snmpRepo.Close()

		cachePrefix := olt.ID
		if olt.ID == cfg.DefaultOLT {
			cachePrefix = "" // default OLT -> unprefixed keys (back-compat)
		}
		uc := usecase.NewOnuUsecaseForOLT(snmpRepo, redisRepo, cfg.ForOLT(olt), cachePrefix)
		h := handler.NewOnuHandler(uc)

		oltRoutes = append(oltRoutes, oltRoute{id: olt.ID, userID: olt.UserID, handler: h, boardPons: olt.BoardPons})
		stacks = append(stacks, oltStack{olt: olt, uc: uc, repo: snmpRepo})
		if olt.ID == cfg.DefaultOLT {
			defaultUsecase = uc
		}

		logger.Info("olt_registered",
			zap.String("olt_id", olt.ID),
			zap.String("host", olt.Host),
			zap.Ints("boards", olt.Boards),
			zap.Bool("default", olt.ID == cfg.DefaultOLT))
	}

	if len(stacks) == 0 {
		return fmt.Errorf("no OLT could be initialized")
	}
	if defaultUsecase == nil {
		defaultUsecase = stacks[0].uc // default OLT failed to init; fall back to first
	}

	// Pre-warm cache for every OLT in the background.
	if cfg.CacheCfg.PreWarm {
		for _, s := range stacks {
			go s.uc.PreWarmCache(ctx)
		}
	}

	// Start SNMP Trap listener if enabled.
	if cfg.TrapCfg.Enabled {
		trap.SetActionMessages(
			cfg.TrapCfg.ActionCritical,
			cfg.TrapCfg.ActionHigh,
			cfg.TrapCfg.ActionMedium,
			cfg.TrapCfg.ActionLow,
		)

		// Webhook is configured either from env (TRAP_WEBHOOK_*) or live from
		// device-registry (REGISTRY_URL). initWebhook installs the active client
		// and, when REGISTRY_URL is set, refreshes it without a restart.
		initWebhook(trap.WebhookSettings{
			URL:     cfg.TrapCfg.WebhookURL,
			Type:    cfg.TrapCfg.WebhookType,
			ChatID:  cfg.TrapCfg.WebhookChatID,
			Enabled: cfg.TrapCfg.WebhookURL != "",
			Retries: cfg.TrapCfg.WebhookRetries,
			Timeout: cfg.TrapCfg.WebhookTimeout,
		})
		// The pipeline (batcher / power monitor) is created when a webhook is
		// possible now or later — env URL set, or a registry that can enable it.
		webhookPossible := cfg.TrapCfg.WebhookURL != "" || os.Getenv("REGISTRY_URL") != ""

		var batcher *trap.Batcher
		if webhookPossible {
			intervals := trap.BuildIntervals(
				cfg.TrapCfg.CriticalInterval,
				cfg.TrapCfg.HighInterval,
				cfg.TrapCfg.MediumInterval,
				cfg.TrapCfg.LowInterval,
			)
			if len(intervals) > 0 {
				batcher = trap.NewBatcher(trap.ActiveWebhook(), defaultUsecase, intervals)
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

		trapHandler := trap.NewHandler(trap.ActiveWebhook(), batcher, defaultUsecase)
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
		if cfg.TrapCfg.PowerMonitor && webhookPossible {
			powerMonitor := trap.NewPowerMonitor(trap.PowerMonitorConfig{
				Interval:      time.Duration(cfg.TrapCfg.PowerMonitorInterval) * time.Second,
				Cron:          cfg.TrapCfg.PowerMonitorCron,
				Timezone:      cfg.TrapCfg.PowerMonitorTimezone,
				HighThreshold: cfg.TrapCfg.RxPowerHighThreshold,
				LowThreshold:  cfg.TrapCfg.RxPowerLowThreshold,
				Source:        cfg.SnmpCfg.IP,
				Boards:        cfg.Boards,
				PonsPerBoard:  cfg.PonsPerBoard,
			}, defaultUsecase, trap.ActiveWebhook())
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

	// Register dependency probes for /readyz. Redis is critical (5s cache).
	// Each OLT gets its own SNMP probe (30s cache): the default OLT is critical
	// (instance not-ready if it's down), secondary OLTs are non-critical so one
	// unreachable device surfaces as degraded rather than taking the pod down.
	checker := health.NewChecker(2 * time.Second)
	checker.Register("redis", 5*time.Second, func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	})
	for _, s := range stacks {
		repo := s.repo // capture for the closure
		probeName := "snmp_" + s.olt.ID
		// repo.Ping is a synchronous gosnmp call with its own (longer)
		// timeout+retry budget, so run it under the checker's 2s context —
		// otherwise one unreachable OLT pins every readyz call for the full
		// SNMP retry window. An abandoned Ping goroutine just drains on the
		// SNMP timeout and exits.
		probe := func(ctx context.Context) error {
			done := make(chan error, 1)
			go func() { done <- repo.Ping() }()
			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if s.olt.ID == cfg.DefaultOLT {
			checker.Register(probeName, 30*time.Second, probe)
		} else {
			checker.RegisterOptional(probeName, 30*time.Second, probe)
		}
	}

	// Build the api_key -> Principal registry for per-tenant auth (nil when
	// API_USERS is unset; the legacy single API_KEY then applies).
	var principals map[string]reqctx.Principal
	if len(cfg.APIUsers) > 0 {
		principals = make(map[string]reqctx.Principal, len(cfg.APIUsers))
		for key, u := range cfg.APIUsers {
			principals[key] = reqctx.Principal{UserID: u.UserID, Admin: u.IsAdmin()}
		}
	}

	// Initialize router with the per-OLT handlers and health checker.
	a.router = loadRoutesMulti(oltRoutes, cfg.DefaultOLT, checker, principals, cfg.APIKey)

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
