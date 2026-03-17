# Scaling O3K: Production Multi-Node Deployment

**📚 Complete Documentation**: See **[INDEX.md](INDEX.md)** for full documentation index with learning paths.


**Purpose**: Scale O3K from single-node demo to production-ready multi-node cluster
**Target Audience**: DevOps engineers, cloud operators, production deployments
**Prerequisites**: Understanding of [SINGLE_NODE_DEPLOYMENT.md](SINGLE_NODE_DEPLOYMENT.md)

---

## Overview

This guide covers scaling O3K from a single demonstration node to a production-ready multi-node cluster with high availability, shared storage, and load balancing.

**Key Differences from Single-Node**:
- ✅ High Availability (HA) for control plane
- ✅ Multiple compute nodes for VM distribution
- ✅ Shared storage backend (Ceph RBD)
- ✅ VXLAN overlay networking for multi-node
- ✅ Load balancing for API services
- ✅ TLS encryption for all endpoints
- ✅ Production-grade monitoring and logging
- ✅ Backup and disaster recovery

---

## Architecture Overview

### Small Production Cluster (3-5 Nodes)

**Recommended for**: 100-500 VMs, small-medium enterprises

```
┌─────────────────────────────────────────────────────────────────┐
│                     Load Balancer (HAProxy)                      │
│  API Endpoints: 35357, 8774, 8776, 9292, 9696 (HTTPS)          │
└────────┬──────────────────────────────────┬─────────────────────┘
         │                                   │
┌────────▼─────────────┐          ┌─────────▼────────────┐
│   Controller Node 1   │          │  Controller Node 2   │
│  (Active)             │◄────────►│  (Standby)           │
│                       │  Keepalived│                    │
│ - O3K Services        │          │ - O3K Services       │
│ - PostgreSQL Primary  │          │ - PostgreSQL Replica │
│ - Horizon Dashboard   │          │ - Horizon Dashboard  │
└───────┬───────────────┘          └──────┬───────────────┘
        │                                  │
        │         ┌────────────────────────┤
        │         │                        │
┌───────▼─────────▼───┐  ┌────────────┐  ┌▼────────────┐
│  Compute Node 1     │  │ Compute 2  │  │ Compute 3   │
│  - KVM Hypervisor   │  │ - KVM      │  │ - KVM       │
│  - libvirt          │  │ - libvirt  │  │ - libvirt   │
│  - VXLAN tunneling  │  │ - VXLAN    │  │ - VXLAN     │
└──────┬──────────────┘  └─────┬──────┘  └─┬───────────┘
       │                        │            │
       └────────────┬───────────┴────────────┘
                    │
         ┌──────────▼──────────┐
         │   Ceph Cluster      │
         │  (Shared Storage)   │
         │  - 3+ OSDs          │
         │  - RBD pools        │
         └─────────────────────┘
```

**Node Requirements**:

| Node Type | Count | CPU | RAM | Disk | Role |
|-----------|-------|-----|-----|------|------|
| Controller | 2 | 8 cores | 32 GB | 200 GB SSD | API, DB, UI |
| Compute | 3+ | 16 cores | 64 GB | 500 GB SSD | VMs (KVM) |
| Ceph OSD | 3+ | 4 cores | 16 GB | 2-4 TB HDD | Storage |

**Total Minimum**: 8 nodes (2 controllers + 3 compute + 3 Ceph)

### Large Production Cluster (10+ Nodes)

**Recommended for**: 1000+ VMs, large enterprises, cloud providers

