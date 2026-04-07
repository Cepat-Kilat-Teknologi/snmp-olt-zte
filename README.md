# Monitoring OLT ZTE C320 with SNMP
[![ci](https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/actions/workflows/ci.yml/badge.svg)](https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320)](https://goreportcard.com/report/github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320)
[![codecov](https://codecov.io/gh/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/graph/badge.svg?token=NB3N7GMUX3)](https://codecov.io/gh/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320)
[![Helm Chart](https://img.shields.io/badge/helm-v2.1.0-blue)](https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/releases/tag/v2.1.0)

REST API service for monitoring ZTE C320 OLT devices via SNMP protocol, built with Go. Provides real-time ONU information including status, optical power levels, uptime, and serial numbers across all board/PON combinations.

### Tech Stack
* [Go 1.26](https://go.dev/) - Programming language
* [Chi](https://github.com/go-chi/chi/) - Lightweight HTTP router
* [GoSNMP](https://github.com/gosnmp/gosnmp) - SNMP library with BulkWalk support
* [Redis](https://github.com/redis/go-redis/v9) - Caching layer with background refresh
* [robfig/cron](https://github.com/robfig/cron) - Cron scheduling for power monitor
* [Zerolog](https://github.com/rs/zerolog) - Structured JSON logger
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
- 98.6% test coverage

### API Documentation
Full OpenAPI 3.1 specification: [`api/openapi.yaml`](api/openapi.yaml)

## Getting Started

### Prerequisites
- Go 1.26+
- Docker & Docker Compose
- Task runner (`go install github.com/go-task/task/v3/cmd/task@latest`)
- Access to a ZTE C320 OLT device (SNMP v2c)

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
docker run -d -p 8081:8081 --name go-snmp-olt-zte-c320 \
--network local-dev -e REDIS_HOST=redis-container \
-e REDIS_PORT=6379 -e REDIS_DB=0 \
-e REDIS_MIN_IDLE_CONNECTIONS=10 -e REDIS_POOL_SIZE=100 \
-e REDIS_POOL_TIMEOUT=30 -e SNMP_HOST=x.x.x.x \
-e SNMP_PORT=161 -e SNMP_COMMUNITY=xxxx \
cepatkilatteknologi/snmp-olt-zte-c320:2.1.0
```

## API Endpoints

### Health Check
```shell
curl -sS localhost:8081/health | jq
```
```json
{"status":"ok"}
```

### Get All ONUs by Board and PON
```shell
curl -sS localhost:8081/api/v1/board/2/pon/7 | jq
```
```json
{
  "code": 200,
  "status": "OK",
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
  "status": "OK",
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
```

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
  "status": "OK",
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
{"code": 200, "status": "OK", "data": [...]}
```

**Success with pagination:**
```json
{"code": 200, "status": "OK", "data": [...], "meta": {"page": 1, "limit": 10, "page_count": 7, "total_rows": 69}}
```

**Error:**
```json
{"code": 400, "status": "Bad Request", "error": {"type": "VALIDATION_ERROR", "message": "board_id must be 1 or 2", "details": {"received": "99"}}}
```

| Pagination Parameter | Default | Max |
|---------------------|---------|-----|
| `page` | 1 | - |
| `limit` | 10 | 100 |

## SNMP Trap Listener

Real-time ONU event detection via SNMP Trap. When an ONU goes offline (LOS, DyingGasp, PowerOff), the trap listener detects it and sends a webhook notification with ONU details.

### Enable Trap Listener
```env
TRAP_ENABLED=true
TRAP_PORT=1620
TRAP_WEBHOOK_URL=https://your-webhook.example.com/olt-alerts
```

### Webhook Payload
```json
{
  "timestamp": "2026-04-07T10:30:45+07:00",
  "source": "192.168.213.174",
  "board": 1,
  "pon": 5,
  "onu_id": 23,
  "event_type": "LOS",
  "status": "offline",
  "name": "Customer-023",
  "description": "Perumahan Graha Ria Blok F No.6",
  "onu_type": "F670LV7.1",
  "serial_number": "ZTEGC12345678"
}
```

Events that trigger webhook: `LOS`, `DyingGasp`, `PowerOff`, `Offline`, `AuthFailed`, `LOSi`, `LOFi`.

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
| Helm Chart | [`examples/helm/`](examples/helm/snmp-olt-zte-c320/) | `helm install olt-monitor snmp-olt/snmp-olt-zte-c320` |
| Kustomize | [`examples/kustomize/`](examples/kustomize/) | `kubectl apply -k examples/kustomize/overlays/production/` |

### Helm Repository
```bash
helm repo add snmp-olt https://cepat-kilat-teknologi.github.io/go-snmp-olt-zte-c320/
helm repo update
helm install olt-monitor snmp-olt/snmp-olt-zte-c320 \
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
| Test Coverage | 98.6% |

### Caching Strategy
- **ONU list**: 30 min TTL (configurable via `REDIS_ONU_INFO_TTL`), background refresh at 20% expiry
- **ONU detail**: 15 min TTL (configurable via `REDIS_ONU_DETAIL_TTL`), fallback from cached list
- **ONU serial numbers**: 30 min TTL, cached in Redis
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

```shell
# Run load test (wait for cache pre-warm before testing)
task load-test

# Run with custom base URL and API key
k6 run -e BASE_URL=http://10.0.0.1:8081 -e API_KEY=your-key scripts/k6-load-test.js
```

## License
[MIT License](https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/blob/main/LICENSE)
