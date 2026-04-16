# Design: o3k Server/Agent Scaling Architecture

**Date**: 2026-04-10
**Status**: Approved
**Author**: senol.colak
**Version**: 1.0.0

---

## Goal

Scale o3k from a single-node binary to a distributed system where one `o3k server`
command runs the control plane and multiple `o3k agent` commands run compute workers —
identical philosophy to k3s.

---

## Background

o3k currently runs all five OpenStack services in a single process on a single node.
Multi-node scaling today requires manual HAProxy + Ceph + keepalived setup (ops-heavy,
no native clustering). The k3s model — single binary, two subcommands, join token,
gRPC back-channel — is the right template.

The existing `compute_nodes` table and `NodeRegistry` heartbeat code prove the
foundation is already there. This design replaces the DB-polling heartbeat loop
with a live gRPC HeartbeatStream signal. The `compute_nodes` table is retained as
the atomic resource reservation store for the scheduler.

**Note**: The existing `NodeRegistry.NewNodeRegistry()` regenerates the UUID on
every call. The agent must be changed to persist the generated UUID to
`/var/lib/o3k/agent/node-id` and reload it on restart.

---

## Architecture Principle Trade-off

The existing core principle "Synchronous Operations: No async state machines —
operations complete before API returns" cannot hold when compute operations execute
on remote agents over a network. A synchronous `POST /servers` that blocks for 30s
on a remote libvirt call violates fail-fast, exhausts HTTP connections, and breaks
under load balancer timeouts.

This design introduces an async task queue for operations dispatched to remote agents.
The tunnel itself remains synchronous and fail-fast: a dead agent produces an
immediate error in the worker. The API layer returns 202 Accepted (which is already
the real OpenStack API contract — Terraform and gophercloud expect it).

**This supersedes the synchronous-operations principle for multi-node compute and
network operations only.** Single-node deployments (`o3k` with no subcommand) retain
synchronous behavior. Keystone, Glance metadata, Cinder (non-compute), and Placement
remain synchronous.

When `docs/ARCHITECTURE.md` is updated for this feature, the "Design Philosophy"
section must note this exception.

---

## Scope

**In scope**: binary interface, join protocol, gRPC tunnel, async task queue,
scheduler with atomic reservation, agent local state, mTLS, HA topology, test
strategy, observability.

**Out of scope**: live migration, VM evacuation on node failure, Ceph deployment
automation, Horizon integration changes.

---

## Design

### 1. Binary Interface

The `o3k` binary gains two top-level subcommands. Everything else stays as-is.

```
o3k server [--config path]
o3k agent  --server https://<host>:6385 --token-file /etc/o3k/token [--node-id <id>]
```

**Token input** (priority order):
1. `--token-file /path/to/file` (preferred — no process list exposure)
2. `O3K_TOKEN` environment variable
3. `--token <value>` (dev/testing only — emits warning: "token visible in process list")

`o3k server` starts all existing services (Keystone, Nova, Neutron, Cinder, Glance,
Placement, Metadata) plus the TunnelHub gRPC server on `:6385`.

`o3k agent` starts only:
- gRPC client -> dials TunnelHub, authenticates, enters task loop
- Local libvirt executor (VM lifecycle)
- Local netlink/VXLAN executor (network namespaces, bridges, security groups)

No OpenStack API ports open on agent nodes.

**Backward compatibility**: running `o3k` with no subcommand defaults to `o3k server`
behaviour so existing deployments are unaffected. Subcommand dispatch uses
`flag.NewFlagSet` per subcommand to avoid breaking existing `--config` flag parsing:

```go
func main() {
    if len(os.Args) < 2 || !isSubcommand(os.Args[1]) {
        runServer(os.Args[1:])  // backward compat
        return
    }
    switch os.Args[1] {
    case "server":
        runServer(os.Args[2:])
    case "agent":
        runAgent(os.Args[2:])
    case "token":
        runTokenCmd(os.Args[2:])
    }
}
```

---

### 2. Join Token and mTLS

Join flow mirrors k3s, with added security hardening.

```bash
# On server node (token auto-generated at first start)
o3k server
o3k token get                     # requires root or o3k service user

# On agent node
o3k agent --server https://10.0.0.1:6385 --token-file /etc/o3k/token
```

**Token format**: `O3K<version>:<cluster-id>:<HMAC-SHA256(node-password, cluster-secret)>`

The `O3K` prefix distinguishes from k3s tokens. Version field enables future rotation.

