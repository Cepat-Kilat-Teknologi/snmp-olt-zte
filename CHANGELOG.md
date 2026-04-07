# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Deployment examples** — Docker Compose, Helm chart, and Kustomize manifests in `examples/`
- **Helm chart repository** — published to GitHub Pages via chart-releaser-action
- **GitHub Release** for v2.1.0 with install instructions

### Changed
- **Helm chart version** synced to match app version (both 2.1.0)
- **golangci-lint** upgraded to v2.11.4 for Go 1.26 compatibility

### Fixed
- **Helm CI** — added Bitnami repo and `skip_existing` for chart-releaser
- **Docker example** — `SERVER_PORT` hardcoded to 8081 inside container

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
- **Trap event enrichment** — webhook payload includes ONU name, alamat, type, serial number
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

[Unreleased]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v2.1.0...HEAD
[2.1.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/compare/v1.0.0...v2.1.0
[1.0.0]: https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/releases/tag/v1.0.0
