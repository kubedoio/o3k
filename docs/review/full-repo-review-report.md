# Full-Repo Review Report

**Date**: 2026-04-07
**Files reviewed**: 173 Go files (~31,400 lines), 59 migration files, 40+ test scripts, CI/CD pipeline, Dockerfile, Makefile
**Total findings**: 42 (Critical: 5, High: 12, Medium: 15, Low: 10)

---

## Codebase Summary

O3K is a single-binary Go implementation of 5 OpenStack services (Keystone, Nova, Neutron, Cinder, Glance) plus Placement and Metadata. It runs 7 HTTP servers on different ports, uses PostgreSQL for all state, and supports stub/real modes for external dependencies (libvirt, Ceph, S3, iptables).

---

## Critical (fix immediately)

### C-1. Internal errors leak raw error messages to API clients
- **Files**: 352 occurrences across all `internal/` packages
- **Pattern**: `c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})`
- **Risk**: Database errors, connection strings, SQL queries, and stack details are returned verbatim to API consumers. This is an information disclosure vulnerability.
- **Fix**: Replace with `common.SendError(c, common.NewInternalServerError("operation failed"))` and log the real error server-side. The `common/error_helpers.go` already provides `HandleDatabaseError()` -- use it consistently.

### C-2. No database transactions for multi-statement operations
- **Files**: All service handlers (nova, cinder, neutron, glance, keystone)
- **Pattern**: Multiple `database.DB.Exec()` calls within a single handler without wrapping in `Begin()`/`Commit()`. Example: `ResetServerMetadata` does DELETE then INSERT in separate calls; a failure between them leaves partial state.
- **Risk**: Data corruption on partial failures. Volume creation, metadata updates, server rebuild, security group rule creation all have this pattern.
- **Fix**: Wrap related DB operations in `pgx.BeginTx()`. Start with the highest-traffic endpoints: volume creation, server creation, metadata operations.

### C-3. MAC address generation uses math/rand (not crypto/rand)
- **File**: `/Users/I761222/git/o3k/internal/neutron/network.go:1126-1131`
- **Pattern**: `rand.Read(buf)` using `math/rand` for MAC address generation.
- **Risk**: Predictable MAC addresses in a multi-tenant environment. An attacker could predict MAC addresses of other tenants' ports, enabling potential ARP spoofing in real networking mode.
- **Fix**: Replace `"math/rand"` import with `"crypto/rand"` and use `crypto/rand.Read(buf)`.

### C-4. Hardcoded passwords in API responses
- **File**: `/Users/I761222/git/o3k/internal/nova/handlers.go:500,1783`
- **Pattern**: `"adminPass": "generated-password"` and `"adminPass": "rescuepass123"` returned to clients.
- **Risk**: Every server creation and rescue operation returns the same known password. In real mode, this means all VMs get the same admin password.
- **Fix**: Generate cryptographically random passwords using `crypto/rand` (16+ character alphanumeric). The `keypairs.go` already has `crypto/rand` usage as a reference pattern.

### C-5. Wildcard CORS allows any origin
- **File**: `/Users/I761222/git/o3k/internal/middleware/logging.go:252`
- **Pattern**: `Access-Control-Allow-Origin: *`
- **Risk**: Any website can make authenticated API calls if a user's browser has a valid token. Combined with the cookie-less X-Auth-Token header approach this is somewhat mitigated, but it is still a violation of defense-in-depth.
- **Fix**: Make CORS origins configurable via `o3k.yaml`. Default to the Horizon host or `localhost` in development.

---

## High (fix this sprint)

### H-1. Only 6 unit test files for 31,400 lines of production code
- **Files**: Only `internal/common/config_test.go`, `internal/common/errors_test.go`, `pkg/hypervisor/libvirt_test.go`, `pkg/hypervisor/xml_template_test.go`, `pkg/networking/netns_test.go`, `pkg/networking/vxlan_test.go`
- **Coverage gap**: Zero unit tests for keystone auth logic, nova handlers, neutron network operations, cinder volume logic, glance image handling, database operations, middleware, metadata service, placement service.
- **Risk**: Regressions are only caught by contract/integration tests which require a running instance. No fast feedback loop for developers.
- **Fix**: Prioritize unit tests for: (1) auth token generation/validation, (2) error helper functions, (3) config loading with env overrides, (4) MAC/IP generation, (5) pagination logic.

