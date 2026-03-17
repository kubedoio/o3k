# Horizon Dashboard Integration with O3K

**📚 Complete Documentation**: See **[INDEX.md](INDEX.md)** for full documentation index with learning paths.


## Overview

This document describes how OpenStack Horizon dashboard integrates with O3K, covering authentication, API compatibility, and implementation patterns.

## Architecture

```
┌─────────────────┐
│  Horizon UI     │ (Django web application)
│  (Port 8080)    │
└────────┬────────┘
         │
         │ HTTP API Calls
         │ (Authenticated via Token)
         ▼
┌─────────────────────────────────────────────────────────────┐
│                         O3K Services                        │
├─────────────────┬─────────────────┬─────────────────────────┤
│  Keystone       │  Nova           │  Neutron                │
│  (Port 35357)   │  (Port 8774)    │  (Port 9696)            │
│  - Auth         │  - Compute      │  - Networking           │
│  - Token Issue  │  - Instances    │  - Networks/Ports       │
│  - Service Cat. │  - Flavors      │  - Security Groups      │
├─────────────────┼─────────────────┼─────────────────────────┤
│  Cinder         │  Glance         │  Placement              │
│  (Port 8776)    │  (Port 9292)    │  (Port 8778)            │
│  - Block Store  │  - Images       │  - Resources (stub)     │
│  - Volumes      │                 │                         │
│  - Transfers    │                 │                         │
└─────────────────┴─────────────────┴─────────────────────────┘
```

## Authentication Flow

### 1. User Login (Horizon → Keystone)

```
User enters credentials in Horizon login form
         ↓
POST /v3/auth/tokens
{
  "auth": {
    "identity": {
      "methods": ["password"],
      "password": {
        "user": {
          "name": "admin",
          "domain": {"name": "Default"},
          "password": "secret"
        }
      }
    },
    "scope": {
      "project": {
        "name": "default",
        "domain": {"name": "Default"}
      }
    }
  }
}
         ↓
Keystone validates credentials against PostgreSQL (bcrypt hash)
         ↓
Keystone generates JWT token (HMAC-SHA256 signed)
         ↓
Response: X-Subject-Token header + service catalog
{
  "token": {
    "user": {"id": "...", "name": "admin"},
    "project": {"id": "...", "name": "default"},
    "catalog": [
      {"type": "identity", "endpoints": [...]},
      {"type": "compute", "endpoints": [...]},
      ...
    ]
  }
}
         ↓
Horizon stores token in Django session
```

### 2. API Calls (Horizon → O3K Services)

All subsequent API calls from Horizon include the token:

```
GET /v3/servers HTTP/1.1
X-Auth-Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
         ↓
O3K Auth Middleware extracts token
         ↓
Validates JWT signature (shared secret: keystone.jwt_secret)
         ↓
Decodes JWT payload:
{
  "user_id": "uuid",
  "project_id": "uuid",
  "roles": ["admin", "member"]
}
         ↓
Sets project_id in Gin context: c.Set("project_id", projectID)
         ↓
Handler uses: projectID := c.GetString("project_id")
         ↓
Database queries auto-filter by project_id
```

## API Compatibility Patterns

### Pattern 1: v2 vs v3 URL Styles

Horizon uses python-cinderclient/python-novaclient which support both v2-style (no project_id in URL) and v3-style (project_id in URL) endpoints.

**Example: Cinder Volumes**

v2-style (project_id from token):
```
GET /v3/volumes/detail
X-Auth-Token: <token>
```

v3-style (project_id explicit):
```
GET /v3/{project_id}/volumes/detail
X-Auth-Token: <token>
```

**O3K Implementation Pattern**:
```go
// Top-level routes (v2-style) - MUST come before parameterized group
r.GET("/v3/volumes", svc.ListVolumes)
r.GET("/v3/volumes/detail", svc.ListVolumesDetail)

// Parameterized group (v3-style)
v3 := r.Group("/v3/:project_id")
{
    v3.POST("/volumes", svc.CreateVolume)
    v3.GET("/volumes/:id", svc.GetVolume)
    v3.DELETE("/volumes/:id", svc.DeleteVolume)
}
```

**Handler Implementation**:
```go
func (svc *Service) ListVolumes(c *gin.Context) {
    // Extract project_id from JWT token (set by auth middleware)
    projectID := c.GetString("project_id")

    rows, err := database.DB.Query(c.Request.Context(), `
        SELECT id, name, size, status
        FROM volumes
        WHERE project_id = $1
    `, projectID)
    // ...
}
```