```
┌──────────────────────────────────────────────────────────────────┐
│              External Load Balancer (Hardware/Cloud)              │
│         HTTPS: 443 → backends (TLS termination)                  │
└───────────┬──────────────────────────────────────────────────────┘
            │
┌───────────▼─────────────────────────────────────────────────────┐
│                  Internal Load Balancer Cluster                  │
│  HAProxy (2 nodes) + Keepalived (VIP: 192.168.10.10)           │
│  Routes to: Keystone, Nova, Neutron, Cinder, Glance            │
└───────┬───────────────────┬────────────────────┬─────────────────┘
        │                   │                    │
┌───────▼──────┐  ┌────────▼───────┐  ┌────────▼───────┐
│ Controller 1 │  │ Controller 2   │  │ Controller 3   │
│ (Active)     │  │ (Active)       │  │ (Standby)      │
│              │  │                │  │                │
│ - O3K API    │  │ - O3K API      │  │ - O3K API      │
│ - Horizon    │  │ - Horizon      │  │ - Horizon      │
└──────┬───────┘  └────────┬───────┘  └────────┬───────┘
       │                   │                    │
       └───────────────────┴────────────────────┘
                           │
           ┌───────────────▼─────────────────┐
           │     PostgreSQL Cluster (HA)     │
           │  - Patroni (3 nodes)            │
           │  - etcd (3 nodes)               │
           │  - Automatic failover           │
           └───────────────┬─────────────────┘
                           │
       ┌───────────────────┴─────────────────────────────┐
       │                                                  │
┌──────▼────────┐  ┌──────────────┐  ... ┌──────────────┐
│ Compute 1     │  │ Compute 2    │      │ Compute N    │
│ - KVM/libvirt │  │ - KVM/libvirt│      │ - KVM/libvirt│
│ - VXLAN       │  │ - VXLAN      │      │ - VXLAN      │
│ - Node Reg    │  │ - Node Reg   │      │ - Node Reg   │
└───────┬───────┘  └──────┬───────┘      └──────┬───────┘
        │                  │                     │
        └──────────────────┴─────────────────────┘
                           │
              ┌────────────▼──────────────┐
              │   Ceph Cluster (HA)       │
              │  - 5+ OSDs (redundancy)   │
              │  - RBD pool (replica=3)   │
              │  - S3 gateway (RGW)       │
              │  - CephFS (shared files)  │
              └───────────────────────────┘
```

**Node Requirements**:

| Node Type | Count | CPU | RAM | Disk | Role |
|-----------|-------|-----|-----|------|------|
| Load Balancer | 2 | 4 cores | 8 GB | 100 GB | HA Proxy |
| Controller | 3 | 16 cores | 64 GB | 500 GB SSD | API, UI |
| Database | 3 | 8 cores | 32 GB | 500 GB SSD | PostgreSQL + etcd |
| Compute | 10+ | 32 cores | 128 GB | 1 TB NVMe | VMs |
| Ceph OSD | 5+ | 8 cores | 32 GB | 4-8 TB HDD | Storage |
| Ceph MON | 3 | 4 cores | 16 GB | 200 GB SSD | Monitors |

**Total Minimum**: 26 nodes (2 LB + 3 controller + 3 DB + 10 compute + 5 OSD + 3 MON)

---

## Scaling Strategy

### Phase 1: Single Node → 3-Node Cluster

**Goal**: Achieve basic high availability

**Steps**:
1. Add 2nd controller node (PostgreSQL replica)
2. Add 1st compute node (move VMs from controller)
3. Deploy Ceph cluster (3 OSDs minimum)
4. Configure VXLAN networking
5. Set up load balancer (HAProxy + Keepalived)

**Timeline**: 2-3 days

### Phase 2: 3-Node → 5-Node Cluster

**Goal**: Improve compute capacity and storage redundancy

**Steps**:
1. Add 2 more compute nodes (total 3 compute)
2. Add 2 more Ceph OSDs (total 5 OSDs)
3. Configure Ceph replica factor to 3
4. Enable VM live migration

**Timeline**: 1-2 days

### Phase 3: 5-Node → Production Cluster (10+ Nodes)

**Goal**: Production-scale deployment

**Steps**:
1. Add 3rd controller node
2. Scale compute nodes based on workload
3. Separate database to dedicated nodes (Patroni + etcd)
4. Add Ceph MON nodes for monitoring
5. Implement monitoring (Prometheus + Grafana)
6. Add backup infrastructure

**Timeline**: 1-2 weeks

---

## Deployment Guide: 3-Node HA Cluster

### Node Configuration

**Controller Nodes (2)**:
- controller1: 192.168.10.11
- controller2: 192.168.10.12
- VIP (Keepalived): 192.168.10.10

