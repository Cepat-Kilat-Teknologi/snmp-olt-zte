# CLAUDE.md — go-snmp-olt-zte-c320

## Project Overview

Read-oriented HTTP adapter for monitoring ZTE C320 OLT devices via SNMP. Exposes ONU status, optical power, uptime, and serial numbers through a REST API backed by a Redis cache and an SNMP connection pool. Part of the ISP billing ecosystem — the billing-agent will eventually consume this service for bandwidth and ONU status checks.

- **Module path:** `github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320`
- **Go version:** 1.26
- **Entrypoint:** `cmd/api/main.go` → `app.New().Start(ctx)`
- **HTTP framework:** chi v5 (NOT Fiber — differs from freeradius-api and genieacs-relay; see §Framework notes below)
- **Reference implementation:** freeradius-api v1.2.0 (`pkg/httputil`, `pkg/logger`, `pkg/middleware/audit.go`, `pkg/metrics`) — adapted to chi conventions

## Architecture

```
cmd/api/main.go          Entry point (init logger, load .env, start app)
app/                     HTTP server wiring
  ├── app.go             Dependency initialization (Redis, SNMP, usecase, handler, health checker)
  └── routes.go          chi router: middleware chain, health, metrics, /api/v1 group
internal/
  ├── handler/           Chi HTTP handlers
  ├── usecase/           Business logic (SNMP calls + Redis cache)
  ├── repository/        SNMP connection pool + Redis storage
  ├── trap/              Async SNMP trap listener + RX Power cron monitor
  ├── middleware/        chi middleware (auth, cors, logger, audit, validation, etc.)
  ├── utils/             Shared HTTP response helpers + context key re-exports
  ├── errors/            Typed AppError (VALIDATION_ERROR, NOT_FOUND, SNMP_ERROR, …)
  ├── health/            Cached dependency probes for /readyz
  ├── reqctx/            Leaf package: request ID context key (avoids import cycle)
  ├── model/             Domain types
  └── config/            Env-var configuration (no YAML)
pkg/
  ├── logger/            Zap wrapper (WithRequestID, WithModule, SetForTest)
  ├── metrics/           Prometheus collectors + chi middleware
  ├── snmp/              SNMP connection factory
  ├── redis/             Redis client factory
  ├── pagination/        Offset pagination helpers
  └── graceful/          Graceful shutdown helper
```

## Framework notes

go-snmp-olt-zte-c320 uses **chi** while `freeradius-api` and `genieacs-relay` use **Fiber v2**. This is deliberate:

- Migration chi → Fiber is a significant refactor with little upside
- chi is standard-library-compatible (`http.Handler`), simpler to test
- The adapter standard (`isp-adapter-standard` wiki) specifies **JSON response format and HTTP contract**, not the framework

When porting patterns from freeradius-api, adapt Fiber-specific APIs:
- `fiber.Ctx` → `http.ResponseWriter + *http.Request`
- `c.Next()` → `next.ServeHTTP(ww, r)` where `ww` is a `chimw.WrapResponseWriter`
- `c.Locals("request_id")` → `reqctx.RequestIDFromContext(r.Context())`
- `c.IP()` → see `middleware/audit.go`'s `clientIP` helper (X-Forwarded-For aware)

## Module Boundaries & Import Rules

- `internal/reqctx` is a **leaf package**. It must not import any other internal/pkg code. Both `pkg/logger` and `internal/utils` depend on it.
- `pkg/logger` imports `reqctx` (for WithRequestID) and `go.uber.org/zap`. Nothing from `internal/` should live here.
- `internal/utils` re-exports `reqctx.RequestIDKey` as `utils.RequestIDKey` for backwards compatibility with handlers that used the old key — prefer `reqctx` directly in new code.
- `internal/middleware` may import `pkg/logger`, `pkg/metrics`, `internal/reqctx`, `internal/utils`. It must NOT be imported by utils/logger/reqctx.
- `internal/health` must not depend on anything in `internal/` except stdlib types.

## Development Commands

```bash
# Run
task run                  # Run the application (Task runner is preferred over make here)
task air                  # Run with hot reload via Air

# Test
go test ./...             # Full unit test suite (no Docker required)
task test-coverage        # HTML coverage report

# Quality
task lint                 # golangci-lint v2
task vulncheck            # govulncheck
task fmt                  # gofmt
```

Infrastructure requirements (local dev): Redis + a reachable SNMP target (real OLT or simulator).

## Key Patterns

### Response Format (aligned to `isp-adapter-standard`)

- **Success**: `{"code":200, "status":"success", "data":..., "meta":{...}}`
- **Error**: `{"code":400, "status":"Bad Request", "error_code":"VALIDATION_ERROR", "data":..., "request_id":"..."}`
- Error codes: `VALIDATION_ERROR`, `NOT_FOUND`, `SNMP_ERROR`, `REDIS_ERROR`, `CONFIG_ERROR`, `INTERNAL_ERROR`

