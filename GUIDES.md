# Deployment Guide - Go SNMP OLT ZTE C320

Complete guide for deploying the Go SNMP OLT ZTE C320 service in various environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [Deployment Methods](#deployment-methods)
  - [Docker Compose (Recommended)](#docker-compose-recommended)
  - [Standalone Docker](#standalone-docker)
  - [Kubernetes](#kubernetes)
  - [Systemd Service](#systemd-service)
  - [Binary Deployment](#binary-deployment)
- [Production Checklist](#production-checklist)
- [Monitoring and Logging](#monitoring-and-logging)
- [Scaling](#scaling)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Hardware Requirements

**Minimum:**
- CPU: 2 cores
- RAM: 512 MB
- Storage: 1 GB

**Recommended (Production):**
- CPU: 4+ cores
- RAM: 2 GB+
- Storage: 10 GB+
- Network: Low-latency connection to OLT

### Software Requirements

- Docker 20.10+ and Docker Compose 2.0+ (for Docker deployment)
- Go 1.26+ (for binary deployment)
- Redis 7.0+ (standalone or external)
- Network access to ZTE C320 OLT (SNMP port 161)

### Network Requirements

- **SNMP Access**: UDP port 161 to OLT device
- **Redis Access**: TCP port 6379 (if using external Redis)
- **HTTP/HTTPS**: Port 8081 (or your custom port)
- **TLS Certificates**: If enabling HTTPS

## Configuration

### Environment Variables

Create a `.env` file in the project root:

```bash
# Application Environment
APP_ENV=production

# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8081
SERVER_MODE=release

# SNMP Configuration (REQUIRED)
SNMP_HOST=192.168.1.1        # Your OLT IP address
SNMP_PORT=161                 # Standard SNMP port
SNMP_COMMUNITY=public         # SNMP community string (change in production!)
SNMP_MAX_CONCURRENT=5

# Redis Configuration (REQUIRED)
REDIS_HOST=redis              # Use 'redis' for Docker Compose, or external Redis IP
REDIS_PORT=6379
REDIS_PASSWORD=               # Set strong password in production!
REDIS_DB=0
REDIS_MIN_IDLE_CONNECTIONS=10
REDIS_POOL_SIZE=100
REDIS_POOL_TIMEOUT=30

# Cache TTL Configuration
REDIS_ONU_INFO_TTL=1800           # ONU list cache TTL (30 min)
REDIS_ONU_DETAIL_TTL=900          # ONU detail cache TTL (15 min)
REDIS_EMPTY_ONU_ID_TTL=300        # Empty ONU ID cache TTL (5 min)
CACHE_PREWARM=true                # Pre-warm cache on startup

# TLS/HTTPS Configuration (Production Recommended)
USE_TLS=false                 # Set to 'true' for HTTPS
TLS_CERT_FILE=/path/to/cert.pem
TLS_KEY_FILE=/path/to/key.pem

# CORS Configuration
CORS_ALLOWED_ORIGINS=https://yourdomain.com
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Accept,Authorization,Content-Type,X-API-Key,X-Request-ID
CORS_ALLOW_CREDENTIALS=true
CORS_MAX_AGE=3600

# API Key Authentication (Optional)
# Set to enable API key validation on /api/v1 routes
API_KEY=your-secret-api-key

# GPON slot topology (single OLT). C320 -> "1,2"; a C300 mixes cards per slot,
# e.g. "3:16,5:8" (GTGH=16 PON in slot 3, GTGO=8 PON in slot 5).
OLT_BOARDS=1,2
OLT_PONS_PER_BOARD=16

# Multi-OLT in ONE instance (optional): a JSON array of OLTs, each with its own
# SNMP pool + namespaced cache, served at /api/v1/olt/{id}/... . Overrides
# SNMP_*/OLT_BOARDS. OLTS_FILE can point at a file holding the same JSON (mount
# it as a Secret). DEFAULT_OLT also answers the bare /api/v1/board/... paths.
# OLTS=[{"id":"c320","user_id":1,"host":"10.0.0.1","community":"public","boards":"1,2"},{"id":"c300a","user_id":2,"host":"10.0.0.2","port":1161,"community":"public","boards":"3:16,5:8"}]
# OLTS_FILE=/etc/olt/olts.json
# DEFAULT_OLT=c320

# Per-tenant access control (optional): maps each X-API-Key to a user_id + role.
# Combined with each OLT's user_id, a caller only sees its own OLTs (cross-tenant
# -> 404); role "admin" sees all. Overrides the single API_KEY above.
# API_USERS=[{"user_id":1,"api_key":"keyA"},{"user_id":2,"api_key":"keyB"},{"user_id":0,"api_key":"adminKey","role":"admin"}]
```

> The full, annotated variable reference lives in `.env.example` (includes the
> `TRAP_*`, `POWER_MONITOR_*`, and `RX_POWER_*` knobs).

### Security Hardening

**CRITICAL - Change defaults:**
1. **SNMP Community**: Use a strong, unique community string
2. **Redis Password**: Set `REDIS_PASSWORD` with a strong password
3. **TLS**: Enable HTTPS in production (`USE_TLS=true`)
4. **CORS**: Restrict `CORS_ALLOWED_ORIGINS` to your domain only
5. **API Key**: Set `API_KEY` to protect API endpoints
6. **Trap Webhook**: Use HTTPS for `TRAP_WEBHOOK_URL` in production

### SNMP Trap Configuration

To enable real-time ONU event monitoring with batched webhook notifications:

```bash
# SNMP Trap Configuration
TRAP_ENABLED=true              # Enable trap listener
TRAP_PORT=1620                 # UDP port for trap listener (default: 1620)
TRAP_COMMUNITY=public          # SNMP community for trap validation
TRAP_WEBHOOK_URL=https://hooks.example.com/snmp-alerts
TRAP_WEBHOOK_TYPE=              # discord|slack|telegram|generic (auto-detected from URL if omitted)
TRAP_WEBHOOK_CHAT_ID=           # Required for Telegram only
TRAP_WEBHOOK_RETRIES=3         # Max retry attempts for webhook
TRAP_WEBHOOK_TIMEOUT=10        # Webhook timeout in seconds

# Per-severity batch intervals (seconds)
TRAP_CRITICAL_INTERVAL=300     # CRITICAL (LOS, Offline, AuthFailed, PowerOff, LOSi, LOFi): 5 min
TRAP_HIGH_INTERVAL=3600        # HIGH (Logging, Synchronization stuck): 1 hr
TRAP_MEDIUM_INTERVAL=14400     # MEDIUM (HighRxPower, LowRxPower): 4 hr
TRAP_LOW_INTERVAL=28800        # LOW (DyingGasp): 8 hr

# RX Power Monitor (requires TRAP_ENABLED=true and TRAP_WEBHOOK_URL)
POWER_MONITOR_ENABLED=true
POWER_MONITOR_INTERVAL=300        # Scan interval in seconds (0 = disable)
POWER_MONITOR_CRON=0 8,12,15,17,0 * * *  # Cron schedule (empty = disable)
POWER_MONITOR_TIMEZONE=Asia/Jakarta       # IANA timezone for cron
RX_POWER_HIGH_THRESHOLD=-8.0      # Overload threshold (dBm)
RX_POWER_LOW_THRESHOLD=-25.0      # Weak signal threshold (dBm)
```

**OLT Configuration Required:**
```
snmp-server host <APP_SERVER_IP> version 2c <COMMUNITY> udp-port <TRAP_PORT>
```

The trap listener classifies events into 4 severity tiers, batches them per severity, deduplicates per ONU, and re-verifies via SNMP before sending. See [`docs/SNMP_TRAP_WEBHOOK.md`](docs/SNMP_TRAP_WEBHOOK.md) for full details.

## Deployment Methods

### Docker Compose (Recommended)

#### Production Deployment with Docker Compose

1. **Clone the repository:**
```bash
git clone https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte.git
cd snmp-olt-zte
```

2. **Create production environment file:**
```bash
cd examples/docker
cp .env.example .env
nano .env  # Edit with production values
```

3. **Start services:**
```bash
docker compose up -d
```

5. **Verify deployment:**
```bash
# Check container status
docker-compose ps

# Check logs
docker-compose logs -f

# Test API
curl http://localhost:8081/
```

#### Docker Compose with External Redis

If you have an external Redis instance:

**docker-compose.override.yaml:**
```yaml
version: '3.8'

services:
  app:
    image: cepatkilatteknologi/snmp-olt-zte:latest
    environment:
      - REDIS_HOST=external.redis.host  # External Redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
    ports:
      - "8081:8081"
    restart: unless-stopped
```

### Standalone Docker

#### With Internal Redis (Recommended for Single Server)

```bash
# Create Docker network
docker network create olt-network

# Start Redis
docker run -d \
  --name redis-olt \
  --network olt-network \
  --restart unless-stopped \
  -v redis-data:/data \
  redis:7.2 redis-server --appendonly yes --requirepass YOUR_REDIS_PASSWORD

# Start Go SNMP OLT
docker run -d \
  --name snmp-olt-zte \
  --network olt-network \
  --restart unless-stopped \
  -p 8081:8081 \
  -e APP_ENV=production \
  -e SNMP_HOST=192.168.1.1 \
  -e SNMP_PORT=161 \
  -e SNMP_COMMUNITY=your_snmp_community \
  -e REDIS_HOST=redis-olt \
  -e REDIS_PORT=6379 \
  -e REDIS_PASSWORD=YOUR_REDIS_PASSWORD \
  -e REDIS_DB=0 \
  -e REDIS_MIN_IDLE_CONNECTIONS=10 \
  -e REDIS_POOL_SIZE=100 \
  -e REDIS_POOL_TIMEOUT=30 \
  cepatkilatteknologi/snmp-olt-zte:latest

# Verify
docker logs -f snmp-olt-zte
```

#### With External Redis

```bash
docker run -d \
  --name snmp-olt-zte \
  --restart unless-stopped \
  -p 8081:8081 \
  -e APP_ENV=production \
  -e SNMP_HOST=192.168.1.1 \
  -e SNMP_PORT=161 \
  -e SNMP_COMMUNITY=your_snmp_community \
  -e REDIS_HOST=external.redis.host \
  -e REDIS_PORT=6379 \
  -e REDIS_PASSWORD=YOUR_REDIS_PASSWORD \
  cepatkilatteknologi/snmp-olt-zte:latest
```

#### With HTTPS/TLS

```bash
# Mount TLS certificates
docker run -d \
  --name snmp-olt-zte \
  --restart unless-stopped \
  -p 443:8081 \
  -v /path/to/certs:/certs:ro \
  -e APP_ENV=production \
  -e USE_TLS=true \
  -e TLS_CERT_FILE=/certs/fullchain.pem \
  -e TLS_KEY_FILE=/certs/privkey.pem \
  -e SNMP_HOST=192.168.1.1 \
  -e SNMP_COMMUNITY=your_snmp_community \
  -e REDIS_HOST=redis-olt \
  -e REDIS_PORT=6379 \
  cepatkilatteknologi/snmp-olt-zte:latest
```

### Kubernetes

#### Kubernetes Deployment

**1. Create namespace:**
```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: olt-monitoring
```

**2. Create secrets:**
```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: olt-secrets
  namespace: olt-monitoring
type: Opaque
stringData:
  snmp-community: "your_snmp_community"
  redis-password: "your_redis_password"
```

**3. Deploy Redis:**
```yaml
# redis-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: olt-monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7.2
        ports:
        - containerPort: 6379
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: olt-secrets
              key: redis-password
        command:
          - redis-server
          - --requirepass
          - $(REDIS_PASSWORD)
        volumeMounts:
        - name: redis-data
          mountPath: /data
      volumes:
      - name: redis-data
        persistentVolumeClaim:
          claimName: redis-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: olt-monitoring
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
```

**4. Deploy application:**
```yaml
# app-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: snmp-olt-zte
  namespace: olt-monitoring
spec:
  replicas: 3
  selector:
    matchLabels:
      app: snmp-olt-zte
  template:
    metadata:
      labels:
        app: snmp-olt-zte
    spec:
      containers:
      - name: snmp-olt-zte
        image: cepatkilatteknologi/snmp-olt-zte:latest
        ports:
        - containerPort: 8081
        env:
        - name: APP_ENV
          value: "production"
        - name: SNMP_HOST
          value: "192.168.1.1"
        - name: SNMP_PORT
          value: "161"
        - name: SNMP_COMMUNITY
          valueFrom:
            secretKeyRef:
              name: olt-secrets
              key: snmp-community
        - name: REDIS_HOST
          value: "redis"
        - name: REDIS_PORT
          value: "6379"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: olt-secrets
              key: redis-password
        - name: REDIS_DB
          value: "0"
        - name: REDIS_MIN_IDLE_CONNECTIONS
          value: "10"
        - name: REDIS_POOL_SIZE
          value: "100"
        - name: REDIS_POOL_TIMEOUT
          value: "30"
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: snmp-olt-zte
  namespace: olt-monitoring
spec:
  selector:
    app: snmp-olt-zte
  ports:
  - port: 80
    targetPort: 8081
  type: LoadBalancer
```

**5. Deploy:**
```bash
kubectl apply -f namespace.yaml
kubectl apply -f secrets.yaml
kubectl apply -f redis-deployment.yaml
kubectl apply -f app-deployment.yaml

# Check status
kubectl get pods -n olt-monitoring
kubectl logs -f deployment/snmp-olt-zte -n olt-monitoring
```

### Systemd Service

For bare-metal Linux servers:

**1. Build binary:**
```bash
task app-build
# Binary will be in ./bin/api
```

**2. Create service user:**
```bash
sudo useradd -r -s /bin/false olt-service
```

**3. Install binary:**
```bash
sudo mkdir -p /opt/snmp-olt-zte
sudo cp ./bin/api /opt/snmp-olt-zte/
sudo cp .env.production /opt/snmp-olt-zte/.env
sudo chown -R olt-service:olt-service /opt/snmp-olt-zte
sudo chmod 755 /opt/snmp-olt-zte/api
```

**4. Create systemd service:**
```bash
sudo nano /etc/systemd/system/snmp-olt-zte.service
```

```ini
[Unit]
Description=Go SNMP OLT ZTE C320 Monitoring Service
After=network.target redis.service

[Service]
Type=simple
User=olt-service
Group=olt-service
WorkingDirectory=/opt/snmp-olt-zte
EnvironmentFile=/opt/snmp-olt-zte/.env
ExecStart=/opt/snmp-olt-zte/api
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=snmp-olt-zte

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/snmp-olt-zte

[Install]
WantedBy=multi-user.target
```

**5. Enable and start:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable snmp-olt-zte
sudo systemctl start snmp-olt-zte

# Check status
sudo systemctl status snmp-olt-zte

# View logs
sudo journalctl -u snmp-olt-zte -f
```

### Binary Deployment

For manual deployment without containers:

**1. Build:**
```bash
# Clone and build
git clone https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte.git
cd snmp-olt-zte
task app-build
```

**2. Configure:**
```bash
cp .env.example .env
nano .env  # Edit configuration
```

**3. Run:**
```bash
./bin/api
```

**4. Run in background (with nohup):**
```bash
nohup ./bin/api > logs/app.log 2>&1 &
```

## Deployment Examples

Pre-built deployment configurations are available in `examples/`:

### Docker Compose
```bash
cd examples/docker
cp .env.example .env
# Edit .env with your values
docker compose up -d
```

### Helm Chart
```bash
# From Helm repository (recommended)
helm repo add snmp-olt https://cepat-kilat-teknologi.github.io/snmp-olt-zte/
helm install olt-monitor snmp-olt/snmp-olt-zte \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community

# From source
helm install olt-monitor examples/helm/snmp-olt-zte \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

### Kustomize
```bash
# Edit base config
vi examples/kustomize/base/configmap.yaml
vi examples/kustomize/base/secret.yaml

# Deploy production overlay
kubectl apply -k examples/kustomize/overlays/production/

# Deploy development overlay
kubectl apply -k examples/kustomize/overlays/development/
```

See [`examples/README.md`](examples/README.md) for full documentation.

## Production Checklist

### Before Deployment

- [ ] Change default SNMP community string
- [ ] Set strong Redis password
- [ ] Enable TLS/HTTPS
- [ ] Configure CORS for specific domains only
- [ ] Review and adjust Redis pool settings
- [ ] Set up monitoring and alerting
- [ ] Configure log rotation
- [ ] Test failover scenarios
- [ ] Document deployment architecture
- [ ] Create backup strategy

### After Deployment

- [ ] Verify API endpoint accessibility
- [ ] Test SNMP connectivity to OLT
- [ ] Verify Redis connection
- [ ] Check log output
- [ ] Run load tests
- [ ] Set up health check monitoring
- [ ] Configure firewall rules
- [ ] Enable log aggregation
- [ ] Set up alerts for errors
- [ ] Document runbook for incidents

## Monitoring and Logging

### Health Checks

```bash
# Basic health check
curl http://localhost:8081/

# API test
curl http://localhost:8081/api/v1/board/1/pon/1
```

### Logs

**Docker:**
```bash
docker logs -f snmp-olt-zte
docker logs --tail 100 snmp-olt-zte
```

**Systemd:**
```bash
sudo journalctl -u snmp-olt-zte -f
sudo journalctl -u snmp-olt-zte --since "1 hour ago"
```

### Metrics to Monitor

- **HTTP Response Time**: p50, p95, p99
- **SNMP Query Success Rate**: Should be >95%
- **Redis Cache Hit Rate**: Should be >95% (with cache pre-warm)
- **Error Rate**: Should be <0.1% (with cache pre-warm)
- **Memory Usage**: Monitor for leaks
- **CPU Usage**: Should be <60% under normal load
- **Goroutines**: Monitor for leaks

## Scaling

### Horizontal Scaling

The service is stateless and can be scaled horizontally:

**Docker Compose:**
```bash
docker-compose up -d --scale app=3
```

**Kubernetes:**
```bash
kubectl scale deployment snmp-olt-zte --replicas=5 -n olt-monitoring
```

### Load Balancing

Use Nginx or HAProxy:

```nginx
upstream olt_backend {
    least_conn;
    server olt-1:8081;
    server olt-2:8081;
    server olt-3:8081;
}

server {
    listen 80;
    server_name olt.example.com;

    location / {
        proxy_pass http://olt_backend;
        proxy_set_header X-Request-ID $request_id;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## Troubleshooting

### Common Issues

**1. SNMP Connection Timeout**
```bash
# Test SNMP connectivity
snmpwalk -v2c -c public 192.168.1.1 system

# Check firewall
sudo ufw status
sudo iptables -L
```

**2. Redis Connection Error**
```bash
# Test Redis
redis-cli -h redis -p 6379 ping

# Check Redis logs
docker logs redis
```

**3. High Memory Usage**
```bash
# Check goroutine leaks
curl http://localhost:8081/debug/pprof/goroutine?debug=1

# Adjust Redis pool size (reduce if memory limited)
REDIS_POOL_SIZE=1000
REDIS_MIN_IDLE_CONNECTIONS=50
```

**4. Slow API Response**
```bash
# Check Redis cache hit rate
redis-cli INFO stats | grep keyspace

# Clear cache if needed
redis-cli FLUSHDB
```

### Debug Mode

Enable debug logging:
```bash
export APP_ENV=development
export LOG_LEVEL=debug
```

---

For additional help, please refer to:
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guide
- [SECURITY.md](SECURITY.md) - Security policy
- [GitHub Issues](https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/issues)
