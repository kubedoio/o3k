package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// captureLogs swaps the global zerolog writer for a buffer for the duration of
// the test. Returns the buffer and a restore func — defer the restore to keep
// other tests' logging untouched.
func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := log.Logger
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	return buf, func() { log.Logger = prev }
}

// findAuditEvent scans buffered JSON-lines log output for the first record
// tagged audit_event=true. Returns nil if no such record exists. Tests use
// this rather than asserting on the whole buffer because LoggingMiddleware
// also writes — we need the audit line specifically.
func findAuditEvent(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if v, ok := rec["audit_event"].(bool); ok && v {
			return rec
		}
	}
	return nil
}

// newAuditTestRouter builds a gin engine with AuditMiddleware mounted and
// stubbed auth context (the real AuthMiddleware is exercised in auth_test.go;
// we just need user_id/project_id available the way it would normally be).
func newAuditTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "user-uuid-123")
		c.Set("user_name", "alice")
		c.Set("project_id", "project-uuid-456")
		c.Set("request_id", "req-uuid-789")
		c.Next()
	})
	r.Use(AuditMiddleware())
	r.Any("/v3/*proxyPath", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.Any("/v2.1/*proxyPath", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// TestAuditMiddleware_EmitsCADFOnMutation: the core conformance check. POST
// to a Nova endpoint must produce a CADF event with action=create,
// outcome=success, the right typeURI, and initiator wiring from context.
func TestAuditMiddleware_EmitsCADFOnMutation(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	r := newAuditTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST",
		"/v2.1/project-uuid-456/servers", nil)
	req.RemoteAddr = "10.0.0.42:54321"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	rec := findAuditEvent(t, buf)
	if rec == nil {
		t.Fatalf("no audit_event record in log buffer:\n%s", buf.String())
	}

	checks := map[string]string{
		"eventType":            "activity",
		"action":               "create",
		"outcome":              "success",
		"initiator.id":         "user-uuid-123",
		"initiator.name":       "alice",
		"initiator.project_id": "project-uuid-456",
		"target.typeURI":       "compute/server",
		"observer.id":          "o3k",
		"observer.typeURI":     "service/security",
		"request_id":           "req-uuid-789",
	}
	for k, want := range checks {
		got, _ := rec[k].(string)
		if got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if got, _ := rec["reason.reasonCode"].(float64); int(got) != 200 {
		t.Errorf("reason.reasonCode = %v, want 200", rec["reason.reasonCode"])
	}
	if id, _ := rec["id"].(string); len(id) != 36 {
		t.Errorf("id = %q, want a UUID", id)
	}
}

// TestAuditMiddleware_SkipsReadRequests: GET/HEAD/OPTIONS must NOT emit a
// CADF event. Read traffic dominates volume and SCS audit guidance only
// requires mutations.
func TestAuditMiddleware_SkipsReadRequests(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	r := newAuditTestRouter()
	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/v2.1/project-uuid-456/servers", nil)
		r.ServeHTTP(w, req)
	}

	if rec := findAuditEvent(t, buf); rec != nil {
		t.Errorf("read request emitted audit event: %v", rec)
	}
}

// TestAuditMiddleware_FailureOutcome: non-2xx responses must produce
// outcome=failure with the actual status code in reason.reasonCode.
func TestAuditMiddleware_FailureOutcome(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "u")
		c.Set("project_id", "p")
		c.Next()
	})
	r.Use(AuditMiddleware())
	r.POST("/v3/projects", func(c *gin.Context) { c.Status(http.StatusConflict) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v3/projects", nil)
	r.ServeHTTP(w, req)

	rec := findAuditEvent(t, buf)
	if rec == nil {
		t.Fatalf("no audit event emitted")
	}
	if got, _ := rec["outcome"].(string); got != "failure" {
		t.Errorf("outcome = %q, want failure", got)
	}
	if got, _ := rec["reason.reasonCode"].(float64); int(got) != 409 {
		t.Errorf("reason.reasonCode = %v, want 409", rec["reason.reasonCode"])
	}
}

// TestAuditMiddleware_TargetIDExtraction: when the path targets a specific
// resource (DELETE /v3/projects/<id>), target.id must contain that UUID.
// When it targets a collection (POST /v3/projects), target.id must be absent.
func TestAuditMiddleware_TargetIDExtraction(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		wantTarget string
		wantHasID  bool
		wantID     string
	}{
		{
			name:       "DELETE with project-scope and resource ID",
			method:     "DELETE",
			path:       "/v2.1/project-uuid-456/servers/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			wantTarget: "compute/server",
			wantHasID:  true,
			wantID:     "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
		{
			name:       "POST collection — no target.id",
			method:     "POST",
			path:       "/v3/projects",
			wantTarget: "identity/project",
			wantHasID:  false,
		},
		{
			name:       "PATCH on Keystone user",
			method:     "PATCH",
			path:       "/v3/users/11111111-2222-3333-4444-555555555555",
			wantTarget: "identity/user",
			wantHasID:  true,
			wantID:     "11111111-2222-3333-4444-555555555555",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf, restore := captureLogs(t)
			defer restore()

			r := newAuditTestRouter()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			r.ServeHTTP(w, req)

			rec := findAuditEvent(t, buf)
			if rec == nil {
				t.Fatalf("no audit event for %s %s", tc.method, tc.path)
			}
			if got, _ := rec["target.typeURI"].(string); got != tc.wantTarget {
				t.Errorf("target.typeURI = %q, want %q", got, tc.wantTarget)
			}
			gotID, hasID := rec["target.id"].(string)
			if hasID != tc.wantHasID {
				t.Errorf("target.id present = %v, want %v (got %q)", hasID, tc.wantHasID, gotID)
			}
			if tc.wantHasID && gotID != tc.wantID {
				t.Errorf("target.id = %q, want %q", gotID, tc.wantID)
			}
		})
	}
}

