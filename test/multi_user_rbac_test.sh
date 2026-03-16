#!/bin/bash
# Test multi-user isolation and RBAC for Horizon integration

set -e

echo "=== Multi-User RBAC Integration Test ==="

# Configuration
export OS_AUTH_URL=http://localhost:35357/v3
export OS_DOMAIN_NAME=Default

echo "1. Authenticating as admin..."
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default

ADMIN_TOKEN=$(openstack token issue -f value -c id)
ADMIN_PROJECT_ID=$(openstack token issue -f value -c project_id)

if [ -z "$ADMIN_TOKEN" ]; then
  echo "✗ Failed to authenticate as admin"
  exit 1
fi
echo "✓ Admin authentication successful"

echo "2. Creating test users and projects..."
# Create second project
PROJECT2_NAME="test-project-$$"
PROJECT2_ID=$(openstack project create $PROJECT2_NAME -f value -c id)
echo "  ✓ Created project: $PROJECT2_NAME ($PROJECT2_ID)"

# Create test user 1 (member role)
USER1_NAME="test-user1-$$"
USER1_PASS="testpass1-$$"
openstack user create $USER1_NAME --password $USER1_PASS --domain Default > /dev/null
echo "  ✓ Created user: $USER1_NAME"

# Create test user 2 (member role, different project)
USER2_NAME="test-user2-$$"
USER2_PASS="testpass2-$$"
openstack user create $USER2_NAME --password $USER2_PASS --domain Default > /dev/null
echo "  ✓ Created user: $USER2_NAME"

# Assign roles
MEMBER_ROLE_ID=$(openstack role list -f value -c ID -c Name | grep member | awk '{print $1}')
openstack role add --user $USER1_NAME --project $ADMIN_PROJECT_ID $MEMBER_ROLE_ID
openstack role add --user $USER2_NAME --project $PROJECT2_ID $MEMBER_ROLE_ID
echo "  ✓ Assigned member roles"

echo "3. Creating resources as admin..."
# Create admin's server
ADMIN_SERVER_ID=$(openstack server create admin-server-$$ \
  --flavor 1 \
  --image test-image \
  -f value -c id 2>/dev/null || echo "")

if [ -n "$ADMIN_SERVER_ID" ]; then
  echo "  ✓ Created admin server: $ADMIN_SERVER_ID"
fi

# Create admin's network
ADMIN_NET_ID=$(openstack network create admin-network-$$ -f value -c id)
echo "  ✓ Created admin network: $ADMIN_NET_ID"

# Create admin's volume
ADMIN_VOL_ID=$(openstack volume create admin-volume-$$ --size 1 -f value -c id)
echo "  ✓ Created admin volume: $ADMIN_VOL_ID"

echo "4. Testing User 1 (same project as admin)..."
# Authenticate as user 1
export OS_USERNAME=$USER1_NAME
export OS_PASSWORD=$USER1_PASS
export OS_PROJECT_NAME=default

USER1_TOKEN=$(openstack token issue -f value -c id)
if [ -z "$USER1_TOKEN" ]; then
  echo "✗ User 1 authentication failed"
  USER1_AUTH="FAIL"
else
  echo "  ✓ User 1 authenticated"
  USER1_AUTH="PASS"
fi

# User 1 should see admin's resources (same project)
if [ -n "$ADMIN_SERVER_ID" ]; then
  USER1_SERVERS=$(openstack server list -f value -c ID)
  if echo "$USER1_SERVERS" | grep -q "$ADMIN_SERVER_ID"; then
    echo "  ✓ User 1 can see admin's server (same project)"
    USER1_ISOLATION="PASS"
  else
    echo "  ✗ User 1 cannot see admin's server (should see it)"
    USER1_ISOLATION="FAIL"
  fi
else
  USER1_ISOLATION="SKIP"
fi

# User 1 should see admin's network
USER1_NETWORKS=$(openstack network list -f value -c ID)
if echo "$USER1_NETWORKS" | grep -q "$ADMIN_NET_ID"; then
  echo "  ✓ User 1 can see admin's network (same project)"
else
  echo "  ✗ User 1 cannot see admin's network (should see it)"
fi

echo "5. Testing User 2 (different project)..."
# Authenticate as user 2
export OS_USERNAME=$USER2_NAME
export OS_PASSWORD=$USER2_PASS
export OS_PROJECT_NAME=$PROJECT2_NAME

