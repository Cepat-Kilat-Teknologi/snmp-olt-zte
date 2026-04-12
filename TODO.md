# TODO — Agent Integration Readiness

Standardization tasks agar go-snmp-olt-zte-c320 siap diintegrasikan dengan billing-agent.
Reference format: freeradius-api v1.2.0 + wiki `[[isp-development-requirements]]`.

**Current readiness: 9.2/10** — Most things already in place, minor tweaks needed.

## Priority 1: Response Format — Align with Standard

### 1.1 Rename `error.type` → `error_code` (top-level)
- **File:** `internal/utils/response.go` (lines 22-32)
- **Current:**
  ```json
  {
    "code": 400,
    "status": "Bad Request",
    "error": {
      "type": "VALIDATION_ERROR",
      "message": "...",
      "details": {}
    }
  }
  ```
- **Target:**
  ```json
  {
    "code": 400,
    "status": "Bad Request",
    "error_code": "VALIDATION_ERROR",
    "data": "message or details array"
  }
  ```
- **Tasks:**
  - [ ] Update `ErrorResponse` struct: flatten `error.type` → top-level `error_code`
  - [ ] Rename `error.message` → `data` (string atau array)
  - [ ] Update `error.details` → merge into `data`
  - [ ] Update all handlers yang menggunakan error response
  - [ ] Update OpenAPI spec (`api/openapi.yaml`)
  - [ ] Update unit tests

### 1.2 Success response: "OK" → "success"
- **Current:** `{"code": 200, "status": "OK", "data": {...}}`
- **Target:** `{"code": 200, "status": "success", "data": {...}}`
- **Tasks:**
  - [ ] Update `WebResponse` struct usage in `internal/utils/response.go`
  - [ ] Update all handlers
  - [ ] Update OpenAPI spec
  - [ ] Update tests

## Priority 2: Logging Standardization

Reference: `[[isp-logging-standard]]` (wiki)
**Note:** Currently uses `rs/zerolog`. Standard library is `go.uber.org/zap`.

### 2.1 Migrate from zerolog to zap (consistency across services)
- **Files:** `app/routes.go`, `internal/middleware/logger.go`, all files using `log.Info/Warn/Error`
- **Rationale:** Standard across all ISP adapters is zap (freeradius-api, genieacs-relay)
- **Tasks:**
  - [ ] Replace `github.com/rs/zerolog` with `go.uber.org/zap`
  - [ ] Create `pkg/logger/logger.go` centralized init (pattern dari freeradius-api)
  - [ ] Convert all log calls: `log.Info().Msg(...)` → `logger.Info(...)`
  - [ ] Convert structured fields: `log.Info().Str("key", val)` → `logger.Info("msg", zap.String("key", val))`
  - [ ] Update `go.mod` dependencies

### 2.2 Standardize log schema
- **Tasks:**
  - [ ] Add required base fields: `service`, `version`, `module`
  - [ ] Add `WithRequestID(ctx)` helper function
  - [ ] Ensure ISO8601 UTC timestamps dengan ms precision
  - [ ] Audit key names: ensure snake_case (not camelCase)
  - [ ] Add `_ms` suffix untuk duration fields (e.g., `elapsed_time` → `duration_ms`)
  - [ ] Use standard field names: `request_id`, `operation`, `error_code`, `device_id`
  - [ ] Skip logging `/health`, `/healthz`, `/readyz`, `/metrics` endpoints

### 2.3 Audit log for write operations
- **Current:** Logger middleware logs all requests; no separate audit log
- **Tasks:**
  - [ ] Add audit middleware for POST/DELETE operations
  - [ ] Log: method, path, status, request_id, api_key (masked), duration_ms

## Priority 3: Request ID — Already Good, Minor Tweaks

### 3.1 Include request_id in error responses
- **File:** `internal/utils/response.go`
- **Current:** Request ID in response HEADER only (`X-Request-ID`)
- **Target:** Include `request_id` field in error JSON response body
- **Tasks:**
  - [ ] Add `request_id` field to `ErrorResponse` struct
  - [ ] Extract from context in error response helper
  - [ ] Update tests

### 3.2 Forward X-Request-ID to downstream
- **Current:** Not applicable — SNMP library doesn't support headers
- **Tasks:**
  - [ ] Log request_id di SNMP operation logs untuk tracing
  - [ ] Pass context through usecase → repository → SNMP layer

## Priority 4: Health Endpoints

### 4.1 Rename/alias `/health` → `/healthz` + add `/readyz`
- **Current:** `/health` returns `{"status":"ok"}` (no dependency check)
- **File:** `app/routes.go` (line 89-93)
- **Tasks:**
  - [ ] Keep `/health` for backward compat (alias)
  - [ ] Add `/healthz` with same minimal response: `{"status":"healthy"}`
  - [ ] Add `/readyz` endpoint with dependency checks:
    - Redis connectivity (ping)
    - SNMP OLT reachability (optional, bisa cached)
  - [ ] Response: `{"status":"ready","redis":"connected","snmp":"reachable"}` or 503

