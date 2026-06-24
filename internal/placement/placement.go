package placement

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	placementMinVersion = "1.0"
	placementMaxVersion = "1.40"
)

// RegisterRoutes registers Placement routes
func RegisterRoutes(r *gin.RouterGroup) {
	// Version discovery (no auth required)
	r.GET("/", GetVersions)

	// Root versions endpoint
	r.GET("/v1", GetVersion)

	// Placement API v1.0 endpoints (minimal stub for Horizon compatibility)
	// These endpoints return empty results - full implementation not required for basic Horizon functionality
	r.GET("/resource_providers", ListResourceProviders)
	r.GET("/resource_classes", ListResourceClasses)
	r.GET("/traits", ListTraits)
}

// GetVersions returns the root version discovery response
func GetVersions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"versions": []gin.H{
			{
				"id":          "v1.0",
				"status":      "CURRENT",
				"min_version": placementMinVersion,
				"max_version": placementMaxVersion,
				"links": []gin.H{
					{
						"rel":  "self",
						"href": "http://o3k:8778/",
					},
				},
			},
		},
	})
}

// GetVersion returns v1 version information
func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": gin.H{
			"id":          "v1.0",
			"status":      "CURRENT",
			"min_version": placementMinVersion,
			"max_version": placementMaxVersion,
			"links": []gin.H{
				{
					"rel":  "self",
					"href": "http://o3k:8778/",
				},
			},
		},
	})
}

// ListResourceProviders returns empty list (stub)
func ListResourceProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"resource_providers": []gin.H{},
	})
}

// ListResourceClasses returns empty list (stub)
func ListResourceClasses(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"resource_classes": []gin.H{},
	})
}

// ListTraits returns empty list (stub)
func ListTraits(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"traits": []gin.H{},
	})
}