**Compute Nodes (1-3)**:
- compute1: 192.168.10.21
- compute2: 192.168.10.22
- compute3: 192.168.10.23

**Ceph Nodes (3)**:
- ceph1: 192.168.10.31 (OSD + MON)
- ceph2: 192.168.10.32 (OSD + MON)
- ceph3: 192.168.10.33 (OSD + MON)

**Network Architecture**:
- Management: 192.168.10.0/24 (all nodes)
- Tunnel: 192.168.20.0/24 (VXLAN overlay)
- Storage: 192.168.30.0/24 (Ceph backend)
- External: 192.168.1.0/24 (floating IPs)

### Step 1: Prepare All Nodes

**On all nodes**:

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install common packages
sudo apt install -y \
    vim \
    curl \
    wget \
    net-tools \
    htop \
    iotop \
    iftop \
    tcpdump \
    chrony \
    python3-pip

# Synchronize time (CRITICAL for distributed systems)
sudo systemctl enable --now chrony
sudo chronyc sources

# Configure /etc/hosts with all nodes
sudo tee -a /etc/hosts <<EOF
# O3K Cluster Nodes
192.168.10.10   o3k-api o3k-api.example.com
192.168.10.11   controller1
192.168.10.12   controller2
192.168.10.21   compute1
192.168.10.22   compute2
192.168.10.23   compute3
192.168.10.31   ceph1
192.168.10.32   ceph2
192.168.10.33   ceph3
EOF

# Enable IP forwarding
sudo tee -a /etc/sysctl.conf <<EOF
net.ipv4.ip_forward=1
net.ipv4.conf.all.forwarding=1
net.ipv6.conf.all.forwarding=1
net.bridge.bridge-nf-call-iptables=1
net.bridge.bridge-nf-call-ip6tables=1
EOF

sudo sysctl -p
```

### Step 2: Deploy PostgreSQL HA (Patroni + etcd)

**On controller1**:

```bash
# Install PostgreSQL and Patroni
sudo apt install -y postgresql-18 postgresql-contrib-18 python3-psycopg2
sudo pip3 install patroni[etcd] python-etcd

# Stop default PostgreSQL
sudo systemctl stop postgresql
sudo systemctl disable postgresql

# Install etcd
sudo apt install -y etcd

# Configure etcd on controller1
sudo tee /etc/default/etcd <<EOF
ETCD_NAME="controller1"
ETCD_DATA_DIR="/var/lib/etcd/controller1"
ETCD_LISTEN_CLIENT_URLS="http://192.168.10.11:2379,http://127.0.0.1:2379"
ETCD_ADVERTISE_CLIENT_URLS="http://192.168.10.11:2379"
ETCD_LISTEN_PEER_URLS="http://192.168.10.11:2380"
ETCD_INITIAL_ADVERTISE_PEER_URLS="http://192.168.10.11:2380"
ETCD_INITIAL_CLUSTER="controller1=http://192.168.10.11:2380,controller2=http://192.168.10.12:2380"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_INITIAL_CLUSTER_TOKEN="etcd-cluster-o3k"
EOF

sudo systemctl enable --now etcd
```

**On controller2** (similar etcd config with different IP):

```bash
# Install and configure etcd
sudo apt install -y etcd

sudo tee /etc/default/etcd <<EOF
ETCD_NAME="controller2"
ETCD_DATA_DIR="/var/lib/etcd/controller2"
ETCD_LISTEN_CLIENT_URLS="http://192.168.10.12:2379,http://127.0.0.1:2379"
ETCD_ADVERTISE_CLIENT_URLS="http://192.168.10.12:2379"
ETCD_LISTEN_PEER_URLS="http://192.168.10.12:2380"
ETCD_INITIAL_ADVERTISE_PEER_URLS="http://192.168.10.12:2380"
ETCD_INITIAL_CLUSTER="controller1=http://192.168.10.11:2380,controller2=http://192.168.10.12:2380"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_INITIAL_CLUSTER_TOKEN="etcd-cluster-o3k"
EOF

sudo systemctl enable --now etcd
```

**Verify etcd cluster**:

```bash
etcdctl member list
etcdctl cluster-health
```

**Configure Patroni on controller1**:

```bash
sudo tee /etc/patroni.yml <<EOF
scope: o3k-postgres
namespace: /db/
name: controller1