**Cluster-secret requirements**:
- Generated with 256 bits of CSPRNG entropy at first server start
- Stored at `/var/lib/o3k/server/node-token` with mode `0600`, owned by o3k service user
- `o3k token get` requires root or o3k service user
- `o3k token rotate` generates a new secret with a configurable grace period
  (both old and new secrets validate during grace window, then old is invalidated)

**Join endpoint rate-limiting**: max 5 attempts per source IP per minute. Failed
verifications increment a per-IP counter; after 10 failures, IP blocked for 5 minutes.

**mTLS flow**:
1. Agent presents token on initial gRPC connection metadata.
2. Server verifies token, checks `node-id` -> `public-key-fingerprint` binding in
   `compute_nodes` table (if first join, creates binding; if existing, validates).
3. Server issues a short-lived TLS client certificate (90-day expiry) signed by
   the cluster CA: `CN=<node-id>, O=o3k-agents`.
4. Agent stores cert at `/var/lib/o3k/agent/client.crt`.
5. All subsequent gRPC connections use mTLS — no bearer token on the wire.
6. Server validates `O=o3k-agents` on every connection.

**Certificate lifecycle**:
- Expiry: 90 days. Agent requests renewal before expiry via a `CertRenew` RPC.
- Revocation: `revoked_agent_certs` table in PostgreSQL (`serial_number`, `node_id`,
  `revoked_at`). TunnelHub checks this table on every new connection.
- `o3k node deregister <node-id>` writes to revocation table and marks node `down`.
- Agent attempting to join with a revoked cert gets connection refused.

**Node identity**: UUID auto-generated on first run, persisted to
`/var/lib/o3k/agent/node-id`. On HELLO, server checks for existing rows matching
the agent's `hostname` even if `node_id` differs (handles lost node-id file).

**CA key distribution for HA**: The cluster CA private key lives at
`/var/lib/o3k/server/ca.key`, generated once on the first server node, and must
be distributed to all server nodes via operator tooling (Vault, k8s Secret, or
manual `scp`). All server nodes must share the same CA to accept each other's
agent certificates.

---

### 3. gRPC Tunnel — Three Independent Streams

Single HTTP/2 connection, three logical gRPC streams. Head-of-line blocking
on one stream does not affect the others.

```protobuf
service AgentTunnel {
  // One-shot registration: agent identifies itself, server acknowledges
  rpc Register(Hello) returns (HelloAck) {}

  // Bidirectional: server sends Tasks, agent replies with TaskResults
  rpc TaskStream(stream TaskResult) returns (stream Task) {}

  // Bidirectional: agent reports stats, server acknowledges per-message
  rpc StatsStream(stream AgentStats) returns (stream StatsAck) {}

  // Bidirectional: 5s ping/pong, liveness only
  rpc HeartbeatStream(stream Heartbeat) returns (stream HeartbeatAck) {}

  // Certificate renewal
  rpc CertRenew(CertRenewRequest) returns (CertRenewResponse) {}
}

message Hello {
  string node_id   = 1;
  string hostname  = 2;
  repeated string cached_images = 3;
  repeated OrphanReport orphans = 4;
}

message HelloAck {
  string cluster_id  = 1;
  string server_id   = 2;
  bytes  server_nonce = 3;  // single-use nonce for TaskResult authentication
}

message OrphanReport {
  string task_id      = 1;
  string task_type    = 2;
  bytes  result       = 3;
  string completed_at = 4;
}

enum TaskType {
  TASK_TYPE_UNSPECIFIED       = 0;
  VM_CREATE                   = 1;
  VM_DELETE                   = 2;
  VM_START                    = 3;
  VM_STOP                     = 4;
  VM_REBOOT                   = 5;
  VM_GET_STATE                = 6;
  NET_ENSURE_NAMESPACE        = 7;
  NET_DELETE_NAMESPACE         = 8;
  NET_ADD_PORT                = 9;
  NET_REMOVE_PORT             = 10;
  NET_APPLY_SECURITY_GROUP    = 11;
  NET_REMOVE_SECURITY_GROUP   = 12;
  VXLAN_ADD_PEER              = 13;
  VXLAN_REMOVE_PEER           = 14;
  IMAGE_PREFETCH              = 15;
}

message Task {
  string                     id      = 1;
  TaskType                   type    = 2;
  bytes                      payload = 3;  // validated against type before dispatch
  google.protobuf.Duration   timeout = 4;
  int64                      max_payload_bytes = 5;  // enforced: default 64KB
}

message TaskResult {
  string    id         = 1;
  bytes     data       = 2;  // populated on success
  string    error      = 3;  // empty = success, non-empty = failure
  ErrorCode code       = 4;
  bytes     result_mac = 5;  // HMAC(server_nonce + task_id + data/error, agent_key)
}

enum ErrorCode {
  ERROR_NONE      = 0;
  ERROR_TRANSIENT = 1;  // retry with backoff
  ERROR_PERMANENT = 2;  // skip retries, go to failed immediately
  ERROR_TIMEOUT   = 3;  // counted as transient
}

message AgentStats {
  string   node_id       = 1;
  int64    vcpu_total    = 2;  // physical capacity (stable)
  int64    ram_mb_total  = 3;
  int64    disk_gb_total = 4;
  repeated string cached_images = 5;
}

message StatsAck {
  string node_id    = 1;
  int64  server_seq = 2;
}
```

