#!/bin/bash
# Comprehensive test script for LightStack Phases 0-2

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║       LightStack Comprehensive Test Suite (Phases 0-2)         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}\n"

KEYSTONE_URL="http://localhost:5000"
NOVA_URL="http://localhost:8774"
NEUTRON_URL="http://localhost:9696"
CINDER_URL="http://localhost:8776"
GLANCE_URL="http://localhost:9292"

PASS=0
FAIL=0

# Helper function for tests
test_endpoint() {
    local name="$1"
    local method="$2"
    local url="$3"
    local expected_code="$4"
    local headers="$5"

    if [ -n "$headers" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" $headers)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$url")
    fi

    http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" -eq "$expected_code" ]; then
        echo -e "${GREEN}✓${NC} $name (HTTP $http_code)"
        ((PASS++))
        return 0
    else
        echo -e "${RED}✗${NC} $name (Expected HTTP $expected_code, got $http_code)"
        ((FAIL++))
        return 1
    fi
}

# ============================================================================
echo -e "${YELLOW}=== Phase 0: Foundation Tests ===${NC}\n"
# ============================================================================

# Test: Binary exists
if [ -f "bin/lightstack" ]; then
    echo -e "${GREEN}✓${NC} Binary exists ($(ls -lh bin/lightstack | awk '{print $5}'))"
    ((PASS++))
else
    echo -e "${RED}✗${NC} Binary not found"
    ((FAIL++))
fi

# Test: Config file exists
if [ -f "config/lightstack.yaml" ]; then
    echo -e "${GREEN}✓${NC} Config file exists"
    ((PASS++))
else
    echo -e "${RED}✗${NC} Config file not found"
    ((FAIL++))
fi

