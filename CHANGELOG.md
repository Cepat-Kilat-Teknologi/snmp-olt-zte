# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **SNMP Trap listener** for real-time ONU offline detection (LOS, DyingGasp, PowerOff)
- **Webhook notifications** with exponential backoff retry on ONU offline events
- **Trap event enrichment** — webhook payload includes ONU name, alamat, type, serial number
- **RX Power monitor** with configurable high/low thresholds and webhook alerts
- **Cron scheduling** for power monitor via `POWER_MONITOR_CRON` (e.g., `0 8,12,15,17,0 * * *`)
- **Timezone support** for cron schedule via `POWER_MONITOR_TIMEZONE` (IANA timezone)
- **Dual scheduling mode** — interval + cron can run simultaneously
- **SNMP concurrency semaphore** — limits concurrent SNMP operations to prevent OLT saturation (`SNMP_MAX_CONCURRENT`)
- **Cache pre-warming** — scans all 32 board/pon combos at startup (`CACHE_PREWARM=true`)
- **Configurable Redis TTL** — `REDIS_ONU_INFO_TTL`, `REDIS_ONU_DETAIL_TTL`, `REDIS_EMPTY_ONU_ID_TTL`
- **Redis caching for ONU serial numbers** — previously uncached, now stored with configurable TTL
- **ONU detail fast fallback** — derives basic info from cached ONU list to avoid SNMP query
- **Trap testing script** (`scripts/test-trap.sh`) with fake SNMP trap sender and webhook receiver
- **k6 load test script** (`scripts/k6-load-test.js`) with 6 endpoint scenarios
- **Task commands** `test-trap` and `test-trap-webhook` for trap testing

### Changed
- **Redis TTL defaults increased** — ONU info from 10min to 30min, ONU detail from 2min to 15min
- **DeleteCache** now also clears serial number list cache

### Fixed
- **Race condition** on PowerMonitor `alerted` map — added `sync.Mutex` for concurrent access
- **Double-close panic** on PowerMonitor `stopCh` — added `sync.Once` guard
- **Unnecessary SNMP query** for non-existent ONU — now skipped when ONU not found in cached list

### Performance
- **441x throughput** improvement (10.4 → 4,624 req/s under 100 VU load)
- **14,563x faster p(95)** latency (30s → 2.06ms)
- **0.07% error rate** under load (down from 18.49%)
- **Cache pre-warm** eliminates cold-start latency

## [3.0.0] - 2026-04-07

### Added
- **SNMP connection pool** with 4 parallel connections for concurrent OLT queries
- **SNMP BulkWalk** replacing Walk — 10x fewer SNMP packets per request
- **Batched SNMP Get** — 4 OIDs per request instead of 4 individual calls per ONU
- **Background cache refresh** — async SNMP refresh when Redis TTL drops below 20%
- **ONU detail caching** in Redis with 120s TTL (previously uncached)
- **API key authentication** via `X-API-Key` header on all `/api/v1` routes (optional)
- **Health check endpoint** at `GET /health` returning `{"status":"ok"}`
- **Cache clear endpoint** at `DELETE /api/v1/board/{id}/pon/{id}/cache/clear`
- **OpenAPI 3.1 specification** at `api/openapi.yaml` (8 endpoints, full schemas)
- **Structured error responses** with `error.type`, `error.message`, `error.details`
- **Pagination `meta` field** in response (replaces top-level page/limit fields)
- **Request ID correlation** in all handler log entries
- **Env validation** — fail fast on missing `SNMP_HOST` or `SNMP_COMMUNITY`
- **Configurable server port** via `SERVER_PORT` environment variable
- **`.env` file loading** via godotenv (auto-loads on startup)
- **Semantic versioning in CI** — Docker image tagged from git tags (`v*.*.*`)
- **`type=sha` Docker tag** for commit-level image traceability
- **Security headers middleware** (X-Frame-Options, CSP, X-Content-Type-Options)
- **Rate limiting middleware** (100 req/s, burst 200)
- **Request timeout middleware** (90s)
- **Max body size middleware** (1MB)
- **k6 load test** with 5 scenarios (health, onu_list, onu_detail, pagination, mixed_ops)
- **miniredis** for unit testing `app.Start()` without external Redis
- **Test coverage at 99%** (12 of 13 packages at 100%)

