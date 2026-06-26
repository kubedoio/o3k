# O3K Deployment Scripts

Automated deployment scripts for O3K installation.

---

## deploy-single-node.sh

**Purpose**: Interactive single-node O3K deployment with KVM hypervisor

**What it does**:
- ✅ Checks hardware requirements (CPU, RAM, disk, virtualization support)
- ✅ Installs KVM/libvirt hypervisor
- ✅ Configures network bridge for external connectivity
- ✅ Installs PostgreSQL 18 database
- ✅ Installs Docker and Docker Compose
- ✅ Clones and builds O3K from source
- ✅ Creates optimized configuration file
- ✅ Runs database migrations
- ✅ Creates systemd service for O3K
- ✅ Optionally deploys Horizon dashboard + noVNC
- ✅ Configures firewall (UFW)
- ✅ Installs OpenStack CLI tools
- ✅ Verifies installation
- ✅ Generates environment file for easy CLI access

**Requirements**:
- Fresh Ubuntu 26.04/24.04/22.04 or Debian 12 installation
- Root/sudo access
- Minimum: 4 CPU cores, 16GB RAM, 100GB disk
- CPU with VT-x/AMD-V virtualization support
- Internet connection

**Usage**:

```bash
# Download the script
wget https://raw.githubusercontent.com/cobaltcore-dev/o3k/main/scripts/deploy-single-node.sh

# Make executable
chmod +x deploy-single-node.sh

# Run as root
sudo ./deploy-single-node.sh
```

**Interactive Prompts**:

The script will ask you to configure:

1. **Hostname** - Name for this node (default: o3k-demo)
2. **Host IP** - IP address for this host (auto-detected)
3. **Network Interface** - Primary network interface (auto-detected)
4. **Gateway** - Network gateway (auto-detected)
5. **DNS Servers** - DNS server addresses (default: 8.8.8.8,8.8.4.4)
6. **PostgreSQL Password** - Database password (auto-generated if not provided)
7. **Storage Paths** - Paths for volumes, images, instances (defaults provided)
8. **Horizon Deployment** - Deploy Horizon dashboard? (default: Yes)

**Example Session**:

```bash
$ sudo ./deploy-single-node.sh

╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║   O3K Single-Node Deployment Script                      ║
║   Deploy O3K + KVM Hypervisor on Single Host            ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝

[INFO] Detected OS: ubuntu 24.04
[INFO] Checking hardware requirements...
[SUCCESS] CPU cores: 8 ✓
[SUCCESS] RAM: 32GB ✓
[SUCCESS] Disk space: 250GB ✓
[SUCCESS] Virtualization support: enabled ✓

[INFO] Starting interactive configuration...

Enter hostname for this node [o3k-demo]: my-cloud
Enter host IP address [192.168.1.100]:
Enter primary network interface [eth0]:
Enter network gateway [192.168.1.1]:
Enter DNS servers [8.8.8.8,8.8.4.4]:
Enter PostgreSQL password for O3K database: ********
Enter storage path for volumes [/var/lib/o3k/volumes]:
Enter storage path for images [/var/lib/o3k/images]:
Enter storage path for instances [/var/lib/o3k/instances]:
Deploy Horizon dashboard? [Y/n]: Y

[INFO] Configuration Summary:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Hostname:           my-cloud
  Host IP:            192.168.1.100
  Network Interface:  eth0
  Gateway:            192.168.1.1
  DNS Servers:        8.8.8.8,8.8.4.4
  Volume Path:        /var/lib/o3k/volumes
  Image Path:         /var/lib/o3k/images
  Instance Path:      /var/lib/o3k/instances
  Deploy Horizon:     Y
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Proceed with installation? [Y/n]: Y

[INFO] Starting installation...
[INFO] Updating system packages...
[SUCCESS] System updated
[INFO] Configuring hostname: my-cloud
[SUCCESS] Hostname configured
...
```

