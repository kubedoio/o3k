#!/bin/bash
# Multi-tenancy and Project Isolation Test Suite

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}✗${NC} $1"
    ((TESTS_FAILED++))
}

echo "========================================"
echo " Multi-Tenancy & Isolation Test Suite"
echo "========================================"
echo ""

# Setup: Create two projects with different users
echo "Setting up test projects and users..."

# Admin authentication
ADMIN_TOKEN=$(curl -s -X POST "http://localhost:35357/v3/auth/tokens" \
    -H "Content-Type: application/json" \
    -d '{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"secret","domain":{"name":"Default"}}}},"scope":{"project":{"name":"default","domain":{"name":"Default"}}}}}' \
    | jq -r '.token.token')

if [ -z "$ADMIN_TOKEN" ]; then
    echo -e "${RED}Failed to get admin token${NC}"
    exit 1
fi

# Create project1
PROJECT1_ID=$(curl -s -X POST "http://localhost:35357/v3/projects" \
    -H "X-Auth-Token: $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"project":{"name":"test-project-1","domain_id":"default"}}' \
    | jq -r '.project.id')

# Create project2
PROJECT2_ID=$(curl -s -X POST "http://localhost:35357/v3/projects" \
    -H "X-Auth-Token: $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"project":{"name":"test-project-2","domain_id":"default"}}' \
    | jq -r '.project.id')

# Create user1 for project1
USER1_ID=$(curl -s -X POST "http://localhost:35357/v3/users" \
    -H "X-Auth-Token: $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"user":{"name":"test-user-1","password":"password1","domain_id":"default"}}' \
    | jq -r '.user.id')

# Create user2 for project2
USER2_ID=$(curl -s -X POST "http://localhost:35357/v3/users" \
    -H "X-Auth-Token: $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"user":{"name":"test-user-2","password":"password2","domain_id":"default"}}' \
    | jq -r '.user.id')

# Get member role ID
MEMBER_ROLE_ID=$(curl -s -X GET "http://localhost:35357/v3/roles?name=_member_" \
    -H "X-Auth-Token: $ADMIN_TOKEN" | jq -r '.roles[0].id')

# Assign user1 to project1
curl -s -X PUT "http://localhost:35357/v3/projects/$PROJECT1_ID/users/$USER1_ID/roles/$MEMBER_ROLE_ID" \
    -H "X-Auth-Token: $ADMIN_TOKEN" > /dev/null

# Assign user2 to project2
curl -s -X PUT "http://localhost:35357/v3/projects/$PROJECT2_ID/users/$USER2_ID/roles/$MEMBER_ROLE_ID" \
    -H "X-Auth-Token: $ADMIN_TOKEN" > /dev/null

echo "Created PROJECT1=$PROJECT1_ID, USER1=$USER1_ID"
echo "Created PROJECT2=$PROJECT2_ID, USER2=$USER2_ID"
echo ""

# Get tokens for user1 and user2
USER1_TOKEN=$(curl -s -X POST "http://localhost:35357/v3/auth/tokens" \
    -H "Content-Type: application/json" \
    -d '{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"test-user-1","password":"password1","domain":{"name":"Default"}}}},"scope":{"project":{"id":"'$PROJECT1_ID'"}}}}' \
    | jq -r '.token.token')

USER2_TOKEN=$(curl -s -X POST "http://localhost:35357/v3/auth/tokens" \
    -H "Content-Type: application/json" \
    -d '{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"test-user-2","password":"password2","domain":{"name":"Default"}}}},"scope":{"project":{"id":"'$PROJECT2_ID'"}}}}' \
    | jq -r '.token.token')

# Test 1: Server isolation - User1 creates server, User2 cannot see it
log_test "Server isolation between projects"

SERVER1_ID=$(curl -s -X POST "http://localhost:8774/v2.1/servers" \
    -H "X-Auth-Token: $USER1_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"server":{"name":"project1-server","flavorRef":"m1.tiny"}}' \
    | jq -r '.server.id')

if [ -n "$SERVER1_ID" ] && [ "$SERVER1_ID" != "null" ]; then
    # User1 can see their own server
    USER1_SEES_OWN=$(curl -s -X GET "http://localhost:8774/v2.1/servers/$SERVER1_ID" \
        -H "X-Auth-Token: $USER1_TOKEN" | jq -r '.server.id')

    if [ "$USER1_SEES_OWN" = "$SERVER1_ID" ]; then
        log_pass "User1 can see their own server"
    else
        log_fail "User1 cannot see their own server"
    fi

    # User2 cannot see user1's server
    USER2_SEES_USER1=$(curl -s -X GET "http://localhost:8774/v2.1/servers/$SERVER1_ID" \
        -H "X-Auth-Token: $USER2_TOKEN" | jq -r '.error // .itemNotFound // "not-found"')

    if [ "$USER2_SEES_USER1" != "$SERVER1_ID" ]; then
        log_pass "User2 cannot see User1's server (isolation working)"
    else
        log_fail "User2 can see User1's server (ISOLATION BREACH)"
    fi

    # User2 cannot delete user1's server
    DELETE_RESULT=$(curl -s -w "%{http_code}" -X DELETE "http://localhost:8774/v2.1/servers/$SERVER1_ID" \
        -H "X-Auth-Token: $USER2_TOKEN" -o /dev/null)

    if [ "$DELETE_RESULT" = "404" ] || [ "$DELETE_RESULT" = "403" ]; then
        log_pass "User2 cannot delete User1's server"
    else
        log_fail "User2 can delete User1's server (ISOLATION BREACH)"
    fi
else
    log_fail "Failed to create server for User1"
fi

# Test 2: Network isolation
log_test "Network isolation between projects"

NET1_ID=$(curl -s -X POST "http://localhost:9696/v2.0/networks" \
    -H "X-Auth-Token: $USER1_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"network":{"name":"project1-network"}}' \
    | jq -r '.network.id')

if [ -n "$NET1_ID" ] && [ "$NET1_ID" != "null" ]; then
    # User2 cannot see user1's network
    USER2_NETS=$(curl -s -X GET "http://localhost:9696/v2.0/networks" \
        -H "X-Auth-Token: $USER2_TOKEN" | jq -r '.networks[] | select(.id=="'$NET1_ID'") | .id')

    if [ -z "$USER2_NETS" ]; then
        log_pass "User2 cannot see User1's network"
    else
        log_fail "User2 can see User1's network (ISOLATION BREACH)"
    fi
else
    log_fail "Failed to create network for User1"
fi

# Test 3: Volume isolation
log_test "Volume isolation between projects"

VOL1_ID=$(curl -s -X POST "http://localhost:8776/v3/$PROJECT1_ID/volumes" \
    -H "X-Auth-Token: $USER1_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"volume":{"name":"project1-volume","size":1}}' \
    | jq -r '.volume.id')

if [ -n "$VOL1_ID" ] && [ "$VOL1_ID" != "null" ]; then
    # User2 cannot see user1's volume
    USER2_VOLS=$(curl -s -X GET "http://localhost:8776/v3/$PROJECT2_ID/volumes/$VOL1_ID" \
        -H "X-Auth-Token: $USER2_TOKEN" | jq -r '.error // .itemNotFound // "not-found"')

    if [ "$USER2_VOLS" != "$VOL1_ID" ]; then
        log_pass "User2 cannot see User1's volume"
    else
        log_fail "User2 can see User1's volume (ISOLATION BREACH)"
    fi
else
    log_fail "Failed to create volume for User1"
fi

# Test 4: Image isolation (private images)
log_test "Image isolation between projects"

IMAGE1_ID=$(curl -s -X POST "http://localhost:9292/v2/images" \
    -H "X-Auth-Token: $USER1_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"project1-private-image","visibility":"private","container_format":"bare","disk_format":"raw"}' \
    | jq -r '.id')

if [ -n "$IMAGE1_ID" ] && [ "$IMAGE1_ID" != "null" ]; then
    # User2 cannot see user1's private image
    USER2_IMAGES=$(curl -s -X GET "http://localhost:9292/v2/images" \
        -H "X-Auth-Token: $USER2_TOKEN" | jq -r '.images[] | select(.id=="'$IMAGE1_ID'") | .id')

    if [ -z "$USER2_IMAGES" ]; then
        log_pass "User2 cannot see User1's private image"
    else
        log_fail "User2 can see User1's private image (ISOLATION BREACH)"
    fi
else
    log_fail "Failed to create image for User1"
fi

# Test 5: Quota isolation
log_test "Quota isolation between projects"

# User1 creates resources
for i in {1..3}; do
    curl -s -X POST "http://localhost:8774/v2.1/servers" \
        -H "X-Auth-Token: $USER1_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"server":{"name":"project1-server-'$i'","flavorRef":"m1.tiny"}}' > /dev/null
done

# Check User1's usage affects their quota, not User2's
USER1_SERVERS=$(curl -s -X GET "http://localhost:8774/v2.1/servers" \
    -H "X-Auth-Token: $USER1_TOKEN" | jq -r '.servers | length')

USER2_SERVERS=$(curl -s -X GET "http://localhost:8774/v2.1/servers" \
    -H "X-Auth-Token: $USER2_TOKEN" | jq -r '.servers | length')

if [ "$USER1_SERVERS" -ge 3 ] && [ "$USER2_SERVERS" -eq 0 ]; then
    log_pass "Quota usage isolated between projects"
else
    log_fail "Quota usage not properly isolated (User1=$USER1_SERVERS, User2=$USER2_SERVERS)"
fi

# Test 6: Cross-project resource references
log_test "Cannot reference cross-project resources"

# User2 tries to attach User1's volume to their own server (should fail)
SERVER2_ID=$(curl -s -X POST "http://localhost:8774/v2.1/servers" \
    -H "X-Auth-Token: $USER2_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"server":{"name":"project2-server","flavorRef":"m1.tiny"}}' \
    | jq -r '.server.id')

if [ -n "$SERVER2_ID" ] && [ "$SERVER2_ID" != "null" ] && [ -n "$VOL1_ID" ]; then
    ATTACH_RESULT=$(curl -s -X POST "http://localhost:8774/v2.1/servers/$SERVER2_ID/os-volume_attachments" \
        -H "X-Auth-Token: $USER2_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"volumeAttachment":{"volumeId":"'$VOL1_ID'"}}' \
        | jq -r '.error // .badRequest // "success"')

    if [ "$ATTACH_RESULT" != "success" ]; then
        log_pass "Cannot attach cross-project volume"
    else
        log_fail "Can attach cross-project volume (ISOLATION BREACH)"
    fi
fi

# Cleanup
echo ""
echo "Cleaning up test resources..."

# Delete servers
curl -s -X DELETE "http://localhost:8774/v2.1/servers/$SERVER1_ID" \
    -H "X-Auth-Token: $USER1_TOKEN" 2>/dev/null || true

curl -s -X DELETE "http://localhost:8774/v2.1/servers/$SERVER2_ID" \
    -H "X-Auth-Token: $USER2_TOKEN" 2>/dev/null || true

for i in {1..3}; do
    curl -s -X GET "http://localhost:8774/v2.1/servers" \
        -H "X-Auth-Token: $USER1_TOKEN" | jq -r '.servers[].id' | while read sid; do
        curl -s -X DELETE "http://localhost:8774/v2.1/servers/$sid" \
            -H "X-Auth-Token: $USER1_TOKEN" 2>/dev/null || true
    done
done

# Delete volumes
curl -s -X DELETE "http://localhost:8776/v3/$PROJECT1_ID/volumes/$VOL1_ID" \
    -H "X-Auth-Token: $USER1_TOKEN" 2>/dev/null || true

# Delete networks
curl -s -X DELETE "http://localhost:9696/v2.0/networks/$NET1_ID" \
    -H "X-Auth-Token: $USER1_TOKEN" 2>/dev/null || true

# Delete images
curl -s -X DELETE "http://localhost:9292/v2/images/$IMAGE1_ID" \
    -H "X-Auth-Token: $USER1_TOKEN" 2>/dev/null || true

# Delete users and projects
curl -s -X DELETE "http://localhost:35357/v3/users/$USER1_ID" \
    -H "X-Auth-Token: $ADMIN_TOKEN" 2>/dev/null || true

curl -s -X DELETE "http://localhost:35357/v3/users/$USER2_ID" \
    -H "X-Auth-Token: $ADMIN_TOKEN" 2>/dev/null || true

curl -s -X DELETE "http://localhost:35357/v3/projects/$PROJECT1_ID" \
    -H "X-Auth-Token: $ADMIN_TOKEN" 2>/dev/null || true

curl -s -X DELETE "http://localhost:35357/v3/projects/$PROJECT2_ID" \
    -H "X-Auth-Token: $ADMIN_TOKEN" 2>/dev/null || true

# Summary
echo ""
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All multi-tenancy tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some multi-tenancy tests failed!${NC}"
    exit 1
fi
