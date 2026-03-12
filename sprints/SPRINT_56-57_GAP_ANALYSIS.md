# Sprint 56-57 Gap Analysis: Nova Server Actions Implementation Status

**Analysis Date**: 2026-03-12
**Status**: Implementations exist but are incomplete stubs
**Files Analyzed**:
- `internal/nova/advanced_actions.go` (implementations)
- `internal/nova/handlers.go` (route registration)
- `test/contract/nova/advanced_server_actions_test.go` (tests)
- `migrations/049_nova_server_actions.{up,down}.sql` (database schema)

---

## Executive Summary

**All 8 server actions are registered and have handler implementations, but ALL are incomplete stubs.**

- ✅ **Route registration**: All 8 actions properly registered in `handlers.go`
- ✅ **Database migration**: Migration 049 applied successfully (admin_password_hash column + server_security_groups table)
- ✅ **Contract tests**: 4 tests exist (createBackup, os-resetState, os-resetNetwork, restore)
- ⚠️ **Implementations**: ALL handlers are incomplete stubs that need enhancement
- ❌ **Missing tests**: No tests for migrate, evacuate, changePassword, addSecurityGroup, removeSecurityGroup

**Completion Estimate**: 40-60% complete (routing/schema done, logic incomplete)

---

## Detailed Gap Analysis by Action

### 1. migrate - Cold Migration ⚠️ INCOMPLETE (30% done)

**File**: `internal/nova/advanced_actions.go:423-454`

**What EXISTS**:
- ✅ Validates instance exists
- ✅ Validates instance status is ACTIVE or STOPPED
- ✅ Updates task_state to "migrating"
- ✅ Returns 202 Accepted

**What is MISSING**:
- ❌ No migration record created in `server_migrations` table
- ❌ No host selection logic (should choose different host from current)
- ❌ No actual host change in database (`UPDATE instances SET host = ...`)
- ❌ No background goroutine to complete migration and reset task_state
- ❌ No integration with compute node registry
- ❌ No status transitions (migrating → verifying_resize → active)

**Required Additions**:
```go
// After validation, add:
1. SELECT compute_nodes WHERE id != current_host AND available = true
2. INSERT INTO server_migrations (instance_id, source_host, dest_host, migration_type='migration')
3. UPDATE instances SET host = dest_host, task_state = 'migrating'
4. Background goroutine: sleep 5s (stub), UPDATE instances SET task_state = NULL, status = 'ACTIVE'
```

**Contract Test**: ❌ MISSING - needs `TestNovaMigrateServer_Contract`

---

### 2. evacuate - Host Evacuation ⚠️ INCOMPLETE (20% done)

**File**: `internal/nova/advanced_actions.go:394-420`

**What EXISTS**:
- ✅ Validates instance exists
- ✅ Updates status to "evacuating"
- ✅ Returns 200 OK

**What is MISSING**:
- ❌ No request body parsing (missing `host`, `adminPass`, `onSharedStorage` parameters)
- ❌ No validation that source host is down/failed
- ❌ No migration record created with type = 'evacuation'
- ❌ No actual host change in database
- ❌ No adminPass handling (should return in response if not provided)
- ❌ No microversion handling (2.14+ removes onSharedStorage)
- ❌ No rebuild logic on new host

**Required Additions**:
```go
1. Parse request body: {"evacuate": {"host": "...", "adminPass": "..."}}
2. Validate source host is down: SELECT status FROM compute_nodes WHERE host = current_host (status = 'down')
3. INSERT INTO server_migrations (instance_id, source_host, dest_host, migration_type='evacuation')
4. UPDATE instances SET host = dest_host, status = 'REBUILDING'
5. Background: sleep 3s, UPDATE status = 'ACTIVE'
6. Return adminPass in response body if generated
```

**Contract Test**: ❌ MISSING - needs `TestNovaEvacuateServer_Contract`

---