**Installation Time**:
- Typical installation: 15-20 minutes
- Depends on: internet speed, hardware performance

**What Gets Installed**:

```
/opt/o3k/                          # O3K installation
├── bin/o3k                        # O3K binary
├── config/o3k.yaml                # Configuration file
└── ...

/opt/horizon/                      # Horizon dashboard (optional)
├── docker-compose.yml
└── local_settings.py

/var/lib/o3k/                      # Storage directories
├── volumes/                       # Cinder volumes
├── images/                        # Glance images
└── instances/                     # Nova instances

/etc/systemd/system/o3k.service    # Systemd service

/root/.o3k-env                     # Environment file for CLI
```

**Post-Installation**:

After successful installation:

```bash
# Source environment
source /root/.o3k-env

# Test authentication
openstack token issue

# List services
openstack service list

# Access Horizon (if deployed)
open http://YOUR_HOST_IP

# Login credentials:
#   Domain: Default
#   Username: admin
#   Password: secret
```

**Verify Installation**:

```bash
# Check O3K service status
systemctl status o3k.service

# View O3K logs
journalctl -u o3k.service -f

# Check Horizon (if deployed)
docker ps | grep horizon

# Test OpenStack CLI
openstack --version
openstack catalog list
```

**Troubleshooting**:

1. **KVM acceleration not available**:
   - Check BIOS/UEFI settings for VT-x/AMD-V
   - Run: `kvm-ok` to verify

2. **Network bridge fails**:
   - Check network interface name: `ip link show`
   - Verify netplan syntax: `netplan try`
   - Rollback: `cp /etc/netplan/backup/*.yaml /etc/netplan/`

3. **O3K service won't start**:
   - Check logs: `journalctl -u o3k.service -n 100`
   - Verify PostgreSQL: `systemctl status postgresql`
   - Check libvirt: `systemctl status libvirtd`

4. **Horizon not accessible**:
   - Check container: `docker ps`
   - View logs: `cd /opt/horizon && docker compose logs horizon`
   - Restart: `docker compose restart`

**Uninstall**:

To remove O3K:

```bash
# Stop services
systemctl stop o3k.service
systemctl disable o3k.service
cd /opt/horizon && docker compose down

# Remove installation
rm -rf /opt/o3k /opt/horizon
rm /etc/systemd/system/o3k.service
rm /root/.o3k-env

# Remove storage (CAUTION: deletes all VMs, volumes, images)
rm -rf /var/lib/o3k

# Restore network (optional)
cp /etc/netplan/backup/*.yaml /etc/netplan/
rm /etc/netplan/01-o3k-bridge.yaml
netplan apply
```

**Related Documentation**:
- [SINGLE_NODE_DEPLOYMENT.md](../docs/SINGLE_NODE_DEPLOYMENT.md) - Manual deployment guide
- [SCALING.md](../docs/SCALING.md) - Production multi-node deployment
- [CONFIGURATION.md](../docs/CONFIGURATION.md) - Configuration reference

---

## upgrade-o3k.sh

**Purpose**: Safely upgrade O3K to the latest version with automatic backup and rollback capability

**What it does**:
- ✅ Creates automatic backup before upgrade
- ✅ Checks current installation and version
- ✅ Pulls latest code from Git repository
- ✅ Builds new O3K binary
- ✅ Runs database migrations
- ✅ Performs service restart
- ✅ Verifies upgrade success
- ✅ Provides rollback on failure
- ✅ Keeps last 5 backups automatically

**Requirements**:
- Existing O3K installation at /opt/o3k
- Root/sudo access
- Git repository intact
- Go compiler installed

**Usage**:

```bash
# Upgrade to latest version (main branch)
sudo ./scripts/upgrade-o3k.sh

# Upgrade to specific version/tag
sudo ./scripts/upgrade-o3k.sh --version v0.6.0

# Force upgrade even if same version
sudo ./scripts/upgrade-o3k.sh --force

# Skip backup (not recommended)
sudo ./scripts/upgrade-o3k.sh --no-backup

# Show help
sudo ./scripts/upgrade-o3k.sh --help
```

