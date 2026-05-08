# O3K — Lightweight OpenStack in a Single Binary

O3K replaces the entire OpenStack control plane with one ~35MB Go binary. Like K3s did for Kubernetes — same API, 95% less complexity.

```
Single binary → 5 services → 342 endpoints → 100% Terraform/Horizon/CLI compatible
```

## Quick Start

```bash
cd deployments/
docker compose -f docker-compose-horizon.yml up -d

# Access Horizon: http://localhost/dashboard
# Credentials: admin / secret (domain: Default)

# Or use the CLI:
export OS_AUTH_URL=http://localhost:35357/v3
export OS_USERNAME=admin OS_PASSWORD=secret
export OS_PROJECT_NAME=admin OS_USER_DOMAIN_NAME=Default OS_PROJECT_DOMAIN_NAME=Default
openstack server create --flavor m1.small --image cirros --network default test-vm
```

See [docs/DEPLOYMENT_GUIDE.md](docs/DEPLOYMENT_GUIDE.md) for all deployment options (bare metal, Docker, Kubernetes, multi-node).

## What You Get

| Service | Port | Endpoints |
|---------|------|-----------|
| Keystone (Identity) | 35357 | 61 |
| Nova (Compute) | 8774 | 72 |
| Neutron (Network) | 9696 | 98 |
| Cinder (Block Storage) | 8776 | 73 |
| Glance (Image) | 9292 | 38 |
| **Total** | | **342** |

All endpoints are compatible with:
- Terraform OpenStack provider (zero modifications)
- Horizon dashboard (Flamingo 2025.2)
- OpenStack CLI (`python-openstackclient`)
- gophercloud SDK

## Architecture

```
┌──────────────────────────────────────────────────┐
│                  O3K Binary                       │
│                                                  │
│  Keystone · Nova · Neutron · Cinder · Glance    │
│                                                  │
│  Shared: JWT auth, connection pool, middleware   │
└──────────────────────┬───────────────────────────┘
                       │
              ┌────────┴────────┐
              │   PostgreSQL    │
              └─────────────────┘
```

No RabbitMQ. No Conductor. No Scheduler daemons. One process, one database.

### Operating Modes

| Component | Development | Production |
|-----------|------------|------------|
| Compute | `stub` (fake VMs) | `real` (libvirt/KVM) |
| Networking | `stub` (no netns) | `iptables` or `ebpf` |
| Storage | `stub` or `local` | `rbd` (Ceph), `s3` (MinIO/AWS) |
| Overlay | disabled | VXLAN (multi-node) |

## Configuration

```yaml
# config/o3k.yaml (minimal)
database:
  url: "postgres://o3k:secret@localhost:5432/o3k?sslmode=disable"
keystone:
  jwt_secret: ""  # Set via O3K_JWT_SECRET env var
nova:
  libvirt_mode: stub   # stub | real
neutron:
  networking_mode: stub   # stub | iptables | ebpf
  vxlan_enabled: false    # true for multi-node
cinder:
  storage_mode: local     # stub | local | rbd | s3
glance:
  storage_mode: local     # stub | local | rbd | s3
```

Environment overrides: `O3K_DB_URL`, `O3K_JWT_SECRET`, `O3K_ENV`.

Full reference: [docs/CONFIGURATION.md](docs/CONFIGURATION.md)

## Multi-Node

Run multiple O3K instances against the same PostgreSQL. Enable VXLAN for cross-node VM networking:

```yaml
compute:
  tunnel_ip: 10.0.0.1
neutron:
  vxlan_enabled: true
```

Add a node: install binary, point at shared DB, start. That's it.

See [docs/DEPLOYMENT_GUIDE.md#multi-node-scaling](docs/DEPLOYMENT_GUIDE.md#multi-node-scaling) for full details.

## Documentation

| Topic | Guide |
|-------|-------|
| **Getting started** | [Deployment Guide](docs/DEPLOYMENT_GUIDE.md) |
| **Architecture** | [Architecture](docs/ARCHITECTURE.md) |
| **Configuration** | [Configuration](docs/CONFIGURATION.md) |
| **Operations** | [Operations](docs/OPERATIONS.md) |
| **Networking** | [Networking Modes](docs/NETWORKING_MODES.md) · [VXLAN](docs/VXLAN_IMPLEMENTATION.md) · [L3 Routing](docs/L3_ROUTER_IMPLEMENTATION.md) |
| **Storage** | [Storage Modes](docs/STORAGE_MODES.md) · [S3 Config](docs/S3_CONFIGURATION.md) |
| **Compute** | [Real Libvirt Mode](docs/REAL_LIBVIRT_MODE.md) |
| **Scaling** | [Production Scaling](docs/SCALING.md) · [Kubernetes](docs/KUBERNETES_DEPLOYMENT.md) |
| **API** | [API Reference](docs/API.md) · [Coverage Report](docs/API_COVERAGE_REPORT.md) |
| **Auth** | [Keystone Auth Flow](docs/KEYSTONE_AUTH_FLOW.md) · [Horizon Integration](docs/HORIZON_INTEGRATION.md) |
| **Reference** | [Quick Reference](docs/QUICK_REFERENCE.md) · [Troubleshooting](docs/TROUBLESHOOTING.md) |
| **Contributing** | [Contributing](docs/CONTRIBUTING.md) |
| **Specs** | [Design Specs](docs/specs/) |

## Development

```bash
make build          # Build binary → bin/o3k
make test           # Run unit tests
make dev            # Hot-reload development server
make lint           # golangci-lint
./test/quick_test.sh  # Integration tests
```

## Default Credentials

| Field | Value |
|-------|-------|
| User | `admin` |
| Password | `secret` |
| Project | `admin` |
| Domain | `Default` |

Change `jwt_secret` and `admin_password` in production.

## License

Apache License 2.0