### 3. changePassword - Admin Password Change ⚠️ INCOMPLETE (40% done)

**File**: `internal/nova/advanced_actions.go:623-667`

**What EXISTS**:
- ✅ Validates instance exists
- ✅ Validates instance status is ACTIVE
- ✅ Parses `adminPass` from request body
- ✅ Returns 202 Accepted

**What is MISSING**:
- ❌ No password hashing (should use bcrypt like Keystone)
- ❌ No UPDATE to `instances.admin_password_hash` column
- ❌ No import of `golang.org/x/crypto/bcrypt`
- ❌ No error handling for weak passwords

**Required Additions**:
```go
import "golang.org/x/crypto/bcrypt"

// After parsing adminPass:
1. Validate password strength (min 8 chars)
2. hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
3. UPDATE instances SET admin_password_hash = $1, updated_at = $2 WHERE id = $3
```

**Database Column**: ✅ EXISTS (migration 049 added `admin_password_hash TEXT`)

**Contract Test**: ❌ MISSING - needs `TestNovaChangePassword_Contract`

---

### 4. createBackup - Backup Creation with Rotation ⚠️ INCOMPLETE (30% done)

**File**: `internal/nova/advanced_actions.go:707-753`

**What EXISTS**:
- ✅ Validates instance exists
- ✅ Parses `name`, `backup_type`, `rotation` from request body
- ✅ Returns 202 Accepted

**What is MISSING**:
- ❌ No image creation in `images` table
- ❌ No backup metadata stored (backup_type, rotation, source_server_id)
- ❌ No rotation logic (delete old backups when count > rotation)
- ❌ No Location header with image URL
- ❌ No actual image_id returned

**Required Additions**:
```go
1. Generate new UUID for image_id
2. INSERT INTO images (id, name, status, container_format, disk_format, owner, created_at, properties)
   - properties JSON should include: {"backup_type": "daily", "rotation": 3, "source_server_id": "..."}
3. Query old backups: SELECT id FROM images WHERE properties->>'backup_type' = $1 AND properties->>'source_server_id' = $2 ORDER BY created_at DESC
4. Delete backups beyond rotation limit: DELETE FROM images WHERE id IN (... old backups ...)
5. Set Location header: c.Header("Location", fmt.Sprintf("/v2/images/%s", imageID))
6. Return image_id in response body (microversion 2.45+)
```

**Contract Test**: ✅ EXISTS (`TestNovaCreateBackup_Contract`) but expects Location header

---

### 5. os-resetState - Force State Change ⚠️ INCOMPLETE (70% done)

**File**: `internal/nova/advanced_actions.go:756-811`

**What EXISTS**:
- ✅ Validates instance exists
- ✅ Parses `state` from request body
- ✅ Converts lowercase to uppercase (error → ERROR, active → ACTIVE)
- ✅ Updates instances.status in database
- ✅ Returns 202 Accepted

**What is MISSING**:
- ❌ No admin role check (should require admin role)
- ❌ Allows any user to reset state (security issue)

**Required Additions**:
```go
// At start of function, add:
roles := c.GetStringSlice("roles")
isAdmin := false
for _, role := range roles {
    if role == "admin" {
        isAdmin = true
        break
    }
}
if !isAdmin {
    c.JSON(http.StatusForbidden, gin.H{
        "forbidden": gin.H{
            "message": "Policy doesn't allow os-resetState to be performed",
            "code": 403,
        },
    })
    return
}
```

**Contract Test**: ✅ EXISTS (`TestNovaResetState_Contract`) but doesn't verify admin-only

---

### 6. os-resetNetwork - Network Reset ✅ COMPLETE (stub mode)

**File**: `internal/nova/advanced_actions.go:814-833`

**What EXISTS**:
- ✅ Validates instance exists
- ✅ Returns 202 Accepted
- ✅ Comment acknowledges stub mode limitation

