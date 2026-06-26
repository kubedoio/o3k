#!/bin/sh
# O3K Bootstrap — Kolla-style initial resource creation
# Creates: service catalog endpoints, default network, CirrOS image, test VM
#
# Usage (called by install.sh automatically, or run standalone):
#   O3K_ADMIN_PASSWORD=secret /usr/local/bin/o3k-bootstrap
#
# Environment overrides:
#   O3K_ADMIN_PASSWORD   Admin password (required)
#   O3K_DATA_DIR         Data directory (default: /var/lib/o3k)
#   O3K_AUTH_URL         Keystone URL (default: http://localhost:35357/v3)
#   O3K_BOOTSTRAP_NET    Internal network CIDR (default: 192.168.100.0/24)
#   O3K_SKIP_VM          Set to "true" to skip VM creation
#   O3K_CIRROS_VERSION   CirrOS version (default: 0.6.2)

set -e

AUTH_URL="${O3K_AUTH_URL:-http://localhost:35357/v3}"
DATA_DIR="${O3K_DATA_DIR:-/var/lib/o3k}"
NET_CIDR="${O3K_BOOTSTRAP_NET:-192.168.100.0/24}"
CIRROS_VERSION="${O3K_CIRROS_VERSION:-0.6.2}"

# ─── Colours ──────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { printf "${GREEN}[bootstrap]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[bootstrap]${NC} %s\n" "$*"; }

# ─── Resolve admin password ───────────────────────────────────────────────────
ADMIN_PASS="${O3K_ADMIN_PASSWORD:-}"
if [ -z "$ADMIN_PASS" ] && [ -f "${DATA_DIR}/initial-password" ]; then
    ADMIN_PASS=$(cat "${DATA_DIR}/initial-password")
fi
if [ -z "$ADMIN_PASS" ]; then
    warn "Cannot bootstrap: no admin password. Set O3K_ADMIN_PASSWORD or ensure ${DATA_DIR}/initial-password exists."
    exit 1
fi

export OS_AUTH_URL="$AUTH_URL"
export OS_USERNAME=admin
export OS_PASSWORD="$ADMIN_PASS"
export OS_PROJECT_NAME=default
export OS_USER_DOMAIN_NAME=Default
export OS_PROJECT_DOMAIN_NAME=Default
export OS_IDENTITY_API_VERSION=3

# ─── Wait for Keystone ────────────────────────────────────────────────────────
info "Waiting for Keystone..."
i=0
while [ "$i" -lt 30 ]; do
    if curl -sf --max-time 2 "${AUTH_URL}" >/dev/null 2>&1; then
        break
    fi
    sleep 1
    i=$((i+1))
done
curl -sf --max-time 3 "${AUTH_URL}" >/dev/null 2>&1 || { warn "Keystone not responding at ${AUTH_URL}"; exit 1; }

# ─── Verify CLI auth works ────────────────────────────────────────────────────
if ! command -v openstack >/dev/null 2>&1; then
    warn "openstack CLI not found — skipping bootstrap resource creation."
    warn "Install with: apt-get install python3-openstackclient"
    exit 0
fi

if ! timeout 15 openstack token issue >/dev/null 2>&1; then
    warn "Could not authenticate with admin credentials — skipping bootstrap."
    exit 0
fi
info "Authentication OK."

# ─── Register service catalog endpoints ──────────────────────────────────────
# Detect server IP for public endpoints
MYIP=$(hostname -I 2>/dev/null | awk '{print $1}')
MYIP="${MYIP:-127.0.0.1}"

register_endpoint() {
    _svc_name=$1
    _svc_type=$2
    _pub_url=$3
    # If an endpoint already exists with the correct URL, nothing to do.
    _existing=$(timeout 10 openstack endpoint list --service "$_svc_name" --interface public -f value -c URL 2>/dev/null | head -1)
    if [ "$_existing" = "$_pub_url" ]; then
        return 0
    fi
    # Wrong URL or no endpoint — delete stale endpoints then recreate.
    if [ -n "$_existing" ]; then
        timeout 10 openstack endpoint list --service "$_svc_name" -f value -c ID 2>/dev/null | \
            xargs -r -I{} openstack endpoint delete {} 2>/dev/null || true
    fi
    # Get or create service
    _svc_id=$(timeout 10 openstack service list --type "$_svc_type" -f value -c ID 2>/dev/null | head -1)
    if [ -z "$_svc_id" ]; then
        _svc_id=$(timeout 10 openstack service create --name "$_svc_name" "$_svc_type" -f value -c id 2>/dev/null) || return 0
    fi
    for _iface in public internal admin; do
        timeout 10 openstack endpoint create \
            --region RegionOne \
            "$_svc_id" "$_iface" "$_pub_url" >/dev/null 2>&1 || true
    done
}

