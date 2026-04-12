# TODO — Agent Integration Readiness

> **STATUS: COMPLETED in v3.0.0 (released 2026-04-12)** — see CHANGELOG.md
>
> All standardization tasks listed below have been delivered. This file is
> kept as a historical record of the work that landed in v3.0.0. New tasks
> should go to GitHub Issues, not back into this file.

## Result

`go-snmp-olt-zte-c320 v3.0.0` is now the second compliant adapter (after
`freeradius-api v1.2.0`) for the [ISP adapter standard](https://github.com/Cepat-Kilat-Teknologi/) — see wiki:
- `[[go-snmp-olt-zte-c320]]` — entity page
- `[[isp-adapter-standard]]` — JSON contract
- `[[isp-logging-standard]]` — zap schema
- `[[isp-development-requirements]]` — full dev requirements

## Delivered (all priorities below shipped in v3.0.0)

### Priority 1 — Response Format ✅
- [x] `error_code` flattened from `error.type` to top level
- [x] `error.message` renamed to `data` (string or `{message,details}` map)
- [x] Success status `"OK"` → `"success"`
- [x] All handlers updated, OpenAPI spec regenerated, unit tests updated

### Priority 2 — Logging ✅
- [x] Migrated `github.com/rs/zerolog` → `go.uber.org/zap` (146 call sites across 13 files)
- [x] Centralized `pkg/logger/logger.go` with `Init`, `WithRequestID`, `WithModule`, `SetForTest`
- [x] Required base fields (`service`, `version`) auto-attached
- [x] `WithRequestID(ctx)` helper for per-request loggers
- [x] ISO8601 UTC timestamps with ms precision
- [x] snake_case keys, `_ms` suffix for durations
- [x] Standard field names: `request_id`, `operation`, `error_code`, `device_id`, etc.
- [x] Skip logging `/health`, `/healthz`, `/ready`, `/readyz`, `/metrics` endpoints
- [x] Audit log middleware for POST/PUT/PATCH/DELETE via `"audit"` named sub-logger

### Priority 3 — Request ID ✅
- [x] `request_id` field added to error response body (was header-only)
- [x] `internal/reqctx` leaf package created to break import cycle
- [x] Context propagated through usecase → repository layer
- [x] X-Request-ID echoed in response header AND error body

### Priority 4 — Health Endpoints ✅
- [x] `/health` kept as backwards-compat alias
- [x] `/healthz` added (k8s liveness probe)
- [x] `/readyz` added with cached dependency probes (Redis 5s TTL, SNMP 30s TTL)
- [x] Returns 503 + `{"status":"not_ready", "dependencies":{...}}` when down
- [x] `/version` endpoint added with build metadata (uses ldflags)

### Priority 5 — Prometheus Metrics ✅
- [x] `pkg/metrics/prometheus.go` created
- [x] HTTP middleware records request counter + duration histogram + in-flight gauge
- [x] SNMP operation metrics: `snmp_operations_total`, `snmp_operation_duration_seconds`
- [x] Cache metrics: `snmp_cache_hits_total`, `snmp_cache_misses_total`
- [x] `/metrics` endpoint mounted (unauthenticated)
- [x] Path normalization to avoid label cardinality explosion

### Priority 6 — Framework Migration (chi → Fiber)
- [x] **Decision: SKIP** — chi works fine, JSON contract is what matters
- Documented in CLAUDE.md §Framework notes for future contributors

### Priority 7 — CI/CD Verification ✅
- [x] golangci-lint v2 — 0 issues
- [x] govulncheck — 0 vulnerabilities
- [x] Multi-arch Docker build (amd64, arm64, arm/v7) verified on Docker Hub
- [x] Dockerfile ldflags fix: `main.Version` (uppercase, never matched) → `main.version` + `main.commit` + `main.buildTime`
- [x] CI passes APP_COMMIT and APP_BUILD_TIME to docker build-push-action

### Priority 8 — Documentation ✅
- [x] CLAUDE.md created (project overview, architecture, import boundaries, patterns)
- [x] OpenAPI spec updated to v3.0.0 with new ErrorResponse schema + /healthz, /readyz, /version, /metrics paths
- [x] CHANGELOG.md [3.0.0] section with migration table
- [x] README.md updated (zerolog → zap in tech stack, v3.0.0 in image tag)
- [x] test.http updated with new endpoints + error envelope examples + X-Request-ID tracing example
- [x] k6-load-test.js updated for v3.0.0 (per-scenario thresholds, contract_check scenario, validateErrorEnvelope helper)
- [x] Wiki entity page `go-snmp-olt-zte-c320` created
- [x] Wiki `isp-adapter-standard` and `isp-logging-standard` compliance tables updated

### Priority 9 — Dependencies & Security ✅
- [x] `go mod tidy` clean
- [x] `govulncheck ./...` — 0 vulnerabilities

## Acceptance Criteria — all met

```
[x] Response: status/data/code format (success="success", error_code top-level)
[x] Logging: zap JSON, standard fields, request_id propagated
[x] Health: /healthz (minimal) + /readyz (with deps check)
[x] Request ID in error response body
[x] Audit log for write operations
[x] Tests: 97.9% coverage (vs 98.6% baseline; gap is logger.Fatal os.Exit + Init cfg.Build error path)
[x] golangci-lint v2: 0 issues
[x] govulncheck: 0 vulnerabilities
[x] OpenAPI spec regenerated
[x] CLAUDE.md created
[x] CI/CD passes end-to-end (PR #38 merged, v3.0.0 tag CI passed)
```

## k6 Load Test Baseline (2026-04-12)

```
3542 requests over 3m30s
16.79 req/s sustained, 8 VUs peak
Cache hit rate: 99.5%
p95 cached:     4ms
p99 cached:     7ms
SNMP cold path: ~7s avg, 12s p95 (OLT bottleneck, not adapter)
Validation:     1ms p95
Health probes:  1ms p95 (sub-millisecond)
Real failure rate: 0
All 12 thresholds passed.
```

## Lessons Learned

See `[[go-snmp-olt-zte-c320]]` wiki page §Lessons Learned for the full list.
Highlights:
- ldflags injection silently broken for entire 2.x series due to capitalisation typo
- Naive context-key placement caused import cycle; fixed by extracting to leaf package
- k6 default `http_req_failed` threshold false-alarms on intentional 4xx
- zap chosen over zerolog despite zerolog being faster (consistency wins)
- Cache pre-warming pays off massively (99.5% hit rate from cold start)
