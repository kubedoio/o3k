# O3K Constitution

## Core Principles

### I. OpenStack Flamingo API Compatibility (NON-NEGOTIABLE)
O3K MUST maintain 100% API compatibility with OpenStack Flamingo (2025.2 release).
- All API endpoints must match Flamingo specifications exactly
- Horizon dashboard (Flamingo version) must work without modification
- OpenStack CLI tools must function identically to native OpenStack
- API responses must match expected JSON structure, fields, and data types
- Microversion support follows Flamingo microversion ranges

**Verification**: Every feature must pass contract tests using real OpenStack clients (Horizon, python-openstackclient, gophercloud).

### II. Synchronous Operations
All API operations complete before returning - no message queues, no async state machines.
- VM creation: libvirt calls complete within API request
- Network operations: namespace/bridge creation happens immediately
- Volume operations: storage backend operations complete synchronously
- Database updates happen within the same transaction as the operation

**Rationale**: Simplicity over scalability. Trade throughput for debuggability and operational simplicity.

### III. Fail-Fast Design
External dependency failures return immediately (< 1 second timeouts).
- libvirt connection failures: 1 second timeout
- Ceph operations: 1 second timeout
- Database queries: connection pool exhaustion returns immediately
- Network namespace operations: fail fast if Linux capabilities missing

**Never**: Retry loops, exponential backoff, or delayed error responses. User gets immediate feedback.

### IV. Multi-Mode Architecture
Every service supports stub mode (development) and real mode (production).
- **Stub mode**: Returns fake data, no external dependencies, safe on macOS
- **Real mode**: Full implementation with libvirt/Ceph/iptables, requires Linux
- Mode detection: Check `svc.libvirtMode == "stub"` or `svc.vmManager == nil`
- Default to stub mode when dependencies unavailable

**Requirement**: All features must work in stub mode for development/testing without Linux.

### V. Project Isolation
All resources scoped by `project_id` from JWT token.
- Database queries auto-filter by project_id
- Network namespaces per project for network isolation
- No cross-project resource access without explicit admin role check
- JWT contains: user_id, project_id, roles

### VI. Single Binary Deployment
O3K runs as one ~35MB binary with embedded services.
- All six services (Keystone, Nova, Neutron, Cinder, Glance, Metadata) in cmd/o3k/main.go
- Shared database connection pool (PostgreSQL)
- Shared auth middleware (JWT validation)
- Shared logging middleware (structured JSON)

**No**: Microservices, service meshes, or separate processes per service.

## Technology Constraints

### OpenStack Version Targeting
- **Target Version**: OpenStack Flamingo (2025.2)
- **API Versions**:
  - Identity (Keystone): v3
  - Compute (Nova): v2.1 (microversions 2.1-2.90)
  - Network (Neutron): v2.0
  - Block Storage (Cinder): v3 (microversions 3.0-3.70)
  - Image (Glance): v2
  - Placement: v1.0 (microversions 1.0-1.39)

### Required Dependencies
- **Database**: PostgreSQL 14+ (golang-migrate for migrations)
- **Language**: Go 1.21+ (stdlib + minimal external deps)
- **Real Mode** (Linux only):
  - libvirt + KVM (github.com/digitalocean/go-libvirt)
  - netlink (github.com/vishvananda/netlink)
  - iptables (github.com/coreos/go-iptables)
  - Optional: Ceph RBD (github.com/ceph/go-ceph), S3 (AWS SDK v2)

### Forbidden Patterns
- **No message queues**: RabbitMQ, Kafka, Redis streams
- **No async workers**: Celery, background job processors
- **No state machines**: Conductor-style workflows
- **No service discovery**: Consul, etcd (services on fixed ports)
- **No container orchestration required**: Runs standalone or in Docker Compose

## Development Workflow

### Test-First Development
When implementing new features:
1. **Write contract tests first** - Use OpenStack clients (gophercloud, openstackclient)
2. **Get approval** - Review test strategy with user
3. **Confirm RED** - Tests must fail initially
4. **Implement** - Write code to make tests pass (GREEN)
5. **Refactor** - Clean up while keeping tests green

Integration tests in `test/` directory use bash + OpenStack CLI for end-to-end validation.

### Backwards Compatibility
- All API endpoints maintain OpenStack API compatibility
- Breaking API changes require OpenStack microversion bumps
- Horizon dashboard compatibility tested in `test/horizon_compat_test.sh`
- Database migrations must be reversible (up/down migrations)

### Security Requirements
- JWT secrets MUST be changed in production (config/o3k.yaml)
- Database passwords via environment variables in production
- Token TTL default: 24 hours (configurable)
- bcrypt for password hashing (no plaintext passwords in database)

## Quality Gates

### Before Merge
- [ ] OpenStack CLI compatibility verified (openstack server create, etc.)
- [ ] Horizon dashboard tested (if UI-affecting changes)
- [ ] Contract tests pass (test/contract/ or test/integration_test.sh)
- [ ] Both stub and real modes tested (if applicable)
- [ ] Database migrations applied successfully (up and down)
- [ ] No hardcoded credentials or secrets

### Performance Standards
- API response time: < 100ms for metadata operations
- Fail-fast timeouts: 1 second for external dependencies
- Database connection pool: 20 connections default
- VM creation: Complete within 10 seconds (real mode)

## Governance

This constitution supersedes all other development practices. When in conflict:
1. **OpenStack API compatibility** always wins
2. **Simplicity** over performance optimization
3. **Synchronous operations** over async complexity
4. **Fail-fast** over retry resilience

Amendments require:
- Documentation of rationale
- User approval for breaking changes
- Migration plan for existing deployments

**Version**: 1.0.0 | **Ratified**: 2026-03-13 | **Last Amended**: 2026-03-13
