# snmp-olt-zte Helm Chart

Helm chart for deploying the SNMP OLT ZTE C320 monitoring application on Kubernetes.

## Prerequisites

- Kubernetes 1.23+
- Helm 3.x

## Quick Start

### Option 1: Install from Helm Repository (Recommended)

```bash
helm repo add snmp-olt https://cepat-kilat-teknologi.github.io/snmp-olt-zte/
helm repo update

helm install olt-monitor snmp-olt/snmp-olt-zte \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

### Option 2: Install from Source

```bash
# Add Bitnami repo (for Redis dependency)
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install chart dependencies
cd examples/helm/snmp-olt-zte
helm dependency build

# Install with required values
helm install olt-monitor . \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

## Configuration

### Required Values

| Parameter | Description |
|-----------|-------------|
| `snmp.host` | OLT IP address or hostname |
| `snmp.community` | SNMP community string |

### Common Overrides

```bash
# Production install with all options
helm install my-olt ./examples/helm/snmp-olt-zte \
  --set snmp.host=10.0.0.100 \
  --set snmp.community=myCommunity \
  --set auth.apiKey=my-secret-key \
  --set redis.auth.password=strong-password \
  --set trap.enabled=true \
  --set trap.webhookURL=https://hooks.slack.com/xxx \
  --set powerMonitor.enabled=true \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=olt.example.com
```

### Multi-OLT & per-tenant access

Serve many OLTs from one release and scope each to a tenant. `olt.olts` is the
inline JSON registry (or `olt.oltsFile` to render it into a Secret mounted at
`/etc/olt/olts.json`); `auth.apiUsers` maps API keys to users. A caller sees
only the OLTs whose `user_id` matches theirs (cross-tenant → 404); role
`admin` sees all.

```yaml
olt:
  # Per-OLT topology + owner. boards supports per-slot PON counts ("3:16,5:8").
  olts: |
    [{"id":"c320","user_id":1,"host":"10.0.0.1","community":"public","boards":"1,2"},
     {"id":"c300a","user_id":2,"host":"10.0.0.2","port":1161,"community":"public","boards":"3:16,5:8"}]
  defaultOlt: "c320"   # also served on the bare /api/v1/board/... paths
  # oltsFile: ""       # alternative: same JSON via a mounted Secret file

auth:
  # Per-tenant API keys (overrides auth.apiKey). Stored in the Secret.
  apiUsers: |
    [{"user_id":1,"api_key":"keyA"},
     {"user_id":2,"api_key":"keyB"},
     {"user_id":0,"api_key":"adminKey","role":"admin"}]
```

Single-OLT deployments keep using `snmp.host` / `olt.boards` and `auth.apiKey`.

### Using a values file

Create a `my-values.yaml`:

```yaml
snmp:
  host: "10.0.0.100"
  community: "myCommunity"

auth:
  apiKey: "my-secret-key"

redis:
  auth:
    password: "strong-redis-password"

trap:
  enabled: true
  webhookURL: "https://hooks.slack.com/services/xxx"

powerMonitor:
  enabled: true
  cron: "0 8,12,17 * * *"
  timezone: "Asia/Jakarta"

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: olt.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: olt-tls
      hosts:
        - olt.example.com

resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

Then install:

```bash
helm install my-olt ./examples/helm/snmp-olt-zte -f my-values.yaml
```

### External Redis

To use an external Redis instead of the bundled subchart:

```yaml
redis:
  enabled: false
  host: "redis.example.com"
  port: "6379"
  password: "external-password"
```

## Upgrading

```bash
helm upgrade my-olt ./examples/helm/snmp-olt-zte -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall my-olt
```
