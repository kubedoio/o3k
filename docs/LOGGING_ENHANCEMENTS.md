# Logging Enhancements (v0.4.1)

**Date**: March 13, 2026
**Sprint**: Option A Polish & Bug Fixes
**Task**: #32 - Improve structured logging and observability

---

## Overview

Enhanced O3K's logging system with context propagation, sensitive data redaction, and operation tracking for better observability and debugging.

---

## Changes Implemented

### 1. Middleware Enhancements

**File**: `internal/middleware/logging.go`

#### Sensitive Data Redaction

Added automatic redaction of sensitive fields in query strings:

```go
var SensitiveFields = []string{
	"password", "token", "secret", "api_key", "auth_token",
	"x-auth-token", "x-subject-token", "authorization", "jwt", "credential",
}

func redactSensitiveQuery(query string) string {
	if query == "" {
		return ""
	}
	lowerQuery := strings.ToLower(query)
	for _, field := range SensitiveFields {
		if strings.Contains(lowerQuery, field) {
			return "[REDACTED]"
		}
	}
	return query
}
```

**Example Output**:
```
# Before: ?password=secret&username=admin
# After:  [REDACTED]
```

#### Context Propagation

Enhanced `LoggingMiddleware()` to extract and propagate user and project context:

```go
// Extract user/project from context (populated by auth middleware)
userID, _ := c.Get("user_id")
projectID, _ := c.Get("project_id")

// Add to all log entries
if userID != nil {
	logEvent.Str("user_id", userID.(string))
}
if projectID != nil {
	logEvent.Str("project_id", projectID.(string))
}
```

#### Slow Request Detection

Automatically flag requests taking > 1 second:

```go
if duration > 1*time.Second {
	logEvent.Bool("slow_request", true)
}
```

#### New Logging Helper Functions

Added 4 context-aware logging helpers:

**LogOperationStart**: Log operation initiation
```go
func LogOperationStart(c *gin.Context, operation, resourceType, resourceID string)
```

**LogOperationEnd**: Log operation completion with duration
```go
func LogOperationEnd(c *gin.Context, operation, resourceType, resourceID string, duration time.Duration, err error)
```

**LogExternalService**: Log external service calls (libvirt, Ceph, S3)
```go
func LogExternalService(c *gin.Context, service, operation string, duration time.Duration, err error)
```

**LogDatabaseQuery**: Log database operations with slow query detection
```go
func LogDatabaseQuery(c *gin.Context, query string, duration time.Duration, err error)
```

### 2. Handler Integration

**File**: `internal/nova/handlers.go`

Enhanced `CreateServer` and `DeleteServer` handlers with structured logging:

#### CreateServer

```go
func (svc *Service) CreateServer(c *gin.Context) {
	logger := middleware.GetLogger(c)
	start := time.Now()

	// Log operation start
	middleware.LogOperationStart(c, "create", "server", req.Server.Name)

	// Log database queries
	queryStart := time.Now()
	err := database.DB.QueryRow(...)
	middleware.LogDatabaseQuery(c, "SELECT flavor", time.Since(queryStart), err)

	// Log instance creation
	logger.Info().Str("instance_id", instanceID).Str("flavor", flavor.Name).Msg("Instance record created")

	// Log libvirt operations
	logger.Info().Str("instance_id", instanceID).Msg("Starting VM creation via libvirt")

	// Log operation completion
	middleware.LogOperationEnd(c, "create", "server", instanceID, time.Since(start), nil)
}
```

#### DeleteServer

```go
func (svc *Service) DeleteServer(c *gin.Context) {
	logger := middleware.GetLogger(c)
	start := time.Now()

	middleware.LogOperationStart(c, "delete", "server", instanceID)

	// Log database query
	queryStart := time.Now()
	err := database.DB.QueryRow(...)
	middleware.LogDatabaseQuery(c, "SELECT libvirt_domain_id", time.Since(queryStart), err)

	// Log external service call
	libvirtStart := time.Now()
	err = svc.vmManager.DeleteVM(ctx, libvirtDomainID)
	middleware.LogExternalService(c, "libvirt", "delete_vm", time.Since(libvirtStart), err)

	middleware.LogOperationEnd(c, "delete", "server", instanceID, time.Since(start), nil)
}
```

---

## Example Log Output

### Server Creation Lifecycle

```json
{
  "level": "info",
  "request_id": "214078f3-06eb-4396-b1b9-04f9d39d287c",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "operation": "create",
  "resource_type": "server",
  "resource_id": "test-enhanced-logging",
  "message": "operation started"
}

{
  "level": "info",
  "request_id": "214078f3-06eb-4396-b1b9-04f9d39d287c",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "instance_id": "20664b51-ce3b-49fb-a57e-9d3888dfb421",
  "flavor": "m1.tiny",
  "message": "Instance record created"
}

{
  "level": "info",
  "request_id": "214078f3-06eb-4396-b1b9-04f9d39d287c",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "instance_id": "20664b51-ce3b-49fb-a57e-9d3888dfb421",
  "message": "Starting VM creation via libvirt"
}

{
  "level": "info",
  "request_id": "214078f3-06eb-4396-b1b9-04f9d39d287c",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "instance_id": "20664b51-ce3b-49fb-a57e-9d3888dfb421",
  "libvirt_uuid": "0c75cbaf-7305-4f1e-9889-e75becbd4bf0",
  "duration": 0.044417,
  "message": "VM created successfully via libvirt"
}

{
  "level": "info",
  "request_id": "214078f3-06eb-4396-b1b9-04f9d39d287c",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "operation": "create",
  "resource_type": "server",
  "resource_id": "20664b51-ce3b-49fb-a57e-9d3888dfb421",
  "duration": 12.941191,
  "message": "operation completed"
}
```

