# O3K TLS / HTTPS Configuration

O3K does not require TLS to run, but **production deployments MUST terminate TLS** somewhere — either at the O3K processes themselves or at an upstream reverse proxy.

This document covers both approaches.

## Option A: Native HTTPS (recommended for single-node deployments)

All seven HTTP services (Keystone, Nova, Neutron, Cinder, Glance, Placement, Metadata) can be configured to listen with TLS by providing a single PEM-encoded certificate and key.

### Via CLI flags

```bash
o3k --tls-cert-file /etc/o3k/tls/server.crt \
    --tls-key-file  /etc/o3k/tls/server.key
```

### Via config file (`config/o3k.yaml`)

```yaml
server:
  tls_cert_file: /etc/o3k/tls/server.crt
  tls_key_file:  /etc/o3k/tls/server.key
```

If only one of cert/key is set, O3K refuses to start.

### Generating a self-signed certificate (lab only)

```bash
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout server.key -out server.crt \
  -days 365 \
  -subj "/CN=o3k.example.com" \
  -addext "subjectAltName=DNS:o3k.example.com,DNS:localhost,IP:127.0.0.1"
```

### Service-catalog endpoints

When TLS is enabled, Keystone advertises `https://` URLs in the service catalog. Set `O3K_ENDPOINT_HOST` (or the equivalent in `config/o3k.yaml`) to the externally-visible hostname so clients receive the right URL.

## Option B: Reverse proxy (recommended for multi-node / load-balanced deployments)

Run O3K with plain HTTP, bound to localhost, and terminate TLS at a fronting nginx / Caddy / HAProxy / cloud load balancer.

### Required hardening

When using a reverse proxy, you MUST:

1. Bind O3K to **127.0.0.1** (not 0.0.0.0):
   ```yaml
   server:
     bind_host: 127.0.0.1
   ```
2. Set **HSTS** at the proxy with `max-age >= 31536000; includeSubDomains`.
3. Strip and re-set `X-Forwarded-*` headers at the proxy (do not trust client values).
4. Set `O3K_ENDPOINT_HOST` to the proxy's externally-visible hostname.

### Example: nginx

```nginx
server {
    listen 443 ssl http2;
    server_name o3k.example.com;

    ssl_certificate     /etc/letsencrypt/live/o3k.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/o3k.example.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options nosniff always;

    # Keystone
    location /v3/ {
        proxy_pass http://127.0.0.1:35357;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }

    # Nova / Neutron / Cinder / Glance / Placement: configure each on
    # subpaths or separate vhosts as needed.
}

# Optional plain-HTTP redirect
server {
    listen 80;
    server_name o3k.example.com;
    return 301 https://$server_name$request_uri;
}
```

### Example: Caddy (auto-TLS via Let's Encrypt)

```
o3k.example.com {
    reverse_proxy /v3/* 127.0.0.1:35357
    reverse_proxy /v2.1/* 127.0.0.1:8774
    reverse_proxy /v2.0/* 127.0.0.1:9696
    # ...

    header Strict-Transport-Security "max-age=31536000; includeSubDomains"
}
```

## What still goes plaintext

The following do NOT yet have native TLS support and require a reverse proxy or trusted network:

- **Tunnel hub** (gRPC, port 6443) — has its own TLS plumbing via `tunnel.tls_cert_file` / `tunnel.tls_key_file`. See `internal/tunnel/`.
- **Metadata service** (port 8775) — runs on the management network and historically does not use TLS in OpenStack. Bind to localhost or a private interface.

## Verifying TLS

```bash
# Check that the cert is being served
openssl s_client -connect o3k.example.com:35357 -servername o3k.example.com < /dev/null

# Check HSTS / cipher / protocols
curl -vI https://o3k.example.com:35357/v3 2>&1 | grep -iE "(strict|server|http/)"
```
