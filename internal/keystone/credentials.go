package keystone

import (
	"net/http"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ListCredentials returns all credentials
func (svc *Service) ListCredentials(c *gin.Context) {
	rows, err := database.DB.Query(c.Request.Context(), `
		SELECT id, user_id, project_id, type, blob, created_at
		FROM credentials
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query credentials"})
		return
	}
	defer rows.Close()

	credentials := []map[string]interface{}{}
	for rows.Next() {
		var id, userID, credType, blob string
		var projectID *string
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &projectID, &credType, &blob, &createdAt); err != nil {
			continue
		}

		credential := map[string]interface{}{
			"id":      id,
			"user_id": userID,
			"type":    credType,
			"blob":    blob,
		}

		if projectID != nil {
			credential["project_id"] = *projectID
		}

		credentials = append(credentials, credential)
	}

	c.JSON(http.StatusOK, gin.H{"credentials": credentials})
}

// CreateCredential creates a new credential
func (svc *Service) CreateCredential(c *gin.Context) {
	var req struct {
		Credential struct {
			UserID    string `json:"user_id" binding:"required"`
			ProjectID string `json:"project_id"`
			Type      string `json:"type" binding:"required"`
			Blob      string `json:"blob" binding:"required"`
		} `json:"credential"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	credID := uuid.New()
	now := time.Now()

	var projectID interface{}
	if req.Credential.ProjectID != "" {
		projectID = req.Credential.ProjectID
	}

	_, err := database.DB.Exec(c.Request.Context(), `
		INSERT INTO credentials (id, user_id, project_id, type, blob, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, credID, req.Credential.UserID, projectID, req.Credential.Type, req.Credential.Blob, now, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create credential"})
		return
	}

	credential := map[string]interface{}{
		"id":      credID.String(),
		"user_id": req.Credential.UserID,
		"type":    req.Credential.Type,
		"blob":    req.Credential.Blob,
	}

	if req.Credential.ProjectID != "" {
		credential["project_id"] = req.Credential.ProjectID
	}

	c.JSON(http.StatusCreated, gin.H{"credential": credential})
}

// GetCredential returns a specific credential by ID
func (svc *Service) GetCredential(c *gin.Context) {
	credID := c.Param("id")

	var id, userID, credType, blob string
	var projectID *string

	err := database.DB.QueryRow(c.Request.Context(), `
		SELECT id, user_id, project_id, type, blob
		FROM credentials
		WHERE id = $1
	`, credID).Scan(&id, &userID, &projectID, &credType, &blob)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query credential"})
		return
	}

	credential := map[string]interface{}{
		"id":      id,
		"user_id": userID,
		"type":    credType,
		"blob":    blob,
	}

	if projectID != nil {
		credential["project_id"] = *projectID
	}

	c.JSON(http.StatusOK, gin.H{"credential": credential})
}

// UpdateCredential updates a credential
func (svc *Service) UpdateCredential(c *gin.Context) {
	credID := c.Param("id")

	var req struct {
		Credential map[string]interface{} `json:"credential"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if blob, ok := req.Credential["blob"].(string); ok {
		updates = append(updates, "blob = $"+string(rune('0'+argCount)))
		args = append(args, blob)
		argCount++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	updates = append(updates, "updated_at = $"+string(rune('0'+argCount)))
	args = append(args, time.Now())
	argCount++

	// Add credential ID as final parameter
	args = append(args, credID)

	query := "UPDATE credentials SET " + updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += " WHERE id = $" + string(rune('0'+argCount))

	result, err := database.DB.Exec(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update credential"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
		return
	}

	// Return updated credential
	svc.GetCredential(c)
}

// DeleteCredential deletes a credential
func (svc *Service) DeleteCredential(c *gin.Context) {
	credID := c.Param("id")

	result, err := database.DB.Exec(c.Request.Context(),
		"DELETE FROM credentials WHERE id = $1",
		credID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete credential"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