### H-2. Inconsistent error response formats across services
- **Counts**: 54 calls using structured `SendError()`/`HandleDatabaseError()` vs. 1,176 calls using inline `c.JSON()` with ad-hoc error formats.
- **Pattern**: Keystone uses `{"error": {"message": ..., "code": ..., "title": ...}}`, Nova uses `{"error": gin.H{...}}`, some handlers use `{"error": "string"}`, others use `{"badRequest": {...}}`.
- **Risk**: OpenStack clients (Terraform, CLI, Horizon) expect specific error formats per spec. Inconsistent formats cause unpredictable client behavior and debugging difficulty.
- **Fix**: Audit each service and replace inline error JSON with `common.SendError()` calls. The error framework is already well-designed -- it is just not being used.

### H-3. Global mutable database pool `var DB *pgxpool.Pool`
- **File**: `/Users/I761222/git/o3k/internal/database/db.go:15`
- **Pattern**: 665 direct references to `database.DB` across all packages.
- **Risk**: Impossible to test with a mock database, impossible to run parallel tests, tight coupling between all packages and the global state. Makes dependency injection impractical.
- **Fix**: Pass `*pgxpool.Pool` (or a `DB` interface) through service constructors. Start by defining an interface with `QueryRow`, `Query`, `Exec` methods, then inject into service structs.

### H-4. Linting disabled in CI pipeline
- **File**: `/Users/I761222/git/o3k/.github/workflows/ci.yml:52-55`
- **Pattern**: `echo "Linting temporarily disabled - test file cleanup in progress"`
- **Risk**: Code quality regressions accumulate unchecked. The comment mentions "78 test file errors to fix" which suggests technical debt is growing.
- **Fix**: Fix the 78 lint errors in test files, re-enable linting in CI. Consider running lint on production code only (`./internal/... ./pkg/... ./cmd/...`) while test files are being cleaned up.

### H-5. Missing migration file 042
- **Files**: `/Users/I761222/git/o3k/migrations/` -- sequence jumps from 041 to 043.
- **Risk**: While `golang-migrate` handles gaps fine, this suggests a migration was deleted or incorrectly numbered. Could indicate a lost schema change or a merge conflict that was resolved incorrectly.
- **Fix**: Verify migration 042 was intentionally skipped. If it was removed, document why. If it was lost, recreate it.

### H-6. No rate limiting implementation
- **Files**: `internal/nova/handlers.go:1415`, `internal/cinder/volumes.go:1754,1792`
- **Pattern**: `"rate": []gin.H{}, // No rate limiting implemented"`
- **Risk**: A single client can exhaust database connections or CPU by sending rapid API calls. The `NewRateLimitError()` constructor exists but is never used.
- **Fix**: Add a token-bucket or sliding-window rate limiter middleware per project_id. Gin has community middleware for this (`gin-contrib/ratelimit`).

### H-7. Goroutines launched without tracking or graceful shutdown
- **Files**: 11 `go func()` calls across nova and cinder handlers
- **Pattern**: Goroutines are launched for async VM creation, stub mode simulation, volume operations -- but there is no `sync.WaitGroup` or shutdown coordination.
- **Risk**: During graceful shutdown (5-second timeout in `main.go:264`), in-flight goroutines may be killed mid-operation, leaving database state inconsistent (e.g., instance stuck in BUILD).
- **Fix**: Add a `sync.WaitGroup` to service structs. Register goroutines at launch, wait during shutdown. For stub-mode `time.Sleep` goroutines, use `context.WithCancel` to enable fast shutdown.

### H-8. Duplicate route registration in Cinder
- **File**: `/Users/I761222/git/o3k/internal/cinder/volumes.go:38-166`
- **Pattern**: Many routes are registered twice -- once without `project_id` prefix and once with. Example: `r.POST("/v3/volumes", svc.CreateVolume)` and `v3.POST("/volumes", svc.CreateVolume)`.
- **Risk**: Route conflicts, confusion about which path is canonical, potential for inconsistent behavior. Handlers must check both `c.Param("project_id")` and `c.GetString("project_id")` for the same operation.
- **Fix**: Pick one canonical route structure and redirect the other. OpenStack Cinder API uses `v3/{project_id}/volumes` as the canonical form.

