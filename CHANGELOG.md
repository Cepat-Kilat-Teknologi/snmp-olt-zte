# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — SNMP Trap Webhook Notification System
- **Multi-platform webhook formatter** with auto-detection: Discord (rich embeds), Slack (blocks), Telegram (HTML), Generic (raw JSON)
- **4-tier severity system** with per-severity batch intervals and color-coded notifications:
  - CRITICAL (LOS, Offline, AuthFailed, PowerOff, LOSi, LOFi) — 5 min interval, red
  - HIGH (Logging, Synchronization / stuck) — 1 hr interval, orange
  - MEDIUM (HighRxPower, LowRxPower) — 4 hr interval, yellow
  - LOW (DyingGasp) — 8 hr interval, blue
- **Event batcher** with per-severity flush timers, deduplication by Board/PON/ONU, and severity migration handling
- **Double SNMP verification** — cache invalidated + fresh SNMP GET both on trap receive and on batch flush to eliminate false alarms
- **Recovery detection** — ONU that comes back online is removed from batch queue; re-verified at flush time
- **`InvalidateONUCache`** method on usecase for fresh SNMP status checks
- **`internal/trap/batcher.go`** — per-severity batch queue with dedup, re-verify, and RX power threshold checks
- **`internal/trap/formatter*.go`** — WebhookFormatter interface with Format (single) and FormatBatch (batched) for all 4 platforms
- **Batch message format** — per-customer blocks (Full Name, Address, Event, Board/PON/ONU, RX Power, Last Online) with configurable action messages
- **i18n action messages** — `TRAP_ACTION_CRITICAL/HIGH/MEDIUM/LOW` env vars with English defaults, customizable per language
- **Repeat notifications** — `TRAP_*_REPEAT` env vars for periodic re-notification of persistent alerts
- **`docs/SNMP_TRAP_WEBHOOK.md`** — full architecture documentation

### Changed
- **Trap listener** rewritten to parse snmpTrapOID + ONU data OIDs (name, type, description, serial) from real ZTE C320 trap PDUs
- **Trap handler** no longer trusts trap OID for event type — always verifies via fresh SNMP GET
- **PowerMonitor** routes alerts through batcher when available (was direct webhook)
- **OID prefix matching** uses `.` suffix to prevent collisions (e.g. `.1.1` vs `.1.18`)
- **`alertEventTypes`** expanded: Logging and Synchronization now trigger HIGH alerts (was skipped)

### Added — Environment Variables
- `TRAP_WEBHOOK_TYPE` — override auto-detected platform (discord/slack/telegram/generic)
- `TRAP_WEBHOOK_CHAT_ID` — Telegram chat/group ID
- `TRAP_CRITICAL_INTERVAL` — CRITICAL batch flush interval (default 300s)
- `TRAP_HIGH_INTERVAL` — HIGH batch flush interval (default 3600s)
- `TRAP_MEDIUM_INTERVAL` — MEDIUM batch flush interval (default 14400s)
- `TRAP_LOW_INTERVAL` — LOW batch flush interval (default 28800s)
- `TRAP_CRITICAL_REPEAT` — CRITICAL repeat interval in minutes (default 60, 0 = once only)
- `TRAP_HIGH_REPEAT` — HIGH repeat interval in minutes (default 60)
- `TRAP_MEDIUM_REPEAT` — MEDIUM repeat interval in minutes (default 0)
- `TRAP_LOW_REPEAT` — LOW repeat interval in minutes (default 0)
- `TRAP_ACTION_CRITICAL` — configurable action message for CRITICAL (English default)
- `TRAP_ACTION_HIGH` — configurable action message for HIGH (English default)
- `TRAP_ACTION_MEDIUM` — configurable action message for MEDIUM (English default)
- `TRAP_ACTION_LOW` — configurable action message for LOW (English default)

### Changed
- **Field labels** standardized from Indonesian (Nama/Alamat) to English (Name/Address) for i18n consistency
- **Action messages** moved from hardcoded Indonesian to configurable env vars with English defaults

## [3.0.0] - 2026-04-12

**Breaking changes** — this release updates the JSON response format to match the ISP adapter standard. Clients parsing `error.type` or `status:"OK"` must be updated. See migration notes at the bottom of this entry.

