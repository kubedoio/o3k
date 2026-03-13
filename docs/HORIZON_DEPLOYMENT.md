# Horizon Dashboard Deployment Guide

**O3K Version**: v0.5.0+
**Last Updated**: 2026-03-13
**Status**: Production Ready

## Overview

This guide provides step-by-step instructions for deploying OpenStack Horizon dashboard with O3K as the backend. O3K provides 100% API compatibility with Horizon, supporting all dashboard features including instance management, networking, volumes, images, and VNC console access.

## Prerequisites

- O3K v0.5.0 or higher running
- Docker and Docker Compose installed
- Network connectivity between Horizon and O3K services
- 2GB RAM minimum for Horizon container
- Port 80 (HTTP) available for Horizon dashboard

## Deployment Options

### Option 1: Simplified Horizon Setup (Recommended)

This approach uses a lightweight Horizon configuration without complex Kolla infrastructure.

#### Step 1: Create Deployment Directory

```bash
mkdir -p ~/o3k-horizon
cd ~/o3k-horizon
```

#### Step 2: Create Horizon Configuration

Create `local_settings.py`:

```python
# O3K Horizon Configuration
import os

# Basic Django Settings
DEBUG = False
ALLOWED_HOSTS = ['*']
SECRET_KEY = 'CHANGE-THIS-IN-PRODUCTION-' + os.urandom(32).hex()

# OpenStack Backend Configuration
OPENSTACK_HOST = "o3k"  # Docker service name (or IP if not using Docker)
OPENSTACK_KEYSTONE_URL = "http://%s:35357/v3" % OPENSTACK_HOST
OPENSTACK_KEYSTONE_DEFAULT_ROLE = "_member_"

# API Version Configuration (matches O3K)
OPENSTACK_API_VERSIONS = {
    "identity": 3,      # Keystone v3
    "image": 2,         # Glance v2
    "volume": 3,        # Cinder v3
    "compute": 2.1,     # Nova v2.1
}

# Keystone v3 Configuration
OPENSTACK_KEYSTONE_MULTIDOMAIN_SUPPORT = True
OPENSTACK_KEYSTONE_DEFAULT_DOMAIN = "Default"

# Session Configuration (matches O3K token TTL)
SESSION_TIMEOUT = 14400  # 4 hours
SESSION_COOKIE_SECURE = False  # Set True with HTTPS in production
CSRF_COOKIE_SECURE = False     # Set True with HTTPS in production

# Console Access Configuration
CONSOLE_TYPE = 'novnc'  # VNC console support

# Regional Settings
OPENSTACK_KEYSTONE_REGION_NAME = "RegionOne"
TIME_ZONE = "UTC"

# Service Feature Configuration
OPENSTACK_CINDER_FEATURES = {
    'enable_backup': False,  # Disable if not using Cinder backup
}

OPENSTACK_NEUTRON_NETWORK = {
    'enable_router': True,
    'enable_quotas': True,
    'enable_ipv6': False,
    'enable_distributed_router': False,
    'enable_ha_router': False,
    'enable_fip_topology_check': True,
}

# Logging Configuration
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
        'django': {
            'handlers': ['console'],
            'level': 'INFO',
        },
        'horizon': {
            'handlers': ['console'],
            'level': 'INFO',
        },
        'openstack_dashboard': {
            'handlers': ['console'],
            'level': 'INFO',
        },
    },
}

# Cache Configuration
CACHES = {
    'default': {
        'BACKEND': 'django.core.cache.backends.locmem.LocMemCache',
    }
}
```

#### Step 3: Create Docker Compose Configuration

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  horizon:
    image: quay.io/openstack.kolla/horizon:2023.2-ubuntu-jammy
    container_name: o3k-horizon
    hostname: horizon
    ports:
      - "80:80"
    environment:
      - KOLLA_INSTALL_TYPE=source
      - KOLLA_CONFIG_STRATEGY=COPY_ALWAYS
    volumes:
      - ./kolla-config:/var/lib/kolla/config_files:ro
      - horizon-static:/var/lib/kolla/venv/lib/python3.10/site-packages/static:rw
      - horizon-logs:/var/log/kolla/horizon:rw
    networks:
      - o3k-network
    depends_on:
      - novnc
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/dashboard/auth/login/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s

  novnc:
    image: dougw/novnc:latest
    container_name: o3k-novnc
    hostname: novnc
    ports:
      - "6080:6080"
    environment:
      - DISPLAY_WIDTH=1024
      - DISPLAY_HEIGHT=768
    networks:
      - o3k-network
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "nc", "-z", "localhost", "6080"]
      interval: 30s
      timeout: 5s
      retries: 3