**Task payload validation**: Server validates payload against `TaskType` before
writing to the `tasks` table. Invalid payloads are rejected with `ERROR_PERMANENT`.
Maximum payload size: 64KB (configurable). Typed payload structs for each TaskType
are defined in a companion file (`proto/payloads.proto`).

**Liveness**: HeartbeatStream drops -> server marks agent `offline`. Agent `offline`
status suppresses new dispatch only. In-flight `TaskResult` messages from offline
agents MUST be accepted and processed. Task requeueing happens only after
`task.timeout` expires with no result received.

**Server-side per-agent concurrency**: TunnelHub enforces max in-flight tasks per
agent (default: 1 for v1). Dispatch blocks when the agent's semaphore is full;
worker skips that agent (task stays `pending`).

```go
type agentConn struct {
    stream   AgentTunnel_TaskStreamServer
    inflight atomic.Int32  // max 1 for v1
}
```

---

### 4. Async Task Queue — API Contract

Nova/Neutron APIs return **202 Accepted** immediately. Client polls for status.

**Note**: Cinder is not modified in v1 — volume operations remain synchronous.
Only Nova and Neutron operations that execute on remote agents use the task queue.

```
POST /v2/{project}/servers
-> 202 Accepted
  {
    "server": {
      "id": "uuid",
      "status": "BUILD",
      "OS-EXT-STS:task_state": "scheduling"
    }
  }

GET /v2/{project}/servers/{id}
-> 200 OK  { "status": "ACTIVE" }    <- poll until this
```

**Note**: `adminPass` is returned only in the 202 response to `POST /servers`. It
is not persisted and will not appear in subsequent GET responses. This matches
OpenStack Nova behavior.

Nova handlers extract `X-Idempotency-Key` from the request header and pass it to
the task insert. If absent, `idempotency_key` is NULL (PostgreSQL allows multiple
NULLs in a UNIQUE column). Duplicate `X-Idempotency-Key` returns 202 with the
existing task's resource_id (not 409, not a new task).

**Task lifecycle in DB**:

```
pending -> dispatched -> completed
               |
               +-> pending (retries < 3, with next_retry_at delay, agent_id cleared)
               +-> failed  (retries exhausted, terminal)
```

New `tasks` table:

```sql
CREATE TABLE tasks (
  id              UUID PRIMARY KEY,
  type            TEXT NOT NULL,
  resource_id     UUID NOT NULL,
  project_id      UUID NOT NULL,
  agent_id        UUID REFERENCES compute_nodes(id) ON DELETE SET NULL,
  payload         JSONB NOT NULL,
  status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'dispatched', 'completed', 'failed')),
  retries         INT NOT NULL DEFAULT 0 CHECK (retries <= 3),
  timeout_sec     INT NOT NULL,
  next_retry_at   TIMESTAMPTZ,
  idempotency_key TEXT,
  error           TEXT,
  error_history   JSONB NOT NULL DEFAULT '[]',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  dispatched_at   TIMESTAMPTZ,
  completed_at    TIMESTAMPTZ,

  CONSTRAINT chk_agent_only_when_dispatched
    CHECK (agent_id IS NULL OR status IN ('dispatched', 'completed', 'failed')),
  CONSTRAINT chk_dispatched_has_timestamp
    CHECK (status != 'dispatched' OR dispatched_at IS NOT NULL),
  CONSTRAINT chk_completed_has_timestamp
    CHECK (status != 'completed' OR completed_at IS NOT NULL),
  CONSTRAINT uq_idempotency_per_project
    UNIQUE (project_id, idempotency_key)
);

CREATE INDEX idx_tasks_pending_retry
  ON tasks (next_retry_at) WHERE status = 'pending';

CREATE INDEX idx_tasks_dispatched_timeout
  ON tasks (dispatched_at) WHERE status = 'dispatched';
```

