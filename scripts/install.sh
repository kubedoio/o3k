#!/bin/sh
# O3K Install Script
# Usage:
#   curl -sfL https://get.o3k.io | sh -
#   curl -sfL https://get.o3k.io/v0.2.0 | sh -
#
# Environment variable overrides:
#   O3K_VERSION          Pin to specific version (default: latest)
#   O3K_INSTALL_DIR      Binary install path (default: /usr/local/bin)
#   O3K_DATA_DIR         Data/state directory (default: /var/lib/o3k)
#   O3K_ADMIN_PASSWORD   Set admin password (default: auto-generated)
#   O3K_SKIP_SERVICE     Set to "true" to skip systemd setup
#   O3K_FORCE_CONFIG     Set to "true" to overwrite existing config
#   O3K_NO_HORIZON         Set to "true" to skip Horizon dashboard install
#   O3K_HORIZON_PORT       Port for Horizon dashboard (default: 8080)

set -e

GITHUB_REPO="kubedoio/o3k"
INSTALL_DIR="${O3K_INSTALL_DIR:-/usr/local/bin}"
DATA_DIR="${O3K_DATA_DIR:-/var/lib/o3k}"
CONFIG_DIR="/etc/o3k"
CONFIG_FILE="$CONFIG_DIR/config.yaml"

# ─── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { printf "${GREEN}[INFO]${NC}  %s\n" "$*"; }
warn()  { printf "${YELLOW}[WARN]${NC}  %s\n" "$*"; }
fatal() { printf "${RED}[ERROR]${NC} %s\n" "$*" >&2; exit 1; }

# ─── Phase 1: Preflight ───────────────────────────────────────────────────────
info "O3K installer — preflight checks"

# Must be root
[ "$(id -u)" -eq 0 ] || fatal "Run as root: sudo sh -c '\$(curl -sfL https://get.o3k.io)'"

# Linux only for service install
OS=$(uname -s)
[ "$OS" = "Linux" ] || fatal "Service install is Linux-only. On macOS, run o3k manually in stub mode."

# Architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *) fatal "Unsupported architecture: $ARCH" ;;
esac

# KVM available
if [ ! -e /dev/kvm ]; then
    fatal "/dev/kvm not found. Enable virtualisation in BIOS or nested virt in your hypervisor."
fi
if [ ! -w /dev/kvm ]; then
    warn "/dev/kvm not writable as root — this is unusual. Continuing anyway."
fi

# Required ports free
check_port() {
    _port=$1
    # Match port on any bind address (0.0.0.0, 127.0.0.1, ::, etc.)
    if command -v ss >/dev/null 2>&1; then
        if ss -ltn 2>/dev/null | awk '{print $4}' | grep -q ":${_port}$"; then
            fatal "Port $_port is already in use. Free it before installing o3k."
        fi
    elif command -v netstat >/dev/null 2>&1; then
        if netstat -ltn 2>/dev/null | awk '{print $4}' | grep -q ":${_port}$"; then
            fatal "Port $_port is already in use. Free it before installing o3k."
        fi
    fi
}
for port in 35357 8774 8775 8776 8778 9292 9696; do
    check_port "$port"
done

# Disk space (2 GB minimum in /var/lib)
AVAIL_KB=$(df /var/lib 2>/dev/null | awk 'NR==2{print $4}')
AVAIL_KB="${AVAIL_KB:-9999999}"
[ "$AVAIL_KB" -ge 2097152 ] || fatal "Insufficient disk space in /var/lib. Need at least 2 GB free."

info "Preflight passed."

# ─── Phase 2: Dependencies ────────────────────────────────────────────────────
info "Checking dependencies..."

install_packages_apt() {
    info "Installing packages via apt-get..."
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
        libvirt-daemon-system \
        qemu-kvm \
        qemu-utils \
        curl \
        openssl
}

install_packages_dnf() {
    info "Installing packages via dnf..."
    dnf install -y -q \
        libvirt-daemon \
        qemu-kvm \
        qemu-img \
        curl \
        openssl
}

# Detect package manager and install deps if needed
NEED_INSTALL=0
command -v virsh     >/dev/null 2>&1 || NEED_INSTALL=1
command -v qemu-img  >/dev/null 2>&1 || NEED_INSTALL=1

if [ "$NEED_INSTALL" -eq 1 ]; then
    if command -v apt-get >/dev/null 2>&1; then
        install_packages_apt
    elif command -v dnf >/dev/null 2>&1; then
        install_packages_dnf
    else
        fatal "Cannot auto-install dependencies: no apt-get or dnf found. Install libvirt-daemon, qemu-kvm, qemu-utils manually."
    fi
fi

