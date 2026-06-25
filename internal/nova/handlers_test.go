package nova_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/nova"
	"github.com/cobaltcore-dev/o3k/internal/tunnel"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestListFlavorsReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := database.NewTestDB(t)

	svc := nova.NewServiceWithDB(db, "stub")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/v2.1/flavors", nil)
	c.Set("project_id", "test-project")

	svc.ListFlavors(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "flavors")
}

func TestSetDispatcher(t *testing.T) {
	db := database.NewTestDB(t)
	svc := nova.NewServiceWithDB(db, "stub")

	hub := tunnel.NewHub("secret")
	svc.SetDispatcher(hub)
	// No panic — dispatcher is wired
}

func TestSetFlatBridge(t *testing.T) {
	db := database.NewTestDB(t)
	svc := nova.NewServiceWithDB(db, "stub")
	svc.SetFlatBridge("br-o3k")
	assert.Equal(t, "br-o3k", svc.GetFlatBridge())
}
