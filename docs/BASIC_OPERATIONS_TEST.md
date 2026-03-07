# O3K Basic Operations Test Checklist

This document provides a comprehensive checklist for testing basic O3K operations. Use this for regression testing after code changes or deployments.

## Prerequisites

1. O3K server running (Docker Compose or native)
2. OpenStack CLI installed (`pip install python-openstackclient`)
3. Environment variables configured:
   ```bash
   export OS_AUTH_URL=http://localhost:5000/v3
   export OS_PROJECT_DOMAIN_NAME=default
   export OS_USER_DOMAIN_NAME=default
   export OS_PROJECT_NAME=default
   export OS_USERNAME=admin
   export OS_PASSWORD=secret
   export OS_IDENTITY_API_VERSION=3
   ```

## Test Matrix

### 1. Keystone (Identity)

| Test | Command | Expected Result | Status |
|------|---------|----------------|--------|
| Version discovery | `curl http://localhost:5000/v3` | JSON with version info | ⬜ |
| Unscoped auth | `openstack token issue --os-project-name ""` | Token without catalog | ⬜ |
| Scoped auth | `openstack token issue` | Token with service catalog | ⬜ |
| List projects | `openstack project list` | At least 'default' project | ⬜ |
| List users | `openstack user list` | At least 'admin' user | ⬜ |

### 2. Nova (Compute)

| Test | Command | Expected Result | Status |
|------|---------|----------------|--------|
| List flavors | `openstack flavor list` | 5 flavors (m1.tiny - m1.xlarge) | ⬜ |
| List images | `openstack image list` | Available images | ⬜ |
| List servers | `openstack server list` | Current instances or empty list | ⬜ |
| **Create server** | `openstack server create --flavor m1.small --image cirros --network private test-vm` | Server created, no 'metadata' string | ⬜ |
| **Get server (UUID)** | `openstack server show test-vm` | Full server details | ⬜ |
| **Get server (name)** | `openstack server show <uuid>` | Same details as name lookup | ⬜ |
| **Reboot server** | `openstack server reboot test-vm` | Status changes to REBOOT → ACTIVE | ⬜ |
| **Stop server** | `openstack server stop test-vm` | Status=SHUTOFF, power_state=4 | ⬜ |
| **Start server** | `openstack server start test-vm` | Status=ACTIVE, power_state=1 | ⬜ |
| **Delete server** | `openstack server delete test-vm` | Server removed from list | ⬜ |
| List hypervisors | `openstack hypervisor list` | At least one hypervisor | ⬜ |

### 3. Neutron (Network)

| Test | Command | Expected Result | Status |
|------|---------|----------------|--------|
| List networks | `openstack network list` | Existing networks | ⬜ |
| Create network | `openstack network create test-net` | Network created | ⬜ |
| List subnets | `openstack subnet list` | Existing subnets | ⬜ |
| Create subnet | `openstack subnet create --network test-net --subnet-range 10.0.1.0/24 test-subnet` | Subnet created | ⬜ |
| Delete subnet | `openstack subnet delete test-subnet` | Subnet removed | ⬜ |
| Delete network | `openstack network delete test-net` | Network removed | ⬜ |

### 4. Cinder (Block Storage)

| Test | Command | Expected Result | Status |
|------|---------|----------------|--------|
| List volumes | `openstack volume list` | Current volumes or empty list | ⬜ |
| **Create volume** | `openstack volume create --size 1 test-volume` | Volume created, no 'metadata' string | ⬜ |
| **Get volume (UUID)** | `openstack volume show test-volume` | Full volume details | ⬜ |
| **Get volume (name)** | `openstack volume show <uuid>` | Same details as name lookup | ⬜ |
| **Delete volume** | `openstack volume delete test-volume` | Volume removed | ⬜ |
| Verify storage cleanup | `ls ~/.o3k/volumes/` or `ls /var/lib/o3k/volumes/` | Files cleaned up | ⬜ |

### 5. Glance (Image)