### 4.2 Enhanced `/health` with full info
- **Current:** Minimal response
- **Target:** Include version, uptime, etc. (like freeradius-api)
- **Tasks:**
  - [ ] Response format:
    ```json
    {
      "status": "healthy",
      "version": "2.1.1",
      "api_version": "v1",
      "git_commit": "abc1234",
      "build_time": "...",
      "uptime": "48h30m"
    }
    ```
  - [ ] Use ldflags untuk inject version info

## Priority 5: Prometheus Metrics (Optional but Recommended)

### 5.1 Add `/metrics` endpoint
- **Current:** No metrics library
- **Tasks:**
  - [ ] Add `github.com/prometheus/client_golang` dependency
  - [ ] Create `pkg/metrics/prometheus.go` (pattern dari freeradius-api)
  - [ ] Middleware: record HTTP request metrics
  - [ ] Record SNMP operation metrics:
    ```
    snmp_operations_total{operation, status}
    snmp_operation_duration_seconds{operation}
    snmp_cache_hits_total
    snmp_cache_misses_total
    snmp_connection_pool_active
    ```
  - [ ] Expose at `GET /metrics` (unauthenticated)
  - [ ] Config: `METRICS_ENABLED=true`, `METRICS_PATH=/metrics`

## Priority 6: Framework Migration (OPTIONAL — nice to have)

### 6.1 Consider migrating chi → Fiber v2 (standardization)
- **Current:** chi v5.2.5
- **Rationale:** freeradius-api + billing-agent use Fiber; consistency helps
- **Impact:** Significant refactor, may not be worth it
- **Decision:** Leave as chi unless otherwise prioritized — Go interface compatibility makes this low priority
- **Tasks:**
  - [ ] Evaluate cost vs benefit (probably skip)

## Priority 7: CI/CD Verification

### 7.1 Verify GitHub Actions alignment with standard
- **Current:** Has `.github/workflows/ci.yml` (lint + test + build + helm)
- **Tasks:**
  - [ ] Verify golangci-lint v2 config format
  - [ ] Verify govulncheck runs on Go 1.26.2+
  - [ ] Verify multi-arch Docker build (amd64, arm64, arm/v7) ✅ already present
  - [ ] Verify Docker Hub push on tag
  - [ ] Verify secrets in repo environment `ci`
  - [ ] Ensure release workflow triggers on semver tags

## Priority 8: Documentation

### 8.1 Add CLAUDE.md
- **Current:** No CLAUDE.md
- **Tasks:**
  - [ ] Create `CLAUDE.md` with project overview, architecture, conventions
  - [ ] Reference `[[isp-development-requirements]]` wiki

### 8.2 Update README.md
- **Tasks:**
  - [ ] Add "Agent Integration" section explaining compliance with `[[isp-adapter-standard]]`
  - [ ] Document new response format
  - [ ] Document new health endpoints

### 8.3 Regenerate OpenAPI spec
- **Tasks:**
  - [ ] Update `api/openapi.yaml` with new error response format
  - [ ] Verify all endpoints reflect new schema

### 8.4 Update CHANGELOG.md
- [ ] Add section for standardization changes

## Priority 9: Dependencies & Security

### 9.1 Dependency audit
- **Tasks:**
  - [ ] `go mod tidy`
  - [ ] `govulncheck ./...`
  - [ ] `go get -u` untuk latest patch versions

## Acceptance Criteria

All items must pass before integration with billing-agent:

```
[ ] Response: status/data/code format (success="success", error_code top-level)
[ ] Logging: zap JSON, standard fields, request_id propagated
[ ] Health: /healthz (minimal) + /readyz (with deps check)
[ ] Request ID in error response body
[ ] Audit log for write operations
[ ] Tests: >= 95% coverage maintained (currently 98.6%)
[ ] golangci-lint v2: 0 issues
[ ] govulncheck: 0 vulnerabilities
[ ] OpenAPI spec regenerated
[ ] CLAUDE.md created
[ ] CI/CD passes end-to-end
```

## Reference

- freeradius-api v1.2.0 (reference implementation)
- Wiki: `[[isp-development-requirements]]` — full dev requirements
- Wiki: `[[isp-adapter-standard]]` — JSON response + HTTP contract
- Wiki: `[[isp-logging-standard]]` — logging schema
- Billing agent design spec: `~/Projects/billing-agent/docs/specs/2026-04-12-billing-agent-design.md`

## Notes

**go-snmp-olt is the most mature adapter after freeradius-api.** Highlights:
- 98.6% test coverage
- Multi-stage Docker (distroless)
- OpenAPI 3.1 spec
- Connection pooling + singleflight deduplication
- Redis caching with pre-warming
- Multi-arch Docker build in CI
- Helm chart publishing
- Most changes are naming alignment, not structural
