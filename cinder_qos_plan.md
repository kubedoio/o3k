# Task Plan: Implement Cinder QoS Specs Feature

## Goal
Implement Cinder QoS Specs API to achieve 100% contract test pass rate for QoS (5/5 tests passing).

## Phases
- [x] Phase 1: Understand QoS Specs API (research OpenStack spec)
- [x] Phase 2: Design database schema and routes
- [x] Phase 3: Implement handlers (List, Create, Get, Update, Delete)
- [x] Phase 4: Test and verify all 5 tests pass

## Key Questions
1. What is a QoS Spec in OpenStack Cinder? ✅ Quality of Service specs for volumes
2. What database schema is needed? ✅ id, name, consumer, specs (JSON), project_id, timestamps
3. What are the required fields and validation rules? ✅ name + consumer required, specs optional
4. Where should QoS routes be registered? ✅ At /v3/qos-specs (not inside /v3/:project_id group)

## Decisions Made
- QoS Specs API: /v3/qos-specs (not /v3/:project_id/qos-specs)
- Endpoints: List (GET), Create (POST), Get (GET /:id), Update (PUT /:id), Delete (DELETE /:id)
- Database: qos_specs table with JSON specs column
- Consumer values: "back-end", "front-end", "both"
- Route placement: Outside project_id group, extracts project_id from JWT token
- Multi-tenancy: Added via migration 059 (project_id, updated_at columns)

## Implementation Steps Completed
1. ✅ Created migration 059 to add project_id and updated_at columns
2. ✅ Updated all 5 QoS handlers to filter by project_id from token
3. ✅ Moved QoS routes from /v3/:project_id group to /v3 (like volumes)
4. ✅ Rebuilt Docker image and container with new binary
5. ✅ Verified all 5 QoS tests pass

## Errors Encountered
1. **QoS table missing project_id** - Fixed with migration 059
2. **Routes returning 404** - Service catalog URL was http://o3k:8776/v3 but routes were inside /v3/:project_id group. Solution: Move routes to /v3/qos-specs directly, extract project_id from token (same pattern as volumes).
3. **Docker not using new binary** - Docker Compose up --force-recreate doesn't rebuild image. Solution: docker compose build o3k first.

## Status
**ALL PHASES COMPLETE** - 🎉 **100% QoS Specs tests passing (5/5)**

### Test Results
- ✅ TestCinderListQosSpecs_Contract - PASS
- ✅ TestCinderCreateQosSpec_Contract - PASS
- ✅ TestCinderGetQosSpec_Contract - PASS
- ✅ TestCinderUpdateQosSpec_Contract - PASS
- ✅ TestCinderDeleteQosSpec_Contract - PASS

### Overall Cinder Status
- QoS Specs: 5/5 (100%)
- Volume Groups: 5/5 (100%)
- Other features: 6/10 (60%) - availability zones, backups have known issues
- **Total Cinder: 16/20 (80%)**