| Test | Command | Expected Result | Status |
|------|---------|----------------|--------|
| List images | `openstack image list` | Existing images | ⬜ |
| Create image | `openstack image create --disk-format raw --container-format bare test-image` | Image created (status: queued) | ⬜ |
| Upload image data | `openstack image set --file /path/to/file test-image` | Image status: active | ⬜ |
| Download image | `openstack image save --file /tmp/test.img test-image` | File downloaded | ⬜ |
| Verify data integrity | `md5sum /path/to/file /tmp/test.img` | Checksums match | ⬜ |
| Delete image | `openstack image delete test-image` | Image removed | ⬜ |
| Verify storage cleanup | `ls ~/.o3k/images/` or `ls /var/lib/o3k/images/` | Files cleaned up | ⬜ |

## Critical Regression Tests

These tests verify fixes for previously encountered issues:

### Metadata Field Format
**Issue**: 'metadata' string appearing in CLI output instead of empty dict

| Test | Command | Check |
|------|---------|-------|
| Server create | `openstack server create --flavor m1.small --image cirros --network private meta-test-vm` | Output should NOT contain `'metadata'` as string |
| Volume create | `openstack volume create --size 1 meta-test-vol` | Output should NOT contain `'metadata'` as string |

**Expected Output Pattern**:
```
metadata             | {}
```

**NOT**:
```
metadata             | 'metadata'
```

### Name-Based Lookup
**Issue**: Operations failed when using resource names instead of UUIDs

| Test | Resource | Operation |
|------|----------|-----------|
| Get server by name | Server | `openstack server show <name>` |
| Delete server by name | Server | `openstack server delete <name>` |
| Reboot server by name | Server | `openstack server reboot <name>` |
| Get volume by name | Volume | `openstack volume show <name>` |
| Delete volume by name | Volume | `openstack volume delete <name>` |

All should work identically to UUID-based operations.

### Server Actions in Stub Mode
**Issue**: Server actions (reboot/stop/start) returned 503 errors in stub mode

| Test | Command | Expected Behavior |
|------|---------|------------------|
| Reboot | `openstack server reboot <name>` | Status: REBOOT → ACTIVE (database only) |
| Stop | `openstack server stop <name>` | Status: SHUTOFF, power_state: 4 |
| Start | `openstack server start <name>` | Status: ACTIVE, power_state: 1 |

### Image Upload for Public Images
**Issue**: Image upload returned 404 for public visibility images

| Test | Steps | Expected Result |
|------|-------|----------------|
| Public image upload | 1. `openstack image create --public test-public`<br>2. Upload data | Upload succeeds |
| Private image upload | 1. `openstack image create --private test-private`<br>2. Upload data | Upload succeeds |

### Docker Storage Paths
**Issue**: Image/volume storage directories didn't exist in containers

| Test | Check | Expected |
|------|-------|----------|
| Container paths | `docker exec o3k-lightstack-1 ls -la /var/lib/o3k/` | Directories: `images/`, `volumes/` |
| Permissions | `docker exec o3k-lightstack-1 ls -la /var/lib/o3k/` | Owner: `o3k:o3k` |

## End-to-End Workflow Tests

### Scenario 1: Create VM with Volume
```bash
# 1. Create volume
openstack volume create --size 10 data-volume

# 2. Create network
openstack network create private-net
openstack subnet create --network private-net --subnet-range 192.168.1.0/24 private-subnet

# 3. Create VM
openstack server create --flavor m1.small --image cirros --network private-net --volume data-volume my-vm

# 4. Verify VM created
openstack server show my-vm

# 5. Verify volume attached
openstack volume show data-volume | grep "attached_to_instance_id"

# 6. Stop VM
openstack server stop my-vm

# 7. Start VM
openstack server start my-vm

# 8. Cleanup
openstack server delete my-vm
openstack volume delete data-volume
openstack subnet delete private-subnet
openstack network delete private-net
```

