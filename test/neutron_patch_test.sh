#!/bin/bash
# Neutron PATCH support test
# Tests updating networks, subnets, ports, and security groups

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Base URLs
AUTH_URL="${OS_AUTH_URL:-http://localhost:5001/v3}"
NEUTRON_URL="${OS_NETWORK_URL:-http://localhost:9696}"

echo "=== Neutron PATCH Support Test ==="

# Get auth token
echo "Authenticating..."
TOKEN=$(curl -s -i -X POST "$AUTH_URL/auth/tokens" \
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
  }' | grep -i '^x-subject-token' | cut -d' ' -f2 | tr -d '\r\n')

if [ -z "$TOKEN" ]; then
  echo -e "${RED}FAIL${NC}: Authentication failed"
  exit 1
fi
echo -e "${GREEN}OK${NC}: Authenticated"

# Test 1: PATCH network
echo "Test 1: PATCH network"
NETWORK_ID=$(curl -s -X POST "$NEUTRON_URL/v2.0/networks" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"network":{"name":"test-patch-network"}}' \
  | jq -r '.network.id')

curl -s -X PATCH "$NEUTRON_URL/v2.0/networks/$NETWORK_ID" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"network":{"name":"updated-network"}}' \
  | jq -e '.network.name == "updated-network"' > /dev/null
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: PATCH network"
else
  echo -e "${RED}FAIL${NC}: PATCH network"
fi

# Test 2: PATCH subnet
echo "Test 2: PATCH subnet"
SUBNET_ID=$(curl -s -X POST "$NEUTRON_URL/v2.0/subnets" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"subnet\":{\"network_id\":\"$NETWORK_ID\",\"cidr\":\"10.0.1.0/24\",\"ip_version\":4,\"name\":\"test-patch-subnet\"}}" \
  | jq -r '.subnet.id')

curl -s -X PATCH "$NEUTRON_URL/v2.0/subnets/$SUBNET_ID" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"subnet":{"name":"updated-subnet"}}' \
  | jq -e '.subnet.name == "updated-subnet"' > /dev/null
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: PATCH subnet"
else
  echo -e "${RED}FAIL${NC}: PATCH subnet"
fi

# Test 3: PATCH port
echo "Test 3: PATCH port"
PORT_ID=$(curl -s -X POST "$NEUTRON_URL/v2.0/ports" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"port\":{\"network_id\":\"$NETWORK_ID\",\"name\":\"test-patch-port\"}}" \
  | jq -r '.port.id')

curl -s -X PATCH "$NEUTRON_URL/v2.0/ports/$PORT_ID" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"port":{"name":"updated-port"}}' \
  | jq -e '.port.name == "updated-port"' > /dev/null
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: PATCH port"
else
  echo -e "${RED}FAIL${NC}: PATCH port"
fi

# Test 4: PATCH security group
echo "Test 4: PATCH security group"
SG_ID=$(curl -s -X POST "$NEUTRON_URL/v2.0/security-groups" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"security_group":{"name":"test-patch-sg","description":"test"}}' \
  | jq -r '.security_group.id')

curl -s -X PATCH "$NEUTRON_URL/v2.0/security-groups/$SG_ID" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"security_group":{"description":"updated description"}}' \
  | jq -e '.security_group.description == "updated description"' > /dev/null
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: PATCH security group"
else
  echo -e "${RED}FAIL${NC}: PATCH security group"
fi

# Cleanup
echo "Cleaning up..."
curl -s -X DELETE "$NEUTRON_URL/v2.0/ports/$PORT_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
curl -s -X DELETE "$NEUTRON_URL/v2.0/security-groups/$SG_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
curl -s -X DELETE "$NEUTRON_URL/v2.0/subnets/$SUBNET_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
curl -s -X DELETE "$NEUTRON_URL/v2.0/networks/$NETWORK_ID" -H "X-Auth-Token: $TOKEN" > /dev/null
echo -e "${GREEN}OK${NC}: Cleanup complete"

echo "=== Test Summary ==="
echo "All Neutron PATCH tests completed"
