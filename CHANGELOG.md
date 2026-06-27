# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed — GetBulk hang from unset SNMP max-repetitions
- **SNMP connections now always set a non-zero GETBULK max-repetitions.**
  `createConnection` (and the seed builder in `pkg/snmp`) configured `Timeout`,
  `Retries`, and `MaxOids` but never `MaxRepetitions`, leaving it at the gosnmp
  zero value (0). A GetBulk with max-repetitions=0 **hangs** on some ZTE OLTs:
  proven live on a **ZTE C300 V2.1.0**, where `-Cr0` never returns (the request
  times out and retries, yielding empty results after ~45s) while `-Cr10`/`-Cr50`
  return rows in <0.1s. Connections are now built with a sane default of **20**,
  tunable via the new **`SNMP_MAX_REPETITIONS`** env var (mirrors
  `SNMP_TIMEOUT_SECONDS` / `SNMP_RETRIES`; any value ≤ 0 or non-numeric falls
  back to 20 so the hang can never be reintroduced through config). Pool members
  inherit the value from the seed, defaulting to 20 if the seed lacks one.
  This makes GetBulk usable on the C300, removing the need for the per-OLT
  `walk=true` (GetNext) workaround that was previously required just to read it
  (~0.06s reads via GetBulk).

## [3.2.0] - 2026-06-10

First release as the **relocated standalone module**: repository renamed
`go-snmp-olt-zte-c320` → **`snmp-olt-zte`**, module path
`github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320` →
**`github.com/Cepat-Kilat-Teknologi/snmp-olt-zte`** (old GitHub URLs
auto-redirect; update Go imports). Consolidates the multi-OLT / C300 work
developed inside the `olt-provisioning` monorepo since v3.1.0.

### Fixed — readiness probe storm with a down OLT
- **`/readyz` no longer stalls when an OLT is unreachable.** Two compounding
  bugs made a single down OLT pin every readyz call for the full SNMP
  retry window (~12s measured with two unreachable OLTs):
  1. the SNMP probe ignored the health checker's 2s context — `repo.Ping()`
     is a synchronous gosnmp call with its own longer timeout+retry budget.
     The probe now runs Ping in a goroutine and returns on `ctx.Done()`.
  2. only *successful* probe results were TTL-cached; a failure was re-probed
     on EVERY readyz call (each caller paying the probe timeout while holding
     the dependency mutex). Failures now cache for at most 5s (`failureTTL`),
     so recovery is still detected within a typical k8s probe interval without
     the re-probe storm.
  Net effect: worst case one bounded ~2s probe per failure-TTL window instead
  of 12s+ on every call (verified: 12.2s → 4s first call, then sub-ms cached).

### Docs / spec / tooling (relocation release)
- **OpenAPI**: added the missing `/api/v1/uplinks` + `/api/v1/olt/{olt_id}/uplinks`
  path items with `UplinkTopology` / `UplinkCard` / `UplinkPort` schemas and an
  `Uplink` tag; spec title now covers C320/C300 under the new service name.
- **k6**: both load-test scripts now exercise the 3.2.0 surface — uplink
  auto-detect (with cards/ports body shape check) and `?nocache=true`
  forced-fresh serial-list reads.
- **test.http**: added uplinks (default + per-OLT) and cached-vs-nocache
  serial-list requests.
- **README / SECURITY / TERMS / examples / helm**: refreshed for the rename
  (C320/C300 naming, image tags + chart 3.2.0, relocation note); `CLAUDE.md`
  sanitized for the public repo.
- **Tests**: `config/coverage_extra_test.go` split by subject into
  `config_test.go` / `oid_generator_test.go` / `registry_test.go`.