# Ensure libvirtd is running
if command -v systemctl >/dev/null 2>&1; then
    systemctl enable --now libvirtd 2>/dev/null || systemctl enable --now libvirt-daemon 2>/dev/null || true
fi

# Verify libvirt is actually working
if ! virsh -c qemu:///system version >/dev/null 2>&1; then
    fatal "libvirt is installed but 'virsh -c qemu:///system version' failed. Check that libvirtd is running: systemctl status libvirtd"
fi

info "Dependencies OK."

# ─── Phase 3: Download binary ─────────────────────────────────────────────────
VERSION="${O3K_VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
    info "Resolving latest version..."
    VERSION=$(curl -sfL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
        | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    [ -n "$VERSION" ] || fatal "Could not determine latest version. Set O3K_VERSION explicitly."
fi
info "Installing O3K $VERSION (linux/$ARCH)..."

BASE_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}"
TMP_DIR=$(mktemp -d) || fatal "Failed to create temporary directory"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

# Download binary + checksums
info "Downloading o3k-linux-${ARCH}..."
curl -fL --connect-timeout 30 --max-time 300 --progress-bar \
    "${BASE_URL}/o3k-linux-${ARCH}" -o "${TMP_DIR}/o3k" || \
    fatal "Failed to download binary from ${BASE_URL}/o3k-linux-${ARCH}"
info "Downloading checksums..."
curl -sfL --connect-timeout 30 --max-time 30 \
    "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt" || \
    fatal "Failed to download checksums.txt"

