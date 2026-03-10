#!/bin/bash
# Cinder volume actions test - extend and retype
# Tests volume resize and type change operations

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Base URLs
AUTH_URL="${OS_AUTH_URL:-http://localhost:5001/v3}"
CINDER_URL="${OS_VOLUME_URL:-http://localhost:8776}"
PROJECT_ID="00000000-0000-0000-0000-000000000002"

echo "=== Cinder Volume Actions Test ==="

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

# Test 1: Extend volume
echo "Test 1: Extend volume"
VOLUME_ID=$(curl -s -X POST "$CINDER_URL/v3/$PROJECT_ID/volumes" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"volume":{"size":1,"name":"test-volume-extend"}}' \
  | jq -r '.volume.id')

if [ "$VOLUME_ID" = "null" ] || [ -z "$VOLUME_ID" ]; then
  echo -e "${RED}FAIL${NC}: Volume creation failed"
  exit 1
fi

curl -s -X POST "$CINDER_URL/v3/$PROJECT_ID/volumes/$VOLUME_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"os-extend":{"new_size":5}}' \
  -w "%{http_code}" | grep -q "202\|200"
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: Extend volume"
else
  echo -e "${RED}FAIL${NC}: Extend volume"
fi

# Verify new size
VOLUME_SIZE=$(curl -s -X GET "$CINDER_URL/v3/$PROJECT_ID/volumes/$VOLUME_ID" \
  -H "X-Auth-Token: $TOKEN" | jq -r '.volume.size')
if [ "$VOLUME_SIZE" = "5" ]; then
  echo -e "${GREEN}PASS${NC}: Volume size updated to 5GB"
else
  echo -e "${RED}FAIL${NC}: Volume size not updated (got $VOLUME_SIZE)"
fi

# Test 2: Retype volume
echo "Test 2: Retype volume"
curl -s -X POST "$CINDER_URL/v3/$PROJECT_ID/volumes/$VOLUME_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"os-retype":{"new_type":"ssd","migration_policy":"on-demand"}}' \
  -w "%{http_code}" | grep -q "202\|200"
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: Retype volume"
else
  echo -e "${RED}FAIL${NC}: Retype volume"
fi

# Cleanup
echo "Cleaning up..."
curl -s -X DELETE "$CINDER_URL/v3/$PROJECT_ID/volumes/$VOLUME_ID" \
  -H "X-Auth-Token: $TOKEN" > /dev/null
echo -e "${GREEN}OK${NC}: Cleanup complete"

echo "=== Test Summary ==="
echo "All Cinder volume action tests completed"