### Server Deletion Lifecycle

```json
{
  "level": "info",
  "request_id": "a2ed2c05-a193-4b4f-a7d4-b7e9f23e9415",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "operation": "delete",
  "resource_type": "server",
  "resource_id": "20664b51-ce3b-49fb-a57e-9d3888dfb421",
  "message": "operation started"
}

{
  "level": "info",
  "request_id": "a2ed2c05-a193-4b4f-a7d4-b7e9f23e9415",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "instance_id": "20664b51-ce3b-49fb-a57e-9d3888dfb421",
  "message": "Instance deleted successfully"
}

{
  "level": "info",
  "request_id": "a2ed2c05-a193-4b4f-a7d4-b7e9f23e9415",
  "user_id": "00000000-0000-0000-0000-000000000001",
  "project_id": "00000000-0000-0000-0000-000000000002",
  "operation": "delete",
  "resource_type": "server",
  "resource_id": "20664b51-ce3b-49fb-a57e-9d3888dfb421",
  "duration": 14.817903,
  "message": "operation completed"
}
```

---

## Benefits

### 1. Request Correlation

Every log entry includes `request_id`, allowing complete request tracing:

```bash
# Trace all logs for a specific request
docker logs o3k | jq 'select(.request_id == "214078f3-06eb-4396-b1b9-04f9d39d287c")'
```

### 2. Multi-Tenant Context

`user_id` and `project_id` in every log entry enable:
- Per-tenant log filtering
- User activity tracking
- Project-level debugging

```bash
# All operations for a specific project
docker logs o3k | jq 'select(.project_id == "00000000-0000-0000-0000-000000000002")'
```

### 3. Performance Monitoring

Duration tracking in operation logs:
- Identify slow operations (> 1s)
- Track external service latency (libvirt, Ceph, S3)
- Database query performance monitoring

```bash
# Find slow operations
docker logs o3k | jq 'select(.duration > 1) | {operation, resource_type, duration}'
```

### 4. Security

Sensitive data automatically redacted:
- Passwords in query strings
- Tokens in headers/URLs
- API keys and credentials

### 5. Debugging

Structured logs enable precise debugging:
- See exact operation flow
- Track resource state transitions
- Identify failure points

---

## Next Steps

### Short Term (v0.4.2)

1. **Extend to Other Services**:
   - Neutron handlers (network, subnet, port operations)
   - Cinder handlers (volume, snapshot operations)
   - Glance handlers (image upload/download)
   - Keystone handlers (token operations)

2. **Add Performance Metrics**:
   - HTTP endpoint response time histograms
   - Database query duration percentiles
   - External service latency tracking

3. **Error Tracking**:
   - Structured error logs with stack traces
   - Error rate metrics per endpoint
   - External service failure tracking

### Medium Term (v0.5.x)

1. **Log Aggregation**:
   - ELK stack integration guide
   - Loki/Grafana setup documentation
   - Centralized logging for multi-node deployments

2. **Alerting**:
   - Slow query alerts (> 100ms)
   - High error rate alerts
   - External service failure alerts

3. **Metrics Export**:
   - Prometheus exporter for operation metrics
   - OpenTelemetry tracing integration

---

## Testing

### Manual Testing

```bash
# Start O3K
docker compose -f deployments/docker-compose.yml up -d

# Create and delete server
TOKEN=$(openstack token issue -f value -c id)
SERVER_ID=$(curl -X POST http://localhost:8774/v2.1/servers \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"server": {"name": "test", "flavorRef": "m1.tiny"}}' | jq -r '.server.id')

curl -X DELETE http://localhost:8774/v2.1/servers/$SERVER_ID \
  -H "X-Auth-Token: $TOKEN"

# Check logs
docker logs o3k | jq "select(.instance_id == \"$SERVER_ID\")"
```

### Validation Checks

✅ Request ID propagated to all log entries
✅ User and project context included
✅ Operation start/end logged with duration
✅ Database queries logged (with slow query detection)
✅ External service calls logged (libvirt)
✅ Sensitive data redacted
✅ Structured JSON format maintained

---

## References

- **Logging Middleware**: `internal/middleware/logging.go`
- **Nova Handlers**: `internal/nova/handlers.go`
- **Database Optimization**: `docs/DATABASE_OPTIMIZATION.md`
- **Troubleshooting**: `docs/TROUBLESHOOTING.md`

---

**Last Updated**: March 13, 2026
**Status**: ✅ Complete (Nova service integrated)
**Next**: Extend to Neutron, Cinder, Glance handlers
