# Example S3 Configuration for Glance

This directory contains example configurations for different S3 backends.

## AWS S3

```yaml
glance:
  port: 9292
  storage_mode: s3
  s3_bucket: my-glance-images
  s3_region: us-east-1
  s3_endpoint: ""  # Leave empty for AWS S3
```

**Environment Variables**:
```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

## MinIO (Local S3-Compatible Storage)

```yaml
glance:
  port: 9292
  storage_mode: s3
  s3_bucket: glance-images
  s3_region: us-east-1  # MinIO requires a region
  s3_endpoint: http://localhost:9000
```

**Environment Variables**:
```bash
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
```

**Start MinIO**:
```bash
# Using Docker:
docker run -d -p 9000:9000 -p 9001:9001 \
  -e "MINIO_ROOT_USER=minioadmin" \
  -e "MINIO_ROOT_PASSWORD=minioadmin" \
  -v /data/minio:/data \
  minio/minio server /data --console-address ":9001"

# Create bucket:
mc alias set local http://localhost:9000 minioadmin minioadmin
mc mb local/glance-images
```

## Ceph RGW (Rados Gateway)

```yaml
glance:
  port: 9292
  storage_mode: s3
  s3_bucket: images
  s3_region: default
  s3_endpoint: http://rgw.ceph.local:7480
```

**Environment Variables**:
```bash
export AWS_ACCESS_KEY_ID=<your-rgw-access-key>
export AWS_SECRET_ACCESS_KEY=<your-rgw-secret-key>
```

**Create RGW User**:
```bash
radosgw-admin user create --uid=o3k --display-name="O3K Glance"
# Note the access_key and secret_key from output
```

## Hybrid Modes

### Local Cache + S3 Backup

Best for: Frequently accessed images with cloud backup

```yaml
glance:
  port: 9292
  storage_mode: local,s3
  s3_bucket: glance-backup
  s3_region: us-west-2
  s3_endpoint: ""
```

**Behavior**:
- Upload: Write to `~/.o3k/images/` then replicate to S3
- Download: Serve from local if available, fallback to S3
- Delete: Remove from both locations

### RBD + S3 Disaster Recovery

Best for: Multi-site deployments with cross-region backup

```yaml
glance:
  port: 9292
  ceph_pool: images
  ceph_conf: /etc/ceph/ceph.conf
  storage_mode: rbd,s3
  s3_bucket: dr-images
  s3_region: us-east-2
  s3_endpoint: ""
```

**Behavior**:
- Upload: Write to RBD, optionally replicate to S3
- Download: Serve from RBD, fallback to S3 if unavailable
- Delete: Remove from both locations

## Testing S3 Integration

### 1. Upload Test Image

```bash
# Create test image:
dd if=/dev/urandom of=test-image.raw bs=1M count=100

# Upload via OpenStack CLI:
openstack image create --disk-format raw --file test-image.raw test-image
```

### 2. Verify in S3

```bash
# List S3 objects:
aws s3 ls s3://my-glance-images/images/

# Get object metadata:
aws s3api head-object --bucket my-glance-images --key images/image-{uuid}.raw
```

### 3. Download Test

```bash
# Download via OpenStack CLI:
openstack image save test-image --file downloaded-image.raw

# Verify checksum:
md5sum test-image.raw downloaded-image.raw
```

### 4. Performance Test

```bash
# Measure upload time:
time openstack image create --disk-format raw --file large-image.raw large-test

# Measure download time:
time openstack image save large-test --file /dev/null
```

## Troubleshooting

### S3 Connection Failed

**Error**: `failed to upload to S3: RequestError: send request failed`

**Solution**:
1. Verify endpoint is reachable:
   ```bash
   curl http://minio:9000
   ```

2. Check credentials:
   ```bash
   aws --endpoint-url http://minio:9000 s3 ls
   ```

3. Enable SDK debug logging:
   ```bash
   export AWS_SDK_LOG_LEVEL=debug
   ```

### Access Denied

**Error**: `failed to upload to S3: AccessDenied`

**Solution**:
1. Verify IAM permissions (AWS S3)
2. Check bucket policy
3. For MinIO/RGW, verify user has write permissions

### Bucket Not Found

**Error**: `NoSuchBucket: The specified bucket does not exist`

**Solution**:
```bash
# AWS S3:
aws s3 mb s3://my-glance-images --region us-east-1

# MinIO:
mc mb local/glance-images

# Ceph RGW:
aws --endpoint-url http://rgw:7480 s3 mb s3://images
```

## Performance Tuning

### S3 Transfer Acceleration (AWS Only)

Enable for faster uploads/downloads:

```bash
aws s3api put-bucket-accelerate-configuration \
  --bucket my-glance-images \
  --accelerate-configuration Status=Enabled
```

Update endpoint in config:
```yaml
s3_endpoint: my-glance-images.s3-accelerate.amazonaws.com
```

### Multipart Uploads

For images > 5GB, O3K automatically uses multipart uploads via AWS SDK.

### Connection Pooling

AWS SDK v2 automatically pools HTTP connections. No configuration needed.

## Cost Optimization

### S3 Storage Classes

For infrequently accessed images, use lifecycle policies:

```bash
aws s3api put-bucket-lifecycle-configuration \
  --bucket my-glance-images \
  --lifecycle-configuration file://lifecycle.json
```

**lifecycle.json**:
```json
{
  "Rules": [
    {
      "Id": "Move old images to Glacier",
      "Status": "Enabled",
      "Prefix": "images/",
      "Transitions": [
        {
          "Days": 90,
          "StorageClass": "GLACIER"
        }
      ]
    }
  ]
}
```

### Monitor Costs

```bash
# Check storage usage:
aws s3 ls s3://my-glance-images/images/ --recursive --summarize

# Monitor with CloudWatch:
aws cloudwatch get-metric-statistics \
  --namespace AWS/S3 \
  --metric-name BucketSizeBytes \
  --dimensions Name=BucketName,Value=my-glance-images \
  --start-time 2024-01-01T00:00:00Z \
  --end-time 2024-01-31T23:59:59Z \
  --period 86400 \
  --statistics Average
```

## Security Best Practices

1. **Never commit credentials**:
   - Use environment variables
   - Use IAM roles (EC2/ECS)
   - Use credential files with restricted permissions

2. **Enable encryption**:
   ```bash
   # Enable bucket encryption:
   aws s3api put-bucket-encryption \
     --bucket my-glance-images \
     --server-side-encryption-configuration \
     '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
   ```

3. **Restrict bucket access**:
   - Use bucket policies
   - Enable VPC endpoints (AWS)
   - Use private subnets

4. **Enable logging**:
   ```bash
   # Enable S3 access logging:
   aws s3api put-bucket-logging \
     --bucket my-glance-images \
     --bucket-logging-status file://logging.json
   ```

## Production Checklist

- [ ] S3 bucket created with appropriate region
- [ ] Credentials configured (environment variables or IAM role)
- [ ] Bucket encryption enabled
- [ ] Access logging enabled
- [ ] Lifecycle policies configured (for cost optimization)
- [ ] Monitoring/alerting configured (CloudWatch or equivalent)
- [ ] Tested upload/download operations
- [ ] Verified hybrid mode failover (if using local,s3 or rbd,s3)
- [ ] Documented disaster recovery procedures
- [ ] Cost estimates reviewed