restapi:
  listen: 192.168.10.11:8008
  connect_address: 192.168.10.11:8008

etcd:
  hosts: 192.168.10.11:2379,192.168.10.12:2379

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
    postgresql:
      use_pg_rewind: true
      parameters:
        max_connections: 200
        shared_buffers: 4GB
        effective_cache_size: 12GB
        work_mem: 128MB

  initdb:
  - encoding: UTF8
  - data-checksums

  pg_hba:
  - host replication replicator 192.168.10.0/24 md5
  - host all all 192.168.10.0/24 md5
  - host all all 0.0.0.0/0 md5

  users:
    admin:
      password: admin-password
      options:
        - createrole
        - createdb
    replicator:
      password: replicator-password
      options:
        - replication

postgresql:
  listen: 192.168.10.11:5432
  connect_address: 192.168.10.11:5432
  data_dir: /var/lib/postgresql/18/main
  bin_dir: /usr/lib/postgresql/18/bin
  pgpass: /tmp/pgpass
  authentication:
    replication:
      username: replicator
      password: replicator-password
    superuser:
      username: postgres
      password: postgres-password
  parameters:
    unix_socket_directories: '/var/run/postgresql'

tags:
    nofailover: false
    noloadbalance: false
    clonefrom: false
    nosync: false
EOF

# Create systemd service
sudo tee /etc/systemd/system/patroni.service <<EOF
[Unit]
Description=Patroni PostgreSQL Cluster Manager
After=syslog.target network.target etcd.service

[Service]
Type=simple
User=postgres
Group=postgres
ExecStart=/usr/local/bin/patroni /etc/patroni.yml
KillMode=process
TimeoutSec=30
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now patroni
```

**Configure Patroni on controller2** (similar config with different IP).

**Verify Patroni cluster**:

```bash
patronictl -c /etc/patroni.yml list
```

Expected output:
```
+ Cluster: o3k-postgres ----+----+-----------+
| Member      | Host          | Role   | State   |
+-------------+---------------+--------+---------+
| controller1 | 192.168.10.11 | Leader | running |
| controller2 | 192.168.10.12 | Replica| running |
+-------------+---------------+--------+---------+
```

### Step 3: Deploy Ceph Storage Cluster

**On all Ceph nodes (ceph1, ceph2, ceph3)**:

```bash
# Install cephadm
curl --silent --remote-name --location https://github.com/ceph/ceph/raw/quincy/src/cephadm/cephadm
chmod +x cephadm
sudo mv cephadm /usr/local/bin/

# Add Ceph repository
sudo cephadm add-repo --release quincy
sudo cephadm install ceph-common
```

**On ceph1 (bootstrap node)**:

```bash
# Bootstrap Ceph cluster
sudo cephadm bootstrap \
  --mon-ip 192.168.10.31 \
  --cluster-network 192.168.30.0/24 \
  --public-network 192.168.10.0/24 \
  --initial-dashboard-user admin \
  --initial-dashboard-password admin-password

# Copy SSH keys to other nodes
ssh-copy-id ceph2
ssh-copy-id ceph3

# Add nodes to cluster
sudo ceph orch host add ceph2 192.168.10.32
sudo ceph orch host add ceph3 192.168.10.33

# Label nodes for MON
sudo ceph orch host label add ceph1 mon
sudo ceph orch host label add ceph2 mon
sudo ceph orch host label add ceph3 mon

# Deploy MONs
sudo ceph orch apply mon --placement="ceph1,ceph2,ceph3"

# Add OSDs (assuming /dev/sdb is dedicated disk on each node)
sudo ceph orch daemon add osd ceph1:/dev/sdb
sudo ceph orch daemon add osd ceph2:/dev/sdb
sudo ceph orch daemon add osd ceph3:/dev/sdb

# Wait for OSDs to be up
sudo ceph -s
```

**Create RBD pools for O3K**:

```bash
# Create pools
sudo ceph osd pool create volumes 128 128
sudo ceph osd pool create images 64 64
sudo ceph osd pool create vms 128 128

