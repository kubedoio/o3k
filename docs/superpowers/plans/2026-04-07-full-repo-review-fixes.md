# Full Repo Review Fixes — Master Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all 42 findings from the full-repo-review (5 Critical, 12 High, 15 Medium, 10 Low) across 173 Go files.

**Architecture:** Seven independent phases ordered by risk (security first, then systemic patterns, then cleanup). Each phase produces a working, testable codebase. Phases can be executed sequentially or parallelized where noted.

**Tech Stack:** Go 1.26, Gin, pgx v5, zerolog, crypto/rand, golang-migrate

**Findings Reference:** `/Users/I761222/git/o3k/full-repo-review-report.md`

---

## Phase Overview

| Phase | Scope | Findings | Risk | Est. Tasks |
|-------|-------|----------|------|------------|
| 1 | Security quick-wins | C-3, C-4, C-5, M-14 | Critical | 5 |
| 2 | Common utilities | M-3, M-5, M-8, M-10, H-10, L-4, H-12 | Medium | 7 |
| 3 | Error framework migration | C-1, H-2, H-9, SP-1 | Critical | 8 |
| 4 | Database transactions | C-2 | Critical | 4 |
| 5 | Goroutine lifecycle & context | H-7, M-2, SP-5 | High | 4 |
| 6 | Observability & logging | M-1, M-4, M-6, SP-4 | Medium | 4 |
| 7 | CI, config & cleanup | H-4, H-5, H-8, H-11, M-7, M-9, M-12, M-13, M-15, L-1–L-10 | Low–High | 10 |

**Total: ~42 tasks** (some findings merge into single tasks)

**Out of scope (separate plan):** H-3/SP-2 (database dependency injection) — this is a major architectural refactor that touches 665 call sites. It deserves its own plan after the error framework migration stabilizes. H-6 (rate limiting) — needs design decisions on limits, storage backend, and per-endpoint configuration.

---

## Phase 1: Security Quick-Wins

**Branch:** `fix/security-quick-wins`
**Parallel-safe:** Yes (all files are in different packages)
**Findings:** C-3, C-4, C-5, M-14

### Task 1.1: Fix MAC address generation to use crypto/rand (C-3)

**Files:**
- Modify: `internal/neutron/network.go:1126-1131`
- Test: `internal/neutron/network_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// internal/neutron/network_test.go
package neutron

import (
	"regexp"
	"testing"
)

func TestGenerateMAC_Format(t *testing.T) {
	mac := generateMAC()
	matched, _ := regexp.MatchString(`^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`, mac)
	if !matched {
		t.Errorf("MAC %q does not match expected format", mac)
	}
}

func TestGenerateMAC_LocalBit(t *testing.T) {
	mac := generateMAC()
	// First octet should have local bit set (bit 1) and multicast cleared (bit 0)
	firstByte := mac[0:2]
	var b byte
	fmt.Sscanf(firstByte, "%02x", &b)
	if b&0x02 == 0 {
		t.Errorf("Local bit not set in first byte %02x", b)
	}
	if b&0x01 != 0 {
		t.Errorf("Multicast bit set in first byte %02x", b)
	}
}

func TestGenerateMAC_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		mac := generateMAC()
		if seen[mac] {
			t.Errorf("Duplicate MAC generated: %s", mac)
		}
		seen[mac] = true
	}
}
```

- [ ] **Step 2: Run test to confirm it compiles and passes baseline**

Run: `cd /Users/I761222/git/o3k && go test ./internal/neutron/ -run TestGenerateMAC -v`

- [ ] **Step 3: Replace math/rand with crypto/rand**

In `internal/neutron/network.go`, change the import from `"math/rand"` to `"crypto/rand"` and update the function:

```go
import "crypto/rand"

func generateMAC() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		// Fallback: this should never happen with crypto/rand
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	buf[0] = (buf[0] | 2) & 0xfe // Set local bit, clear multicast bit
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}
```

**Note:** Check if `math/rand` is imported for any other use in this file. If it is only used for MAC generation, remove the import entirely. If used elsewhere, keep both imports with an alias: `cryptorand "crypto/rand"`.

- [ ] **Step 4: Run tests**

Run: `cd /Users/I761222/git/o3k && go test ./internal/neutron/ -run TestGenerateMAC -v`
Expected: All 3 tests PASS

- [ ] **Step 5: Run full build to check for import conflicts**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Expected: Clean build

- [ ] **Step 6: Commit**

```bash
git add internal/neutron/network.go internal/neutron/network_test.go
git commit -m "fix(neutron): use crypto/rand for MAC address generation

Replaces math/rand with crypto/rand to prevent predictable MAC
addresses in multi-tenant environments. Adds unit tests for format,
local bit, and uniqueness.

Fixes: C-3"
```

---

### Task 1.2: Generate cryptographic admin passwords (C-4)

**Files:**
- Modify: `internal/nova/handlers.go:500,1783`
- Create: `internal/common/password.go`
- Test: `internal/common/password_test.go`

- [ ] **Step 1: Write the password generator test**

```go
// internal/common/password_test.go
package common

import (
	"regexp"
	"testing"
)

func TestGeneratePassword_Length(t *testing.T) {
	pw := GeneratePassword(16)
	if len(pw) != 16 {
		t.Errorf("Expected length 16, got %d", len(pw))
	}
}

func TestGeneratePassword_Alphanumeric(t *testing.T) {
	pw := GeneratePassword(32)
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9]+$`, pw)
	if !matched {
		t.Errorf("Password %q contains non-alphanumeric characters", pw)
	}
}

func TestGeneratePassword_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pw := GeneratePassword(16)
		if seen[pw] {
			t.Errorf("Duplicate password generated: %s", pw)
		}
		seen[pw] = true
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `cd /Users/I761222/git/o3k && go test ./internal/common/ -run TestGeneratePassword -v`
Expected: FAIL — `GeneratePassword` not defined

- [ ] **Step 3: Implement password generator**

```go
// internal/common/password.go
package common

import "crypto/rand"

const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GeneratePassword returns a cryptographically random alphanumeric string of the given length.
func GeneratePassword(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	for i := range b {
		b[i] = alphanumeric[int(b[i])%len(alphanumeric)]
	}
	return string(b)
}
```

- [ ] **Step 4: Run test — expect PASS**

Run: `cd /Users/I761222/git/o3k && go test ./internal/common/ -run TestGeneratePassword -v`
Expected: All 3 tests PASS

- [ ] **Step 5: Replace hardcoded passwords in nova handlers**

In `internal/nova/handlers.go`, find the two occurrences:

1. Around line 500 — `"adminPass": "generated-password"` → `"adminPass": common.GeneratePassword(16)`
2. Around line 1783 — `"adminPass": "rescuepass123"` → `"adminPass": common.GeneratePassword(16)`

Add import: `"github.com/cobaltcore-dev/o3k/internal/common"` if not already present.

- [ ] **Step 6: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Expected: Clean build

- [ ] **Step 7: Commit**

```bash
git add internal/common/password.go internal/common/password_test.go internal/nova/handlers.go
git commit -m "fix(nova): generate cryptographic admin passwords instead of hardcoded values

Adds common.GeneratePassword() using crypto/rand. Replaces
'generated-password' and 'rescuepass123' with random 16-char
alphanumeric strings.

Fixes: C-4"
```

---

### Task 1.3: Make CORS origins configurable (C-5)

