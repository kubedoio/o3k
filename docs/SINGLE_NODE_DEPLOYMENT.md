# Single-Node O3K Deployment with KVM

**Purpose**: Deploy O3K on a single host with KVM hypervisor for demonstration and testing
**Target Audience**: Developers, evaluators, demo environments
**Production Ready**: No - for single-node demos only (see SCALING.md for production)

---

## Overview

This guide deploys O3K with **real mode** on a single Linux host with KVM, enabling full virtualization capabilities for demonstrating OpenStack functionality via Horizon dashboard.

**What You Get**:
- ✅ O3K services (Keystone, Nova, Neutron, Cinder, Glance) in real mode
- ✅ KVM hypervisor for actual VM creation
- ✅ Horizon dashboard for web UI access
- ✅ noVNC console for VM access
- ✅ Real networking with Linux bridges
- ✅ Local storage backend
- ✅ PostgreSQL database

**Architecture**:
```
┌─────────────────────────────────────────────────────────────┐
│                     Single Host (Linux)                      │
├─────────────────────────────────────────────────────────────┤
│  Horizon Dashboard         :80                              │
│  O3K Services                                                │
│    ├─ Keystone            :35357                            │
│    ├─ Nova                :8774                             │
│    ├─ Neutron             :9696                             │
│    ├─ Cinder              :8776                             │
│    ├─ Glance              :9292                             │
│    └─ Metadata            :8775                             │
│  noVNC Proxy              :6080                             │
│  PostgreSQL               :5432                             │
│                                                              │
│  KVM/libvirt              (VMs run here)                    │
│  Linux Networking         (bridges, namespaces)             │
│  Local Storage            (/var/lib/o3k/volumes)            │
└─────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

### 1. Hardware Requirements

**Minimum** (for basic demos):
- CPU: 4 cores with VT-x/AMD-V virtualization support
- RAM: 16 GB
- Disk: 100 GB SSD
- Network: 1 Gbps NIC

**Recommended** (for realistic demos):
- CPU: 8 cores with VT-x/AMD-V
- RAM: 32 GB
- Disk: 250 GB NVMe SSD
- Network: 10 Gbps NIC

**Verify Virtualization Support**:
```bash
# Check CPU virtualization extensions
egrep -c '(vmx|svm)' /proc/cpuinfo
# Output > 0 means supported

# Check if KVM modules are loaded
lsmod | grep kvm
# Expected: kvm_intel (Intel) or kvm_amd (AMD)
```

### 2. Operating System

**Supported**:
- Ubuntu 24.04 LTS (recommended)
- Ubuntu 22.04 LTS
- Debian 12
- RHEL 9 / Rocky Linux 9

**This guide uses Ubuntu 24.04 LTS.**

### 3. Network Configuration

**Requirements**:
- Static IP address on host
- Internet access for package downloads
- No conflicting services on ports: 35357, 8774-8776, 9292, 9696, 6080, 80

**Example Network Setup**:
```
Host: 192.168.1.100/24
Gateway: 192.168.1.1
DNS: 8.8.8.8, 8.8.4.4
```

---

## Installation Steps

### Step 1: System Preparation

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Set hostname
sudo hostnamectl set-hostname o3k-demo

# Configure /etc/hosts
echo "127.0.0.1 o3k-demo" | sudo tee -a /etc/hosts
echo "192.168.1.100 o3k-demo" | sudo tee -a /etc/hosts  # Replace with your IP
```

### Step 2: Install KVM and Dependencies

```bash
# Install KVM and libvirt
sudo apt install -y \
    qemu-kvm \
    libvirt-daemon-system \
    libvirt-clients \
    bridge-utils \
    virt-manager \
    cpu-checker

# Verify KVM is working
sudo kvm-ok
# Expected output: "KVM acceleration can be used"

# Add your user to libvirt groups
sudo usermod -aG libvirt,kvm $USER
newgrp libvirt

# Start and enable libvirt
sudo systemctl enable --now libvirtd
sudo systemctl status libvirtd

# Verify libvirt connection
virsh list --all
# Should list VMs (empty initially)
```