networks:
  o3k-network:
    external: true
    name: deployments_o3k-network  # Must match O3K network name

volumes:
  horizon-static:
  horizon-logs:
```

#### Step 4: Create Kolla Configuration Structure

Create the Kolla configuration directory and files:

```bash
mkdir -p kolla-config
```

Create `kolla-config/config.json`:

```json
{
    "command": "/usr/sbin/apache2ctl -DFOREGROUND",
    "config_files": [
        {
            "source": "/var/lib/kolla/config_files/local_settings",
            "dest": "/etc/openstack-dashboard/local_settings.py",
            "owner": "horizon",
            "perm": "0644"
        }
    ],
    "permissions": [
        {
            "path": "/var/log/kolla/horizon",
            "owner": "horizon:horizon",
            "recurse": true
        },
        {
            "path": "/var/lib/kolla/venv/lib/python3.10/site-packages/static",
            "owner": "horizon:horizon",
            "recurse": true
        }
    ]
}
```

Copy your local_settings.py to the kolla-config directory:

```bash
cp local_settings.py kolla-config/local_settings
```

#### Step 5: Deploy Horizon

```bash
# Start services
docker compose up -d

# Check status
docker compose ps

# Watch logs
docker compose logs -f horizon
```

Wait for the health check to pass (may take 30-60 seconds).

#### Step 6: Access Dashboard

Open your browser to:
```
http://localhost/dashboard
```

**Login Credentials**:
- Domain: `Default`
- User Name: `admin`
- Password: `secret`

### Option 2: Development/Testing Setup

For quick testing without persistence, use this minimal configuration:

```yaml
version: '3.8'

services:
  horizon:
    image: quay.io/openstack.kolla/horizon:2023.2-ubuntu-jammy
    container_name: o3k-horizon-dev
    ports:
      - "8080:80"
    environment:
      - KOLLA_INSTALL_TYPE=source
      - KOLLA_CONFIG_STRATEGY=COPY_ALWAYS
      - OPENSTACK_HOST=host.docker.internal  # For local O3K
      - OPENSTACK_KEYSTONE_URL=http://host.docker.internal:35357/v3
    volumes:
      - ./kolla-config:/var/lib/kolla/config_files:ro
    restart: "no"

networks:
  default:
    name: bridge
```

## Configuration Details

### Network Configuration

**Docker Network Name**: The `o3k-network` must match your O3K deployment network. Check with:

```bash
docker network ls | grep o3k
```

Common network names:
- `deployments_o3k-network` (deployments/docker-compose.yml)
- `o3k_default` (root docker-compose.yml)
- Custom name specified in your O3K compose file

Update `docker-compose.yml` accordingly:

```yaml
networks:
  o3k-network:
    external: true
    name: YOUR_ACTUAL_NETWORK_NAME
```

### Service Discovery

Horizon resolves O3K services using the `OPENSTACK_HOST` setting. Options:

1. **Docker service name** (recommended): `OPENSTACK_HOST = "o3k"`
   - Requires Horizon and O3K on same Docker network

2. **Host IP address**: `OPENSTACK_HOST = "192.168.1.100"`
   - Use for non-Docker O3K deployments

3. **Localhost (host.docker.internal)**: `OPENSTACK_HOST = "host.docker.internal"`
   - For O3K running on Docker host machine

### Console Access

VNC console requires noVNC proxy. Configuration:

```python
CONSOLE_TYPE = 'novnc'
```

O3K generates console tokens that Horizon uses to establish VNC connections through the noVNC proxy (port 6080).

## Verification

### 1. Health Checks

```bash
# Check container status
docker compose ps

# Verify Horizon is responding
curl -I http://localhost/dashboard/auth/login/