# Verify SHA256
EXPECTED=$(grep "o3k-linux-${ARCH}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
[ -n "$EXPECTED" ] || fatal "No checksum entry found for o3k-linux-${ARCH} in checksums.txt"
ACTUAL=$(sha256sum "${TMP_DIR}/o3k" | awk '{print $1}')
[ "$ACTUAL" = "$EXPECTED" ] || fatal "SHA256 mismatch! Expected $EXPECTED, got $ACTUAL. Aborting."

chmod +x "${TMP_DIR}/o3k"
mkdir -p "$INSTALL_DIR"
mv "${TMP_DIR}/o3k" "${INSTALL_DIR}/o3k"
rm -rf "$TMP_DIR"
info "Binary installed: ${INSTALL_DIR}/o3k ($VERSION)"

# ─── Phase 4: Config generation ───────────────────────────────────────────────
mkdir -p "$CONFIG_DIR" "$DATA_DIR"
chmod 700 "$CONFIG_DIR"

if [ -f "$CONFIG_FILE" ] && [ "${O3K_FORCE_CONFIG:-false}" != "true" ]; then
    info "Config already exists at $CONFIG_FILE — skipping generation (set O3K_FORCE_CONFIG=true to overwrite)"
else
    info "Generating config at $CONFIG_FILE..."
    JWT_SECRET=$(openssl rand -base64 48)
    cat > "$CONFIG_FILE" <<EOF
# O3K configuration — generated by installer $VERSION
# To regenerate: O3K_FORCE_CONFIG=true curl -sfL https://get.o3k.io | sh -

database:
  datastore: "sqlite://${DATA_DIR}/state.db"

keystone:
  port: 35357
  jwt_secret: "${JWT_SECRET}"
  token_ttl: 24h
  admin_user: admin

nova:
  port: 8774
  libvirt_uri: "qemu:///system"
  libvirt_mode: real

neutron:
  port: 9696
  networking_mode: stub

cinder:
  port: 8776
  storage_mode: local

glance:
  port: 9292
  storage_mode: local

placement:
  port: 8778

metadata:
  port: 8775

server:
  bind_host: "0.0.0.0"
EOF
    chmod 600 "$CONFIG_FILE"
    info "Config written (JWT secret embedded, file mode 600)."
fi

# ─── Phase 5: Systemd service ─────────────────────────────────────────────────
if [ "${O3K_SKIP_SERVICE:-false}" = "true" ]; then
    info "Skipping service setup (O3K_SKIP_SERVICE=true)."
    info "Run manually: ${INSTALL_DIR}/o3k --config ${CONFIG_FILE}"
    exit 0
fi

if ! command -v systemctl >/dev/null 2>&1; then
    warn "No systemd detected. Start manually: ${INSTALL_DIR}/o3k --config ${CONFIG_FILE}"
    exit 0
fi

cat > /etc/systemd/system/o3k.service <<EOF
[Unit]
Description=O3K OpenStack Server
Documentation=https://github.com/kubedoio/o3k
After=network-online.target libvirtd.service
Wants=network-online.target
Requires=libvirtd.service

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/o3k --config ${CONFIG_FILE}
Environment=O3K_DATA_DIR=${DATA_DIR}
Environment=O3K_ADMIN_PASSWORD=${O3K_ADMIN_PASSWORD:-}
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=60
StartLimitBurst=3
LimitNOFILE=65535
PrivateTmp=true
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now o3k
info "Service enabled and started."

# ─── Phase 5b: Horizon dashboard (opt-in) ─────────────────────────────────────
if [ "${O3K_NO_HORIZON:-false}" != "true" ]; then
    HORIZON_PORT="${O3K_HORIZON_PORT:-8080}"
    info "Installing Horizon dashboard on port $HORIZON_PORT (set O3K_NO_HORIZON=true to skip)..."

    # Install Docker if not present
    if ! command -v docker >/dev/null 2>&1; then
        info "Docker not found — installing..."
        if command -v apt-get >/dev/null 2>&1; then
            DEBIAN_FRONTEND=noninteractive apt-get install -y -qq docker.io
        elif command -v dnf >/dev/null 2>&1; then
            dnf install -y -q docker
        else
            fatal "Cannot install Docker: no apt-get or dnf found. Install Docker manually and re-run with O3K_HORIZON=true."
        fi
        systemctl enable --now docker
    fi

    # Verify Docker is working
    docker info >/dev/null 2>&1 || fatal "Docker is installed but not running. Start it with: systemctl start docker"

    # Write Horizon local_settings.py using printf to avoid heredoc indentation issues
    HORIZON_SETTINGS="/etc/o3k/horizon-local_settings.py"
    HORIZON_SECRET=$(openssl rand -hex 32)
    MYIP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")

    printf '%s\n' \
        'import os' \
        'OPENSTACK_KEYSTONE_URL = "http://127.0.0.1:35357/v3"' \
        'OPENSTACK_ENDPOINT_TYPE = "publicURL"' \
        "SECRET_KEY = \"${HORIZON_SECRET}\"" \
        'ALLOWED_HOSTS = ["*"]' \
        'USE_X_FORWARDED_HOST = True' \
        'USE_X_FORWARDED_PORT = True' \
        'SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")' \
        'OPENSTACK_API_VERSIONS = {"identity": 3, "image": 2, "volume": 3}' \
        'OPENSTACK_KEYSTONE_MULTIDOMAIN_SUPPORT = True' \
        'OPENSTACK_KEYSTONE_DEFAULT_DOMAIN = "Default"' \
        'OPENSTACK_KEYSTONE_DEFAULT_ROLE = "member"' \
        'SESSION_ENGINE = "django.contrib.sessions.backends.cache"' \
        'CACHES = {"default": {"BACKEND": "django.core.cache.backends.locmem.LocMemCache"}}' \
        'COMPRESS_OFFLINE = False' \
        'DEBUG = False' \
        'SESSION_COOKIE_SECURE = False' \
        'CSRF_COOKIE_SECURE = False' \
        'SESSION_COOKIE_SAMESITE = None' \
        'CSRF_COOKIE_SAMESITE = None' \
        'SESSION_SAVE_EVERY_REQUEST = True' \
        'SESSION_ENGINE = "django.contrib.sessions.backends.db"' \
        'DATABASES = {"default": {"ENGINE": "django.db.backends.sqlite3", "NAME": "/tmp/horizon_sessions.db"}}' \
        'LOGIN_REDIRECT_URL = "/project/"' \
        > "$HORIZON_SETTINGS"
    chmod 644 "$HORIZON_SETTINGS"

    # Write the Apache vhost config for Horizon (not shipped in the Kolla image)
    HORIZON_APACHE_CONF="/etc/o3k/horizon-apache.conf"
    cat > "$HORIZON_APACHE_CONF" <<'EOF'
WSGIScriptAlias / /var/lib/kolla/venv/lib/python3.12/site-packages/openstack_dashboard/wsgi.py
WSGIDaemonProcess horizon processes=3 threads=10 home=/var/lib/kolla/venv python-home=/var/lib/kolla/venv
WSGIProcessGroup horizon
WSGIApplicationGroup %{GLOBAL}
WSGIPassAuthorization On

<Directory /var/lib/kolla/venv/lib/python3.12/site-packages/openstack_dashboard>
    <Files wsgi.py>
        Require all granted
    </Files>
</Directory>

Alias /static /var/lib/kolla/venv/lib/python3.12/site-packages/static
<Directory /var/lib/kolla/venv/lib/python3.12/site-packages/static>
    Options -Indexes
    Require all granted
</Directory>
EOF

    # Stop existing container if running
    docker rm -f o3k-horizon 2>/dev/null || true

    # Pull and start Horizon
    info "Pulling Horizon image (this may take a few minutes)..."
    docker pull quay.io/openstack.kolla/horizon:2025.1-ubuntu-noble

    # Run Horizon bypassing kolla_set_configs entirely.
    # We mount local_settings.py directly and start Apache ourselves.
    # The Kolla image ships an empty ports.conf — we must inject Listen 80.
    docker run -d \
        --name o3k-horizon \
        --restart unless-stopped \
        --network host \
        -v "$HORIZON_SETTINGS:/etc/openstack-dashboard/local_settings.py" \
        -v "$HORIZON_APACHE_CONF:/etc/apache2/sites-available/horizon.conf" \
        -v "$HORIZON_APACHE_CONF:/etc/apache2/sites-enabled/horizon.conf" \
        --entrypoint /bin/bash \
        quay.io/openstack.kolla/horizon:2025.1-ubuntu-noble \
        -c "umask 000 && mkdir -p /var/log/apache2 /var/run/apache2 /var/lib/kolla/venv/lib/python3.12/site-packages/static/dashboard/css /var/lib/kolla/venv/lib/python3.12/site-packages/static/dashboard/js /var/lib/kolla/venv/lib/python3.12/site-packages/openstack_dashboard/local/local_settings.d && printf 'CACHES={\"default\":{\"BACKEND\":\"django.core.cache.backends.locmem.LocMemCache\",\"LOCATION\":\"horizon\"}}\n' > /var/lib/kolla/venv/lib/python3.12/site-packages/openstack_dashboard/local/local_settings.d/_99_o3k_cache.py && chmod 644 /etc/openstack-dashboard/local_settings.py && DJANGO_SETTINGS_MODULE=openstack_dashboard.settings /var/lib/kolla/venv/bin/python /var/lib/kolla/venv/bin/manage.py collectstatic --noinput -v0 && DJANGO_SETTINGS_MODULE=openstack_dashboard.settings /var/lib/kolla/venv/bin/python /var/lib/kolla/venv/bin/manage.py migrate --run-syncdb 2>/dev/null; touch /tmp/horizon_sessions.db && chmod 666 /tmp/horizon_sessions.db && echo 'Listen ${HORIZON_PORT}' > /etc/apache2/ports.conf && apache2ctl -DFOREGROUND"

    # Write systemd unit for horizon so it starts on boot independently of Docker restart policy
    cat > /etc/systemd/system/o3k-horizon.service <<EOF
[Unit]
Description=O3K Horizon Dashboard
After=docker.service o3k.service
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/docker start o3k-horizon
ExecStop=/usr/bin/docker stop o3k-horizon
Restart=no

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable o3k-horizon
    info "Horizon service enabled."
fi

# ─── Phase 6: Wait for ready + print credentials ──────────────────────────────
info "Waiting for O3K to start (up to 30s)..."
i=0
while [ "$i" -lt 30 ]; do
    if curl -sf http://localhost:35357/v3 >/dev/null 2>&1; then
        PASS=""
        [ -f "${DATA_DIR}/initial-password" ] && PASS=$(cat "${DATA_DIR}/initial-password")
        printf "\n"
        printf "════════════════════════════════════════════════\n"
        printf "  O3K %s installed and running\n" "$VERSION"
        printf "════════════════════════════════════════════════\n"
        printf "  Keystone:  http://localhost:35357/v3\n"
        printf "  Nova:      http://localhost:8774/v2.1\n"
        printf "  Glance:    http://localhost:9292/v2\n"
        if [ "${O3K_NO_HORIZON:-false}" != "true" ]; then
            MYIP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")
            printf "  Horizon:   http://%s:%s (admin dashboard)\n" "$MYIP" "${O3K_HORIZON_PORT:-8080}"
        fi
        if [ -n "$PASS" ]; then
            printf "  User:      admin\n"
            printf "  Password:  %s\n" "$PASS"
        fi
        printf "────────────────────────────────────────────────\n"
        printf "  Quick start (requires python-openstackclient):\n"
        printf "\n"
        printf "    export OS_AUTH_URL=http://localhost:35357/v3\n"
        printf "    export OS_USERNAME=admin\n"
        [ -n "$PASS" ] && printf "    export OS_PASSWORD=%s\n" "$PASS"
        printf "    export OS_PROJECT_NAME=default\n"
        printf "    export OS_USER_DOMAIN_NAME=Default\n"
        printf "    export OS_PROJECT_DOMAIN_NAME=Default\n"
        printf "    openstack token issue\n"
        printf "════════════════════════════════════════════════\n"
        exit 0
    fi
    sleep 1
    i=$((i + 1))
done

warn "O3K service started but API not responding after 30s."
warn "Check logs: journalctl -u o3k -f"
warn "Manual check: curl http://localhost:35357/v3"
