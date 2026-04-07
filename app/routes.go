package app

import (
	"net/http"
	"os"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func loadRoutes(onuHandler *handler.OnuHandler) http.Handler { // Function to configure and return the HTTP router

	// Initialize logger
	l := log.Output(zerolog.ConsoleWriter{ // Create a new logger with console writer output
		Out: os.Stdout, // Set output to standard out
	})

	// Initialize router using chi
	router := chi.NewRouter() // Create a new instance of Chi router

	// Request ID tracking (should be first for all requests)
	router.Use(middleware.RequestID) // Add unique request ID to each request

	// Security middleware
	router.Use(middleware.SecurityHeaders)                  // Apply security headers middleware
	router.Use(middleware.RequestTimeout(90 * time.Second)) // Set request timeout to the 90s (allows cold cache SNMP queries up to the 60s without a timeout)
	router.Use(middleware.RateLimiter(100, 200))            // Apply rate limiting: 100 requests per second, burst up to 200
	router.Use(middleware.MaxBodySize(1 << 20))             // Limit request body size to 1MB (1 << 20 bytes)

	// Middleware for logging requests
	router.Use(middleware.Logger(l)) // Apply logging middleware using the initialized logger

	// Middleware for CORS (now configurable via environment variables)
	router.Use(middleware.CorsMiddleware()) // Apply Cross-Origin Resource Sharing (CORS) middleware

	// Define a simple root endpoint
	router.Get("/", rootHandler)       // Register the GET handler for the root path "/"
	router.Get("/health", healthHandler) // Register the GET handler for the health check endpoint

	// Create a group for /api/v1/
	apiV1Group := chi.NewRouter() // Create a new router instance for API version 1 group
	apiV1Group.Use(middleware.APIKeyAuth)

	// Define routes for /api/v1/
	apiV1Group.Route("/board", func(r chi.Router) { // Create a route group starting with "/board"
		r.Route("/{board_id}/pon/{pon_id}", func(r chi.Router) { // Nested route group with board_id and pon_id parameters
			// Apply validation middleware for board_id and pon_id
			r.Use(middleware.ValidateBoardPonParams) // specific middleware to validate these parameters

			r.Get("/", onuHandler.GetByBoardIDAndPonID)                // GET /board/{board_id}/pon/{pon_id}/ - Fetch ONUs by board and PON
			r.Delete("/cache/clear", onuHandler.DeleteCache)           // DELETE .../cache/clear - Clear Redis cache for board/pon
			r.Get("/onu_id/empty", onuHandler.GetEmptyOnuID)        // GET .../onu_id/empty - Fetch empty ONU IDs
			r.Get("/onu_id_sn", onuHandler.GetOnuIDAndSerialNumber) // GET .../onu_id_sn - Fetch ONU IDs and serial numbers
			r.Post("/onu_id/update", onuHandler.UpdateEmptyOnuID)   // POST .../onu_id/update - Update empty ONU IDs

			// Routes with onu_id parameter
			r.Route("/onu/{onu_id}", func(r chi.Router) { // Nested route group for specific ONU ID
				r.Use(middleware.ValidateOnuIDParam)             // Validate onu_id parameter
				r.Get("/", onuHandler.GetByBoardIDPonIDAndOnuID) // GET .../onu/{onu_id} - Fetch specific ONU details
			})
		})
	})

	// Define routes for /api/v1/paginate
	apiV1Group.Route("/paginate", func(r chi.Router) { // Create a route group for pagination
		r.Route("/board/{board_id}/pon/{pon_id}", func(r chi.Router) { // Nested route group with board and PON IDs
			r.Use(middleware.ValidateBoardPonParams)                // Apply parameter validation
			r.Get("/", onuHandler.GetByBoardIDAndPonIDWithPaginate) // GET .../ - Fetch paginated ONU list
		})
	})

	// Mount /api/v1/ to root router
	router.Mount("/api/v1", apiV1Group) // Mount the API v1 group to the main router under /api/v1 prefix

	return router // Return the configured router
}

// rootHandler is a simple handler for a root endpoint
func rootHandler(w http.ResponseWriter, _ *http.Request) { // Handler function for the root URL
	w.WriteHeader(http.StatusOK)                                // Set the HTTP status code to 200 OK
	_, _ = w.Write([]byte("Hello, this is the root endpoint!")) // Write a simple welcome message to the response body
}

// healthHandler returns application health status as JSON
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