### Pattern 2: Route Registration Order

**CRITICAL**: Gin router matches routes in registration order. Specific routes MUST be registered before parameterized routes.

**WRONG** (causes 404 errors):
```go
v3 := r.Group("/v3/:project_id")
{
    v3.GET("/volumes", svc.ListVolumes)  // Matches /v3/:project_id/volumes
}

r.GET("/v3/volumes", svc.ListVolumes)    // Never matches (shadowed)
```

**CORRECT**:
```go
// Register specific routes first
r.GET("/v3/volumes", svc.ListVolumes)
r.GET("/v3/os-volume-transfer", svc.ListVolumeTransfersNoProject)

// Then parameterized group
v3 := r.Group("/v3/:project_id")
{
    v3.POST("/volumes", svc.CreateVolume)
}
```

### Pattern 3: Alias Handlers for v2-style Endpoints

Some endpoints need both v2 and v3 style access to the same handler logic:

```go
// ListVolumeTransfersNoProject is an alias for routes without project_id in URL
func (svc *Service) ListVolumeTransfersNoProject(c *gin.Context) {
    svc.ListVolumeTransfers(c)
}

// Route registration
r.GET("/v3/os-volume-transfer", svc.ListVolumeTransfersNoProject)
v3.GET("/os-volume-transfer", svc.ListVolumeTransfers)
```

Both routes call the same underlying handler, which extracts project_id from token.

## Horizon Pages and Required Endpoints

### Project Dashboard (/dashboard/project/)

#### Instances Page (/dashboard/project/instances/)

**Dependencies**:
- **Nova**: `GET /v2.1/servers/detail` - List instances
- **Nova**: `GET /v2.1/flavors/detail` - Flavor names
- **Glance**: `GET /v2/images` - Image names
- **Cinder**: `GET /v3/volumes/detail` - Volume attachments
- **Cinder**: `GET /v3/os-volume-transfer/detail` - Available transfers
- **Neutron**: `GET /v2.0/networks` - Network details
- **Neutron**: `GET /v2.0/ports` - Port mappings

**Fixed Issues**:
- ✅ Volume transfer endpoint returning 404 (commit 1a4c634)
- ✅ Volumes list endpoint returning 404 (commit 7c160b8)

### Admin Dashboard (/dashboard/admin/)

#### Instances Page (/dashboard/admin/instances/)

**Dependencies**: Same as project instances page, but with admin scope

## Troubleshooting

### Debugging 404 Errors

1. **Check O3K logs for the exact path**:
```bash
docker logs o3k 2>&1 | grep "status.*404"
```

Example output:
```json
{"level":"warn","path":"/v3/volumes/detail","status":404}
```

2. **Verify route registration order** in service file (e.g., `internal/cinder/volumes.go`):
   - List operations (no ID) → top-level routes
   - CRUD operations (with ID) → v3/:project_id group

3. **Test endpoint directly**:
```bash
TOKEN=$(openstack token issue -f value -c id)
curl -H "X-Auth-Token: $TOKEN" http://localhost:8776/v3/volumes/detail
```

### Debugging Authentication Issues

1. **Check token validity**:
```bash
openstack token issue
```

2. **Verify JWT secret matches** across all services in `config/o3k.yaml`:
```yaml
keystone:
  jwt_secret: "your-secret-key-change-in-production"
```

3. **Check auth middleware is applied** in service initialization:
```go
r.Use(middleware.AuthMiddleware(authService))
```

### Common Django Errors in Horizon

**Error**: `cinderclient.exceptions.NotFound: Not Found (HTTP 404)`

**Cause**: Endpoint not registered or shadowed by parameterized route

**Solution**: Register specific routes before parameterized group

**Error**: `keystoneauth1.exceptions.http.Unauthorized: The request you have made requires authentication`

**Cause**: Token expired or invalid

**Solution**: Re-login to Horizon or check JWT secret configuration

## Service Catalog

Keystone returns service catalog with all available endpoints. Horizon uses this to discover service URLs.

**Example catalog entry (Cinder)**:
```json
{
  "type": "volumev3",
  "name": "cinderv3",
  "endpoints": [
    {
      "id": "cinder-v3-endpoint",
      "interface": "public",
      "region": "RegionOne",
      "url": "http://localhost:8776/v3/$(project_id)s"
    }
  ]
}
```