# Enable RBD application
sudo ceph osd pool application enable volumes rbd
sudo ceph osd pool application enable images rbd
sudo ceph osd pool application enable vms rbd

# Set replica size
sudo ceph osd pool set volumes size 3
sudo ceph osd pool set images size 3
sudo ceph osd pool set vms size 3

# Create Ceph user for O3K
sudo ceph auth get-or-create client.o3k \
  mon 'allow r' \
  osd 'allow class-read object_prefix rbd_children, allow rwx pool=volumes, allow rwx pool=images, allow rwx pool=vms' \
  -o /etc/ceph/ceph.client.o3k.keyring

# Copy keyring to O3K nodes
scp /etc/ceph/ceph.conf controller1:/tmp/
scp /etc/ceph/ceph.client.o3k.keyring controller1:/tmp/
# Repeat for controller2, compute1, compute2, compute3
```

**Verify Ceph cluster**:

```bash
sudo ceph -s
sudo ceph osd tree
sudo ceph df
```

### Step 4: Configure Compute Nodes

**On all compute nodes**:

```bash
# Install KVM and libvirt
sudo apt install -y \
    qemu-kvm \
    libvirt-daemon-system \
    libvirt-clients \
    bridge-utils \
    virt-manager

# Install Ceph client libraries
sudo apt install -y ceph-common librbd-dev

# Copy Ceph configuration
sudo cp /tmp/ceph.conf /etc/ceph/
sudo cp /tmp/ceph.client.o3k.keyring /etc/ceph/
sudo chmod 644 /etc/ceph/ceph.conf
sudo chmod 600 /etc/ceph/ceph.client.o3k.keyring

# Enable libvirt
sudo systemctl enable --now libvirtd

# Configure Ceph RBD for libvirt
sudo tee /etc/libvirt/qemu.conf <<EOF
user = "root"
group = "root"
cgroup_device_acl = [
    "/dev/null", "/dev/full", "/dev/zero",
    "/dev/random", "/dev/urandom",
    "/dev/ptmx", "/dev/kvm", "/dev/kqemu",
    "/dev/rtc","/dev/hpet", "/dev/vfio/vfio"
]
EOF

sudo systemctl restart libvirtd

# Verify Ceph connectivity
rbd --id o3k ls volumes
```

**Configure VXLAN networking**:

```bash
# Create VXLAN interface for tunneling
sudo ip link add vxlan-tunnel type vxlan \
  id 1000 \
  dstport 4789 \
  local 192.168.20.21 \  # Change per node: .21, .22, .23
  nolearning

sudo ip addr add 10.255.0.21/24 dev vxlan-tunnel  # Change per node
sudo ip link set vxlan-tunnel up

# Make persistent (add to /etc/network/interfaces or netplan)
```

### Step 5: Deploy O3K on Controller Nodes

**On controller1 and controller2**:

```bash
# Clone O3K
cd /opt
sudo git clone https://github.com/cobaltcore-dev/o3k.git
sudo chown -R $USER:$USER o3k
cd o3k

# Build O3K
make build

# Create configuration
mkdir -p /opt/o3k/config
cat > /opt/o3k/config/o3k.yaml <<EOF
database:
  url: "postgres://o3k:o3k-password@192.168.10.10:5432/o3k?sslmode=disable"  # VIP from HAProxy

keystone:
  host: "0.0.0.0"
  port: 35357
  jwt_secret: "PRODUCTION-SECRET-MIN-32-CHARS-CHANGE-THIS-IMMEDIATELY"
  token_ttl: 24h

nova:
  host: "0.0.0.0"
  port: 8774
  libvirt_mode: real
  libvirt_uri: "qemu+tcp://compute1/system"  # Will be load-balanced
  instance_storage_path: "rbd:vms/instances"  # Ceph RBD
  console_proxy_base_url: "https://o3k-api.example.com:6080/vnc_auto.html"

neutron:
  host: "0.0.0.0"
  port: 9696
  networking_mode: iptables
  vxlan_enabled: true  # IMPORTANT: Enable for multi-node
  vxlan_local_ip: "192.168.20.11"  # Tunnel network IP (change per node)

