# Configuration Guide

Complete reference for configuring O3K.

---

## Table of Contents

- [Overview](#overview)
- [Configuration File](#configuration-file)
- [Environment Variables](#environment-variables)
- [Database Configuration](#database-configuration)
- [Service Configuration](#service-configuration)
- [Security Configuration](#security-configuration)
- [Storage Configuration](#storage-configuration)
- [Networking Configuration](#networking-configuration)
- [Logging Configuration](#logging-configuration)
- [Advanced Configuration](#advanced-configuration)

---

## Overview

O3K can be configured through:

1. **Configuration file** (`config/o3k.yaml`) - Primary configuration method
2. **Environment variables** - Override specific settings
3. **Command-line flags** - Runtime overrides

**Priority order:** Command-line flags > Environment variables > Configuration file > Defaults

---

## Configuration File

### Location

**Default locations** (checked in order):
1. `./o3k.yaml` (current directory)
2. `/etc/o3k/o3k.yaml` (system-wide)
3. `~/.o3k/o3k.yaml` (user-specific)

**Specify custom location:**
```bash
o3k --config /path/to/custom.yaml
```

### Complete Example

**File:** `config/o3k.yaml`

```yaml
# Database connection
database:
  url: "postgres://lightstack:secret@localhost/lightstack?sslmode=disable"
  max_connections: 20
  max_idle: 5
  conn_max_lifetime: 1h

# Keystone (Identity Service)
keystone:
  port: 35357
  jwt_secret: "change-me-in-production"
  token_ttl: 24h
  admin_user: admin
  admin_password: secret

# Compute node configuration
compute:
  node_id: auto              # Node UUID (auto-generates if "auto")
  tunnel_ip: auto            # VXLAN tunnel IP (auto-detects if "auto")
  vxlan_port: 4789           # VXLAN UDP port
  heartbeat_interval: 30s    # Node heartbeat interval

# Nova (Compute Service)
nova:
  port: 8774
  libvirt_uri: "qemu:///system"
  default_flavor: m1.small
  libvirt_mode: stub         # "stub" or "real"

# Neutron (Network Service)
neutron:
  port: 9696
  dhcp_lease_time: 24h
  iptables_enabled: true
  networking_mode: stub      # "stub", "iptables", or "ebpf"

  # VXLAN overlay networking (multi-node)
  vxlan_enabled: false
  vni_range_start: 1000
  vni_range_end: 10000
  coordination_poll_interval: 1s
  vxlan_mtu: 1450

# Cinder (Block Storage Service)
cinder:
  port: 8776
  ceph_pool: volumes
  ceph_conf: /etc/ceph/ceph.conf
  storage_mode: local        # "stub", "local", "rbd", or "local,rbd"

# Glance (Image Service)
glance:
  port: 9292
  ceph_pool: images
  ceph_conf: /etc/ceph/ceph.conf
  storage_mode: local        # "stub", "local", "rbd", "s3", etc.
  s3_bucket: ""              # S3 bucket name (for S3 storage)
  s3_region: us-east-1       # S3 region
  s3_endpoint: ""            # Custom S3 endpoint (MinIO, Ceph RGW)

# Logging
logging:
  level: info                # debug, info, warn, error
  format: json               # json, text
```

---

## Environment Variables

All configuration options can be overridden via environment variables.

### Naming Convention

- Prefix: `O3K_`
- Nested keys: joined with `_`
- Example: `database.url` → `O3K_DATABASE_URL`

### Common Environment Variables

```bash
# Database
export O3K_DATABASE_URL="postgres://user:pass@host/db"
export O3K_DATABASE_MAX_CONNECTIONS="20"

# Keystone
export O3K_KEYSTONE_PORT="35357"
export O3K_KEYSTONE_JWT_SECRET="your-secret-here"
export O3K_KEYSTONE_TOKEN_TTL="24h"
export O3K_KEYSTONE_ADMIN_USER="admin"
export O3K_KEYSTONE_ADMIN_PASSWORD="secret"

# Nova
export O3K_NOVA_PORT="8774"
export O3K_NOVA_LIBVIRT_URI="qemu:///system"
export O3K_NOVA_LIBVIRT_MODE="real"

# Neutron
export O3K_NEUTRON_PORT="9696"
export O3K_NEUTRON_NETWORKING_MODE="iptables"
export O3K_NEUTRON_VXLAN_ENABLED="true"

# Cinder
export O3K_CINDER_PORT="8776"
export O3K_CINDER_STORAGE_MODE="rbd"
export O3K_CINDER_CEPH_POOL="volumes"

# Glance
export O3K_GLANCE_PORT="9292"
export O3K_GLANCE_STORAGE_MODE="rbd"
export O3K_GLANCE_CEPH_POOL="images"

# Logging
export O3K_LOGGING_LEVEL="debug"
export O3K_LOGGING_FORMAT="text"
```

### Docker Compose Example

```yaml
services:
  o3k:
    image: lightstack-o3k:latest
    environment:
      O3K_DATABASE_URL: "postgres://lightstack:secret@postgres:5432/lightstack?sslmode=disable"
      O3K_KEYSTONE_JWT_SECRET: "${JWT_SECRET}"
      O3K_LOGGING_LEVEL: "info"
      O3K_NOVA_LIBVIRT_MODE: "real"
      O3K_NEUTRON_NETWORKING_MODE: "iptables"
```

---

## Database Configuration

### PostgreSQL Connection

```yaml
database:
  url: "postgres://user:password@host:port/database?sslmode=disable"
  max_connections: 20        # Maximum open connections
  max_idle: 5                # Maximum idle connections
  conn_max_lifetime: 1h      # Connection maximum lifetime
```

### Connection String Format

```
postgres://[user]:[password]@[host]:[port]/[database]?[parameters]
```

**Parameters:**
- `sslmode` - SSL mode (disable, require, verify-ca, verify-full)
- `connect_timeout` - Connection timeout in seconds
- `application_name` - Application name for logging

**Examples:**

```bash
# Local development (no SSL)
postgres://lightstack:secret@localhost:5432/lightstack?sslmode=disable

# Production (SSL required)
postgres://lightstack:secret@db.example.com:5432/lightstack?sslmode=require

# With timeout
postgres://lightstack:secret@localhost:5432/lightstack?sslmode=disable&connect_timeout=10
```

### Connection Pool Tuning

```yaml
database:
  max_connections: 20        # Default: 20
  max_idle: 5                # Default: 5
  conn_max_lifetime: 1h      # Default: 1 hour
```

**Guidelines:**
- **Small deployments:** 10-20 connections
- **Medium deployments:** 20-50 connections
- **Large deployments:** 50-100 connections

**Formula:** `max_connections = (expected_concurrent_requests * 1.5)`

---

## Service Configuration

### Keystone (Identity Service)

```yaml
keystone:
  port: 35357                        # API port
  jwt_secret: "change-me-in-production"  # JWT signing secret
  token_ttl: 24h                     # Token lifetime
  admin_user: admin                  # Default admin username
  admin_password: secret             # Default admin password
```

**Security Notes:**
- ⚠️ **Always change `jwt_secret` in production!**
- ⚠️ **Change default admin credentials!**
- Use strong random string for `jwt_secret` (32+ characters)

**Generate secure JWT secret:**
```bash
openssl rand -hex 32
```

### Nova (Compute Service)

```yaml
nova:
  port: 8774                         # API port
  libvirt_uri: "qemu:///system"      # libvirt connection URI
  default_flavor: m1.small           # Default flavor for VMs
  libvirt_mode: stub                 # "stub" or "real"
```

**libvirt_uri options:**
- `qemu:///system` - System-wide QEMU/KVM (requires root)
- `qemu:///session` - User session QEMU/KVM
- `qemu+tcp://host/system` - Remote libvirt over TCP
- `qemu+ssh://user@host/system` - Remote libvirt over SSH

**libvirt_mode:**
- `stub` - Mock libvirt (for testing, no actual VMs)
- `real` - Real libvirt integration (creates actual VMs)

### Neutron (Network Service)

```yaml
neutron:
  port: 9696                         # API port
  dhcp_lease_time: 24h               # DHCP lease duration
  iptables_enabled: true             # Enable iptables security groups
  networking_mode: stub              # "stub", "iptables", or "ebpf"

  # VXLAN overlay (multi-node)
  vxlan_enabled: false               # Enable VXLAN overlay
  vni_range_start: 1000              # VNI range start
  vni_range_end: 10000               # VNI range end
  coordination_poll_interval: 1s     # Coordination check interval
  vxlan_mtu: 1450                    # VXLAN MTU
```

**networking_mode:**
- `stub` - Mock networking (no actual networks)
- `iptables` - iptables-based security groups (production)
- `ebpf` - eBPF-based filtering (future, not implemented)

**VXLAN configuration** (multi-node only):
- `vxlan_enabled: true` - Enable VXLAN overlay networking
- `vni_range` - VXLAN Network Identifier range (1000-10000)
- `vxlan_mtu: 1450` - Accounts for VXLAN overhead (50 bytes)

### Cinder (Block Storage Service)

```yaml
cinder:
  port: 8776                         # API port
  ceph_pool: volumes                 # Ceph pool for volumes
  ceph_conf: /etc/ceph/ceph.conf     # Ceph config file
  storage_mode: local                # "stub", "local", "rbd", "local,rbd"
```

**storage_mode options:**
- `stub` - Mock storage (no actual volumes)
- `local` - Local filesystem storage (for testing)
- `rbd` - Ceph RBD storage (production)
- `local,rbd` - Hybrid (local + Ceph fallback)

### Glance (Image Service)

```yaml
glance:
  port: 9292                         # API port
  ceph_pool: images                  # Ceph pool for images
  ceph_conf: /etc/ceph/ceph.conf     # Ceph config file
  storage_mode: local                # Storage backend
  s3_bucket: ""                      # S3 bucket (for S3 storage)
  s3_region: us-east-1               # S3 region
  s3_endpoint: ""                    # Custom S3 endpoint
```

**storage_mode options:**
- `stub` - Mock storage (no actual images)
- `local` - Local filesystem
- `rbd` - Ceph RBD
- `s3` - S3-compatible storage
- `local,rbd` - Hybrid (local + Ceph)
- `local,s3` - Hybrid (local + S3)
- `rbd,s3` - Hybrid (Ceph + S3)

**S3 configuration:**
```yaml
glance:
  storage_mode: s3
  s3_bucket: "my-images-bucket"
  s3_region: "us-west-2"
  # For MinIO/Ceph RGW:
  s3_endpoint: "https://s3.example.com"
```

**Environment variables for S3 credentials:**
```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
```

---

## Security Configuration

### JWT Token Security

```yaml
keystone:
  jwt_secret: "change-me-in-production"  # CRITICAL: Change this!
  token_ttl: 24h                         # Token lifetime
```

**Best practices:**
1. **Use strong secret:** 32+ random characters
2. **Rotate regularly:** Change every 90 days
3. **Store securely:** Use secrets management (Vault, AWS Secrets Manager)
4. **Never commit:** Add to `.gitignore`

**Generate secure secret:**
```bash
# Option 1: OpenSSL
openssl rand -hex 32

# Option 2: /dev/urandom
head -c 32 /dev/urandom | base64

# Option 3: UUID
uuidgen | sha256sum | cut -d' ' -f1
```

### Default Credentials

```yaml
keystone:
  admin_user: admin
  admin_password: secret
```

**⚠️ CRITICAL: Change these in production!**

**Change via environment:**
```bash
export O3K_KEYSTONE_ADMIN_USER="myadmin"
export O3K_KEYSTONE_ADMIN_PASSWORD="$(openssl rand -hex 16)"
```

### Database Security

```yaml
database:
  url: "postgres://user:password@host/db?sslmode=require"
```

**Production settings:**
- Use `sslmode=require` or `sslmode=verify-full`
- Use strong database password
- Restrict database access by IP
- Use connection pooling

---

## Storage Configuration

### Local Storage

```yaml
cinder:
  storage_mode: local

glance:
  storage_mode: local
```

**Storage locations:**
- Volumes: `/var/lib/o3k/volumes/`
- Images: `/var/lib/o3k/images/`

**Requirements:**
- Sufficient disk space
- Fast I/O (SSD recommended)
- Regular backups

### Ceph RBD Storage

```yaml
cinder:
  storage_mode: rbd
  ceph_pool: volumes
  ceph_conf: /etc/ceph/ceph.conf

glance:
  storage_mode: rbd
  ceph_pool: images
  ceph_conf: /etc/ceph/ceph.conf
```

**Prerequisites:**
```bash
# Install Ceph client
apt-get install ceph-common

# Verify Ceph access
rbd ls -p volumes
```

**Ceph pools must exist:**
```bash
ceph osd pool create volumes 128
ceph osd pool create images 128
```

### S3 Storage (Images Only)

```yaml
glance:
  storage_mode: s3
  s3_bucket: "my-images-bucket"
  s3_region: "us-west-2"
  s3_endpoint: ""  # Optional: for MinIO, Ceph RGW
```

**AWS S3:**
```bash
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

**MinIO:**
```yaml
glance:
  storage_mode: s3
  s3_bucket: "images"
  s3_region: "us-east-1"
  s3_endpoint: "https://minio.example.com"
```

### Hybrid Storage

```yaml
glance:
  storage_mode: local,rbd  # Try local first, fallback to Ceph
```

**Use cases:**
- Fast local cache + reliable Ceph backup
- Gradual migration between storage backends
- Cost optimization (local SSD + slower network storage)

---

## Networking Configuration

### Single-Node Setup

```yaml
neutron:
  networking_mode: iptables
  iptables_enabled: true
  vxlan_enabled: false
```

**Features:**
- Bridge-based networking
- iptables security groups
- No VXLAN overhead

### Multi-Node Setup

```yaml
compute:
  tunnel_ip: 192.168.1.10    # This node's tunnel IP
  vxlan_port: 4789           # VXLAN UDP port

neutron:
  networking_mode: iptables
  vxlan_enabled: true
  vni_range_start: 1000
  vni_range_end: 10000
  vxlan_mtu: 1450
```

**Requirements:**
- All nodes must reach each other on `tunnel_ip`
- UDP port 4789 open between nodes
- MTU adjusted for VXLAN overhead

**Auto-detection:**
```yaml
compute:
  tunnel_ip: auto  # Auto-detects primary IP
```

---

## Logging Configuration

### Log Levels

```yaml
logging:
  level: info    # debug, info, warn, error
  format: json   # json, text
```

**Levels:**
- `debug` - Verbose logging (development)
- `info` - Normal operations (production)
- `warn` - Warnings only
- `error` - Errors only

### Log Formats

**JSON format** (production):
```json
{"time":"2026-03-07T21:45:12Z","level":"info","msg":"Server started","port":8774}
```

**Text format** (development):
```
2026-03-07T21:45:12Z [INFO] Server started port=8774
```

### Log Output

**Stdout** (default):
```yaml
logging:
  level: info
  format: json
```

**File** (via systemd):
```ini
[Service]
StandardOutput=append:/var/log/o3k/o3k.log
StandardError=append:/var/log/o3k/o3k-error.log
```

**Syslog** (via command):
```bash
o3k 2>&1 | logger -t o3k
```

---

## Advanced Configuration

### Compute Node Registration

```yaml
compute:
  node_id: auto                # Auto-generate UUID
  tunnel_ip: auto              # Auto-detect IP
  heartbeat_interval: 30s      # Heartbeat frequency
```

**Manual node ID:**
```yaml
compute:
  node_id: "550e8400-e29b-41d4-a716-446655440000"
  tunnel_ip: "192.168.1.10"
```

### Performance Tuning

**Database connections:**
```yaml
database:
  max_connections: 50          # Increase for high load
  max_idle: 10
  conn_max_lifetime: 30m       # Shorter for frequent schema changes
```

**DHCP lease time:**
```yaml
neutron:
  dhcp_lease_time: 12h         # Shorter for dynamic environments
```

### Resource Limits

**Default quotas** (set in database):
```sql
UPDATE projects SET quota_instances = 100 WHERE name = 'production';
UPDATE projects SET quota_cores = 200 WHERE name = 'production';
UPDATE projects SET quota_ram = 204800 WHERE name = 'production';
```

### Multi-Tenant Isolation

**Network isolation:**
```yaml
neutron:
  vxlan_enabled: true          # Required for tenant isolation
  vni_range_start: 1000
  vni_range_end: 10000         # 9000 networks max
```

---

## Configuration Validation

### Validate Configuration

```bash
# Check config syntax
o3k --config /path/to/o3k.yaml --validate

# Dry-run (validate without starting)
o3k --config /path/to/o3k.yaml --dry-run
```

### Common Issues

**Missing required fields:**
```
Error: database.url is required
```

**Invalid values:**
```
Error: logging.level must be one of: debug, info, warn, error
```

**Port conflicts:**
```
Error: port 8774 already in use
```

---

## Examples

### Development Setup

```yaml
database:
  url: "postgres://lightstack:secret@localhost/lightstack?sslmode=disable"

keystone:
  jwt_secret: "dev-secret-do-not-use-in-prod"
  token_ttl: 1h

nova:
  libvirt_mode: stub

neutron:
  networking_mode: stub

cinder:
  storage_mode: local

glance:
  storage_mode: local

logging:
  level: debug
  format: text
```

### Production Setup (Single-Node)

```yaml
database:
  url: "postgres://lightstack:${DB_PASSWORD}@db.internal:5432/lightstack?sslmode=require"
  max_connections: 50

keystone:
  jwt_secret: "${JWT_SECRET}"  # From environment
  token_ttl: 24h
  admin_password: "${ADMIN_PASSWORD}"

nova:
  libvirt_mode: real
  libvirt_uri: "qemu:///system"

neutron:
  networking_mode: iptables
  vxlan_enabled: false

cinder:
  storage_mode: rbd
  ceph_pool: volumes

glance:
  storage_mode: rbd
  ceph_pool: images

logging:
  level: info
  format: json
```

### Production Setup (Multi-Node)

```yaml
database:
  url: "postgres://lightstack:${DB_PASSWORD}@db.internal:5432/lightstack?sslmode=require"

compute:
  tunnel_ip: auto
  vxlan_port: 4789

keystone:
  jwt_secret: "${JWT_SECRET}"

nova:
  libvirt_mode: real

neutron:
  networking_mode: iptables
  vxlan_enabled: true
  vni_range_start: 1000
  vni_range_end: 10000

cinder:
  storage_mode: rbd

glance:
  storage_mode: rbd

logging:
  level: info
  format: json
```

---

## Summary

**Key configuration points:**
- ✅ Change `jwt_secret` in production
- ✅ Change default admin credentials
- ✅ Use SSL for database connections
- ✅ Choose appropriate storage backends
- ✅ Enable VXLAN for multi-node
- ✅ Tune database connection pool
- ✅ Set appropriate log levels

**Next steps:**
- [INSTALLATION.md](INSTALLATION.md) - Install O3K
- [OPERATIONS.md](OPERATIONS.md) - Day-to-day operations
- [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md) - Docker-specific config
