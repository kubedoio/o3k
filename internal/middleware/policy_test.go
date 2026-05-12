package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestPolicyMiddleware_AdminRoleAllowsAllMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := gin.New()
			r.Use(setRoles([]string{"admin"}, false))
			r.Use(PolicyMiddleware())
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
	r := gin.New()
	r.Use(setRoles([]string{}, true)) // is_admin=true, no roles listed
	r.Use(PolicyMiddleware())
	r.POST("/resource", func(c *gin.Context) { c.Status(http.StatusCreated) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("got %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestPolicyMiddleware_MemberRoleAllowsAllMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := gin.New()
			r.Use(setRoles([]string{"member"}, false))
			r.Use(PolicyMiddleware())
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
			r.Use(PolicyMiddleware())
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
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := gin.New()
			r.Use(setRoles([]string{}, false)) // authenticated but no roles
			r.Use(PolicyMiddleware())
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
	// When roles are not set at all (e.g. version discovery skipped auth),
	// the middleware must not block the request.
	r := gin.New()
	// Deliberately do NOT set roles in context.
	r.Use(PolicyMiddleware())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want %d (unauthenticated public endpoint must pass through)", w.Code, http.StatusOK)
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