### Step 3: Configure Networking

**Create Bridge for External Network**:

```bash
# Install network tools
sudo apt install -y bridge-utils net-tools

# Create bridge configuration
# Note: This assumes eth0 is your primary interface
# Adjust if using different interface name (ip link show)

sudo tee /etc/netplan/01-o3k-bridge.yaml <<EOF
network:
  version: 2
  renderer: networkd
  ethernets:
    eth0:
      dhcp4: no
      dhcp6: no
  bridges:
    br-ext:
      interfaces: [eth0]
      addresses:
        - 192.168.1.100/24
      routes:
        - to: default
          via: 192.168.1.1
      nameservers:
        addresses: [8.8.8.8, 8.8.4.4]
      dhcp4: no
      dhcp6: no
      parameters:
        stp: false
        forward-delay: 0
EOF

# Apply network configuration
sudo netplan apply

# Verify bridge exists
ip addr show br-ext
brctl show
```

**Enable IP Forwarding**:

```bash
# Enable IP forwarding permanently
sudo tee -a /etc/sysctl.conf <<EOF
net.ipv4.ip_forward=1
net.ipv4.conf.all.forwarding=1
net.ipv6.conf.all.forwarding=1
EOF

# Apply sysctl changes
sudo sysctl -p
```

### Step 4: Install Docker and Docker Compose

```bash
# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Install Docker Compose
sudo apt install -y docker-compose-plugin

# Verify installation
docker --version
docker compose version
```

### Step 5: Install PostgreSQL

```bash
# Install PostgreSQL 18
sudo apt install -y postgresql-18 postgresql-client-18

# Start and enable PostgreSQL
sudo systemctl enable --now postgresql

# Create O3K database and user
sudo -u postgres psql <<EOF
CREATE DATABASE o3k;
CREATE USER o3k WITH ENCRYPTED PASSWORD 'your-secure-password';
GRANT ALL PRIVILEGES ON DATABASE o3k TO o3k;
ALTER DATABASE o3k OWNER TO o3k;
\q
EOF

# Verify connection
psql -h localhost -U o3k -d o3k -c "SELECT version();"
# Enter password when prompted
```

### Step 6: Clone O3K Repository

```bash
# Install Git
sudo apt install -y git

# Clone O3K
cd /opt
sudo git clone https://github.com/cobaltcore-dev/o3k.git
sudo chown -R $USER:$USER o3k
cd o3k
```

### Step 7: Configure O3K for Single-Node Real Mode

Create configuration file:

```bash
mkdir -p /opt/o3k/config
cat > /opt/o3k/config/o3k.yaml <<'EOF'
# O3K Single-Node Configuration
# Real mode with KVM hypervisor

database:
  url: "postgres://o3k:your-secure-password@localhost:5432/o3k?sslmode=disable"

keystone:
  host: "0.0.0.0"
  port: 35357
  jwt_secret: "CHANGE-THIS-IN-PRODUCTION-min-32-chars-required-for-security"
  token_ttl: 24h

nova:
  host: "0.0.0.0"
  port: 8774
  libvirt_mode: real  # IMPORTANT: Enable real KVM mode
  libvirt_uri: "qemu:///system"
  instance_storage_path: "/var/lib/o3k/instances"
  console_proxy_base_url: "http://192.168.1.100:6080/vnc_auto.html"  # Replace with your host IP

neutron:
  host: "0.0.0.0"
  port: 9696
  networking_mode: iptables  # Real networking with iptables
  external_bridge: "br-ext"  # Bridge created earlier
  vxlan_enabled: false  # Single node doesn't need VXLAN

cinder:
  host: "0.0.0.0"
  port: 8776
  storage_mode: local  # Local filesystem storage
  local_storage_path: "/var/lib/o3k/volumes"

glance:
  host: "0.0.0.0"
  port: 9292
  storage_mode: local
  local_storage_path: "/var/lib/o3k/images"

metadata:
  host: "0.0.0.0"
  port: 8775

compute:
  node_id: "compute-node-1"
  node_name: "o3k-demo"
  tunnel_ip: "192.168.1.100"  # Replace with your host IP
EOF

# Replace password and IPs
sed -i 's/your-secure-password/YourActualPassword/g' /opt/o3k/config/o3k.yaml
sed -i 's/192.168.1.100/YOUR_HOST_IP/g' /opt/o3k/config/o3k.yaml  # Replace YOUR_HOST_IP
```