**Files:**
- Modify: `internal/middleware/logging.go:249-263`
- Modify: `internal/common/config.go` (add CORS config struct)
- Modify: `config/o3k.yaml` (add CORS section)
- Test: `internal/middleware/cors_test.go` (create)

- [ ] **Step 1: Write the CORS middleware test**

```go
// internal/middleware/cors_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddleware_DefaultOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddlewareWithConfig([]string{}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost" {
		t.Errorf("Expected default origin 'http://localhost', got %q", origin)
	}
}

func TestCORSMiddleware_ConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddlewareWithConfig([]string{"https://horizon.example.com"}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://horizon.example.com")
	r.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://horizon.example.com" {
		t.Errorf("Expected origin 'https://horizon.example.com', got %q", origin)
	}
}

func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddlewareWithConfig([]string{"https://horizon.example.com"}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	r.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("Expected no origin header for disallowed origin, got %q", origin)
	}
}

func TestCORSMiddleware_OptionsPreflightReturns204(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddlewareWithConfig([]string{"https://horizon.example.com"}))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://horizon.example.com")
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("Expected 204 for OPTIONS, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `cd /Users/I761222/git/o3k && go test ./internal/middleware/ -run TestCORS -v`
Expected: FAIL — `CORSMiddlewareWithConfig` not defined

- [ ] **Step 3: Add CORS config to config struct**

In `internal/common/config.go`, add to the main config struct:

```go
type ServerConfig struct {
	// ... existing fields ...
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
}
```

In `config/o3k.yaml`, add:

```yaml
server:
  cors_allowed_origins:
    - "http://localhost"
    - "http://localhost:3000"
```

- [ ] **Step 4: Implement CORSMiddlewareWithConfig**

In `internal/middleware/logging.go`, add the new function and keep the old one as a wrapper:

```go
// CORSMiddlewareWithConfig adds CORS headers with configurable allowed origins.
// If allowedOrigins is empty, defaults to localhost only.
func CORSMiddlewareWithConfig(allowedOrigins []string) gin.HandlerFunc {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost"}
	}
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Auth-Token, X-Subject-Token, OpenStack-API-Version, X-OpenStack-Nova-API-Version, Accept")
			c.Writer.Header().Set("Access-Control-Max-Age", "3600")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// CORSMiddleware is the legacy wildcard CORS middleware.
// Deprecated: Use CORSMiddlewareWithConfig instead.
func CORSMiddleware() gin.HandlerFunc {
	return CORSMiddlewareWithConfig([]string{})
}
```

- [ ] **Step 5: Update main.go to pass CORS config**

In `cmd/o3k/main.go`, replace `middleware.CORSMiddleware()` calls with `middleware.CORSMiddlewareWithConfig(cfg.Server.CORSAllowedOrigins)` in each `createXxxServer()` function. There are 7 server factory functions that need updating.

- [ ] **Step 6: Run tests**

Run: `cd /Users/I761222/git/o3k && go test ./internal/middleware/ -run TestCORS -v`
Expected: All 4 tests PASS

Run: `cd /Users/I761222/git/o3k && go build ./...`
Expected: Clean build

- [ ] **Step 7: Commit**

```bash
git add internal/middleware/logging.go internal/middleware/cors_test.go internal/common/config.go config/o3k.yaml cmd/o3k/main.go
git commit -m "fix(middleware): make CORS origins configurable, default to localhost

Replaces wildcard Access-Control-Allow-Origin with origin-checked
CORS middleware. Adds PATCH to allowed methods, adds OpenStack-specific
headers. Configurable via server.cors_allowed_origins in o3k.yaml.

Fixes: C-5, L-3"
```

---

### Task 1.4: Harden Glance PATCH field allowlist (M-14)

**Files:**
- Modify: `internal/glance/images.go:454-493`
- Test: `internal/glance/images_test.go` (create)

- [ ] **Step 1: Write the test**

```go
// internal/glance/images_test.go
package glance

import "testing"