info "Registering service catalog endpoints..."
register_endpoint keystone  identity  "http://${MYIP}:35357/v3"
register_endpoint nova      compute   "http://${MYIP}:8774/v2.1"
register_endpoint neutron   network   "http://${MYIP}:9696"
register_endpoint cinder    volume    "http://${MYIP}:8776/v3"
register_endpoint cinderv3  volumev3  "http://${MYIP}:8776/v3"
register_endpoint glance    image     "http://${MYIP}:9292/v2"
register_endpoint placement placement "http://${MYIP}:8778"
info "Service catalog ready."

# ─── Default network ──────────────────────────────────────────────────────────
info "Creating default network..."
if timeout 10 openstack network show default-net >/dev/null 2>&1; then
    info "  network 'default-net' already exists — skipping."
else
    NET_ID=$(timeout 15 openstack network create default-net -f value -c id 2>&1) || \
        { warn "  Failed to create network: $NET_ID"; NET_ID=""; }
    if [ -n "$NET_ID" ]; then
        SUBNET_ID=$(timeout 15 openstack subnet create default-subnet \
            --network default-net \
            --subnet-range "$NET_CIDR" \
            --dns-nameserver 8.8.8.8 \
            -f value -c id 2>&1) || \
            { warn "  Failed to create subnet: $SUBNET_ID"; SUBNET_ID=""; }
        [ -n "$SUBNET_ID" ] && info "  Network ready: default-net ($NET_CIDR)"
    fi
fi

# ─── CirrOS image ─────────────────────────────────────────────────────────────
CIRROS_NAME="cirros-${CIRROS_VERSION}"
CIRROS_URL="https://download.cirros-cloud.net/${CIRROS_VERSION}/cirros-${CIRROS_VERSION}-x86_64-disk.img"
CIRROS_LOCAL="${DATA_DIR}/cirros-${CIRROS_VERSION}.img"

if timeout 10 openstack image show "$CIRROS_NAME" >/dev/null 2>&1; then
    info "Image '${CIRROS_NAME}' already exists — skipping."
else
    info "Downloading CirrOS ${CIRROS_VERSION} (~20MB)..."
    if curl -fL --connect-timeout 15 --max-time 120 --progress-bar \
            "$CIRROS_URL" -o "$CIRROS_LOCAL" 2>&1; then
        info "Uploading CirrOS to Glance..."
        IMG_ID=$(timeout 60 openstack image create "$CIRROS_NAME" \
            --file "$CIRROS_LOCAL" \
            --disk-format qcow2 \
            --container-format bare \
            --public \
            -f value -c id 2>&1) || { warn "  Image upload failed: $IMG_ID"; IMG_ID=""; }
        [ -n "$IMG_ID" ] && info "  Image ready: ${CIRROS_NAME} (${IMG_ID})"
        # Keep local copy for Nova (real mode needs it on disk)
        mkdir -p "${DATA_DIR}/images"
        cp "$CIRROS_LOCAL" "${DATA_DIR}/images/${IMG_ID}.qcow2" 2>/dev/null || true
        # Allow libvirt-qemu (non-root) to read the image and traverse parent dirs
        chmod o+x "${DATA_DIR}" "${DATA_DIR}/images" 2>/dev/null || true
        chmod 644 "${DATA_DIR}/images/${IMG_ID}.qcow2" 2>/dev/null || true
    else
        warn "  CirrOS download failed — skipping image creation."
        warn "  Manually: openstack image create cirros --file /path/to/cirros.img --disk-format qcow2 --container-format bare --public"
    fi
fi

# ─── Test VM ──────────────────────────────────────────────────────────────────
if [ "${O3K_SKIP_VM:-false}" = "true" ]; then
    info "Skipping VM creation (O3K_SKIP_VM=true)."
else
    if timeout 10 openstack server show test-vm >/dev/null 2>&1; then
        info "Server 'test-vm' already exists — skipping."
    else
        # Verify image and network exist before attempting
        IMG_ID=$(timeout 10 openstack image show "$CIRROS_NAME" -f value -c id 2>/dev/null)
        if [ -z "$IMG_ID" ]; then
            warn "  Image '${CIRROS_NAME}' not found — skipping VM creation."
        else
            info "Creating test VM 'test-vm' (m1.tiny, ${CIRROS_NAME})..."
            VM_ID=$(timeout 30 openstack server create test-vm \
                --flavor m1.tiny \
                --image "$CIRROS_NAME" \
                --network default-net \
                --wait \
                -f value -c id 2>&1) || { warn "  VM creation failed: $VM_ID"; VM_ID=""; }
            if [ -n "$VM_ID" ]; then
                VM_STATUS=$(timeout 10 openstack server show test-vm -f value -c status 2>/dev/null)
                info "  VM ready: test-vm (${VM_STATUS})"
            fi
        fi
    fi
fi

info "Bootstrap complete."
printf "\n"
printf "  Resources created:\n"
printf "    Network:  default-net (%s)\n" "$NET_CIDR"
printf "    Image:    %s\n" "$CIRROS_NAME"
[ "${O3K_SKIP_VM:-false}" != "true" ] && printf "    Server:   test-vm (m1.tiny)\n"
printf "\n"