**Background worker** (runs on every server node):

The worker uses two separate transactions — never holds a DB transaction across
network I/O:

```
loop (woken by pg_notify('new_task') or 500ms fallback poll):

  -- Tx1: claim tasks (fast, releases lock immediately)
  BEGIN;
  tasks = SELECT ... FROM tasks WHERE status='pending'
          AND (next_retry_at IS NULL OR next_retry_at <= now())
          FOR UPDATE SKIP LOCKED
          LIMIT 10;
  UPDATE tasks SET status='dispatched', agent_id=$id, dispatched_at=now()
    WHERE id = ANY($task_ids);
  COMMIT;

  -- Outside any transaction: dispatch over gRPC
  for each task:
    result = TunnelHub.Dispatch(agent, task, task.timeout_sec)

    -- Tx2: record result (atomic: task + resource + reservation)
    BEGIN;
    if error:
      if result.code == ERROR_PERMANENT or task.retries >= 2:
        UPDATE tasks SET status='failed', error=$err,
          error_history = error_history || $entry, retries=retries+1;
      else:
        UPDATE tasks SET status='pending', agent_id=NULL,
          next_retry_at=now()+backoff, error=$err,
          error_history = error_history || $entry, retries=retries+1;

      UPDATE compute_nodes SET reserved_vcpu = reserved_vcpu - $v,
        reserved_ram_mb = reserved_ram_mb - $r WHERE id = $agent_id;
    else:
      UPDATE tasks SET status='completed', completed_at=now();
      UPDATE instances SET status='ACTIVE' WHERE id = $resource_id;
      UPDATE compute_nodes SET reserved_vcpu = reserved_vcpu - $v,
        reserved_ram_mb = reserved_ram_mb - $r WHERE id = $agent_id;
    COMMIT;

  -- DB error handling:
  if SELECT fails: log ERROR, increment consecutive_failures counter,
    expose /healthz as unhealthy after 5 consecutive failures,
    backoff before next poll.
```

**Immediate task wakeup**: Nova handler calls `pg_notify('new_task', task_id)` after
INSERT. Worker listens via pgx `WaitForNotification`, waking immediately. The 500ms
poll is a reliability backstop only.

`FOR UPDATE SKIP LOCKED` on the tasks table ensures two server nodes never process
the same task. For agent-side idempotency, agents check `task_id` in local state
before executing — preventing double execution during server failover.

---

### 5. Scheduler — Atomic Resource Reservation

Stats from agents update the `compute_nodes` table (total capacity only — `total_*`
columns). `reserved_*` columns are managed exclusively by the scheduler transaction.
Free capacity is always computed as `total - reserved`, never stored directly.

```sql
ALTER TABLE compute_nodes ADD COLUMN total_vcpu      INT NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN total_ram_mb    BIGINT NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN total_disk_gb   BIGINT NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN reserved_vcpu   INT NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN reserved_ram_mb BIGINT NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN reserved_disk_gb BIGINT NOT NULL DEFAULT 0;
ALTER TABLE compute_nodes ADD COLUMN stats_updated_at TIMESTAMPTZ;
ALTER TABLE compute_nodes ADD COLUMN agent_stream_server_id TEXT;

ALTER TABLE compute_nodes ADD CONSTRAINT chk_reservation_within_capacity
  CHECK (reserved_vcpu >= 0 AND reserved_vcpu <= total_vcpu
     AND reserved_ram_mb >= 0 AND reserved_ram_mb <= total_ram_mb
     AND reserved_disk_gb >= 0 AND reserved_disk_gb <= total_disk_gb);

CREATE INDEX idx_compute_nodes_scheduling
  ON compute_nodes (total_vcpu, total_ram_mb)
  WHERE status = 'active';
```

**Note**: After migration, existing nodes have `total_vcpu = 0` and are invisible to
the scheduler until they reconnect and report stats via StatsStream. This is expected
behavior during rolling upgrades.

Scheduling transaction uses `FOR UPDATE SKIP LOCKED` on `compute_nodes` to avoid
blocking concurrent schedulers. If the best-fit node is locked by another scheduler,
the query skips it and selects the next-best node rather than blocking:

