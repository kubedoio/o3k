# Storage Modes Guide

O3K supports multiple storage backends for both Cinder (block storage) and Glance (image service), allowing flexible deployment options from development to production.

## Overview

### Cinder Storage Modes
- **stub**: Simulated storage (no actual storage operations)
- **local**: Local qcow2 files on libvirt host (`~/.o3k/volumes/`)
- **rbd**: Ceph RBD volumes (shared storage cluster)
- **local,rbd**: Hybrid mode (local + RBD for performance + redundancy)

### Glance Storage Modes
- **stub**: Simulated storage (no actual storage operations)
- **local**: Local raw files (`~/.o3k/images/`)
- **rbd**: Ceph RBD images (shared storage cluster)
- **s3**: S3-compatible object storage (AWS S3, MinIO, Ceph RGW)
- **local,rbd**: Hybrid mode (local cache + RBD backing)
- **local,s3**: Hybrid mode (local cache + S3 backing)
- **rbd,s3**: Dual backend (RBD primary, S3 fallback)

## Configuration

### Cinder Configuration (`config/o3k.yaml`)

```yaml
cinder:
  port: 8776
  ceph_pool: volumes
  ceph_conf: /etc/ceph/ceph.conf
  storage_mode: local  # "stub", "local", "rbd", or "local,rbd"
```

### Glance Configuration (`config/o3k.yaml`)

```yaml
glance:
  port: 9292
  ceph_pool: images
  ceph_conf: /etc/ceph/ceph.conf
  storage_mode: local,s3  # "stub", "local", "rbd", "s3", "local,rbd", "local,s3", "rbd,s3"
  s3_bucket: my-glance-images
  s3_region: us-east-1
  s3_endpoint: ""  # Optional: Custom S3 endpoint (for MinIO, Ceph RGW, etc.)
```

## Storage Mode Details

### 1. Stub Mode

**Use Case**: Development, testing without storage
**Behavior**: All operations are simulated in memory

```yaml
storage_mode: stub
```

**Characteristics**:
- No actual disk I/O
- Data lost on restart
- Zero dependencies
- Instant operations

### 2. Local Mode

**Use Case**: Single-node deployments, development, testing
**Behavior**: Stores files on local filesystem

```yaml
storage_mode: local
```

**Storage Locations**:
- Cinder volumes: `~/.o3k/volumes/volume-{uuid}.qcow2`
- Glance images: `~/.o3k/images/image-{uuid}.raw`

**Characteristics**:
- Fast (local disk I/O)
- No shared storage required
- Data persists across restarts
- No multi-node support
- Uses sparse files for efficiency

**Example**:
```bash
# After creating a volume, check the file:
ls -lh ~/.o3k/volumes/
# -rw-r--r-- 1 user user 10G Jan 15 10:30 volume-abc-123.qcow2
```

### 3. RBD Mode (Ceph)

**Use Case**: Production multi-node clusters
**Behavior**: Stores volumes/images in Ceph RBD

```yaml
storage_mode: rbd
ceph_pool: volumes  # or images
ceph_conf: /etc/ceph/ceph.conf
```

**Requirements**:
- Ceph cluster configured
- `ceph.conf` and keyrings in place
- Network connectivity to Ceph monitors

**Characteristics**:
- Shared storage across nodes
- High availability
- Network latency (typically < 1ms in datacenter)
- Supports live migration
- Production-grade durability

**Example**:
```bash
# Check RBD images:
rbd ls volumes
# volume-abc-123
# volume-def-456

# Check image info:
rbd info volumes/volume-abc-123
# rbd image 'volume-abc-123':
#     size 10 GiB in 2560 objects
#     order 22 (4 MiB objects)
```

### 4. S3 Mode (Object Storage)

**Use Case**: Cloud deployments, cost-effective image storage
**Behavior**: Stores images in S3-compatible object storage

```yaml
storage_mode: s3
s3_bucket: my-glance-images
s3_region: us-east-1
s3_endpoint: ""  # Optional: for MinIO, Ceph RGW
```

**S3 Configuration Options**:

#### AWS S3
```yaml
glance:
  storage_mode: s3
  s3_bucket: my-glance-images
  s3_region: us-west-2
  s3_endpoint: ""  # Use AWS default
```

**Authentication**: Uses AWS SDK default credential chain:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM instance profile (on EC2)

