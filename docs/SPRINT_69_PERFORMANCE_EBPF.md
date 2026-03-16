# Sprint 69: Performance Optimization & eBPF Security Groups

**Status**: 🔵 PLANNED
**Priority**: 🔴 CRITICAL
**Timeline**: 2-3 weeks
**Goal**: Improve production performance and implement eBPF-based security groups

---

## Overview

With 91% API coverage complete, Sprint 69 focuses on **performance** and **scale** improvements:
- eBPF security groups (10x faster packet filtering)
- Query optimization (reduce database round-trips)
- Connection pooling improvements
- Caching layer enhancements
- Performance benchmarking suite

---

## Phase 1: eBPF Security Groups (Week 1-2)

### Current State: iptables-based Security Groups

**Implementation**: `pkg/networking/security_groups.go`

**How it works**:
```go
// Current iptables approach (lines 150-200)
func (rm *RouterManager) ApplySecurityGroup(portID, sgID string, rules []SecurityGroupRule) error {
    for _, rule := range rules {
        // One iptables command per rule
        cmd := exec.Command("iptables", "-A", "FORWARD",
            "-p", rule.Protocol,
            "--dport", rule.PortRange,
            "-j", "ACCEPT")
        cmd.Run()
    }
}
```

**Performance**:
- ~10ms per rule (userspace → kernel context switch)
- 100 rules = 1 second to apply
- High CPU usage during rule updates

---

### Target State: eBPF-based Security Groups

**Goal**: Replace iptables with eBPF programs for 10x performance improvement.

**Implementation Plan**:

#### 1. eBPF Program Structure

**File**: `pkg/networking/ebpf/secgroup.c` (new)

```c
// XDP program for security group filtering
// Runs at network driver level (before kernel networking stack)

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>

// Security group rule map (port_id -> rules)
struct bpf_map_def SEC("maps") sg_rules = {
    .type = BPF_MAP_TYPE_HASH,
    .key_size = sizeof(__u32),    // port_id
    .value_size = sizeof(struct sg_rule_set),
    .max_entries = 10000,
};

// Per-rule verdict map
struct sg_rule_set {
    __u32 rule_count;
    struct sg_rule rules[100];  // Max 100 rules per port
};

struct sg_rule {
    __u8 protocol;           // IPPROTO_TCP, IPPROTO_UDP, IPPROTO_ICMP
    __u16 port_min;
    __u16 port_max;
    __u32 remote_ip_prefix;  // CIDR prefix
    __u32 remote_ip_mask;    // CIDR mask
    __u8 direction;          // 0=ingress, 1=egress
};

SEC("xdp_secgroup")
int xdp_security_group_filter(struct xdp_md *ctx) {
    void *data = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    // Parse Ethernet header
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_DROP;

    // Only process IPv4
    if (eth->h_proto != htons(ETH_P_IP))
        return XDP_PASS;

    // Parse IP header
    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end)
        return XDP_DROP;

    // Lookup security group rules for this port
    __u32 port_id = get_port_id_from_mac(eth->h_dest);
    struct sg_rule_set *rules = bpf_map_lookup_elem(&sg_rules, &port_id);
    if (!rules)
        return XDP_DROP;  // No rules = deny all

    // Check each rule
    for (int i = 0; i < rules->rule_count && i < 100; i++) {
        struct sg_rule *rule = &rules->rules[i];

        // Match protocol
        if (rule->protocol != 0 && rule->protocol != ip->protocol)
            continue;

        // Match port range (for TCP/UDP)
        if (ip->protocol == IPPROTO_TCP || ip->protocol == IPPROTO_UDP) {
            struct tcphdr *tcp = (void *)ip + sizeof(*ip);
            if ((void *)(tcp + 1) > data_end)
                continue;

            __u16 dport = ntohs(tcp->dest);
            if (dport < rule->port_min || dport > rule->port_max)
                continue;
        }

        // Match source IP (CIDR)
        __u32 src_ip = ntohl(ip->saddr);
        if ((src_ip & rule->remote_ip_mask) != rule->remote_ip_prefix)
            continue;

        // Rule matched - ACCEPT
        return XDP_PASS;
    }

    // No rules matched - DROP
    return XDP_DROP;
}

char _license[] SEC("license") = "GPL";
```

