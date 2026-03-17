# Quick Start Guide

**📚 Complete Documentation**: See **[INDEX.md](INDEX.md)** for full documentation index with learning paths.

Get O3K running and create your first VM in **5 minutes**.

---

## Prerequisites

- Docker and Docker Compose V2 installed
- 4GB+ RAM available
- Basic familiarity with terminal/CLI

---

## Step 1: Start O3K (30 seconds)

```bash
cd /Users/I761222/git/lightstack
docker compose up -d
```

**Expected output:**
```
[+] Running 2/2
 ✔ Container o3k-postgres  Started
 ✔ Container o3k           Started
```

Wait for services to be healthy (~30 seconds):
```bash
docker compose ps
```

**Should show:**
```
NAME           STATUS
o3k            Up (healthy)
o3k-postgres   Up (healthy)
```

---

## Step 2: Install OpenStack CLI (1 minute)

```bash
# macOS
brew install pipx
pipx install python-openstackclient

# Ubuntu/Debian
apt install pipx
pipx install python-openstackclient

# Verify
openstack --version
```

---

## Step 3: Configure Environment (10 seconds)

```bash
# Create environment file
cat > ~/.o3k-env << 'EOF'
export OS_AUTH_URL=http://localhost:5001/v3
export OS_PROJECT_NAME=default
export OS_PROJECT_DOMAIN_NAME=Default
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_USER_DOMAIN_NAME=Default
export OS_IDENTITY_API_VERSION=3
EOF

# Load it
source ~/.o3k-env
```

---

## Step 4: Verify Installation (30 seconds)

```bash
# Test authentication
openstack token issue
```

**Expected output:**
```
+------------+------------------+
| Field      | Value            |
+------------+------------------+
| expires    | 2026-03-08...    |
| id         | eyJhbGc...       |
| project_id | 00000000-...     |
| user_id    | 00000000-...     |
+------------+------------------+
```

```bash
# List available flavors
openstack flavor list
```

**Expected output:**
```
+--------------------------------------+-----------+-------+------+-----------+-------+
| ID                                   | Name      |   RAM | Disk | Ephemeral | VCPUs |
+--------------------------------------+-----------+-------+------+-----------+-------+
| 00000000-0000-0000-0000-000000000010 | m1.tiny   |   512 |    1 |         0 |     1 |
| 00000000-0000-0000-0000-000000000011 | m1.small  |  2048 |   20 |         0 |     1 |
| 00000000-0000-0000-0000-000000000012 | m1.medium |  4096 |   40 |         0 |     2 |
| 00000000-0000-0000-0000-000000000013 | m1.large  |  8192 |   80 |         0 |     4 |
| 00000000-0000-0000-0000-000000000014 | m1.xlarge | 16384 |  160 |         0 |     8 |
+--------------------------------------+-----------+-------+------+-----------+-------+
```

---

## Step 5: Create Network (30 seconds)

```bash
# Create a private network
openstack network create my-network

# Create a subnet
openstack subnet create \
  --network my-network \
  --subnet-range 192.168.1.0/24 \
  my-subnet

# Verify
openstack network list
```

---

## Step 6: Create Your First VM (1 minute)

```bash
# Create a VM
openstack server create \
  --flavor m1.small \
  --image cirros \
  --network my-network \
  my-first-vm
```

**Expected output:**
```
+-----------------------------+--------------------------------------------------+
| Field                       | Value                                            |
+-----------------------------+--------------------------------------------------+
| id                          | d5490d6d-a79c-4f37-9f9b-c926c3e66c6f            |
| name                        | my-first-vm                                      |
| status                      | ACTIVE                                           |
| OS-EXT-STS:power_state      | Running                                          |
| flavor                      | m1.small (1 vCPU, 2048 MB RAM, 20 GB Disk)      |
| image                       | cirros                                           |
| created                     | 2026-03-07T21:47:31Z                             |
+-----------------------------+--------------------------------------------------+
```

```bash
# List all VMs
openstack server list
```

**Expected output:**
```
+--------------------------------------+-------------+--------+----------+--------+-----------+
| ID                                   | Name        | Status | Networks | Image  | Flavor    |
+--------------------------------------+-------------+--------+----------+--------+-----------+
| d5490d6d-a79c-4f37-9f9b-c926c3e66c6f | my-first-vm | ACTIVE | N/A      | cirros | m1.small  |
+--------------------------------------+-------------+--------+----------+--------+-----------+
```

---

## Step 7: Inspect Your VM (30 seconds)

```bash
# Get detailed VM information
openstack server show my-first-vm

# Check resource quotas
openstack quota show --compute
```

**Quota output:**
```
+-----------------------------+-------+
| Resource                    | Limit |
+-----------------------------+-------+
| instances                   |    10 |
| instances_used              |     1 |
| cores                       |    20 |
| cores_used                  |     1 |
| ram                         | 51200 |
| ram_used                    |  2048 |
+-----------------------------+-------+
```

---

## Step 8: Create More Resources (2 minutes)

### Create Multiple VMs

```bash
# Create a web server tier (2 small VMs)
openstack server create \
  --flavor m1.small \
  --image cirros \
  --network my-network \
  web-server-1

openstack server create \
  --flavor m1.small \
  --image cirros \
  --network my-network \
  web-server-2

# Create a database server (larger flavor)
openstack server create \
  --flavor m1.medium \
  --image cirros \
  --network my-network \
  db-server

# List all servers
openstack server list
```