### Added — OLT uplink auto-detect
- **`GET /api/v1/olt/{id}/uplinks`** (and the bare `/api/v1/uplinks`) — SNMP auto-detect of OLT **cards** (ENTITY-MIB `entPhysicalDescr` / `entPhysicalClass` → role: `gpon` / `control` / `uplink` / `power`) and **uplink ethernet ports** (IF-MIB `ifName` / admin / oper / speed → `xgei` = 10G, `gei` = 1G with slot/port). Field-agnostic across ZTE C320 / C300 regardless of card layout or port numbering.

### Added — Forced-fresh serial-list reads (`?nocache=true`)
- **`?nocache=true` query parameter** on `GET /api/v1/board/{board}/pon/{pon}/onu_id_sn` and its multi-OLT variant `GET /api/v1/olt/{olt_id}/board/{board}/pon/{pon}/onu_id_sn`. When set, the handler forces a fresh SNMP read that bypasses the `board_<b>_pon_<p>_serial_list` Redis cache and then refreshes that cache with the result. Consumed by write-olt-zte's pre-write ONU existence/uniqueness checks so a delete/replace issued right after a provision sees the OLT's real state instead of a stale serial-list snapshot. Implemented via a context flag (`usecase.WithNoCache` / `noCacheFromContext`) set from the query param in `GetOnuIDAndSerialNumber`; any value other than `true` keeps the cached behaviour, so the change is backward compatible.
- **OpenAPI**: documented the optional `nocache` boolean query parameter (default `false`) on both `onu_id_sn` operations.

### Fixed
- **Startup no longer crash-loops when device-registry isn't Ready yet** (cold-cluster race). When `REGISTRY_URL` is set, snmp-olt-zte fetches its OLT inventory from device-registry at boot; if the registry pod scheduled later, the fetch failed (`connection refused`) and the process exited `fatal`, so k8s restarted it repeatedly (CrashLoopBackOff / inflated restart counts) until the registry came up. The startup fetch now **retries with exponential backoff** (1s→8s) for `REGISTRY_STARTUP_TIMEOUT` (default **60s**; `"0"` disables → single attempt) and only fails once that window is exhausted. It still **fails fast on a genuinely-misconfigured/unreachable registry** — deliberately *no* fallback to stale/empty static config, because snmp builds its per-OLT SNMP pools once at startup with no live registry refresher (unlike write-olt-zte, which can degrade gracefully since it re-reads the registry per request).
- **`InvalidateONUCache` now also clears the serial-list cache key** (`board_<b>_pon_<p>_serial_list`). It previously deleted only the ONU-detail and board/PON-list keys, leaving the separate `onu_id_sn` serial-list cache stale — so a freshly provisioned/removed ONU kept failing later existence checks until that key expired. The serial list is now invalidated alongside the other two, keeping all three coherent.
- **Healthy ONUs no longer log `parse_last_offline_failed` errors**: `getLastDownDuration` now treats an empty last-offline / last-online timestamp (an ONU that has never gone offline) as a normal empty result and returns an empty duration without logging, instead of attempting to parse the empty string and logging a parse error.
- **Cache writes survive a canceled/timed-out caller**: the ONU-info and serial-list Redis cache writes (`SaveONUInfoList`, `SaveONUSerialList`) now run on a context detached from the request (`context.WithoutCancel` + a short 5s timeout). Previously, when the original caller canceled or timed out (e.g. a write-olt-zte existence check that gave up, or a singleflight leader that abandoned), the cache write aborted with a spurious `context canceled` error and left the cache unpopulated — so the next read re-walked SNMP.

