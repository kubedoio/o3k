package cinder

import (
	"net/http"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ListQosSpecs lists all QoS specifications
func (svc *Service) ListQosSpecs(c *gin.Context) {
	rows, err := database.DB.Query(c.Request.Context(), `
		SELECT id, name, consumer, specs, created_at
		FROM qos_specs
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	qosSpecs := []map[string]interface{}{}
	for rows.Next() {
		var (
			id        string
			name      string
			consumer  string
			specs     map[string]string
			createdAt time.Time
		)

		err := rows.Scan(&id, &name, &consumer, &specs, &createdAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		qosSpec := map[string]interface{}{
			"id":       id,
			"name":     name,
			"consumer": consumer,
			"specs":    specs,
		}
		qosSpecs = append(qosSpecs, qosSpec)
	}

	c.JSON(http.StatusOK, gin.H{"qos_specs": qosSpecs})
}

// CreateQosSpec creates a new QoS specification
func (svc *Service) CreateQosSpec(c *gin.Context) {
	var req struct {
		QosSpecs struct {
			Name     string            `json:"name" binding:"required"`
			Consumer string            `json:"consumer"`
			Specs    map[string]string `json:"specs"`
		} `json:"qos_specs" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.QosSpecs.Consumer == "" {
		req.QosSpecs.Consumer = "back-end"
	}

	if req.QosSpecs.Specs == nil {
		req.QosSpecs.Specs = make(map[string]string)
	}

	qosID := uuid.New().String()
	now := time.Now()

	_, err := database.DB.Exec(c.Request.Context(), `
		INSERT INTO qos_specs (id, name, consumer, specs, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, qosID, req.QosSpecs.Name, req.QosSpecs.Consumer, req.QosSpecs.Specs, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"qos_specs": map[string]interface{}{
			"id":       qosID,
			"name":     req.QosSpecs.Name,
			"consumer": req.QosSpecs.Consumer,
			"specs":    req.QosSpecs.Specs,
		},
	})
}

// GetQosSpec retrieves a specific QoS specification
func (svc *Service) GetQosSpec(c *gin.Context) {
	qosID := c.Param("id")

	var (
		name      string
		consumer  string
		specs     map[string]string
		createdAt time.Time
	)

	err := database.DB.QueryRow(c.Request.Context(), `
		SELECT name, consumer, specs, created_at
		FROM qos_specs
		WHERE id = $1
	`, qosID).Scan(&name, &consumer, &specs, &createdAt)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "QoS spec not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"qos_specs": map[string]interface{}{
			"id":       qosID,
			"name":     name,
			"consumer": consumer,
			"specs":    specs,
		},
	})
}

// UpdateQosSpec updates a QoS specification
func (svc *Service) UpdateQosSpec(c *gin.Context) {
	qosID := c.Param("id")

	var req struct {
		QosSpecs map[string]string `json:"qos_specs" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if QoS spec exists and get current specs
	var currentSpecs map[string]string
	err := database.DB.QueryRow(c.Request.Context(),
		"SELECT specs FROM qos_specs WHERE id = $1",
		qosID,
	).Scan(&currentSpecs)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "QoS spec not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Merge specs (update existing keys)
	if currentSpecs == nil {
		currentSpecs = make(map[string]string)
	}
	for k, v := range req.QosSpecs {
		currentSpecs[k] = v
	}

	// Update database
	_, err = database.DB.Exec(c.Request.Context(), `
		UPDATE qos_specs
		SET specs = $1
		WHERE id = $2
	`, currentSpecs, qosID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch updated QoS spec
	var (
		name     string
		consumer string
		specs    map[string]string
	)

	err = database.DB.QueryRow(c.Request.Context(), `
		SELECT name, consumer, specs
		FROM qos_specs
		WHERE id = $1
	`, qosID).Scan(&name, &consumer, &specs)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"qos_specs": map[string]interface{}{
			"id":       qosID,
			"name":     name,
			"consumer": consumer,
			"specs":    specs,
		},
	})
}

// DeleteQosSpec deletes a QoS specification
func (svc *Service) DeleteQosSpec(c *gin.Context) {
	qosID := c.Param("id")

	result, err := database.DB.Exec(c.Request.Context(),
		"DELETE FROM qos_specs WHERE id = $1",
		qosID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "QoS spec not found"})
		return
	}

	c.Status(http.StatusAccepted)
}
