# snmp-olt-zte-c320 Helm Chart

Helm chart for deploying the SNMP OLT ZTE C320 monitoring application on Kubernetes.

## Prerequisites

- Kubernetes 1.23+
- Helm 3.x

## Quick Start

### Option 1: Install from Helm Repository (Recommended)

```bash
helm repo add snmp-olt https://cepat-kilat-teknologi.github.io/go-snmp-olt-zte-c320/
helm repo update

helm install olt-monitor snmp-olt/snmp-olt-zte-c320 \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

### Option 2: Install from Source

```bash
# Add Bitnami repo (for Redis dependency)
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install chart dependencies
cd examples/helm/snmp-olt-zte-c320
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
helm install my-olt ./examples/helm/snmp-olt-zte-c320 \
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
helm install my-olt ./examples/helm/snmp-olt-zte-c320 -f my-values.yaml
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
helm upgrade my-olt ./examples/helm/snmp-olt-zte-c320 -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall my-olt
```