### Step 8: Create Storage Directories

```bash
# Create directories for volumes and images
sudo mkdir -p /var/lib/o3k/{volumes,images,instances}
sudo chown -R $USER:$USER /var/lib/o3k
sudo chmod 755 /var/lib/o3k/{volumes,images,instances}

# Verify
ls -la /var/lib/o3k/
```

### Step 9: Build and Run O3K

```bash
cd /opt/o3k

# Build O3K binary
make build

# Run database migrations
make migrate

# Start O3K services
./bin/o3k --config config/o3k.yaml &

# Or use systemd (recommended)
sudo tee /etc/systemd/system/o3k.service <<'EOF'
[Unit]
Description=O3K OpenStack Services
After=network.target postgresql.service libvirtd.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/o3k
ExecStart=/opt/o3k/bin/o3k --config /opt/o3k/config/o3k.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Enable and start O3K service
sudo systemctl daemon-reload
sudo systemctl enable --now o3k.service

# Check status
sudo systemctl status o3k.service

# View logs
sudo journalctl -u o3k.service -f
```

### Step 10: Deploy Horizon Dashboard

Create Docker Compose file for Horizon:

```bash
mkdir -p /opt/horizon
cat > /opt/horizon/docker-compose.yml <<'EOF'
version: '3.8'

services:
  horizon:
    image: quay.io/openstack.kolla/horizon:2025.2-ubuntu-noble
    container_name: horizon
    restart: unless-stopped
    ports:
      - "80:80"
    environment:
      - OPENSTACK_HOST=192.168.1.100  # Replace with your host IP
      - OPENSTACK_KEYSTONE_URL=http://192.168.1.100:35357/v3
      - OPENSTACK_KEYSTONE_MULTIDOMAIN_SUPPORT=True
      - OPENSTACK_KEYSTONE_DEFAULT_DOMAIN=Default
      - CONSOLE_TYPE=novnc
      - NOVNC_PROXY_BASE_URL=http://192.168.1.100:6080/vnc_auto.html
    volumes:
      - ./local_settings.py:/etc/openstack-dashboard/local_settings.py:ro
    networks:
      - o3k-net

  novnc:
    image: quay.io/openstack.kolla/nova-novncproxy:2025.2-ubuntu-noble
    container_name: novnc-proxy
    restart: unless-stopped
    ports:
      - "6080:6080"
    environment:
      - NOVA_NOVNCPROXY_BASE_URL=http://192.168.1.100:6080/vnc_auto.html
    command: >
      /usr/bin/nova-novncproxy
      --web /usr/share/novnc
      --novncproxy_host=0.0.0.0
      --novncproxy_port=6080
    networks:
      - o3k-net

networks:
  o3k-net:
    driver: bridge
EOF

# Create Horizon configuration
cat > /opt/horizon/local_settings.py <<'EOF'
import os
from django.utils.translation import gettext_lazy as _

WEBROOT = '/'
OPENSTACK_HOST = os.environ.get('OPENSTACK_HOST', '192.168.1.100')
OPENSTACK_KEYSTONE_URL = f"http://{OPENSTACK_HOST}:35357/v3"

OPENSTACK_API_VERSIONS = {
    "identity": 3,
    "image": 2,
    "volume": 3,
    "compute": 2.1,
}

OPENSTACK_KEYSTONE_MULTIDOMAIN_SUPPORT = True
OPENSTACK_KEYSTONE_DEFAULT_DOMAIN = 'Default'
OPENSTACK_KEYSTONE_DEFAULT_ROLE = 'member'

SESSION_TIMEOUT = 14400  # 4 hours

CONSOLE_TYPE = 'novnc'
OPENSTACK_CONSOLE_NOVNC_PROXY_URL = f"http://{OPENSTACK_HOST}:6080/vnc_auto.html"

ALLOWED_HOSTS = ['*']
DEBUG = False

CACHES = {
    'default': {
        'BACKEND': 'django.core.cache.backends.locmem.LocMemCache',
    },
}

TIME_ZONE = "UTC"
LOGGING = {
    'version': 1,
    'disable_existing_loggers': False,
    'handlers': {
        'console': {
            'level': 'INFO',
            'class': 'logging.StreamHandler',
        },
    },
    'loggers': {
        'horizon': {
            'handlers': ['console'],
            'level': 'INFO',
            'propagate': False,
        },
        'openstack_dashboard': {
            'handlers': ['console'],
            'level': 'INFO',
            'propagate': False,
        },
    },
}
EOF

# Replace IP addresses
sed -i 's/192.168.1.100/YOUR_HOST_IP/g' /opt/horizon/docker-compose.yml
sed -i 's/192.168.1.100/YOUR_HOST_IP/g' /opt/horizon/local_settings.py

# Start Horizon
cd /opt/horizon
docker compose up -d

# Check status
docker compose ps
docker compose logs -f horizon
```

