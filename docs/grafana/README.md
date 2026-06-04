# O3K Grafana Monitoring

Dashboard and alerting rules for the O3K observability stack.

## Files

| File | Purpose |
|------|---------|
| `o3k-overview.json` | Grafana dashboard — request rate, error rate, P99 latency per service |
| `o3k-alerts.yaml` | Prometheus alerting rules |

## Requirements

The `/metrics` endpoint is always exposed on every O3K service port,
regardless of `libvirt_mode`, `networking_mode`, or any other service-mode
setting. Stub mode and real mode both emit metrics — the only difference is
which workloads drive traffic through the middleware. Verify metrics are
available before importing the dashboard:

```bash
curl -s http://localhost:8774/metrics | grep o3k_http
```

---

## Import the Grafana dashboard

1. Open Grafana and click the **+** icon in the left sidebar.
2. Select **Import**.
3. Click **Upload JSON file** and select `o3k-overview.json`.
4. On the next screen, choose your Prometheus data source from the
   **Prometheus** dropdown.
5. Click **Import**.

The dashboard opens at `Dashboards / O3K — Service Overview`.

The dashboard ships with a `$datasource` variable so you can pick which
Prometheus instance backs the panels. Every service row (Keystone, Nova,
Neutron, Cinder, Glance, Placement, Metadata) is always visible; scroll to
the row you care about.

---

## Load the alerting rules into Prometheus

Copy the rule file to Prometheus's rules directory and reload:

```bash
sudo cp o3k-alerts.yaml /etc/prometheus/rules/o3k-alerts.yaml

# Reload without restart (requires --web.enable-lifecycle flag on Prometheus)
curl -X POST http://localhost:9090/-/reload

# Or restart the service
sudo systemctl restart prometheus
```

Verify the rules are loaded:

```bash
curl -s http://localhost:9090/api/v1/rules | jq '.data.groups[].name'
```

You should see `"o3k.service"` in the output.

---

## Prometheus scrape configuration

Add the following job blocks to your `prometheus.yml`. Each O3K service
exposes `/metrics` on its own port.

```yaml
scrape_configs:
  - job_name: o3k-keystone
    static_configs:
      - targets: ["localhost:35357"]

  - job_name: o3k-nova
    static_configs:
      - targets: ["localhost:8774"]

  - job_name: o3k-neutron
    static_configs:
      - targets: ["localhost:9696"]

  - job_name: o3k-cinder
    static_configs:
      - targets: ["localhost:8776"]

  - job_name: o3k-glance
    static_configs:
      - targets: ["localhost:9292"]

  - job_name: o3k-placement
    static_configs:
      - targets: ["localhost:8778"]

  - job_name: o3k-metadata
    static_configs:
      - targets: ["localhost:8775"]
```

Replace `localhost` with the actual host if Prometheus runs on a separate
machine. The `O3KServiceDown` alert matches jobs with the prefix `o3k-`, so
keep that naming convention.

---

## Dashboard panels

Each service row contains three panels:

**Request Rate** — `rate(o3k_http_requests_total{service="..."}[5m])` broken
down by HTTP method and status code. A sudden drop to zero means the service
stopped receiving traffic (or stopped responding).

**Error Rate %** — percentage of requests returning 5xx. Threshold lines at 1%
(yellow) and 5% (red). Sustained values above 1% warrant investigation; above
5% triggers the `O3KHighErrorRate` alert.

**P99 Latency** — 99th-percentile response time from the histogram. Threshold
lines at 1s (yellow) and 10s (red). Sustained values above 2s trigger
`O3KHighLatencyP99`; above 10s trigger `O3KHighLatencyP99Critical`.

---

## Alert reference

| Alert | Severity | Condition | Meaning |
|-------|----------|-----------|---------|
| `O3KHighErrorRate` | warning | 5xx rate > 5% for 5m | Service is returning errors to clients |
| `O3KServiceDown` | critical | scrape target missing for 2m | Process crashed or port unreachable |
| `O3KHighLatencyP99` | warning | p99 > 2s for 5m | Clients experiencing slow responses |
| `O3KHighLatencyP99Critical` | critical | p99 > 10s for 2m | Client timeouts likely occurring |

All alerts carry a `runbook` annotation pointing to `docs/TROUBLESHOOTING.md`
and a `dashboard` annotation linking back to the Grafana overview. Update
those URLs to match your environment before deploying.
