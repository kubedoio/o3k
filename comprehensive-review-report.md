# O3K Production Readiness Assessment — v5 (Post-Fix Deep Audit)

**Date**: 2026-05-07
**Review Method**: 5 per-package deep-read agents (Wave 0), spec-grounded analysis
**Branch**: `fix/keystone-idor-access-control` (post all v4 fixes)
**Previous Score (v4)**: 5.5/10
**Current Score**: **6.5/10**

---

## Executive Summary

The v4 fix pass resolved 55 issues and raised the actual score from 5.5 to 6.5. Key improvements: app credential auth works, token revocation is DB-backed, microversion middleware exists, marker pagination on Nova ListServers, admin-only access controls added. However, significant gaps remain in three categories:

1. **Spec features never implemented**: Access rules, policy engine, server_usages, proper quota format
2. **Inconsistent fix application**: Pagination fixed in some endpoints but not others; rows.Err() added in some paths but missed in auth paths; CapLimit applied inconsistently
3. **Response shape gaps**: Horizon will still break on quota display, network topology, subnet listing, and server host columns

**What works for Terraform**: Basic CRUD for VMs, networks, volumes, security groups, app credentials
**What breaks for Horizon**: Quota display (wrong format), server host columns (missing), network topology (wrong tenant_id), subnet pagination (broken marker), tenant usage (empty)
**What's missing from spec**: Access rules, policy engine, legacy migration workflow

---

## Score Progression

```
v4 assessment:        5.5/10 (spec coverage measured honestly)
After v4 fix pass:    6.5/10 (app creds, revocation, microversions, pagination partial)
Phase 1 target:       7.5/10 (fix remaining CRITICALs + HIGHs)
Phase 2 target:       8.5/10 (implement access rules, fix quota format, full pagination)
```

**Why 6.5 not 7.5**: The v4 fix pass fixed the right things but applied them inconsistently. Marker pagination exists for ListServers but not ListServersDetail, ListSubnets, ListRouters, or ListFloatingIPs. rows.Err() was added systemically in Phase 5 but missed in auth.go's most critical paths. The quota format fix was never done (still flat).

---

## Findings by Severity

### CRITICAL (8) — Production blockers

| # | Package | Finding | Impact |
|---|---------|---------|--------|
| C1 | keystone | compat/embedded.go registers Keystone with NO AuthMiddleware | Full unauthenticated admin access via embedded server |
| C2 | nova | GetServer/ListServersDetail missing OS-EXT-SRV-ATTR:host attributes | Horizon host column blank |
| C3 | nova | GetQuotaSet response is flat — not nested per spec | Horizon Quota panel shows all zeros |
| C4 | nova | server_usages always empty in tenant-usage | Horizon Usage tab shows zero rows |
| C5 | neutron | allocateFloatingIP scans ALL floating IPs with no subnet filter | Wrong collision detection, performance |
| C6 | neutron | allocateFloatingIP has TOCTOU race — no transaction | Duplicate floating IP assignment |
| C7 | neutron | ListNetworks returns caller's project_id as tenant_id | Wrong ownership for shared networks |
| C8 | nova | allocateNextIP strips CIDR assuming /24 — invalid IPs for other masks | Port creation with malformed addresses |

### HIGH (14)

| # | Package | Finding | Impact |
|---|---------|---------|--------|
| H1 | keystone | BuildServiceCatalog uses database.DB directly (bypasses DI) | Tests panic on nil DB |
| H2 | keystone | AuthenticatePassword/Token role-fetch missing rows.Err() | Tokens issued with partial roles on DB blip |
| H3 | keystone | ChangePassword requires original_password for admin resets | Admin password reset impossible |
| H4 | keystone | roleRows in app_credentials not deferred — resource leak | DB connection leak |
| H5 | keystone | BuildServiceCatalog missing rows.Err() — partial catalog cached 24h | Broken endpoint discovery for 24h |
| H6 | nova | Microversion comparison doesn't validate major == "2" | Version 3.x passes uncapped |
| H7 | nova | ListServersDetail marker uses created_at alone (not stable) | Non-deterministic pagination |
| H8 | nova | Console GetRemoteConsole drops non-ErrNoRows DB errors | Fabricated VNC URL on DB failure |
| H9 | neutron | ListSubnets uses OFFSET pagination (broken marker logic) | Wrong page results |
| H10 | neutron | ListRouters and ListFloatingIPs use OFFSET pagination | Wrong page results |
| H11 | neutron | AddRouterInterface response missing network_id, tenant_id, id | Horizon topology broken |
| H12 | neutron | CreateSecurityGroup/UpdateSecurityGroup missing rules array | Horizon SG panel rendering fails |
| H13 | neutron | SG rule fields use zero values instead of null | Terraform state drift |
| H14 | neutron | UpdateSecurityGroup response missing security_group_rules | Post-update rendering broken |

