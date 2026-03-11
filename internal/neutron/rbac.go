package neutron

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateRBACPolicy handles POST /v2.0/rbac-policies
func (svc *Service) CreateRBACPolicy(c *gin.Context) {
	projectID := c.GetString("project_id")

	var req struct {
		RBACPolicy struct {
			ObjectType   string `json:"object_type" binding:"required"`
			ObjectID     string `json:"object_id" binding:"required"`
			TargetTenant string `json:"target_tenant" binding:"required"`
			Action       string `json:"action" binding:"required"`
		} `json:"rbac_policy" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policyID := uuid.New().String()
	now := time.Now()

	_, err := database.DB.Exec(c.Request.Context(), `
		INSERT INTO rbac_policies (id, project_id, object_type, object_id, target_tenant, action, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, policyID, projectID, req.RBACPolicy.ObjectType, req.RBACPolicy.ObjectID, req.RBACPolicy.TargetTenant, req.RBACPolicy.Action, now, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"rbac_policy": gin.H{
			"id":            policyID,
			"project_id":    projectID,
			"object_type":   req.RBACPolicy.ObjectType,
			"object_id":     req.RBACPolicy.ObjectID,
			"target_tenant": req.RBACPolicy.TargetTenant,
			"action":        req.RBACPolicy.Action,
		},
	})
}

// ListRBACPolicies handles GET /v2.0/rbac-policies
func (svc *Service) ListRBACPolicies(c *gin.Context) {
	projectID := c.GetString("project_id")

	rows, err := database.DB.Query(c.Request.Context(), `
		SELECT id, project_id, object_type, object_id, target_tenant, action
		FROM rbac_policies
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	policies := []gin.H{}
	for rows.Next() {
		var id, projID, objectType, objectID, targetTenant, action string

		if err := rows.Scan(&id, &projID, &objectType, &objectID, &targetTenant, &action); err != nil {
			continue
		}

		policies = append(policies, gin.H{
			"id":            id,
			"project_id":    projID,
			"object_type":   objectType,
			"object_id":     objectID,
			"target_tenant": targetTenant,
			"action":        action,
		})
	}

	c.JSON(http.StatusOK, gin.H{"rbac_policies": policies})
}

// GetRBACPolicy handles GET /v2.0/rbac-policies/:id
func (svc *Service) GetRBACPolicy(c *gin.Context) {
	policyID := c.Param("id")
	projectID := c.GetString("project_id")

	var projID, objectType, objectID, targetTenant, action string

	err := database.DB.QueryRow(c.Request.Context(), `
		SELECT project_id, object_type, object_id, target_tenant, action
		FROM rbac_policies
		WHERE id = $1 AND project_id = $2
	`, policyID, projectID).Scan(&projID, &objectType, &objectID, &targetTenant, &action)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "RBAC policy not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rbac_policy": gin.H{
			"id":            policyID,
			"project_id":    projID,
			"object_type":   objectType,
			"object_id":     objectID,
			"target_tenant": targetTenant,
			"action":        action,
		},
	})
}

// UpdateRBACPolicy handles PUT /v2.0/rbac-policies/:id
func (svc *Service) UpdateRBACPolicy(c *gin.Context) {
	policyID := c.Param("id")
	projectID := c.GetString("project_id")

	var req struct {
		RBACPolicy struct {
			TargetTenant *string `json:"target_tenant"`
		} `json:"rbac_policy" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build update query
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if req.RBACPolicy.TargetTenant != nil {
		updates = append(updates, fmt.Sprintf("target_tenant = $%d", argPos))
		args = append(args, *req.RBACPolicy.TargetTenant)
		argPos++
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	if len(updates) == 1 { // Only updated_at
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	args = append(args, policyID, projectID)
	query := fmt.Sprintf("UPDATE rbac_policies SET %s WHERE id = $%d AND project_id = $%d",
		strings.Join(updates, ", "), argPos, argPos+1)

	_, err := database.DB.Exec(c.Request.Context(), query, args...)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return updated policy
	svc.GetRBACPolicy(c)
}

// DeleteRBACPolicy handles DELETE /v2.0/rbac-policies/:id
func (svc *Service) DeleteRBACPolicy(c *gin.Context) {
	policyID := c.Param("id")
	projectID := c.GetString("project_id")

	result, err := database.DB.Exec(c.Request.Context(),
		"DELETE FROM rbac_policies WHERE id = $1 AND project_id = $2",
		policyID, projectID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "RBAC policy not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
