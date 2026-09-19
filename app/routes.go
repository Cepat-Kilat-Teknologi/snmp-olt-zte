package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/buildinfo"
	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/health"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/middleware"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/metrics"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/snmp"
	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi"
)

// oltRoute binds an OLT id to its handler and per-slot PON topology (used for
// board/pon validation). id == "" denotes the legacy/default single-OLT
// (bare /board routes only).
type oltRoute struct {
	id        string
	userID    int // owner (tenant) id; enforced by RequireOLTOwner when per-user auth is on
	handler   *handler.OnuHandler
	boardPons map[int]int // physical GPON slot -> PON count (GTGO=8, GTGH=16)
}

// loadRoutes wires a single-OLT router (legacy/default path and tests). `boards`
// is the slot list and `maxPonID` the uniform PON count; nil/0 -> {1,2}, 16.
func loadRoutes(onuHandler *handler.OnuHandler, checker *health.Checker, boards []int, maxPonID int) http.Handler {
	if maxPonID < 1 {
		maxPonID = 16
	}
	boardPons := make(map[int]int, len(boards))
	for _, b := range boards {
		boardPons[b] = maxPonID
	}
	return loadRoutesMulti([]oltRoute{{id: "", handler: onuHandler, boardPons: boardPons}}, "", checker, nil, "")
}

// loadRoutesMulti wires the HTTP router for one or more OLTs. Each OLT with a
// non-empty id is exposed at /api/v1/olt/{id}/board/...; the OLT whose id equals
// defaultOLT is ALSO exposed at the bare /api/v1/board/... paths (back-compat).
// `checker` supplies readiness probes for /readyz; pass nil to disable.
// `users` is the api_key -> Principal registry (nil = per-user auth off, legacy
// single key `legacyKey` applies). When users is set, each OLT is guarded by
// RequireOLTOwner so a tenant only sees its own OLTs.
func loadRoutesMulti(olts []oltRoute, defaultOLT string, checker *health.Checker, users map[string]reqctx.Principal, legacyKey string) http.Handler {
	router := chi.NewRouter()

	// Request ID tracking (must be first so all downstream middleware sees it).
	router.Use(middleware.RequestID)

	// OpenTelemetry trace spans for every HTTP request (noop when tracing is disabled).
	router.Use(otelchi.Middleware("snmp-olt-zte"))

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

	// /api/v1/ route group. Authenticator resolves X-API-Key to a Principal
	// (per-tenant mode) or enforces the legacy single key.
	apiV1Group := chi.NewRouter()
	apiV1Group.Use(middleware.Authenticator(users, legacyKey))

	// Ad-hoc SNMP connection test (target supplied in the body, not the registry)
	// — used by the Devices UI to verify reachability/community before saving.
	apiV1Group.Post("/test", snmpTestHandler)

	// Send a test trap notification using the current webhook config (Settings UI).
	apiV1Group.Post("/webhook/test", webhookTestHandler)
	// Send a custom notification (the BFF calls this on provisioning failures).
	apiV1Group.Post("/webhook/notify", webhookNotifyHandler)

	for _, o := range olts {
		validateBoardPon := middleware.ValidateBoardPonParams(o.boardPons)

		// Per-OLT routes at /olt/{id}/..., guarded by ownership.
		if o.id != "" {
			apiV1Group.Route("/olt/"+o.id, func(r chi.Router) {
				r.Use(middleware.RequireOLTOwner(o.userID))
				mountONURoutes(r, o.handler, validateBoardPon)
			})
		}

		// Default OLT also serves the bare /board and /paginate paths (back-compat),
		// scoped to the default OLT's owner.
		if o.id == defaultOLT {
			apiV1Group.Group(func(r chi.Router) {
				r.Use(middleware.RequireOLTOwner(o.userID))
				mountONURoutes(r, o.handler, validateBoardPon)
			})
		}
	}

	router.Mount("/api/v1", apiV1Group)

	return router
}

// mountONURoutes registers the ONU read/cache endpoints (under /board and
// /paginate) on the supplied router, guarded by the given board/pon validator.
func mountONURoutes(router chi.Router, onuHandler *handler.OnuHandler, validateBoardPon func(http.Handler) http.Handler) {
	// Uplink/card auto-detect (read-only). Not board/pon-scoped — walks standard
	// MIBs, so it lives at the router root: /api/v1/uplinks and
	// /api/v1/olt/{id}/uplinks.
	router.Get("/uplinks", onuHandler.GetUplinkTopology)

	router.Route("/board", func(r chi.Router) {
		r.Route("/{board_id}/pon/{pon_id}", func(r chi.Router) {
			r.Use(validateBoardPon)

			r.Get("/", onuHandler.GetByBoardIDAndPonID)
			r.Delete("/cache/clear", onuHandler.DeleteCache)
			r.Get("/onu_id/empty", onuHandler.GetEmptyOnuID)
			r.Get("/onu_id_sn", onuHandler.GetOnuIDAndSerialNumber)
			r.Post("/onu_id/update", onuHandler.UpdateEmptyOnuID)

			r.Route("/onu/{onu_id}", func(r chi.Router) {
				r.Use(middleware.ValidateOnuIDParam)
				r.Get("/", onuHandler.GetByBoardIDPonIDAndOnuID)
				r.Delete("/cache/clear", onuHandler.InvalidateOnuCache)
			})
		})
	})

	router.Route("/paginate", func(r chi.Router) {
		r.Route("/board/{board_id}/pon/{pon_id}", func(r chi.Router) {
			r.Use(validateBoardPon)
			r.Get("/", onuHandler.GetByBoardIDAndPonIDWithPaginate)
		})
	})
}

