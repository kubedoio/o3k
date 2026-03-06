#!/bin/bash
# Test script for LightStack Phase 1 (Keystone)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

BASE_URL="http://localhost:5000"

echo -e "${YELLOW}=== LightStack Keystone Test Suite ===${NC}\n"

# Test 1: Version Discovery
echo -e "${YELLOW}Test 1: Version Discovery${NC}"
response=$(curl -s "${BASE_URL}/")
if echo "$response" | grep -q '"id":"v3.14"'; then
    echo -e "${GREEN}✓ Root version discovery works${NC}"
else
    echo -e "${RED}✗ Root version discovery failed${NC}"
    exit 1
fi

response=$(curl -s "${BASE_URL}/v3")
if echo "$response" | grep -q '"id":"v3.14"'; then
    echo -e "${GREEN}✓ v3 version discovery works${NC}\n"
else
    echo -e "${RED}✗ v3 version discovery failed${NC}"
    exit 1
fi

# Test 2: Unscoped Authentication
echo -e "${YELLOW}Test 2: Unscoped Authentication${NC}"
unscoped_response=$(curl -s -i -X POST "${BASE_URL}/v3/auth/tokens" \
  -H "Content-Type: application/json" \
  -d '{
    "auth": {
      "identity": {
        "methods": ["password"],
        "password": {
          "user": {
            "name": "admin",
            "password": "secret"
          }
        }
      }
    }
  }')

unscoped_token=$(echo "$unscoped_response" | grep -i "X-Subject-Token:" | awk '{print $2}' | tr -d '\r')

if [ -n "$unscoped_token" ]; then
    echo -e "${GREEN}✓ Unscoped authentication successful${NC}"
    echo -e "  Token: ${unscoped_token:0:20}...\n"
else
    echo -e "${RED}✗ Unscoped authentication failed${NC}"
    echo "$unscoped_response"
    exit 1
fi

# Test 3: Scoped Authentication
echo -e "${YELLOW}Test 3: Scoped Authentication (with catalog)${NC}"
scoped_response=$(curl -s -i -X POST "${BASE_URL}/v3/auth/tokens" \
  -H "Content-Type: application/json" \
  -d '{
    "auth": {
      "identity": {
        "methods": ["password"],
        "password": {
          "user": {
            "name": "admin",
            "password": "secret"
          }
        }
      },
      "scope": {
        "project": {
          "name": "default"
        }
      }
    }
  }')

scoped_token=$(echo "$scoped_response" | grep -i "X-Subject-Token:" | awk '{print $2}' | tr -d '\r')
catalog=$(echo "$scoped_response" | grep -o '"catalog"')

if [ -n "$scoped_token" ] && [ -n "$catalog" ]; then
    echo -e "${GREEN}✓ Scoped authentication successful${NC}"
    echo -e "  Token: ${scoped_token:0:20}..."
    echo -e "${GREEN}✓ Service catalog present${NC}\n"
else
    echo -e "${RED}✗ Scoped authentication failed${NC}"
    exit 1
fi

# Test 4: List Projects
echo -e "${YELLOW}Test 4: List Projects${NC}"
projects_response=$(curl -s "${BASE_URL}/v3/projects" \
  -H "X-Auth-Token: $scoped_token")

if echo "$projects_response" | grep -q '"name":"default"'; then
    echo -e "${GREEN}✓ Project listing works${NC}"
    echo -e "  Found 'default' project\n"
else
    echo -e "${RED}✗ Project listing failed${NC}"
    exit 1
fi

# Test 5: List Users
echo -e "${YELLOW}Test 5: List Users${NC}"
users_response=$(curl -s "${BASE_URL}/v3/users" \
  -H "X-Auth-Token: $scoped_token")

if echo "$users_response" | grep -q '"name":"admin"'; then
    echo -e "${GREEN}✓ User listing works${NC}"
    echo -e "  Found 'admin' user\n"
else
    echo -e "${RED}✗ User listing failed${NC}"
    exit 1
