# Operations Guide

Day-to-day operations, monitoring, and maintenance for O3K deployments.

---

## Table of Contents

- [Overview](#overview)
- [Daily Operations](#daily-operations)
- [Monitoring](#monitoring)
- [Backup and Recovery](#backup-and-recovery)
- [Maintenance](#maintenance)
- [Troubleshooting](#troubleshooting)
- [Security](#security)
- [Performance](#performance)
- [High Availability](#high-availability)

---

## Overview

This guide covers operational tasks for running O3K in production environments.

**Target audience:** System administrators, DevOps engineers, SREs

---

## Daily Operations

### Check System Status

```bash
# Docker deployment
docker compose ps

# Expected output:
# NAME           STATUS
# o3k            Up (healthy)
# o3k-postgres   Up (healthy)

# Systemd deployment
sudo systemctl status o3k
sudo systemctl status postgresql
```

### View Logs

```bash
# Docker
docker compose logs o3k --tail=100 --follow

# Systemd
sudo journalctl -u o3k -f

# Check for errors
docker compose logs o3k | grep -i error
```

### Resource Usage

```bash
# Docker container stats
docker stats o3k o3k-postgres

# System resources
top
htop
vmstat 1
```

### Check API Health

```bash
# Keystone
curl http://localhost:5001/v3
# Should return: {"version": {...}}

# Nova
curl http://localhost:8774/v2.1
# Should return: {"versions": [...]}

# Neutron
curl http://localhost:9696/v2.0
# Should return: {"versions": [...]}

# All services
for port in 5001 8774 9696 8776 9292; do
  echo "Port $port: $(curl -s http://localhost:$port | jq -r '.version.status // .versions[0].status // "ERROR"')"
done
```

### Database Health

```bash
# Docker
docker exec o3k-postgres psql -U lightstack -d lightstack -c "SELECT version();"

# Check database size
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  SELECT pg_size_pretty(pg_database_size('lightstack')) AS size;
"

# Check table sizes
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  SELECT schemaname, tablename, pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
  FROM pg_tables
  WHERE schemaname = 'public'
  ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
  LIMIT 10;
"
```

### Active Resources

```bash
# Using OpenStack CLI
source ~/.o3k-env

# Count active resources
openstack server list --all-projects | wc -l
openstack network list --all-projects | wc -l
openstack volume list --all-projects | wc -l

# Resource usage by project
openstack server list --all-projects -f json | jq 'group_by(.Project) | map({project: .[0].Project, count: length})'
```

---

## Monitoring

### Metrics to Monitor

**Service Health:**
- API response times (target: <50ms p95)
- API error rates (target: <1%)
- Service uptime (target: 99.9%)

**Resource Usage:**
- CPU usage (target: <80%)
- Memory usage (target: <85%)
- Disk usage (target: <80%)
- Database connections (target: <80% of max)

**Business Metrics:**
- Total VMs
- Total networks
- Total volumes
- Quota utilization per project

### Health Check Endpoints

```bash
# Keystone health
curl http://localhost:5001/v3

# Nova health
curl http://localhost:8774/v2.1

# Database connectivity (via API)
openstack token issue
```

### Log Monitoring

**Important log patterns to watch:**

```bash
# Errors
docker compose logs o3k | grep -i "ERROR"

# Authentication failures
docker compose logs o3k | grep "authentication failed"

# Database issues
docker compose logs o3k | grep "database"

# Slow queries (if debug logging enabled)
docker compose logs o3k | grep "slow query"
```

### Alerting Rules

**Critical alerts:**
- Service down (any API unreachable for >1 minute)
- Database connection failure
- Disk usage >90%
- Memory usage >95%

**Warning alerts:**
- API response time >100ms (p95)
- Error rate >5%
- Disk usage >80%
- High database connection usage (>80%)

### Prometheus Metrics (Future)

```bash
# Endpoint (when implemented)
curl http://localhost:9090/metrics
```

**Key metrics:**
- `o3k_api_requests_total{service="nova",method="POST",status="200"}`
- `o3k_api_request_duration_seconds{service="nova"}`
- `o3k_database_connections{state="active"}`
- `o3k_instances_total{state="active"}`

---

## Backup and Recovery

### Database Backup

**Automated daily backup:**

```bash
#!/bin/bash
# /usr/local/bin/o3k-backup.sh

BACKUP_DIR="/backups/o3k"
DATE=$(date +%Y%m%d-%H%M%S)
BACKUP_FILE="$BACKUP_DIR/o3k-$DATE.sql.gz"

mkdir -p $BACKUP_DIR

# Create backup
docker exec o3k-postgres pg_dump -U lightstack lightstack | gzip > $BACKUP_FILE

# Keep only last 7 days
find $BACKUP_DIR -name "o3k-*.sql.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_FILE"
```

**Crontab entry:**
```bash
# Daily backup at 2 AM
0 2 * * * /usr/local/bin/o3k-backup.sh
```

**Manual backup:**
```bash
# Backup
docker exec o3k-postgres pg_dump -U lightstack lightstack > o3k-backup.sql

# Compressed backup
docker exec o3k-postgres pg_dump -U lightstack lightstack | gzip > o3k-backup.sql.gz
```

### Database Restore

```bash
# Stop O3K (prevent writes during restore)
docker compose stop o3k

# Restore from backup
cat o3k-backup.sql | docker exec -i o3k-postgres psql -U lightstack lightstack

# Or compressed:
gunzip -c o3k-backup.sql.gz | docker exec -i o3k-postgres psql -U lightstack lightstack

# Start O3K
docker compose start o3k
```

### Configuration Backup

```bash
# Backup configuration
cp docker-compose.yml docker-compose.yml.backup-$(date +%Y%m%d)
cp config/o3k.yaml config/o3k.yaml.backup-$(date +%Y%m%d)

# Or tar everything:
tar -czf o3k-config-$(date +%Y%m%d).tar.gz docker-compose.yml config/ deployments/
```

### Disaster Recovery Plan

**Recovery Time Objective (RTO):** <1 hour
**Recovery Point Objective (RPO):** <24 hours (daily backups)

**Recovery steps:**
1. Install O3K on new system
2. Restore configuration files
3. Restore database from backup
4. Start services
5. Verify all APIs respond
6. Check resource counts

```bash
# Recovery script
#!/bin/bash
set -e

# 1. Setup
cd /opt/o3k
docker compose down -v

# 2. Restore config (from backup location)
cp /backups/config/docker-compose.yml .
cp /backups/config/o3k.yaml config/

# 3. Start database only
docker compose up -d postgres
sleep 10

# 4. Restore database
gunzip -c /backups/o3k/latest.sql.gz | docker exec -i o3k-postgres psql -U lightstack lightstack

# 5. Start O3K
docker compose up -d o3k

# 6. Verify
sleep 10
curl http://localhost:5001/v3 || echo "ERROR: Keystone not responding"
openstack token issue || echo "ERROR: Authentication failed"

echo "Recovery complete!"
```

---

## Maintenance

### Update O3K

```bash
# Backup first!
./o3k-backup.sh

# Pull latest code
cd /opt/o3k
git pull

# Rebuild image
docker compose build o3k

# Stop and start (runs migrations automatically)
docker compose down
docker compose up -d

# Verify
docker compose ps
docker compose logs o3k --tail=50
```

### Database Migrations

```bash
# Check current migration version
docker exec o3k /app/o3k-migrate status

# Run pending migrations
docker exec o3k /app/o3k-migrate up

# Rollback last migration (if needed)
docker exec o3k /app/o3k-migrate down 1
```

### Database Maintenance

```bash
# Vacuum database (reclaim space)
docker exec o3k-postgres psql -U lightstack -d lightstack -c "VACUUM ANALYZE;"

# Reindex (improve query performance)
docker exec o3k-postgres psql -U lightstack -d lightstack -c "REINDEX DATABASE lightstack;"

# Check for table bloat
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  SELECT schemaname, tablename,
         pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
  FROM pg_tables
  WHERE schemaname = 'public'
  ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
"
```

### Log Rotation

**Docker logs:**
```yaml
# docker-compose.yml
services:
  o3k:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

**Systemd logs:**
```bash
# /etc/systemd/journald.conf
SystemMaxUse=1G
SystemKeepFree=2G
```

### Cleanup Old Data

```bash
# Delete old deleted VMs (soft-deleted)
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  DELETE FROM instances
  WHERE deleted = true
    AND deleted_at < NOW() - INTERVAL '30 days';
"

# Delete old tokens (if storing in DB)
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  DELETE FROM tokens WHERE expires_at < NOW();
"
```

---

## Troubleshooting

### Service Won't Start

**Check logs:**
```bash
docker compose logs o3k
docker compose logs postgres
```

**Common issues:**
- Database not ready → Wait and retry
- Port conflict → Check `lsof -i:5001`
- Migration failure → Check database state
- Config error → Validate YAML syntax

### High CPU Usage

```bash
# Check container stats
docker stats o3k

# Check processes inside container
docker exec o3k top

# Check for slow queries (enable query logging)
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  ALTER DATABASE lightstack SET log_min_duration_statement = 1000;
"
```

### High Memory Usage

```bash
# Check memory usage
docker stats o3k

# Restart if memory leak suspected
docker compose restart o3k
```

### Database Connection Exhaustion

```bash
# Check active connections
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  SELECT count(*) FROM pg_stat_activity WHERE datname = 'lightstack';
"

# Kill idle connections
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  SELECT pg_terminate_backend(pid)
  FROM pg_stat_activity
  WHERE datname = 'lightstack'
    AND state = 'idle'
    AND state_change < NOW() - INTERVAL '10 minutes';
"

# Increase max_connections in config
# config/o3k.yaml:
# database:
#   max_connections: 50
```

### Disk Space Issues

```bash
# Check disk usage
df -h

# Check Docker volumes
docker system df

# Clean up old images/containers
docker system prune -a

# Check database size
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  SELECT pg_size_pretty(pg_database_size('lightstack'));
"
```

---

## Security

### Security Checklist

**Configuration:**
- ✅ Changed default admin password
- ✅ Changed JWT secret
- ✅ Using SSL for database connection
- ✅ Firewall configured (only necessary ports open)
- ✅ Strong passwords for all accounts

**Network:**
- ✅ API behind reverse proxy (nginx, traefik)
- ✅ TLS/SSL enabled for all external access
- ✅ Internal services not exposed to internet

**Database:**
- ✅ Database password changed from default
- ✅ Database access restricted by IP
- ✅ Regular backups enabled
- ✅ Backup encryption enabled

### Rotate JWT Secret

```bash
# 1. Generate new secret
NEW_SECRET=$(openssl rand -hex 32)

# 2. Update config
# Edit docker-compose.yml:
# O3K_KEYSTONE_JWT_SECRET: "$NEW_SECRET"

# 3. Restart O3K
docker compose restart o3k

# 4. All existing tokens invalidated - users must re-authenticate
```

### Change Admin Password

```bash
# Using SQL
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  UPDATE users
  SET password_hash = crypt('new-secure-password', gen_salt('bf'))
  WHERE name = 'admin';
"

# Or via OpenStack CLI
openstack user password set
```

### Audit Logs

```bash
# Review recent authentication attempts
docker compose logs o3k | grep "authentication"

# Review API access
docker compose logs o3k | grep "POST\|DELETE"

# Failed login attempts
docker compose logs o3k | grep "authentication failed"
```

### Firewall Configuration

```bash
# Allow only necessary ports
ufw allow 5001/tcp   # Keystone (if external)
ufw allow 8774/tcp   # Nova (if external)
ufw allow 9696/tcp   # Neutron (if external)

# Or allow only from specific IPs
ufw allow from 192.168.1.0/24 to any port 5001
```

---

## Performance

### Optimization Tips

**Database:**
```yaml
database:
  max_connections: 50          # Increase for high concurrency
  max_idle: 10
  conn_max_lifetime: 30m
```

**PostgreSQL tuning:**
```sql
-- /var/lib/postgresql/data/postgresql.conf
shared_buffers = 256MB         # 25% of RAM
effective_cache_size = 1GB     # 50-75% of RAM
work_mem = 16MB
maintenance_work_mem = 128MB
```

**Indexes:**
```sql
-- Ensure key indexes exist
CREATE INDEX IF NOT EXISTS idx_instances_project ON instances(project_id);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);
CREATE INDEX IF NOT EXISTS idx_networks_project ON networks(project_id);
```

### Performance Monitoring

```bash
# API latency
time openstack server list

# Database query time
docker exec o3k-postgres psql -U lightstack -d lightstack -c "
  SELECT query, mean_exec_time, calls
  FROM pg_stat_statements
  ORDER BY mean_exec_time DESC
  LIMIT 10;
"
```

### Scaling Guidelines

**Single-node limits:**
- ~100-200 concurrent VMs
- ~50-100 concurrent API requests
- ~1000 total VMs (database limited)

**When to scale:**
- CPU usage consistently >80%
- Memory usage >85%
- API response time >100ms (p95)
- Database connections >80% of max

**Scaling options:**
1. Vertical: Increase CPU/RAM
2. Horizontal: Add compute nodes (multi-node setup)
3. Database: Use managed PostgreSQL (RDS, Cloud SQL)

---

## High Availability

### Database HA

**Option 1: PostgreSQL Replication**
```yaml
# Primary-Replica setup
services:
  postgres-primary:
    image: postgres:18.3-alpine
    environment:
      POSTGRES_REPLICATION_MODE: master

  postgres-replica:
    image: postgres:18.3-alpine
    environment:
      POSTGRES_REPLICATION_MODE: slave
      POSTGRES_MASTER_HOST: postgres-primary
```

**Option 2: Managed Database**
- AWS RDS PostgreSQL
- Google Cloud SQL
- Azure Database for PostgreSQL

### O3K HA

**Load Balancer:**
```
           ┌─────────────┐
           │   HAProxy   │
           │  (VIP: IP)  │
           └──────┬──────┘
                  │
      ┌───────────┼───────────┐
      │           │           │
   ┌──▼──┐     ┌──▼──┐     ┌──▼──┐
   │ o3k1│     │ o3k2│     │ o3k3│
   └─────┘     └─────┘     └─────┘
      │           │           │
      └───────────┼───────────┘
                  │
           ┌──────▼──────┐
           │  PostgreSQL │
           │  (Primary)  │
           └─────────────┘
```

**HAProxy config:**
```
frontend o3k_keystone
    bind *:5001
    default_backend o3k_keystone_backend

backend o3k_keystone_backend
    balance roundrobin
    option httpchk GET /v3
    server o3k1 10.0.0.1:5001 check
    server o3k2 10.0.0.2:5001 check
    server o3k3 10.0.0.3:5001 check
```

### Health Checks

```bash
# Script: /usr/local/bin/o3k-healthcheck.sh
#!/bin/bash
curl -f http://localhost:5001/v3 && exit 0 || exit 1
```

---

## Summary

**Key operations:**
- ✅ Daily health checks
- ✅ Log monitoring
- ✅ Regular backups
- ✅ Security audits
- ✅ Performance monitoring
- ✅ Database maintenance

**Automation opportunities:**
- Automated backups (cron)
- Log rotation (systemd/Docker)
- Health monitoring (Prometheus)
- Alert notifications (PagerDuty, Slack)

**Next steps:**
- [CONFIGURATION.md](CONFIGURATION.md) - Detailed configuration
- [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md) - Docker-specific operations
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Detailed troubleshooting (if created)
