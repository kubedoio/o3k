package middleware

import (
	"github.com/cobaltcore-dev/o3k/internal/common"
	"github.com/gin-gonic/gin"
)

// PolicyMiddleware enforces role-based access control for a given service.
// It must run after AuthMiddleware so that roles and is_admin are set on
// the context.
//
// Rules:
//   - admin (is_admin=true or role "admin"): all methods allowed
//   - member / _member_: all methods allowed (full project-scoped access)
//   - reader: GET and HEAD only
//   - no recognised role: denied
//
// Routes that were not authenticated (e.g. version discovery, token issuance)
// never reach this middleware because AuthMiddleware calls c.Abort() first or
// returns early before setting roles.  When roles are absent we let the
// request through so those public endpoints continue to work.
func PolicyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
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

		isAdmin, _ := c.Get("is_admin")
		if adminBool, ok := isAdmin.(bool); ok && adminBool {
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