#### MinIO (Self-Hosted)
```yaml
glance:
  storage_mode: s3
  s3_bucket: glance-images
  s3_region: us-east-1  # MinIO requires region, use any value
  s3_endpoint: http://minio.example.com:9000
```

**Environment Variables**:
```bash
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export AWS_ENDPOINT_URL=http://localhost:9000
```

#### Ceph RGW (Object Gateway)
```yaml
glance:
  storage_mode: s3
  s3_bucket: images
  s3_region: default
  s3_endpoint: http://rgw.ceph.local:7480
```

**Environment Variables**:
```bash
export AWS_ACCESS_KEY_ID=<rgw-access-key>
export AWS_SECRET_ACCESS_KEY=<rgw-secret-key>
```

**Characteristics**:
- Cost-effective for large images
- Scalable object storage
- Slower than block storage (network + object overhead)
- No live migration support (images only)
- Ideal for image archives

**Object Key Structure**:
```
s3://my-bucket/images/image-{uuid}.raw
```

**Example**:
```bash
# List images in S3 bucket:
aws s3 ls s3://my-glance-images/images/
# 2024-01-15 10:30:00 1073741824 image-abc-123.raw
# 2024-01-15 11:45:00  536870912 image-def-456.raw
```

### 5. Hybrid Modes

#### local,rbd (Local Cache + RBD)

**Use Case**: Performance-critical workloads with shared storage requirement
**Behavior**:
- **Upload**: Write to local first, then replicate to RBD
- **Download**: Read from local if available, fallback to RBD
- **Delete**: Remove from both (best effort)

```yaml
storage_mode: local,rbd
ceph_pool: volumes  # or images
ceph_conf: /etc/ceph/ceph.conf
```

**Characteristics**:
- Local speed for cached data
- RBD durability and sharing
- Automatic failover on cache miss
- Requires local disk space

**Use Case Example**:
```
Node 1: Creates volume → writes to local + RBD
Node 2: Attaches volume → reads from RBD (local cache miss)
Node 1: Re-attaches volume → reads from local (cache hit)
```

#### local,s3 (Local Cache + S3)

**Use Case**: Image service with local caching for frequently used images
**Behavior**:
- **Upload**: Write to local, asynchronously replicate to S3
- **Download**: Serve from local if available, fallback to S3
- **Delete**: Remove from both (best effort)

```yaml
storage_mode: local,s3
s3_bucket: my-glance-images
s3_region: us-east-1
```

**Characteristics**:
- Fast local access for hot images
- S3 durability for cold storage
- Automatic S3 fallback
- Reduces S3 egress costs

**Example**:
```bash
# Popular image (Ubuntu 22.04) cached locally:
$ time openstack image download ubuntu-22.04 -f value -c size
# Real: 0.5s (local cache)

# Rare image (FreeBSD 13) fetched from S3:
$ time openstack image download freebsd-13 -f value -c size
# Real: 3.2s (S3 download)
```

#### rbd,s3 (RBD Primary + S3 Fallback)

**Use Case**: Multi-site deployments with RBD in primary datacenter
**Behavior**:
- **Upload**: Write to RBD, optionally replicate to S3
- **Download**: Read from RBD, fallback to S3 if unavailable
- **Delete**: Remove from both

```yaml
storage_mode: rbd,s3
ceph_pool: images
ceph_conf: /etc/ceph/ceph.conf
s3_bucket: dr-images
s3_region: us-west-2
```

**Characteristics**:
- RBD performance in primary site
- S3 disaster recovery
- Cross-region redundancy
- Automatic failover

## Performance Comparison

### Latency (Single Image Download, 1GB)

| Mode | Latency | Throughput | Notes |
|------|---------|------------|-------|
| **stub** | < 1ms | N/A | In-memory only |
| **local** | 50-200ms | 5-10 GB/s | Local NVMe SSD |
| **rbd** | 100-500ms | 1-5 GB/s | 10GbE network, 3x replication |
| **s3** | 1-5s | 100-500 MB/s | Internet or cross-region |
| **local,rbd** | 50-200ms (hit) | 5-10 GB/s | Cache hit = local speed |
| **local,s3** | 50-200ms (hit) | 5-10 GB/s | Cache hit = local speed |

### Cinder Volume Attachment

| Mode | Attachment Time | Live Migration |
|------|-----------------|----------------|
| **stub** | Instant | No |
| **local** | < 100ms | No |
| **rbd** | 200-500ms | Yes |
| **local,rbd** | 200-500ms | Yes |

