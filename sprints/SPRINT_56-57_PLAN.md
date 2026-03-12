# Sprint 56-57: Nova Server Actions (Remaining 8 Actions)

**Sprint Period**: March 2026
**Sprint Goal**: Implement remaining Nova server actions for complete operational coverage
**Priority**: HIGH (completes core Nova functionality)

---

## Objectives

Implement the remaining 8 Nova server actions identified in GAP_ANALYSIS.md:

1. **migrate** - Cold migration to different host
2. **evacuate** - Evacuate server from failed host
3. **changePassword** - Change admin password
4. **createBackup** - Create backup with rotation policy
5. **os-resetState** - Reset server to error state (admin)
6. **os-resetNetwork** - Reset server networking
7. **addSecurityGroup** - Add security group to server
8. **removeSecurityGroup** - Remove security group from server

All actions use `POST /v2.1/:project_id/servers/:id/action` endpoint.

---

## Technical Approach

### 1. Cold Migration (migrate)

**Action Handler**: `MigrateServer()`

```go
// Request body:
{
  "migrate": null  // or {} for microversion 2.56+
}
```

**Implementation**:
- Create migration record with type `migration` (not `live-migration`)
- In stub mode: simulate 5-second migration, update host in database
- In real mode: use libvirt offline migration APIs
- Update server status: available → migrating → active
- Update compute_node association

**Database**:
- Uses existing `server_migrations` table
- Set `migration_type = 'migration'`

### 2. Evacuate Server (evacuate)

**Action Handler**: `EvacuateServer()`

```go
// Request body:
{
  "evacuate": {
    "host": "compute-2",           // optional target host
    "onSharedStorage": false,      // deprecated in 2.14
    "adminPass": "newpassword"     // optional
  }
}
```

**Implementation**:
- Validates source host is down/failed
- Creates migration record with type `evacuation`
- Rebuilds server on new host (similar to rebuild action)
- Stub mode: simulate 3-second evacuation
- Real mode: would use libvirt define + start on new host

**Microversions**:
- 2.14: Remove `onSharedStorage` parameter
- 2.29: Add `force` parameter

### 3. Change Password (changePassword)

**Action Handler**: `ChangeServerPassword()`

```go
// Request body:
{
  "changePassword": {
    "adminPass": "newSecurePassword123"
  }
}
```

**Implementation**:
- Stores hashed password in `instances` table
- In stub mode: just update database
- In real mode: would inject password via cloud-init/guest agent
- No status change required

**Database**:
```sql
ALTER TABLE instances ADD COLUMN admin_password_hash TEXT;
```

### 4. Create Backup (createBackup)

**Action Handler**: `CreateBackupServer()`

```go
// Request body:
{
  "createBackup": {
    "name": "daily-backup",
    "backup_type": "daily",       // daily or weekly
    "rotation": 7                 // keep last N backups
  }
}
```

**Implementation**:
- Creates Glance image with metadata indicating backup
- Applies rotation policy (deletes old backups of same type)
- Similar to `createImage` but with backup semantics
- Returns image_id in response

**Database**:
- Creates entry in `images` table
- Adds metadata: `backup_type`, `rotation`, `source_server_id`

### 5. Reset State (os-resetState)

**Action Handler**: `ResetServerState()`

```go
// Request body:
{
  "os-resetState": {
    "state": "error"  // or "active", "stopped", etc.
  }
}
```

**Implementation**:
- **Admin-only** operation (check roles)
- Directly updates server status in database
- No validation, force state change
- Used to recover from stuck states

**Authorization**: Requires `admin` role

### 6. Reset Network (os-resetNetwork)

**Action Handler**: `ResetServerNetwork()`

```go
// Request body:
{
  "os-resetNetwork": null
}
```

**Implementation**:
- Re-applies security group rules via Neutron
- In stub mode: no-op, return 202
- In real mode: would reset iptables rules, restart network interfaces
- No status change

**Integration**: Calls Neutron service to reapply port security

### 7. Add Security Group (addSecurityGroup)

**Action Handler**: `AddSecurityGroup()`

```go
// Request body:
{
  "addSecurityGroup": {
    "name": "web-servers"  // security group name
  }
}
```

**Implementation**:
- Validates security group exists in Neutron
- Associates security group with server's ports
- Updates Neutron port security_groups list
- Re-applies iptables rules in real mode

**Database**:
```sql
CREATE TABLE IF NOT EXISTS server_security_groups (
  server_id UUID NOT NULL,
  security_group_id UUID NOT NULL,
  PRIMARY KEY (server_id, security_group_id)
);
```

### 8. Remove Security Group (removeSecurityGroup)