```sql
BEGIN;

SELECT id, reserved_vcpu, total_vcpu, reserved_ram_mb, total_ram_mb
FROM compute_nodes
WHERE status = 'active'
  AND agent_stream_server_id = $this_server_id
  AND stats_updated_at > now() - interval '30 seconds'
  AND (total_vcpu - reserved_vcpu) >= $req_vcpu
  AND (total_ram_mb - reserved_ram_mb) >= $req_ram_mb
  AND (total_disk_gb - reserved_disk_gb) >= $req_disk_gb
ORDER BY (total_vcpu - reserved_vcpu) DESC
LIMIT 1
FOR UPDATE SKIP LOCKED;

UPDATE compute_nodes
SET reserved_vcpu    = reserved_vcpu    + $req_vcpu,
    reserved_ram_mb  = reserved_ram_mb  + $req_ram_mb,
    reserved_disk_gb = reserved_disk_gb + $req_disk_gb
WHERE id = $agent_id;

COMMIT;
```

**`agent_stream_server_id`**: Each server node updates this column when an agent's
gRPC connection is established. Workers only dispatch to agents connected to their
own server node, preventing split-brain dispatch in HA.

**Reservation lifecycle**: The reservation decrement is part of the same DB
transaction as task completion (see worker Tx2 in Section 4). It is never a separate
call.

**Reconciliation goroutine**: Scans tasks in `dispatched` state where
`now() > dispatched_at + 2 * timeout_sec * interval '1 second'`. Uses
`SELECT FOR UPDATE` on the task row, re-checks status after acquiring lock,
then releases reservation with `GREATEST(0, reserved - req)` as a safety floor.
Adds a `reconciled_at` column to prevent double-reconciliation.

**Image-aware placement**: Scheduler prefers agents that already have the requested
image cached (from `cached_images` in AgentStats) when resources are otherwise equal.

---

### 6. Image Pre-Fetch — Decoupled from VM Create

Image pull is separated from VM creation. Two phases:

**Phase 1 — `IMAGE_PREFETCH` task** (timeout from config `image_prefetch_timeout`,
retryable):
- Dispatched by Nova when `POST /servers` is received, before `VM_CREATE`
- Agent checks local image cache (keyed by `image_id` + `checksum`)
- Cache hit is valid only if the local file checksum matches the current Glance
  image checksum. Stale cache entries (checksum mismatch) are evicted and re-pulled.
- Image download URL is a signed, time-limited token generated server-side —
  the raw Glance backend URL is never passed in the task payload (prevents SSRF).
- On success: task completes. The `VM_CREATE` task insertion is part of the same
  transaction as the `IMAGE_PREFETCH` completion write (atomic).

**Phase 2 — `VM_CREATE` task** (timeout: 30s):
- Image is guaranteed local — no image download I/O
- `vm.create` payload includes `image_local_path` populated from the image cache
  entry recorded during prefetch completion.
- libvirt domain create only: fast, bounded

**Image prefetch timeout**: 5 minutes is an intentional exception to the fail-fast
rule. Image downloads are long-running data transfers that cannot complete in <1s.
The timeout is bounded and retryable, consistent with how container runtimes
(containerd, CRI-O) handle image pulls separately from container creation.

---

### 7. Agent Local State

Agent maintains state at `/var/lib/o3k/agent/state.db` using `modernc.org/sqlite`
(pure Go, CGO-free, required for static single-binary builds — do not use
`mattn/go-sqlite3`):

```sql
CREATE TABLE current_task (
  singleton   INTEGER PRIMARY KEY DEFAULT 1 CHECK (singleton = 1),
  task_id     TEXT NOT NULL,
  type        TEXT NOT NULL,
  payload     TEXT NOT NULL,
  status      TEXT NOT NULL CHECK (status IN ('executing', 'completed', 'failed')),
  result      TEXT,
  error       TEXT,
  started_at  INTEGER NOT NULL  -- Unix epoch seconds, UTC
);

CREATE TABLE image_cache (
  image_id    TEXT PRIMARY KEY,
  local_path  TEXT NOT NULL,
  checksum    TEXT NOT NULL,     -- md5 from Glance metadata
  size_bytes  INTEGER NOT NULL,
  cached_at   INTEGER NOT NULL,  -- Unix epoch seconds
  verified_at INTEGER            -- NULL = never re-verified after caching
);
```