// TestCADFActionMapping: the HTTP-method to CADF-verb mapping is small but
// load-bearing — wrong action verbs would silently corrupt the audit trail.
func TestCADFActionMapping(t *testing.T) {
	cases := map[string]string{
		"POST":   "create",
		"PUT":    "update",
		"PATCH":  "update",
		"DELETE": "delete",
		"GET":    "",
		"HEAD":   "",
	}
	for method, want := range cases {
		if got := cadfAction(method); got != want {
			t.Errorf("cadfAction(%q) = %q, want %q", method, got, want)
		}
	}
}

// TestTargetTypeURI: the path-to-typeURI mapping covers all five core
// services. Conservative — unknown collections fall through to raw path so we
// never drop an audit event on a new endpoint.
func TestTargetTypeURI(t *testing.T) {
	cases := map[string]string{
		"/v2.1/project-id/servers":                         "compute/server",
		"/v2.1/project-id/flavors":                         "compute/flavor",
		"/v2.1/project-id/os-keypairs":                     "compute/keypair",
		"/v2.0/networks":                                   "network/network",
		"/v2.0/security-groups":                            "network/security_group",
		"/v3/volumes/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee": "block-storage/volume",
		"/v2/images":                                       "image/image",
		"/v3/projects":                                     "identity/project",
		"/v3/auth/tokens":                                  "identity/auth",
		"/placement/resource_providers":                    "placement/resource_provider",
	}
	for path, want := range cases {
		if got := targetTypeURI(path); got != want {
			t.Errorf("targetTypeURI(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestIsUUID: cheap shape check for path-segment classification. Must not
// false-positive on collection names or short hex strings.
func TestIsUUID(t *testing.T) {
	yes := []string{
		"00000000-0000-0000-0000-000000000001",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	}
	no := []string{
		"servers",
		"abc",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeee",  // 35 chars
		"aaaaaaaabbbbccccddddeeeeeeeeeeee0000", // no dashes
	}
	for _, s := range yes {
		if !isUUID(s) {
			t.Errorf("isUUID(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isUUID(s) {
			t.Errorf("isUUID(%q) = true, want false", s)
		}
	}
}
