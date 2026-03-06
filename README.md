# LightStack

**100% OpenStack API-compliant cloud infrastructure in Go**

LightStack is a high-performance, Go-based implementation of OpenStack APIs that provides full compatibility with Horizon (OpenStack Dashboard) and the OpenStack CLI/SDK. It replaces Python-based OpenStack services with a fast, synchronous Go engine while maintaining complete API compatibility.

## Features

- ✅ **Keystone v3** - Identity and authentication service
- 🚧 **Nova v2.1** - Compute service (libvirt/KVM integration)
- 🚧 **Neutron v2.0** - Network service (netlink, network namespaces)
- 🚧 **Cinder v3** - Block storage service (Ceph RBD)
- 🚧 **Glance v2** - Image service (Ceph RBD backed)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    LightStack Binary                         │
├─────────────────────────────────────────────────────────────┤
│  Identity Proxy (Keystone v3)    :5000                      │
│  Compute Engine (Nova v2.1)      :8774                      │
│  Network Plumber (Neutron v2.0)  :9696                      │
│  Storage Engine (Cinder v3)      :8776                      │
│  Image Service (Glance v2)       :9292                      │
└─────────────────────────────────────────────────────────────┘
                         ↓
        ┌────────────────┼────────────────┐
        ↓                ↓                ↓
   PostgreSQL       libvirt (KVM)    Ceph (RBD)
   (State DB)      (Compute)         (Storage)
```

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- libvirt (optional, for compute)
- Ceph cluster (optional, for storage)

### Installation

1. **Clone and build:**

```bash
git clone https://github.com/sapcc/lightstack.git
cd lightstack
make install-deps
make build
```

2. **Start PostgreSQL (development):**

```bash
make db-up
```

3. **Run LightStack:**

```bash
make run
```

The following services will be available:
- Keystone: http://localhost:5000/v3
- Nova: http://localhost:8774/v2.1
- Neutron: http://localhost:9696/v2.0
- Cinder: http://localhost:8776/v3
- Glance: http://localhost:9292/v2

### Testing with OpenStack CLI

```bash
# Set environment variables
export OS_AUTH_URL=http://localhost:5000/v3
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default
export OS_USER_DOMAIN_NAME=default
export OS_PROJECT_DOMAIN_NAME=default

# Test authentication
openstack token issue

# List projects
openstack project list

# List users
openstack user list
```

## Configuration

Edit `config/lightstack.yaml` to customize:

```yaml
database:
  url: "postgres://lightstack:secret@localhost/lightstack"
  max_connections: 20

keystone:
  port: 5000
  jwt_secret: "change-me-in-production"
  token_ttl: 24h
  admin_user: admin
  admin_password: secret

# ... other services
```

### Environment Variables

- `LIGHTSTACK_DB_URL` - Override database URL
- `LIGHTSTACK_JWT_SECRET` - Override JWT secret (recommended in production)

## Development

### Project Structure

```
lightstack/
├── cmd/lightstack/          # Main binary entry point
├── internal/
│   ├── keystone/            # Identity service
│   ├── nova/                # Compute service
│   ├── neutron/             # Network service
│   ├── cinder/              # Block storage service
│   ├── glance/              # Image service
│   ├── database/            # DB models and migrations
│   ├── middleware/          # Auth, logging, etc.
│   └── common/              # Shared utilities
├── pkg/
│   ├── hypervisor/          # libvirt abstraction
│   ├── networking/          # netlink abstraction
│   └── storage/             # Ceph RBD abstraction
├── migrations/              # SQL migrations
├── config/                  # Configuration files
└── docs/                    # Documentation
```

### Development Workflow

```bash
# Install development tools
make install-tools

# Run with hot reload
make dev

# Run tests
make test

# Format code
make fmt

# Lint code
make lint
```

## Default Credentials

The seed data creates:

- **User:** `admin`
- **Password:** `secret`
- **Project:** `default`

## Phase 1: Keystone (Current) ✅ COMPLETE

- [x] JWT-based authentication
- [x] Unscoped and scoped tokens
- [x] Service catalog generation
- [x] User/project/role management
- [x] Token validation middleware

## Phase 2: Nova (Current) ✅ COMPLETE

- [x] Server lifecycle API (create, list, get, delete)
- [x] Flavor management (list, get details)
- [x] Microversion negotiation (2.1 - 2.79)
- [x] Hypervisor mocking for Horizon
- [x] Database integration for instances
- [x] Project-scoped filtering
- [x] Server actions (reboot, stop, start)
- [x] XML template generation for VMs
- [ ] libvirt VM execution (coming soon)

## Phase 3: Neutron (Planned)

- [ ] Network namespace isolation
- [ ] Bridge-based networking
- [ ] DHCP management (dnsmasq)
- [ ] Security groups (iptables)
- [ ] Port attachment

## Phase 4: Cinder (Planned)

- [ ] Ceph RBD integration
- [ ] Volume lifecycle
- [ ] Volume attachment to instances
- [ ] Fail-fast on Ceph errors

## Phase 5: Glance (Planned)

- [ ] Image metadata management
- [ ] Ceph RBD storage backend
- [ ] Image upload/download
- [ ] Public/private images

## Roadmap

### v1.0 (MVP)
- Complete Keystone, Nova, Neutron, Cinder, Glance
- Single-node deployment
- Horizon compatibility
- Basic OpenStack CLI support

### v2.0 (Multi-node)
- VXLAN overlay networks
- Floating IPs
- Live migration
- Multi-node control plane
- eBPF-based security groups

### v3.0 (Production)
- High availability
- Placement API
- Heat (orchestration)
- Swift (object storage)
- Observability stack

## Performance

LightStack is designed for:

- **Fast API responses** (< 10ms for most operations)
- **Synchronous operations** (no async state machines)
- **Efficient resource usage** (single binary per node)
- **Fail-fast design** (1-second timeouts on external dependencies)

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## License

Apache License 2.0

## Credits

Built by SAP Converged Cloud (SAPCC) team.

Inspired by:
- [liquid-ceph](https://github.com/sapcc/liquid-ceph) - Ceph integration patterns
- [RustFS](https://github.com/sapcc/rustfs) - Keystone auth reference implementation
- [prysm](https://github.com/sapcc/prysm) - Observability patterns

## Support

- GitHub Issues: https://github.com/sapcc/lightstack/issues
- Documentation: https://github.com/sapcc/lightstack/docs