#### 2. Go Integration Layer

**File**: `pkg/networking/ebpf/secgroup_ebpf.go` (new)

```go
package ebpf

import (
    "fmt"
    "net"
    "syscall"

    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
)

// SecurityGroupManager manages eBPF-based security groups
type SecurityGroupManager struct {
    prog     *ebpf.Program
    sgRules  *ebpf.Map
    links    map[string]link.Link  // interface -> XDP link
}

// SecurityGroupRule represents a security group rule
type SecurityGroupRule struct {
    Protocol      uint8   // syscall.IPPROTO_TCP, IPPROTO_UDP, IPPROTO_ICMP
    PortMin       uint16
    PortMax       uint16
    RemoteIPCIDR  string  // "0.0.0.0/0", "192.168.1.0/24", etc.
    Direction     uint8   // 0=ingress, 1=egress
}

// NewSecurityGroupManager creates eBPF security group manager
func NewSecurityGroupManager() (*SecurityGroupManager, error) {
    // Load compiled eBPF program
    spec, err := ebpf.LoadCollectionSpec("pkg/networking/ebpf/secgroup.o")
    if err != nil {
        return nil, fmt.Errorf("failed to load eBPF spec: %w", err)
    }

    coll, err := ebpf.NewCollection(spec)
    if err != nil {
        return nil, fmt.Errorf("failed to create eBPF collection: %w", err)
    }

    prog := coll.Programs["xdp_security_group_filter"]
    sgRules := coll.Maps["sg_rules"]

    return &SecurityGroupManager{
        prog:    prog,
        sgRules: sgRules,
        links:   make(map[string]link.Link),
    }, nil
}

// AttachToInterface attaches XDP program to network interface
func (m *SecurityGroupManager) AttachToInterface(ifaceName string) error {
    iface, err := net.InterfaceByName(ifaceName)
    if err != nil {
        return fmt.Errorf("failed to get interface %s: %w", ifaceName, err)
    }

    // Attach XDP program to interface
    l, err := link.AttachXDP(link.XDPOptions{
        Program:   m.prog,
        Interface: iface.Index,
        Flags:     link.XDPGenericMode,  // Use generic mode (fallback)
    })
    if err != nil {
        return fmt.Errorf("failed to attach XDP: %w", err)
    }

    m.links[ifaceName] = l
    return nil
}

// UpdateSecurityGroup updates rules for a specific port
func (m *SecurityGroupManager) UpdateSecurityGroup(portID uint32, rules []SecurityGroupRule) error {
    // Convert to eBPF format
    ruleSet := make([]byte, 4+len(rules)*32)  // 4 bytes count + rules
    ruleSet[0] = byte(len(rules))

    offset := 4
    for _, rule := range rules {
        // Protocol
        ruleSet[offset] = rule.Protocol
        offset++

        // Port range
        ruleSet[offset] = byte(rule.PortMin >> 8)
        ruleSet[offset+1] = byte(rule.PortMin)
        ruleSet[offset+2] = byte(rule.PortMax >> 8)
        ruleSet[offset+3] = byte(rule.PortMax)
        offset += 4

        // CIDR (IP + mask)
        ip, ipnet, err := net.ParseCIDR(rule.RemoteIPCIDR)
        if err != nil {
            return fmt.Errorf("invalid CIDR %s: %w", rule.RemoteIPCIDR, err)
        }

        ipv4 := ip.To4()
        mask := ipnet.Mask
        copy(ruleSet[offset:offset+4], ipv4)
        copy(ruleSet[offset+4:offset+8], mask)
        offset += 8

        // Direction
        ruleSet[offset] = rule.Direction
        offset++
    }

    // Update map
    key := portID
    if err := m.sgRules.Put(key, ruleSet); err != nil {
        return fmt.Errorf("failed to update eBPF map: %w", err)
    }

    return nil
}

// RemoveSecurityGroup removes rules for a specific port
func (m *SecurityGroupManager) RemoveSecurityGroup(portID uint32) error {
    key := portID
    if err := m.sgRules.Delete(key); err != nil {
        return fmt.Errorf("failed to delete from eBPF map: %w", err)
    }
    return nil
}

// Close detaches all XDP programs and releases resources
func (m *SecurityGroupManager) Close() error {
    for _, l := range m.links {
        l.Close()
    }
    m.prog.Close()
    m.sgRules.Close()
    return nil
}
```