# Check O3K connectivity from Horizon
docker exec o3k-horizon curl -s http://o3k:35357/v3 | jq
```

### 2. Login Test

1. Navigate to `http://localhost/dashboard`
2. Enter credentials (Domain: Default, User: admin, Password: secret)
3. Verify successful login to Overview page

### 3. Feature Testing

Test key Horizon features:

```bash
# Instance Management
# - Navigate to Compute > Instances
# - Click "Launch Instance"
# - Verify wizard loads with flavors, images, networks

# Network Topology
# - Navigate to Network > Network Topology
# - Verify topology visualization loads

# VNC Console
# - Launch or select an existing instance
# - Click instance name > Console tab
# - Verify noVNC console loads

# Volume Management
# - Navigate to Volumes > Volumes
# - Click "Create Volume"
# - Verify volume creation workflow
```

## Troubleshooting

### Container Restart Loops

**Symptom**: `docker compose ps` shows "Restarting"

**Diagnosis**:
```bash
docker logs o3k-horizon --tail 50
```

**Common Issues**:

1. **KOLLA_CONFIG_STRATEGY not set**:
   ```
   ERROR: KOLLA_CONFIG_STRATEGY is not set properly
   ```
   Solution: Add to docker-compose.yml environment:
   ```yaml
   - KOLLA_CONFIG_STRATEGY=COPY_ALWAYS
   ```

2. **Missing config.json**:
   ```
   FileNotFoundError: /var/lib/kolla/config_files/config.json
   ```
   Solution: Ensure `kolla-config/config.json` exists and is mounted

3. **Apache fails to start**:
   ```
   AH00015: Unable to open logs
   ```
   Solution: Add named volume for logs:
   ```yaml
   volumes:
     - horizon-logs:/var/log/kolla/horizon:rw
   ```

### Connection Refused

**Symptom**: Horizon shows "Unable to retrieve ..." errors

**Diagnosis**:
```bash
# From Horizon container
docker exec o3k-horizon curl -v http://o3k:35357/v3

# Check network connectivity
docker exec o3k-horizon ping -c 3 o3k
```

**Solutions**:

1. **Wrong network**: Verify both containers on same network:
   ```bash
   docker network inspect deployments_o3k-network
   ```

2. **Wrong OPENSTACK_HOST**: Update `local_settings.py`:
   ```python
   OPENSTACK_HOST = "correct-service-name"
   ```

3. **O3K not running**: Start O3K services:
   ```bash
   cd ~/git/o3k
   docker compose -f deployments/docker-compose.yml up -d
   ```

### Authentication Errors

**Symptom**: "Unable to authenticate" on login

**Checks**:

1. Verify O3K Keystone is responsive:
   ```bash
   curl http://localhost:35357/v3
   ```

2. Verify default user exists:
   ```bash
   export OS_AUTH_URL=http://localhost:35357/v3
   export OS_USERNAME=admin
   export OS_PASSWORD=secret
   export OS_PROJECT_NAME=default
   export OS_USER_DOMAIN_NAME=Default
   export OS_PROJECT_DOMAIN_NAME=Default

   openstack token issue
   ```

3. Check Horizon logs for authentication errors:
   ```bash
   docker logs o3k-horizon | grep -i auth
   ```

### VNC Console Issues

**Symptom**: Console tab shows "Unable to connect"

**Checks**:

1. Verify noVNC container is running:
   ```bash
   docker ps | grep novnc
   curl http://localhost:6080
   ```

2. Check O3K Nova console token generation:
   ```bash
   # Create instance first
   openstack server create --flavor m1.small --image cirros test-vm

   # Get console URL
   openstack console url show test-vm
   ```

3. Verify network connectivity between Horizon, noVNC, and O3K

### Performance Issues

**Symptom**: Dashboard is slow to load

**Solutions**:

1. **Increase container resources**:
   ```yaml
   services:
     horizon:
       deploy:
         resources:
           limits:
             memory: 2G
             cpus: '2'
   ```

2. **Enable caching**: Update `local_settings.py`:
   ```python
   CACHES = {
       'default': {
           'BACKEND': 'django.core.cache.backends.memcached.PyMemcacheCache',
           'LOCATION': 'memcached:11211',
       }
   }
   ```

   Add memcached service to docker-compose.yml:
   ```yaml
   memcached:
     image: memcached:alpine
     container_name: o3k-memcached
     networks:
       - o3k-network
   ```