**Command Line Options**:

- `--version VERSION` - Upgrade to specific version (tag or branch name)
- `--force` - Force upgrade even if already at target version
- `--no-backup` - Skip backup creation (dangerous, not recommended)
- `-h, --help` - Show help message

**What Gets Backed Up**:

Automatic backup before every upgrade:

```
/opt/o3k-backups/
└── o3k-backup-YYYYMMDD-HHMMSS-<commit>/
    ├── o3k-binary              # Current binary
    ├── o3k.yaml                # Configuration
    ├── commit-hash.txt         # Git commit
    ├── version.txt             # Version tag
    └── metadata.txt            # Backup info
```

**Upgrade Process**:

1. **Pre-checks**:
   - Verify O3K installation exists
   - Get current version and commit
   - Check if service is running

2. **Backup**:
   - Create timestamped backup
   - Save binary, config, git state
   - Keep last 5 backups

3. **Stop Service**:
   - Stop o3k.service gracefully
   - Wait for clean shutdown

4. **Update Code**:
   - Stash local changes (if any)
   - Fetch latest from Git
   - Checkout target version

5. **Build**:
   - Compile new binary
   - Verify build success

6. **Migrate**:
   - Run database migrations
   - Handle migration failures

7. **Start Service**:
   - Start o3k.service
   - Wait for services to be ready

8. **Verify**:
   - Test API endpoints
   - Verify authentication
   - Check service health

**Example Session**:

```bash
$ sudo ./scripts/upgrade-o3k.sh

╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║   O3K Upgrade Script                                     ║
║   Safely upgrade to the latest version                   ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝

[INFO] O3K installation found at /opt/o3k
[INFO] Current version: v0.5.0 (commit: c75f9dd)
[INFO] O3K service is running

[INFO] Starting upgrade process...

[INFO] Creating backup...
[SUCCESS] Backup created: /opt/o3k-backups/o3k-backup-20260317-143022-c75f9dd
[INFO] Kept last 5 backups, removed older ones

[INFO] Stopping O3K service...
[SUCCESS] Service stopped

[INFO] Updating code from repository...
[INFO] Fetching latest changes...
[INFO] Upgrading to latest version (main branch)
[INFO] Checking out origin/main...
[SUCCESS] Code updated to origin/main (commit: a1b2c3d)

[INFO] Building new O3K binary...
[SUCCESS] Binary built successfully
[INFO] Binary size: 36M

[INFO] Running database migrations...
[SUCCESS] Database migrations completed

[INFO] Starting O3K service...
[SUCCESS] Service started successfully

[INFO] Verifying upgrade...
[INFO] Waiting for services to be ready (10 seconds)...
[SUCCESS] ✓ Keystone API responding
[SUCCESS] ✓ Authentication working
[SUCCESS] Upgrade verification completed

╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║   O3K Upgrade Complete! 🎉                               ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝

[INFO] Upgrade Summary:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Previous Version:  v0.5.0 (commit: c75f9dd)
  New Version:       v0.6.0 (commit: a1b2c3d)

  Backup Location:   /opt/o3k-backups/o3k-backup-20260317-143022-c75f9dd
  Configuration:     /opt/o3k/config/o3k.yaml
  Service Status:    active

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[INFO] Next Steps:
  1. Verify services: systemctl status o3k.service
  2. Check logs: journalctl -u o3k.service -f
  3. Test API: openstack token issue
```

**Upgrade Scenarios**:

1. **Regular Upgrade to Latest**:
   ```bash
   sudo ./scripts/upgrade-o3k.sh
   ```

2. **Upgrade to Specific Release**:
   ```bash
   sudo ./scripts/upgrade-o3k.sh --version v0.6.0
   ```

