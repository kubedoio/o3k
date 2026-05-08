# O3K Deployment Guide

O3K is a single-binary OpenStack implementation. Like K3s replaced full Kubernetes with one binary, O3K replaces the entire OpenStack control plane — Keystone, Nova, Neutron, Cinder, and Glance — with a single ~35MB executable.

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Architecture](#architecture)
3. [Deployment Options](#deployment-options)
4. [Running Horizon Dashboard](#running-horizon-dashboard)
5. [Module Detachment](#module-detachment)
6. [Multi-Node Scaling](#multi-node-scaling)
7. [VXLAN Overlay Network](#vxlan-overlay-network)
8. [Configuration Reference](#configuration-reference)
9. [Operations](#operations)
10. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Requirements

- PostgreSQL 15+ (shared database for all services)
- Linux (for real networking/VM mode) or any OS (for stub/development mode)
- Docker (optional, for containerized deployment)

### 30-Second Start (Docker Compose)

```bash
cd deployments/
docker compose up -d
source ~/.o3k-env    # or export manually:
# export OS_AUTH_URL=http://localhost:35357/v3
# export OS_USERNAME=admin
# export OS_PASSWORD=secret
# export OS_PROJECT_NAME=admin
# export OS_USER_DOMAIN_NAME=Default
# export OS_PROJECT_DOMAIN_NAME=Default

openstack token issue
```

### 30-Second Start (Binary)

```bash
# Start PostgreSQL
docker run -d --name o3k-db -e POSTGRES_DB=o3k -e POSTGRES_USER=o3k \
  -e POSTGRES_PASSWORD=secret -p 5432:5432 postgres:17-alpine

# Run O3K
export O3K_DB_URL="postgres://o3k:secret@localhost:5432/o3k?sslmode=disable"
export O3K_JWT_SECRET="$(openssl rand -hex 32)"
./bin/o3k --config config/o3k.yaml --migrations migrations
```

O3K starts 6 HTTP servers:

| Service | Port | Purpose |
|---------|------|---------|
| Keystone | 35357 | Identity, auth, service catalog |
| Nova | 8774 | Compute (VMs) |
| Neutron | 9696 | Networking |
| Cinder | 8776 | Block storage |
| Glance | 9292 | Images |
| Metadata | 8775 | EC2-compatible instance metadata |

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    O3K Binary                            │
│                                                         │
│  ┌──────────┐ ┌──────┐ ┌─────────┐ ┌────────┐ ┌─────┐│
│  │ Keystone │ │ Nova │ │ Neutron │ │ Cinder │ │Glance││
│  │  :35357  │ │:8774 │ │  :9696  │ │ :8776  │ │:9292 ││
│  └────┬─────┘ └──┬───┘ └────┬────┘ └───┬────┘ └──┬──┘│
│       │           │          │           │         │    │
│       └───────────┴──────────┴───────────┴─────────┘    │
│                          │                              │
│              ┌───────────┴────────────┐                 │
│              │   Shared PostgreSQL    │                 │
│              │   Connection Pool      │                 │
│              └────────────────────────┘                 │
└─────────────────────────────────────────────────────────┘
         │                              │
         ▼                              ▼
┌─────────────────┐          ┌────────────────────┐
│   PostgreSQL    │          │  Storage Backend   │
│   (required)    │          │  (local/RBD/S3)    │
└─────────────────┘          └────────────────────┘
```

All services share one process, one DB connection pool, and one JWT secret. There are no message queues, no separate daemons, no conductor services.

### Operating Modes

Each service supports multiple backends:

| Service | Modes | Default |
|---------|-------|---------|
| Nova | `stub`, `real` (libvirt/KVM) | `stub` |
| Neutron | `stub`, `iptables`, `ebpf` | `stub` |
| Cinder | `stub`, `local`, `rbd`, `s3`, hybrid | `local` |
| Glance | `stub`, `local`, `rbd`, `s3`, hybrid | `local` |

**Stub mode** returns plausible fake data with no external dependencies. Use it for development, CI, or Terraform testing on macOS/Windows.

**Real mode** operates actual infrastructure. Requires Linux with appropriate packages (libvirt, iproute2, etc.).

---

## Deployment Options

### Option 1: Docker Compose (Recommended for Getting Started)

```bash
cd deployments/
docker compose up -d
```

This starts PostgreSQL + O3K. All data persists in Docker volumes.

### Option 2: Docker Compose with Horizon

```bash
cd deployments/
docker compose -f docker-compose-horizon.yml up -d
```

Adds the OpenStack Horizon dashboard on port 80 and noVNC console proxy on port 6080.

### Option 3: Bare Metal / systemd

```bash
# Install
make build
sudo cp bin/o3k /usr/local/bin/
sudo mkdir -p /etc/o3k /var/lib/o3k
sudo cp config/o3k.yaml /etc/o3k/o3k.yaml

# Edit config
sudo vim /etc/o3k/o3k.yaml
# Set database.url, keystone.jwt_secret, and modes (real/stub)

# Install systemd service
sudo cp deployments/systemd/lightstack.service /etc/systemd/system/o3k.service
sudo systemctl daemon-reload
sudo systemctl enable --now o3k
```

### Option 4: Kubernetes (Helm)

See [docs/KUBERNETES_DEPLOYMENT.md](KUBERNETES_DEPLOYMENT.md) for full Helm chart instructions.

```bash
helm install o3k ./charts/o3k \
  --set database.url="postgres://..." \
  --set keystone.jwtSecret="$(openssl rand -hex 32)"
```

---

## Running Horizon Dashboard

Horizon is the standard OpenStack web dashboard. O3K is 100% compatible with Horizon Flamingo (2025.2).

### Prerequisites

- O3K running and healthy
- Memcached (required for Horizon sessions)
- noVNC proxy (for VM console access)

### Deploy with Docker Compose

The simplest path is the full-stack Compose file:

```bash
cd deployments/
docker compose -f docker-compose-horizon.yml up -d
```

This starts:
- **postgres** — Database
- **memcached** — Session cache for Horizon
- **o3k** — All OpenStack APIs
- **horizon** — Dashboard on port 80
- **novnc** — VNC console proxy on port 6080

Access Horizon at `http://localhost/dashboard/auth/login/`.

**Credentials**: `admin` / `secret` (domain: `Default`)

### Horizon Configuration

The Horizon container mounts config from `deployments/horizon-config/`:

```
deployments/horizon-config/
├── config.json          # Kolla bootstrap config
├── local_settings       # Django settings (Keystone URL, session backend)
└── apache/
    ├── ports.conf       # Apache listen ports
    └── horizon-nolist.conf  # Apache vhost
```

Key settings in `local_settings`:

```python
OPENSTACK_HOST = "o3k"                              # Docker hostname of O3K
OPENSTACK_KEYSTONE_URL = "http://o3k:35357/v3"      # Internal Keystone URL
OPENSTACK_API_VERSIONS = {
    "identity": 3,
    "image": 2,
    "volume": 3,
    "compute": 2,    # Must be integer 2, NOT "2.1"
}
SESSION_ENGINE = 'django.contrib.sessions.backends.cache'
CACHES = {
    'default': {
        'BACKEND': 'django.core.cache.backends.memcached.PyMemcacheCache',
        'LOCATION': 'memcached:11211',
    }
}
```

### Custom Deployment (External Horizon)

If you already have Horizon deployed, point it at O3K by updating `local_settings`:

```python
OPENSTACK_KEYSTONE_URL = "http://<o3k-host>:35357/v3"
```

O3K's service catalog returns the correct endpoints for all services. Horizon discovers Nova, Neutron, Cinder, and Glance URLs from the token response automatically.

### Console Access

For VNC console to work, O3K must know the public URL of your noVNC proxy. Set in `o3k.yaml`:

```yaml
nova:
  novnc_proxy_host: "novnc"   # hostname/IP of the noVNC proxy
```

Or for production with a public hostname:

```yaml
nova:
  novnc_proxy_host: "console.example.com"
```

### Supported Horizon Version

Only **Horizon Flamingo 2025.2** (Kolla image `quay.io/openstack.kolla/horizon:2025.2-ubuntu-noble`) is tested and supported. Earlier versions (Zed, Yoga, etc.) are not supported.

---

## Module Detachment

O3K runs all services in one process by default. You can "detach" a module by running the standard OpenStack service separately and pointing clients at it instead.

### How It Works

O3K's Keystone generates a service catalog included in every token. This catalog tells clients (Horizon, CLI, Terraform) where each service lives. By default, all endpoints point at the O3K host.

To detach a module:

1. **Deploy the standard OpenStack service** (e.g., real Nova with conductor, scheduler, compute agents)
2. **Update the service catalog** to point at the external service's URL
3. **Remove or ignore** the O3K built-in service on that port

### Example: Detaching Nova (Using Real OpenStack Nova)

```sql
-- Update the service catalog in O3K's database
UPDATE endpoints SET url = 'http://nova-api.example.com:8774/v2.1'
WHERE service_id = (SELECT id FROM services WHERE type = 'compute');
```

Or via the Keystone API:

```bash
# Find the compute endpoint
openstack endpoint list --service compute

# Update it
openstack endpoint set <endpoint-id> --url "http://nova-api.example.com:8774/v2.1"
```

Now when Horizon or Terraform authenticates against O3K Keystone, the token's service catalog points `compute` at your real Nova deployment.

### Example: Using O3K as Identity-Only (Keystone Replacement)

Run O3K with all other services in stub mode, and update endpoints to point at your real services:

```yaml
# o3k.yaml — minimal identity-only config
nova:
  libvirt_mode: stub
neutron:
  networking_mode: stub
cinder:
  storage_mode: stub
glance:
  storage_mode: stub
```

```bash
# Point endpoints at real services
openstack endpoint set <nova-endpoint> --url "http://real-nova:8774/v2.1"
openstack endpoint set <neutron-endpoint> --url "http://real-neutron:9696"
openstack endpoint set <cinder-endpoint> --url "http://real-cinder:8776/v3"
openstack endpoint set <glance-endpoint> --url "http://real-glance:9292"
```

### Detachment Compatibility Matrix

| Module | Can Detach? | Notes |
|--------|------------|-------|
| Keystone | No* | O3K uses JWT tokens; standard Keystone uses Fernet. Token formats are incompatible. |
| Nova | Yes | Update `compute` endpoint. External Nova needs its own Keystone or token validation. |
| Neutron | Yes | Update `network` endpoint. External Neutron manages its own agents. |
| Cinder | Yes | Update `block-storage` endpoint. |
| Glance | Yes | Update `image` endpoint. |

*To use standard Keystone instead of O3K's: deploy Keystone separately and point ALL clients at it. O3K's built-in auth becomes unused.

### Gradual Migration Path

```
Day 1:  All-in-one O3K (development)
Day 30: Detach Glance → use Ceph RGW + real Glance for large image library
Day 60: Detach Nova → real Nova with multiple hypervisor hosts
Day 90: Full OpenStack with O3K as lightweight Keystone
```

---

## Multi-Node Scaling

O3K supports multi-node deployments where multiple O3K instances share the same PostgreSQL database and coordinate via VXLAN overlay networking.

### Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│    Node 1       │     │    Node 2       │     │    Node 3       │
│                 │     │                 │     │                 │
│  O3K binary     │     │  O3K binary     │     │  O3K binary     │
│  (all services) │     │  (all services) │     │  (all services) │
│                 │     │                 │     │                 │
│  tunnel_ip:     │     │  tunnel_ip:     │     │  tunnel_ip:     │
│  10.0.0.1       │     │  10.0.0.2       │     │  10.0.0.3       │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         │    VXLAN (UDP 4789)   │                       │
         └───────────────────────┴───────────────────────┘
                                 │
                    ┌────────────┴────────────┐
                    │   Shared PostgreSQL     │
                    │   (single instance or   │
                    │    Patroni HA cluster)  │
                    └─────────────────────────┘
```

### Adding a New Node

**Step 1: Install O3K on the new host**

```bash
# Copy binary and config
scp bin/o3k node2:/usr/local/bin/
scp config/o3k.yaml node2:/etc/o3k/o3k.yaml
```

**Step 2: Configure the node**

Edit `/etc/o3k/o3k.yaml` on the new node:

```yaml
database:
  url: "postgres://o3k:secret@db-host:5432/o3k?sslmode=disable"

compute:
  node_id: auto          # Generates a unique UUID on first run
  tunnel_ip: auto        # Auto-detects this node's IP (or set explicitly)
  vxlan_port: 4789
  heartbeat_interval: 30s

neutron:
  vxlan_enabled: true    # Required for multi-node
  networking_mode: iptables  # Or "ebpf" for better performance

nova:
  libvirt_mode: real     # Real VMs on this hypervisor
```

**Step 3: Start O3K**

```bash
sudo systemctl start o3k
```

That's it. The new node:
1. Registers itself in the `compute_nodes` table
2. Starts sending heartbeats every 30 seconds
3. Creates VXLAN interfaces for existing networks
4. Begins receiving FDB entries for cross-node connectivity

**Step 4: Verify**

```bash
# On any node
openstack hypervisor list
# Should show the new node

# Check VXLAN interfaces
ip -d link show type vxlan
```

### Node Lifecycle

| Event | What Happens |
|-------|-------------|
| Node starts | Registers in `compute_nodes`, creates VXLAN tunnels |
| Node heartbeats | Updates `last_heartbeat` every 30s |
| Node dies | After 60s (2× heartbeat), marked inactive. FDB entries for dead node removed on next sync. |
| Node returns | Re-registers, resumes heartbeat, VXLANs re-sync |

### Load Balancing API Traffic

O3K nodes are stateless API servers (state lives in PostgreSQL). Put a load balancer in front:

```
                    ┌──────────────┐
                    │  HAProxy /   │
                    │  nginx / LB  │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
         ┌────┴───┐  ┌────┴───┐  ┌────┴───┐
         │ Node 1 │  │ Node 2 │  │ Node 3 │
         │ :35357 │  │ :35357 │  │ :35357 │
         │ :8774  │  │ :8774  │  │ :8774  │
         │ :9696  │  │ :9696  │  │ :9696  │
         └────────┘  └────────┘  └────────┘
```

HAProxy example:

```
frontend openstack_identity
    bind *:35357
    default_backend o3k_keystone

backend o3k_keystone
    balance roundrobin
    option httpchk GET /v3
    server node1 10.0.0.1:35357 check
    server node2 10.0.0.2:35357 check
    server node3 10.0.0.3:35357 check
```

### Scaling Recommendations

| Cluster Size | PostgreSQL | Caching | Notes |
|-------------|-----------|---------|-------|
| 1 node | Single instance | Optional | Development / small deployments |
| 3 nodes | Single instance | Redis recommended | Production minimum |
| 5-10 nodes | Patroni HA (3-node) | Redis required | Mid-scale production |
| 10+ nodes | Patroni HA + read replicas | Redis cluster | Large-scale |

---

## VXLAN Overlay Network

O3K uses VXLAN (Virtual Extensible LAN) to create L2 overlay networks across multiple physical hosts. VMs on different nodes can communicate as if they're on the same LAN.

### How It Works

```
Node 1 (10.0.0.1)                    Node 2 (10.0.0.2)
┌────────────────────┐                ┌────────────────────┐
│  VM-A (10.1.0.5)   │                │  VM-B (10.1.0.6)   │
│       │             │                │       │             │
│  ┌────┴────┐        │                │  ┌────┴────┐        │
│  │  br-net │        │                │  │  br-net │        │
│  └────┬────┘        │                │  └────┬────┘        │
│  ┌────┴────────┐    │                │  ┌────┴────────┐    │
│  │ vxlan-abcd  │    │                │  │ vxlan-abcd  │    │
│  │ VNI: 1001   │    │                │  │ VNI: 1001   │    │
│  └────┬────────┘    │                │  └────┬────────┘    │
└───────┼─────────────┘                └───────┼─────────────┘
        │                                      │
        │         UDP 4789 (VXLAN)             │
        └──────────────────────────────────────┘
                 Physical Network
```

1. **VM-A sends a frame** to VM-B (same virtual network, different host)
2. The bridge forwards it to the **VXLAN interface** (VNI 1001)
3. The kernel encapsulates it in **UDP:4789** with the VXLAN header
4. The outer IP header has `src=10.0.0.1, dst=10.0.0.2`
5. Node 2 decapsulates and delivers to VM-B via its bridge

### Configuration

```yaml
# config/o3k.yaml
compute:
  tunnel_ip: 10.0.0.1      # This node's VTEP IP (or "auto")
  vxlan_port: 4789          # UDP port for VXLAN traffic

neutron:
  vxlan_enabled: true
  vni_range_start: 1000     # Lowest VNI to allocate
  vni_range_end: 10000      # Highest VNI to allocate
  coordination_poll_interval: 1s  # How often to sync FDB entries
  vxlan_mtu: 1450           # Inner MTU (outer needs +50 bytes)
```

### Network Requirements

| Requirement | Value |
|------------|-------|
| UDP port open between nodes | 4789 |
| Physical MTU | ≥ 1550 (VXLAN adds 50 bytes of overhead) |
| Shared PostgreSQL | All nodes must reach the same database |

### VNI Allocation

Each virtual network gets a unique VNI (VXLAN Network Identifier). O3K allocates VNIs from the configured range using atomic database operations:

```sql
INSERT INTO network_vni_allocations (network_id, vni)
VALUES ($1, $2)
ON CONFLICT (vni) DO NOTHING;
```

The allocator scans sequentially from `vni_range_start`. With the default range (1000–10000), you can have up to 9000 overlay networks.

### FDB (Forwarding Database) Synchronization

O3K uses a **poll-based** coordination model (no direct node-to-node messaging):

1. When a port is created, its MAC + VTEP IP is written to `vxlan_fdb_entries`
2. Every node polls the database every `coordination_poll_interval` (default: 1s)
3. New entries → `bridge fdb add` on the local VXLAN interface
4. Stale entries (from dead nodes) → `bridge fdb del`

This means cross-node connectivity establishes within ~1 second of port creation.

### Debugging VXLAN

```bash
# List VXLAN interfaces on this node
ip -d link show type vxlan

# Check FDB entries for a VXLAN interface
bridge fdb show dev vxlan-abcdef12

# Check if VNIs are allocated
psql -c "SELECT * FROM network_vni_allocations;"

# Check which nodes are active
psql -c "SELECT hostname, tunnel_ip, last_heartbeat FROM compute_nodes
          WHERE last_heartbeat > NOW() - INTERVAL '60 seconds';"

# Test UDP connectivity between nodes
nc -u -z node2 4789
```

### Performance Tuning

| Setting | Default | Production Recommendation |
|---------|---------|--------------------------|
| `coordination_poll_interval` | 1s | 1s (fast convergence) or 5s (lower DB load) |
| `vxlan_mtu` | 1450 | 1450 (standard) or 8950 (jumbo frames) |
| Physical MTU | 1500 | 9000 (jumbo) if switch supports it |
| `heartbeat_interval` | 30s | 30s |

---

## Configuration Reference

### Full `o3k.yaml` Template

```yaml
database:
  url: "postgres://o3k:secret@localhost:5432/o3k?sslmode=disable"
  max_connections: 50
  min_connections: 2

keystone:
  port: 35357
  jwt_secret: ""                    # REQUIRED in production (use O3K_JWT_SECRET env var)
  token_ttl: 24h
  admin_user: admin
  admin_password: secret

compute:
  node_id: auto                     # UUID, auto-generated if "auto"
  tunnel_ip: auto                   # VTEP IP, auto-detected if "auto"
  vxlan_port: 4789
  heartbeat_interval: 30s

nova:
  port: 8774
  libvirt_uri: "qemu:///system"
  libvirt_mode: stub                # stub | real
  novnc_proxy_host: ""             # Public hostname for VNC console URLs

neutron:
  port: 9696
  networking_mode: stub             # stub | iptables | ebpf
  vxlan_enabled: false
  vni_range_start: 1000
  vni_range_end: 10000
  coordination_poll_interval: 1s
  vxlan_mtu: 1450

cinder:
  port: 8776
  storage_mode: local               # stub | local | rbd | s3 | local,rbd | local,s3
  ceph_pool: volumes
  ceph_conf: /etc/ceph/ceph.conf

glance:
  port: 9292
  storage_mode: local               # stub | local | rbd | s3 | local,rbd | local,s3 | rbd,s3
  ceph_pool: images
  ceph_conf: /etc/ceph/ceph.conf
  s3_bucket: ""
  s3_region: us-east-1
  s3_endpoint: ""                   # MinIO/Ceph RGW endpoint

cache:
  enabled: false
  redis_url: "redis://localhost:6379/0"

tunnel:
  port: 6385                        # gRPC hub for remote agents (0 = disabled)
  token_secret: ""                  # Agent auth token (empty = open enrollment)

server:
  cors_allowed_origins:
    - "http://localhost"
    - "http://localhost:80"

logging:
  level: info                       # debug | info | warn | error
  format: json                      # json | text
```

### Environment Variable Overrides

| Variable | Overrides | Example |
|----------|-----------|---------|
| `O3K_DB_URL` | `database.url` | `postgres://user:pass@host/db` |
| `O3K_JWT_SECRET` | `keystone.jwt_secret` | `$(openssl rand -hex 32)` |
| `O3K_ENV` | Guard behavior | `development`, `test`, or unset (production) |
| `O3K_ENDPOINT_HOST` | Service catalog hostname | `o3k` (Docker), `api.example.com` (prod) |

Any YAML value can use `${VAR:default}` syntax for env expansion.

---

## Operations

### Health Checks

```bash
# Keystone health
curl -f http://localhost:35357/v3

# All services (from inside the container)
for port in 35357 8774 9696 8776 9292; do
  curl -sf http://localhost:$port/ > /dev/null && echo "Port $port: OK" || echo "Port $port: FAIL"
done
```

### Backup and Restore

All state lives in PostgreSQL. Back up the database:

```bash
pg_dump -U o3k -h localhost o3k > o3k_backup.sql
```

Restore:

```bash
psql -U o3k -h localhost o3k < o3k_backup.sql
```

### Upgrading

1. Build new binary: `make build`
2. Stop O3K: `systemctl stop o3k`
3. Replace binary: `cp bin/o3k /usr/local/bin/o3k`
4. Start O3K: `systemctl start o3k`

Migrations run automatically on startup. No manual migration step needed.

### Monitoring

O3K logs structured JSON to stdout:

```json
{"level":"info","time":"2026-05-08T10:00:00Z","message":"server started","port":35357,"service":"keystone"}
```

Monitor with any log aggregator (Loki, Elasticsearch, CloudWatch).

---

## Troubleshooting

### Common Issues

**"FATAL: JWT secret is set to the insecure default"**

Set a real secret:
```bash
export O3K_JWT_SECRET="$(openssl rand -hex 32)"
```

**Horizon shows "Unable to establish connection"**

Check that `OPENSTACK_KEYSTONE_URL` in `local_settings` resolves from inside the Horizon container:
```bash
docker exec o3k-horizon curl -f http://o3k:35357/v3
```

**VMs on different nodes can't communicate**

1. Check UDP 4789 is open: `nc -u -z node2 4789`
2. Check physical MTU ≥ 1550: `ip link show eth0`
3. Check VXLAN is enabled: grep `vxlan_enabled` in config
4. Check FDB entries: `bridge fdb show dev vxlan-<id>`

**Horizon "compute API version" error**

Ensure `local_settings` has `"compute": 2` (integer), not `"compute": "2.1"` (string).

**Node not appearing in hypervisor list**

Check the node is heartbeating:
```sql
SELECT hostname, tunnel_ip, last_heartbeat,
       CASE WHEN last_heartbeat > NOW() - INTERVAL '60s' THEN 'ACTIVE' ELSE 'DEAD' END
FROM compute_nodes;
```