### H-9. ErrorHandlingMiddleware defined but not used
- **File**: `/Users/I761222/git/o3k/internal/middleware/errors.go:11` and `/Users/I761222/git/o3k/cmd/o3k/main.go:276-400`
- **Pattern**: `ErrorHandlingMiddleware()` and `NotFoundHandler()` / `MethodNotAllowedHandler()` are defined but never registered in any of the `createXxxServer()` functions.
- **Risk**: Panics in handlers result in Gin's default recovery response (which does not match OpenStack format). Undefined routes return Gin's default 404 rather than an OpenStack-formatted error.
- **Fix**: Add `r.Use(middleware.ErrorHandlingMiddleware())` to each server factory. Register `NotFoundHandler()` via `r.NoRoute()`.

### H-10. Placement service returns inconsistent version numbers
- **File**: `/Users/I761222/git/o3k/internal/placement/placement.go:39,59`
- **Pattern**: `GetVersions` returns `max_version: "1.40"` but `GetVersion` returns `max_version: "1.39"`.
- **Risk**: Clients may negotiate different API versions depending on which endpoint they hit, leading to unpredictable behavior.
- **Fix**: Use a single constant for the supported placement API version.

### H-11. Hardcoded `localhost` URLs in response bodies
- **Files**: 15+ occurrences across nova/handlers.go, nova/console.go, keystone/handlers.go, keystone/services.go
- **Pattern**: `"href": "http://localhost:8774/v2.1/servers/%s"` -- self-links and console URLs hardcode `localhost`.
- **Risk**: When O3K runs behind a load balancer or in Docker, these URLs point to the wrong host. Clients following HATEOAS links will fail.
- **Fix**: Extract base URL from the incoming request's `Host` header or from configuration (similar to how `O3K_ENDPOINT_HOST` is already used for the service catalog).

### H-12. Nova `GetLimits` returns hardcoded quota values
- **File**: `/Users/I761222/git/o3k/internal/nova/handlers.go:1396-1442`
- **Pattern**: `"maxTotalInstances": 100, "maxTotalCores": 200` are hardcoded despite a quota system existing in the database.
- **Risk**: Quota changes made via `UpdateQuotaSet` are not reflected in the limits API. Terraform and Horizon show incorrect quota information.
- **Fix**: Query the `nova_quotas` table for the project's actual limits (falling back to defaults if no custom quota exists).

---

## Medium (fix when touching these files)

### M-1. Mixed logging: stdlib `log.Printf` and zerolog `logger`
- **Count**: 30 calls to `log.Printf`/`log.Print` vs. 33 calls to structured `logger`.
- **Fix**: Replace stdlib log calls with zerolog. Most are debug messages in nova/handlers.go goroutines.

### M-2. `context.Background()` used in 20 places inside request handlers
- **Files**: nova/handlers.go, nova/advanced_actions.go, keystone/auth.go, metadata/service.go
- **Pattern**: Goroutines spawned from handlers use `context.Background()` instead of deriving from request context.
- **Risk**: Cancellation signals from client disconnects or server shutdown are not propagated. Database queries continue after the client has left.
- **Fix**: Pass a derived context with timeout instead of `context.Background()`. For goroutines that must outlive the request, use a service-level context that respects shutdown.

### M-3. Pagination logic duplicated across all services
- **Files**: nova/handlers.go, cinder/volumes.go, neutron/network.go, glance/images.go
- **Pattern**: Each service has its own copy of marker-based and offset-based pagination parsing.
- **Fix**: Extract pagination helper to `internal/common/pagination.go` with `ParsePagination(c *gin.Context) (limit int, offset int, marker string)`.

### M-4. `continue` on scan errors silently drops rows
- **Files**: All list handlers across all services.
- **Pattern**: `if err := rows.Scan(...); err != nil { continue }` -- scan failures are silently ignored.
- **Risk**: Corrupted or unexpected data is silently dropped from API responses. Users see incomplete lists with no indication of failure.
- **Fix**: Log scan errors at warning level, include a count of failed rows in debug output.

### M-5. `Flavor.IsPublic` JSON tag is wrong in model
- **File**: `/Users/I761222/git/o3k/internal/database/models.go:58`
- **Pattern**: `IsPublic bool \`json:"OS-FLV-EXT-DATA:ephemeral"\`` -- the `IsPublic` field has the JSON tag for the `ephemeral` field.
- **Fix**: Change to `json:"os-flavor-access:is_public"`.

### M-6. Swallowed errors in critical paths
- **Files**: 15 instances of `_, _ =`, `_ =`, or `// Ignore error` in production code.
- **Notable**: `neutron/network.go:35` swallows the SecurityGroupManager init error, `glance/images.go:442` swallows image store deletion error.
- **Fix**: At minimum, log swallowed errors. For security group manager initialization, propagate the error to the caller.