#### 3. Neutron Integration

**File**: `pkg/networking/security_groups.go` (modify)

```go
// Add eBPF mode support
type SecurityGroupMode string

const (
    SecurityGroupModeIPTables SecurityGroupMode = "iptables"
    SecurityGroupModeEBPF     SecurityGroupMode = "ebpf"
    SecurityGroupModeStub     SecurityGroupMode = "stub"
)

type RouterManager struct {
    mode       string
    sgMode     SecurityGroupMode  // NEW
    ebpfMgr    *ebpf.SecurityGroupManager  // NEW
    // ... existing fields
}

// NewRouterManager creates a new router manager
func NewRouterManager(mode string, sgMode SecurityGroupMode) (*RouterManager, error) {
    rm := &RouterManager{
        mode:   mode,
        sgMode: sgMode,
    }

    // Initialize eBPF manager if mode is ebpf
    if sgMode == SecurityGroupModeEBPF {
        mgr, err := ebpf.NewSecurityGroupManager()
        if err != nil {
            return nil, fmt.Errorf("failed to create eBPF manager: %w", err)
        }
        rm.ebpfMgr = mgr
    }

    return rm, nil
}

// ApplySecurityGroup applies security group rules (supports both modes)
func (rm *RouterManager) ApplySecurityGroup(portID, sgID string, rules []SecurityGroupRule) error {
    if rm.mode == "stub" {
        return nil
    }

    switch rm.sgMode {
    case SecurityGroupModeEBPF:
        return rm.applySecurityGroupEBPF(portID, sgID, rules)
    case SecurityGroupModeIPTables:
        return rm.applySecurityGroupIPTables(portID, sgID, rules)
    default:
        return fmt.Errorf("unsupported security group mode: %s", rm.sgMode)
    }
}

// applySecurityGroupEBPF applies rules using eBPF
func (rm *RouterManager) applySecurityGroupEBPF(portID, sgID string, rules []SecurityGroupRule) error {
    // Convert string portID to uint32 hash
    portIDHash := hash(portID)

    // Convert rules to eBPF format
    ebpfRules := make([]ebpf.SecurityGroupRule, len(rules))
    for i, rule := range rules {
        ebpfRules[i] = ebpf.SecurityGroupRule{
            Protocol:     protocolToInt(rule.Protocol),
            PortMin:      uint16(rule.PortMin),
            PortMax:      uint16(rule.PortMax),
            RemoteIPCIDR: rule.RemoteIPPrefix,
            Direction:    0,  // ingress
        }
    }

    return rm.ebpfMgr.UpdateSecurityGroup(portIDHash, ebpfRules)
}

// applySecurityGroupIPTables applies rules using iptables (existing implementation)
func (rm *RouterManager) applySecurityGroupIPTables(portID, sgID string, rules []SecurityGroupRule) error {
    // Existing iptables implementation
    // ... (keep current code)
}
```

#### 4. Configuration

**File**: `config/o3k.yaml` (add option)

```yaml
neutron:
  networking_mode: real  # stub, iptables, ebpf
  security_group_mode: ebpf  # NEW: stub, iptables, ebpf
  vxlan_enabled: true
```

#### 5. Prerequisites & Build

**Dependencies**:
```bash
# Install eBPF development tools
sudo apt-get install -y \
    clang llvm \
    libbpf-dev \
    linux-headers-$(uname -r)

# Install Go eBPF library
go get github.com/cilium/ebpf@latest
```

**Makefile** (add eBPF compilation):
```makefile
# Build eBPF programs
.PHONY: build-ebpf
build-ebpf:
	clang -O2 -target bpf -c pkg/networking/ebpf/secgroup.c -o pkg/networking/ebpf/secgroup.o

# Build with eBPF
.PHONY: build-with-ebpf
build-with-ebpf: build-ebpf build
```

---

### eBPF Performance Benchmarks