3. **Upgrade to Branch**:
   ```bash
   sudo ./scripts/upgrade-o3k.sh --version feature/new-api
   ```

4. **Force Rebuild**:
   ```bash
   sudo ./scripts/upgrade-o3k.sh --force
   ```

**Automatic Rollback on Failure**:

If migration fails or service won't start:

```
[ERROR] Migration failed. See /tmp/o3k-migrate.log for details
Rollback to previous version? [Y/n]: Y

[WARNING] Starting rollback...
[INFO] Rolling back to: o3k-backup-20260317-143022-c75f9dd
[SUCCESS] Binary restored
[SUCCESS] Service started successfully
[SUCCESS] Rollback completed
```

**Manual Rollback**:

If you need to rollback after upgrade:

```bash
# Find latest backup
ls -t /opt/o3k-backups/ | head -n1

# Restore from backup
sudo systemctl stop o3k.service
sudo cp /opt/o3k-backups/o3k-backup-TIMESTAMP/o3k-binary /opt/o3k/bin/o3k
sudo chmod +x /opt/o3k/bin/o3k
sudo systemctl start o3k.service

# Verify
systemctl status o3k.service
openstack token issue
```

**Troubleshooting**:

1. **Build fails**:
   - Check Go installation: `go version`
   - View build log: `cat /tmp/o3k-build.log`
   - Ensure all dependencies available

2. **Migration fails**:
   - Check database connectivity
   - View migration log: `cat /tmp/o3k-migrate.log`
   - Verify PostgreSQL is running: `systemctl status postgresql`

3. **Service won't start**:
   - Check logs: `journalctl -u o3k.service -n 100`
   - Verify configuration: `cat /opt/o3k/config/o3k.yaml`
   - Check port availability: `netstat -tulpn | grep -E '35357|8774|9696|8776|9292'`

4. **API not responding**:
   - Wait longer (services may need 30+ seconds to start)
   - Check service status: `systemctl status o3k.service`
   - Test manually: `curl http://localhost:35357/v3`

**Safety Features**:

- ✅ **Automatic backup** before any changes
- ✅ **Graceful service stop** with verification
- ✅ **Build verification** before replacement
- ✅ **Migration dry-run** check
- ✅ **Service start verification**
- ✅ **API health checks** post-upgrade
- ✅ **Automatic rollback** on critical failures
- ✅ **Backup retention** (keeps last 5)

**Best Practices**:

1. **Test in non-production first**: Always test upgrades on staging/dev before production
2. **Backup database separately**: Script backs up binary/config, not database
3. **Read release notes**: Check for breaking changes before upgrading
4. **Schedule maintenance window**: Expect 2-5 minutes downtime
5. **Monitor after upgrade**: Watch logs for at least 10 minutes post-upgrade

**Integration with CI/CD**:

```bash
#!/bin/bash
# Automated upgrade in CI/CD pipeline

# Run upgrade
if sudo /opt/o3k/scripts/upgrade-o3k.sh --version "$NEW_VERSION"; then
  echo "Upgrade successful"

  # Run smoke tests
  /opt/o3k/test/smoke_test.sh

else
  echo "Upgrade failed, rollback performed"
  exit 1
fi
```

**Related Documentation**:
- [OPERATIONS.md](../docs/OPERATIONS.md) - Day-to-day management
- [TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md) - Common issues
- [SINGLE_NODE_DEPLOYMENT.md](../docs/SINGLE_NODE_DEPLOYMENT.md) - Initial deployment

---

## Future Scripts

Planned deployment scripts:

- `deploy-multi-node.sh` - Multi-node cluster deployment
- `deploy-ha-cluster.sh` - High availability cluster setup
- `add-compute-node.sh` - Add compute node to existing cluster
- `backup-o3k.sh` - Backup configuration and database
- `restore-o3k.sh` - Restore from backup

---

**Last Updated**: March 17, 2026