### M-7. Unused variables in neutron handlers
- **Files**: `neutron/auto_allocated_topology.go:19,58,130`, `nova/advanced_actions.go:542-543`
- **Pattern**: `_ = projectIDParam`, `_ = host`, `_ = blockMigration` -- function parameters are received then discarded.
- **Fix**: Either use the parameters or remove them from the function signature.

### M-8. Duplicate `RunMigrations` function
- **Files**: `database/db.go:104-124` (`RunMigrations`) and `database/migrate.go:54-79` (`MigrateUp`)
- **Pattern**: Two functions that do the same thing with slightly different error handling.
- **Fix**: Remove `RunMigrations` from db.go and use `MigrateUp` everywhere.

### M-9. Docker health check port mismatch
- **File**: `/Users/I761222/git/o3k/build/package/Dockerfile:69,72`
- **Pattern**: `EXPOSE 5000 8774 9696 8776 9292 8775` exposes port 5000 (which is not used), but does not expose 35357 (Keystone). The health check correctly uses 35357.
- **Fix**: Replace `5000` with `35357` in the EXPOSE directive.

### M-10. `ListFlavorsDetail` pagination with `id > $marker` assumes UUID ordering
- **File**: `/Users/I761222/git/o3k/internal/nova/handlers.go:1088-1089`
- **Pattern**: `query += " AND id > $marker"` -- lexicographic UUID comparison does not provide deterministic pagination order.
- **Fix**: Use `created_at`-based cursor pagination (consistent with ListServers).

### M-11. Metadata service uses `context.Background()` for all DB queries
- **File**: `/Users/I761222/git/o3k/internal/metadata/service.go:286,296,309`
- **Pattern**: All database queries use `context.Background()` instead of the request context.
- **Fix**: Pass `c.Request.Context()` through to database queries.

### M-12. `generate-password` placeholder in Makefile DB URL
- **File**: `/Users/I761222/git/o3k/Makefile:10`
- **Pattern**: `DB_URL?=postgres://o3k:secret@localhost:5432/o3k?sslmode=disable`
- **Risk**: Default password `secret` is fine for development but the `sslmode=disable` default is a concern if the Makefile is used in production-like environments.
- **Fix**: Document that this is development-only. Consider defaulting to `sslmode=prefer`.

### M-13. E2E tests are empty stubs in CI
- **File**: `/Users/I761222/git/o3k/.github/workflows/ci.yml:270-273,301-304`
- **Pattern**: Both e2e-fast and e2e-full jobs are no-ops that print "not yet implemented".
- **Fix**: Either implement the E2E tests or remove the jobs from CI to avoid false confidence.

### M-14. Glance image update allows SQL column injection via PATCH field names
- **File**: `/Users/I761222/git/o3k/internal/glance/images.go:486`
- **Pattern**: `query := fmt.Sprintf("UPDATE images SET %s = $1...", field)` where `field` comes from JSON keys in the PATCH request body.
- **Risk**: While Gin's JSON binding limits this somewhat, a crafted PATCH body could inject SQL column names. The existing code does whitelist fields, but the pattern is fragile.
- **Fix**: Use an explicit allowlist map (`validFields["name"] = "name"`) and reject any field not in the map before interpolation.

### M-15. `db-up` in Makefile uses postgres:16, docs say PostgreSQL 18
- **File**: `/Users/I761222/git/o3k/Makefile:121` vs. CLAUDE.md
- **Pattern**: `docker run ... postgres:16` but documentation states "PostgreSQL 18".
- **Fix**: Align the version. Use a variable: `POSTGRES_VERSION?=18`.

---

## Low (nice to have)

### L-1. TODO comments in storage package
- **File**: `/Users/I761222/git/o3k/pkg/storage/image_store.go:207,350,450,533`
- **Pattern**: 4 TODO comments about using go-ceph for RBD operations.

### L-2. `Dockerfile` creates non-root user but runs as root
- **File**: `/Users/I761222/git/o3k/build/package/Dockerfile:42-43,65`
- **Pattern**: Creates user `o3k` but does not `USER o3k`. Comment explains this is for network namespace operations.

### L-3. `Access-Control-Allow-Headers` is incomplete
- **File**: `/Users/I761222/git/o3k/internal/middleware/logging.go:254`
- **Pattern**: Only allows `Content-Type, X-Auth-Token, X-Subject-Token`. Missing headers like `OpenStack-API-Version`, `X-OpenStack-Nova-API-Version`, `Accept`.