**Test Setup**:
- 1000 ports with 10 security group rules each (10,000 total rules)
- 100,000 packets/second throughput

**Expected Results**:

| Operation | iptables | eBPF | Improvement |
|-----------|----------|------|-------------|
| Rule Application | 10 seconds | 100ms | **100x faster** |
| Packet Filtering | ~50µs/packet | ~5µs/packet | **10x faster** |
| CPU Usage | 40% | 4% | **10x reduction** |
| Memory | 500MB | 50MB | **10x reduction** |

**Why eBPF is Faster**:
1. **Kernel-space execution** - No userspace context switch
2. **XDP (eXpress Data Path)** - Processes packets before kernel networking stack
3. **Map-based lookups** - O(1) rule matching vs O(n) iptables chains
4. **JIT compilation** - eBPF programs compiled to native machine code

---

## Phase 2: Database Query Optimization (Week 1-2)

### Current Issues

**Problem**: N+1 query patterns in list endpoints

**Example** (`internal/nova/servers.go` lines 200-250):
```go
// ListServers - current implementation
func (svc *Service) ListServers(c *gin.Context) {
    // Query 1: Get all servers
    rows, _ := database.DB.Query(ctx, "SELECT id, name, flavor_id, image_id FROM instances WHERE project_id = $1", projectID)

    for rows.Next() {
        var server Server
        rows.Scan(&server.ID, &server.Name, &server.FlavorID, &server.ImageID)

        // Query 2-N: Get flavor for each server (N queries!)
        database.DB.QueryRow(ctx, "SELECT name FROM flavors WHERE id = $1", server.FlavorID).Scan(&server.FlavorName)

        // Query N+1: Get image for each server (N queries!)
        database.DB.QueryRow(ctx, "SELECT name FROM images WHERE id = $1", server.ImageID).Scan(&server.ImageName)

        servers = append(servers, server)
    }
}
```

**Impact**: 100 servers = 201 queries (1 + 100*2)

---

### Solution: JOIN Queries

**File**: `internal/nova/servers.go` (optimize)

```go
// ListServers - optimized implementation
func (svc *Service) ListServers(c *gin.Context) {
    // Single query with JOINs
    query := `
        SELECT
            i.id, i.name, i.status, i.power_state,
            f.id AS flavor_id, f.name AS flavor_name, f.vcpus, f.ram, f.disk,
            img.id AS image_id, img.name AS image_name,
            n.id AS network_id, n.name AS network_name
        FROM instances i
        LEFT JOIN flavors f ON i.flavor_id = f.id
        LEFT JOIN images img ON i.image_id = img.id
        LEFT JOIN ports p ON i.id = p.device_id
        LEFT JOIN networks n ON p.network_id = n.id
        WHERE i.project_id = $1
    `

    rows, _ := database.DB.Query(ctx, query, projectID)

    for rows.Next() {
        var server Server
        rows.Scan(
            &server.ID, &server.Name, &server.Status, &server.PowerState,
            &server.Flavor.ID, &server.Flavor.Name, &server.Flavor.VCPUs, &server.Flavor.RAM, &server.Flavor.Disk,
            &server.Image.ID, &server.Image.Name,
            &server.Network.ID, &server.Network.Name,
        )
        servers = append(servers, server)
    }
}
```

**Impact**: 100 servers = 1 query (201 → 1)

---

### Query Optimization Checklist

**Files to Optimize**:
- [ ] `internal/nova/servers.go` - ListServers, ListServersDetail
- [ ] `internal/neutron/networks.go` - ListNetworks (join subnets)
- [ ] `internal/neutron/ports.go` - ListPorts (join networks, IPs)
- [ ] `internal/cinder/volumes.go` - ListVolumes (join snapshots, attachments)
- [ ] `internal/glance/images.go` - ListImages (join members, tags)

**Expected Impact**:
- 10x reduction in query count
- 5x faster list endpoints
- 50% reduction in database CPU usage

---

## Phase 3: Connection Pooling (Week 2)

### Current Configuration

**File**: `internal/database/connection.go`

```go
// Current settings (suboptimal)
config, _ := pgxpool.ParseConfig(dbURL)
config.MaxConns = 20           // Too low for production
config.MinConns = 5            // Too high for idle periods
config.MaxConnLifetime = 0     // Never close connections
config.MaxConnIdleTime = 0     // Never close idle connections
```