*Note: RBD attachment slower due to RADOS connection setup*

### Storage Efficiency

| Mode | Sparse Files | Deduplication | Compression |
|------|--------------|---------------|-------------|
| **stub** | N/A | N/A | N/A |
| **local** | Yes (qcow2) | No | qcow2 native |
| **rbd** | Yes (thin provisioning) | Pool-level | Pool-level |
| **s3** | No (raw objects) | No | Bucket-level |

## Deployment Scenarios

### 1. Development Workstation

**Goal**: Fast iteration, no infrastructure dependencies

```yaml
cinder:
  storage_mode: local
glance:
  storage_mode: local
```

**Characteristics**:
- Zero external dependencies
- Fast local disk I/O
- Data in `~/.o3k/`
- Single-node only

### 2. Production Single-Node

**Goal**: Production-ready single server

```yaml
cinder:
  storage_mode: local
glance:
  storage_mode: local,s3
  s3_bucket: prod-images
  s3_region: us-east-1
```

**Characteristics**:
- Cinder: Local volumes (no live migration needed)
- Glance: Local cache + S3 backup
- S3 provides disaster recovery
- Cost-effective

### 3. Production Multi-Node (Ceph)

**Goal**: High-availability cluster with shared storage

```yaml
cinder:
  storage_mode: rbd
  ceph_pool: volumes
glance:
  storage_mode: rbd
  ceph_pool: images
```

**Characteristics**:
- Live migration support
- Shared storage across all nodes
- High availability
- Requires Ceph cluster

### 4. Hybrid Cloud (Ceph + S3)

**Goal**: On-premises Ceph with cloud backup

```yaml
cinder:
  storage_mode: rbd
  ceph_pool: volumes
glance:
  storage_mode: rbd,s3
  ceph_pool: images
  s3_bucket: cloud-backup-images
  s3_region: us-east-1
```

**Characteristics**:
- Primary storage: Ceph RBD (fast, on-prem)
- Secondary: AWS S3 (disaster recovery)
- Automatic failover to S3
- Cross-region redundancy

### 5. Pure Cloud (S3-Only)

**Goal**: Fully cloud-native deployment

```yaml
cinder:
  storage_mode: local  # No S3 for block storage (volumes)
glance:
  storage_mode: s3
  s3_bucket: glance-images
  s3_region: us-west-2
```

**Characteristics**:
- Glance: S3-backed (cost-effective)
- Cinder: Local volumes (S3 unsuitable for block devices)
- No on-premises storage infrastructure
- Cloud-native

## Troubleshooting

### Ceph Connection Issues

**Symptom**: `Ceph cluster not configured` errors

**Check**:
1. Verify `ceph.conf` exists and is readable:
   ```bash
   cat /etc/ceph/ceph.conf
   ```

2. Test Ceph connectivity:
   ```bash
   ceph status
   rbd ls volumes
   ```

3. Check network connectivity to monitors:
   ```bash
   ping <mon-host>
   telnet <mon-host> 6789
   ```

**Solution**:
- Ensure `ceph.conf` has correct monitor addresses
- Verify keyring exists: `/etc/ceph/ceph.client.admin.keyring`
- Check firewall rules for Ceph traffic

### S3 Connection Issues

**Symptom**: `failed to upload to S3` or `S3 client not initialized`

**Check**:
1. Verify AWS credentials:
   ```bash
   aws s3 ls s3://my-bucket/
   ```

2. Test custom endpoint:
   ```bash
   aws --endpoint-url http://minio:9000 s3 ls
   ```

3. Check bucket permissions:
   ```bash
   aws s3api get-bucket-acl --bucket my-bucket
   ```

**Solution**:
- Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
- For custom endpoints, set `s3_endpoint` in config
- Verify bucket exists and you have write permissions

### Local Storage Permissions

**Symptom**: `failed to create volume file` or `permission denied`

**Check**:
```bash
ls -ld ~/.o3k/volumes/
ls -ld ~/.o3k/images/
```

**Solution**:
```bash
mkdir -p ~/.o3k/{volumes,images}
chmod 755 ~/.o3k/{volumes,images}
```

### Hybrid Mode Replication Failures

**Symptom**: `uploaded to local but failed to replicate to S3`