// ---------------------------------------------------------------------------
// Dynamic OLT routing — registry-backed resolution at request time
// ---------------------------------------------------------------------------

// oltEntryKeyType is the unexported context key for the resolved OLTEntry.
type oltEntryKeyType struct{}

// oltEntryCtxKey is the context key used to store/retrieve the resolved *OLTEntry.
var oltEntryCtxKey = oltEntryKeyType{}

// OLTEntryFromContext returns the OLTEntry placed in context by resolveOLT /
// resolveDefaultOLT. Panics if absent (only called inside routes that are
// guaranteed to have the middleware).
func OLTEntryFromContext(ctx context.Context) *OLTEntry {
	return ctx.Value(oltEntryCtxKey).(*OLTEntry)
}

// loadRoutesWithRegistry is the dynamic counterpart of loadRoutesMulti. Instead
// of a static []oltRoute it takes the live *OLTRegistry. OLT resolution moves
// from route setup time to request time, so new OLTs are served immediately
// after the poller reconciles them — no restart needed.
func loadRoutesWithRegistry(reg *OLTRegistry, checker *health.Checker, users map[string]reqctx.Principal, legacyKey string) http.Handler {
	router := chi.NewRouter()

	// ── Global middleware (identical to loadRoutesMulti) ──
	router.Use(middleware.RequestID)
	router.Use(otelchi.Middleware("snmp-olt-zte"))
	router.Use(middleware.APIVersionHeader(middleware.DefaultAPIVersionConfig(
		buildinfo.APIVersion, buildinfo.Version, buildinfo.Commit,
	)))
	router.Use(middleware.SecurityHeaders)
	router.Use(middleware.RequestTimeout(90 * time.Second))
	router.Use(middleware.RateLimiter(100, 200))
	router.Use(middleware.MaxBodySize(1 << 20))
	router.Use(metrics.Middleware())
	router.Use(middleware.Logger())
	router.Use(middleware.AuditLog())
	router.Use(middleware.CorsMiddleware())

	// ── Unauthenticated endpoints ──
	router.Get("/", rootHandler)
	router.Get("/health", healthHandler)
	router.Get("/healthz", healthzHandler)
	router.Get("/readyz", makeReadyzHandler(checker))
	router.Get("/version", versionHandler)
	router.Handle("/metrics", metrics.Handler())

	// ── /api/v1 group (authenticated) ──
	apiV1Group := chi.NewRouter()
	apiV1Group.Use(middleware.Authenticator(users, legacyKey))

	apiV1Group.Post("/test", snmpTestHandler)
	apiV1Group.Post("/webhook/test", webhookTestHandler)
	apiV1Group.Post("/webhook/notify", webhookNotifyHandler)

	// Dynamic per-OLT routes: /olt/{olt_id}/board/...
	apiV1Group.Route("/olt/{olt_id}", func(r chi.Router) {
		r.Use(resolveOLT(reg))
		mountDynamicONURoutes(r)
	})

	// Default OLT bare routes: /board/... (back-compat)
	apiV1Group.Group(func(r chi.Router) {
		r.Use(resolveDefaultOLT(reg))
		mountDynamicONURoutes(r)
	})

	router.Mount("/api/v1", apiV1Group)
	return router
}