### Step 11: Configure Firewall (Optional)

```bash
# Install UFW
sudo apt install -y ufw

# Allow SSH
sudo ufw allow 22/tcp

# Allow OpenStack services
sudo ufw allow 35357/tcp  # Keystone
sudo ufw allow 8774/tcp   # Nova
sudo ufw allow 8775/tcp   # Metadata
sudo ufw allow 8776/tcp   # Cinder
sudo ufw allow 9292/tcp   # Glance
sudo ufw allow 9696/tcp   # Neutron
sudo ufw allow 6080/tcp   # noVNC
sudo ufw allow 80/tcp     # Horizon

# Enable firewall
sudo ufw enable

# Check status
sudo ufw status
```

### Step 12: Verify Installation

```bash
# Check O3K services
sudo systemctl status o3k.service

# Check Horizon
docker ps | grep horizon

# Test OpenStack CLI
export OS_AUTH_URL=http://192.168.1.100:35357/v3
export OS_PROJECT_NAME=default
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_USER_DOMAIN_NAME=Default
export OS_PROJECT_DOMAIN_NAME=Default

# Install OpenStack CLI
sudo apt install -y python3-openstackclient

# Get token
openstack token issue

# List endpoints
openstack endpoint list

# Check services
openstack service list
```

---

## Initial Setup and Configuration

### 1. Upload CirrOS Test Image

```bash
# Download CirrOS (lightweight test image)
wget http://download.cirros-cloud.net/0.6.2/cirros-0.6.2-x86_64-disk.img

# Upload to Glance
openstack image create \
  --disk-format qcow2 \
  --container-format bare \
  --public \
  --file cirros-0.6.2-x86_64-disk.img \
  cirros

# Verify
openstack image list
```

### 2. Create Flavors

```bash
# Create standard flavors
openstack flavor create --ram 512 --disk 1 --vcpus 1 m1.tiny
openstack flavor create --ram 2048 --disk 20 --vcpus 1 m1.small
openstack flavor create --ram 4096 --disk 40 --vcpus 2 m1.medium
openstack flavor create --ram 8192 --disk 80 --vcpus 4 m1.large

# List flavors
openstack flavor list
```

### 3. Create Networks

**Internal Network**:
```bash
# Create internal network
openstack network create internal-net

# Create subnet
openstack subnet create \
  --network internal-net \
  --subnet-range 10.0.0.0/24 \
  --gateway 10.0.0.1 \
  --dns-nameserver 8.8.8.8 \
  internal-subnet

# Verify
openstack network list
openstack subnet list
```

