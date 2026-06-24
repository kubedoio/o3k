# O3K Security Model

## Why O3K runs as root

O3K requires root privileges for the following reasons:

1. **libvirt socket access** — the `qemu:///system` URI connects to `/run/libvirt/libvirt-sock`, which is owned by root (or the `libvirt` group on some distros). Even with group membership, the runner process needs to interact with kernel subsystems that require elevated privileges.
2. **KVM device access** — `/dev/kvm` requires `kvm` group membership at minimum; some operations require root.
3. **Network namespace management** — future iptables-based networking mode requires CAP_NET_ADMIN and CAP_NET_RAW.

This is the same model used by k3s, MicroK8s, and every other single-binary hypervisor manager. **Treat the o3k host as you would any bare-metal hypervisor node.**

## Mitigations in v0.2.0

The systemd unit includes the following hardening directives:

| Directive | Effect |
|-----------|--------|
| `NoNewPrivileges=true` | The o3k process and its children cannot gain new privileges via setuid/setgid |
| `PrivateTmp=true` | `/tmp` and `/var/tmp` are private to the service, preventing tmp-based attacks |
| `Restart=on-failure` | Service restarts only on unexpected exit, not on clean shutdown |
| `Requires=libvirtd.service` | Service will not start if libvirt is not running |

The JWT secret is:
- Generated with `openssl rand -base64 48` (384 bits of entropy)
- Written to `/etc/o3k/config.yaml` with mode `0600` (root read-only)
- Never placed in environment variables or systemd unit files

## Network exposure

By default, all O3K ports bind to `0.0.0.0`:

| Port | Service |
|------|---------|
| 35357 | Keystone (identity, token issuance) |
| 8774 | Nova (compute) |
| 8775 | Metadata (no auth — EC2-compatible) |
| 8776 | Cinder (block storage) |
| 8778 | Placement |
| 9292 | Glance (image) |
| 9696 | Neutron (network) |

**Recommended firewall rules** — allow only from trusted management subnets:

```bash
# Allow Keystone from management subnet only
ufw allow from 10.0.0.0/24 to any port 35357
# Block all other OpenStack ports from public interfaces
ufw deny 8774 8775 8776 8778 9292 9696
```

The metadata port (8775) has no authentication — restrict it to the local machine if you are not using EC2-compatible metadata:

```bash
ufw deny in on eth0 to any port 8775
```

## Known limitations in v0.2.0

- No TLS on any port — all traffic is plaintext. Use a reverse proxy (nginx/caddy) or VPN for remote access.
- Root execution — no fine-grained capability dropping yet.
- No audit logging — API access is logged but not in an audit-trail format.

## Roadmap (v0.3)

- Dedicated `o3k` system user with fine-grained Linux capabilities (`CAP_NET_ADMIN`, `CAP_NET_RAW`)
- `systemd-creds` for encrypted secret storage (Ubuntu 22.04+)
- Optional TLS on Keystone and Nova ports
- Bind-address configuration (default `127.0.0.1` instead of `0.0.0.0`)