### Create a Volume

```bash
# Create a 10GB volume
openstack volume create --size 10 data-volume

# List volumes
openstack volume list

# Attach volume to VM
openstack server add volume my-first-vm data-volume
```

---

## Common Operations

### VM Lifecycle

```bash
# Stop a VM
openstack server stop my-first-vm

# Start a VM
openstack server start my-first-vm

# Reboot a VM
openstack server reboot my-first-vm

# Delete a VM
openstack server delete my-first-vm
```

### Resource Management

```bash
# List all resources
openstack server list        # VMs
openstack network list       # Networks
openstack volume list        # Volumes
openstack flavor list        # Available VM sizes
openstack image list         # Available images

# Show details
openstack server show <vm-name>
openstack network show <network-name>
openstack volume show <volume-name>
```

### Check System Status

```bash
# View container logs
docker compose logs o3k --tail=50

# Check health
docker compose ps

# View database
docker exec o3k-postgres psql -U lightstack -d lightstack -c "SELECT COUNT(*) FROM instances;"
```

---

## Stopping and Restarting

### Stop O3K

```bash
docker compose down
```

**Note:** This preserves all data (VMs, networks, volumes) in the PostgreSQL database.

### Restart O3K

```bash
docker compose up -d
```

All your resources will still be there:
```bash
source ~/.o3k-env
openstack server list
```

### Clean Start (Delete All Data)

```bash
# WARNING: This deletes everything!
docker compose down -v
docker compose up -d
```

---

## Troubleshooting

### Authentication Fails

**Symptom:** `invalid credentials (HTTP 401)`

**Solution:**
```bash
# Verify environment is loaded
echo $OS_AUTH_URL

# Reload environment
source ~/.o3k-env

# Check O3K is running
docker compose ps
```

### Port Already in Use

**Symptom:** `Bind for 0.0.0.0:5001 failed: port is already allocated`

**Solution:**
```bash
# Find what's using the port
lsof -i:5001

# Stop the conflicting service or change O3K port in docker-compose.yml
```

### Services Not Healthy

**Symptom:** `o3k   Up 2 minutes (unhealthy)`

**Solution:**
```bash
# Check logs
docker compose logs o3k --tail=50

# Common fix: Wait 10 more seconds for database
docker compose ps

# Restart if needed
docker compose restart o3k
```

### OpenStack CLI Not Found

**Symptom:** `command not found: openstack`

**Solution:**
```bash
# Install with pipx
pipx install python-openstackclient

# Ensure pipx bin is in PATH
pipx ensurepath

# Restart terminal or run:
source ~/.bashrc   # or ~/.zshrc
```

---

## Quick Reference

### Essential Commands

```bash
# Environment
source ~/.o3k-env                    # Load credentials

# Authentication
openstack token issue                # Get auth token

# VMs
openstack server create --flavor m1.small --image cirros --network <net> <name>
openstack server list                # List VMs
openstack server show <name>         # VM details
openstack server delete <name>       # Delete VM

# Networks
openstack network create <name>      # Create network
openstack network list               # List networks

# Volumes
openstack volume create --size 10 <name>  # Create volume
openstack volume list                     # List volumes
openstack server add volume <vm> <vol>    # Attach volume

# Quotas
openstack quota show --compute       # Show resource limits
```

### Service Ports

| Service | Port | URL |
|---------|------|-----|
| Keystone (Identity) | 5001 | http://localhost:5001/v3 |
| Nova (Compute) | 8774 | http://localhost:8774/v2.1 |
| Neutron (Network) | 9696 | http://localhost:9696/v2.0 |
| Cinder (Volumes) | 8776 | http://localhost:8776/v3 |
| Glance (Images) | 9292 | http://localhost:9292 |
| Metadata | 8775 | http://localhost:8775 |

### Default Credentials

```
Domain:   Default
Username: admin
Password: secret
Project:  default
```

⚠️ **Change these in production!**

---

## What's Next?

### Learn More

- **[Installation Guide](INSTALLATION.md)** - Detailed setup options (Docker, binary)
- **[Configuration Guide](CONFIGURATION.md)** - Customize your deployment
- **[Operations Guide](OPERATIONS.md)** - Production deployment and management
- **[API Reference](API.md)** - Complete API documentation

### Try Advanced Features

```bash
# Create a 3-tier application
openstack network create app-network

# Web tier (2 instances)
openstack server create --flavor m1.small --image cirros --network app-network web-1
openstack server create --flavor m1.small --image cirros --network app-network web-2

# App tier (2 instances)
openstack server create --flavor m1.medium --image cirros --network app-network app-1
openstack server create --flavor m1.medium --image cirros --network app-network app-2

# Database tier (with storage)
openstack volume create --size 50 db-volume
openstack server create --flavor m1.large --image cirros --network app-network --volume db-volume db-1

# View deployment
openstack server list
openstack network list
openstack volume list
```

---

## Success!

You now have a fully functional OpenStack-compatible cloud running locally! 🎉

**What you've accomplished:**
- ✅ Deployed O3K in Docker containers
- ✅ Configured OpenStack CLI
- ✅ Created virtual machines
- ✅ Created networks and volumes
- ✅ Managed resource quotas

**Your O3K cloud can:**
- Create and manage virtual machines
- Provision isolated networks
- Attach persistent block storage
- Track resource usage and quotas
- Support multiple tenants (projects)

**Ready for production?** See [OPERATIONS.md](OPERATIONS.md) for deployment best practices.