### Scenario 2: Image Upload and Boot
```bash
# 1. Download CirrOS image
wget http://download.cirros-cloud.net/0.6.2/cirros-0.6.2-x86_64-disk.img

# 2. Upload to Glance
openstack image create --disk-format qcow2 --container-format bare --public --file cirros-0.6.2-x86_64-disk.img cirros-test

# 3. Wait for image to be active
openstack image show cirros-test -c status

# 4. Boot from image
openstack server create --flavor m1.small --image cirros-test --network private test-cirros

# 5. Verify boot
openstack server show test-cirros

# 6. Cleanup
openstack server delete test-cirros
openstack image delete cirros-test
rm cirros-0.6.2-x86_64-disk.img
```

## Performance Benchmarks

Expected latency for operations (stub/local mode):

| Operation | Expected Latency | Acceptable Range |
|-----------|-----------------|------------------|
| Authentication | 50ms | < 100ms |
| List resources | 10-15ms | < 50ms |
| Create network | 25ms | < 100ms |
| Create volume (1GB) | 150ms | < 500ms |
| Upload image (1MB) | 80ms | < 200ms |
| Server create | 100ms | < 300ms |
| Server action | 50ms | < 150ms |

## Error Handling Tests

| Test | Command | Expected Error |
|------|---------|---------------|
| Invalid credentials | `openstack --os-password wrong token issue` | HTTP 401 |
| Missing token | `curl http://localhost:8774/v2.1/servers` | HTTP 401 |
| Resource not found | `openstack server show nonexistent-vm` | HTTP 404 |
| Invalid flavor | `openstack server create --flavor invalid ...` | HTTP 400 |
| Attach volume to deleted VM | Try attaching to non-existent instance | HTTP 404 |

## Test Report Template

After running tests, document results:

```markdown
## Test Run: YYYY-MM-DD HH:MM

**Environment**:
- Deployment: [Docker Compose / Native]
- Mode: [Stub / Real]
- O3K Version: [commit hash]
- OS: [Linux / macOS]

**Results**:
- Total Tests: X
- Passed: ✅ X
- Failed: ❌ X
- Skipped: ⏭️ X

**Failed Tests**:
1. [Test name]: [Error description]
2. ...

**Performance Notes**:
- [Any notable performance observations]

**Bugs Found**:
- [New issues discovered]

**Tester**: [Name]
```

## Automated Test Script

A quick test script for basic operations:

```bash
#!/bin/bash
# test/basic_operations.sh

set -e

echo "🧪 O3K Basic Operations Test"
echo "============================"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() {
    echo -e "${GREEN}✓${NC} $1"
}

fail() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

# Test authentication
openstack token issue > /dev/null && pass "Authentication" || fail "Authentication"

# Test server operations
VM_NAME="test-vm-$(date +%s)"
openstack server create --flavor m1.small --image cirros --network private "$VM_NAME" > /dev/null && pass "Create server" || fail "Create server"
openstack server show "$VM_NAME" > /dev/null && pass "Get server" || fail "Get server"
openstack server reboot "$VM_NAME" && pass "Reboot server" || fail "Reboot server"
openstack server stop "$VM_NAME" && pass "Stop server" || fail "Stop server"
openstack server start "$VM_NAME" && pass "Start server" || fail "Start server"
openstack server delete "$VM_NAME" && pass "Delete server" || fail "Delete server"

# Test volume operations
VOL_NAME="test-vol-$(date +%s)"
openstack volume create --size 1 "$VOL_NAME" > /dev/null && pass "Create volume" || fail "Create volume"
openstack volume show "$VOL_NAME" > /dev/null && pass "Get volume" || fail "Get volume"
openstack volume delete "$VOL_NAME" && pass "Delete volume" || fail "Delete volume"

echo ""
echo "✅ All basic operations passed!"
```

Run with:
```bash
chmod +x test/basic_operations.sh
./test/basic_operations.sh
```

---

**Last Updated**: 2026-03-08
**Status**: ✅ All tests passing (v1.0.0)