### MEDIUM (19)

| # | Package | Finding |
|---|---------|---------|
| M1 | keystone | access_rules for app credentials NOT IMPLEMENTED (spec req 4) |
| M2 | keystone | ValidateToken doesn't re-check user.enabled from DB |
| M3 | keystone | ListUsers uses interface{} comparison instead of c.GetBool |
| M4 | keystone | substituteURLTemplates overly broad %s replacement |
| M5 | keystone | IsTokenRevoked uses context.Background() — no timeout |
| M6 | nova | GetServer derives launched_at from created_at instead of DB |
| M7 | nova | UpdateServer hardcodes user_id = projectID |
| M8 | nova | ListFlavorsDetail does not apply CapLimit |
| M9 | nova | GetAvailabilityZones rows.Err() not checked |
| M10 | nova | Hypervisor statistics use hardcoded values |
| M11 | neutron | allocateIPFromSubnet locks ALL ports table-wide |
| M12 | neutron | CreateNetwork hard-codes provider:network_type as "flat" |
| M13 | neutron | GetSecurityGroup duplicates inline logic vs. helper |
| M14 | cinder | Volume detail missing volume_type, encrypted, availability_zone |
| M15 | cinder | Volume list (brief) omits status, bootable, AZ |
| M16 | glance | Image responses missing owner, schema, file fields |
| M17 | common | Empty JWT secret not caught (only "change-me-in-production") |
| M18 | common | Empty project_id in token not validated in middleware |
| M19 | nova | DRY: novaMaxVersion duplicated in microversion.go and handlers.go |

### LOW (7)

| # | Package | Finding |
|---|---------|---------|
| L1 | keystone | GetDomain doesn't distinguish ErrNoRows from other DB errors |
| L2 | keystone | Migration 064 duplicates migration 029 (IF NOT EXISTS) |
| L3 | keystone | CreateDomain swallows all DB errors as conflict |
| L4 | nova | Console URLs hardcode localhost:6080 — non-production-safe |
| L5 | nova | CreateServer response missing OS-EXT-STS fields |
| L6 | neutron | ListFloatingIPs returns no next-page links |
| L7 | neutron | fmt.Printf used for warnings instead of structured logger |

---

## Spec Coverage Analysis (Updated)

### keystone-minimal-iam-design-v2

| Requirement | v4 Status | Current Status |
|-------------|-----------|----------------|
| Application credential CRUD | Crash (missing table) | **WORKING** (migration 064 applied) |
| Application credential authentication | NOT IMPLEMENTED | **WORKING** (dispatch + AuthenticateApplicationCredential) |
| Access rules for app credentials | NOT IMPLEMENTED | **NOT IMPLEMENTED** — no table, no enforcement |
| Policy engine (basic RBAC) | PARTIAL | **PARTIAL** — ad-hoc isAdmin checks, no policy evaluation |
| Token revocation (durable) | BROKEN | **WORKING** — sync.Map + DB slow path |
| bcrypt cost 12 | PARTIAL (app creds only) | **WORKING** — both user passwords and app creds use cost 12 |
| Service catalog in token | PARTIAL | **WORKING** — present in auth and ValidateToken |
| Role UUID in token | BROKEN | **WORKING** — lookup by name, UUID returned |
| Legacy migration (cost 10→12) | NOT IMPLEMENTED | **NOT IMPLEMENTED** — no re-hash on login |

**Coverage: ~60%** (up from 30%)

### SPEC-002 (Horizon Full Compatibility)

| Requirement | v4 Status | Current Status |
|-------------|-----------|----------------|
| FR-001: Token format with catalog | PARTIAL | **WORKING** (catalog in ValidateToken) |
| FR-003: Microversion headers | NOT IMPLEMENTED | **WORKING** (middleware adds headers) |
| FR-005: Server extended attributes | NOT IMPLEMENTED | **PARTIAL** — OS-EXT-STS/AZ/DCF present, OS-EXT-SRV-ATTR missing |
| FR-006: Marker-based pagination | PARTIAL (1 endpoint) | **PARTIAL** — Nova ListServers fixed, 4 Neutron endpoints still broken |
| FR-009: Network provider attributes | NOT IMPLEMENTED | **PARTIAL** — fields present but hardcoded "flat" |
| FR-010: Security group rules in response | NOT IMPLEMENTED | **PARTIAL** — List/Get have rules, Create/Update don't |
| FR-011: Router interface response | INCOMPLETE | **STILL INCOMPLETE** — missing 3 fields |
| FR-012: Volume detail attributes | INCOMPLETE | **STILL INCOMPLETE** — missing 3 fields |
| FR-014: Quota set format | INCORRECT | **STILL INCORRECT** — flat format, not nested |
| FR-015: Tenant usage with servers | BROKEN | **STILL BROKEN** — empty array |
| FR-020: Hypervisor statistics | NOT ASSESSED | **BROKEN** — hardcoded capacity values |

