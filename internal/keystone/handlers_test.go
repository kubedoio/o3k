package keystone_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/keystone"
)

func TestListProjectsWithMockDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database.NewTestDB(t)

	authSvc := keystone.NewAuthServiceWithDB(database.DB, "test-secret", 0, nil)
	svc := keystone.NewServiceWithDB(database.DB, authSvc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/v3/projects", nil)
	c.Set("project_id", "test-project")
	c.Set("user_id", "test-user")
	c.Set("roles", "admin")

	svc.ListProjects(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "projects")
}
