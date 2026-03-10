#!/bin/bash
# Nova server actions test - pause, unpause, lock, unlock, forceDelete
# Tests additional server lifecycle actions

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Base URLs
AUTH_URL="${OS_AUTH_URL:-http://localhost:5001/v3}"
NOVA_URL="${OS_COMPUTE_URL:-http://localhost:8774}"

echo "=== Nova Server Actions Test (pause/lock/force) ==="

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

# Test 1: Pause server
echo "Test 1: Pause server"
SERVER_ID=$(curl -s -X POST "$NOVA_URL/v2.1/servers" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "server": {
      "name": "test-server-pause",
      "flavorRef": "00000000-0000-0000-0000-000000000010",
      "imageRef": "00000000-0000-0000-0000-000000000001"
    }
  }' | jq -r '.server.id')

curl -s -X POST "$NOVA_URL/v2.1/servers/$SERVER_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pause": null}' \
  -w "%{http_code}" | grep -q "202\|200"
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: Pause server"
else
  echo -e "${RED}FAIL${NC}: Pause server"
fi

# Test 2: Unpause server
echo "Test 2: Unpause server"
curl -s -X POST "$NOVA_URL/v2.1/servers/$SERVER_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"unpause": null}' \
  -w "%{http_code}" | grep -q "202\|200"
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: Unpause server"
else
  echo -e "${RED}FAIL${NC}: Unpause server"
fi

# Test 3: Lock server
echo "Test 3: Lock server"
curl -s -X POST "$NOVA_URL/v2.1/servers/$SERVER_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"lock": null}' \
  -w "%{http_code}" | grep -q "202\|200"
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: Lock server"
else
  echo -e "${RED}FAIL${NC}: Lock server"
fi

# Test 4: Unlock server
echo "Test 4: Unlock server"
curl -s -X POST "$NOVA_URL/v2.1/servers/$SERVER_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"unlock": null}' \
  -w "%{http_code}" | grep -q "202\|200"
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: Unlock server"
else
  echo -e "${RED}FAIL${NC}: Unlock server"
fi

# Test 5: Force delete server
echo "Test 5: Force delete server"
curl -s -X POST "$NOVA_URL/v2.1/servers/$SERVER_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"forceDelete": null}' \
  -w "%{http_code}" | grep -q "202\|204"
if [ $? -eq 0 ]; then
  echo -e "${GREEN}PASS${NC}: Force delete server"
else
  echo -e "${RED}FAIL${NC}: Force delete server"
fi

echo "=== Test Summary ==="
echo "All Nova server action tests completed"