**Behavior**:
- Upload succeeds to local storage
- S3 replication fails (non-blocking)
- Image is available locally
- Warning logged

**Solution**:
- Check S3 connectivity
- Manual sync if needed:
  ```bash
  aws s3 sync ~/.o3k/images/ s3://my-bucket/images/ --exclude "*" --include "image-*.raw"
  ```

## Best Practices

### 1. Development

- Use **stub** or **local** modes
- No infrastructure required
- Fast iteration

### 2. Staging

- Use **local** mode with periodic S3 sync
- Test S3 integration separately
- Mirror production topology if possible

### 3. Production

- **Single-node**: `local` for Cinder, `local,s3` for Glance
- **Multi-node**: `rbd` for both (requires Ceph)
- **Cloud**: `local` + `s3` for Glance

### 4. Disaster Recovery

- Always use `local,s3` or `rbd,s3` for Glance
- Regularly test S3 failover
- Monitor replication lag

### 5. Cost Optimization

- Use `local` cache for frequently accessed images
- S3 Standard for active images
- S3 Glacier for archival images
- Monitor S3 egress costs

## Migration Between Modes

### From stub to local

No migration needed - stub data is ephemeral.

### From local to rbd

**Cinder Volumes**:
```bash
# For each volume:
qemu-img convert -f qcow2 -O raw \
  ~/.o3k/volumes/volume-{uuid}.qcow2 \
  rbd:volumes/volume-{uuid}
```

**Glance Images**:
```bash
# For each image:
rbd import ~/.o3k/images/image-{uuid}.raw \
  images/image-{uuid}
```

### From local to s3

**Glance Images**:
```bash
# Bulk upload:
aws s3 sync ~/.o3k/images/ s3://my-bucket/images/ \
  --exclude "*" --include "image-*.raw"
```

### From rbd to s3

**Glance Images**:
```bash
# Export from RBD, upload to S3:
for img in $(rbd ls images); do
  rbd export images/$img - | \
    aws s3 cp - s3://my-bucket/images/$img.raw
done
```

## Monitoring

### Metrics to Track

1. **Storage Usage**:
   - Local: `du -sh ~/.o3k/volumes ~/.o3k/images`
   - RBD: `rbd du volumes` / `rbd du images`
   - S3: AWS CloudWatch metrics

2. **Performance**:
   - Upload time (image create + data upload)
   - Download time (image download)
   - Attachment time (volume attach)

3. **Availability**:
   - Ceph cluster health
   - S3 API availability
   - Local disk space

4. **Costs** (S3):
   - Storage costs (GB/month)
   - Request costs (GET/PUT)
   - Egress costs (data transfer out)

### Alerting

1. **Local storage**:
   - Alert if `~/.o3k/` > 80% disk capacity

2. **Ceph**:
   - Alert on Ceph cluster warnings
   - Monitor OSD failures

3. **S3**:
   - Alert on S3 API errors
   - Monitor replication lag in hybrid modes
   - Track egress costs

## Security Considerations

### Local Mode

- Files stored with user permissions (0644)
- No encryption at rest by default
- Use full-disk encryption (LUKS) if needed

### RBD Mode

- Ceph authentication via keyrings
- In-transit encryption: Cephx protocol
- At-rest encryption: Ceph encrypted OSDs

### S3 Mode

- In-transit encryption: HTTPS (TLS 1.2+)
- At-rest encryption: S3 SSE (Server-Side Encryption)
- Access control: IAM policies, bucket policies
- Credential management: Never commit credentials to config

**Example S3 Bucket Policy**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": "arn:aws:iam::123456789012:user/o3k"},
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
      "Resource": "arn:aws:s3:::my-glance-images/*"
    }
  ]
}
```

## Summary

| Mode | Use Case | Performance | HA | Cost |
|------|----------|-------------|----|----- |
| **stub** | Development | Instant | No | Free |
| **local** | Single-node | Fast | No | Low |
| **rbd** | Multi-node | Medium | Yes | Medium |
| **s3** | Cloud/archive | Slow | Yes | Variable |
| **local,rbd** | Performance + HA | Fast (cache hit) | Yes | Medium |
| **local,s3** | Cache + cloud | Fast (cache hit) | Yes | Low-Medium |
| **rbd,s3** | Multi-site DR | Medium | Yes | Medium-High |

Choose the mode that best fits your deployment requirements, considering performance, availability, and cost constraints.
