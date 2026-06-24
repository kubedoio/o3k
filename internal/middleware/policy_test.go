package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cobaltcore-dev/o3k/internal/keystone"
	"github.com/cobaltcore-dev/o3k/internal/keystone/policy"
	"github.com/gin-gonic/gin"
)

// setRoles is a test helper that injects roles and is_admin into the context
// the same way AuthMiddleware does.
func setRoles(roles []string, isAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("roles", roles)
		c.Set("is_admin", isAdmin)
		c.Next()
	}
}

// resetPolicyEngine swaps in a fresh engine for the duration of a test and
// restores the original on cleanup. Tests that need to load specific rules
// should call this and then keystone.PolicyEngine.LoadPolicy(...).
func resetPolicyEngine(t *testing.T) {
	t.Helper()
	original := keystone.PolicyEngine
	keystone.PolicyEngine = policy.NewEngine()
	t.Cleanup(func() {
		keystone.PolicyEngine = original
	})
}

func TestPolicyMiddleware_AdminRoleAllowsAllMethods(t *testing.T) {
	resetPolicyEngine(t)
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := gin.New()
			r.Use(setRoles([]string{"admin"}, false))
			r.Use(PolicyMiddleware("compute"))
			r.Handle(method, "/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/resource", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("method %s: got %d, want %d", method, w.Code, http.StatusOK)
			}
		})
	}
}

func TestPolicyMiddleware_IsAdminFlagAllowsAllMethods(t *testing.T) {
	resetPolicyEngine(t)
	r := gin.New()
	r.Use(setRoles([]string{}, true)) // is_admin=true, no roles listed
	r.Use(PolicyMiddleware("compute"))
	r.POST("/resource", func(c *gin.Context) { c.Status(http.StatusCreated) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("got %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestPolicyMiddleware_MemberRoleAllowsAllMethods(t *testing.T) {
	resetPolicyEngine(t)
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := gin.New()
			r.Use(setRoles([]string{"member"}, false))
			r.Use(PolicyMiddleware("compute"))
			r.Handle(method, "/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/resource", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("method %s: got %d, want %d", method, w.Code, http.StatusOK)
			}
		})
	}
}

func TestPolicyMiddleware_ReaderRoleAllowsGetHead(t *testing.T) {
	resetPolicyEngine(t)
	tests := []struct {
		method string
		want   int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodHead, http.StatusOK},
		{http.MethodPost, http.StatusForbidden},
		{http.MethodPut, http.StatusForbidden},
		{http.MethodDelete, http.StatusForbidden},
		{http.MethodPatch, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			r := gin.New()
			r.Use(setRoles([]string{"reader"}, false))
			r.Use(PolicyMiddleware("compute"))
			r.Handle(tt.method, "/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, "/resource", nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("method %s: got %d, want %d", tt.method, w.Code, tt.want)
			}
		})
	}
}

func TestPolicyMiddleware_NoRoleBlocksEverything(t *testing.T) {
	resetPolicyEngine(t)
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := gin.New()
			r.Use(setRoles([]string{}, false)) // authenticated but no roles
			r.Use(PolicyMiddleware("compute"))
			r.Handle(method, "/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/resource", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("method %s: got %d, want %d", method, w.Code, http.StatusForbidden)
			}
		})
	}
}

func TestPolicyMiddleware_UnauthenticatedPassThrough(t *testing.T) {
	resetPolicyEngine(t)
	// When roles are not set at all (e.g. version discovery skipped auth),
	// the middleware must not block the request.
	r := gin.New()
	// Deliberately do NOT set roles in context.
	r.Use(PolicyMiddleware("compute"))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want %d (unauthenticated public endpoint must pass through)", w.Code, http.StatusOK)
	}
}

// --- Policy engine integration tests ---