---

### Optimized Configuration

```go
// Optimized settings for production
config, _ := pgxpool.ParseConfig(dbURL)

// Connection pool sizing
config.MaxConns = 50                            // Higher limit for concurrent requests
config.MinConns = 2                             // Lower minimum (save resources when idle)
config.MaxConnLifetime = 1 * time.Hour          // Recycle connections hourly
config.MaxConnIdleTime = 10 * time.Minute       // Close idle connections after 10 min

// Health checks
config.HealthCheckPeriod = 30 * time.Second     // Check connection health every 30s

// Connection timeout
config.ConnConfig.ConnectTimeout = 5 * time.Second
```

**Rationale**:
- `MaxConns = 50` - Supports 50 concurrent API requests (typical load)
- `MinConns = 2` - Minimal overhead during idle periods
- `MaxConnLifetime = 1h` - Prevents stale connections, forces refresh
- `MaxConnIdleTime = 10min` - Aggressively close idle connections (save database resources)

---

## Phase 4: Caching Layer (Week 2-3)

### Target: Immutable Data Caching

**Candidates** (data that rarely/never changes):
- Flavors (compute resources definitions)
- Images metadata (image properties)
- Networks (topology rarely changes)
- Service catalog (static endpoints)

---

### Implementation

**File**: `pkg/cache/cache.go` (new)

```go
package cache

import (
    "context"
    "encoding/json"
    "time"

    "github.com/redis/go-redis/v9"
)

// Cache provides Redis-backed caching
type Cache struct {
    client *redis.Client
}

// NewCache creates a new cache instance
func NewCache(redisURL string) (*Cache, error) {
    opt, err := redis.ParseURL(redisURL)
    if err != nil {
        return nil, err
    }

    client := redis.NewClient(opt)
    return &Cache{client: client}, nil
}

// Get retrieves a cached value
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
    val, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return ErrCacheMiss
    }
    if err != nil {
        return err
    }

    return json.Unmarshal([]byte(val), dest)
}

// Set stores a value in cache
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }

    return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a value from cache
func (c *Cache) Delete(ctx context.Context, key string) error {
    return c.client.Del(ctx, key).Err()
}
```

---

### Usage Example: Flavor Caching

**File**: `internal/nova/flavors.go` (add caching)

```go
// GetFlavor retrieves flavor (with cache)
func (svc *Service) GetFlavor(c *gin.Context) {
    flavorID := c.Param("id")
    ctx := c.Request.Context()

    // Try cache first
    cacheKey := fmt.Sprintf("flavor:%s", flavorID)
    var flavor Flavor
    if err := svc.cache.Get(ctx, cacheKey, &flavor); err == nil {
        c.JSON(http.StatusOK, gin.H{"flavor": flavor})
        return
    }

    // Cache miss - query database
    err := database.DB.QueryRow(ctx, `
        SELECT id, name, vcpus, ram, disk, is_public
        FROM flavors
        WHERE id = $1
    `, flavorID).Scan(&flavor.ID, &flavor.Name, &flavor.VCPUs, &flavor.RAM, &flavor.Disk, &flavor.IsPublic)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "flavor not found"})
        return
    }

    // Store in cache (24 hour TTL - flavors rarely change)
    svc.cache.Set(ctx, cacheKey, flavor, 24*time.Hour)

    c.JSON(http.StatusOK, gin.H{"flavor": flavor})
}
```

---

### Cache Invalidation

**Strategy**: Invalidate on write

```go
// CreateFlavor - invalidate cache on create
func (svc *Service) CreateFlavor(c *gin.Context) {
    // ... create flavor ...

    // Invalidate list cache
    svc.cache.Delete(ctx, "flavors:list")

    c.JSON(http.StatusCreated, gin.H{"flavor": flavor})
}

// DeleteFlavor - invalidate cache on delete
func (svc *Service) DeleteFlavor(c *gin.Context) {
    flavorID := c.Param("id")

    // Delete from database
    // ...

    // Invalidate caches
    svc.cache.Delete(ctx, fmt.Sprintf("flavor:%s", flavorID))
    svc.cache.Delete(ctx, "flavors:list")

    c.Status(http.StatusNoContent)
}
```