**Action Handler**: `RemoveSecurityGroup()`

```go
// Request body:
{
  "removeSecurityGroup": {
    "name": "web-servers"
  }
}
```

**Implementation**:
- Disassociates security group from server
- Updates Neutron port security_groups list
- Cannot remove last security group (OpenStack constraint)

---

## Database Migrations

### Migration 024: Server Actions Enhancement

```sql
-- Add admin password storage
ALTER TABLE instances ADD COLUMN admin_password_hash TEXT;

-- Add security group associations
CREATE TABLE IF NOT EXISTS server_security_groups (
  server_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
  security_group_id UUID NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY (server_id, security_group_id)
);

CREATE INDEX idx_server_security_groups_server ON server_security_groups(server_id);
CREATE INDEX idx_server_security_groups_sg ON server_security_groups(security_group_id);

-- Add backup metadata to images table (if not exists)
-- Images table already exists, just ensure backup fields in metadata
```

---

## Contract Tests

Create `test/contract/nova/server_actions_extended_test.go` with 8 tests:

### 1. TestNovaMigrateServer_Contract
```go
// 1. Create server
// 2. Wait for active status
// 3. POST /servers/:id/action with {"migrate": null}
// 4. Assert 202 Accepted
// 5. Wait for migration to complete
// 6. Verify server is still active
```

### 2. TestNovaEvacuateServer_Contract
```go
// 1. Create server on compute-1
// 2. Mark compute-1 as down (update compute_nodes table)
// 3. POST /servers/:id/action with {"evacuate": {"host": "compute-2"}}
// 4. Assert 200 OK
// 5. Verify server moved to compute-2
```

### 3. TestNovaChangePassword_Contract
```go
// 1. Create server
// 2. POST /servers/:id/action with {"changePassword": {"adminPass": "newpass"}}
// 3. Assert 202 Accepted
// 4. Verify password stored (cannot verify actual password, just check no error)
```

### 4. TestNovaCreateBackup_Contract
```go
// 1. Create server
// 2. POST /servers/:id/action with {"createBackup": {...}}
// 3. Assert 202 Accepted
// 4. Verify backup image created in Glance
// 5. Check backup metadata (backup_type, rotation)
```

### 5. TestNovaResetState_Contract
```go
// 1. Create server
// 2. POST /servers/:id/action with {"os-resetState": {"state": "error"}}
// 3. Assert 202 Accepted (requires admin role)
// 4. Verify server status changed to error
```

### 6. TestNovaResetNetwork_Contract
```go
// 1. Create server with network
// 2. POST /servers/:id/action with {"os-resetNetwork": null}
// 3. Assert 202 Accepted
// 4. Verify no errors (network reset is mostly no-op in stub mode)
```

### 7. TestNovaAddSecurityGroup_Contract
```go
// 1. Create server
// 2. Create security group in Neutron
// 3. POST /servers/:id/action with {"addSecurityGroup": {"name": "test-sg"}}
// 4. Assert 202 Accepted
// 5. Verify security group associated with server
```

### 8. TestNovaRemoveSecurityGroup_Contract
```go
// 1. Create server with 2 security groups
// 2. POST /servers/:id/action with {"removeSecurityGroup": {"name": "sg-1"}}
// 3. Assert 202 Accepted
// 4. Verify security group removed but server still has default group
```

---

## Implementation Plan

### Day 1-2: Database & Migration Actions

**Tasks**:
1. Create migration `024_nova_server_actions.up.sql`
2. Implement `MigrateServer()` handler
3. Implement `EvacuateServer()` handler
4. Write contract tests for migrate & evacuate
5. Test and verify migration workflows

**Files**:
- `migrations/024_nova_server_actions.up.sql`
- `migrations/024_nova_server_actions.down.sql`
- `internal/nova/actions_extended.go` (new file)
- `test/contract/nova/server_actions_extended_test.go`

### Day 3-4: Administrative Actions

**Tasks**:
1. Implement `ChangeServerPassword()` handler
2. Implement `ResetServerState()` handler (with admin check)
3. Implement `ResetServerNetwork()` handler
4. Write contract tests for password, reset state, reset network
5. Test admin role enforcement

**Files**:
- Continue in `internal/nova/actions_extended.go`
- Continue in `test/contract/nova/server_actions_extended_test.go`

### Day 5-6: Security Group Actions & Backup

**Tasks**:
1. Implement `AddSecurityGroup()` handler
2. Implement `RemoveSecurityGroup()` handler
3. Implement `CreateBackupServer()` handler
4. Write contract tests for security groups & backup
5. Integration test with Neutron security groups
6. Test backup rotation logic