### Added — Agent Integration Readiness
- **Response format**: `error_code` field at top level (was nested `error.type`), success `status` changed from `"OK"` to `"success"` to align with ISP adapter standard (`isp-adapter-standard` wiki)
- **request_id** field in error response body (was header-only)
- **Zap logger** via new `pkg/logger` package with `service`/`version`/`module` base fields and `WithRequestID(ctx)` helper
- **`internal/reqctx`** package to share request-ID context key without import cycles
- **`internal/health`** package providing cached dependency probes (`Checker.Register(name, ttl, probe)`)
- **`internal/middleware/audit.go`** — audit log (`audit` sub-logger) for POST/PUT/PATCH/DELETE with masked API key, `duration_ms`, `body_size`, `X-Forwarded-For`-aware client IP
- **`/healthz`** endpoint (alias of `/health`, matches k8s convention)
- **`/readyz`** endpoint with Redis ping (5s TTL cache) + SNMP OLT reachability ping (30s TTL cache); returns 503 + `{"status":"not_ready", "dependencies":{...}}` when any probe fails
- **`SnmpRepositoryInterface.Ping()`** — sysUpTime (1.3.6.1.2.1.1.3.0) reachability check used by readyz
- **`pkg/metrics`** — Prometheus collectors: `http_requests_total`, `http_request_duration_seconds`, `http_requests_in_flight`, `snmp_operations_total`, `snmp_operation_duration_seconds`, `snmp_cache_hits_total`, `snmp_cache_misses_total`
- **`/metrics`** endpoint (unauthenticated, scrapers on-network) with path normalization to avoid cardinality explosion
- **`pkg/logger.SetForTest`** helper for test log capture

### Changed
- **Removed** `github.com/rs/zerolog` dependency entirely (migrated 146 call sites across 13 files)
- **`middleware.Logger()`** now takes no arguments; uses global zap logger from `pkg/logger`; skips `/health`, `/healthz`, `/ready`, `/readyz`, `/metrics`
- **`utils.HandleError` / `ErrorBadRequest` / `ErrorNotFound` / `ErrorInternalServerError`** signature: now takes `(w http.ResponseWriter, r *http.Request, err error)` so request_id can be propagated from context into the error body
- **`utils.ErrorResponse`** struct: flat `error_code` (was nested `error.type`), `data` field (was nested `error.message`), new `request_id` field
- **`loadRoutes`** signature: now takes `(handler, *health.Checker)` (pass nil to skip dependency gating)
- **Handler logs**: `log := logger.WithRequestID(r.Context())` at the top of each handler — removes explicit `zap.String("request_id", ...)` plumbing
- **Duration fields**: renamed `elapsed_time` → `duration_ms` in request logs (snake_case + `_ms` suffix per `isp-logging-standard`)
- **API version exposure**: new `APIVersionHeader` middleware emits `X-API-Version` (v1), `X-App-Version` (semver), and `X-Build-Commit` (short SHA) on every response
- **`internal/buildinfo`** package exposing version/commit/build-time/uptime — wired from `main.go` ldflags
- **`/version`** endpoint returns JSON build metadata for release verification and dashboard panels
- **Dockerfile ldflags fix**: `-X main.Version=` (uppercase, never injected!) → `-X main.version=` / `main.commit=` / `main.buildTime=` (now correctly populated at build time)
- **CI**: passes `APP_COMMIT=${{ github.sha }}` and `APP_BUILD_TIME=<UTC timestamp>` to the Docker build

### Migration from 2.x

If you consume `go-snmp-olt-zte-c320` from another service, update client code:

| Before (2.x)                              | After (3.0.0)                                      |
|-------------------------------------------|----------------------------------------------------|
| `{"code":200,"status":"OK",...}`          | `{"code":200,"status":"success",...}`              |
| `body.error.type`                         | `body.error_code`                                  |
| `body.error.message`                      | `body.data` (string or `{message, details}` object)|
| `body.error.details`                      | `body.data.details` (inside data object)           |
| (request ID only in `X-Request-ID` header) | Still in header AND `body.request_id`             |