---

### Cache Configuration

**File**: `config/o3k.yaml` (add Redis)

```yaml
cache:
  enabled: true
  redis_url: "redis://localhost:6379/0"
  default_ttl: 1h

  # Per-resource TTL overrides
  ttl:
    flavors: 24h        # Flavors rarely change
    images: 1h          # Images change occasionally
    networks: 30m       # Networks change more frequently
    service_catalog: 24h
```

**Docker Compose** (`deployments/docker-compose.yml`):
```yaml
services:
  redis:
    image: redis:7-alpine
    container_name: o3k-redis
    restart: unless-stopped
    ports:
      - "6379:6379"
    networks:
      - o3k-network
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

---

## Phase 5: Performance Benchmarking (Week 3)

### Benchmark Suite

**File**: `test/benchmark/api_bench_test.go` (new)

```go
package benchmark

import (
    "net/http"
    "testing"
)

// BenchmarkTokenIssue benchmarks token creation
func BenchmarkTokenIssue(b *testing.B) {
    client := setupClient()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        resp, _ := client.Post("/v3/auth/tokens", authPayload)
        resp.Body.Close()
    }
}

// BenchmarkListServers benchmarks server list endpoint
func BenchmarkListServers(b *testing.B) {
    client := setupAuthenticatedClient()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        resp, _ := client.Get("/v2.1/servers/detail")
        resp.Body.Close()
    }
}

// BenchmarkCreateServer benchmarks server creation
func BenchmarkCreateServer(b *testing.B) {
    client := setupAuthenticatedClient()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        resp, _ := client.Post("/v2.1/servers", serverPayload)
        resp.Body.Close()
    }
}
```

**Run benchmarks**:
```bash
go test -bench=. -benchmem ./test/benchmark/
```

---

### Load Testing

**Tool**: k6 (https://k6.io)

**File**: `test/loadtest/api_loadtest.js` (new)

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

// Test configuration
export let options = {
    stages: [
        { duration: '1m', target: 50 },   // Ramp up to 50 users
        { duration: '3m', target: 50 },   // Stay at 50 users
        { duration: '1m', target: 100 },  // Ramp up to 100 users
        { duration: '3m', target: 100 },  // Stay at 100 users
        { duration: '1m', target: 0 },    // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<250'],  // 95% of requests under 250ms
        http_req_failed: ['rate<0.01'],    // <1% failure rate
    },
};

// Authenticate once per VU
export function setup() {
    let authRes = http.post('http://localhost:35357/v3/auth/tokens', JSON.stringify({
        auth: {
            identity: {
                methods: ['password'],
                password: {
                    user: { domain: { name: 'Default' }, name: 'admin', password: 'secret' }
                }
            }
        }
    }));

    return { token: authRes.headers['X-Subject-Token'] };
}

// Test scenario
export default function(data) {
    let params = {
        headers: { 'X-Auth-Token': data.token },
    };

    // List servers
    let res1 = http.get('http://localhost:8774/v2.1/servers/detail', params);
    check(res1, { 'list servers OK': (r) => r.status === 200 });

    sleep(1);

    // List networks
    let res2 = http.get('http://localhost:9696/v2.0/networks', params);
    check(res2, { 'list networks OK': (r) => r.status === 200 });

    sleep(1);

    // List volumes
    let res3 = http.get('http://localhost:8776/v3/volumes/detail', params);
    check(res3, { 'list volumes OK': (r) => r.status === 200 });

    sleep(1);
}
```

**Run load test**:
```bash
k6 run test/loadtest/api_loadtest.js
```

---

## Success Metrics

### Performance Targets

| Metric | Before | After (Target) | Improvement |
|--------|--------|----------------|-------------|
| Security Group Rule Application | 10s (1000 rules) | 100ms | **100x** |
| Packet Filtering Latency | 50µs | 5µs | **10x** |
| List Servers Query Count | 201 (100 servers) | 1 | **201x** |
| List Servers Response Time | 500ms | 50ms | **10x** |
| Database Connections (idle) | 20 | 2 | **10x reduction** |
| Cache Hit Rate | 0% | 80%+ | **NEW** |
| p95 API Response Time | 300ms | <250ms | **1.2x** |
| Throughput | 100 req/s | 500+ req/s | **5x** |