cinder:
  host: "0.0.0.0"
  port: 8776
  storage_mode: rbd  # IMPORTANT: Use Ceph RBD
  rbd_pool: volumes
  rbd_user: o3k
  rbd_ceph_conf: /etc/ceph/ceph.conf

glance:
  host: "0.0.0.0"
  port: 9292
  storage_mode: rbd  # IMPORTANT: Use Ceph RBD
  rbd_pool: images
  rbd_user: o3k
  rbd_ceph_conf: /etc/ceph/ceph.conf

metadata:
  host: "0.0.0.0"
  port: 8775

compute:
  registry_enabled: true  # IMPORTANT: Enable for multi-node
  node_id: "controller1"  # Change per node
  node_name: "controller1"
  tunnel_ip: "192.168.20.11"  # Change per node
EOF

# Run migrations (only on controller1)
./bin/o3k migrate --config config/o3k.yaml

# Create systemd service
sudo tee /etc/systemd/system/o3k.service <<EOF
[Unit]
Description=O3K OpenStack Services
After=network.target patroni.service

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

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now o3k.service
```

### Step 6: Deploy HAProxy Load Balancer

**On controller1 and controller2**:

```bash
# Install HAProxy and Keepalived
sudo apt install -y haproxy keepalived

# Configure HAProxy
sudo tee /etc/haproxy/haproxy.cfg <<EOF
global
    log /dev/log local0
    log /dev/log local1 notice
    chroot /var/lib/haproxy
    stats socket /run/haproxy/admin.sock mode 660 level admin
    stats timeout 30s
    user haproxy
    group haproxy
    daemon

defaults
    log     global
    mode    http
    option  httplog
    option  dontlognull
    timeout connect 5000
    timeout client  50000
    timeout server  50000

# Statistics
listen stats
    bind *:9000
    mode http
    stats enable
    stats uri /stats
    stats refresh 30s
    stats realm HAProxy\ Statistics
    stats auth admin:password

# Keystone
listen keystone
    bind 192.168.10.10:35357
    mode http
    balance roundrobin
    option httpchk GET /v3
    server controller1 192.168.10.11:35357 check inter 2000 rise 2 fall 5
    server controller2 192.168.10.12:35357 check inter 2000 rise 2 fall 5

# Nova
listen nova
    bind 192.168.10.10:8774
    mode http
    balance roundrobin
    option httpchk GET /
    server controller1 192.168.10.11:8774 check inter 2000 rise 2 fall 5
    server controller2 192.168.10.12:8774 check inter 2000 rise 2 fall 5

# Neutron
listen neutron
    bind 192.168.10.10:9696
    mode http
    balance roundrobin
    option httpchk GET /
    server controller1 192.168.10.11:9696 check inter 2000 rise 2 fall 5
    server controller2 192.168.10.12:9696 check inter 2000 rise 2 fall 5

# Cinder
listen cinder
    bind 192.168.10.10:8776
    mode http
    balance roundrobin
    option httpchk GET /v3
    server controller1 192.168.10.11:8776 check inter 2000 rise 2 fall 5
    server controller2 192.168.10.12:8776 check inter 2000 rise 2 fall 5

# Glance
listen glance
    bind 192.168.10.10:9292
    mode http
    balance roundrobin
    option httpchk GET /
    server controller1 192.168.10.11:9292 check inter 2000 rise 2 fall 5
    server controller2 192.168.10.12:9292 check inter 2000 rise 2 fall 5

# Horizon
listen horizon
    bind 192.168.10.10:80
    mode http
    balance roundrobin
    option httpchk GET /
    server controller1 192.168.10.11:80 check inter 2000 rise 2 fall 5
    server controller2 192.168.10.12:80 check inter 2000 rise 2 fall 5
EOF

# Configure Keepalived (controller1 - MASTER)
sudo tee /etc/keepalived/keepalived.conf <<EOF
vrrp_script chk_haproxy {
    script "killall -0 haproxy"
    interval 2
    weight 2
}