### Added — Per-tenant access control (multi-user)
- **`API_USERS` registry** (JSON array of `{user_id, api_key, role}`): maps each `X-API-Key` to a user and role. Combined with a `user_id` on each OLT in `OLTS`, a caller only sees the OLTs they own — requesting another tenant's OLT returns **404** (not 403, to prevent enumeration of other tenants' OLT ids). `role:"admin"` sees all OLTs (NOC/monitoring); an OLT with `user_id` 0/unset is admin-only.
- **`user_id` field on OLTS entries** marks each OLT's owner.
- Enforced by `middleware.Authenticator` (resolves the key to a `reqctx.Principal`, 401 on unknown/missing) + `middleware.RequireOLTOwner` (per-OLT 404), wired in `app/routes.go`. The default OLT's bare `/board` paths are scoped to its owner too.
- Backward compatible: when `API_USERS` is unset, the legacy single `API_KEY` applies unchanged. JSON inline only for now (a file/Secret variant may follow, like `OLTS_FILE`).
- Verified end-to-end against live C320 + C300: user 1 sees only the C320, user 2 only the C300, admin both, bad/missing keys 401.

### Added — Multi-OLT in a single instance
- **OLT registry via `OLTS`** (JSON array): one instance now manages MANY OLTs — any mix of C320 and C300 — each with its own SNMP pool, slot topology, and Redis cache namespace. Backward compatible: with `OLTS` unset, the legacy `SNMP_*` / `OLT_BOARDS` single-OLT behavior is unchanged.
- **OLT dimension in the API**: `GET /api/v1/olt/{olt_id}/board/{slot}/pon/{pon}/...`. The `DEFAULT_OLT` is also served on the bare `/api/v1/board/...` paths for back-compat.
- **Per-slot PON counts** (`OLT_BOARDS=3:16,5:8`): a C300's 14 service slots can mix GTGO (8 PON) and GTGH (16 PON) cards; OID generation, validation, and cache are all per-slot. `board_id`/`pon_id` are validated against the specific card (e.g. `/board/5/pon/9` is rejected on an 8-port GTGO).
- **Per-OLT Redis cache namespacing** (`olt_<id>_...`) so multiple OLTs sharing one Redis never collide. The default OLT keeps unprefixed keys.
- **Per-OLT readiness probes**: `/readyz` reports `snmp_<id>` per OLT; the default OLT is critical, secondary OLTs are non-critical (one unreachable device → degraded, not not-ready).
- `NewOnuUsecaseForOLT`, `snmp.SetupSnmpConnectionWith`, `health.RegisterOptional`, `config.OLTRuntimeConfig` / `Config.ForOLT`, and `loadRoutesMulti` underpin the registry.
- **OLT registry via `OLTS_FILE`**: the same JSON array can be supplied as a file path instead of inline `OLTS` — preferred for many OLTs and for mounting community strings as a Kubernetes Secret. Inline `OLTS` wins; loading is fail-fast (a set-but-unreadable/empty file aborts startup rather than silently using `SNMP_*`).
- Helm: optional `olt.olts` (raw JSON) to run multi-OLT in one release; `olt.oltsFile` renders the registry into a Secret mounted at `/etc/olt/olts.json` with `OLTS_FILE` wired automatically.
- **OpenAPI**: explicit `/api/v1/olt/{olt_id}/...` path items for every ONU/cache endpoint (new `OltId` parameter), so generated clients see the multi-OLT surface; `/readyz` documents the per-OLT `snmp_<id>` probes. Spec version bumped to 3.2.0.
- **k6**: both load-test scripts are now multi-OLT aware — pass the same `OLTS` JSON and the test targets `/api/v1/olt/{id}/...` with each OLT's valid board/pon ranges, asserts the per-OLT `snmp_<id>` readiness probes, and checks that an unknown OLT id returns 404.
- **Tests**: `OLTS_FILE` resolution covered at unit level (`resolveOLTSJSON`) and end-to-end through `LoadConfig` (file load, inline-wins-over-file precedence, fail-fast on missing file).