**Coverage: ~45%** (up from 25%)

### SPEC-000 (Compliance Addendum)

| Requirement | Status |
|-------------|--------|
| Contract test suite | NOT STARTED |
| Terraform test suite | NOT STARTED |
| CLI test suite | Partial (bash scripts only) |
| Schema validation | NOT STARTED |
| Zero failures policy | Cannot evaluate |

**Coverage: ~10%** (unchanged)

---

## What Now Works (After v4 Fixes)

- Application credential authentication (full flow)
- Token revocation persisted to DB (survives restart)
- Microversion headers on Nova responses
- Marker-based pagination on Nova ListServers
- Admin-only access for ListUsers, GetUser, ListRoleAssignments
- Admin-only for token revocation of other users' tokens
- OS-EXT-STS:*, OS-EXT-AZ:*, OS-DCF:* on GetServer/ListServersDetail
- rows.Err() checked in ~67 locations (Phase 5 pass)
- CapLimit(1000) on most Nova list endpoints
- IP allocation with FOR UPDATE (ports.go)
- bcrypt cost 12 for all password hashing

---

## What Still Breaks

| Scenario | What Happens | Root Cause |
|----------|--------------|------------|
| Horizon loads Quota Summary | All zeros | C3: flat format |
| Horizon shows server host column | Blank | C2: OS-EXT-SRV-ATTR missing |
| Horizon Usage tab | Zero instance rows | C4: server_usages empty |
| Horizon network topology shared nets | Wrong owner | C7: caller's project_id as tenant_id |
| Two floating IP allocations same time | Possible duplicate | C6: no transaction |
| Terraform creates port on /16 subnet | Malformed IP stored | C8: /24 assumption |
| Integration tests via compat server | Full admin without auth | C1: no AuthMiddleware |
| Admin resets user password | 400 Bad Request | H3: original_password required |
| Neutron subnet listing with marker | Wrong page | H9: OFFSET pagination |
| SG creation in Horizon | Rules panel empty | H12: no rules in create response |

---

## Remediation Priority

### Phase 1: Fix CRITICALs (3-4 hours)
1. **C1**: Add AuthMiddleware to compat/embedded.go keystoneGin — 15 min
2. **C2**: Add OS-EXT-SRV-ATTR:host, instance_name to GetServer/ListServersDetail — 30 min
3. **C3**: Convert GetQuotaSet to nested format `{in_use, limit, reserved}` — 45 min
4. **C4**: Populate server_usages from DB in tenant-usage endpoints — 45 min
5. **C5+C6**: Fix allocateFloatingIP (subnet filter + transaction) — 60 min
6. **C7**: Read project_id from DB row in ListNetworks/GetNetwork/CreateNetwork — 20 min
7. **C8**: Replace CIDR string manipulation with net.ParseCIDR + proper IP arithmetic — 30 min

### Phase 2: Fix HIGHs (4-5 hours)
1. **H1+H5**: Convert BuildServiceCatalog to method on AuthService with rows.Err() — 30 min
2. **H2**: Add rows.Err() to auth.go role-fetch loops — 15 min
3. **H3**: Skip original_password when admin resets another user — 20 min
4. **H4**: Defer roleRows.Close() in application_credentials.go — 15 min
5. **H6**: Validate major version == "2" in microversion middleware — 10 min
6. **H7**: ListServersDetail: composite keyset (created_at, id) — 20 min
7. **H8**: Check err != nil after QueryRow in all 5 console handlers — 20 min
8. **H9+H10**: Convert ListSubnets/ListRouters/ListFloatingIPs to marker-based + CapLimit — 90 min
9. **H11**: Add network_id, tenant_id, id to AddRouterInterface response — 15 min
10. **H12+H14**: Add security_group_rules to CreateSG/UpdateSG responses — 20 min
11. **H13**: Use nil for absent optional SG rule fields in CreateSecurityGroupRule — 15 min

### Phase 3: Fix MEDIUMs (3-4 hours)
1. **M1**: Access rules — requires migration + enforcement middleware (~3 hours alone)
2. **M2-M19**: Remaining medium fixes — 2 hours

### Phase 4: Spec compliance tests (ongoing)
- SPEC-000 contract tests, Terraform tests — multi-day effort, not in scope for code fixes

**Estimated total to reach 8/10: 12-15 hours** (Phases 1-3)

---

## Methodology

1. Dispatched 5 per-package Wave 0 agents reading ALL code in each package
2. Each agent reviewed against three design specifications
3. Findings cross-referenced with v4 fix list to identify what was resolved vs. what persists
4. Every finding includes file:line references
5. No assumptions made — every claim verified by reading actual code

---

## Build Status

- `go build ./...` — PASS
- `go vet ./...` — PASS (ignoring test/contract files)
- Unit tests — PASS
- Contract tests — DO NOT EXIST
