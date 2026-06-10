# Docker Deployment Examples

Docker deployment patterns for `cepatkilatteknologi/snmp-olt-zte:3.0.0`.

The application listens on **TCP 8081** (HTTP API) and **UDP 1620** (SNMP Trap).

## Quick Start

```bash
cp .env.example .env
# Edit .env with your SNMP host, community, and credentials
docker compose up -d
```

---

## 1. Standalone (no Redis)

Run the app container by itself. Caching is disabled without Redis.

```bash
docker run -d \
  --name snmp-olt \
  -p 8081:8081 \
  -p 1620:1620/udp \
  -e SNMP_HOST=192.168.1.100 \
  -e SNMP_COMMUNITY=public \
  -e TRAP_ENABLED=true \
  -e TZ=Asia/Jakarta \
  cepatkilatteknologi/snmp-olt-zte:3.0.0
```

## 2. With Redis (recommended)

Use the provided `docker-compose.yaml` for a production-ready setup with Redis caching, health checks, and restart policies.

```bash
cp .env.example .env
# Edit .env
docker compose up -d
```

This starts:
- **app** -- the SNMP OLT service, connected to Redis
- **redis** -- Redis 7.2 with password authentication and persistent storage

## 3. With TLS

Enable HTTPS by setting TLS environment variables and mounting your certificate files.

```bash
# In .env
USE_TLS=true
TLS_CERT_FILE=/certs/cert.pem
TLS_KEY_FILE=/certs/key.pem
```

Uncomment the TLS volumes section in `docker-compose.yaml`, then place your `cert.pem` and `key.pem` in a local `./certs/` directory:

```bash
mkdir -p certs
cp /path/to/cert.pem certs/
cp /path/to/key.pem certs/
docker compose up -d
```

The app will serve HTTPS on port 8081.

---

## Health Check

The app exposes a `/health` endpoint. The compose file includes a health check that polls this endpoint every 30 seconds.

## Stopping

```bash
docker compose down
# To also remove volumes:
docker compose down -v
```