fi

# Test 6: List Roles
echo -e "${YELLOW}Test 6: List Roles${NC}"
roles_response=$(curl -s "${BASE_URL}/v3/roles" \
  -H "X-Auth-Token: $scoped_token")

if echo "$roles_response" | grep -q '"name":"admin"' && \
   echo "$roles_response" | grep -q '"name":"member"'; then
    echo -e "${GREEN}✓ Role listing works${NC}"
    echo -e "  Found 'admin' and 'member' roles\n"
else
    echo -e "${RED}✗ Role listing failed${NC}"
    exit 1
fi

# Test 7: Token Validation
echo -e "${YELLOW}Test 7: Token Validation${NC}"
validation_response=$(curl -s -w "\n%{http_code}" \
  "${BASE_URL}/v3/auth/tokens" \
  -H "X-Auth-Token: $scoped_token" \
  -H "X-Subject-Token: $scoped_token")

http_code=$(echo "$validation_response" | tail -n1)
if [ "$http_code" -eq 200 ]; then
    echo -e "${GREEN}✓ Token validation works${NC}\n"
else
    echo -e "${RED}✗ Token validation failed (HTTP $http_code)${NC}"
    exit 1
fi

# Test 8: Invalid Credentials
echo -e "${YELLOW}Test 8: Invalid Credentials (should fail)${NC}"
invalid_response=$(curl -s -w "\n%{http_code}" -X POST "${BASE_URL}/v3/auth/tokens" \
  -H "Content-Type: application/json" \
  -d '{
    "auth": {
      "identity": {
        "methods": ["password"],
        "password": {
          "user": {
            "name": "admin",
            "password": "wrongpassword"
          }
        }
      }
    }
  }')

http_code=$(echo "$invalid_response" | tail -n1)
if [ "$http_code" -eq 401 ]; then
    echo -e "${GREEN}✓ Invalid credentials correctly rejected${NC}\n"
else
    echo -e "${RED}✗ Invalid credentials test failed (expected HTTP 401, got $http_code)${NC}"
    exit 1
fi

# Test 9: Missing Token (should fail)
echo -e "${YELLOW}Test 9: Missing Auth Token (should fail)${NC}"
noauth_response=$(curl -s -w "\n%{http_code}" "${BASE_URL}/v3/users")

http_code=$(echo "$noauth_response" | tail -n1)
if [ "$http_code" -eq 401 ]; then
    echo -e "${GREEN}✓ Missing token correctly rejected${NC}\n"
else
    echo -e "${RED}✗ Missing token test failed (expected HTTP 401, got $http_code)${NC}"
    exit 1
fi

# Test 10: Service Endpoints
echo -e "${YELLOW}Test 10: Service Endpoints (stubs)${NC}"

# Check Nova
nova_response=$(curl -s -w "\n%{http_code}" "http://localhost:8774/")
http_code=$(echo "$nova_response" | tail -n1)
if [ "$http_code" -eq 200 ]; then
    echo -e "${GREEN}✓ Nova endpoint responding${NC}"
else
    echo -e "${YELLOW}⚠ Nova endpoint not responding (HTTP $http_code)${NC}"
fi

# Check Neutron
neutron_response=$(curl -s -w "\n%{http_code}" "http://localhost:9696/v2.0")
http_code=$(echo "$neutron_response" | tail -n1)
if [ "$http_code" -eq 200 ]; then
    echo -e "${GREEN}✓ Neutron endpoint responding${NC}"
else
    echo -e "${YELLOW}⚠ Neutron endpoint not responding (HTTP $http_code)${NC}"
fi

echo -e "\n${GREEN}=== All Keystone Tests Passed! ===${NC}"
echo -e "${YELLOW}Phase 1 (Keystone Identity Service) is complete.${NC}\n"

echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Implement Phase 2 (Nova Compute) - VM creation with libvirt"
echo "  2. Implement Phase 3 (Neutron Network) - Network isolation and port management"
echo "  3. Test with Horizon dashboard"