### Changed
- **Go version** upgraded from 1.24 to 1.26
- **Response format standardized** — all endpoints use `{code, status, data, meta?}` for success, `{code, status, error{type, message, details?}}` for errors
- **Update endpoint** changed from `GET` to `POST /onu_id/update` (REST compliance)
- **Cache clear endpoint** changed from `DELETE /board/{id}/pon/{id}` to `DELETE /board/{id}/pon/{id}/cache/clear` (clearer intent)
- **SNMP repository** refactored from per-request connection to shared connection pool
- **Redis pool defaults** reduced from `MinIdleConns=200, PoolSize=12000` to `MinIdleConns=10, PoolSize=100`
- **Pagination response** moved `page`, `limit`, `page_count`, `total_rows` into `meta` object
- **Pagination endpoint** now uses cached ONU list instead of fresh SNMP Walk per request
- **`interface{}` replaced with `any`** across all packages (Go 1.18+ alias)
- **Package-level mutable state** removed from `pkg/snmp` and `pkg/redis` (moved to local variables)
- **CI workflow** updated: `setup-go@v5`, `golangci-lint@v6`, `codecov@v5`, `build-push@v6`
- **Dockerfile** accepts `APP_VERSION` build arg for version injection via ldflags

### Fixed
- **Main goroutine deadlock** — `stopSignal` channel was never written to, blocking forever
- **Wrong error variable logged** in SNMP connection check (logged Redis error instead)
- **Double SNMP connection** — `SetupSnmpConnection` already called `Connect()`, duplicate removed
- **Nil pointer panic** on SNMP close when setup fails (added early return)
- **Unsafe type assertions** in `getLastOnline`/`getLastOffline` — now uses comma-ok pattern
- **Timezone bug** — uptime was 7 hours too long due to adding WIB offset to duration
- **Docker Go version** — changed from non-existent `1.25.5` to `1.26`
- **`.dockerignore`** excluded `.air.toml` needed by dev stage (added exception)
- **Redis `--requirepass` typo** in production docker-compose (was `--requires`)
- **SNMP thread safety** — shared connection caused panics under concurrent load (fixed via pool)

### Removed
- **Dead code `GetConfigPath()`** — config migrated to environment variables
- **Viper dependency** — replaced with `os.Getenv()` + `godotenv`
- **Hardcoded Docker image version** (`2.0.0`) — now from git tag
- **`load.go` and `load_test.go`** — unused config path utilities

### Security
- **API key authentication** on all `/api/v1` routes (optional, backward compatible)
- **Redis password** enabled in production docker-compose
- **Redis port** removed from production expose (internal network only)
- **All dependencies updated** to latest — 0 vulnerabilities (govulncheck clean)
- **go-chi/chi** updated from v5.2.3 to v5.2.5 (fixed open redirect CVE in RedirectSlashes)

### Performance
- **32x throughput** improvement (0.21 → 6.75 req/s under load)
- **30x faster avg response** (17.5s → 571ms)
- **80.8% cache hit rate** with background refresh
- **Cache hit latency ~3ms** vs cold cache ~4s

## [2.0.0] - 2025-xx-xx

### Added
- Dynamic OID generation replacing hardcoded config file (32 board/PON combinations)
- Singleflight request deduplication
- Redis caching for ONU list and empty ONU IDs
- Docker multi-arch build support (amd64, arm64, arm/v7)
- Docker Compose for development and production
- Distroless production image
- CI/CD pipeline with GitHub Actions
- Trivy security scanning
- Codecov integration
- TLS/HTTPS support
- CORS configuration via environment variables
- Typed context keys for middleware
- Pagination support

### Changed
- Config from YAML/Viper to environment variables
- Board/PON OIDs from 20KB config file to mathematical formula generation

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
[3.0.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v1.0.0...v3.0.0
[2.0.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/releases/tag/v1.0.0