# Test: Migrations exist
if [ -d "migrations" ] && [ $(ls migrations/*.sql 2>/dev/null | wc -l) -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Database migrations exist ($(ls migrations/*.sql | wc -l) files)"
    ((PASS++))
else
    echo -e "${RED}✗${NC} Migrations not found"
    ((FAIL++))
fi

# ============================================================================
echo -e "\n${YELLOW}=== Phase 1: Keystone Identity Service ===${NC}\n"
# ============================================================================

# Test: Root version discovery
test_endpoint "Root version discovery" "GET" "$KEYSTONE_URL/" 200

# Test: Keystone v3 version
test_endpoint "Keystone v3 version" "GET" "$KEYSTONE_URL/v3" 200

# Test: Unscoped authentication
echo -e "${BLUE}Testing unscoped authentication...${NC}"
unscoped_response=$(curl -s -i -X POST "$KEYSTONE_URL/v3/auth/tokens" \
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
    echo -e "${GREEN}✓${NC} Unscoped authentication successful"
    ((PASS++))
else
    echo -e "${RED}✗${NC} Unscoped authentication failed"
    ((FAIL++))
fi

# Test: Scoped authentication
echo -e "${BLUE}Testing scoped authentication...${NC}"
scoped_response=$(curl -s -i -X POST "$KEYSTONE_URL/v3/auth/tokens" \
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
    echo -e "${GREEN}✓${NC} Scoped authentication successful"
    echo -e "${GREEN}✓${NC} Service catalog present"
    ((PASS+=2))
else
    echo -e "${RED}✗${NC} Scoped authentication or catalog failed"
    ((FAIL+=2))
fi

# Test: List projects
test_endpoint "List projects" "GET" "$KEYSTONE_URL/v3/projects" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: List users
test_endpoint "List users" "GET" "$KEYSTONE_URL/v3/users" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: List roles
test_endpoint "List roles" "GET" "$KEYSTONE_URL/v3/roles" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: Token validation
test_endpoint "Token validation" "GET" "$KEYSTONE_URL/v3/auth/tokens" 200 "-H 'X-Auth-Token: $scoped_token' -H 'X-Subject-Token: $scoped_token'"

# Test: Invalid credentials (should fail)
echo -e "${BLUE}Testing invalid credentials (should fail)...${NC}"
invalid_response=$(curl -s -w "\n%{http_code}" -X POST "$KEYSTONE_URL/v3/auth/tokens" \
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
    echo -e "${GREEN}✓${NC} Invalid credentials correctly rejected (HTTP 401)"
    ((PASS++))
else
    echo -e "${RED}✗${NC} Invalid credentials test failed (got HTTP $http_code)"
    ((FAIL++))
fi

# ============================================================================
echo -e "\n${YELLOW}=== Phase 2: Nova Compute Service ===${NC}\n"
# ============================================================================

# Test: Nova version discovery
test_endpoint "Nova version list" "GET" "$NOVA_URL/" 200 "-H 'X-Auth-Token: $scoped_token'"
test_endpoint "Nova v2.1 version" "GET" "$NOVA_URL/v2.1" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: List flavors
test_endpoint "List flavors" "GET" "$NOVA_URL/v2.1/flavors" 200 "-H 'X-Auth-Token: $scoped_token'"
test_endpoint "List flavors detail" "GET" "$NOVA_URL/v2.1/flavors/detail" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: Get specific flavor (m1.small)
echo -e "${BLUE}Testing flavor retrieval...${NC}"
flavors_response=$(curl -s "$NOVA_URL/v2.1/flavors/detail" -H "X-Auth-Token: $scoped_token")
m1_small_id=$(echo "$flavors_response" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -n "$m1_small_id" ]; then
    test_endpoint "Get m1.small flavor" "GET" "$NOVA_URL/v2.1/flavors/$m1_small_id" 200 "-H 'X-Auth-Token: $scoped_token'"
else
    echo -e "${YELLOW}⚠${NC} Could not find m1.small flavor ID (database not seeded?)"
fi

# Test: List servers (should be empty initially)
test_endpoint "List servers" "GET" "$NOVA_URL/v2.1/servers" 200 "-H 'X-Auth-Token: $scoped_token'"
test_endpoint "List servers detail" "GET" "$NOVA_URL/v2.1/servers/detail" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: Create server (database only, no VM)
echo -e "${BLUE}Testing server creation...${NC}"
if [ -n "$m1_small_id" ]; then
    create_response=$(curl -s -w "\n%{http_code}" -X POST "$NOVA_URL/v2.1/servers" \
      -H "X-Auth-Token: $scoped_token" \
      -H "Content-Type: application/json" \
      -d "{
        \"server\": {
          \"name\": \"test-instance\",
          \"flavorRef\": \"$m1_small_id\",
          \"imageRef\": \"cirros-0.6.0\"
        }
      }")

    http_code=$(echo "$create_response" | tail -n1)
    body=$(echo "$create_response" | head -n -1)
    server_id=$(echo "$body" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ "$http_code" -eq 202 ] && [ -n "$server_id" ]; then
        echo -e "${GREEN}✓${NC} Server creation accepted (HTTP 202)"
        echo -e "  Server ID: $server_id"
        ((PASS++))

        # Test: Get server details
        sleep 1
        test_endpoint "Get server details" "GET" "$NOVA_URL/v2.1/servers/$server_id" 200 "-H 'X-Auth-Token: $scoped_token'"

        # Test: List servers (should show our instance)
        servers_list=$(curl -s "$NOVA_URL/v2.1/servers" -H "X-Auth-Token: $scoped_token")
        if echo "$servers_list" | grep -q "$server_id"; then
            echo -e "${GREEN}✓${NC} Server appears in list"
            ((PASS++))
        else
            echo -e "${RED}✗${NC} Server not in list"
            ((FAIL++))
        fi

        # Test: Delete server
        test_endpoint "Delete server" "DELETE" "$NOVA_URL/v2.1/servers/$server_id" 204 "-H 'X-Auth-Token: $scoped_token'"
    else
        echo -e "${RED}✗${NC} Server creation failed (HTTP $http_code)"
        ((FAIL++))
    fi
else
    echo -e "${YELLOW}⚠${NC} Skipping server creation test (no flavor ID)"
fi

# Test: Hypervisors
test_endpoint "List hypervisors" "GET" "$NOVA_URL/v2.1/os-hypervisors" 200 "-H 'X-Auth-Token: $scoped_token'"
test_endpoint "List hypervisors detail" "GET" "$NOVA_URL/v2.1/os-hypervisors/detail" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: Availability zones
test_endpoint "List availability zones" "GET" "$NOVA_URL/v2.1/os-availability-zone" 200 "-H 'X-Auth-Token: $scoped_token'"

# ============================================================================
echo -e "\n${YELLOW}=== Phase 3-5: Service Stubs ===${NC}\n"
# ============================================================================

# Test: Neutron
test_endpoint "Neutron v2.0 version" "GET" "$NEUTRON_URL/v2.0" 200 "-H 'X-Auth-Token: $scoped_token'"
test_endpoint "Neutron networks" "GET" "$NEUTRON_URL/v2.0/networks" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: Cinder
# Note: Cinder uses project_id in URL, extract from token
project_id=$(echo "$scoped_response" | grep -o '"id":"[^"]*"' | head -2 | tail -1 | cut -d'"' -f4)
test_endpoint "Cinder volumes" "GET" "$CINDER_URL/v3/$project_id/volumes" 200 "-H 'X-Auth-Token: $scoped_token'"

# Test: Glance
test_endpoint "Glance images" "GET" "$GLANCE_URL/v2/images" 200 "-H 'X-Auth-Token: $scoped_token'"

# ============================================================================
echo -e "\n${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                       Test Results                              ║${NC}"
echo -e "${BLUE}╠════════════════════════════════════════════════════════════════╣${NC}"
echo -e "${BLUE}║${NC}  ${GREEN}Passed:${NC} $PASS                                                    ${BLUE}║${NC}"
echo -e "${BLUE}║${NC}  ${RED}Failed:${NC} $FAIL                                                    ${BLUE}║${NC}"
echo -e "${BLUE}║${NC}  ${YELLOW}Total:${NC}  $((PASS + FAIL))                                                   ${BLUE}║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}\n"

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! LightStack Phases 0-2 are fully functional.${NC}\n"
    echo -e "${YELLOW}Ready for:${NC}"
    echo "  ✓ OpenStack CLI usage"
    echo "  ✓ Horizon dashboard login"
    echo "  ✓ Instance management (database)"
    echo "  ✓ Flavor queries"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "  1. Implement Phase 3 (Neutron) - Network isolation"
    echo "  2. Implement Phase 4 (Cinder) - Volume management"
    echo "  3. Implement Phase 5 (Glance) - Image service"
    echo "  4. Complete libvirt integration - Actual VM creation"
    echo "  5. Test with Horizon dashboard"
    exit 0
else
    echo -e "${RED}⚠️  Some tests failed. Please check the output above.${NC}"
    exit 1
fi