**Files**:
- Continue in `internal/nova/actions_extended.go`
- Continue in `test/contract/nova/server_actions_extended_test.go`

### Day 7: Integration & Documentation

**Tasks**:
1. Register all 8 actions in `internal/nova/handlers.go`
2. Run full test suite (all 150+ tests)
3. Update GAP_ANALYSIS.md
4. Create Sprint 56-57 results document
5. Commit and push

---

## Route Registration

Update `internal/nova/handlers.go` in `ServerAction()` method:

```go
func (svc *Service) ServerAction(c *gin.Context) {
    // ... existing code ...

    // Check for new actions
    if _, ok := req["migrate"]; ok {
        svc.MigrateServer(c)
        return
    }

    if _, ok := req["evacuate"]; ok {
        svc.EvacuateServer(c)
        return
    }

    if _, ok := req["changePassword"]; ok {
        svc.ChangeServerPassword(c)
        return
    }

    if _, ok := req["createBackup"]; ok {
        svc.CreateBackupServer(c)
        return
    }

    if _, ok := req["os-resetState"]; ok {
        svc.ResetServerState(c)
        return
    }

    if _, ok := req["os-resetNetwork"]; ok {
        svc.ResetServerNetwork(c)
        return
    }

    if _, ok := req["addSecurityGroup"]; ok {
        svc.AddSecurityGroup(c)
        return
    }

    if _, ok := req["removeSecurityGroup"]; ok {
        svc.RemoveSecurityGroup(c)
        return
    }

    // ... rest of existing actions ...
}
```

---

## Validation Gates

### Gate 1: Contract Tests
- All 8 new contract tests must pass
- Existing Nova tests must still pass (no regressions)

### Gate 2: Integration with Neutron
- Security group add/remove must correctly update Neutron ports
- Test with real Neutron API calls

### Gate 3: Admin Authorization
- Verify `os-resetState` requires admin role
- Non-admin users get 403 Forbidden

### Gate 4: Backup Rotation
- Create 8 daily backups, verify only last 7 remain
- Test weekly backup rotation separately

### Gate 5: Full Test Suite
```bash
go test ./test/contract/... -count=1
# Expected: 158+ tests pass (150 existing + 8 new)
```

---

## Success Metrics

**Quantitative**:
- 8 new Nova server actions implemented
- 8 new contract tests (100% pass rate)
- API coverage: 79.4% → 81.8% (+2.4%)
- Nova coverage: 80% → 86% (+6%)

**Qualitative**:
- Complete Nova operational action coverage
- Full backup/restore workflow functional
- Security group management from compute API
- Admin troubleshooting capabilities (reset state/network)

---

## Risk Mitigation

**Risk 1: Neutron Integration Complexity**
- Mitigation: Start with stub mode (no actual Neutron calls), add integration in follow-up sprint

**Risk 2: Backup Rotation Logic**
- Mitigation: Test with extensive fixtures (create 20 backups, verify cleanup)

**Risk 3: Admin Authorization**
- Mitigation: Reuse existing `checkAdminRole()` middleware pattern from Keystone

**Risk 4: Migration Conflicts with Sprint 54-55**
- Mitigation: Cold migration uses different status flow than live migration, minimal overlap

---

## Dependencies

### External
- Neutron service (for security group lookups)
- Glance service (for backup image creation)

### Internal
- Existing server_migrations table (Sprint 54-55)
- Existing images table
- Auth middleware for admin role checks

---

## Follow-up Work (Sprint 58-59)

After completing Sprint 56-57:
1. **Console Access** (4 endpoints) - VNC/SPICE/Serial/RDP console URLs
2. **Tenant Usage** (3 endpoints) - Billing/metering data
3. **Availability Zones** (4 endpoints) - AZ management

---

## Timeline Estimate

- **Optimistic**: 5 days (1 day per 1-2 actions)
- **Realistic**: 7 days (including integration testing)
- **Conservative**: 10 days (buffer for Neutron integration issues)

**Recommendation**: Plan for 7 days, track against 5-day goal.

---

## Deliverables Checklist

- [ ] Migration 024 created and tested
- [ ] 8 action handlers implemented in `actions_extended.go`
- [ ] 8 contract tests written and passing
- [ ] Route registration updated in `handlers.go`
- [ ] Admin role check for `os-resetState`
- [ ] Neutron integration for security groups
- [ ] Backup rotation logic tested
- [ ] Full test suite passes (158+ tests)
- [ ] GAP_ANALYSIS.md updated
- [ ] Sprint 56-57 results document created
- [ ] All changes committed and pushed

---

**Ready to begin implementation?** Use `/implement` to start Sprint 56-57.