The `singleton = 1` constraint enforces at most one row in `current_task`. Any
attempt to insert a second row fails immediately. All writes to `current_task`
must hold a mutex.

**Reconnect recovery**:
1. Agent dials new server (via VIP).
2. Calls `Register(Hello)` with `node_id`, `hostname`, `cached_images`, and any
   `OrphanReport` entries from local state.
3. Agent queries `current_task` for status `completed` or `failed`.
4. If found: sends `TaskResult` (with HMAC) to new server.
5. Server validates HMAC, then checks task ownership:
   ```sql
   UPDATE tasks SET status='completed', completed_at=now(), agent_id=$agent
   WHERE id=$task_id AND status='dispatched' AND agent_id=$original_agent
   RETURNING id;
   ```
   If the task was already completed by a different agent (retried during
   disconnect), the update returns 0 rows. Server sends a `VM_DELETE` cleanup
   task to the reconnecting agent for the orphaned domain.
6. If server doesn't know the task (server crashed before writing to DB): agent
   sends an `OrphanReport` in the `Hello` message. Server creates the missing
   task row as `completed`, updates the resource row, and releases reservation.
   If `OrphanReport` contains a `VM_CREATE` result, server reconciles the instance.
7. Agent clears local state after server acknowledges.

---

### 8. HA — Multiple Servers

Three or more server nodes behind a load balancer (or keepalived VIP). PostgreSQL
is the shared state store — no etcd required (already in use).

```
          +----------------------------------------------+
          |  Load Balancer / VIP                         |
          |  :6385  (agent gRPC tunnel, internal only)   |
          |  :35357 :8774 :9696 :8776 :9292 :8778 (API) |
          +------------+-----------------+---------------+
                       |                 |
               +-------v--+       +------v----+
               | Server 1 |       | Server 2  |   active/active
               | TunnelHub|       | TunnelHub |   shared PostgreSQL
               | Worker   |       | Worker    |   FOR UPDATE SKIP LOCKED
               +----------+       +-----------+
                       |                 |
               +-------v-----------------v------+
               |        PostgreSQL               |
               |  tasks, compute_nodes, ...      |
               +--------------------------------+
                       |
           +-----------+-----------+
       +---v---+   +---v---+   +---v---+
       |Agent 1|   |Agent 2|   |Agent 3|
       +-------+   +-------+   +-------+
```

**Port 6385 must be on a separate, internal-only listener** — not exposed to the
public network. Agent join/tunnel traffic is internal infrastructure, not
user-facing API.

Agent connections are sticky to one server until that server dies. On disconnect,
agent reconnects to VIP (any healthy server). No state loss because all task state
lives in PostgreSQL and agent local SQLite.

Workers only dispatch to agents whose `agent_stream_server_id` matches their own
server node ID (see Section 5). This prevents split-brain dispatch during network
partitions.

---

### 9. Failure Mode Table

| Failure | Detection | Outcome |
|---------|-----------|---------|
| Agent HeartbeatStream drops | 5s timeout, stream EOF, or TCP close | Agent marked `offline`, new dispatch suppressed. In-flight tasks accepted until `task.timeout` expires, then retried. |
| Server crashes mid-dispatch | Task stays `dispatched` in DB | Reconciler detects after 2x `timeout_sec`, releases reservation, requeues to any eligible agent |
| Agent crashes mid-execution | Agent local SQLite records state | On restart/reconnect: sends result or OrphanReport. Server reconciles. |
| Double-booking race | `SELECT FOR UPDATE SKIP LOCKED` on compute_nodes | One scheduler wins, other selects next-best node |
| Image pull timeout | `IMAGE_PREFETCH` retried up to 3x with backoff | `VM_CREATE` queued only after successful prefetch (atomic) |
| All agents offline | Scheduler finds no eligible agent | Task stays `pending`. No 503 returned. Structured WARN log after 60s with no eligible agent. |
| Worker DB unavailable | Consecutive query failures | /healthz returns 503 after 5 failures, LB stops routing |
| Agent reconnect with stale result | Task already completed by another agent | Server rejects stale result (UPDATE returns 0 rows), sends cleanup task |
| Image deleted between prefetch and vm.create | Image row missing at vm.create dispatch | Instance set to ERROR, task set to failed immediately (no retry) |

---

### 10. Observability

**Structured audit log**: Separate from application debug logs. Append-only
`audit_events` table in PostgreSQL:

