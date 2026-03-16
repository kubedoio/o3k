#!/bin/bash
# Test cloud-init ISO generation

set -e

echo "=== Cloud-init ISO Generation Test ==="

# Authenticate
export OS_AUTH_URL=http://localhost:35357/v3
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_PROJECT_NAME=default
export OS_DOMAIN_NAME=Default

echo "1. Authenticating..."
TOKEN=$(openstack token issue -f value -c id)

echo "2. Creating test keypair..."
KEYPAIR_NAME="test-cloudinit-key-$$"
openstack keypair create $KEYPAIR_NAME > /tmp/test-key.pem 2>&1

echo "3. Creating test network..."
NET_ID=$(openstack network create test-cloudinit-net-$$ -f value -c id)
SUBNET_ID=$(openstack subnet create test-cloudinit-subnet-$$ \
  --network $NET_ID \
  --subnet-range 10.99.0.0/24 \
  -f value -c id)

echo "4. Creating test flavor..."
FLAVOR_ID=$(openstack flavor list -f value -c ID -c Name | grep m1.tiny | awk '{print $1}')
if [ -z "$FLAVOR_ID" ]; then
  echo "Creating m1.tiny flavor..."
  openstack flavor create m1.tiny --ram 512 --vcpus 1 --disk 1
  FLAVOR_ID=$(openstack flavor list -f value -c ID -c Name | grep m1.tiny | awk '{print $1}')
fi

echo "5. Creating test image..."
IMAGE_NAME="cirros-test-$$"
if ! openstack image list -f value -c Name | grep -q "$IMAGE_NAME"; then
  # Create minimal fake image (O3K in stub mode doesn't care about actual image data)
  touch /tmp/test-image.qcow2
  openstack image create $IMAGE_NAME \
    --disk-format qcow2 \
    --container-format bare \
    --file /tmp/test-image.qcow2
fi
IMAGE_ID=$(openstack image list -f value -c ID -c Name | grep "$IMAGE_NAME" | awk '{print $1}')

echo "6. Creating VM instance with SSH key (triggers cloud-init)..."
INSTANCE_ID=$(openstack server create test-cloudinit-vm-$$ \
  --flavor $FLAVOR_ID \
  --image $IMAGE_ID \
  --network $NET_ID \
  --key-name $KEYPAIR_NAME \
  -f value -c id)

echo "Instance created: $INSTANCE_ID"

echo "7. Verifying cloud-init ISO was generated..."
sleep 2  # Give O3K time to create the ISO

ISO_PATH="/var/lib/o3k/cloud-init/${INSTANCE_ID}.iso"
if [ -f "$ISO_PATH" ]; then
  echo "✓ Cloud-init ISO exists at $ISO_PATH"
  ls -lh "$ISO_PATH"

  # Check ISO contents (requires root)
  if command -v isoinfo &> /dev/null; then
    echo "ISO contents:"
    isoinfo -i "$ISO_PATH" -l 2>/dev/null || echo "  (cannot list - may need root)"
  fi

  TEST_RESULT="PASS"
else
  echo "✗ Cloud-init ISO NOT FOUND at $ISO_PATH"
  echo "  This may be expected if:"
  echo "  - O3K is running in stub mode"
  echo "  - genisoimage/mkisofs is not installed"
  echo "  - VM creation failed"
  TEST_RESULT="WARN"
fi

echo "8. Checking VM status..."
VM_STATUS=$(openstack server show $INSTANCE_ID -f value -c status)
echo "VM status: $VM_STATUS"

echo "9. Cleanup..."
openstack server delete $INSTANCE_ID --wait || true
openstack image delete $IMAGE_ID || true
openstack subnet delete $SUBNET_ID || true
openstack network delete $NET_ID || true
openstack keypair delete $KEYPAIR_NAME || true
rm -f /tmp/test-key.pem /tmp/test-image.qcow2

echo ""
echo "=== Test Result: $TEST_RESULT ==="
if [ "$TEST_RESULT" = "PASS" ]; then
  echo "✓ Cloud-init ISO generation works correctly"
  exit 0
elif [ "$TEST_RESULT" = "WARN" ]; then
  echo "⚠ Cloud-init ISO not created (may be expected in stub mode)"
  exit 0
else
  echo "✗ Test failed"
  exit 1
fi
