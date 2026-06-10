# Deployment Examples

Example deployment configurations for the SNMP OLT ZTE C320 monitoring service.

## Options

| Method | Best For | Directory |
|--------|----------|-----------|
| [Docker Compose](docker/) | Single server, quick setup | `examples/docker/` |
| [Helm Chart](helm/snmp-olt-zte/) | Kubernetes with package management | `examples/helm/` |
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
helm repo add snmp-olt https://cepat-kilat-teknologi.github.io/snmp-olt-zte/
helm repo update
helm install olt-monitor snmp-olt/snmp-olt-zte \
  --set snmp.host=192.168.1.1 \
  --set snmp.community=your-community
```

### Helm (from Source)
```bash
helm install olt-monitor examples/helm/snmp-olt-zte \
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

## Multi-OLT & per-tenant access

The examples below show the single-OLT setup (`snmp.host` / `SNMP_HOST`). To run
many OLTs from one instance and scope each to a tenant, use the `OLTS` /
`OLTS_FILE` registry (each OLT carries a `user_id`) plus `API_USERS` — see the
root `README.md` and `examples/helm/snmp-olt-zte/README.md`.

## Image

All examples default to the latest released image; pin a version tag for
reproducible deploys.

Available tags: `latest`, semver releases (e.g. `3.1.0`, `3.0.0`), and the
`3` / `3.0` rolling channels.