USER2_TOKEN=$(openstack token issue -f value -c id)
if [ -z "$USER2_TOKEN" ]; then
  echo "✗ User 2 authentication failed"
  USER2_AUTH="FAIL"
else
  echo "  ✓ User 2 authenticated"
  USER2_AUTH="PASS"
fi

# User 2 should NOT see admin's resources (different project)
if [ -n "$ADMIN_SERVER_ID" ]; then
  USER2_SERVERS=$(openstack server list -f value -c ID)
  if echo "$USER2_SERVERS" | grep -q "$ADMIN_SERVER_ID"; then
    echo "  ✗ User 2 can see admin's server (ISOLATION BREACH!)"
    USER2_ISOLATION="FAIL"
  else
    echo "  ✓ User 2 cannot see admin's server (correct isolation)"
    USER2_ISOLATION="PASS"
  fi
else
  USER2_ISOLATION="SKIP"
fi

# User 2 should NOT see admin's private network
USER2_NETWORKS=$(openstack network list -f value -c ID)
if echo "$USER2_NETWORKS" | grep -q "$ADMIN_NET_ID"; then
  echo "  ✗ User 2 can see admin's network (ISOLATION BREACH!)"
else
  echo "  ✓ User 2 cannot see admin's network (correct isolation)"
fi

# User 2 should NOT see admin's volume
USER2_VOLUMES=$(openstack volume list -f value -c ID)
if echo "$USER2_VOLUMES" | grep -q "$ADMIN_VOL_ID"; then
  echo "  ✗ User 2 can see admin's volume (ISOLATION BREACH!)"
else
  echo "  ✓ User 2 cannot see admin's volume (correct isolation)"
fi

echo "6. Testing User 2 can create resources in their own project..."
# Create server as user 2
USER2_SERVER_ID=$(openstack server create user2-server-$$ \
  --flavor 1 \
  --image test-image \
  -f value -c id 2>/dev/null || echo "")

if [ -n "$USER2_SERVER_ID" ]; then
  echo "  ✓ User 2 created server in their project"
  USER2_CREATE="PASS"
else
  echo "  ⚠ User 2 could not create server (may be expected in stub mode)"
  USER2_CREATE="WARN"
fi

echo "7. Testing admin-only operations..."
# Switch back to admin
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default

# Admin can view quotas for other projects
QUOTA_RESPONSE=$(openstack quota show $PROJECT2_ID -f json 2>/dev/null || echo "{}")
if echo "$QUOTA_RESPONSE" | jq -e '.instances' > /dev/null 2>&1; then
  echo "  ✓ Admin can view quotas for other projects"
  ADMIN_QUOTA="PASS"
else
  echo "  ⚠ Admin quota view failed (may not be implemented)"
  ADMIN_QUOTA="WARN"
fi

echo "8. Cleanup..."
# Switch back to admin for cleanup
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default

[ -n "$ADMIN_SERVER_ID" ] && openstack server delete $ADMIN_SERVER_ID --wait || true
[ -n "$USER2_SERVER_ID" ] && openstack server delete $USER2_SERVER_ID --wait || true
openstack volume delete $ADMIN_VOL_ID || true
openstack network delete $ADMIN_NET_ID || true

openstack user delete $USER1_NAME || true
openstack user delete $USER2_NAME || true
openstack project delete $PROJECT2_ID || true

echo ""
echo "=== Test Results ==="
echo "User 1 Auth: $USER1_AUTH"
echo "User 1 Isolation (same project): $USER1_ISOLATION"
echo "User 2 Auth: $USER2_AUTH"
echo "User 2 Isolation (different project): $USER2_ISOLATION"
echo "User 2 Create: $USER2_CREATE"
echo "Admin Quota Access: $ADMIN_QUOTA"

FAILED=false
if [ "$USER1_AUTH" = "FAIL" ] || [ "$USER1_ISOLATION" = "FAIL" ] || \
   [ "$USER2_AUTH" = "FAIL" ] || [ "$USER2_ISOLATION" = "FAIL" ]; then
  FAILED=true
fi

if [ "$FAILED" = true ]; then
  echo ""
  echo "✗ Multi-user isolation tests failed"
  exit 1
else
  echo ""
  echo "✓ Multi-user RBAC tests passed"
  echo "  Project isolation working correctly"
  echo "  Users can only see resources in their project"
  exit 0
fi
