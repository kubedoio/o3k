#!/bin/bash
set -e

echo "=== VM Networking Integration Test ==="
echo "Requires: Linux, KVM, o3k running in real mode"

# Check prerequisites
if [ "$(uname)" != "Linux" ]; then
    echo "SKIP: Not on Linux"
    exit 0
fi

if ! virsh version > /dev/null 2>&1; then
    echo "SKIP: libvirt not available"
    exit 0
fi

if ! curl -s http://localhost:35357/v3 > /dev/null; then
    echo "SKIP: o3k not running"
    exit 0
fi

# Source credentials
export OS_AUTH_URL=http://localhost:35357/v3
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default
export OS_USER_DOMAIN_NAME=Default
export OS_PROJECT_DOMAIN_NAME=Default
export OS_IDENTITY_API_VERSION=3

echo "Step 1: Create network"
NET_ID=$(openstack network create test-net -f json | jq -r '.id')
echo "  Network: $NET_ID"

echo "Step 2: Create subnet with DHCP"
SUBNET_ID=$(openstack subnet create --network $NET_ID --subnet-range 192.168.100.0/24 --gateway 192.168.100.1 test-subnet -f json | jq -r '.id')
echo "  Subnet: $SUBNET_ID"

echo "Step 3: Verify bridge exists"
BRIDGE_NAME="br-${NET_ID:0:8}"
if ip link show $BRIDGE_NAME > /dev/null 2>&1; then
    echo "  OK: Bridge $BRIDGE_NAME exists"
else
    echo "  WARN: Bridge $BRIDGE_NAME not found (expected in real mode)"
fi

echo "Step 4: Create server"
SERVER_ID=$(openstack server create --network $NET_ID --flavor m1.tiny --image cirros test-vm -f json | jq -r '.id')
echo "  Server: $SERVER_ID"

echo "Step 5: Wait for ACTIVE (max 60s)"
for i in $(seq 1 30); do
    STATUS=$(openstack server show $SERVER_ID -f json | jq -r '.status')
    if [ "$STATUS" = "ACTIVE" ]; then
        echo "  Server is ACTIVE"
        break
    elif [ "$STATUS" = "ERROR" ]; then
        echo "  FAIL: Server went to ERROR"
        openstack server show $SERVER_ID -f json | jq '.fault'
        break
    fi
    sleep 2
done

if [ "$STATUS" != "ACTIVE" ]; then
    echo "  FAIL: Server did not become ACTIVE (status: $STATUS)"
fi

echo "Step 6: Check port allocation"
PORT_INFO=$(openstack port list --server $SERVER_ID -f json 2>/dev/null || echo "[]")
PORT_COUNT=$(echo $PORT_INFO | jq '. | length')
echo "  Ports allocated: $PORT_COUNT"

if [ "$PORT_COUNT" -ge 1 ]; then
    MAC=$(echo $PORT_INFO | jq -r '.[0]["MAC Address"]')
    IP=$(echo $PORT_INFO | jq -r '.[0]["Fixed IP Addresses"]' | grep -oP '\d+\.\d+\.\d+\.\d+' | head -1)
    echo "  MAC: $MAC"
    echo "  IP: $IP"
fi

echo "Step 7: Cleanup"
openstack server delete $SERVER_ID 2>/dev/null || true
sleep 3
openstack subnet delete $SUBNET_ID 2>/dev/null || true
openstack network delete $NET_ID 2>/dev/null || true

if [ "$STATUS" = "ACTIVE" ] && [ "$PORT_COUNT" -ge 1 ]; then
    echo ""
    echo "=== PASS ==="
    exit 0
else
    echo ""
    echo "=== FAIL (status=$STATUS, ports=$PORT_COUNT) ==="
    exit 1
fi