**What is MISSING** (acceptable for stub mode):
- Real mode only: Would reset network interfaces via libvirt/netlink
- Real mode only: Would reapply security group rules

**Status**: ✅ **COMPLETE for stub mode** - no changes needed

**Contract Test**: ✅ EXISTS (`TestNovaResetNetwork_Contract`)

---

### 7. addSecurityGroup - Add Security Group ⚠️ INCOMPLETE (50% done)

**File**: `internal/nova/advanced_actions.go:517-567`

**What EXISTS**:
- ✅ Parses security group `name` from request body
- ✅ Validates instance exists and is not deleted
- ✅ Validates security group exists in Neutron
- ✅ Returns 202 Accepted

**What is MISSING**:
- ❌ No INSERT into `server_security_groups` table
- ❌ No duplicate check (should prevent adding same SG twice)
- ❌ No Neutron port update (ports should have updated security_groups list)
- ❌ Real mode: No iptables rule application

**Required Additions**:
```go
// After validating SG exists:
1. Check if already associated:
   SELECT EXISTS(SELECT 1 FROM server_security_groups WHERE server_id = $1 AND security_group_id = $2)
   If exists, return 409 Conflict
2. INSERT INTO server_security_groups (server_id, security_group_id, created_at)
   VALUES ($1, $2, NOW())
3. Update Neutron ports:
   SELECT port_id FROM ports WHERE device_id = instance_id
   For each port: UPDATE ports SET security_groups = array_append(security_groups, sg_id)
```

**Database Table**: ✅ EXISTS (migration 049 created `server_security_groups`)

**Contract Test**: ❌ MISSING - needs `TestNovaAddSecurityGroup_Contract`

---

### 8. removeSecurityGroup - Remove Security Group ⚠️ INCOMPLETE (50% done)

**File**: `internal/nova/advanced_actions.go:570-620`

**What EXISTS**:
- ✅ Parses security group `name` from request body
- ✅ Validates instance exists and is not deleted
- ✅ Validates security group exists in Neutron
- ✅ Returns 202 Accepted

