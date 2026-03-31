# Task Plan: Fix Cinder Availability Zones and Backups Test Failures

## Goal
Fix 4 failing Cinder contract tests (1 availability zone, 3 backups) to achieve 100% pass rate for these specific endpoints.

## Phases
- [x] Phase 1: Understand what availability zones and backups endpoints need
- [x] Phase 2: Diagnose route placement issue
- [x] Phase 3: Fix route registration (move outside project_id group)
- [x] Phase 4: Verify all 4 tests pass

## Key Questions
1. What do availability zones return in OpenStack? ✅ Simple list with zone names and availability status
2. What are the backup endpoints and their expected responses? ✅ Full CRUD + restore operations
3. Are these stub implementations or need real functionality? ✅ Handlers already implemented, just route placement issue

## Decisions Made
- **Root Cause**: Routes were registered inside `/v3/:project_id` group but service catalog URL is `/v3` without project_id
- **Solution**: Move routes to `/v3` root level and extract project_id from JWT token (same as QoS Specs pattern)
- **Routes moved**: Backups (6 routes), Availability zones (1 route)

## Errors Encountered
- **Routes returning 404**: Handlers existed but routes were inside `/v3/:project_id` group. Service catalog URL `http://localhost:8776/v3` + `backups` = `/v3/backups`, but routes expected `/v3/:project_id/backups`.
- **Solution**: Moved routes to `/v3` root, same pattern as volumes and QoS specs.

## Status
**ALL PHASES COMPLETE** - 🎉 **100% of target tests passing (4/4)**

### Test Results
- ✅ TestCinderListAvailabilityZones_Contract - PASS
- ✅ TestCinderListBackups_Contract - PASS
- ✅ TestCinderCreateBackup_Contract - PASS
- ✅ TestCinderGetBackup_Contract - PASS
- ✅ (Bonus) TestCinderDeleteBackup_Contract - PASS
- ✅ (Bonus) TestCinderRestoreBackup_Contract - PASS

### Implementation
All handlers already existed in:
- `internal/cinder/availability_zones.go` - Returns default "nova" zone
- `internal/cinder/backups.go` - Full CRUD + restore for volume backups

Only needed to fix route placement by moving from `/v3/:project_id` group to `/v3` root level.
