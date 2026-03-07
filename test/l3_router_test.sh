#!/bin/bash
# O3K L3 Router API Test (macOS Compatible - Stub Mode)
# Tests router and floating IP API endpoints without requiring Linux networking

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0

O3K_URL="http://localhost:35357"
NEUTRON_URL="http://localhost:9696"

print_test() {
    echo -e "\n${YELLOW}[TEST]${NC} $1"
}

print_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    PASSED=$((PASSED + 1))
}

print_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    FAILED=$((FAILED + 1))
}

print_info() {
    echo -e "  ℹ  $1"
}

# Test 1: Authenticate
print_test "Authentication"

response=$(curl -s -i -X POST "${O3K_URL}/v3/auth/tokens" \
    -H "Content-Type: application/json" \
    -d '{
        "auth": {
            "identity": {
                "methods": ["password"],
                "password": {
                    "user": {
                        "name": "admin",
                        "domain": {"name": "Default"},
                        "password": "secret"
                    }
                }
            },
            "scope": {
                "project": {
                    "name": "default",
                    "domain": {"name": "Default"}
                }
            }
        }
    }')

TOKEN=$(echo "$response" | grep -i "X-Subject-Token:" | awk '{print $2}' | tr -d '\r')

if [ -n "$TOKEN" ]; then
    print_pass "Authenticated successfully"
    print_info "Token: ${TOKEN:0:50}..."
else
    print_fail "Authentication failed"
    exit 1
fi

export TOKEN

# Test 2: Create Network (prerequisite for router)
print_test "Create Test Network"

network_response=$(curl -s -X POST "${NEUTRON_URL}/v2.0/networks" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "network": {
            "name": "test-network",
            "admin_state_up": true
        }
    }')

NETWORK_ID=$(echo "$network_response" | jq -r '.network.id')

if [ "$NETWORK_ID" != "null" ] && [ -n "$NETWORK_ID" ]; then
    print_pass "Created network: $NETWORK_ID"
else
    print_fail "Failed to create network"
    echo "$network_response" | jq '.'
fi

# Test 3: Create Subnet
print_test "Create Test Subnet"

subnet_response=$(curl -s -X POST "${NEUTRON_URL}/v2.0/subnets" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "subnet": {
            "name": "test-subnet",
            "network_id": "'$NETWORK_ID'",
            "cidr": "10.0.1.0/24",
            "ip_version": 4,
            "gateway_ip": "10.0.1.1"
        }
    }')

SUBNET_ID=$(echo "$subnet_response" | jq -r '.subnet.id')

if [ "$SUBNET_ID" != "null" ] && [ -n "$SUBNET_ID" ]; then
    print_pass "Created subnet: $SUBNET_ID"
else
    print_fail "Failed to create subnet"
fi

# Test 4: Create External Network
print_test "Create External Network"

ext_network_response=$(curl -s -X POST "${NEUTRON_URL}/v2.0/networks" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "network": {
            "name": "external-network",
            "admin_state_up": true
        }
    }')

EXT_NETWORK_ID=$(echo "$ext_network_response" | jq -r '.network.id')

if [ "$EXT_NETWORK_ID" != "null" ] && [ -n "$EXT_NETWORK_ID" ]; then
    print_pass "Created external network: $EXT_NETWORK_ID"
else
    print_fail "Failed to create external network"
fi

# Test 5: Create External Subnet
print_test "Create External Subnet"

ext_subnet_response=$(curl -s -X POST "${NEUTRON_URL}/v2.0/subnets" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "subnet": {
            "name": "external-subnet",
            "network_id": "'$EXT_NETWORK_ID'",
            "cidr": "203.0.113.0/24",
            "ip_version": 4,
            "gateway_ip": "203.0.113.1"
        }
    }')

EXT_SUBNET_ID=$(echo "$ext_subnet_response" | jq -r '.subnet.id')

if [ "$EXT_SUBNET_ID" != "null" ] && [ -n "$EXT_SUBNET_ID" ]; then
    print_pass "Created external subnet: $EXT_SUBNET_ID"
else
    print_fail "Failed to create external subnet"
fi

# Test 6: List Routers (should be empty initially)
print_test "List Routers (Initial)"

routers=$(curl -s -H "X-Auth-Token: $TOKEN" "${NEUTRON_URL}/v2.0/routers")

if echo "$routers" | jq -e '.routers' > /dev/null; then
    router_count=$(echo "$routers" | jq '.routers | length')
    print_pass "Listed $router_count routers"
else
    print_fail "Failed to list routers"
fi

# Test 7: Create Router
print_test "Create Router"

