package middleware

import (
	"strings"

	"github.com/cobaltcore-dev/o3k/internal/common"
	"github.com/cobaltcore-dev/o3k/internal/keystone"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates OpenStack tokens
func AuthMiddleware(authService *keystone.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for version discovery endpoints
		if strings.HasSuffix(c.Request.URL.Path, "/v3") ||
			strings.HasSuffix(c.Request.URL.Path, "/v2.1") ||
			strings.HasSuffix(c.Request.URL.Path, "/v2.0") ||
			c.Request.URL.Path == "/" {
			c.Next()
			return
		}

		// Skip auth for token issuance endpoint
		if c.Request.Method == "POST" && strings.HasSuffix(c.Request.URL.Path, "/auth/tokens") {
			c.Next()
			return
		}

		// Get token from X-Auth-Token header
		token := c.GetHeader("X-Auth-Token")
		if token == "" {
			common.AbortWithError(c, common.NewUnauthorizedError("authentication required"))
			return
		}

		// Validate token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			common.AbortWithError(c, common.NewUnauthorizedError("invalid or expired token"))
			return
		}

		// Store claims in context
		c.Set("user_id", claims.UserID)
		c.Set("user_name", claims.UserName)
		c.Set("project_id", claims.ProjectID)
		c.Set("roles", claims.Roles)

		c.Next()
	}
}

// RequireProjectScope ensures the token is project-scoped
func RequireProjectScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID, exists := c.Get("project_id")
		if !exists || projectID == "" {
			common.AbortWithError(c, common.NewForbiddenError("project-scoped token required"))
			return
		}
		c.Next()
	}
}

// RequireRole ensures the user has a specific role
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("roles")
		if !exists {
			common.AbortWithError(c, common.NewForbiddenError("insufficient privileges"))
			return
		}

		roleList := roles.([]string)
		for _, r := range roleList {
			if r == role || r == "admin" {
				c.Next()
				return
			}
		}

		common.AbortWithError(c, common.NewForbiddenError("insufficient privileges"))
	}
}