No endpoint URLs changed. Only the JSON envelope of responses changed. Health endpoints (`/health`, `/healthz`, `/readyz`) and the new `/metrics` and `/version` endpoints are added without breaking existing `/api/v1/*` paths.

## [2.1.1] - 2026-04-08

### Added
- **Deployment examples** — Docker Compose, Helm chart, and Kustomize manifests in `examples/`
- **Helm chart repository** — published to GitHub Pages via chart-releaser-action
- **GitHub Release** for v2.1.0 with install instructions

### Changed
- **Helm chart version** synced to match app version (both 2.1.0)
- **golangci-lint** upgraded to v2.11.4 for Go 1.26 compatibility
- **Production deployment** consolidated to `examples/docker/` — removed `docker-compose.prod.yaml` from root
- **Taskfile prod tasks** now use `examples/docker/docker-compose.yaml`
- **Docker image name** standardized to `cepatkilatteknologi/snmp-olt-zte-c320` across all files

### Fixed
- **Helm CI** — added Bitnami repo and `skip_existing` for chart-releaser
- **Helm Redis host** — `_helpers.tpl` generated wrong service name (`fullname-redis-master` → `release-redis-master`)
- **Kustomize secret** — `API_KEY` had placeholder value that enabled auth unintentionally, now defaults to empty
- **Docker example** — `SERVER_PORT` hardcoded to 8081 inside container

### Removed
- **`docker-compose.prod.yaml`** — replaced by `examples/docker/docker-compose.yaml`

## [2.1.0] - 2026-04-07

### Added
- **SNMP connection pool** with 4 parallel connections and concurrency semaphore (`SNMP_MAX_CONCURRENT`)
- **SNMP BulkWalk** replacing Walk — 10x fewer SNMP packets per request
- **Batched SNMP Get** — 4 OIDs per request instead of 4 individual calls per ONU
- **Cache pre-warming** — scans all 32 board/pon combos at startup (`CACHE_PREWARM=true`)
- **Configurable Redis TTL** — `REDIS_ONU_INFO_TTL` (30min), `REDIS_ONU_DETAIL_TTL` (15min), `REDIS_EMPTY_ONU_ID_TTL` (5min)
- **Background cache refresh** — async SNMP refresh when Redis TTL drops below 20%
- **Redis caching for ONU serial numbers** — previously uncached, now stored with configurable TTL
- **ONU detail fast fallback** — derives basic info from cached ONU list to avoid SNMP query
- **SNMP Trap listener** for real-time ONU offline detection (LOS, DyingGasp, PowerOff)
- **Webhook notifications** with exponential backoff retry on ONU offline events
- **Trap event enrichment** — webhook payload includes ONU name, address, type, serial number
- **RX Power monitor** with configurable high/low thresholds and webhook alerts
- **Cron scheduling** for power monitor via `POWER_MONITOR_CRON` (e.g., `0 8,12,15,17,0 * * *`)
- **Timezone support** for cron schedule via `POWER_MONITOR_TIMEZONE` (IANA timezone)
- **Dual scheduling mode** — interval + cron can run simultaneously
- **API key authentication** via `X-API-Key` header on all `/api/v1` routes (optional)
- **Health check endpoint** at `GET /health` returning `{"status":"ok"}`
- **Cache clear endpoint** at `DELETE /api/v1/board/{id}/pon/{id}/cache/clear`
- **OpenAPI 3.1 specification** at `api/openapi.yaml` (8 endpoints, full schemas)
- **Structured error responses** with `error.type`, `error.message`, `error.details`
- **Pagination `meta` field** in response (replaces top-level page/limit fields)
- **Request ID correlation** in all handler log entries
- **Env validation** — fail fast on missing `SNMP_HOST` or `SNMP_COMMUNITY`
- **Dynamic OID generation** replacing hardcoded config file (32 board/PON combinations)
- **Singleflight request deduplication** to prevent SNMP storms
- **Docker multi-arch build** support (amd64, arm64, arm/v7)
- **Distroless production image** for minimal attack surface
- **CI/CD pipeline** with GitHub Actions, Trivy scanning, Codecov integration
- **TLS/HTTPS support** via `USE_TLS`, `TLS_CERT_FILE`, `TLS_KEY_FILE`
- **CORS configuration** via environment variables
- **Security headers middleware** (X-Frame-Options, CSP, X-Content-Type-Options)
- **Rate limiting middleware** (100 req/s, burst 200)
- **Request timeout middleware** (90s) and max body size (1MB)
- **Semantic versioning in CI** — Docker image tagged from git tags (`v*.*.*`)
- **k6 load test script** (`scripts/k6-load-test.js`) with 6 endpoint scenarios
- **Trap testing script** (`scripts/test-trap.sh`) with fake SNMP trap sender
- **miniredis** for unit testing `app.Start()` without external Redis
- **Test coverage at 98.6%** with race detector clean