router_response=$(curl -s -X POST "${NEUTRON_URL}/v2.0/routers" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "router": {
            "name": "test-router",
            "admin_state_up": true
        }
    }')

ROUTER_ID=$(echo "$router_response" | jq -r '.router.id')
ROUTER_STATUS=$(echo "$router_response" | jq -r '.router.status')

if [ "$ROUTER_ID" != "null" ] && [ -n "$ROUTER_ID" ]; then
    print_pass "Created router: $ROUTER_ID"
    print_info "Status: $ROUTER_STATUS"
else
    print_fail "Failed to create router"
    echo "$router_response" | jq '.'
fi

# Test 8: Get Router Details
print_test "Get Router Details"

router_details=$(curl -s -H "X-Auth-Token: $TOKEN" "${NEUTRON_URL}/v2.0/routers/$ROUTER_ID")

if echo "$router_details" | jq -e '.router.id' > /dev/null; then
    router_name=$(echo "$router_details" | jq -r '.router.name')
    print_pass "Retrieved router: $router_name"
else
    print_fail "Failed to get router details"
fi

# Test 9: Update Router (Set External Gateway)
print_test "Update Router - Set External Gateway"

update_response=$(curl -s -X PUT "${NEUTRON_URL}/v2.0/routers/$ROUTER_ID" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "router": {
            "external_gateway_info": {
                "network_id": "'$EXT_NETWORK_ID'",
                "enable_snat": true
            }
        }
    }')

if echo "$update_response" | jq -e '.router.external_gateway_info' > /dev/null; then
    print_pass "Set external gateway on router"
    gateway_info=$(echo "$update_response" | jq -r '.router.external_gateway_info')
    print_info "Gateway: $gateway_info"
else
    print_fail "Failed to set external gateway"
fi

# Test 10: Add Router Interface
print_test "Add Router Interface (Attach Subnet)"

interface_response=$(curl -s -X PUT "${NEUTRON_URL}/v2.0/routers/$ROUTER_ID/add_router_interface" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "subnet_id": "'$SUBNET_ID'"
    }')

INTERFACE_PORT_ID=$(echo "$interface_response" | jq -r '.port_id')

if [ "$INTERFACE_PORT_ID" != "null" ] && [ -n "$INTERFACE_PORT_ID" ]; then
    print_pass "Attached subnet to router"
    print_info "Interface port: $INTERFACE_PORT_ID"
else
    print_fail "Failed to attach subnet to router"
    echo "$interface_response" | jq '.'
fi

# Test 11: List Floating IPs (should be empty)
print_test "List Floating IPs (Initial)"

floatingips=$(curl -s -H "X-Auth-Token: $TOKEN" "${NEUTRON_URL}/v2.0/floatingips")

if echo "$floatingips" | jq -e '.floatingips' > /dev/null; then
    fip_count=$(echo "$floatingips" | jq '.floatingips | length')
    print_pass "Listed $fip_count floating IPs"
else
    print_fail "Failed to list floating IPs"
fi

# Test 12: Create Floating IP
print_test "Create Floating IP"

fip_response=$(curl -s -X POST "${NEUTRON_URL}/v2.0/floatingips" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "floatingip": {
            "floating_network_id": "'$EXT_NETWORK_ID'",
            "description": "Test floating IP"
        }
    }')

FIP_ID=$(echo "$fip_response" | jq -r '.floatingip.id')
FIP_ADDRESS=$(echo "$fip_response" | jq -r '.floatingip.floating_ip_address')
FIP_STATUS=$(echo "$fip_response" | jq -r '.floatingip.status')

if [ "$FIP_ID" != "null" ] && [ -n "$FIP_ID" ]; then
    print_pass "Created floating IP: $FIP_ADDRESS"
    print_info "ID: $FIP_ID"
    print_info "Status: $FIP_STATUS"
else
    print_fail "Failed to create floating IP"
    echo "$fip_response" | jq '.'
fi

# Test 13: Get Floating IP Details
print_test "Get Floating IP Details"

fip_details=$(curl -s -H "X-Auth-Token: $TOKEN" "${NEUTRON_URL}/v2.0/floatingips/$FIP_ID")

if echo "$fip_details" | jq -e '.floatingip.id' > /dev/null; then
    print_pass "Retrieved floating IP details"
else
    print_fail "Failed to get floating IP details"
fi

# Test 14: Create a Port (to associate with floating IP)
print_test "Create Port for Floating IP Association"

port_response=$(curl -s -X POST "${NEUTRON_URL}/v2.0/ports" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "port": {
            "name": "test-vm-port",
            "network_id": "'$NETWORK_ID'",
            "fixed_ips": [
                {
                    "subnet_id": "'$SUBNET_ID'",
                    "ip_address": "10.0.1.10"
                }
            ]
        }
    }')

