# O3K Documentation Index

This is the master index for O3K's documentation. Start with the README at the
repo root for the project overview, then come here to find the right doc for
the task you're trying to accomplish.

> **Status**: Alpha. Suitable for evaluation, dev clusters, CI scaffolding,
> and small-scale labs. Not production-ready. See
> [production-readiness.md](production-readiness.md) for the honest grade and
> the operator pre-flight checklist.

---

## Getting started

| Doc | What it covers |
|-----|----------------|
| [QUICKSTART](QUICKSTART.md) | Get O3K running in 5 minutes (zero-config) |
| [QUICK_REFERENCE](QUICK_REFERENCE.md) | Command cheat sheet |
| [INSTALLATION](INSTALLATION.md) | Binary + Docker install paths |
| [UNIFIED_DEPLOYMENT](UNIFIED_DEPLOYMENT.md) | O3K + Horizon together via Docker Compose |

---

## Operating O3K

| Doc | What it covers |
|-----|----------------|
| [production-readiness.md](production-readiness.md) | Honest readiness grade, pre-flight checklist (7 sections) |
| [release-verification.md](release-verification.md) | Verify cosign signatures, SBOMs, container signing |
| [backup-restore-upgrade.md](backup-restore-upgrade.md) | `o3k-backup.sh` / `o3k-restore.sh`, upgrade contract |
| [tls-configuration.md](tls-configuration.md) | Native TLS or reverse-proxy patterns |
| [grafana/README.md](grafana/README.md) | Prometheus scrape config, dashboard import, alert deployment |
| [OPERATIONS](OPERATIONS.md) | Day-to-day operational tasks |
| [TROUBLESHOOTING](TROUBLESHOOTING.md) | Common issues |

---

## Configuration

| Doc | What it covers |
|-----|----------------|
| [CONFIGURATION](CONFIGURATION.md) | Full config reference |
| [STORAGE_MODES](STORAGE_MODES.md) | local / RBD / S3 / hybrid |
| [NETWORKING_MODES](NETWORKING_MODES.md) | stub / iptables / eBPF |
| [REAL_LIBVIRT_MODE](REAL_LIBVIRT_MODE.md) | Real KVM virtualization setup |
| [REAL_MODE_TESTING](REAL_MODE_TESTING.md) | Testing real-mode deployments |
| [S3_CONFIGURATION](S3_CONFIGURATION.md) | AWS S3 / MinIO setup |
| [VXLAN_IMPLEMENTATION](VXLAN_IMPLEMENTATION.md) | Multi-node VXLAN networking |
| [L3_ROUTER_IMPLEMENTATION](L3_ROUTER_IMPLEMENTATION.md) | Routers and floating IPs |

---

## Deployment

| Doc | What it covers |
|-----|----------------|
| [DEPLOYMENT](DEPLOYMENT.md) | Top-level deployment guide |
| [DOCKER_DEPLOYMENT](DOCKER_DEPLOYMENT.md) | Docker-specific |
| [SINGLE_NODE_DEPLOYMENT](SINGLE_NODE_DEPLOYMENT.md) | Single Linux host with real KVM |
| [KUBERNETES_DEPLOYMENT](KUBERNETES_DEPLOYMENT.md) | Manifests + Helm chart |
| [SCALING](SCALING.md) | Multi-node production architecture (HAProxy + Keepalived + Patroni + Ceph) |
| [MULTIARCH](MULTIARCH.md) | ARM64 + AMD64 support |
| [ci-self-hosted.md](ci-self-hosted.md) | Running self-hosted CI runners |

---

## Horizon dashboard

O3K supports Horizon for browser-based management. Login, compute, network,
and storage panes work; the long tail of Horizon-specific endpoints is still
gap-filling.

| Doc | What it covers |
|-----|----------------|
| [HORIZON_INTEGRATION](HORIZON_INTEGRATION.md) | Integration overview |
| [HORIZON_DEPLOYMENT](HORIZON_DEPLOYMENT.md) | Deploy Horizon separately to existing O3K |
| [HORIZON_SETUP](HORIZON_SETUP.md) | Configuration and troubleshooting |

---

## Architecture & API

| Doc | What it covers |
|-----|----------------|
| [ARCHITECTURE](ARCHITECTURE.md) | System design, component overview |
| [COMPONENT_STATUS](COMPONENT_STATUS.md) | Per-component real vs stub status matrix |
| [KEYSTONE_AUTH_FLOW](KEYSTONE_AUTH_FLOW.md) | JWT authentication flow |
| [API](API.md) | OpenStack API compatibility details |
| [API_COVERAGE_REPORT](API_COVERAGE_REPORT.md) | Per-service endpoint coverage and fidelity estimates |

---

## Standards alignment

| Doc | What it covers |
|-----|----------------|
| [scs-alignment.md](scs-alignment.md) | SCS standards mapping (flavors, images, volumes, federation, audit) |
| [kimi-analyse-for-completion.md](kimi-analyse-for-completion.md) | External readiness audit (source document for Phase 1–4 work) |

---

## Advanced topics

| Doc | What it covers |
|-----|----------------|
| [EBPF_STATUS](EBPF_STATUS.md) | eBPF security groups (experimental) |
| [TEST_ENVIRONMENT](TEST_ENVIRONMENT.md) | Running server/agent in test environments |
| [ELEKTRA_INTEGRATION_ANALYSIS](ELEKTRA_INTEGRATION_ANALYSIS.md) | Why O3K does not target SAP Elektra |

---

## Reviews

Audit-trail of comprehensive code reviews.

| Doc | What it covers |
|-----|----------------|
| [reviews/2026-06-04-phase4-slice4.3.md](reviews/2026-06-04-phase4-slice4.3.md) | Comprehensive review of Prometheus + Grafana slice |

---

## Contributing

| Doc | What it covers |
|-----|----------------|
| [CONTRIBUTING](CONTRIBUTING.md) | Development guidelines and contribution process |

For additional help:
- GitHub Issues: <https://github.com/cobaltcore-dev/o3k/issues>
- GitHub Discussions: <https://github.com/cobaltcore-dev/o3k/discussions>
- Security advisories: see [SECURITY.md](../SECURITY.md)
