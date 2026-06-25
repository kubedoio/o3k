# Task Plan: O3K Bootstrap — Kolla-style Post-Install Setup

## Goal
After `curl -sfL https://get.o3k.io | sh -` completes, o3k automatically has a
working network, a CirrOS image, and a running test VM — ready to use like a
fresh Kolla install.

## Phases
- [x] Phase 1: Understand current state
- [x] Phase 2: Design the bootstrap sequence
- [ ] Phase 3: Implement bootstrap script + installer integration
- [ ] Phase 4: Validate end-to-end on the server

## Decisions Made
- Bootstrap is a standalone shell script (`scripts/bootstrap.sh`) called from
  `install.sh` after the service is confirmed ready
- Uses `openstack` CLI (already installed by installer) via the generated openrc
- CirrOS 0.6.2 from official release URL (UEFI-compatible, ~20MB qcow2)
- Network: `default-net` / `192.168.100.0/24`, router `default-router`
- Flavor: `m1.tiny` (already seeded)
- Image: `cirros-0.6.2` uploaded to Glance
- VM: `test-vm` on m1.tiny + cirros + default-net
- Idempotent: skip steps where resource already exists
- Errors: hard-fail on auth, warn-and-continue on individual resources
- O3K_NO_BOOTSTRAP=true to skip

## Status
**Currently in Phase 3** - Implementing bootstrap script