PORT_ID=$(echo "$port_response" | jq -r '.port.id')

if [ "$PORT_ID" != "null" ] && [ -n "$PORT_ID" ]; then
    print_pass "Created port: $PORT_ID"
else
    print_fail "Failed to create port"
fi

# Test 15: Associate Floating IP with Port
print_test "Associate Floating IP with Port"

assoc_response=$(curl -s -X PUT "${NEUTRON_URL}/v2.0/floatingips/$FIP_ID" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "floatingip": {
            "port_id": "'$PORT_ID'",
            "fixed_ip_address": "10.0.1.10"
        }
    }')

ASSOC_STATUS=$(echo "$assoc_response" | jq -r '.floatingip.status')
ASSOC_FIXED_IP=$(echo "$assoc_response" | jq -r '.floatingip.fixed_ip_address')

if [ "$ASSOC_STATUS" = "ACTIVE" ]; then
    print_pass "Associated floating IP with port"
    print_info "Floating IP: $FIP_ADDRESS → Fixed IP: $ASSOC_FIXED_IP"
    print_info "Status: $ASSOC_STATUS"
else
    print_fail "Failed to associate floating IP"
    echo "$assoc_response" | jq '.'
fi

# Test 16: Disassociate Floating IP
print_test "Disassociate Floating IP"

disassoc_response=$(curl -s -X PUT "${NEUTRON_URL}/v2.0/floatingips/$FIP_ID" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "floatingip": {
            "port_id": null
        }
    }')

DISASSOC_STATUS=$(echo "$disassoc_response" | jq -r '.floatingip.status')

if [ "$DISASSOC_STATUS" = "DOWN" ]; then
    print_pass "Disassociated floating IP"
    print_info "Status: $DISASSOC_STATUS"
else
    print_fail "Failed to disassociate floating IP"
    print_info "Expected: DOWN, Got: $DISASSOC_STATUS"
    echo "$disassoc_response" | jq '.'
fi

# Test 17: Delete Floating IP
print_test "Delete Floating IP"

delete_fip=$(curl -s -w "%{http_code}" -o /dev/null -X DELETE \
    "${NEUTRON_URL}/v2.0/floatingips/$FIP_ID" \
    -H "X-Auth-Token: $TOKEN")

if [ "$delete_fip" = "204" ]; then
    print_pass "Deleted floating IP"
else
    print_fail "Failed to delete floating IP (HTTP $delete_fip)"
fi

# Test 18: Remove Router Interface
print_test "Remove Router Interface"

remove_interface=$(curl -s -X PUT "${NEUTRON_URL}/v2.0/routers/$ROUTER_ID/remove_router_interface" \
    -H "X-Auth-Token: $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "subnet_id": "'$SUBNET_ID'"
    }')

if echo "$remove_interface" | jq -e '.subnet_id' > /dev/null; then
    print_pass "Removed router interface"
else
    print_fail "Failed to remove router interface"
fi

# Test 19: Delete Router
print_test "Delete Router"

delete_router=$(curl -s -w "%{http_code}" -o /dev/null -X DELETE \
    "${NEUTRON_URL}/v2.0/routers/$ROUTER_ID" \
    -H "X-Auth-Token: $TOKEN")

if [ "$delete_router" = "204" ]; then
    print_pass "Deleted router"
else
    print_fail "Failed to delete router (HTTP $delete_router)"
fi

# Cleanup
print_test "Cleanup Test Resources"

curl -s -X DELETE "${NEUTRON_URL}/v2.0/ports/$PORT_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
curl -s -X DELETE "${NEUTRON_URL}/v2.0/subnets/$SUBNET_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
curl -s -X DELETE "${NEUTRON_URL}/v2.0/networks/$NETWORK_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
curl -s -X DELETE "${NEUTRON_URL}/v2.0/subnets/$EXT_SUBNET_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
curl -s -X DELETE "${NEUTRON_URL}/v2.0/networks/$EXT_NETWORK_ID" -H "X-Auth-Token: $TOKEN" > /dev/null

print_pass "Cleaned up test resources"

echo ""
echo "=========================================="
echo " Test Summary"
echo "=========================================="
echo "Total Passed: $PASSED"
echo "Total Failed: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All L3 router API tests passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. L3 router API is fully functional (tested in stub mode)"
    echo "  2. On Linux with root: networking_mode: iptables for real NAT"
    echo "  3. Test with Horizon dashboard router operations"
    echo "  4. Proceed to Phase 2B: VXLAN multi-node overlay"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
