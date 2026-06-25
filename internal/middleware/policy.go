package middleware

import (
	"strings"

	"github.com/cobaltcore-dev/o3k/internal/common"
	"github.com/cobaltcore-dev/o3k/internal/keystone"
	"github.com/gin-gonic/gin"
)

// PolicyMiddleware enforces RBAC for a given service. It must run AFTER
// AuthMiddleware so that roles, is_admin, project_id, and user_id are set on
// the gin context.
//
// The middleware queries the keystone policy engine first using a rule name
// derived from the service and request method (e.g. "compute:create" for a
// POST to a Nova endpoint). When a matching policy rule exists, the engine's
// verdict is authoritative — it can permit or deny based on roles, project
// scope, or user identity per the rule expression.
//
// When no matching policy rule is loaded, the middleware falls back to the
// coarse role-based check below. This preserves existing behaviour for
// deployments that have not authored custom policies, while allowing
// operators to tighten enforcement by inserting policies via
// POST /v3/policies (Keystone) without code changes.
//
// Fallback rules:
//   - admin (is_admin=true or role "admin"): all methods allowed
//   - member / _member_: all methods allowed (full project-scoped access)
//   - reader: GET and HEAD only
//   - no recognised role: denied
//
// Routes that were not authenticated (e.g. version discovery, token issuance)
// do not reach this middleware because AuthMiddleware aborts or returns
// before setting roles.  When roles are absent we let the request through so
// public endpoints continue to work.
//
// serviceName is the OpenStack service tag ("identity", "compute", "network",
// "block-storage", "image", "placement"). It is used as the prefix when
// looking up policy rules.
func PolicyMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// These endpoints are callable with unscoped tokens — skip role check.
		path := c.Request.URL.Path
		if path == "/v3/auth/projects" || path == "/v3/auth/domains" {
			c.Next()
			return
		}
		// /v3/users/:id/projects is also callable with an unscoped token
		// (used by Horizon to list projects for the authenticated user).
		if strings.HasSuffix(path, "/projects") && strings.Contains(path, "/users/") {
			c.Next()
			return
		}

		rolesRaw, exists := c.Get("roles")
		if !exists {
			// No roles set — request was not authenticated (public endpoint).
			c.Next()
			return
		}

		roles, ok := rolesRaw.([]string)
		if !ok {
			common.AbortWithError(c, common.NewForbiddenError("insufficient permissions"))
			return
		}

		isAdmin := false
		if v, _ := c.Get("is_admin"); v != nil {
			if b, ok := v.(bool); ok {
				isAdmin = b
			}
		}

		// 1. Try the policy engine first.
		if keystone.PolicyEngine != nil && serviceName != "" {
			rule := buildPolicyRule(serviceName, c.Request.Method)
			creds := map[string]interface{}{
				"roles":      roles,
				"is_admin":   isAdmin,
				"project_id": c.GetString("project_id"),
				"user_id":    c.GetString("user_id"),
			}
			target := map[string]interface{}{
				"project_id": c.GetString("project_id"),
				"user_id":    c.GetString("user_id"),
			}

			if hasPolicyRule(rule) {
				if keystone.PolicyEngine.Enforce(rule, target, creds) {
					c.Next()
					return
				}
				common.AbortWithError(c, common.NewForbiddenError("denied by policy rule "+rule))
				return
			}
		}

		// 2. Fallback: coarse role-based check.
		if isAdmin {
			c.Next()
			return
		}

		for _, r := range roles {
			switch r {
			case "admin":
				c.Next()
				return
			case "member", "_member_":
				c.Next()
				return
			case "reader":
				if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
					c.Next()
					return
				}
				common.AbortWithError(c, common.NewForbiddenError("reader role cannot perform write operations"))
				return
			}
		}

		// No recognised role.
		common.AbortWithError(c, common.NewForbiddenError("insufficient permissions"))
	}
}

// buildPolicyRule returns the policy rule name for a service+method pair.
// Convention: <service>:<verb> where verb is derived from HTTP method.
//   - GET, HEAD     → list
//   - POST          → create
//   - PUT, PATCH    → update
//   - DELETE        → delete
//
// Operators can author rules at any granularity; if a coarser rule like
// "compute:create" is not defined the middleware falls back to role checks.
func buildPolicyRule(service, method string) string {
	var verb string
	switch method {
	case "GET", "HEAD":
		verb = "list"
	case "POST":
		verb = "create"
	case "PUT", "PATCH":
		verb = "update"
	case "DELETE":
		verb = "delete"
	default:
		verb = "default"
	}
	return service + ":" + verb
}

// hasPolicyRule returns true when the given rule exists in the loaded policy
// set. We expose this as an indirection so the engine can be replaced or
// stubbed in tests without touching the middleware.
func hasPolicyRule(rule string) bool {
	if keystone.PolicyEngine == nil {
		return false
	}
	return keystone.PolicyEngine.HasRule(rule)
}

// EnforceProjectScope verifies that resourceProjectID matches the caller's
// project.  Admins bypass the check.  On mismatch it responds with 404 (same
// as OpenStack — cross-project resources appear not to exist) and returns
// false.  Callers must return immediately when false is returned.
func EnforceProjectScope(c *gin.Context, resourceProjectID string) bool {
	isAdmin, _ := c.Get("is_admin")
	if adminBool, ok := isAdmin.(bool); ok && adminBool {
		return true
	}

	// Also accept admin via roles slice.
	if rolesRaw, exists := c.Get("roles"); exists {
		if roles, ok := rolesRaw.([]string); ok {
			for _, r := range roles {
				if r == "admin" {
					return true
				}
			}
		}
	}

	callerProject := c.GetString("project_id")
	if resourceProjectID != callerProject {
		common.AbortWithError(c, common.NewNotFoundError("Resource"))
		return false
	}
	return true
}