---

## Implementation Checklist

### Week 1: eBPF Foundation
- [ ] Set up eBPF development environment
- [ ] Write eBPF C program for security groups
- [ ] Compile and test eBPF program in isolation
- [ ] Implement Go integration layer (`pkg/networking/ebpf/`)
- [ ] Add eBPF mode to RouterManager
- [ ] Update configuration (o3k.yaml, docker-compose.yml)
- [ ] Write unit tests for eBPF integration

### Week 2: Query Optimization & Connection Pooling
- [ ] Audit all list endpoints for N+1 queries
- [ ] Rewrite queries with JOINs (Nova, Neutron, Cinder, Glance)
- [ ] Add query execution time logging
- [ ] Optimize database connection pool settings
- [ ] Add health check monitoring
- [ ] Test under load (100+ concurrent requests)

### Week 2-3: Caching Layer
- [ ] Add Redis to docker-compose.yml
- [ ] Implement cache abstraction (`pkg/cache/cache.go`)
- [ ] Add caching to flavors endpoints
- [ ] Add caching to images endpoints
- [ ] Add caching to service catalog
- [ ] Implement cache invalidation on writes
- [ ] Add cache hit/miss metrics

### Week 3: Benchmarking & Validation
- [ ] Create Go benchmark suite (`test/benchmark/`)
- [ ] Create k6 load testing scripts
- [ ] Run baseline benchmarks (before optimizations)
- [ ] Run optimized benchmarks (after optimizations)
- [ ] Compare results and validate targets met
- [ ] Document performance improvements
- [ ] Create performance tuning guide

---

## Documentation Updates

**New Documents**:
- [ ] `docs/PERFORMANCE.md` - Performance tuning guide
- [ ] `docs/EBPF_SECURITY_GROUPS.md` - eBPF setup and architecture
- [ ] `docs/CACHING.md` - Cache configuration and invalidation strategies
- [ ] `docs/BENCHMARKING.md` - How to run performance tests

**Updated Documents**:
- [ ] `README.md` - Add performance metrics
- [ ] `docs/CONFIGURATION.md` - Add cache and eBPF settings
- [ ] `docs/ARCHITECTURE.md` - Add eBPF and caching layers to diagram

---

## Risks & Mitigation

### Risk 1: eBPF Kernel Compatibility
**Risk**: eBPF requires Linux kernel 4.18+ with BPF features enabled

**Mitigation**:
- Fallback to iptables mode if eBPF unavailable
- Add kernel version check at startup
- Document kernel requirements clearly

### Risk 2: Redis Dependency
**Risk**: Adding Redis increases deployment complexity

**Mitigation**:
- Make caching optional (disable if Redis unavailable)
- Fallback to in-memory caching (with sync.Map)
- Document Redis setup clearly

### Risk 3: Query Optimization Breaking Changes
**Risk**: JOIN queries might change response structure

**Mitigation**:
- Maintain API compatibility (map JOIN results to existing structs)
- Add contract tests for optimized endpoints
- Test with Horizon dashboard

---

## Post-Sprint 69 Review

**Success Criteria**:
- eBPF security groups functional (10x performance improvement)
- Database queries optimized (10x reduction in query count)
- Caching layer operational (80%+ hit rate)
- Performance targets met (p95 < 250ms)
- Documentation complete

**Next Sprint (70)**:
- Production deployment validation
- Tempest integration tests
- High availability (multi-node control plane)
- Advanced monitoring (Prometheus/Grafana)

---

## Timeline Summary

| Week | Phase | Deliverables |
|------|-------|--------------|
| 1 | eBPF Foundation | eBPF program, Go integration, configuration |
| 2 | Query Optimization | JOIN queries, connection pooling, caching layer |
| 3 | Benchmarking | Performance tests, load tests, documentation |

**Total Duration**: 2-3 weeks
**Developer Effort**: 1 full-time developer

---

**Status**: 🔵 READY TO START
**Approval**: Awaiting go-ahead to begin implementation
