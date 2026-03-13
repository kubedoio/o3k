# Installation Guide

Complete installation guide for O3K (OpenStack-compatible cloud platform).

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
  - [Docker Compose (Recommended)](#docker-compose-recommended)
  - [Binary Installation](#binary-installation)
- [First Run](#first-run)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

---

## Prerequisites

### System Requirements

**Minimum:**
- CPU: 2 cores
- RAM: 4GB
- Disk: 10GB free space
- OS: Linux (Ubuntu 20.04+, Debian 11+, RHEL 8+)

**Recommended:**
- CPU: 4+ cores
- RAM: 8GB+
- Disk: 50GB+ free space
- OS: Ubuntu 22.04 LTS or macOS (ARM64/AMD64)

### Required Software

**For Docker Compose Installation:**
- Docker 20.10+ ([install guide](https://docs.docker.com/engine/install/))
- Docker Compose V2 (included with Docker Desktop)

**For Binary Installation:**
- PostgreSQL 18+ ([install guide](https://www.postgresql.org/download/))
- Go 1.26+ (for building from source)

---

## Installation Methods

### Docker Compose (Recommended)

Docker Compose provides the easiest way to get O3K running with all dependencies.

#### Step 1: Clone Repository

```bash
git clone https://github.com/yourusername/lightstack.git
cd lightstack
```

#### Step 2: Start Services

```bash
docker compose up -d
```

This starts:
- **PostgreSQL** on port 5432
- **O3K APIs** on ports 5001, 8774, 9696, 8776, 9292, 8775

#### Step 3: Wait for Services

```bash
# Check status
docker compose ps

# Wait for healthy status
docker compose logs -f o3k
```

First startup takes 30-60 seconds as migrations run automatically.

#### Step 4: Configure CLI Environment

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

# Load environment
source ~/.o3k-env
```

#### Step 5: Install OpenStack CLI

```bash
# Using pipx (recommended)
brew install pipx  # macOS
# or: apt install pipx  # Ubuntu/Debian

pipx install python-openstackclient
```

✅ **Installation Complete!** Jump to [Verification](#verification).

---

### Binary Installation

For advanced users who want to run O3K directly without Docker.

#### Step 1: Install PostgreSQL

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install postgresql-14 postgresql-client-14
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

**macOS:**
```bash
brew install postgresql@14
brew services start postgresql@14
```

#### Step 2: Create Database

```bash
sudo -u postgres psql << EOF
CREATE DATABASE lightstack;
CREATE USER lightstack WITH PASSWORD 'secret';
GRANT ALL PRIVILEGES ON DATABASE lightstack TO lightstack;
EOF
```

#### Step 3: Build O3K

```bash
git clone https://github.com/yourusername/lightstack.git
cd lightstack

# Build
go build -o o3k ./cmd/o3k
go build -o o3k-migrate ./cmd/o3k-migrate

# Move binaries
sudo mv o3k /usr/local/bin/
sudo mv o3k-migrate /usr/local/bin/
```

#### Step 4: Configure O3K

```bash
# Create config directory
sudo mkdir -p /etc/o3k

# Copy config
sudo cp config/o3k.yaml /etc/o3k/

# Edit config
sudo nano /etc/o3k/o3k.yaml
```

Update database URL:
```yaml
database:
  url: "postgres://lightstack:secret@localhost/lightstack?sslmode=disable"
```

#### Step 5: Run Migrations

```bash
o3k-migrate up
```

#### Step 6: Start O3K

```bash
# Run in foreground
o3k --config /etc/o3k/o3k.yaml

# Or create systemd service (see docs/OPERATIONS.md)
```

✅ **Installation Complete!** Jump to [Verification](#verification).

---

## First Run

### Default Credentials

```
Domain:   Default
Username: admin
Password: secret
Project:  default
```

⚠️ **Change these in production!**

### Environment Setup

All OpenStack CLI commands need these environment variables:

```bash
export OS_AUTH_URL=http://localhost:5001/v3  # or :35357 for binary
export OS_PROJECT_NAME=default
export OS_PROJECT_DOMAIN_NAME=Default
export OS_USERNAME=admin
export OS_PASSWORD=secret
export OS_USER_DOMAIN_NAME=Default
export OS_IDENTITY_API_VERSION=3
```

Save to `~/.o3k-env` and source it:
```bash
source ~/.o3k-env
```

---

## Verification

### 1. Check Services

**Docker Compose:**
```bash
docker compose ps

# Should show:
# o3k            Up (healthy)
# o3k-postgres   Up (healthy)
```

**Binary:**
```bash
curl http://localhost:35357/v3
# Should return JSON with version info
```

### 2. Test Authentication

```bash
source ~/.o3k-env
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

### 3. List Resources

```bash
# List flavors
openstack flavor list

# List networks
openstack network list

# Show quotas
openstack quota show --compute
```

### 4. Create Test VM

```bash
# Create network
openstack network create test-net

# Create VM
openstack server create \
  --flavor m1.small \
  --image cirros \
  --network test-net \
  test-vm

# Verify
openstack server list
```

✅ **If all commands succeed, O3K is working correctly!**

---

## Troubleshooting

### Issue: Authentication Fails (401)

**Symptoms:**
```
invalid credentials (HTTP 401)
```

**Solutions:**
1. Check credentials match defaults (Domain: "Default" with capital D)
2. Verify O3K is running: `docker compose ps` or `curl http://localhost:35357/v3`
3. Check database has domain:
   ```bash
   docker exec o3k-postgres psql -U lightstack -d lightstack \
     -c "SELECT * FROM domains;"
   ```

### Issue: Port Already in Use

**Symptoms:**
```
Bind for 0.0.0.0:5001 failed: port is already allocated
```

**Solutions:**
```bash
# Find what's using the port
lsof -i:5001

# Kill the process or change port in docker-compose.yml
```

### Issue: Database Connection Failed

**Symptoms:**
```
failed to connect to postgres
```

**Solutions:**
1. Verify PostgreSQL is running:
   ```bash
   docker compose ps postgres
   # or: sudo systemctl status postgresql
   ```

2. Check database exists:
   ```bash
   docker exec o3k-postgres psql -U lightstack -l
   ```

3. Test connection:
   ```bash
   psql "postgres://lightstack:secret@localhost/lightstack?sslmode=disable"
   ```

### Issue: Migrations Failed

**Symptoms:**
```
migration failed: relation "users" does not exist
```

**Solutions:**
```bash
# Check migration status
docker exec o3k /app/o3k-migrate status

# Run migrations manually
docker exec o3k /app/o3k-migrate up

# Reset and rerun (⚠️ deletes all data)
docker exec o3k /app/o3k-migrate reset
docker exec o3k /app/o3k-migrate up
```

### Issue: Service Not Healthy

**Symptoms:**
```
o3k   Up 2 minutes (unhealthy)
```

**Solutions:**
```bash
# Check logs
docker compose logs o3k --tail=50

# Common issues:
# - Database not ready: wait 10 more seconds
# - Port conflict: check lsof output
# - Config error: check o3k.yaml syntax
```

### Issue: OpenStack CLI Not Found

**Symptoms:**
```
command not found: openstack
```

**Solutions:**
```bash
# Install with pipx
pipx install python-openstackclient

# Or with pip
pip install python-openstackclient

# Verify
openstack --version
```

### Getting Help

1. **Check logs:**
   ```bash
   docker compose logs o3k
   ```

2. **Check database:**
   ```bash
   docker exec o3k-postgres psql -U lightstack -d lightstack
   ```

3. **Test APIs directly:**
   ```bash
   curl http://localhost:5001/v3
   curl http://localhost:8774/v2.1/
   ```

4. **GitHub Issues:** Report bugs at repository issues page

---

## Next Steps

### Learn the Basics
- **[Quickstart Guide](QUICKSTART.md)** - Create your first VM in 5 minutes
- **[Operations Guide](OPERATIONS.md)** - Day-to-day management tasks

### Configure Your Installation
- **[Configuration Guide](CONFIGURATION.md)** - Networking, storage, security
- **[Docker Deployment](DOCKER_DEPLOYMENT.md)** - Docker-specific configuration

### Advanced Topics
- **[Multi-Architecture](MULTIARCH.md)** - Building for ARM/AMD64
- **[Architecture](ARCHITECTURE.md)** - System design and components
- **[API Reference](API.md)** - OpenStack API compatibility

### Production Deployment
See `docs/OPERATIONS.md` for:
- Systemd service setup
- Monitoring and logging
- Backup strategies
- Security hardening
- High availability

---

## Quick Reference

### Common Commands

```bash
# Docker Compose
docker compose up -d          # Start services
docker compose down           # Stop services
docker compose ps             # Check status
docker compose logs -f o3k    # View logs

# OpenStack CLI
source ~/.o3k-env             # Load environment
openstack server list         # List VMs
openstack network list        # List networks
openstack volume list         # List volumes
openstack quota show          # Show quotas

# Database
docker exec o3k-postgres psql -U lightstack -d lightstack  # Connect
docker exec o3k /app/o3k-migrate status                    # Migration status
```

### Default Ports

| Service | Port | Description |
|---------|------|-------------|
| Keystone | 5001 | Identity/Auth (Docker) |
| Keystone | 35357 | Identity/Auth (Binary) |
| Nova | 8774 | Compute |
| Neutron | 9696 | Network |
| Cinder | 8776 | Block Storage |
| Glance | 9292 | Images |
| Metadata | 8775 | VM Metadata |
| PostgreSQL | 5432 | Database |

---

## Support

- **Documentation:** `/docs` directory
- **Examples:** `docs/OPERATIONS.md`
- **Issues:** GitHub Issues
- **Community:** GitHub Discussions

---

**Congratulations! O3K is now installed and ready to use!** 🚀
