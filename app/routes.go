package app

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/buildinfo"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/health"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/middleware"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/metrics"
	"github.com/go-chi/chi/v5"
)

// loadRoutes wires the HTTP router with all middleware and endpoints.
// `checker` supplies readiness probes for /readyz; pass nil to disable
// dependency checking (readyz will then unconditionally report ready).
func loadRoutes(onuHandler *handler.OnuHandler, checker *health.Checker) http.Handler {
	router := chi.NewRouter()

	// Request ID tracking (must be first so all downstream middleware sees it).
	router.Use(middleware.RequestID)

	// API/build version headers on every response.
	router.Use(middleware.APIVersionHeader(middleware.DefaultAPIVersionConfig(
		buildinfo.APIVersion,
		buildinfo.Version,
		buildinfo.Commit,
	)))

	// Security middleware.
	router.Use(middleware.SecurityHeaders)
	router.Use(middleware.RequestTimeout(90 * time.Second)) // allows cold-cache SNMP queries
	router.Use(middleware.RateLimiter(100, 200))            // 100 rps, burst 200
	router.Use(middleware.MaxBodySize(1 << 20))             // 1 MB body limit

	// Prometheus metrics middleware (records request counts, durations,
	// in-flight gauge). Skips health and metrics endpoints to avoid label
	// explosion.
	router.Use(metrics.Middleware())

	// Structured request logging via zap (skips /health, /healthz, /ready, /readyz, /metrics).
	router.Use(middleware.Logger())

	// Audit log for write operations (POST/PUT/PATCH/DELETE).
	router.Use(middleware.AuditLog())

	// CORS (configurable via environment variables).
	router.Use(middleware.CorsMiddleware())

	// Root, health, version, metrics endpoints (no auth).
	router.Get("/", rootHandler)
	router.Get("/health", healthHandler)
	router.Get("/healthz", healthzHandler)
	router.Get("/readyz", makeReadyzHandler(checker))
	router.Get("/version", versionHandler)

	// Prometheus metrics endpoint (no auth — scrapers run on-network).
	router.Handle("/metrics", metrics.Handler())

	// /api/v1/ route group.
	apiV1Group := chi.NewRouter()
	apiV1Group.Use(middleware.APIKeyAuth)

	apiV1Group.Route("/board", func(r chi.Router) {
		r.Route("/{board_id}/pon/{pon_id}", func(r chi.Router) {
			r.Use(middleware.ValidateBoardPonParams)

			r.Get("/", onuHandler.GetByBoardIDAndPonID)
			r.Delete("/cache/clear", onuHandler.DeleteCache)
			r.Get("/onu_id/empty", onuHandler.GetEmptyOnuID)
			r.Get("/onu_id_sn", onuHandler.GetOnuIDAndSerialNumber)
			r.Post("/onu_id/update", onuHandler.UpdateEmptyOnuID)

			r.Route("/onu/{onu_id}", func(r chi.Router) {
				r.Use(middleware.ValidateOnuIDParam)
				r.Get("/", onuHandler.GetByBoardIDPonIDAndOnuID)
			})
		})
	})

	apiV1Group.Route("/paginate", func(r chi.Router) {
		r.Route("/board/{board_id}/pon/{pon_id}", func(r chi.Router) {
			r.Use(middleware.ValidateBoardPonParams)
			r.Get("/", onuHandler.GetByBoardIDAndPonIDWithPaginate)
		})
	})

	router.Mount("/api/v1", apiV1Group)

	return router
}

// rootHandler is a simple handler for the root endpoint.
func rootHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Hello, this is the root endpoint!"))
}

// healthHandler returns the application liveness status.
// Kept for backwards compatibility; /healthz is the preferred name.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// versionHandler exposes build metadata (version, commit, build time, uptime)
// for debugging and release verification. Unauthenticated; safe because the
// information is already published in the Docker image labels.
func versionHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSONStatus(w, http.StatusOK, buildinfo.Info())
}

// healthzHandler is the liveness probe endpoint (standard name).
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// makeReadyzHandler builds the readiness handler. When a checker is supplied,
// it runs the registered probes (with cached results) and returns 503 if any
// dependency is down; otherwise it reports the up/down state of each.
// When checker is nil, readyz unconditionally returns ready — useful for
// tests and for deployments that don't want dependency gating.
func makeReadyzHandler(checker *health.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checker == nil {
			writeJSONStatus(w, http.StatusOK, map[string]string{"status": "ready"})
			return
		}

		statuses, healthy := checker.Check(r.Context())

		body := map[string]any{
			"status": "ready",
		}
		deps := make(map[string]any, len(statuses))
		for _, s := range statuses {
			entry := map[string]any{"state": s.State}
			if s.Err != "" {
				entry["error"] = s.Err
			}
			deps[s.Name] = entry
		}
		body["dependencies"] = deps

		code := http.StatusOK
		if !healthy {
			code = http.StatusServiceUnavailable
			body["status"] = "not_ready"
		}
		writeJSONStatus(w, code, body)
	}
}

func writeJSONStatus(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