**External Network** (for floating IPs):
```bash
# Create external network
openstack network create --external --provider-network-type flat ext-net

# Create external subnet (use your host network range)
openstack subnet create \
  --network ext-net \
  --subnet-range 192.168.1.0/24 \
  --gateway 192.168.1.1 \
  --no-dhcp \
  --allocation-pool start=192.168.1.200,end=192.168.1.250 \
  ext-subnet

# Verify
openstack network list --external
```

### 4. Create Router

```bash
# Create router
openstack router create demo-router

# Set external gateway
openstack router set --external-gateway ext-net demo-router

# Add internal subnet to router
openstack router add subnet demo-router internal-subnet

# Verify
openstack router show demo-router
openstack port list --router demo-router
```

### 5. Configure Security Groups

```bash
# Allow ICMP (ping)
openstack security group rule create \
  --protocol icmp \
  --ingress \
  default

# Allow SSH
openstack security group rule create \
  --protocol tcp \
  --dst-port 22 \
  --ingress \
  default

# Allow HTTP
openstack security group rule create \
  --protocol tcp \
  --dst-port 80 \
  --ingress \
  default

# Verify
openstack security group rule list default
```

### 6. Create SSH Keypair

```bash
# Generate SSH key
ssh-keygen -t rsa -b 2048 -f ~/.ssh/o3k-demo -N ""

# Import to OpenStack
openstack keypair create --public-key ~/.ssh/o3k-demo.pub demo-key

# Verify
openstack keypair list
```

---

## Demonstration Scenarios

### Scenario 1: Launch VM via Horizon (Web UI)

1. **Access Horizon**:
   - URL: `http://192.168.1.100` (replace with your host IP)
   - Domain: `Default`
   - Username: `admin`
   - Password: `secret`

2. **Launch Instance**:
   - Navigate: Project → Compute → Instances
   - Click "Launch Instance"
   - Details:
     - Instance Name: `demo-vm-1`
     - Description: Test instance
   - Source:
     - Select Boot Source: Image
     - Choose: `cirros`
   - Flavor:
     - Choose: `m1.small`
   - Networks:
     - Select: `internal-net`
   - Security Groups:
     - Select: `default`
   - Key Pair:
     - Select: `demo-key`
   - Click "Launch Instance"

3. **Monitor Creation**:
   - Watch status change: BUILD → ACTIVE
   - Note internal IP address (e.g., 10.0.0.10)

4. **Allocate Floating IP**:
   - Click dropdown on instance → "Associate Floating IP"
   - Click "+" to allocate new IP from `ext-net`
   - Select allocated IP → "Associate"
   - Note external IP (e.g., 192.168.1.200)

5. **Access Console**:
   - Click instance name → "Console" tab
   - Login via noVNC console:
     - Username: `cirros`
     - Password: `gocubsgo`
   - Verify network: `ip addr show`

6. **SSH to VM**:
   ```bash
   ssh -i ~/.ssh/o3k-demo cirros@192.168.1.200
   ```

### Scenario 2: Create Volume and Attach

1. **Via Horizon**:
   - Navigate: Project → Volumes → Volumes
   - Click "Create Volume"
   - Volume Name: `demo-volume`
   - Size: 10 GB
   - Click "Create Volume"

2. **Attach to Instance**:
   - Click dropdown → "Manage Attachments"
   - Select instance: `demo-vm-1`
   - Click "Attach Volume"

3. **Verify in VM**:
   ```bash
   # SSH to VM
   ssh -i ~/.ssh/o3k-demo cirros@192.168.1.200

   # Check block devices
   lsblk

   # Format and mount
   sudo mkfs.ext4 /dev/vdb
   sudo mkdir /mnt/volume
   sudo mount /dev/vdb /mnt/volume
   df -h
   ```

### Scenario 3: Network Topology Visualization

1. **Via Horizon**:
   - Navigate: Project → Network → Network Topology
   - View graphical representation:
     - External network (`ext-net`)
     - Router (`demo-router`)
     - Internal network (`internal-net`)
     - Instances attached to network

2. **Interact with Topology**:
   - Click on instance → "Console"
   - Click on network → View details
   - Drag elements to reorganize

### Scenario 4: Create Snapshot

