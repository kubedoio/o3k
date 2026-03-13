# Contributing to O3K

Thank you for considering contributing to O3K! This document provides guidelines for contributing to the project.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)
- [Documentation](#documentation)

---

## Code of Conduct

This project follows a standard code of conduct:
- Be respectful and inclusive
- Focus on constructive feedback
- Prioritize the project's goals and users
- Help others learn and grow

---

## Getting Started

### Prerequisites

**Required:**
- Go 1.26 or higher
- PostgreSQL 18 or higher
- Git

**Optional (for real mode testing):**
- libvirt + KVM (Linux only)
- Ceph cluster (for RBD storage)
- AWS S3 / MinIO / Ceph RGW (for S3 storage)

### Fork and Clone

```bash
# Fork the repository on GitHub, then:
git clone https://github.com/YOUR_USERNAME/o3k.git
cd o3k
git remote add upstream https://github.com/cobaltcore-dev/o3k.git
```

---

## Development Setup

### 1. Install Dependencies

```bash
# Install Go dependencies
make install-deps

# Install development tools
make install-tools
```

### 2. Start PostgreSQL

```bash
# Using Docker
docker run -d --name o3k-postgres \
  -e POSTGRES_DB=o3k \
  -e POSTGRES_USER=o3k \
  -e POSTGRES_PASSWORD=secret \
  -p 5432:5432 postgres:18.3

# Or use your local PostgreSQL instance
```

### 3. Run Migrations

```bash
make migrate-up
```

### 4. Build and Run

```bash
# Build
make build

# Run
./bin/o3k --config config/o3k.yaml

# Or run with hot reload
make dev
```

### 5. Verify Installation

```bash
# Test authentication
export OS_AUTH_URL=http://localhost:35357/v3
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default
export OS_USER_DOMAIN_NAME=default
export OS_PROJECT_DOMAIN_NAME=default

openstack token issue
```

---

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/add-floating-ips` - New features
- `fix/volume-attachment-bug` - Bug fixes
- `refactor/storage-backend` - Code refactoring
- `docs/api-reference` - Documentation updates

### Commit Messages

Follow conventional commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(glance): add S3 backend support for image storage

fix(nova): correct power_state in server detail response

refactor(cinder): simplify volume attachment logic

docs(readme): update installation instructions
```

### Code Organization

```
o3k/
├── cmd/o3k/               # Main binary entry point
├── internal/              # Internal packages (not importable)
│   ├── keystone/         # Keystone service implementation
│   ├── nova/             # Nova service implementation
│   ├── neutron/          # Neutron service implementation
│   ├── cinder/           # Cinder service implementation
│   ├── glance/           # Glance service implementation
│   ├── database/         # Database models and migrations
│   ├── middleware/       # HTTP middleware
│   └── common/           # Shared utilities
├── pkg/                   # Public packages (importable)
│   ├── hypervisor/       # libvirt abstraction
│   ├── networking/       # netlink abstraction
│   └── storage/          # Storage backends
├── migrations/            # SQL migrations
├── config/               # Configuration files
├── docs/                 # Documentation
└── test/                 # Integration tests
```

---

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# Run tests for specific package
go test ./internal/keystone/...

# Run with coverage
go test -cover ./...
```

### Integration Tests

```bash
# Run full integration test suite
./test/quick_test.sh

# Run Horizon compatibility tests
./test/horizon_compat_test.sh
```

### Manual Testing

```bash
# Test authentication
openstack token issue

# Test compute
openstack server create --flavor m1.small --image cirros --network private test-vm
openstack server list
openstack server delete test-vm

# Test networking
openstack network create test-network
openstack subnet create --network test-network --subnet-range 192.168.1.0/24 test-subnet
openstack network delete test-network

# Test storage
openstack volume create --size 10 test-volume
openstack volume list
openstack volume delete test-volume

# Test images
openstack image create --file cirros.img --disk-format qcow2 test-image
openstack image list
openstack image delete test-image
```

---

## Submitting Changes

### Pull Request Process

1. **Update your fork:**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run tests:**
   ```bash
   make test
   ./test/quick_test.sh
   ```

3. **Format code:**
   ```bash
   make fmt
   make lint
   ```

4. **Push changes:**
   ```bash
   git push origin feature/your-feature-name
   ```

5. **Create Pull Request:**
   - Go to GitHub and create a PR from your fork
   - Fill out the PR template
   - Link any related issues

### Pull Request Checklist

- [ ] Tests pass (`make test` and `./test/quick_test.sh`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation is updated
- [ ] Commit messages follow conventional format
- [ ] PR description is clear and complete
- [ ] Related issues are linked

---

## Code Style

### Go Style Guidelines

O3K follows standard Go conventions:

1. **Naming:**
   - Use `camelCase` for local variables
   - Use `PascalCase` for exported functions/types
   - Use descriptive names (`imageStore` not `is`)

2. **Error Handling:**
   ```go
   // Good
   if err != nil {
       return fmt.Errorf("failed to create volume: %w", err)
   }

   // Bad
   if err != nil {
       return err
   }
   ```

3. **Context:**
   - Always pass `context.Context` as first parameter
   - Use `ctx` as the variable name
   - Respect context cancellation

4. **Comments:**
   - Public functions must have doc comments
   - Comments should explain "why", not "what"
   - Use complete sentences

5. **Imports:**
   - Standard library first
   - Third-party packages second
   - Internal packages last

**Example:**
```go
package glance

import (
    "context"
    "fmt"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "github.com/cobaltcore-dev/o3k/internal/database"
    "github.com/cobaltcore-dev/o3k/pkg/storage"
)

// CreateImage creates a new image in the image service.
// It returns the image ID and any error encountered.
func (svc *Service) CreateImage(ctx context.Context, req CreateImageRequest) (string, error) {
    if err := validateImageRequest(req); err != nil {
        return "", fmt.Errorf("invalid image request: %w", err)
    }

    imageID := uuid.New().String()
    // ... implementation
    return imageID, nil
}
```

### HTTP Response Format

Follow OpenStack API conventions:

```go
// Success (200/201)
c.JSON(http.StatusOK, gin.H{
    "resource": gin.H{
        "id":   resourceID,
        "name": name,
        // ... fields
    },
})

// List (200)
c.JSON(http.StatusOK, gin.H{
    "resources": []gin.H{
        // ... items
    },
})

// Error (4xx/5xx)
c.JSON(http.StatusBadRequest, gin.H{
    "error": gin.H{
        "message": "Invalid request",
        "code":    400,
        "title":   "Bad Request",
    },
})
```

---

## Documentation

### Code Documentation

- All exported functions must have doc comments
- Use examples for complex functions
- Document error conditions

### User Documentation

When adding features, update:
- `README.md` - If it affects quick start
- `docs/STORAGE_MODES.md` - If it adds storage options
- `docs/REAL_LIBVIRT_MODE.md` - If it affects VM lifecycle
- `docs/MVP_V1_COMPLETE.md` - If it completes a major feature

### Example Documentation

```go
// CreateVolume creates a new block storage volume.
//
// The volume is created using the configured storage backend
// (local, RBD, or S3). If the backend is unavailable, it returns
// an error immediately (fail-fast).
//
// Parameters:
//   - ctx: Request context (respects cancellation)
//   - size: Volume size in GB (1-1000)
//   - name: Optional volume name
//
// Returns:
//   - volumeID: UUID of the created volume
//   - error: Any error encountered
//
// Example:
//   volumeID, err := svc.CreateVolume(ctx, 10, "my-volume")
//   if err != nil {
//       log.Fatalf("Failed to create volume: %v", err)
//   }
func (svc *Service) CreateVolume(ctx context.Context, size int, name string) (string, error) {
    // ... implementation
}
```

---

## Areas Needing Help

### High Priority

1. **Multi-node Networking**
   - VXLAN overlay networks
   - Cross-node VM communication
   - Distributed DHCP

2. **Floating IPs**
   - External network access
   - NAT configuration
   - IP allocation/deallocation

3. **Router L3 Forwarding**
   - Inter-network routing
   - Static routes
   - Default gateway configuration

### Medium Priority

4. **eBPF Security Groups**
   - Replace iptables with eBPF
   - Kernel-space filtering
   - Performance improvement

5. **Live Migration**
   - VM migration between hosts
   - Storage migration
   - Downtime minimization

6. **High Availability**
   - Multi-node control plane
   - Leader election
   - Failover handling

### Low Priority

7. **Heat Orchestration**
   - Template parsing
   - Stack creation
   - Resource dependencies

8. **Placement API**
   - Resource scheduling
   - Affinity/anti-affinity
   - Resource providers

---

## Getting Help

- **GitHub Issues**: https://github.com/cobaltcore-dev/o3k/issues
- **Documentation**: `docs/` directory
- **Code Examples**: `test/` directory

---

## License

By contributing to O3K, you agree that your contributions will be licensed under the Apache License 2.0.

---

**Thank you for contributing to O3K!**