| Event | Fields |
|-------|--------|
| `agent.join` | node_id, source_ip, cert_serial, timestamp |
| `agent.leave` | node_id, reason (disconnect/deregister), timestamp |
| `agent.cert_issued` | node_id, cert_serial, expiry, timestamp |
| `agent.cert_revoked` | node_id, cert_serial, revoked_by, timestamp |
| `task.dispatched` | task_id, node_id, type, timestamp |
| `task.completed` | task_id, node_id, duration_ms, timestamp |
| `task.failed` | task_id, node_id, error, retry_count, timestamp |
| `reconciler.fired` | task_id, old_agent, action, timestamp |

**Structured log events**: Every task lifecycle state transition emits a structured
log entry with fields: `task_id`, `node_id`, `task_type`, `status`, `error` (if any).

**Inspection commands**:
- `o3k node list` — shows all agents, status, connected server, resource utilization
- `o3k node status <node-id>` — detailed agent info including in-flight tasks
- `o3k task list --status=pending` — query task queue state
- `o3k node reconcile` — scan for orphaned resources across all agents

---

### 11. New Files and Changes

**New packages** (all require creation — none exist yet):
- `internal/tunnel/` — TunnelHub, gRPC server, stream management
- `internal/worker/` — background task worker, retry logic
- `internal/scheduler/` — atomic reservation, placement algorithm
- `internal/agent/` — agent main loop, task executor, local state
- `proto/agent.proto` — gRPC service definition
- `proto/payloads.proto` — typed payload structs per TaskType

**Modified**:
- `cmd/o3k/main.go` — add `server` / `agent` / `token` subcommand dispatch via
  `flag.NewFlagSet` (preserves backward compat for bare `o3k` invocation)
- `internal/nova/` — return 202 on create, write task to DB, add
  `OS-EXT-STS:task_state` to response, extract `X-Idempotency-Key` header
- `internal/neutron/` — same pattern for port/network operations that touch compute
- `internal/placement/` — update resource provider inventory from `compute_nodes`
  reservation columns to keep Placement in sync with scheduler state
- `internal/compute/node_registry.go` — remove DB-polling heartbeat loop. Liveness
  now detected via HeartbeatStream drop. `last_heartbeat` column retained but
  updated by gRPC heartbeat handler, not the old ticker goroutine.

**New DB migrations**:
- `tasks` table (with CHECK constraints, indexes, FK)
- `compute_nodes` additions (resource columns, CHECK constraints, indexes)
- `revoked_agent_certs` table
- `audit_events` table

**Unchanged**: Keystone, Cinder, Glance, Metadata, middleware, existing config
structure (new `agent` and `server` sections added to `Config`).

---

### 12. Configuration

New sections in `config/o3k.yaml`:

```yaml
server:
  state_dir: "/var/lib/o3k/server"
  tunnel_port: 6385
  max_agent_inflight: 1

agent:
  server_url: "https://10.0.0.1:6385"    # required
  token: ""                                # or O3K_TOKEN env var (required)
  token_file: ""                           # preferred over token
  node_id: "auto"                          # auto = UUID persisted to disk
  state_dir: "/var/lib/o3k/agent"
  heartbeat_interval: 5s
  stats_interval: 10s
  image_cache_dir: "/var/lib/o3k/agent/images"

task_timeouts:
  default: 30s
  IMAGE_PREFETCH: 5m
```

**Config validation**: Agent config `Validate()` checks at startup (fail-fast):
- `server_url` is non-empty and valid URL
- `token` or `token_file` or `O3K_TOKEN` is set (error if all empty)
- `heartbeat_interval > 0`, `stats_interval > 0`
- All duration fields parsed via `time.ParseDuration`

---

## What This Is Not

- Not a full scheduler (no anti-affinity, no availability zones in v1)
- Not live migration (VMs stay on their node)
- Not automatic recovery from agent death (VMs on dead agent stay in `ERROR` state,
  operator triggers evacuation manually)
- Not a replacement for Ceph (shared storage still required for image backends)

These are follow-on specs.

---

## Test Strategy

Per Constitution Article III, TDD is mandatory. All tests must be written and
confirmed RED before any implementation begins.

### Test Infrastructure

| Component | Tool | Rationale |
|-----------|------|-----------|
| Scheduler tests | `dockertest` (real PostgreSQL) | Must exercise `FOR UPDATE SKIP LOCKED` |
| TunnelHub unit tests | `google.golang.org/grpc/test/bufconn` | In-process gRPC, no network |
| Agent local state | `modernc.org/sqlite` with `:memory:` | Fast, CGO-free |
| Stream drop simulation | Cancel server-side context | Assert client detects within heartbeat interval |