### Added — Multi-model support (ZTE C320 + C300)
- **Configurable GPON slot topology** via `OLT_BOARDS` (comma-separated physical slots, default `1,2`) and `OLT_PONS_PER_BOARD` (default `16`). A single image now serves both C320 and C300 — only `OLT_BOARDS` differs (C320 `1,2`; a C300 chassis e.g. `3,5`).
- **Slot-parametric OID encoding** in `config/oid_generator.go` — replaced the hardcoded board-1/board-2 constants with formulas (`onuIDSuffix = 0x11010000 + slot*0x100 + pon`, `onuTypeSuffix = 0x10000000 + slot*0x10000 + pon*0x100`) that reproduce the original C320 values exactly and extend to any slot. C300 V2.1.0 was verified to share the identical MIB tree and ifIndex encoding (live-tested against real hardware).
- **C300 OID generation tests** plus `parseBoards`/`BoardSet`/`OLT_BOARDS` coverage.

### Changed
- `ValidateBoardPonParams` is now a config-driven factory accepting the configured GPON slots; `board_id` is validated against `OLT_BOARDS` instead of a hardcoded `1..2`.
- `InitializeBoardPonMap(boards, ponsPerBoard)` and `ValidateConfig` iterate the configured slot set; cache pre-warm and the RX-power monitor scan the configured slots; the SNMP-trap index decoder is now slot-parametric.
- OpenAPI `board_id` parameter documents the physical GPON slot (C320 1-2, C300 higher).
- Helm chart exposes `olt.boards` / `olt.ponsPerBoard`.

### Fixed
- **401 responses now use the standard error envelope**: authentication failures previously returned an ad-hoc `{"code","status","message"}` body that did not match the documented contract (the OpenAPI even showed `error_code: INTERNAL_ERROR` for 401). They now return `{"code":401,"status":"Unauthorized","error_code":"UNAUTHORIZED","data":...,"request_id":...}` like every other error — new `UNAUTHORIZED` error code added.
- **Infra endpoints bypass the rate limiter**: `/health`, `/healthz`, `/ready`, `/readyz`, and `/metrics` are no longer subject to the global rate limiter. Previously a burst of API traffic could exhaust the shared 100 rps bucket and return 429 to Kubernetes liveness/readiness probes (risking pod restarts) and Prometheus scrapes — more likely now that one instance fronts many OLTs. Verified under a 40-VU k6 load test: `/readyz` stays 200 while API endpoints are still limited.

### Notes
- **Backward compatible**: with `OLT_BOARDS` unset, behaviour is identical to prior C320 releases (slots 1-2, 16 PONs).
- The API rate limiter remains a single global bucket (`RateLimiter(100, 200)` in `app/routes.go`). For high-fan-out multi-OLT deployments consider raising it or moving to per-client limiting — tracked as a follow-up.
- **Multi-tenant exposure caveat**: the unauthenticated ops endpoints `/readyz` (lists `snmp_<id>` per OLT) and `/metrics` (per-OLT path labels when OLT ids are non-numeric) reveal OLT ids — they grant no access (API still enforces 401/404) but do disclose existence. Restrict these endpoints to a trusted network in multi-tenant deployments rather than exposing them publicly.

## [3.1.0] - 2026-04-21

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

### Fixed
- Resolve all 18 golangci-lint issues: exhaustive switch warnings (SeverityUnknown), `WriteString(Sprintf)` → `Fprintf`, American English spelling
- Remove unnecessary type conversions and whitespace

### Improved
- Test coverage raised: repository 96.5%→100%, usecase 98.1%→100%
- Added `jsonMarshal` mock pattern for `SaveONUDetail`/`SaveONUSerialList` marshal error paths
- Added tests for `InvalidateONUCache`, `DeleteCache` edge cases, concurrent repository init

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

If you consume `snmp-olt-zte` from another service, update client code:

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
- **Docker image name** standardized to `cepatkilatteknologi/snmp-olt-zte` across all files

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

[Unreleased]: https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/compare/v3.0.0...HEAD
[3.0.0]: https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/compare/v2.1.1...v3.0.0
[2.1.1]: https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/compare/v1.0.0...v2.1.0
[1.0.0]: https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/releases/tag/v1.0.0
