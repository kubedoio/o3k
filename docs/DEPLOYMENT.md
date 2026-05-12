# O3K Deployment Guide

Covers the main deployment patterns as of v0.7.1.

---

## 1. Single Binary (Zero Config)

The simplest deployment. No config file required.

```bash
# Download the binary
curl -L https://github.com/cobaltcore-dev/o3kio/releases/latest/download/o3k -o o3k
chmod +x o3k

# Run
./o3k
```

What you get out of the box:

- SQLite database at `./o3k.db`
- All services in stub mode (no external dependencies)
- JWT secret auto-generated at startup and printed once
- Agent join token auto-generated and printed once
- TLS auto-generated for the gRPC tunnel
- Health, metrics, and tracing enabled

Default ports:

| Service   | Port  |
|-----------|-------|
| Keystone  | 35357 |
| Nova      | 8774  |
| Neutron   | 9696  |
| Cinder    | 8776  |
| Glance    | 9292  |
| Metadata  | 8775  |

Change the base port with `--port`:

```bash
./o3k --port 5000  # Keystone: 5357, Nova: 5774, Neutron: 5696, ...
```

Default bind address is `127.0.0.1`. Override with `--host 0.0.0.0` to listen on all interfaces.

---

## 2. PostgreSQL Mode

For persistent state or multi-node deployments.

```bash
# Inline connection string
./o3k --datastore postgres --db-url "postgres://o3k:secret@localhost:5432/o3k?sslmode=disable"

# Via environment variable (preferred for production)
export O3K_DB_URL="postgres://o3k:secret@localhost:5432/o3k?sslmode=disable"
./o3k --datastore postgres
```

Migrations run automatically at startup.

### Docker Compose (PostgreSQL + O3K)

```bash
cd deployments/
docker compose up -d
```

Services started: PostgreSQL 17 + O3K with all five OpenStack services.

### Docker Compose (with Horizon)

```bash
cd deployments/
docker compose -f docker-compose-horizon.yml up -d
```

Horizon available at `http://localhost/dashboard`. Default credentials: `admin` / `secret`.

---

## 3. Environment Variables Reference

All sensitive settings can be provided via environment variables. Environment variables override values in the config file.

| Variable            | Purpose                                      | Required in Production |
|---------------------|----------------------------------------------|------------------------|
| `O3K_JWT_SECRET`    | HMAC-SHA256 signing key for tokens           | Yes (min 32 chars)     |
| `O3K_DB_URL`        | PostgreSQL connection string                 | If using Postgres       |
| `O3K_OTEL_ENDPOINT` | OpenTelemetry OTLP collector endpoint        | No (stdout fallback)   |

`O3K_JWT_SECRET` is rejected at startup if shorter than 32 characters. The binary will not start.

---

## 4. Health Checks (Kubernetes)

All five services expose health endpoints on their respective ports.

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 35357   # Keystone port; repeat for each service
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 35357
  initialDelaySeconds: 5
  periodSeconds: 5
```

- `/healthz` — returns 200 if the process is alive
- `/readyz` — returns 200 when the service has completed startup and is ready to serve traffic

For a complete Kubernetes deployment example see [KUBERNETES_DEPLOYMENT.md](KUBERNETES_DEPLOYMENT.md).

---

## 5. Observability

### Metrics

Each service exposes Prometheus-format metrics at `/metrics` on its own port.

```bash
# Example: Keystone metrics
curl http://localhost:35357/metrics
```

Key metrics exposed:

- `o3k_http_requests_total` — request counter by service, method, path, status
- `o3k_http_request_duration_seconds` — latency histogram
- `o3k_reconciler_runs_total` — scheduler reconciler activity

Scrape all five ports in your Prometheus config:

```yaml
scrape_configs:
  - job_name: o3k
    static_configs:
      - targets:
          - localhost:35357  # keystone
          - localhost:8774   # nova
          - localhost:9696   # neutron
          - localhost:8776   # cinder
          - localhost:9292   # glance
    metrics_path: /metrics
```

### Request Tracing

Every response includes:

- `X-Request-Id` — unique UUID per request
- `X-OpenStack-Request-Id` — same value in OpenStack format (`req-<uuid>`)

### OpenTelemetry

Tracing is enabled by default with a stdout exporter. To send traces to a collector:

```bash
export O3K_OTEL_ENDPOINT="http://otel-collector:4318"
./o3k
```

The OTLP exporter uses the HTTP/protobuf format. If the endpoint is unreachable, tracing falls back to stdout silently.

---

## 6. Security Hardening Checklist

Before exposing O3K to a network:

- [ ] Set `O3K_JWT_SECRET` to a random string of at least 32 characters
  ```bash
  export O3K_JWT_SECRET=$(openssl rand -hex 32)
  ```
- [ ] Change the seed `admin` password in the database or via the Keystone API after first boot
- [ ] Bind to `127.0.0.1` or a private interface; put a reverse proxy (nginx, Caddy) in front for external access
- [ ] Enable TLS on your reverse proxy; do not expose plain HTTP token endpoints externally
- [ ] Set `server.cors_allowed_origins` in `config/o3k.yaml` to your actual Horizon origin (not `*`)
- [ ] Review rate limiting: token creation is limited to 10 req/min per IP by default
- [ ] For multi-node deployments: rotate the gRPC tunnel join token (`o3k token --node-id <id>`) and keep it out of version control

### Known Hardcoded Defaults (document before production)

| Item | Default | Action |
|------|---------|--------|
| Admin user/password | `admin` / `secret` | Change via Keystone API after first boot |
| JWT secret (zero-config) | Auto-generated, printed at startup | Persist via `O3K_JWT_SECRET` |
| SQLite path (zero-config) | `./o3k.db` | Use `--db-path` or switch to PostgreSQL |

---

## Related Docs

- [Configuration Reference](CONFIGURATION.md)
- [Kubernetes Deployment](KUBERNETES_DEPLOYMENT.md)
- [Operations Guide](OPERATIONS.md)
- [Networking Modes](NETWORKING_MODES.md)
- [Storage Modes](STORAGE_MODES.md)
- [Troubleshooting](TROUBLESHOOTING.md)