### Required Tests (must be RED before implementation)

**internal/tunnel/tunnel_test.go**:
- `TestTunnelHub_AgentRegistersAndReceivesTask`
- `TestTunnelHub_RejectsWrongOrganization` — cert with `O=wrong-org` -> refused
- `TestTunnelHub_RejectsExpiredCert` — expired client cert -> refused
- `TestTunnelHub_RejectsInvalidToken` — bad token -> 401, not 500
- `TestTunnelHub_RejectsMismatchedNodeID` — HELLO node-id != cert CN -> refused
- `TestTunnelHub_HeartbeatTimeoutMarksOffline` — clean disconnect, partition, kill
- `TestTunnelHub_AcceptsResultFromOfflineAgent` — late TaskResult still processed
- `TestTunnelHub_MaxInflightEnforced` — second task blocked until first completes

**internal/scheduler/scheduler_test.go** (requires real PostgreSQL via dockertest):
- `TestScheduler_NoConcurrentDoubleBooking` — 8 goroutines, 4 vCPU node, exactly 4 scheduled
- `TestScheduler_ReservationReleasedOnFailure`
- `TestScheduler_SkipsStaleStatsNodes` — stats_updated_at older than 30s
- `TestScheduler_PrefersImageWarmAgent`
- `TestScheduler_SkipsLockedAgent` — SKIP LOCKED behavior

**internal/worker/worker_test.go** (requires real PostgreSQL):
- `TestWorker_TaskRetryStateMachine` (table-driven: retries=0/2/3, permanent error)
- `TestWorker_SeparateTransactions` — Tx1 and Tx2 are independent
- `TestWorker_ReconcilerReleasesStaleReservation`
- `TestWorker_ReconcilerDoesNotDoubleDecrement` — concurrent completion + reconciler
- `TestWorker_PrefetchThenVmCreateAtomic` — crash between = no orphan
- `TestWorker_DBFailureExposesUnhealthyEndpoint`
- `TestWorker_PgNotifyWakesImmediately`

**internal/agent/agent_test.go**:
- `TestAgent_JoinAndReceiveFirstTaskWithin5s`
- `TestAgent_Reconnect_DeliversCompletedResult`
- `TestAgent_Reconnect_ServerUnknownTask_SendsOrphanReport`
- `TestAgent_Reconnect_TaskAlreadyRetried_AcceptsCleanup`
- `TestAgent_TaskTimeoutCancelsExecution`
- `TestAgent_ImageCacheValidatesChecksum`
- `TestAgent_SingletonCurrentTaskEnforced`
- `TestAgent_ConcurrentCompleteAndReconnect`

**Contract tests** (Article IX — must pass before either side is implemented):
- `TestProto_TaskStreamRoundTrip` — serialize/deserialize all TaskTypes
- `TestProto_TaskResultRoundTrip` — success and all ErrorCode variants
- `TestProto_AgentStatsRoundTrip`

**Integration tests** (`test/`):
- `TestBinaryBackwardCompat` — `o3k` with no args starts all services on correct ports
- `TestIdempotentServerCreate` — same key returns same server ID
- `TestHATaskPickup_CrossServer` — Server 1 crashes, Server 2 picks up task

---

## Success Criteria

1. `o3k agent --server ... --token-file ...` joins cluster and receives work within 5s
2. `POST /servers` returns 202 in < 100ms at p99 under 50 concurrent requests
3. Agent node failure detected within 10s (2x heartbeat interval)
4. No double-booking under concurrent load (verified by `TestScheduler_NoConcurrentDoubleBooking`)
5. Agent reconnect delivers in-flight task result to new server
6. Agent reconnect when server has no record produces OrphanReport (not silent loss)
7. Rolling server update causes zero task loss (tasks in DB survive)

---

## Documentation Updates Required

When this spec is implemented, the following docs must be updated:

- `docs/ARCHITECTURE.md`: Update "Design Philosophy" to note async exception for
  multi-node. Update service list to include Placement. Remove "No VXLAN in v1"
  (VXLAN is implemented). Remove "v2 - Future" label from multi-node.
- `docs/SCALING.md`: Add notice at top that server/agent architecture supersedes
  the HAProxy model described there.
- `docs/INDEX.md`: Add "Design Specs" section linking to this document.
