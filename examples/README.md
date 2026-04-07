# Deployment Examples

Example deployment configurations for the SNMP OLT ZTE C320 monitoring service.

## Options

| Method | Best For | Directory |
|--------|----------|-----------|
| [Docker Compose](docker/) | Single server, quick setup | `examples/docker/` |
| [Helm Chart](helm/snmp-olt-zte-c320/) | Kubernetes with package management | `examples/helm/` |
| [Kustomize](kustomize/) | Kubernetes with overlay-based config | `examples/kustomize/` |

## Quick Start

### Docker Compose
```bash
cd examples/docker
cp .env.example .env
# Edit .env with your OLT IP and credentials
docker compose up -d
```

### Helm (from Repository)
```bash
helm repo add snmp-olt https://cepat-kilat-teknologi.github.io/go-snmp-olt-zte-c320/
helm repo update
helm install olt-monitor snmp-olt/snmp-olt-zte-c320 \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

### Helm (from Source)
```bash
helm install olt-monitor examples/helm/snmp-olt-zte-c320 \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

### Kustomize
```bash
# Edit base config
vi examples/kustomize/base/configmap.yaml
vi examples/kustomize/base/secret.yaml

# Deploy production
kubectl apply -k examples/kustomize/overlays/production/

# Deploy development
kubectl apply -k examples/kustomize/overlays/development/
```

## Image

All examples use `cepatkilatteknologi/snmp-olt-zte-c320:2.1.0` by default.

Available tags: `latest`, `2.1.0`, `2.1`, `2`