**What is MISSING**:
- ❌ No DELETE from `server_security_groups` table
- ❌ No check if SG is actually associated (should return 404 if not)
- ❌ No check for last security group (OpenStack doesn't allow removing last SG)
- ❌ No Neutron port update (ports should have updated security_groups list)
- ❌ Real mode: No iptables rule removal

**Required Additions**:
```go
// After validating SG exists:
1. Check if associated:
   SELECT EXISTS(SELECT 1 FROM server_security_groups WHERE server_id = $1 AND security_group_id = $2)
   If not exists, return 404 Not Found
2. Check if last SG:
   SELECT COUNT(*) FROM server_security_groups WHERE server_id = $1
   If count = 1, return 400 Bad Request "Cannot remove last security group"
3. DELETE FROM server_security_groups WHERE server_id = $1 AND security_group_id = $2
4. Update Neutron ports:
   SELECT port_id FROM ports WHERE device_id = instance_id
   For each port: UPDATE ports SET security_groups = array_remove(security_groups, sg_id)
```

**Database Table**: ✅ EXISTS (migration 049 created `server_security_groups`)

**Contract Test**: ❌ MISSING - needs `TestNovaRemoveSecurityGroup_Contract`

---

## Summary Table

| Action | Routing | DB Schema | Implementation | Contract Test | Completion |
|--------|---------|-----------|----------------|---------------|------------|
| **migrate** | ✅ | ✅ (reuses server_migrations) | ⚠️ Stub only | ❌ | 30% |
| **evacuate** | ✅ | ✅ (reuses server_migrations) | ⚠️ Stub only | ❌ | 20% |
| **changePassword** | ✅ | ✅ (admin_password_hash) | ⚠️ No hashing | ❌ | 40% |
| **createBackup** | ✅ | ✅ (reuses images) | ⚠️ No rotation | ✅ | 30% |
| **os-resetState** | ✅ | ✅ (no schema needed) | ⚠️ No admin check | ✅ | 70% |
| **os-resetNetwork** | ✅ | ✅ (no schema needed) | ✅ Complete (stub) | ✅ | 100% |
| **addSecurityGroup** | ✅ | ✅ (server_security_groups) | ⚠️ No insert | ❌ | 50% |
| **removeSecurityGroup** | ✅ | ✅ (server_security_groups) | ⚠️ No delete | ❌ | 50% |

**Overall Sprint 56-57 Completion**: ~45% (routing + schema done, logic incomplete)

---

## Priority for Completion

### Priority 1: Security Issue (URGENT)
- **os-resetState**: Add admin role check (currently allows any user)

### Priority 2: Database Operations (HIGH)
- **changePassword**: Add bcrypt hashing and database UPDATE
- **addSecurityGroup**: Add INSERT to server_security_groups + Neutron port update
- **removeSecurityGroup**: Add DELETE from server_security_groups + Neutron port update

### Priority 3: Advanced Logic (MEDIUM)
- **createBackup**: Add image creation + rotation logic
- **migrate**: Add migration record creation + host change
- **evacuate**: Add request parsing + host validation + migration record

### Priority 4: Contract Tests (MEDIUM)
- Create 5 missing tests: migrate, evacuate, changePassword, addSecurityGroup, removeSecurityGroup

---

## Effort Estimate

| Task | Estimated Time | Complexity |
|------|----------------|------------|
| os-resetState admin check | 30 min | Low |
| changePassword full implementation | 1 hour | Low |
| addSecurityGroup full implementation | 2 hours | Medium |
| removeSecurityGroup full implementation | 2 hours | Medium |
| createBackup full implementation | 3 hours | Medium |
| migrate full implementation | 4 hours | High |
| evacuate full implementation | 4 hours | High |
| Contract tests (5 tests) | 3 hours | Medium |
| **TOTAL** | **~19.5 hours** | **~2.5 days** |

---

## Recommended Implementation Order

### Day 1: Security + Password + Tests Foundation
1. Fix os-resetState admin check (30 min)
2. Complete changePassword implementation (1 hour)
3. Write changePassword contract test (30 min)
4. Test and verify (30 min)

### Day 2: Security Groups
1. Complete addSecurityGroup implementation (2 hours)
2. Complete removeSecurityGroup implementation (2 hours)
3. Write both contract tests (1.5 hours)
4. Test and verify (30 min)

### Day 3: Backup + Migration
1. Complete createBackup implementation (3 hours)
2. Complete migrate implementation (4 hours)
3. Write migrate contract test (30 min)

### Day 4: Evacuation + Final Testing
1. Complete evacuate implementation (4 hours)
2. Write evacuate contract test (30 min)
3. Run full test suite (30 min)
4. Update documentation (1 hour)

**Total**: 4 days with buffer for debugging/testing

---

## Files to Modify

1. **internal/nova/advanced_actions.go** - All 7 incomplete implementations
2. **test/contract/nova/advanced_server_actions_test.go** - Add 5 missing tests
3. **internal/nova/advanced_actions.go** - Add `import "golang.org/x/crypto/bcrypt"` at top
4. **GAP_ANALYSIS.md** - Update after completion
5. **SPRINT_56-57_PLAN.md** - Mark tasks as complete

---

## No Breaking Changes Required

All enhancements are additive:
- No API signature changes
- No database migration changes (schema already correct)
- No route changes (already registered)
- Existing tests will continue to pass

This is purely **filling in stubbed implementations** with real logic.

---

## Next Steps

1. **Review this gap analysis** with team
2. **Approve implementation priority** (security fix first?)
3. **Start Day 1 tasks** (os-resetState admin check + changePassword)
4. **TDD approach**: Write contract test → confirm RED → implement → confirm GREEN

**Ready to proceed?** Use `/implement` to start Day 1 tasks.