// resolveOLT is chi middleware that reads the {olt_id} URL param, looks the OLT
// up in the live registry, enforces per-tenant ownership, and stores the
// *OLTEntry in context for downstream handlers and validators.
func resolveOLT(reg *OLTRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			oltID := chi.URLParam(r, "olt_id")
			entry, ok := reg.Get(oltID)
			if !ok {
				utils.ErrorNotFound(w, r, apperrors.NewNotFoundError("olt", oltID))
				return
			}
			// Ownership check — mirrors RequireOLTOwner but reads userID from
			// the live entry instead of a closure-captured constant.
			p, hasPrincipal := reqctx.PrincipalFromContext(r.Context())
			if hasPrincipal && !p.Admin && p.UserID != entry.UserID {
				utils.ErrorNotFound(w, r, apperrors.NewNotFoundError("olt", oltID))
				return
			}
			ctx := context.WithValue(r.Context(), oltEntryCtxKey, entry)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolveDefaultOLT is chi middleware for the bare /board/... routes (backward
// compatibility). It resolves the registry's default OLT and stores it in
// context, applying the same ownership check as resolveOLT.
func resolveDefaultOLT(reg *OLTRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entry, ok := reg.GetDefault()
			if !ok {
				utils.ErrorNotFound(w, r, apperrors.NewNotFoundError("olt", ""))
				return
			}
			p, hasPrincipal := reqctx.PrincipalFromContext(r.Context())
			if hasPrincipal && !p.Admin && p.UserID != entry.UserID {
				utils.ErrorNotFound(w, r, apperrors.NewNotFoundError("olt", ""))
				return
			}
			ctx := context.WithValue(r.Context(), oltEntryCtxKey, entry)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// dynamicValidateBoardPon reads the resolved OLTEntry from context and
// validates the {board_id}/{pon_id} URL params against its BoardPons map.
// It delegates to the standard ValidateBoardPonParams middleware so validation
// logic stays in one place.
func dynamicValidateBoardPon(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := OLTEntryFromContext(r.Context())
		middleware.ValidateBoardPonParams(entry.BoardPons)(next).ServeHTTP(w, r)
	})
}

// dynamicHandler wraps an OnuHandler method expression, resolving the handler
// from the OLTEntry stored in context by resolveOLT / resolveDefaultOLT.
// Usage: dynamicHandler((*handler.OnuHandler).GetByBoardIDAndPonID)
func dynamicHandler(method func(*handler.OnuHandler, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entry := OLTEntryFromContext(r.Context())
		method(entry.Handler, w, r)
	}
}

// mountDynamicONURoutes is the dynamic counterpart of mountONURoutes. The route
// tree is identical, but every handler reads the *OLTEntry from context (set by
// resolveOLT/resolveDefaultOLT) instead of being bound to a specific handler at
// startup. Board/pon validation also resolves dynamically.
func mountDynamicONURoutes(router chi.Router) {
	router.Get("/uplinks", dynamicHandler((*handler.OnuHandler).GetUplinkTopology))

	router.Route("/board", func(r chi.Router) {
		r.Route("/{board_id}/pon/{pon_id}", func(r chi.Router) {
			r.Use(dynamicValidateBoardPon)

			r.Get("/", dynamicHandler((*handler.OnuHandler).GetByBoardIDAndPonID))
			r.Delete("/cache/clear", dynamicHandler((*handler.OnuHandler).DeleteCache))
			r.Get("/onu_id/empty", dynamicHandler((*handler.OnuHandler).GetEmptyOnuID))
			r.Get("/onu_id_sn", dynamicHandler((*handler.OnuHandler).GetOnuIDAndSerialNumber))
			r.Post("/onu_id/update", dynamicHandler((*handler.OnuHandler).UpdateEmptyOnuID))

			r.Route("/onu/{onu_id}", func(r chi.Router) {
				r.Use(middleware.ValidateOnuIDParam)
				r.Get("/", dynamicHandler((*handler.OnuHandler).GetByBoardIDPonIDAndOnuID))
				r.Delete("/cache/clear", dynamicHandler((*handler.OnuHandler).InvalidateOnuCache))
			})
		})
	})

	router.Route("/paginate", func(r chi.Router) {
		r.Route("/board/{board_id}/pon/{pon_id}", func(r chi.Router) {
			r.Use(dynamicValidateBoardPon)
			r.Get("/", dynamicHandler((*handler.OnuHandler).GetByBoardIDAndPonIDWithPaginate))
		})
	})
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

// snmpTestHandler performs a read-only SNMP probe (sysUpTime GET) against a
// target supplied in the request body — NOT the registry — so the Devices UI can
// verify reachability + community before saving. On failure it returns a non-200
// with a `message` field (the BFF surfaces it).
func snmpTestHandler(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Host      string `json:"host"`
		Port      uint16 `json:"port"`
		Community string `json:"community"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"message": "invalid JSON body"})
		return
	}
	if b.Host == "" || b.Community == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"message": "host and community are required"})
		return
	}
	if b.Port == 0 {
		b.Port = 161
	}
	client, err := snmp.SetupSnmpConnectionWith(b.Host, b.Port, b.Community)
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"message": err.Error()})
		return
	}
	defer func() { _ = client.Conn.Close() }()

	res, err := client.Get([]string{"1.3.6.1.2.1.1.3.0"}) // sysUpTime.0
	if err != nil || len(res.Variables) == 0 {
		msg := "no SNMP response (check host/port/community + device SNMP ACL)"
		if err != nil {
			msg = err.Error()
		}
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"message": msg})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"code": 200, "status": "success",
		"data": map[string]any{"ok": true, "sysUpTime": fmt.Sprintf("%v", res.Variables[0].Value)},
	})
}
