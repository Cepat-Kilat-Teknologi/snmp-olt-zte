# Monitoring OLT ZTE C320 & C300 with SNMP
[![ci](https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/actions/workflows/ci.yml/badge.svg)](https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Cepat-Kilat-Teknologi/snmp-olt-zte)](https://goreportcard.com/report/github.com/Cepat-Kilat-Teknologi/snmp-olt-zte)
[![codecov](https://codecov.io/gh/Cepat-Kilat-Teknologi/snmp-olt-zte/graph/badge.svg?token=NB3N7GMUX3)](https://codecov.io/gh/Cepat-Kilat-Teknologi/snmp-olt-zte)
[![Helm Chart](https://img.shields.io/badge/helm-v3.0.0-blue)](https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/releases/tag/v3.0.0)

> **Repository moved (v3.2.0).** This module was previously published as
> `github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320` (C320-only). It is now
> the canonical multi-OLT, C320 **and** C300 SNMP adapter at
> `github.com/Cepat-Kilat-Teknologi/snmp-olt-zte` — the old URL auto-redirects
> on GitHub, but update Go imports to the new module path:
>
> ```bash
> go get github.com/Cepat-Kilat-Teknologi/snmp-olt-zte@v3.2.0
> ```

REST API service for monitoring ZTE **C320 and C300** OLT devices via SNMP protocol, built with Go. Provides real-time ONU information including status, optical power levels, uptime, and serial numbers across all board/PON combinations. C300 and C320 V2.1.0 share the same MIB tree and ifIndex encoding — a single image serves both; only the populated GPON slots differ (configured via `OLT_BOARDS`).

### Tech Stack
* [Go 1.26](https://go.dev/) - Programming language
* [Chi](https://github.com/go-chi/chi/) - Lightweight HTTP router
* [GoSNMP](https://github.com/gosnmp/gosnmp) - SNMP library with BulkWalk support
* [Redis](https://github.com/redis/go-redis/v9) - Caching layer with background refresh
* [robfig/cron](https://github.com/robfig/cron) - Cron scheduling for power monitor
* [Zap](https://github.com/uber-go/zap) - Structured JSON logger (standardized across all ISP adapters)
* [Prometheus client_golang](https://github.com/prometheus/client_golang) - Metrics collection
* [Godotenv](https://github.com/joho/godotenv) - Environment variable loader
* [Miniredis](https://github.com/alicebob/miniredis) - In-memory Redis for testing
* [Docker](https://www.docker.com/) - Containerization with distroless production image
* [Task](https://github.com/go-task/task) - Task runner
* [Air](https://github.com/cosmtrek/air) - Hot reload for development
* [k6](https://k6.io/) - Load testing

### Key Features
- SNMP connection pool (4 connections) with concurrency semaphore (max 5 concurrent ops)
- Redis caching with configurable TTL and cache pre-warming at startup
- Cron-based and interval-based scheduling for RX Power monitor
- API key authentication (optional, via `X-API-Key` header)
- Singleflight request deduplication to prevent SNMP storms
- Batched SNMP Get (4 OIDs per request) and BulkWalk for optimal performance
- Consistent JSON response format with structured error details
- SNMP Trap listener for real-time ONU offline detection with webhook notification
- RX Power monitor with configurable high/low thresholds and cron scheduling
- 99% test coverage

### API Documentation
- OpenAPI 3.1 spec: [`api/openapi.yaml`](api/openapi.yaml)
- REST collection (VS Code REST Client / JetBrains HTTP Client):
  [`test.http`](test.http) — health, per-tenant isolation (`API_USERS`), multi-OLT
  paths, validation/error contract
- k6 load test: [`k6-load-test.js`](k6-load-test.js) (+ stages variant in
  [`scripts/k6-load-test.js`](scripts/k6-load-test.js))

## Getting Started

### Prerequisites
- Go 1.26+
- Docker & Docker Compose
- Task runner (`go install github.com/go-task/task/v3/cmd/task@latest`)
- Access to a ZTE C320 or C300 OLT device (SNMP v2c)

### Quick Start
```shell
# 1. Clone and configure
cp .env.example .env
# Edit .env with your OLT IP, SNMP community, and Redis settings

# 2. Start development (Redis in Docker + App with hot reload)
task dev

# 3. Test
curl http://localhost:8081/health
curl http://localhost:8081/api/v1/board/1/pon/1 | jq
```

### OLT model & slot configuration (C320 / C300)

The OID encoding is identical for ZTE C320 and C300 V2.1.0 — only the physical
slots that hold GPON line cards differ. `OLT_BOARDS` lists them, optionally with
each card's PON-port count as `slot:pons` (a C300 has up to 14 service slots,
each a **GTGO** with 8 PONs or a **GTGH** with 16, mixed freely):

| Device | `OLT_BOARDS` | Notes |
|--------|--------------|-------|
| ZTE C320 | `1,2` (default) | Line cards in slots 1-2 (16 PON each) |
| ZTE C300 | `3:16,5:8` | Slot 3 GTGH (16 PON), slot 5 GTGO (8 PON) |

`board_id` in the API path is the **physical slot**, so a C300 with cards in
slots 3 and 5 is queried as `/api/v1/board/3/pon/1` and `/api/v1/board/5/pon/1`.
A slot not in `OLT_BOARDS`, or a `pon_id` beyond that card's count
(e.g. `/board/5/pon/9` on an 8-port GTGO), returns `400`.

> Tip: to find a C300's GPON slots, walk `ifName` and look for `gpon_<shelf>/<slot>/<port>`
> entries — the `<slot>` values are your `OLT_BOARDS`.

### Multiple OLTs in one instance (`OLTS`)

Set `OLTS` to a JSON array to manage many OLTs — any mix of C320/C300 — from a
single instance:

```bash
OLTS='[
  {"id":"c320","host":"10.0.0.1","port":161,"community":"public","boards":"1,2"},
  {"id":"c300a","host":"10.0.0.2","port":1161,"community":"public","boards":"3:16,5:8"}
]'
DEFAULT_OLT=c320
```

- Each OLT is reachable at `GET /api/v1/olt/{id}/board/{slot}/pon/{pon}/...`
- The `DEFAULT_OLT` is **also** served on the bare `/api/v1/board/...` paths (back-compat)
- Each OLT gets its own SNMP pool and a **namespaced Redis cache** (`olt_<id>_...`) — no collisions
- `/readyz` reports `snmp_<id>` per OLT; one unreachable secondary OLT degrades (not 503)

When `OLTS` is unset, the single-OLT `SNMP_*` / `OLT_BOARDS` settings above apply unchanged.

For many OLTs (or to keep community strings out of env), put the same JSON array in
a file and point `OLTS_FILE` at it — handy for mounting as a Kubernetes Secret:

```bash
OLTS_FILE=/etc/olt/olts.json
```

Inline `OLTS` wins over `OLTS_FILE`. Loading is **fail-fast**: a set-but-unreadable
or empty `OLTS_FILE` aborts startup rather than silently falling back to `SNMP_*`.

### Per-tenant access control (`API_USERS`)

Give each OLT a `user_id` and map API keys to users so a caller only sees the
OLTs they own. Requesting another tenant's OLT returns **404** (not 403 — so a
tenant can't enumerate others' OLTs). A `role:"admin"` key sees every OLT.

```bash
OLTS='[{"id":"c320","user_id":1,"host":"10.0.0.1","community":"public","boards":"1,2"},
       {"id":"c300a","user_id":2,"host":"10.0.0.2","port":1161,"community":"public","boards":"3:16"}]'
API_USERS='[{"user_id":1,"api_key":"keyA"},
            {"user_id":2,"api_key":"keyB"},
            {"user_id":0,"api_key":"adminKey","role":"admin"}]'
```

| Caller (`X-API-Key`) | `GET /api/v1/olt/c320/...` | `GET /api/v1/olt/c300a/...` |
|---|---|---|
| `keyA` (user 1) | `200` | `404` |
| `keyB` (user 2) | `404` | `200` |
| `adminKey` (admin) | `200` | `200` |
| missing / unknown key | `401` | `401` |

An OLT with no `user_id` (0) is unowned → admin-only. When `API_USERS` is unset,
the single `API_KEY` applies unchanged (no per-tenant scoping).

### Docker Compose (Development)
```shell
task up
```

### Docker Compose (Production)
```shell
cd examples/docker
cp .env.example .env
# Edit .env with production values
docker compose up -d
```

### Standalone Docker
```shell
docker network create local-dev && \
docker run -d --name redis-container \
--network local-dev -p 6379:6379 redis:7.2 && \
docker run -d -p 8081:8081 --name snmp-olt-zte \
--network local-dev -e REDIS_HOST=redis-container \
-e REDIS_PORT=6379 -e REDIS_DB=0 \
-e REDIS_MIN_IDLE_CONNECTIONS=10 -e REDIS_POOL_SIZE=100 \
-e REDIS_POOL_TIMEOUT=30 -e SNMP_HOST=x.x.x.x \
-e SNMP_PORT=161 -e SNMP_COMMUNITY=xxxx \
cepatkilatteknologi/snmp-olt-zte:3.0.0
```

## API Endpoints

### Health Check
```shell
curl -sS localhost:8081/health | jq
```
```json
{"status":"healthy"}
```

### Get All ONUs by Board and PON
```shell
curl -sS localhost:8081/api/v1/board/2/pon/7 | jq
```
```json
{
  "code": 200,
  "status": "success",
  "data": [
    {
      "board": 2,
      "pon": 7,
      "onu_id": 3,
      "name": "Customer-001",
      "onu_type": "F670LV7.1",
      "serial_number": "ZTEGC*******",
      "rx_power": "-22.22",
      "status": "Online"
    }
  ]
}
```

### Get Specific ONU Detail
```shell
curl -sS localhost:8081/api/v1/board/2/pon/7/onu/4 | jq
```
```json
{
  "code": 200,
  "status": "success",
  "data": {
    "board": 2,
    "pon": 7,
    "onu_id": 4,
    "name": "Customer-002",
    "description": "Location Description",
    "onu_type": "F670LV7.1",
    "serial_number": "ZTEGC*******",
    "rx_power": "-20.71",
    "tx_power": "2.57",
    "status": "Online",
    "ip_address": "10.x.x.x",
    "last_online": "2024-08-11 10:09:37",
    "last_offline": "2024-08-11 10:08:35",
    "uptime": "5 days 13 hours 10 minutes 50 seconds",
    "last_down_time_duration": "0 days 0 hours 1 minutes 2 seconds",
    "offline_reason": "PowerOff",
    "gpon_optical_distance": "6701"
  }
}
```

### Get Empty ONU IDs
```shell
curl -sS localhost:8081/api/v1/board/2/pon/5/onu_id/empty | jq
```

### Get ONU IDs with Serial Numbers
```shell
curl -sS localhost:8081/api/v1/board/2/pon/7/onu_id_sn | jq

# Force a fresh SNMP read, bypassing + refreshing the serial-list cache
curl -sS 'localhost:8081/api/v1/board/2/pon/7/onu_id_sn?nocache=true' | jq
```

By default this endpoint is served from the `board_<b>_pon_<p>_serial_list`
Redis cache. Pass `?nocache=true` to skip the cache, read live from SNMP, and
repopulate the cache with the fresh result. This exists for pre-write existence
checks (e.g. write-olt-zte verifying an ONU right before a delete/replace), where
a stale cached snapshot right after a provision would otherwise hide the OLT's
real state. The flag is also accepted on the multi-OLT path
`/api/v1/olt/{id}/board/{b}/pon/{p}/onu_id_sn`.

### Update Empty ONU ID Cache
```shell
curl -sS -X POST localhost:8081/api/v1/board/2/pon/5/onu_id/update | jq
```

### Paginated ONU List
```shell
curl -sS 'http://localhost:8081/api/v1/paginate/board/2/pon/8?limit=3&page=2' | jq
```
```json
{
  "code": 200,
  "status": "success",
  "data": [
    {"board": 2, "pon": 8, "onu_id": 4, "name": "Customer-004", "onu_type": "F670LV7.1", "serial_number": "ZTEGC*******", "rx_power": "-19.17", "status": "Online"},
    {"board": 2, "pon": 8, "onu_id": 5, "name": "Customer-005", "onu_type": "F660V6.0", "serial_number": "ZTEGD*******", "rx_power": "-19.54", "status": "Online"}
  ],
  "meta": {
    "page": 2,
    "limit": 3,
    "page_count": 23,
    "total_rows": 69
  }
}
```

### Clear Cache
```shell
curl -sS -X DELETE localhost:8081/api/v1/board/2/pon/7/cache/clear | jq
```

## Authentication

When `API_KEY` environment variable is set, all `/api/v1` routes require the `X-API-Key` header:
```shell
curl -sS -H "X-API-Key: your-api-key" localhost:8081/api/v1/board/2/pon/7 | jq
```

Without a valid API key, the server returns `401 Unauthorized`. If `API_KEY` is not set, authentication is disabled (backward compatible). Health check (`/health`) never requires authentication.

## Response Format

**Success:**
```json
{"code": 200, "status": "success", "data": [...]}
```

**Success with pagination:**
```json
{"code": 200, "status": "success", "data": [...], "meta": {"page": 1, "limit": 10, "page_count": 7, "total_rows": 69}}
```

**Error** (top-level `error_code` + `data` + `request_id`):
```json
{"code": 400, "status": "Bad Request", "error_code": "VALIDATION_ERROR", "data": {"message": "board_id is not a configured GPON slot", "details": {"received": "99"}}, "request_id": "d7dhp4pciod1cuth4i80"}
```

Error codes: `VALIDATION_ERROR`, `NOT_FOUND`, `UNAUTHORIZED`, `SNMP_ERROR`, `REDIS_ERROR`, `CONFIG_ERROR`, `INTERNAL_ERROR`.

| Pagination Parameter | Default | Max |
|---------------------|---------|-----|
| `page` | 1 | - |
| `limit` | 10 | 100 |

## SNMP Trap Listener

Real-time ONU event monitoring via SNMP Trap with multi-platform webhook notifications (Discord, Slack, Telegram). Events are classified into a 4-tier severity system with per-severity batch intervals:

| Severity | Events | Default Interval |
|----------|--------|------------------|
| CRITICAL | LOS, LOSi, LOFi, Offline, AuthFailed, PowerOff | 5 minutes |
| HIGH | Logging, Synchronization (stuck) | 1 hour |
| MEDIUM | HighRxPower, LowRxPower | 4 hours |
| LOW | DyingGasp | 8 hours |

Key features:
- **Deduplication** — each ONU appears only once per batch (keyed by Board/PON/ONU)
- **Recovery detection** — ONUs that come back online before flush are automatically removed
- **Double verification** — SNMP GET on trap receive and again at batch flush to eliminate false alarms

```env
TRAP_ENABLED=true
TRAP_PORT=1620
TRAP_WEBHOOK_URL=https://hooks.example.com/snmp-alerts
TRAP_WEBHOOK_TYPE=discord          # discord|slack|telegram|generic (auto-detected from URL if omitted)
TRAP_WEBHOOK_CHAT_ID=              # Required for Telegram only
```

See [`docs/SNMP_TRAP_WEBHOOK.md`](docs/SNMP_TRAP_WEBHOOK.md) for full architecture documentation.

### RX Power Monitor

Periodic scanning of all ONUs for abnormal optical power levels with webhook alerts.

```env
# Interval only (every 5 minutes)
POWER_MONITOR_ENABLED=true
POWER_MONITOR_INTERVAL=300

# Cron only (specific times)
POWER_MONITOR_INTERVAL=0
POWER_MONITOR_CRON=0 8,12,15,17,0 * * *
POWER_MONITOR_TIMEZONE=Asia/Jakarta

# Both interval + cron
POWER_MONITOR_INTERVAL=300
POWER_MONITOR_CRON=0 8,12,15,17,0 * * *
```

Thresholds: `RX_POWER_HIGH_THRESHOLD=-8.0` (overload), `RX_POWER_LOW_THRESHOLD=-25.0` (weak signal).

### Testing Traps Locally
```shell
# Terminal 1: Start app with trap enabled
# Set in .env: TRAP_ENABLED=true, TRAP_PORT=1620, TRAP_WEBHOOK_URL=http://localhost:9999/test
task dev

# Terminal 2: Run trap tests (sends 6 fake traps + starts webhook receiver)
task test-trap
```

## Deployment Examples

Ready-to-use deployment configurations in [`examples/`](examples/):

| Method | Directory | Install |
|--------|-----------|---------|
| Docker Compose | [`examples/docker/`](examples/docker/) | `docker compose up -d` |
| Helm Chart | [`examples/helm/`](examples/helm/snmp-olt-zte/) | `helm install olt-monitor snmp-olt/snmp-olt-zte` |
| Kustomize | [`examples/kustomize/`](examples/kustomize/) | `kubectl apply -k examples/kustomize/overlays/production/` |

### Helm Repository
```bash
helm repo add snmp-olt https://cepat-kilat-teknologi.github.io/snmp-olt-zte/
helm repo update
helm install olt-monitor snmp-olt/snmp-olt-zte \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

## Architecture

```
cmd/api/          Entry point (loads .env, starts server)
app/              HTTP server setup, routing, middleware chain
config/           Environment-based configuration, OID generation
internal/
  handler/        HTTP handlers with request ID correlation
  middleware/     Auth, CORS, rate limiting, security headers, validation
  usecase/        Business logic, singleflight, caching strategy, cache pre-warming
  repository/     SNMP connection pool, Redis operations
  model/          Data models (ONU info, pagination)
  trap/           SNMP Trap listener, event handler, webhook notifications
  errors/         Typed application errors (validation, SNMP, Redis)
  utils/          OID extractors, power converters, response helpers
pkg/
  graceful/       Graceful shutdown with signal handling
  pagination/     Pagination calculation
  redis/          Redis client factory
  snmp/           SNMP connection setup
api/              OpenAPI 3.1 specification
scripts/          Trap testing tools
examples/         Deployment examples (Docker, Helm, Kustomize)
```

### Performance

Tested with k6 (100 VUs, 1m40s) against a real ZTE C320 OLT:

| Metric | Value |
|--------|-------|
| Throughput | 4,624 req/s |
| p(95) Response Time | 2.06ms |
| p(99) Response Time | 4.88ms |
| Median Response Time | 217µs |
| Iterations (100 VUs) | 51,043 |
| Real Error Rate | 0.07% |
| Test Coverage | 99% |

### Caching Strategy
- **ONU list**: 30 min TTL (configurable via `REDIS_ONU_INFO_TTL`), background refresh at 20% expiry
- **ONU detail**: 15 min TTL (configurable via `REDIS_ONU_DETAIL_TTL`), fallback from cached list
- **ONU serial numbers**: 30 min TTL, cached in Redis (`onu_id_sn`); bypass + refresh with `?nocache=true` for pre-write existence checks that need a live read
- **Empty ONU IDs**: 5 min TTL (configurable via `REDIS_EMPTY_ONU_ID_TTL`)
- **Cache pre-warming**: All 32 board/pon combos scanned at startup (`CACHE_PREWARM=true`)
- **SNMP concurrency limit**: Max 5 concurrent operations (`SNMP_MAX_CONCURRENT=5`)
- **Connection pool**: 4 parallel SNMP connections

## Available Tasks

Run `task --list` or `task help` to see all available tasks.

| Task | Description |
|------|-------------|
| `task dev` | Start development (Redis in Docker + hot reload) |
| `task up` | Start full Docker development environment |
| `task test` | Run all unit tests |
| `task test-coverage` | Run tests with coverage report |
| `task test-html` | Generate HTML coverage report |
| `task load-test` | Run k6 load testing |
| `task prod-up` | Start production containers |
| `task app-build` | Build the app binary |
| `task build-image` | Build Docker image (local) |
| `task push-image` | Build and push multi-arch image to Docker Hub |
| `task clean` | Clean up containers, volumes, artifacts |
| `task test-trap` | Test SNMP Trap listener with fake traps |
| `task test-trap-webhook` | Start webhook receiver for manual testing |

### Load Testing

`task load-test` runs the scenario-based `k6-load-test.js` (health, ONU list,
ONU detail, pagination, mixed ops, contract conformance). A lighter
stages-based variant lives at `scripts/k6-load-test.js`.

```shell
# Run load test (wait for cache pre-warm before testing)
task load-test

# Run with custom base URL and API key
k6 run -e BASE_URL=http://10.0.0.1:8081 -e API_KEY=your-key k6-load-test.js
```

**Multi-OLT load testing** — pass the *same* `OLTS` JSON the server uses and the
test automatically targets the per-OLT paths `/api/v1/olt/{id}/board/...` with
each OLT's valid board/pon ranges (derived from its `boards` spec), and asserts
the per-OLT `snmp_<id>` readiness probes plus an unknown-OLT `404`:

```shell
k6 run -e BASE_URL=http://localhost:8081 \
  -e OLTS='[{"id":"c320","host":"10.0.0.1","community":"public","boards":"1,2"},
            {"id":"c300a","host":"10.0.0.2","port":1161,"community":"public","boards":"3:16,5:8"}]' \
  k6-load-test.js
```

When `OLTS` is omitted the test falls back to the single-OLT bare
`/api/v1/board/...` paths.

When the server runs with per-tenant `API_USERS`, give the load test keys that
can reach the OLTs: either an admin key via `-e API_KEY=<adminKey>` (sees all),
or per-tenant keys via `-e OLT_KEYS='{"c320":"keyA","c300a":"keyB"}'` (each OLT
request is sent with its owner's key).

## License
[MIT License](https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/blob/main/LICENSE)