### Changed
- **Go version** upgraded from 1.24 to 1.26
- **Config** from YAML/Viper to environment variables with `godotenv`
- **Board/PON OIDs** from 20KB config file to mathematical formula generation
- **Response format standardized** — `{code, status, data, meta?}` for success, `{code, status, error{type, message, details?}}` for errors
- **Update endpoint** changed from `GET` to `POST /onu_id/update` (REST compliance)
- **Cache clear endpoint** changed to `DELETE /board/{id}/pon/{id}/cache/clear`
- **SNMP repository** refactored from per-request connection to shared connection pool
- **Redis pool defaults** reduced from `MinIdleConns=200, PoolSize=12000` to `MinIdleConns=10, PoolSize=100`
- **Redis TTL defaults increased** — ONU info from 10min to 30min, ONU detail from 2min to 15min
- **Pagination response** moved `page`, `limit`, `page_count`, `total_rows` into `meta` object
- **Pagination endpoint** now uses cached ONU list instead of fresh SNMP Walk per request
- **DeleteCache** now also clears serial number list cache
- **`interface{}` replaced with `any`** across all packages (Go 1.18+ alias)

### Fixed
- **Race condition** on PowerMonitor `alerted` map — added `sync.Mutex` for concurrent access
- **Double-close panic** on PowerMonitor `stopCh` — added `sync.Once` guard
- **Unnecessary SNMP query** for non-existent ONU — skipped when not found in cached list
- **Main goroutine deadlock** — `stopSignal` channel was never written to
- **Wrong error variable logged** in SNMP connection check
- **Double SNMP connection** — duplicate `Connect()` call removed
- **Nil pointer panic** on SNMP close when setup fails
- **Unsafe type assertions** in `getLastOnline`/`getLastOffline`
- **Timezone bug** — uptime was 7 hours too long due to WIB offset
- **SNMP thread safety** — shared connection panics under concurrent load (fixed via pool)
- **Redis `--requirepass` typo** in production docker-compose

### Removed
- **Dead code `GetConfigPath()`** — config migrated to environment variables
- **Viper dependency** — replaced with `os.Getenv()` + `godotenv`
- **Hardcoded Docker image version** — now from git tag
- **Hardcoded TTL constants** — replaced by configurable `CacheConfig`

### Security
- **API key authentication** on all `/api/v1` routes (optional, backward compatible)
- **Redis password** enabled in production docker-compose
- **Redis port** removed from production expose (internal network only)
- **All dependencies updated** to latest — 0 vulnerabilities (govulncheck clean)

### Performance
- **441x throughput** improvement (10.4 → 4,624 req/s under 100 VU load)
- **14,563x faster p(95)** latency (30s → 2.06ms)
- **0.07% error rate** under load (down from 18.49%)
- **Cache pre-warm** eliminates cold-start latency
- **80%+ cache hit rate** with background refresh

## [1.0.0] - 2024-xx-xx

### Added
- Initial release
- SNMP v2c integration with ZTE C320 OLT
- REST API for ONU monitoring (board/PON/ONU queries)
- ONU information: name, type, serial number, RX power, status
- ONU detail: TX power, IP address, description, online/offline times, uptime
- Empty ONU ID detection
- ONU serial number listing
- Redis caching
- Chi HTTP router
- Zerolog structured logging
- Docker support
- Air hot reload for development

[Unreleased]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v3.0.0...HEAD
[3.0.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v2.1.1...v3.0.0
[2.1.1]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v1.0.0...v2.1.0
[1.0.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/releases/tag/v1.0.0
