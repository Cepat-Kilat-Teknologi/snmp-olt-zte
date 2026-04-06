package app

import (
	"context"
	"net/http"
	"os"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/config"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/repository"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/trap"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/usecase"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/graceful"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/redis"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/snmp"
	rds "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// App represents the main application structure that holds the HTTP router
// and manages the application lifecycle, including dependency initialization
// and server startup.
type App struct { // Define the App struct
	router http.Handler // HTTP handler for routing requests
}

// New creates and returns a new instance of the App with initialized dependencies.
// It prepares the application for startup but does not start the server.
func New() *App { // Factory function to create a new App instance
	return &App{} // Return a pointer to a new App struct
}

// Start initializes the application components, sets up connections to external services
// (Redis and SNMP), and starts the HTTP server. It handles graceful shutdown on context
// cancellation and ensures proper cleanup of resources.
//
// Parameters:
//   - ctx: context.Context for cancellation and timeout propagation
//
// Returns:
//   - error: returns any error that occurs during application startup or shutdown
func (a *App) Start(ctx context.Context) error { // Method to start the application

	// Load configuration from environment variables (no config file needed)
	// Board/PON OID mappings are generated dynamically using mathematical formulas
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
		return err
	}

	// Initialize Redis client
	redisClient := redis.NewRedisClient(cfg) // Create a new Redis client using the configuration

	// Check Redis connection
	err = redisClient.Ping(ctx).Err() // Ping Redis to verify connection
	if err != nil {                   // Check if ping failed
		log.Error().Err(err).Msg("Failed to ping Redis server") // Log the error
	} else { // If ping succeeded
		log.Info().Msg("Redis server successfully connected") // Log success message
	}

	// Close Redis client
	defer func(redisClient *rds.Client) { // Defer closure of a Redis client until Start function exits
		err := redisClient.Close() // Close the Redis connection
		if err != nil {            // Check if closing failed
			log.Error().Err(err).Msg("Failed to close Redis client") // Log the error
		}
	}(redisClient) // Pass redisClient to the deferred function

	// Initialize SNMP connection
	snmpConn, err := snmp.SetupSnmpConnection(cfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup SNMP connection")
		return err
	}
	log.Info().Msg("SNMP server successfully connected")

	// Initialize repository (creates connection pool from seed connection)
	snmpRepo := repository.NewPonRepository(snmpConn)

	// Close all pool connections on shutdown
	defer snmpRepo.Close()
	redisRepo := repository.NewOnuRedisRepo(redisClient)                                        // Create new ONU Redis repository

	// Initialize usecase
	onuUsecase := usecase.NewOnuUsecase(snmpRepo, redisRepo, cfg) // Create new ONU usecase with repositories and config

	// Initialize handler
	onuHandler := handler.NewOnuHandler(onuUsecase) // Create new ONU handler with usecase

	// Start SNMP Trap listener if enabled
	if cfg.TrapCfg.Enabled {
		var webhookClient *trap.WebhookClient
		if cfg.TrapCfg.WebhookURL != "" {
			webhookClient = trap.NewWebhookClient(
				cfg.TrapCfg.WebhookURL,
				cfg.TrapCfg.WebhookRetries,
				cfg.TrapCfg.WebhookTimeout,
			)
		}

		trapHandler := trap.NewHandler(webhookClient, onuUsecase)
		trapListener := trap.NewListener(trap.ListenerConfig{
			Port:      cfg.TrapCfg.Port,
			Community: cfg.TrapCfg.Community,
			OnEvent:   trapHandler.HandleEvent,
		})

		go func() {
			if err := trapListener.Start(); err != nil {
				log.Error().Err(err).Msg("SNMP Trap listener failed")
			}
		}()
		// Wait for listener to be ready
		<-trapListener.Listening()
		log.Info().Uint16("port", cfg.TrapCfg.Port).Msg("SNMP Trap listener started")

		defer trapListener.Close()
	}

	// Initialize router
	a.router = loadRoutes(onuHandler) // Load all routes and middleware, assigning to app router

	// Start server
	addr := os.Getenv("SERVER_PORT") // Read port from environment variable
	if addr == "" {
		addr = "8081" // Default to port 8081 if not set
	}
	server := &http.Server{ // Create a new HTTP server struct
		Addr:    ":" + addr, // Set the address
		Handler: a.router,   // Set the handler (router)
	}

	// Start server at given address
	log.Info().Msgf("Application started at %s", addr) // Log startup message with address

	// Graceful shutdown
	return graceful.Shutdown(ctx, server) // Start a server with graceful shutdown handling
}