// TestPolicyMiddleware_PolicyRulePermitsRequest verifies that when a policy
// rule is loaded, the engine's verdict overrides the role-based fallback —
// here a "reader" identity is granted POST access via an explicit policy.
func TestPolicyMiddleware_PolicyRulePermitsRequest(t *testing.T) {
	resetPolicyEngine(t)
	keystone.PolicyEngine.LoadPolicy(map[string]string{
		"compute:create": "role:reader",
	})

	r := gin.New()
	r.Use(setRoles([]string{"reader"}, false))
	r.Use(PolicyMiddleware("compute"))
	r.POST("/servers", func(c *gin.Context) { c.Status(http.StatusCreated) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/servers", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("policy rule allowing reader to POST: got %d, want %d", w.Code, http.StatusCreated)
	}
}

// TestPolicyMiddleware_PolicyRuleDeniesRequest verifies that an explicit
// policy rule denies access even when role-based fallback would have
// permitted the request — here "member" is normally permitted, but the
// loaded rule restricts compute:create to admins only.
func TestPolicyMiddleware_PolicyRuleDeniesRequest(t *testing.T) {
	resetPolicyEngine(t)
	keystone.PolicyEngine.LoadPolicy(map[string]string{
		"compute:create": "role:admin",
	})

	r := gin.New()
	r.Use(setRoles([]string{"member"}, false))
	r.Use(PolicyMiddleware("compute"))
	r.POST("/servers", func(c *gin.Context) { c.Status(http.StatusCreated) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/servers", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("policy rule denying member POST: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

// TestPolicyMiddleware_FallbackWhenRuleAbsent verifies that if no rule
// matches the service+method, the middleware falls back to the coarse
// role-based check.
func TestPolicyMiddleware_FallbackWhenRuleAbsent(t *testing.T) {
	resetPolicyEngine(t)
	// Load rules for a different service/method combination only.
	keystone.PolicyEngine.LoadPolicy(map[string]string{
		"network:delete": "role:admin",
	})

	r := gin.New()
	r.Use(setRoles([]string{"member"}, false))
	r.Use(PolicyMiddleware("compute"))
	r.POST("/servers", func(c *gin.Context) { c.Status(http.StatusCreated) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/servers", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("absent rule should fall back to role check: got %d, want %d", w.Code, http.StatusCreated)
	}
}

// TestPolicyMiddleware_OrExpressionPermitsEither verifies the policy engine
// handles disjunction so operators can write rules like "admin or member".
func TestPolicyMiddleware_OrExpressionPermitsEither(t *testing.T) {
	resetPolicyEngine(t)
	keystone.PolicyEngine.LoadPolicy(map[string]string{
		"compute:delete": "role:admin or role:member",
	})

	r := gin.New()
	r.Use(setRoles([]string{"member"}, false))
	r.Use(PolicyMiddleware("compute"))
	r.DELETE("/servers/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/servers/abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OR expression matching member: got %d, want %d", w.Code, http.StatusNoContent)
	}
}

// TestPolicyMiddleware_BuildPolicyRule documents the method→verb mapping.
func TestPolicyMiddleware_BuildPolicyRule(t *testing.T) {
	cases := map[string]string{
		http.MethodGet:    "compute:list",
		http.MethodHead:   "compute:list",
		http.MethodPost:   "compute:create",
		http.MethodPut:    "compute:update",
		http.MethodPatch:  "compute:update",
		http.MethodDelete: "compute:delete",
	}
	for method, want := range cases {
		t.Run(method, func(t *testing.T) {
			if got := buildPolicyRule("compute", method); got != want {
				t.Errorf("buildPolicyRule(compute, %s) = %q, want %q", method, got, want)
			}
		})
	}
}

func TestEnforceProjectScope_AdminBypasses(t *testing.T) {
	r := gin.New()
	r.Use(setRoles([]string{"admin"}, true))
	r.GET("/resource", func(c *gin.Context) {
		if !EnforceProjectScope(c, "other-project") {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("admin should bypass scope check: got %d", w.Code)
	}
}

func TestEnforceProjectScope_ProjectMatch(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("roles", []string{"member"})
		c.Set("is_admin", false)
		c.Set("project_id", "proj-123")
		c.Next()
	})
	r.GET("/resource", func(c *gin.Context) {
		if !EnforceProjectScope(c, "proj-123") {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("same project: got %d, want 200", w.Code)
	}
}

func TestEnforceProjectScope_ProjectMismatch(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("roles", []string{"member"})
		c.Set("is_admin", false)
		c.Set("project_id", "proj-123")
		c.Next()
	})
	r.GET("/resource", func(c *gin.Context) {
		if !EnforceProjectScope(c, "proj-other") {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("cross-project: got %d, want 404", w.Code)
	}
}

// TestPolicyMiddleware_UnscopedTokenOnAuthProjects verifies that /v3/auth/projects
// and /v3/auth/domains pass through PolicyMiddleware even when the token carries
// no roles (unscoped token). Per the OpenStack Identity v3 spec these endpoints
// are explicitly designed for unscoped use.
func TestPolicyMiddleware_UnscopedTokenOnAuthProjects(t *testing.T) {
	for _, path := range []string{"/v3/auth/projects", "/v3/auth/domains"} {
		t.Run(path, func(t *testing.T) {
			r := gin.New()
			// Simulate AuthMiddleware setting empty roles (unscoped token)
			r.Use(func(c *gin.Context) {
				c.Set("roles", []string{})
				c.Set("is_admin", false)
				c.Next()
			})
			r.Use(PolicyMiddleware("identity"))
			r.GET(path, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("unscoped token on %s: got %d, want 200", path, w.Code)
			}
		})
	}
}