### L-4. Nova version discovery returns inconsistent version numbers
- **File**: `/Users/I761222/git/o3k/internal/nova/handlers.go:191,208`
- **Pattern**: `ListVersions` says `"version": "2.79"` but `GetVersion` says `"version": "2.90"`.

### L-5. Benchmark scripts not integrated into CI
- **Files**: `test/benchmark/` contains Go benchmarks and shell scripts but they are not run in CI.

### L-6. `database/query_logger.go` exists but is not used
- **File**: `/Users/I761222/git/o3k/internal/database/query_logger.go`
- **Pattern**: `QueryLogger` struct is defined but never instantiated in `main.go`.

### L-7. Docker Compose files proliferation
- **Files**: 6 different docker-compose files in `deployments/` with overlapping purposes.
- **Fix**: Consolidate into docker-compose.yml + docker-compose.override.yml + docker-compose.test.yml.

### L-8. `run_nova_tests_with_fix.sh` in repo root
- **File**: `/Users/I761222/git/o3k/run_nova_tests_with_fix.sh`
- **Pattern**: Appears to be a leftover debug/fix script committed to root.
- **Fix**: Move to `test/` or delete if no longer needed.

### L-9. Contract test helper code duplicated
- **Files**: `test/contract/helpers.go` and `test/contract/cinder/helpers.go`
- **Pattern**: Both files contain similar auth/client setup code.

### L-10. Missing `.golangci.yml` configuration review
- **File**: `/Users/I761222/git/o3k/.golangci.yml`
- **Pattern**: Linter config exists but linting is disabled in CI (see H-4). Config may be stale.

---

## Systemic Patterns

### SP-1. Inline error formatting instead of structured error framework (1,176 occurrences)
**Seen in**: All 5 service packages, every handler file.
**Description**: The codebase has a well-designed error framework (`internal/common/errors.go` + `error_helpers.go`) with 30+ constructors for OpenStack-compatible errors. However, 95% of handlers bypass it and construct ad-hoc JSON error responses inline. This is the single highest-leverage fix in the codebase.
**Fix approach**: Service-by-service migration. Start with keystone (most critical for auth), then nova (most handlers), then neutron, cinder, glance.

### SP-2. Direct global database access instead of dependency injection (665 occurrences)
**Seen in**: Every handler across all services.
**Description**: All database access goes through the global `database.DB` variable. No service accepts a database connection as a constructor parameter.
**Fix approach**: Define a `DB` interface in `internal/database/`, inject it into service constructors. This enables unit testing with mocks.

### SP-3. Pagination logic reimplemented per endpoint (~20 implementations)
**Seen in**: nova/handlers.go, cinder/volumes.go, neutron/network.go, glance/images.go
**Description**: Every list endpoint parses `limit`, `offset`, `marker` independently with slightly different defaults and behaviors.
**Fix approach**: Create `common.ParsePagination()` returning a `PaginationParams` struct. Use it in all list handlers.

### SP-4. Silent row scan failures across all list operations (~40 occurrences)
**Seen in**: Every `rows.Next()` loop in every service.
**Description**: `if err := rows.Scan(...); err != nil { continue }` silently drops rows that fail to scan.
**Fix approach**: Add `logger.Warn().Err(err).Msg("failed to scan row")` to each continue statement, or create a helper function.

### SP-5. Stub mode simulation via `time.Sleep` in goroutines (5 occurrences)
**Seen in**: nova/handlers.go, nova/advanced_actions.go, cinder/volumes.go
**Description**: Stub mode simulates async operations with `go func() { time.Sleep(N); database.DB.Exec(...) }()`. These goroutines use `context.Background()`, cannot be cancelled, and are not tracked.
**Fix approach**: Use `time.AfterFunc` with cancellation, or a dedicated background worker with a shutdown channel.

---

## Review Metadata
- **Packages analyzed**: 12 Go packages (cmd/o3k, internal/*, pkg/*)
- **Non-Go files reviewed**: Makefile, Dockerfile, CI pipeline, docker-compose files, migration files
- **Approach**: Full source file scan with pattern-based analysis across all packages
- **Score pre-check**: N/A (score-component.py not available for Go projects)
- **Duration**: Full-repo review
- **Key metric**: 95% of error responses bypass the structured error framework