vrrp_instance VI_1 {
    state MASTER  # BACKUP on controller2
    interface eth0  # Change to your interface
    virtual_router_id 51
    priority 101  # 100 on controller2
    advert_int 1

    authentication {
        auth_type PASS
        auth_pass keepalived-secret
    }

    virtual_ipaddress {
        192.168.10.10/24 dev eth0 label eth0:vip
    }

    track_script {
        chk_haproxy
    }
}
EOF

# Enable services
sudo systemctl enable --now haproxy
sudo systemctl enable --now keepalived

# Verify VIP is assigned
ip addr show eth0
# Should see 192.168.10.10 on MASTER
```

### Step 7: TLS Certificate Configuration

**Generate self-signed certificate** (for testing):

```bash
# On controller1
sudo mkdir -p /etc/ssl/o3k
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/o3k/o3k.key \
  -out /etc/ssl/o3k/o3k.crt \
  -subj "/CN=o3k-api.example.com"

# Combine for HAProxy
sudo cat /etc/ssl/o3k/o3k.crt /etc/ssl/o3k/o3k.key | \
  sudo tee /etc/ssl/o3k/o3k.pem

# Copy to controller2
scp /etc/ssl/o3k/* controller2:/tmp/
ssh controller2 "sudo mkdir -p /etc/ssl/o3k && sudo mv /tmp/o3k.* /etc/ssl/o3k/"

# Update HAProxy config for HTTPS
sudo tee -a /etc/haproxy/haproxy.cfg <<EOF
# HTTPS Frontend
frontend https_front
    bind *:443 ssl crt /etc/ssl/o3k/o3k.pem
    mode http
    default_backend keystone

backend keystone
    mode http
    balance roundrobin
    server controller1 192.168.10.11:35357 check
    server controller2 192.168.10.12:35357 check
EOF

sudo systemctl restart haproxy
```

**For production, use Let's Encrypt**:

```bash
sudo apt install -y certbot python3-certbot-haproxy
sudo certbot --haproxy -d o3k-api.example.com
```

### Step 8: Monitoring and Logging

**Deploy Prometheus + Grafana**:

```bash
# On separate monitoring node or controller1
docker compose -f deployments/monitoring/docker-compose.yml up -d
```

**Contents of `deployments/monitoring/docker-compose.yml`**:

```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana

  node-exporter:
    image: prom/node-exporter:latest
    container_name: node-exporter
    restart: unless-stopped
    ports:
      - "9100:9100"

volumes:
  prometheus-data:
  grafana-data:
```

**Prometheus configuration** (`prometheus.yml`):

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'o3k-controllers'
    static_configs:
      - targets:
        - '192.168.10.11:9090'
        - '192.168.10.12:9090'

  - job_name: 'o3k-compute'
    static_configs:
      - targets:
        - '192.168.10.21:9100'
        - '192.168.10.22:9100'
        - '192.168.10.23:9100'

  - job_name: 'ceph'
    static_configs:
      - targets:
        - '192.168.10.31:9283'  # Ceph exporter
```

---

## Operations

### Health Checks

**Check cluster status**:

```bash
# PostgreSQL HA
patronictl -c /etc/patroni.yml list

# HAProxy stats
curl http://192.168.10.10:9000/stats

# Ceph cluster
sudo ceph -s
sudo ceph osd tree

# O3K services
systemctl status o3k.service

# Compute nodes
openstack compute service list
```

### Scaling Operations

**Add new compute node**:

```bash
# 1. Prepare node (install KVM, configure networking)
# 2. Copy O3K binary and config
# 3. Register with compute registry
# 4. Start O3K service
# 5. Verify: openstack compute service list
```

**Add new OSD**:

```bash
sudo ceph orch daemon add osd ceph4:/dev/sdb
sudo ceph osd tree
```

### Backup and Disaster Recovery

**Database backup**:

```bash
# Automated daily backup script
sudo tee /usr/local/bin/backup-o3k-db.sh <<'EOF'
#!/bin/bash
BACKUP_DIR="/backup/postgres"
DATE=$(date +%Y%m%d-%H%M%S)
mkdir -p $BACKUP_DIR

# Backup from primary
pg_dump -h 192.168.10.10 -U o3k -d o3k | gzip > \
  $BACKUP_DIR/o3k-$DATE.sql.gz

# Retain last 7 days
find $BACKUP_DIR -name "o3k-*.sql.gz" -mtime +7 -delete
EOF

sudo chmod +x /usr/local/bin/backup-o3k-db.sh

# Schedule daily at 2 AM
echo "0 2 * * * /usr/local/bin/backup-o3k-db.sh" | sudo crontab -
```

**Ceph snapshots**:

```bash
# Snapshot volumes pool
sudo rbd snap create volumes@daily-$(date +%Y%m%d)
sudo rbd snap ls volumes

# Restore from snapshot
sudo rbd snap rollback volumes@daily-20260317
```

### Failover Testing

**Test database failover**:

```bash
# On controller1 (primary)
sudo systemctl stop patroni

# Verify failover
patronictl -c /etc/patroni.yml list
# controller2 should become Leader

# Restart controller1
sudo systemctl start patroni
# Should rejoin as Replica
```

**Test HAProxy failover**:

```bash
# On controller1
sudo systemctl stop keepalived

# Verify VIP moved to controller2
ip addr show eth0

# Restart
sudo systemctl start keepalived
```

---

## Performance Tuning

### Database Optimization

```sql
-- Connection pooling (PgBouncer)
sudo apt install -y pgbouncer

-- /etc/pgbouncer/pgbouncer.ini
[databases]
o3k = host=192.168.10.10 port=5432 dbname=o3k

[pgbouncer]
listen_addr = 127.0.0.1
listen_port = 6432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 50
```

### Ceph Performance

```bash
# Enable fast read for RBD
sudo rbd feature enable volumes fast-diff
sudo rbd feature enable images fast-diff

# Tune OSD settings
sudo ceph tell osd.* config set osd_max_backfills 4
sudo ceph tell osd.* config set osd_recovery_max_active 4
```

### Network Optimization

```bash
# Increase MTU for VXLAN
sudo ip link set dev vxlan-tunnel mtu 8950

# Enable jumbo frames on storage network
sudo ip link set dev eth1 mtu 9000  # Ceph storage network
```

---

## Troubleshooting

### Compute Node Not Registering

```bash
# Check compute registry
psql -h 192.168.10.10 -U o3k -d o3k \
  -c "SELECT * FROM compute_nodes;"

# Force re-registration
# Edit config: change node_id
sudo systemctl restart o3k.service
```

### VXLAN Connectivity Issues

```bash
# Check VXLAN interface
ip -d link show vxlan-tunnel

# Test tunnel connectivity
ping 10.255.0.22  # From compute1 to compute2

# Verify FDB entries
bridge fdb show dev vxlan-tunnel
```

### Ceph Slow Performance

```bash
# Check OSD performance
sudo ceph osd perf

# Check network latency
iperf3 -c ceph2  # From ceph1

# Reweight slow OSDs
sudo ceph osd reweight osd.1 0.9
```

---

## Security Best Practices

1. **Change all default passwords**
2. **Use TLS for all endpoints** (Let's Encrypt)
3. **Isolate management network** (VLANs)
4. **Enable firewall rules** (iptables/UFW)
5. **Regular security updates** (unattended-upgrades)
6. **Audit logging** (centralized syslog)
7. **Access control** (SSH keys only, no passwords)
8. **Secrets management** (Vault for sensitive configs)

---

## Conclusion

This scaling guide provides a production-ready O3K deployment with:
- ✅ High Availability (2-3 controllers)
- ✅ Horizontal Scaling (N compute nodes)
- ✅ Shared Storage (Ceph RBD)
- ✅ Load Balancing (HAProxy + Keepalived)
- ✅ Multi-node Networking (VXLAN)
- ✅ Monitoring (Prometheus + Grafana)
- ✅ Backup & DR (automated backups)

**Next Steps**:
- Implement CI/CD for O3K updates
- Add more compute nodes based on workload
- Scale Ceph cluster for capacity
- Integrate with external authentication (LDAP/AD)
- Deploy additional OpenStack services (if needed)

---

**Document Version**: 1.0
**Last Updated**: March 17, 2026
**Tested On**: Ubuntu 24.04 LTS (3-node cluster)
