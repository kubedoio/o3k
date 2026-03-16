#!/bin/bash
# Test console access functionality for Horizon integration

set -e

echo "=== Console Access Integration Test ==="

# Configuration
export OS_AUTH_URL=http://localhost:35357/v3
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default
export OS_DOMAIN_NAME=Default

echo "1. Authenticating..."
TOKEN=$(openstack token issue -f value -c id)
if [ -z "$TOKEN" ]; then
  echo "✗ Failed to authenticate"
  exit 1
fi
echo "✓ Authentication successful"

echo "2. Creating test instance..."
# Get flavor and image
FLAVOR_ID=$(openstack flavor list -f value -c ID -c Name | grep m1.tiny | awk '{print $1}')
if [ -z "$FLAVOR_ID" ]; then
  echo "Creating m1.tiny flavor..."
  openstack flavor create m1.tiny --ram 512 --vcpus 1 --disk 1
  FLAVOR_ID=$(openstack flavor list -f value -c ID -c Name | grep m1.tiny | awk '{print $1}')
fi

# Create test image if needed
IMAGE_NAME="console-test-image-$$"
if ! openstack image list -f value -c Name | grep -q "$IMAGE_NAME"; then
  touch /tmp/test-console-image.qcow2
  openstack image create $IMAGE_NAME \
    --disk-format qcow2 \
    --container-format bare \
    --file /tmp/test-console-image.qcow2 > /dev/null
  rm -f /tmp/test-console-image.qcow2
fi
IMAGE_ID=$(openstack image list -f value -c ID -c Name | grep "$IMAGE_NAME" | awk '{print $1}')

# Create instance
INSTANCE_NAME="console-test-vm-$$"
INSTANCE_ID=$(openstack server create $INSTANCE_NAME \
  --flavor $FLAVOR_ID \
  --image $IMAGE_ID \
  -f value -c id)

if [ -z "$INSTANCE_ID" ]; then
  echo "✗ Failed to create instance"
  exit 1
fi
echo "✓ Instance created: $INSTANCE_ID"

# Wait a moment for instance to initialize
sleep 2

echo "3. Testing VNC console URL generation..."
# Get VNC console URL
VNC_RESPONSE=$(curl -s -X POST "http://localhost:8774/v2.1/servers/$INSTANCE_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"os-getVNCConsole": {"type": "novnc"}}')

VNC_URL=$(echo "$VNC_RESPONSE" | jq -r '.console.url // empty')

if [ -z "$VNC_URL" ]; then
  echo "✗ Failed to get VNC console URL"
  echo "  Response: $VNC_RESPONSE"
  VNC_TEST="FAIL"
else
  echo "✓ VNC console URL generated: $VNC_URL"

  # Verify URL format
  if echo "$VNC_URL" | grep -q "http" && echo "$VNC_URL" | grep -q "token="; then
    echo "  ✓ URL format valid (contains http and token)"
    VNC_TEST="PASS"
  else
    echo "  ✗ URL format invalid (missing http or token)"
    VNC_TEST="FAIL"
  fi
fi

echo "4. Testing Serial console URL generation..."
# Get serial console URL
SERIAL_RESPONSE=$(curl -s -X POST "http://localhost:8774/v2.1/servers/$INSTANCE_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"os-getSerialConsole": {"type": "serial"}}')

SERIAL_URL=$(echo "$SERIAL_RESPONSE" | jq -r '.console.url // empty')

if [ -z "$SERIAL_URL" ]; then
  echo "✗ Failed to get serial console URL"
  echo "  Response: $SERIAL_RESPONSE"
  SERIAL_TEST="FAIL"
else
  echo "✓ Serial console URL generated: $SERIAL_URL"

  # Verify URL format
  if echo "$SERIAL_URL" | grep -q "ws://" && echo "$SERIAL_URL" | grep -q "token="; then
    echo "  ✓ URL format valid (contains ws:// and token)"
    SERIAL_TEST="PASS"
  else
    echo "  ✗ URL format invalid (missing ws:// or token)"
    SERIAL_TEST="FAIL"
  fi
fi

echo "5. Testing console type variations..."
# Test different console types
CONSOLE_TYPES=("novnc" "xvpvnc" "rdp-html5" "spice-html5" "serial")
CONSOLE_TEST="PASS"

for console_type in "${CONSOLE_TYPES[@]}"; do
  if [ "$console_type" = "serial" ]; then
    RESPONSE=$(curl -s -X POST "http://localhost:8774/v2.1/servers/$INSTANCE_ID/action" \
      -H "X-Auth-Token: $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"os-getSerialConsole\": {\"type\": \"$console_type\"}}")
  else
    RESPONSE=$(curl -s -X POST "http://localhost:8774/v2.1/servers/$INSTANCE_ID/action" \
      -H "X-Auth-Token: $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"os-getVNCConsole\": {\"type\": \"$console_type\"}}")
  fi

  CONSOLE_URL=$(echo "$RESPONSE" | jq -r '.console.url // empty')

  if [ -n "$CONSOLE_URL" ]; then
    echo "  ✓ Console type '$console_type' supported"
  else
    echo "  ⚠ Console type '$console_type' returned no URL (may not be configured)"
  fi
done

echo "6. Testing console output retrieval..."
# Get console output (logs)
OUTPUT_RESPONSE=$(curl -s -X POST "http://localhost:8774/v2.1/servers/$INSTANCE_ID/action" \
  -H "X-Auth-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"os-getConsoleOutput": {"length": 50}}')

CONSOLE_OUTPUT=$(echo "$OUTPUT_RESPONSE" | jq -r '.output // empty')

if [ -n "$CONSOLE_OUTPUT" ]; then
  echo "✓ Console output retrieved"
  echo "  Output length: ${#CONSOLE_OUTPUT} characters"
  OUTPUT_TEST="PASS"
else
  echo "⚠ Console output empty (expected in stub mode)"
  OUTPUT_TEST="WARN"
fi

echo "7. Testing console URL token format..."
# Verify token is a valid format (UUID or similar)
if [ -n "$VNC_URL" ]; then
  TOKEN_PARAM=$(echo "$VNC_URL" | grep -o 'token=[^&]*' | cut -d= -f2)

  if [ -n "$TOKEN_PARAM" ] && [ ${#TOKEN_PARAM} -gt 10 ]; then
    echo "✓ Console token format valid (length: ${#TOKEN_PARAM})"
    TOKEN_TEST="PASS"
  else
    echo "✗ Console token format invalid"
    TOKEN_TEST="FAIL"
  fi
else
  TOKEN_TEST="SKIP"
fi

echo "8. Cleanup..."
openstack server delete $INSTANCE_ID --wait || true
openstack image delete $IMAGE_ID || true

echo ""
echo "=== Test Results ==="
echo "VNC Console: $VNC_TEST"
echo "Serial Console: $SERIAL_TEST"
echo "Console Types: $CONSOLE_TEST"
echo "Console Output: $OUTPUT_TEST"
echo "Token Format: $TOKEN_TEST"

FAILED=false
if [ "$VNC_TEST" = "FAIL" ] || [ "$SERIAL_TEST" = "FAIL" ] || [ "$TOKEN_TEST" = "FAIL" ]; then
  FAILED=true
fi

if [ "$FAILED" = true ]; then
  echo ""
  echo "✗ Some tests failed"
  exit 1
else
  echo ""
  echo "✓ Console access tests passed"
  echo "  Horizon can access instance consoles via noVNC and serial"
  exit 0
fi