1. **Via Horizon**:
   - Navigate: Project → Compute → Instances
   - Click dropdown on `demo-vm-1` → "Create Snapshot"
   - Snapshot Name: `demo-vm-1-backup`
   - Click "Create Snapshot"

2. **Launch from Snapshot**:
   - Navigate: Project → Compute → Images
   - Find snapshot: `demo-vm-1-backup`
   - Click "Launch" → Follow instance creation wizard

### Scenario 5: Resize Instance

1. **Via Horizon**:
   - Navigate: Project → Compute → Instances
   - Click dropdown on `demo-vm-1` → "Resize Instance"
   - Select new flavor: `m1.medium`
   - Click "Resize"
   - Wait for status: VERIFY_RESIZE
   - Click "Confirm Resize"

### Scenario 6: CLI Operations

```bash
# Create instance via CLI
openstack server create \
  --flavor m1.small \
  --image cirros \
  --network internal-net \
  --key-name demo-key \
  demo-vm-2

# List instances
openstack server list

# Show instance details
openstack server show demo-vm-2

# Create floating IP
FIP=$(openstack floating ip create ext-net -f value -c floating_ip_address)
echo "Allocated IP: $FIP"

# Associate floating IP
openstack server add floating ip demo-vm-2 $FIP

# SSH to instance
ssh -i ~/.ssh/o3k-demo cirros@$FIP

# Stop instance
openstack server stop demo-vm-2

# Start instance
openstack server start demo-vm-2

# Delete instance
openstack server delete demo-vm-2
```

---

## Troubleshooting

### Issue 1: O3K Service Won't Start

**Check logs**:
```bash
sudo journalctl -u o3k.service -n 100 --no-pager
```

**Common causes**:
- Database connection failure → verify PostgreSQL is running
- Libvirt not accessible → check user permissions (`usermod -aG libvirt`)
- Port conflicts → check if ports are already in use (`netstat -tulpn`)

### Issue 2: Cannot Create VMs

**Verify KVM is working**:
```bash
sudo kvm-ok
virsh list --all
sudo systemctl status libvirtd
```

**Check Nova configuration**:
```bash
# Verify libvirt_mode is "real"
grep libvirt_mode /opt/o3k/config/o3k.yaml

# Check libvirt connection
virsh -c qemu:///system list --all
```

**Check storage directories**:
```bash
ls -la /var/lib/o3k/instances/
sudo chmod 755 /var/lib/o3k/instances
```

### Issue 3: Horizon Not Accessible

**Check Horizon container**:
```bash
cd /opt/horizon
docker compose ps
docker compose logs horizon

# Restart if needed
docker compose restart horizon
```

**Verify network connectivity**:
```bash
curl http://192.168.1.100
# Should return Horizon HTML
```

### Issue 4: Cannot Access VM Console

**Check noVNC proxy**:
```bash
docker compose ps novnc
docker compose logs novnc

# Verify port 6080 is open
sudo netstat -tulpn | grep 6080
```

**Check Nova console URL configuration**:
```bash
grep console_proxy_base_url /opt/o3k/config/o3k.yaml
# Should match your host IP
```

### Issue 5: Floating IPs Not Working

**Verify bridge configuration**:
```bash
brctl show br-ext
ip addr show br-ext

# Check routing
ip route
```

**Check Neutron external network**:
```bash
openstack network show ext-net --external
# Verify provider:network_type is "flat"
```

**Verify iptables rules**:
```bash
sudo iptables -t nat -L -n -v
# Should see SNAT/DNAT rules for floating IPs
```

---

## Performance Tuning

### 1. KVM CPU Pinning (Optional)

For better performance, pin VCPUs to physical cores:

```xml
<!-- Edit VM XML: virsh edit <instance-uuid> -->
<vcpu placement='static' cpuset='0-3'>4</vcpu>
<cputune>
  <vcpupin vcpu='0' cpuset='0'/>
  <vcpupin vcpu='1' cpuset='1'/>
  <vcpupin vcpu='2' cpuset='2'/>
  <vcpupin vcpu='3' cpuset='3'/>
</cputune>
```

