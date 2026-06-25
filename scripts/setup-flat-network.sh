#!/usr/bin/env bash
# setup-flat-network.sh — idempotent flat network setup for O3K single-host deployments.
# Run as root. Safe to re-run: all operations check existing state first.
set -euo pipefail

BRIDGE="${O3K_FLAT_BRIDGE:-br-o3k}"
BRIDGE_IP="${O3K_FLAT_GATEWAY:-192.168.100.1}"
BRIDGE_PREFIX="${O3K_FLAT_PREFIX:-24}"
SUBNET="${O3K_FLAT_SUBNET:-192.168.100.0/24}"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { printf "${GREEN}[INFO]${NC}  %s\n" "$*"; }
warn() { printf "${YELLOW}[WARN]${NC}  %s\n" "$*"; }
fatal() { printf "\033[0;31m[FATAL]\033[0m  %s\n" "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fatal "Must run as root"

# ─── 1. Create bridge ─────────────────────────────────────────────────────────
if ip link show "$BRIDGE" &>/dev/null; then
    info "Bridge $BRIDGE already exists — skipping creation."
else
    info "Creating bridge $BRIDGE..."
    ip link add name "$BRIDGE" type bridge
    ip link set "$BRIDGE" up
    ip addr add "${BRIDGE_IP}/${BRIDGE_PREFIX}" dev "$BRIDGE"
    info "Bridge $BRIDGE created with IP ${BRIDGE_IP}/${BRIDGE_PREFIX}."
fi

# ─── 2. IP forwarding ─────────────────────────────────────────────────────────
info "Enabling IP forwarding..."
sysctl -w net.ipv4.ip_forward=1 >/dev/null
cat > /etc/sysctl.d/99-o3k-forward.conf <<'SYSCTL'
net.ipv4.ip_forward = 1
SYSCTL
info "IP forwarding enabled."

# ─── 3. NAT masquerade ────────────────────────────────────────────────────────
OUTNIC=$(ip route show default 2>/dev/null | awk '/default/ {print $5}' | head -1)
if [ -z "$OUTNIC" ]; then
    warn "No default route found — MASQUERADE rule not added. Configure manually."
else
    info "Adding MASQUERADE rule for $SUBNET via $OUTNIC..."
    iptables -t nat -C POSTROUTING -s "$SUBNET" -o "$OUTNIC" -j MASQUERADE 2>/dev/null || \
        iptables -t nat -A POSTROUTING -s "$SUBNET" -o "$OUTNIC" -j MASQUERADE
    info "MASQUERADE rule active."
fi

# ─── 4. Persist bridge ────────────────────────────────────────────────────────
if command -v netplan &>/dev/null; then
    info "Writing netplan config for $BRIDGE..."
    cat > /etc/netplan/99-o3k-bridge.yaml <<NETPLAN
network:
  version: 2
  bridges:
    ${BRIDGE}:
      addresses: [${BRIDGE_IP}/${BRIDGE_PREFIX}]
      parameters:
        stp: false
        forward-delay: 0
NETPLAN
    netplan apply 2>/dev/null || warn "netplan apply failed — bridge IP may not persist on reboot."
elif [ -d /etc/systemd/network ]; then
    info "Writing systemd-networkd config for $BRIDGE..."
    cat > "/etc/systemd/network/10-${BRIDGE}.netdev" <<NETDEV
[NetDev]
Name=${BRIDGE}
Kind=bridge
NETDEV
    cat > "/etc/systemd/network/10-${BRIDGE}.network" <<NETNET
[Match]
Name=${BRIDGE}
[Network]
Address=${BRIDGE_IP}/${BRIDGE_PREFIX}
NETNET
    systemctl restart systemd-networkd 2>/dev/null || warn "Failed to restart systemd-networkd."
else
    warn "No netplan or systemd-networkd found. Bridge $BRIDGE will not persist on reboot."
fi

# ─── 5. Persist iptables ──────────────────────────────────────────────────────
if command -v iptables-save &>/dev/null && [ -d /etc/iptables ]; then
    iptables-save > /etc/iptables/rules.v4
    info "iptables rules persisted to /etc/iptables/rules.v4"
fi

info ""
info "Flat network setup complete."
info "  Bridge:  $BRIDGE  (${BRIDGE_IP}/${BRIDGE_PREFIX})"
info "  NAT via: ${OUTNIC:-(none — configure manually)}"
info ""
info "Next: set networking_mode: flat in /etc/o3k/config.yaml and restart o3k."