`utils.HandleError(w, r, err)` is the single entry point. It inspects the error (via `errors.As` on `*apperrors.AppError`) and writes the appropriate status code, error code, and data payload, with `request_id` auto-extracted from the request context.

### Logging (aligned to `isp-logging-standard`)

- **Library**: `go.uber.org/zap` via `pkg/logger` (zerolog removed in v2.2.0)
- **Base fields**: `service`, `version` (always attached)
- **Per-request**: `logger.WithRequestID(r.Context())` at the top of each handler adds `request_id` automatically
- **Naming**: snake_case keys, `_ms` suffix for durations (`duration_ms`)
- **Skipped**: `/health`, `/healthz`, `/ready`, `/readyz`, `/metrics` do NOT emit request logs
- **Audit log**: write operations emit a second entry via the `"audit"` sub-logger for compliance (method, path, status, masked api_key, duration_ms, body_size)
- **Test capture**: `logger.SetForTest(customZap)` returns a restore function

### Health Endpoints

| Endpoint  | Purpose                           | Response                                    |
|-----------|-----------------------------------|---------------------------------------------|
| `/health` | Backwards-compat liveness         | `{"status":"healthy"}` (always 200)         |
| `/healthz`| Kubernetes liveness probe         | `{"status":"healthy"}` (always 200)         |
| `/readyz` | Kubernetes readiness probe        | 200 or 503 + `{"dependencies":{...}}`       |
| `/metrics`| Prometheus scrape endpoint        | OpenMetrics exposition                      |

`/readyz` runs cached probes registered in `app.go`:
- **redis** (5s TTL): `redisClient.Ping`
- **snmp** (30s TTL): `snmpRepo.Ping()` — sysUpTime SNMP Get against the OLT

Successful results are cached per-probe TTL; failures are re-probed on every request so recovery is detected immediately.

### Request ID Propagation

1. `middleware.RequestID` extracts `X-Request-ID` header (or generates an xid)
2. It stores the ID in `context.Context` via `reqctx.RequestIDKey`
3. Handlers read it via `logger.WithRequestID(r.Context())` or `reqctx.RequestIDFromContext(ctx)`
4. `utils.HandleError` extracts it from the request for the error body
5. Response always echoes `X-Request-ID` header (set in the middleware)

### Metrics

Prometheus collectors registered in `pkg/metrics`:
- `http_requests_total{method,path,status}` — path is normalized (`/board/1/pon/8` → `/board/:id/pon/:id`)
- `http_request_duration_seconds{method,path}`
- `http_requests_in_flight` (gauge)
- `snmp_operations_total{operation,status}` — use via `defer metrics.RecordSNMPOperation("get", time.Now(), &err)`
- `snmp_operation_duration_seconds{operation}`
- `snmp_cache_hits_total{type}` / `snmp_cache_misses_total{type}`

The `/metrics` endpoint is unauthenticated — Prometheus scrapers typically run on the same private network.

## Testing Patterns

- **Mocks**: interface mocks in `usecase/onu_test.go` (`mockSnmpRepository`, `mockRedisRepository`) — functional fields (e.g. `GetFunc func(...)`)
- **Log capture**: `pkg/logger.SetForTest(newTestLogger(buf))` + restore closure
- **Health checker**: tests can pass `nil` to `loadRoutes` to skip dependency gating (`/readyz` then always returns 200)
- **Coverage**: aim for >95% (currently 98.6% per wiki)
- **Integration**: `examples/` contains Docker Compose + Helm for end-to-end smoke tests

## Environment Configuration

Loaded from `.env` via godotenv. Key variables:

- `APP_ENV` — `development` / `production` / `staging` (controls zap encoder)
- `SERVER_PORT` — HTTP listen port (default 8081)
- `API_KEY` — optional; when set, all `/api/v1/*` endpoints require `X-API-Key` header
- `SNMP_HOST`, `SNMP_PORT`, `SNMP_COMMUNITY` — OLT connection
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` — cache
- `TRAP_*` — SNMP trap listener configuration (optional)
- Power monitor knobs: `POWER_MONITOR_INTERVAL`, `POWER_MONITOR_CRON`, `RX_POWER_HIGH_THRESHOLD`, `RX_POWER_LOW_THRESHOLD`

Metrics and audit logging are always on; there is no `METRICS_ENABLED` flag (the overhead is negligible and the `/metrics` endpoint is harmless when nothing scrapes it).

## Related docs

- Wiki: `isp-development-requirements` — MUST READ for adapter standards
- Wiki: `isp-adapter-standard` — JSON response format, error codes, HTTP contract
- Wiki: `isp-logging-standard` — zap schema, request ID, snake_case conventions
- `TODO.md` — agent integration readiness checklist (updated as this migration progresses)
- `CHANGELOG.md` — version history
- `api/openapi.yaml` — OpenAPI 3.1 spec
