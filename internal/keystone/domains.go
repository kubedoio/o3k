package keystone

import (
	"net/http"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListDomains handles GET /v3/domains
func (svc *Service) ListDomains(c *gin.Context) {
	rows, err := database.DB.Query(c.Request.Context(), `
		SELECT id, name, description, enabled
		FROM domains
		ORDER BY name ASC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	domains := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		var description *string
		var enabled bool

		err := rows.Scan(&id, &name, &description, &enabled)
		if err != nil {
			continue
		}

		domain := map[string]interface{}{
			"id":      id.String(),
			"name":    name,
			"enabled": enabled,
		}

		if description != nil {
			domain["description"] = *description
		}

		domains = append(domains, domain)
	}

	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

// CreateDomain handles POST /v3/domains
func (svc *Service) CreateDomain(c *gin.Context) {
	var req struct {
		Domain struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
			Enabled     *bool  `json:"enabled"`
		} `json:"domain" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	domainID := uuid.New()
	enabled := true
	if req.Domain.Enabled != nil {
		enabled = *req.Domain.Enabled
	}

	var description *string
	if req.Domain.Description != "" {
		description = &req.Domain.Description
	}

	_, err := database.DB.Exec(c.Request.Context(), `
		INSERT INTO domains (id, name, description, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, domainID, req.Domain.Name, description, enabled, time.Now(), time.Now())

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Domain already exists"})
		return
	}

	domain := map[string]interface{}{
		"id":      domainID.String(),
		"name":    req.Domain.Name,
		"enabled": enabled,
	}

	if description != nil {
		domain["description"] = *description
	}

	c.JSON(http.StatusCreated, gin.H{"domain": domain})
}

// GetDomain handles GET /v3/domains/:id
func (svc *Service) GetDomain(c *gin.Context) {
	domainID := c.Param("id")

	var id uuid.UUID
	var name string
	var description *string
	var enabled bool

	err := database.DB.QueryRow(c.Request.Context(), `
		SELECT id, name, description, enabled
		FROM domains
		WHERE id = $1
	`, domainID).Scan(&id, &name, &description, &enabled)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	domain := map[string]interface{}{
		"id":      id.String(),
		"name":    name,
		"enabled": enabled,
	}

	if description != nil {
		domain["description"] = *description
	}

	c.JSON(http.StatusOK, gin.H{"domain": domain})
}

// UpdateDomain handles PATCH /v3/domains/:id
func (svc *Service) UpdateDomain(c *gin.Context) {
	domainID := c.Param("id")

	var req struct {
		Domain map[string]interface{} `json:"domain" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Verify domain exists
	var exists bool
	err := database.DB.QueryRow(c.Request.Context(), `
		SELECT EXISTS(SELECT 1 FROM domains WHERE id = $1)
	`, domainID).Scan(&exists)

	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if name, ok := req.Domain["name"].(string); ok {
		updates = append(updates, "name = $"+string(rune('0'+argCount)))
		args = append(args, name)
		argCount++
	}

	if description, ok := req.Domain["description"].(string); ok {
		updates = append(updates, "description = $"+string(rune('0'+argCount)))
		args = append(args, description)
		argCount++
	}

	if enabled, ok := req.Domain["enabled"].(bool); ok {
		updates = append(updates, "enabled = $"+string(rune('0'+argCount)))
		args = append(args, enabled)
		argCount++
	}

	updates = append(updates, "updated_at = $"+string(rune('0'+argCount)))
	args = append(args, time.Now())
	argCount++

	args = append(args, domainID)

	if len(updates) > 1 { // More than just updated_at
		query := "UPDATE domains SET "
		for i, update := range updates {
			if i > 0 {
				query += ", "
			}
			query += update
		}
		query += " WHERE id = $" + string(rune('0'+argCount))

		_, err = database.DB.Exec(c.Request.Context(), query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Fetch updated domain
	var id uuid.UUID
	var name string
	var description *string
	var enabled bool

	err = database.DB.QueryRow(c.Request.Context(), `
		SELECT id, name, description, enabled
		FROM domains
		WHERE id = $1
	`, domainID).Scan(&id, &name, &description, &enabled)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	domain := map[string]interface{}{
		"id":      id.String(),
		"name":    name,
		"enabled": enabled,
	}

	if description != nil {
		domain["description"] = *description
	}

	c.JSON(http.StatusOK, gin.H{"domain": domain})
}

// DeleteDomain handles DELETE /v3/domains/:id
func (svc *Service) DeleteDomain(c *gin.Context) {
	domainID := c.Param("id")

	// Check if domain is enabled
	var enabled bool
	err := database.DB.QueryRow(c.Request.Context(), `
		SELECT enabled FROM domains WHERE id = $1
	`, domainID).Scan(&enabled)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	if enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete enabled domain. Disable it first."})
		return
	}

	result, err := database.DB.Exec(c.Request.Context(),
		"DELETE FROM domains WHERE id = $1",
		domainID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetDomainConfig handles GET /v3/domains/:id/config
func (svc *Service) GetDomainConfig(c *gin.Context) {
	domainID := c.Param("id")

	// Verify domain exists
	var exists bool
	err := database.DB.QueryRow(c.Request.Context(), `
		SELECT EXISTS(SELECT 1 FROM domains WHERE id = $1)
	`, domainID).Scan(&exists)

	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// Return empty config (stub implementation)
	config := map[string]interface{}{
		"identity": map[string]interface{}{},
		"ldap":     map[string]interface{}{},
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}