func TestIsAllowedImageField(t *testing.T) {
	tests := []struct {
		path    string
		allowed bool
		field   string
	}{
		{"/name", true, "name"},
		{"/visibility", true, "visibility"},
		{"/min_disk", true, "min_disk_gb"},
		{"/min_ram", true, "min_ram_mb"},
		{"/malicious; DROP TABLE images;--", false, ""},
		{"/nonexistent", false, ""},
	}
	for _, tt := range tests {
		field, ok := allowedImageUpdateField(tt.path)
		if ok != tt.allowed {
			t.Errorf("allowedImageUpdateField(%q) ok = %v, want %v", tt.path, ok, tt.allowed)
		}
		if ok && field != tt.field {
			t.Errorf("allowedImageUpdateField(%q) field = %q, want %q", tt.path, field, tt.field)
		}
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `cd /Users/I761222/git/o3k && go test ./internal/glance/ -run TestIsAllowedImageField -v`
Expected: FAIL — `allowedImageUpdateField` not defined

- [ ] **Step 3: Extract the field allowlist into a lookup function**

In `internal/glance/images.go`, add above the `UpdateImage` handler:

```go
var imageUpdateFields = map[string]string{
	"/name":       "name",
	"/visibility": "visibility",
	"/min_disk":   "min_disk_gb",
	"/min_ram":    "min_ram_mb",
}

// allowedImageUpdateField returns the DB column name for a JSON Patch path,
// and false if the path is not in the allowlist.
func allowedImageUpdateField(path string) (string, bool) {
	col, ok := imageUpdateFields[path]
	return col, ok
}
```

Then update the `UpdateImage` handler's switch block to use the map:

```go
if op == "replace" {
	field, ok := allowedImageUpdateField(path)
	if !ok {
		continue
	}
	query := fmt.Sprintf("UPDATE images SET %s = $1, updated_at = $2 WHERE id = $3 AND (visibility != 'public' OR project_id = $4)", field)
	database.DB.Exec(c.Request.Context(), query, value, time.Now(), imageID, projectID)
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/I761222/git/o3k && go test ./internal/glance/ -run TestIsAllowedImageField -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/glance/images.go internal/glance/images_test.go
git commit -m "fix(glance): harden image PATCH field allowlist against SQL injection

Extracts field validation into a map-based lookup function instead
of a switch/sprintf pattern. Rejects unknown fields before SQL
interpolation.

Fixes: M-14"
```

---

### Task 1.5: Fix rescue password (second C-4 location)

**Note:** This is covered in Task 1.2, Step 5 (both locations). This task is a verification checkpoint.

- [ ] **Step 1: Verify both adminPass locations were fixed**

Run: `cd /Users/I761222/git/o3k && grep -n '"adminPass"' internal/nova/handlers.go`
Expected: Both lines show `common.GeneratePassword(16)`, not hardcoded strings.

- [ ] **Step 2: Run contract tests to verify CreateServer still works**

Run: `cd /Users/I761222/git/o3k && go test ./test/contract/nova/ -run TestCreateServer -v -count=1`
Expected: PASS (response still contains `adminPass` field, just with a random value now)

---

## Phase 2: Common Utilities

**Branch:** `fix/common-utilities`
**Parallel-safe:** Yes (new files, no handler changes yet)
**Findings:** M-3, M-5, M-8, M-10, H-10, L-4, H-12

### Task 2.1: Create shared pagination helper (M-3, SP-3)

**Files:**
- Create: `internal/common/pagination.go`
- Test: `internal/common/pagination_test.go`

- [ ] **Step 1: Write the pagination test**

```go
// internal/common/pagination_test.go
package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParsePagination_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)

	p := ParsePagination(c, 1000)
	if p.Limit != 1000 {
		t.Errorf("Expected default limit 1000, got %d", p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("Expected default offset 0, got %d", p.Offset)
	}
	if p.Marker != "" {
		t.Errorf("Expected empty marker, got %q", p.Marker)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test?limit=50&offset=10&marker=abc-123", nil)

	p := ParsePagination(c, 1000)
	if p.Limit != 50 {
		t.Errorf("Expected limit 50, got %d", p.Limit)
	}
	if p.Offset != 10 {
		t.Errorf("Expected offset 10, got %d", p.Offset)
	}
	if p.Marker != "abc-123" {
		t.Errorf("Expected marker 'abc-123', got %q", p.Marker)
	}
}

func TestParsePagination_InvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test?limit=-5", nil)

	p := ParsePagination(c, 1000)
	if p.Limit != 1000 {
		t.Errorf("Expected default limit for invalid value, got %d", p.Limit)
	}
}

func TestParsePagination_SortKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test?sort_key=name&sort_dir=desc", nil)

	p := ParsePagination(c, 1000)
	if p.SortKey != "name" {
		t.Errorf("Expected sort_key 'name', got %q", p.SortKey)
	}
	if p.SortDir != "desc" {
		t.Errorf("Expected sort_dir 'desc', got %q", p.SortDir)
	}
}
```

- [ ] **Step 2: Run test — expect compile failure**

Run: `cd /Users/I761222/git/o3k && go test ./internal/common/ -run TestParsePagination -v`
Expected: FAIL — `ParsePagination` not defined

- [ ] **Step 3: Implement pagination helper**

```go
// internal/common/pagination.go
package common

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// PaginationParams holds parsed pagination query parameters.
type PaginationParams struct {
	Limit   int
	Offset  int
	Marker  string
	SortKey string
	SortDir string
}

// ParsePagination extracts limit, offset, marker, sort_key, and sort_dir
// from query parameters. defaultLimit is used when limit is absent or invalid.
func ParsePagination(c *gin.Context, defaultLimit int) PaginationParams {
	p := PaginationParams{
		Limit:   defaultLimit,
		Marker:  c.Query("marker"),
		SortKey: c.DefaultQuery("sort_key", "created_at"),
		SortDir: c.DefaultQuery("sort_dir", "desc"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			p.Limit = v
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			p.Offset = v
		}
	}

	// Normalize sort_dir
	if p.SortDir != "asc" && p.SortDir != "desc" {
		p.SortDir = "desc"
	}

	return p
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/I761222/git/o3k && go test ./internal/common/ -run TestParsePagination -v`
Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/common/pagination.go internal/common/pagination_test.go
git commit -m "feat(common): add shared pagination query parameter parser

Extracts duplicated limit/offset/marker/sort parsing into a single
utility. All 32+ list handlers will be migrated to use this in
subsequent commits.

Fixes: M-3, SP-3"
```

**Note:** The actual migration of ~32 list handlers to use `ParsePagination` will happen as a follow-up task per service. Create a separate task for each service migration (nova, neutron, cinder, glance) to keep diffs reviewable.

---

### Task 2.2: Fix Flavor IsPublic JSON tag (M-5)

**Files:**
- Modify: `internal/database/models.go:58`

- [ ] **Step 1: Fix the JSON tag**

In `internal/database/models.go`, line 58, change:

```go
// FROM:
IsPublic  bool      `json:"OS-FLV-EXT-DATA:ephemeral"`
// TO:
IsPublic  bool      `json:"os-flavor-access:is_public"`
```

- [ ] **Step 2: Search for any code that relies on the old tag**

Run: `grep -rn "OS-FLV-EXT-DATA:ephemeral" /Users/I761222/git/o3k/internal/`

If any handler manually sets this key in a `gin.H{}` response, update those too.

- [ ] **Step 3: Run build and tests**

Run: `cd /Users/I761222/git/o3k && go build ./... && go test ./internal/common/ -v`
Expected: Clean build, tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/database/models.go
git commit -m "fix(database): correct Flavor.IsPublic JSON tag

Was using 'OS-FLV-EXT-DATA:ephemeral' (ephemeral disk size) instead
of 'os-flavor-access:is_public'. This caused incorrect JSON
serialization in flavor API responses.

Fixes: M-5"
```

---

### Task 2.3: Remove duplicate RunMigrations function (M-8)

**Files:**
- Modify: `internal/database/db.go:104-124` (delete `RunMigrations`)
- Search: all callers of `RunMigrations`

- [ ] **Step 1: Find all callers**

Run: `grep -rn "RunMigrations\|MigrateUp" /Users/I761222/git/o3k/ --include="*.go"`

- [ ] **Step 2: Replace any RunMigrations calls with MigrateUp**

If `cmd/o3k/main.go` or any other file calls `database.RunMigrations()`, change to `database.MigrateUp()`.

- [ ] **Step 3: Delete RunMigrations from db.go**

Remove lines 104-124 from `internal/database/db.go` (the `RunMigrations` function).

- [ ] **Step 4: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add internal/database/db.go cmd/o3k/main.go
git commit -m "refactor(database): remove duplicate RunMigrations, use MigrateUp

RunMigrations in db.go duplicated MigrateUp in migrate.go with
slightly different error handling. Consolidated to MigrateUp which
has structured logging and version reporting.

Fixes: M-8"
```

---

### Task 2.4: Fix Placement version inconsistency (H-10)

**Files:**
- Modify: `internal/placement/placement.go:40,59`

- [ ] **Step 1: Define a version constant**

At the top of `internal/placement/placement.go`, add:

```go
const placementMaxVersion = "1.40"
```

- [ ] **Step 2: Replace both hardcoded version strings**

- Line 40: Replace `"max_version": "1.40"` with `"max_version": placementMaxVersion`
- Line 59: Replace `"max_version": "1.39"` with `"max_version": placementMaxVersion`

Also check for `"min_version"` — both should use constants.

- [ ] **Step 3: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/placement/placement.go
git commit -m "fix(placement): use consistent version constant across endpoints

GetVersions returned 1.40 while GetVersion returned 1.39. Both now
use the placementMaxVersion constant.

Fixes: H-10"
```

---

### Task 2.5: Fix Nova version inconsistency (L-4)

**Files:**
- Modify: `internal/nova/handlers.go:191,208`

- [ ] **Step 1: Define version constants at top of file**

```go
const (
	novaMinVersion     = "2.1"
	novaCurrentVersion = "2.90"
)
```

- [ ] **Step 2: Replace hardcoded version strings**

- Line 191 (`ListVersions`): Replace `"version": "2.79"` with `novaCurrentVersion`
- Line 208 (`GetVersion`): Ensure it also uses `novaCurrentVersion`
- Both places should use the same constant

- [ ] **Step 3: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/nova/handlers.go
git commit -m "fix(nova): use consistent version constants across version endpoints

ListVersions returned 2.79 while GetVersion returned 2.90. Both now
use novaCurrentVersion constant.

Fixes: L-4"
```

---

### Task 2.6: Fix GetLimits to query quota table (H-12)

**Files:**
- Modify: `internal/nova/handlers.go:1396-1442`

- [ ] **Step 1: Verify the nova_quotas table schema**

Run: `grep -rn "nova_quotas\|quotas" /Users/I761222/git/o3k/migrations/ --include="*.sql" | head -20`

This tells us the exact column names to query. Adjust the implementation below to match the actual schema.

- [ ] **Step 2: Update GetLimits to query quotas**

Replace the hardcoded values in `GetLimits` with a database query:

```go
func (svc *Service) GetLimits(c *gin.Context) {
	projectID := c.GetString("project_id")

	// Query current usage
	var instancesUsed, coresUsed, ramUsed int
	err := database.DB.QueryRow(c.Request.Context(),
		`SELECT COUNT(*), COALESCE(SUM(vcpus), 0), COALESCE(SUM(memory_mb), 0)
		FROM instances WHERE project_id = $1 AND status != 'DELETED'`,
		projectID,
	).Scan(&instancesUsed, &coresUsed, &ramUsed)
	if err != nil {
		common.HandleDatabaseError(c, err, "limits")
		return
	}

	// Query project quotas (fall back to defaults)
	defaults := map[string]int{
		"instances": 100, "cores": 200, "ram": 512000,
		"keypairs": 100, "server_groups": 10, "server_group_members": 10,
	}

	row := database.DB.QueryRow(c.Request.Context(),
		`SELECT instances, cores, ram, keypairs, server_groups, server_group_members
		FROM nova_quotas WHERE project_id = $1`, projectID)

	var qi, qc, qr, qk, qsg, qsgm int
	if err := row.Scan(&qi, &qc, &qr, &qk, &qsg, &qsgm); err != nil {
		// No custom quota — use defaults
		qi, qc, qr = defaults["instances"], defaults["cores"], defaults["ram"]
		qk, qsg, qsgm = defaults["keypairs"], defaults["server_groups"], defaults["server_group_members"]
	}

	c.JSON(200, gin.H{
		"limits": gin.H{
			"rate": []gin.H{},
			"absolute": gin.H{
				"maxTotalInstances":       qi,
				"maxTotalCores":           qc,
				"maxTotalRAMSize":         qr,
				"maxTotalKeypairs":        qk,
				"maxServerMeta":           128,
				"maxPersonality":          5,
				"maxPersonalitySize":      10240,
				"maxServerGroups":         qsg,
				"maxServerGroupMembers":   qsgm,
				"maxTotalFloatingIps":     10,
				"maxSecurityGroups":       50,
				"maxSecurityGroupRules":   100,
				"maxImageMeta":            128,
				"totalInstancesUsed":      instancesUsed,
				"totalCoresUsed":          coresUsed,
				"totalRAMUsed":            ramUsed,
				"totalFloatingIpsUsed":    0,
				"totalSecurityGroupsUsed": 0,
				"totalServerGroupsUsed":   0,
			},
		},
	})
}
```

- [ ] **Step 3: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Run: `cd /Users/I761222/git/o3k && go test ./test/contract/nova/ -run TestLimits -v -count=1`
Expected: Build passes, limits test passes

- [ ] **Step 4: Commit**

```bash
git add internal/nova/handlers.go
git commit -m "fix(nova): query quota table for GetLimits instead of hardcoded values

GetLimits now reads project-specific quotas from nova_quotas table,
falling back to default values if no custom quota exists. Previously
all projects showed the same hardcoded limits.

Fixes: H-12"
```

---

### Task 2.7: Fix ListFlavorsDetail UUID-based pagination (M-10)

**Files:**
- Modify: `internal/nova/handlers.go:1088-1089`

- [ ] **Step 1: Locate the problematic pagination query**

Run: `grep -n 'id > \$marker\|id > marker' /Users/I761222/git/o3k/internal/nova/handlers.go`

The current pattern uses `id > $marker` which does lexicographic UUID comparison — this produces non-deterministic pagination order because UUIDv4 values are not sequential.

- [ ] **Step 2: Replace with created_at-based cursor pagination**

Change the marker handling in `ListFlavorsDetail` (and `GetFlavors` if it uses the same pattern) from:

```go
// FROM:
query += " AND id > $marker"
// TO (use same pattern as ListServers):
// Look up the marker's created_at, then filter by created_at
var markerTime time.Time
err := database.DB.QueryRow(c.Request.Context(),
	"SELECT created_at FROM flavors WHERE id = $1", marker).Scan(&markerTime)
if err != nil {
	common.SendError(c, common.NewBadRequestError("invalid marker"))
	return
}
query += fmt.Sprintf(" AND created_at < $%d", paramIdx)
args = append(args, markerTime)
```

This matches the pagination pattern used by ListServers and other list endpoints.

- [ ] **Step 3: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Run: `cd /Users/I761222/git/o3k && go test ./test/contract/nova/ -run TestFlavor -v -count=1`
Expected: Build passes, flavor tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/nova/handlers.go
git commit -m "fix(nova): use created_at cursor pagination for flavors instead of UUID ordering

ListFlavorsDetail used lexicographic UUID comparison for marker-based
pagination, which produces non-deterministic order. Now uses
created_at-based cursor consistent with other list endpoints.

Fixes: M-10"
```

---

## Phase 3: Error Framework Migration (C-1, H-2, H-9, SP-1)

**Branch:** `fix/error-framework-migration`
**Parallel-safe:** Per-service (each service can be migrated independently)
**Scope:** Replace 1,176 inline `c.JSON()` error calls with `common.SendError()` calls

This is the highest-leverage fix in the codebase. The error framework already exists and is well-designed — it just needs to be used.

### Strategy

1. First: register the error handling middleware (H-9)
2. Then: migrate service-by-service (keystone → nova → neutron → cinder → glance → placement → metadata)
3. Each service migration is one task with one commit

### Task 3.1: Register ErrorHandlingMiddleware and NotFoundHandler (H-9)

**Files:**
- Modify: `cmd/o3k/main.go` (all 7 `createXxxServer()` functions)

- [ ] **Step 1: Add middleware registration to each server factory**

In each of the 7 `createXxxServer()` functions in `cmd/o3k/main.go`, add after the existing middleware registrations:

```go
r.Use(middleware.ErrorHandlingMiddleware())
r.NoRoute(middleware.NotFoundHandler())
r.HandleMethodNotAllowed = true
r.NoMethod(middleware.MethodNotAllowedHandler())
```

**Verify** that `middleware.NotFoundHandler()` and `middleware.MethodNotAllowedHandler()` exist in `internal/middleware/errors.go`. If `MethodNotAllowedHandler` doesn't exist, create it:

```go
func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		SendError(c, common.NewMethodNotAllowedError(c.Request.Method, c.Request.URL.Path))
	}
}
```

- [ ] **Step 2: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Expected: Clean build

- [ ] **Step 3: Run contract tests to verify no regression**

Run: `cd /Users/I761222/git/o3k && go test ./test/contract/... -v -count=1 2>&1 | tail -20`
Expected: Same pass rate as before

- [ ] **Step 4: Commit**

```bash
git add cmd/o3k/main.go internal/middleware/errors.go
git commit -m "feat(middleware): register error handling middleware and custom 404/405 handlers

Enables ErrorHandlingMiddleware for panic recovery and OpenStack-
formatted error responses. Adds NoRoute and NoMethod handlers so
undefined routes return proper OpenStack error format instead of
Gin defaults.

Fixes: H-9"
```

---

### Task 3.2: Migrate Keystone error responses

**Files:**
- Modify: `internal/keystone/handlers.go`
- Modify: `internal/keystone/auth.go`
- Modify: `internal/keystone/services.go`
- Modify: `internal/middleware/auth.go`

- [ ] **Step 1: Audit all inline error responses in keystone**

Run: `grep -n 'c.JSON(http.Status' /Users/I761222/git/o3k/internal/keystone/*.go | wc -l`

This gives the count of inline responses to migrate.

- [ ] **Step 2: Migrate each inline error to structured error**

Replace patterns like:
```go
// FROM:
c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
// TO:
common.SendError(c, common.NewBadRequestError("invalid request body"))

// FROM:
c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// TO:
log.Error().Err(err).Str("operation", "create_user").Msg("database error")
common.HandleDatabaseError(c, err, "user")

// FROM:
c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
// TO:
common.SendError(c, common.NewNotFoundError("user"))

// FROM:
c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "...", "code": 401, "title": "Unauthorized"}})
// TO:
common.SendError(c, common.NewUnauthorizedError("..."))

// FROM:
c.JSON(http.StatusConflict, gin.H{"error": "resource already exists"})
// TO:
common.SendError(c, common.NewConflictError("resource already exists"))
```

**Critical rules:**
- NEVER include `err.Error()` in the response sent to clients for 500 errors
- Always `log.Error().Err(err)` the real error server-side before sending generic message
- For 4xx errors, the message can be descriptive (it's a client error, not an internal leak)
- Use `HandleDatabaseError()` for all database errors — it handles pgx.ErrNoRows → 404

- [ ] **Step 3: Migrate auth middleware**

`internal/middleware/auth.go` has 5 inline error responses. Replace all with `common.SendError()`.

- [ ] **Step 4: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Run: `cd /Users/I761222/git/o3k && go test ./test/contract/keystone/ -v -count=1`
Expected: Clean build, contract tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/keystone/ internal/middleware/auth.go
git commit -m "refactor(keystone): migrate all error responses to structured error framework

Replaces inline c.JSON error calls with common.SendError/HandleDatabaseError.
All 500 errors now log the real error server-side and return a generic
message to clients. Error responses are now OpenStack-compatible format.

Fixes: C-1 (keystone), H-2 (keystone)"
```

---

### Task 3.3: Migrate Nova error responses

**Files:**
- Modify: `internal/nova/handlers.go`
- Modify: `internal/nova/advanced_actions.go`
- Modify: `internal/nova/console.go`
- Modify: `internal/nova/keypairs.go`
- Modify: `internal/nova/volume_attachment.go`

- [ ] **Step 1: Audit count**

Run: `grep -n 'c.JSON(http.Status' /Users/I761222/git/o3k/internal/nova/*.go | wc -l`

- [ ] **Step 2: Migrate all inline errors to structured errors**

Same replacement patterns as Task 3.2. Nova has the most handlers (~40% of all errors). Work file by file:

1. `handlers.go` (largest — do this first)
2. `advanced_actions.go`
3. `console.go`
4. `keypairs.go`
5. `volume_attachment.go`

- [ ] **Step 3: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Run: `cd /Users/I761222/git/o3k && go test ./test/contract/nova/ -v -count=1`
Expected: Clean build, contract tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/nova/
git commit -m "refactor(nova): migrate all error responses to structured error framework

Replaces ~400 inline c.JSON error calls across 5 handler files.
Internal errors no longer leak database details to clients.

Fixes: C-1 (nova), H-2 (nova)"
```

---

### Task 3.4: Migrate Neutron error responses

**Files:**
- Modify: `internal/neutron/network.go`
- Modify: `internal/neutron/floatingip.go`
- Modify: `internal/neutron/ports.go`
- Modify: `internal/neutron/router.go`
- Modify: `internal/neutron/auto_allocated_topology.go`

- [ ] **Step 1: Audit and migrate**

Same pattern as Tasks 3.2/3.3.

- [ ] **Step 2: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Run: `cd /Users/I761222/git/o3k && go test ./test/contract/neutron/ -v -count=1`

- [ ] **Step 3: Commit**

```bash
git add internal/neutron/
git commit -m "refactor(neutron): migrate all error responses to structured error framework

Fixes: C-1 (neutron), H-2 (neutron)"
```

---

### Task 3.5: Migrate Cinder error responses

**Files:**
- Modify: `internal/cinder/volumes.go`
- Modify: `internal/cinder/qos_specs.go`

- [ ] **Step 1: Audit and migrate**

Same pattern as previous tasks.

- [ ] **Step 2: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Run: `cd /Users/I761222/git/o3k && go test ./test/contract/cinder/ -v -count=1`

- [ ] **Step 3: Commit**

```bash
git add internal/cinder/
git commit -m "refactor(cinder): migrate all error responses to structured error framework

Fixes: C-1 (cinder), H-2 (cinder)"
```

---

### Task 3.6: Migrate Glance error responses

**Files:**
- Modify: `internal/glance/images.go`

- [ ] **Step 1: Audit and migrate**

Same pattern.

- [ ] **Step 2: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./...`
Run: `cd /Users/I761222/git/o3k && go test ./test/contract/glance/ -v -count=1`

- [ ] **Step 3: Commit**

```bash
git add internal/glance/
git commit -m "refactor(glance): migrate all error responses to structured error framework

Fixes: C-1 (glance), H-2 (glance)"
```

---

### Task 3.7: Migrate Placement and Metadata error responses

**Files:**
- Modify: `internal/placement/placement.go`
- Modify: `internal/metadata/service.go`

- [ ] **Step 1: Audit and migrate**

These are small services with few endpoints.

- [ ] **Step 2: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/placement/ internal/metadata/
git commit -m "refactor(placement,metadata): migrate error responses to structured framework

Fixes: C-1 (placement, metadata), H-2 (placement, metadata)"
```

---

### Task 3.8: Verify zero inline error responses remain

- [ ] **Step 1: Run grep to count remaining inline errors**

Run: `grep -rn 'c\.JSON(http\.Status.*gin\.H{"error"' /Users/I761222/git/o3k/internal/ | wc -l`
Expected: 0

Run: `grep -rn 'err\.Error()' /Users/I761222/git/o3k/internal/ | grep 'c\.JSON' | wc -l`
Expected: 0

- [ ] **Step 2: Run full contract test suite**

Run: `cd /Users/I761222/git/o3k && go test ./test/contract/... -v -count=1 2>&1 | tail -30`
Expected: Same or better pass rate than before migration

- [ ] **Step 3: Commit verification note**

No code change needed. If tests pass, the error migration is complete.

---

## Phase 4: Database Transactions (C-2)

**Branch:** `fix/database-transactions`
**Findings:** C-2

### Task 4.1: Add transaction helper

**Files:**
- Create: `internal/database/tx.go`
- Test: `internal/database/tx_test.go`

- [ ] **Step 1: Write the transaction helper test**

```go
// internal/database/tx_test.go
package database

import "testing"

func TestWithTx_Interface(t *testing.T) {
	// Verify the WithTx function signature exists and compiles
	// Full integration testing requires a live database
	_ = WithTx // reference to prevent "unused" if needed
}
```

- [ ] **Step 2: Implement transaction helper**

```go
// internal/database/tx.go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// WithTx executes fn within a database transaction. If fn returns an error,
// the transaction is rolled back. Otherwise it is committed.
func WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/database/tx.go internal/database/tx_test.go
git commit -m "feat(database): add WithTx transaction helper

Provides a simple fn-based transaction wrapper that handles
begin/commit/rollback lifecycle. Handlers use this to wrap
multi-statement operations.

Fixes: C-2 (infrastructure)"
```

---

### Task 4.2: Wrap Nova multi-statement operations in transactions

**Files:**
- Modify: `internal/nova/handlers.go` (CreateServer, ResetServerMetadata, RebuildServer)

- [ ] **Step 1: Identify multi-statement handlers in Nova**

Run: `grep -n 'database.DB.Exec\|database.DB.QueryRow' /Users/I761222/git/o3k/internal/nova/handlers.go | head -40`

Look for handlers with 2+ database calls that should be atomic.

- [ ] **Step 2: Wrap in database.WithTx**

Example for `ResetServerMetadata` (DELETE then INSERT):

```go
err := database.WithTx(c.Request.Context(), func(tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "DELETE FROM instance_metadata WHERE instance_id = $1", instanceID)
	if err != nil {
		return err
	}
	for key, value := range metadata {
		_, err := tx.Exec(ctx, "INSERT INTO instance_metadata (instance_id, key, value) VALUES ($1, $2, $3)", instanceID, key, value)
		if err != nil {
			return err
		}
	}
	return nil
})
if err != nil {
	common.HandleDatabaseError(c, err, "metadata")
	return
}
```

- [ ] **Step 3: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./... && go test ./test/contract/nova/ -v -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/nova/handlers.go
git commit -m "fix(nova): wrap multi-statement operations in database transactions

CreateServer, ResetServerMetadata, and RebuildServer now use
database.WithTx to ensure atomicity. Partial failures no longer
leave inconsistent state.

Fixes: C-2 (nova)"
```

---

### Task 4.3: Wrap Cinder multi-statement operations in transactions

**Files:**
- Modify: `internal/cinder/volumes.go` (CreateVolume, volume action handlers)

- [ ] **Step 1: Identify and wrap multi-statement Cinder handlers**

Same approach as Task 4.2 for cinder.

- [ ] **Step 2: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./... && go test ./test/contract/cinder/ -v -count=1`

- [ ] **Step 3: Commit**

```bash
git add internal/cinder/volumes.go
git commit -m "fix(cinder): wrap multi-statement operations in database transactions

Fixes: C-2 (cinder)"
```

---

### Task 4.4: Wrap Neutron multi-statement operations in transactions

**Files:**
- Modify: `internal/neutron/network.go` (CreateNetwork + CreateSubnet, security group + rules)
- Modify: `internal/neutron/ports.go`

- [ ] **Step 1: Identify and wrap**

Same approach.

- [ ] **Step 2: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./... && go test ./test/contract/neutron/ -v -count=1`

- [ ] **Step 3: Commit**

```bash
git add internal/neutron/
git commit -m "fix(neutron): wrap multi-statement operations in database transactions

Fixes: C-2 (neutron)"
```

---

## Phase 5: Goroutine Lifecycle & Context (H-7, M-2, SP-5)

**Branch:** `fix/goroutine-lifecycle`
**Findings:** H-7, M-2, SP-5

### Task 5.1: Add WaitGroup to service structs

**Files:**
- Modify: `internal/nova/service.go` (or wherever the Service struct is defined)
- Modify: `internal/cinder/volumes.go` (Service struct)

- [ ] **Step 1: Add sync.WaitGroup and context.CancelFunc to Nova Service**

```go
type Service struct {
	// ... existing fields ...
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}
```

Initialize in constructor:
```go
ctx, cancel := context.WithCancel(context.Background())
svc := &Service{
	// ... existing fields ...
	ctx:    ctx,
	cancel: cancel,
}
```

- [ ] **Step 2: Add shutdown method**

```go
func (svc *Service) Shutdown() {
	svc.cancel()
	svc.wg.Wait()
}
```

- [ ] **Step 3: Update main.go graceful shutdown to call service Shutdown()**

In `cmd/o3k/main.go`, in the shutdown sequence, call each service's `Shutdown()` before the HTTP server stops.

- [ ] **Step 4: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/nova/ internal/cinder/ cmd/o3k/main.go
git commit -m "feat(nova,cinder): add goroutine lifecycle management with WaitGroup

Services now track background goroutines and wait for completion
during shutdown. Adds service-level context for cancellation.

Fixes: H-7 (infrastructure)"
```

---

### Task 5.2: Migrate goroutines to use tracked context

**Files:**
- Modify: `internal/nova/handlers.go` (goroutines at lines ~356, ~991, ~1724)
- Modify: `internal/nova/advanced_actions.go` (goroutines at lines ~54, ~107, ~174, ~321, ~493)
- Modify: `internal/nova/volume_attachment.go` (goroutine at line ~140)
- Modify: `internal/cinder/volumes.go` (goroutines at lines ~223, ~915)

- [ ] **Step 1: Update each goroutine to use WaitGroup and service context**

Replace pattern:
```go
// FROM:
go func() {
	defer func() { if r := recover(); r != nil { ... } }()
	ctx := context.Background()
	time.Sleep(2 * time.Second)
	database.DB.Exec(ctx, "UPDATE ...", ...)
}()

// TO:
svc.wg.Add(1)
go func() {
	defer svc.wg.Done()
	defer func() { if r := recover(); r != nil { ... } }()

	select {
	case <-time.After(2 * time.Second):
		// proceed
	case <-svc.ctx.Done():
		return // shutting down
	}

	ctx, cancel := context.WithTimeout(svc.ctx, 5*time.Second)
	defer cancel()
	database.DB.Exec(ctx, "UPDATE ...", ...)
}()
```

- [ ] **Step 2: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/nova/ internal/cinder/
git commit -m "fix(nova,cinder): track goroutines with WaitGroup, use service context

All 12 background goroutines now register with service WaitGroup
and derive context from service-level cancellable context. Goroutines
respect shutdown signals and no longer use context.Background().

Fixes: H-7, M-2, SP-5"
```

---

### Task 5.3: Fix metadata service context.Background() usage (M-11)

**Files:**
- Modify: `internal/metadata/service.go:286,296,309`

- [ ] **Step 1: Replace context.Background() with request context**

Change all `context.Background()` calls in metadata handlers to `c.Request.Context()`.

- [ ] **Step 2: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/metadata/service.go
git commit -m "fix(metadata): use request context instead of context.Background()

Database queries now respect request cancellation and timeouts.

Fixes: M-11"
```

---

### Task 5.4: Fix keystone auth context.Background() usage

**Files:**
- Modify: `internal/keystone/auth.go:547`

- [ ] **Step 1: Replace context.Background() with appropriate context**

Check what function this is in. If it's inside a request handler, use `c.Request.Context()`. If it's in a background operation, use a service-level context.

- [ ] **Step 2: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/keystone/auth.go
git commit -m "fix(keystone): replace context.Background() with request context

Fixes: M-2 (keystone)"
```

---

## Phase 6: Observability & Logging (M-1, M-4, M-6, SP-4)

**Branch:** `fix/observability`
**Findings:** M-1, M-4, M-6, SP-4

### Task 6.1: Replace stdlib log calls with zerolog (M-1)

**Files:**
- Modify: all files containing `log.Printf` or `log.Print` (30 occurrences)

- [ ] **Step 1: Find all stdlib log calls**

Run: `grep -rn 'log\.Printf\|log\.Print\b\|log\.Println\|log\.Fatal' /Users/I761222/git/o3k/internal/ --include="*.go"`

- [ ] **Step 2: Replace each with zerolog equivalent**

```go
// FROM:
log.Printf("Creating VM: %s", name)
// TO:
log.Info().Str("vm_name", name).Msg("creating VM")

// FROM:
log.Printf("Error: %v", err)
// TO:
log.Error().Err(err).Msg("operation failed")
```

**Make sure** to change the import from `"log"` to `"github.com/rs/zerolog/log"` in each file. Check that no file needs both imports.

- [ ] **Step 3: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/
git commit -m "refactor: replace stdlib log with zerolog across all services

Migrates 30 stdlib log.Printf/Print calls to structured zerolog
equivalents. All logging is now consistent.

Fixes: M-1"
```

---

### Task 6.2: Log scan errors instead of silently continuing (M-4, SP-4)

**Files:**
- Modify: all list handlers across nova, cinder, neutron, glance (~15 instances)

- [ ] **Step 1: Find all silent scan continues**

Run: `grep -n 'Scan.*err.*continue\|err.*Scan.*\n.*continue' /Users/I761222/git/o3k/internal/ -r`

Alternatively search for the pattern with multiline:
Run: `grep -rn 'continue' /Users/I761222/git/o3k/internal/ --include="*.go" -B1 | grep 'Scan'`

- [ ] **Step 2: Add logging to each scan error**

```go
// FROM:
if err := rows.Scan(&id, &name); err != nil {
	continue
}
// TO:
if err := rows.Scan(&id, &name); err != nil {
	log.Warn().Err(err).Msg("failed to scan row")
	continue
}
```

- [ ] **Step 3: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/
git commit -m "fix: log scan errors instead of silently dropping rows

Adds zerolog warnings to ~15 instances of silent scan-and-continue
patterns across all list handlers. Previously, corrupted or unexpected
rows were silently excluded from API responses.

Fixes: M-4, SP-4"
```

---

### Task 6.3: Propagate swallowed errors (M-6)

**Files:**
- 15 instances of `_, _ =` or `// Ignore error` in production code

- [ ] **Step 1: Find all swallowed errors**

Run: `grep -rn '_, _ =\|_ =.*Exec\|// Ignore error\|// ignore error' /Users/I761222/git/o3k/internal/ --include="*.go"`

- [ ] **Step 2: For each, decide: log or propagate**

- If the error is in a critical path (security group init, image deletion): **propagate**
- If the error is in a best-effort cleanup: **log at warn level**

Example for `neutron/network.go:35` (SecurityGroupManager init):
```go
// FROM:
svc.sgManager, _ = networking.NewSecurityGroupManager(...)
// TO:
var err error
svc.sgManager, err = networking.NewSecurityGroupManager(...)
if err != nil {
	log.Warn().Err(err).Msg("failed to initialize security group manager, security groups will not function")
}
```

- [ ] **Step 3: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/
git commit -m "fix: log or propagate previously swallowed errors

Addresses 15 instances where errors were silently discarded.
Critical-path errors are now propagated, best-effort cleanup
errors are logged at warn level.

Fixes: M-6"
```

---

### Task 6.4: Remove unused variables in neutron/nova (M-7)

**Files:**
- Modify: `internal/neutron/auto_allocated_topology.go:19,58,130`
- Modify: `internal/nova/advanced_actions.go:542-543`

- [ ] **Step 1: For each unused variable, either use it or remove the assignment**

- `_ = projectIDParam` → If the param should be used for filtering, use it. If not, remove the line.
- `_ = host` / `_ = blockMigration` → These are likely placeholder params for unimplemented features. If stub mode, add a TODO comment. If truly unused, remove.

- [ ] **Step 2: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/neutron/auto_allocated_topology.go internal/nova/advanced_actions.go
git commit -m "fix: remove unused variable assignments in neutron and nova

Fixes: M-7"
```

---

## Phase 7: CI, Config & Cleanup

**Branch:** `fix/ci-config-cleanup`
**Findings:** H-4, H-5, H-8, H-11, M-9, M-12, M-13, M-15, L-1–L-10

### Task 7.1: Fix Docker EXPOSE port mismatch (M-9)

**Files:**
- Modify: `build/package/Dockerfile:69`

- [ ] **Step 1: Fix the EXPOSE directive**

```dockerfile
# FROM:
EXPOSE 5000 8774 9696 8776 9292 8775
# TO:
EXPOSE 35357 8774 9696 8776 9292 8775
```

- [ ] **Step 2: Commit**

```bash
git add build/package/Dockerfile
git commit -m "fix(docker): expose port 35357 (Keystone) instead of 5000

Port 5000 is not used. Keystone runs on 35357.

Fixes: M-9"
```

---

### Task 7.2: Align PostgreSQL version in Makefile (M-15)

**Files:**
- Modify: `Makefile:121`

- [ ] **Step 1: Change postgres version**

```makefile
# FROM:
postgres:16
# TO:
postgres:18
```

Also add a variable at the top:
```makefile
POSTGRES_VERSION?=18
```

And use it in the docker run command: `postgres:$(POSTGRES_VERSION)`

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "fix(makefile): align PostgreSQL version with docker-compose (18)

Makefile used postgres:16 while docker-compose.yml uses 18.3-alpine.
Now uses a configurable POSTGRES_VERSION variable defaulting to 18.

Fixes: M-15"
```

---

### Task 7.3: Consolidate Cinder duplicate routes (H-8)

**Files:**
- Modify: `internal/cinder/volumes.go:38-166`

- [ ] **Step 1: Identify the canonical route structure**

OpenStack Cinder API uses `/v3/{project_id}/volumes` as canonical. The `/v3/volumes` routes exist for backward compatibility.

- [ ] **Step 2: Refactor to use a shared route registration function**

```go
func (svc *Service) registerVolumeRoutes(group *gin.RouterGroup) {
	group.POST("/volumes", svc.CreateVolume)
	group.GET("/volumes", svc.ListVolumes)
	group.GET("/volumes/detail", svc.ListVolumesDetail)
	group.GET("/volumes/:id", svc.GetVolume)
	// ... etc
}

func (svc *Service) RegisterRoutes(r *gin.Engine) {
	v3 := r.Group("/v3")
	svc.registerVolumeRoutes(v3)

	v3WithProject := r.Group("/v3/:project_id")
	svc.registerVolumeRoutes(v3WithProject)
}
```

This eliminates the duplication while preserving both URL patterns.

- [ ] **Step 3: Run build and contract tests**

Run: `cd /Users/I761222/git/o3k && go build ./... && go test ./test/contract/cinder/ -v -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/cinder/volumes.go
git commit -m "refactor(cinder): deduplicate route registration

Extracts shared route registration helper to serve both /v3/ and
/v3/:project_id/ prefixes without code duplication.

Fixes: H-8"
```

---

### Task 7.4: Fix hardcoded localhost URLs in responses (H-11)

**Files:**
- Modify: `internal/nova/handlers.go` (~8 occurrences)
- Modify: `internal/nova/console.go` (~2 occurrences)
- Modify: `internal/keystone/handlers.go` (~3 occurrences)
- Modify: `internal/keystone/services.go` (~2 occurrences)
- Create: `internal/common/baseurl.go`

- [ ] **Step 1: Create a base URL helper**

```go
// internal/common/baseurl.go
package common

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

// BaseURL returns the external base URL for self-links.
// Uses O3K_ENDPOINT_HOST env var if set, otherwise derives from request.
func BaseURL(c *gin.Context, defaultPort int) string {
	if host := os.Getenv("O3K_ENDPOINT_HOST"); host != "" {
		return fmt.Sprintf("http://%s:%d", host, defaultPort)
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}
```

- [ ] **Step 2: Replace hardcoded localhost URLs**

Run: `grep -rn 'localhost:8774\|localhost:35357\|localhost:9696\|localhost:8776\|localhost:9292' /Users/I761222/git/o3k/internal/ --include="*.go"`

Replace each `http://localhost:PORT` with `common.BaseURL(c, PORT)`.

- [ ] **Step 3: Run build**

Run: `cd /Users/I761222/git/o3k && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/common/baseurl.go internal/nova/ internal/keystone/
git commit -m "fix: derive base URL from request instead of hardcoding localhost

Adds common.BaseURL() that uses O3K_ENDPOINT_HOST env var or
request Host header. Replaces ~15 hardcoded localhost:PORT URLs
in self-links and console URLs.

Fixes: H-11"
```

---

### Task 7.5: Investigate missing migration 042 (H-5)

**Files:**
- Check: `migrations/` directory

- [ ] **Step 1: Check git history for migration 042**

Run: `cd /Users/I761222/git/o3k && git log --all --oneline -- 'migrations/042_*'`

- [ ] **Step 2: If it was deleted, document why. If it was never created, add a placeholder**

If the git log shows it was deleted intentionally, no action needed. If it was accidentally skipped, create an empty migration:

```sql
-- migrations/042_placeholder.up.sql
-- Migration 042 was skipped during development (041 → 043).
-- This placeholder exists to maintain sequential numbering.

-- migrations/042_placeholder.down.sql
-- No-op placeholder
```

- [ ] **Step 3: Commit**

```bash
git add migrations/042_*
git commit -m "docs(migrations): add placeholder for skipped migration 042

Maintains sequential numbering between 041_keystone_credentials
and 043_server_tags.

Fixes: H-5"
```

---

### Task 7.6: Re-enable linting in CI (H-4)

**Files:**
- Modify: `.github/workflows/ci.yml:52-55`

- [ ] **Step 1: Re-enable linting for production code only**

```yaml
- name: Run linters
  run: |
    golangci-lint run ./cmd/... ./internal/... ./pkg/... --timeout=5m
```

This runs linting on production code while excluding test files until they're cleaned up.

- [ ] **Step 2: Verify lint passes locally**

Run: `cd /Users/I761222/git/o3k && golangci-lint run ./cmd/... ./internal/... ./pkg/... --timeout=5m`

If there are failures, fix them first.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: re-enable linting for production code

Runs golangci-lint on cmd/, internal/, and pkg/ directories.
Test files excluded until test cleanup is complete.

Fixes: H-4"
```

---

### Task 7.7: Remove empty E2E test stubs from CI (M-13)

**Files:**
- Modify: `.github/workflows/ci.yml:270-304`

- [ ] **Step 1: Remove or comment out the empty e2e jobs**

Replace the no-op jobs with a comment explaining they'll be added later, or remove them entirely and add a TODO.

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: remove empty E2E test stubs to avoid false confidence

The e2e-fast and e2e-full jobs were no-ops printing 'not yet
implemented'. Removed until actual E2E tests are ready.

Fixes: M-13"
```

---

### Task 7.8: Clean up stale files (L-8, L-6)

**Files:**
- Delete or move: `run_nova_tests_with_fix.sh`
- Evaluate: `internal/database/query_logger.go`

- [ ] **Step 1: Check if run_nova_tests_with_fix.sh is still needed**

Run: `cat /Users/I761222/git/o3k/run_nova_tests_with_fix.sh | head -5`

If it's a one-off debug script, delete it. If useful, move to `test/`.

- [ ] **Step 2: Check if query_logger.go is useful**

If `QueryLogger` is never used anywhere, either wire it into the database connection or delete it.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "chore: remove stale debug script and unused query logger

Fixes: L-6, L-8"
```

---

### Task 7.9: Document development-only defaults in Makefile (M-12)

**Files:**
- Modify: `Makefile:10`

- [ ] **Step 1: Add comment clarifying dev-only defaults**

```makefile
# Development-only defaults. Do NOT use in production.
# Production should set DB_URL via environment variable with sslmode=require.
DB_URL?=postgres://o3k:secret@localhost:5432/o3k?sslmode=disable
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "docs(makefile): document that DB_URL default is development-only

Fixes: M-12"
```

---

### Task 7.10: Add CORS headers for OpenStack-specific headers (L-3)

**Note:** This is already handled in Task 1.3 (CORS middleware update), which adds `OpenStack-API-Version`, `X-OpenStack-Nova-API-Version`, and `Accept` to the allowed headers list, plus `PATCH` to allowed methods.

- [ ] **Step 1: Verify Task 1.3 covered this**

Run: `grep -n 'Allow-Headers' /Users/I761222/git/o3k/internal/middleware/logging.go`
Expected: Shows the expanded header list including OpenStack headers.

No additional commit needed.

---

## Deferred Items (Separate Plans)

These items are too large or require design decisions beyond this plan:

| Finding | Why Deferred | Recommended Next Step |
|---------|-------------|----------------------|
| **H-3/SP-2**: Database dependency injection (665 call sites) | Major refactor touching every handler. Requires interface design, mock strategy, and incremental migration plan | Create separate plan: `database-dependency-injection.md` |
| **H-6**: Rate limiting | Needs design decisions: per-project vs per-IP, token bucket vs sliding window, in-memory vs Redis, per-endpoint configuration | Create design spec first, then plan |
| **H-1**: Unit test coverage gap (6 test files for 31K lines) | Ongoing effort, not a one-time fix. Each phase above adds tests for the code it touches | Track as a continuous improvement initiative |
| **L-1**: TODO comments in storage package | Tracks planned go-ceph migration — leave as documentation | No action needed |
| **L-2**: Dockerfile runs as root | Required for network namespace operations. Document the trade-off | Add comment to Dockerfile |
| **L-5**: Benchmarks not in CI | Nice to have, low priority | Add CI job when stabilized |
| **L-7**: Docker Compose proliferation | Low impact, organize when doing next Docker work | Consolidate during next Docker task |
| **L-9**: Contract test helper duplication | Low impact, organize when touching test infrastructure | Consolidate during next test refactor |
| **L-10**: golangci.yml staleness | Addressed by H-4 (re-enabling linting) | Will surface when linting runs |

---

## Execution Order

```
Phase 1 (Security)  ──┐
Phase 2 (Utilities)  ──┼── Can run in parallel
                       │
Phase 3 (Errors)    ───┤  Depends on Phase 2 (uses common utilities)
                       │
Phase 4 (Transactions)─┤  Independent, can parallel with Phase 3
Phase 5 (Goroutines) ──┤  Independent, can parallel with Phase 3
                       │
Phase 6 (Observability)┤  Best after Phase 3 (error framework in place)
                       │
Phase 7 (Cleanup)   ───┘  Independent, can run anytime
```

**Recommended sequential order for a single executor:** 1 → 2 → 3 → 4 → 5 → 6 → 7

**Recommended parallel order (2 workers):**
- Worker A: Phase 1 → Phase 3 → Phase 6
- Worker B: Phase 2 → Phase 4 → Phase 5 → Phase 7
