# Kustomize Deployment Examples

Kustomize manifests for deploying the SNMP OLT ZTE C320 monitoring application to Kubernetes.

## Directory Structure

```
kustomize/
├── base/                          # Base manifests
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   └── redis.yaml
└── overlays/
    ├── development/               # Development overlay
    │   ├── kustomization.yaml
    │   └── configmap-patch.yaml
    └── production/                # Production overlay
        ├── kustomization.yaml
        ├── deployment-patch.yaml
        └── configmap-patch.yaml
```

## Prerequisites

- Kubernetes cluster (v1.21+)
- `kubectl` with Kustomize support (v1.14+)

## Configuration

Before deploying, update the following values:

### ConfigMap (`base/configmap.yaml`)

- `SNMP_HOST` — IP address of your ZTE C320 OLT
- `TRAP_WEBHOOK_URL` — Webhook URL for trap and power monitor notifications

### Secret (`base/secret.yaml`)

Replace the base64-encoded placeholder values with your actual credentials:

```bash
# Generate base64 values
echo -n 'your-snmp-community' | base64
echo -n 'your-redis-password' | base64
echo -n 'your-api-key' | base64
```

## Quick Start

### Deploy to Development

```bash
# Preview the manifests
kubectl kustomize examples/kustomize/overlays/development/

# Apply
kubectl apply -k examples/kustomize/overlays/development/
```

### Deploy to Production

```bash
# Preview the manifests
kubectl kustomize examples/kustomize/overlays/production/

# Apply
kubectl apply -k examples/kustomize/overlays/production/
```

### Deploy Base (Staging)

```bash
kubectl apply -k examples/kustomize/base/
```

## Verify Deployment

```bash
# Check all resources in the namespace
kubectl get all -n olt-monitoring

# Check pod logs
kubectl logs -n olt-monitoring -l app.kubernetes.io/name=snmp-olt-zte

# Test health endpoint
kubectl port-forward -n olt-monitoring svc/snmp-olt-zte 8081:8081
curl http://localhost:8081/health
```

## Overlay Differences

| Setting | Base (Staging) | Development | Production |
|---------|---------------|-------------|------------|
| APP_ENV | staging | development | production |
| Replicas | 1 | 1 | 3 |
| CACHE_PREWARM | false | false | true |
| ONU_INFO_TTL | 300s | 60s | 600s |
| ONU_DETAIL_TTL | 300s | 60s | 600s |
| CPU Request | 250m | 250m | 500m |
| Memory Request | 256Mi | 256Mi | 512Mi |
| CPU Limit | 1000m | 1000m | 2000m |
| Memory Limit | 1Gi | 1Gi | 2Gi |

## Cleanup

```bash
kubectl delete -k examples/kustomize/overlays/development/
# or
kubectl delete -k examples/kustomize/overlays/production/
```