3. **Reduce session timeout**: Lower SESSION_TIMEOUT in `local_settings.py`

## Production Considerations

### Security

1. **Change SECRET_KEY**: Generate unique secret:
   ```python
   SECRET_KEY = os.urandom(64).hex()
   ```

2. **Enable HTTPS**: Use reverse proxy (nginx, traefik) with TLS certificates:
   ```yaml
   services:
     nginx:
       image: nginx:alpine
       ports:
         - "443:443"
       volumes:
         - ./nginx.conf:/etc/nginx/nginx.conf:ro
         - ./certs:/etc/nginx/certs:ro
   ```

3. **Restrict ALLOWED_HOSTS**:
   ```python
   ALLOWED_HOSTS = ['horizon.example.com', '10.0.1.50']
   ```

4. **Enable secure cookies**:
   ```python
   SESSION_COOKIE_SECURE = True
   CSRF_COOKIE_SECURE = True
   ```

### High Availability

For production HA setup:

1. **Multiple Horizon instances**: Run 2+ Horizon containers behind load balancer
2. **Shared session storage**: Use Redis/Memcached for sessions
3. **Persistent volumes**: Use network storage for static files
4. **Health checks**: Configure load balancer health checks

Example with Redis sessions:

```python
SESSION_ENGINE = 'django.contrib.sessions.backends.cache'
CACHES = {
    'default': {
        'BACKEND': 'django_redis.cache.RedisCache',
        'LOCATION': 'redis://redis:6379/0',
    }
}
```

### Monitoring

Monitor Horizon health:

```bash
# Container metrics
docker stats o3k-horizon

# Application logs
docker logs -f o3k-horizon

# HTTP response times
curl -w "@curl-format.txt" -o /dev/null -s http://localhost/dashboard/

# Health endpoint
watch -n 10 'curl -f http://localhost/dashboard/auth/login/ || echo "DOWN"'
```

## Maintenance

### Updates

Update Horizon to newer version:

```bash
# Pull new image
docker compose pull horizon

# Recreate container
docker compose up -d horizon

# Verify
docker compose logs -f horizon
```

### Backups

Backup Horizon configuration:

```bash
tar czf horizon-backup-$(date +%Y%m%d).tar.gz \
  local_settings.py \
  docker-compose.yml \
  kolla-config/
```

### Cleanup

Remove Horizon deployment:

```bash
# Stop and remove containers
docker compose down -v

# Remove configuration
cd ~ && rm -rf o3k-horizon
```

## Advanced Configuration

### Custom Branding

Customize Horizon appearance by mounting custom static files:

```yaml
volumes:
  - ./custom-logo.png:/var/lib/kolla/venv/lib/python3.10/site-packages/openstack_dashboard/static/dashboard/img/logo.png:ro
```

### Additional Panels

Install custom Horizon panels:

```bash
# Create Dockerfile
FROM quay.io/openstack.kolla/horizon:2023.2-ubuntu-jammy
RUN pip install horizon-custom-panel
COPY panel-config.py /etc/openstack-dashboard/enabled/
```

Build and use custom image:

```bash
docker build -t o3k-horizon-custom .
```

Update docker-compose.yml:

```yaml
services:
  horizon:
    image: o3k-horizon-custom
```

### Multi-Region Support

Configure Horizon for multiple O3K regions:

```python
AVAILABLE_REGIONS = [
    ('http://region1:35357/v3', 'RegionOne'),
    ('http://region2:35357/v3', 'RegionTwo'),
]
OPENSTACK_KEYSTONE_URL = AVAILABLE_REGIONS[0][0]
```

## Support

- **Documentation**: https://github.com/cobaltcore-dev/o3k/tree/main/docs
- **Issues**: https://github.com/cobaltcore-dev/o3k/issues
- **Spec**: [OpenStack Horizon 100% Compatibility](../specs/002-horizon-full-compatibility/)

## References

- OpenStack Horizon Documentation: https://docs.openstack.org/horizon/latest/
- Kolla Documentation: https://docs.openstack.org/kolla/latest/
- O3K API Documentation: [docs/API.md](./API.md)
- O3K Configuration Guide: [docs/CONFIGURATION.md](./CONFIGURATION.md)
