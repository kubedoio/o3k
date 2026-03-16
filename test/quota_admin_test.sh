#!/bin/bash
# Test quota admin check enforcement

set -e

echo "=== Quota Admin Check Test ==="

# Authenticate as admin
export OS_AUTH_URL=http://localhost:35357/v3
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default
export OS_DOMAIN_NAME=Default

echo "1. Authenticating as admin..."
ADMIN_TOKEN=$(openstack token issue -f value -c id)
PROJECT_ID=$(openstack token issue -f json | jq -r '.project_id')

echo "2. Creating test non-admin user..."
TEST_USER="quota-test-user-$$"
TEST_PASS="test-pass-$$"

# Create user
openstack user create $TEST_USER --password $TEST_PASS --domain Default || true
USER_ID=$(openstack user list -f value -c ID -c Name | grep $TEST_USER | awk '{print $1}')

# Assign member role (not admin)
MEMBER_ROLE_ID=$(openstack role list -f value -c ID -c Name | grep member | awk '{print $1}')
openstack role add --user $USER_ID --project $PROJECT_ID $MEMBER_ROLE_ID || true

echo "3. Getting non-admin user token..."
NON_ADMIN_TOKEN=$(curl -s -X POST http://localhost:35357/v3/auth/tokens \
  -H "Content-Type: application/json" \
  -d '{
    "auth": {
      "identity": {
        "methods": ["password"],
        "password": {
          "user": {
            "name": "'$TEST_USER'",
            "domain": {"name": "Default"},
            "password": "'$TEST_PASS'"
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
  }' | jq -r '.token.id // empty')

if [ -z "$NON_ADMIN_TOKEN" ]; then
  echo "Failed to get non-admin token, trying alternative method..."
  NON_ADMIN_TOKEN=$(curl -s -i -X POST http://localhost:35357/v3/auth/tokens \
    -H "Content-Type: application/json" \
    -d '{
      "auth": {
        "identity": {
          "methods": ["password"],
          "password": {
            "user": {
              "name": "'$TEST_USER'",
              "domain": {"name": "Default"},
              "password": "'$TEST_PASS'"
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
    }' | grep -i "X-Subject-Token:" | awk '{print $2}' | tr -d '\r')
fi

echo "4. Testing admin can update quotas..."
ADMIN_UPDATE=$(curl -s -X PUT http://localhost:8774/v2.1/os-quota-sets/$PROJECT_ID \
  -H "X-Auth-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "quota_set": {
      "instances": 20,
      "cores": 40
    }
  }')

ADMIN_STATUS=$(echo "$ADMIN_UPDATE" | jq -r '.quota_set // "error"')
if [ "$ADMIN_STATUS" != "error" ]; then
  echo "✓ Admin successfully updated quotas"
  ADMIN_TEST="PASS"
else
  echo "✗ Admin failed to update quotas: $ADMIN_UPDATE"
  ADMIN_TEST="FAIL"
fi

echo "5. Testing non-admin cannot update quotas..."
NON_ADMIN_UPDATE=$(curl -s -X PUT http://localhost:8774/v2.1/os-quota-sets/$PROJECT_ID \
  -H "X-Auth-Token: $NON_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "quota_set": {
      "instances": 50,
      "cores": 100
    }
  }')

NON_ADMIN_ERROR=$(echo "$NON_ADMIN_UPDATE" | jq -r '.error.code // empty')
if [ "$NON_ADMIN_ERROR" = "403" ]; then
  echo "✓ Non-admin correctly denied (403 Forbidden)"
  NON_ADMIN_TEST="PASS"
else
  echo "✗ Non-admin was allowed to update quotas (should be 403): $NON_ADMIN_UPDATE"
  NON_ADMIN_TEST="FAIL"
fi

echo "6. Testing non-admin can read quotas..."
NON_ADMIN_READ=$(curl -s -X GET http://localhost:8774/v2.1/os-quota-sets/$PROJECT_ID \
  -H "X-Auth-Token: $NON_ADMIN_TOKEN")

NON_ADMIN_READ_STATUS=$(echo "$NON_ADMIN_READ" | jq -r '.quota_set // "error"')
if [ "$NON_ADMIN_READ_STATUS" != "error" ]; then
  echo "✓ Non-admin can read quotas (expected)"
  READ_TEST="PASS"
else
  echo "✗ Non-admin cannot read quotas: $NON_ADMIN_READ"
  READ_TEST="FAIL"
fi

echo "7. Cleanup..."
openstack user delete $TEST_USER || true

echo ""
echo "=== Test Results ==="
echo "Admin update: $ADMIN_TEST"
echo "Non-admin denied: $NON_ADMIN_TEST"
echo "Non-admin read: $READ_TEST"

if [ "$ADMIN_TEST" = "PASS" ] && [ "$NON_ADMIN_TEST" = "PASS" ] && [ "$READ_TEST" = "PASS" ]; then
  echo ""
  echo "✓ All tests passed - Quota admin check working correctly"
  exit 0
else
  echo ""
  echo "✗ Some tests failed"
  exit 1
fi