Note: `$(project_id)s` is a template variable that Horizon replaces with the actual project ID from the token.

## OpenStack API Microversions

O3K implements specific microversion ranges per service:

- **Identity (Keystone)**: v3
- **Compute (Nova)**: v2.1 (microversions 2.1-2.90)
- **Network (Neutron)**: v2.0
- **Block Storage (Cinder)**: v3 (microversions 3.0-3.71)
- **Image (Glance)**: v2
- **Placement**: v1.0 (microversions 1.0-1.40, stub implementation)

Horizon requests microversions via headers:
```
OpenStack-API-Version: compute 2.90
X-OpenStack-Nova-API-Version: 2.90
```

O3K checks these headers and returns version-specific responses.

## Testing Horizon Integration

### 1. Start Services

```bash
docker compose -f deployments/docker-compose-horizon.yml up -d
```

### 2. Access Horizon

```
URL: http://localhost:8080/dashboard
Username: admin
Password: secret
Domain: Default
```

### 3. Verify Key Pages

- ✅ Login page loads
- ✅ Project overview loads
- ✅ Project instances page loads (no Django errors)
- ✅ Admin instances page loads (no Django errors)
- ✅ Networks page loads
- ✅ Volumes page loads

### 4. Monitor O3K API Calls

```bash
docker logs o3k -f | grep -E "(GET|POST|PUT|DELETE)"
```

Watch for 404 or 401 errors indicating missing endpoints.

## Configuration

### Horizon Container

**Image**: `quay.io/openstack.kolla/horizon:2025.2-ubuntu-noble` (Flamingo version)

**Environment Variables**:
```yaml
KEYSTONE_URL: "http://o3k:35357/v3"
DEFAULT_REGION: "RegionOne"
```

**Volume Mount**:
```yaml
volumes:
  - ./config/local_settings.py:/etc/openstack-dashboard/local_settings.py:ro
```

### O3K Configuration

**config/o3k.yaml**:
```yaml
keystone:
  jwt_secret: "your-secret-key-change-in-production"  # MUST match across all services
  token_ttl: 24h

nova:
  libvirt_mode: "stub"  # or "real" on Linux with KVM

neutron:
  networking_mode: "stub"  # or "iptables"/"ebpf" on Linux

cinder:
  storage_mode: "stub"  # or "local"/"rbd"/"s3"

glance:
  storage_mode: "stub"  # or "local"/"rbd"/"s3"
```

## Implementation Status

### ✅ Working Features

- **Authentication**: JWT-based auth with Keystone
- **Service Discovery**: Service catalog for endpoint discovery
- **Instance Management**: List, create, delete instances
- **Network Management**: Networks, subnets, ports, security groups
- **Volume Management**: Volumes, snapshots, volume types
- **Volume Transfers**: Create, list, accept, delete transfers
- **Image Management**: Upload, list, delete images
- **Flavor Management**: List available flavors
- **Keypair Management**: Import, list, delete SSH keys

### 🚧 Stub Implementations

- **Placement API**: Returns empty results (sufficient for Horizon compatibility)
  - Resource providers: `[]`
  - Resource classes: `[]`
  - Traits: `[]`

### 📋 Tested Horizon Pages

- ✅ Login page
- ✅ Project overview
- ✅ Project instances page
- ✅ Admin instances page
- ✅ Networks page
- ✅ Volumes page
- ⚠️  Other pages (images, security groups, etc.) need verification

## Future Enhancements

1. **Enhanced Placement API**: Implement resource tracking for scheduling
2. **Horizon Dashboard Testing**: Comprehensive testing of all Horizon pages
3. **API Microversion Negotiation**: Proper version detection and response formatting
4. **Performance Optimization**: Caching service catalog, reducing database queries
5. **Multi-Region Support**: Currently single region (RegionOne)

## References

- [OpenStack Flamingo API Documentation](https://docs.openstack.org/2025.2/)
- [Horizon Documentation](https://docs.openstack.org/horizon/2025.2/)
- [O3K Constitution](/.specify/memory/constitution.md)
- [Cinder API Reference](https://docs.openstack.org/api-ref/block-storage/)
- [Nova API Reference](https://docs.openstack.org/api-ref/compute/)