### 2. Enable Huge Pages

```bash
# Allocate 2GB huge pages
echo 1024 | sudo tee /sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages

# Make persistent
echo "vm.nr_hugepages=1024" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# Configure libvirt
sudo tee -a /etc/libvirt/qemu.conf <<EOF
hugetlbfs_mount = "/dev/hugepages"
EOF

sudo systemctl restart libvirtd
```

### 3. PostgreSQL Tuning

```bash
# Edit PostgreSQL config
sudo vim /etc/postgresql/18/main/postgresql.conf

# Increase shared_buffers (25% of RAM for demo)
shared_buffers = 4GB
effective_cache_size = 12GB
maintenance_work_mem = 1GB
work_mem = 128MB

# Restart PostgreSQL
sudo systemctl restart postgresql
```

---

## Maintenance

### Daily Operations

**Check Service Health**:
```bash
# O3K services
sudo systemctl status o3k.service

# Horizon
cd /opt/horizon && docker compose ps

# Database
sudo systemctl status postgresql

# Check logs
sudo journalctl -u o3k.service --since "1 hour ago"
```

**Monitor Resources**:
```bash
# CPU/RAM usage
htop

# Disk usage
df -h /var/lib/o3k

# Network
iftop -i br-ext
```

### Backup

**Database Backup**:
```bash
# Backup database
pg_dump -U o3k -h localhost o3k > o3k-backup-$(date +%Y%m%d).sql

# Restore
psql -U o3k -h localhost o3k < o3k-backup-20260317.sql
```

**Configuration Backup**:
```bash
# Backup configs
tar -czf o3k-config-$(date +%Y%m%d).tar.gz \
  /opt/o3k/config \
  /opt/horizon/docker-compose.yml \
  /opt/horizon/local_settings.py
```

### Updates

**Update O3K**:
```bash
cd /opt/o3k
git pull origin main
make build
sudo systemctl restart o3k.service
```

**Update Horizon**:
```bash
cd /opt/horizon
docker compose pull
docker compose up -d
```

---

## Security Considerations

**⚠️ WARNING**: This single-node setup is for **demonstration only**. Do NOT use in production.

**Security Limitations**:
- ❌ No TLS/HTTPS (all traffic unencrypted)
- ❌ Default passwords (admin:secret)
- ❌ Single point of failure
- ❌ No firewall rules (if UFW disabled)
- ❌ No intrusion detection

**For Production**: See [SCALING.md](SCALING.md) for secure multi-node deployment.

---

## Next Steps

**Scaling to Production**:
- Read [SCALING.md](SCALING.md) for multi-node architecture
- Implement HA (high availability) setup
- Add TLS certificates
- Configure external storage (Ceph)
- Set up monitoring (Prometheus + Grafana)

**Advanced Features**:
- Multi-node networking with VXLAN
- Ceph RBD for shared storage
- Load balancing for O3K services
- Backup and disaster recovery

---

## Reference

**Configuration Files**:
- O3K config: `/opt/o3k/config/o3k.yaml`
- Horizon config: `/opt/horizon/local_settings.py`
- Systemd service: `/etc/systemd/system/o3k.service`

**Default Credentials**:
- OpenStack admin: `admin` / `secret`
- PostgreSQL: `o3k` / (password from config)
- CirrOS VM: `cirros` / `gocubsgo`

**Important Paths**:
- O3K binary: `/opt/o3k/bin/o3k`
- VM instances: `/var/lib/o3k/instances/`
- Volumes: `/var/lib/o3k/volumes/`
- Images: `/var/lib/o3k/images/`

**Log Locations**:
- O3K logs: `sudo journalctl -u o3k.service`
- Horizon logs: `/opt/horizon && docker compose logs`
- PostgreSQL logs: `/var/log/postgresql/`
- libvirt logs: `/var/log/libvirt/`

---

**Document Version**: 1.0
**Last Updated**: March 17, 2026
**Tested On**: Ubuntu 24.04 LTS with KVM
