package nova_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/nova"
	"github.com/cobaltcore-dev/o3k/internal/tunnel"
)

// novaGinContext builds a gin context with auth pre-set.
func novaGinContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("project_id", "test-project")
	c.Set("user_id", "test-user")
	c.Set("roles", []string{"member"})
	return c, w
}

// createServerBody returns a minimal valid server-create JSON body.
func createServerBody(name string) string {
	return fmt.Sprintf(`{"server":{"name":%q,"flavorRef":"flavor-m1-small","imageRef":"img-cirros"}}`, name)
}

func insertFlavor(t *testing.T, db *sql.DB, id, name string, vcpus, ram, disk int) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		database.Q(`INSERT INTO flavors (id, name, vcpus, ram_mb, disk_gb, is_public)
			VALUES ($1, $2, $3, $4, $5, $6)`),
		id, name, vcpus, ram, disk, true,
	)
	require.NoError(t, err)
}

func insertImage(t *testing.T, db *sql.DB, id, name, projectID string) {
	t.Helper()
	insertImageWithStatusFormat(t, db, id, name, projectID, "active", "qcow2")
}

func insertImageWithStatusFormat(t *testing.T, db *sql.DB, id, name, projectID, status, diskFormat string) {
	t.Helper()
	nowStr := time.Now().Format(time.RFC3339)
	_, err := db.ExecContext(context.Background(),
		database.Q(`INSERT INTO images (id, name, project_id, status, visibility, disk_format, container_format, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'public', $5, 'bare', $6, $7)`),
		id, name, projectID, status, diskFormat, nowStr, nowStr,
	)
	require.NoError(t, err)
}

func insertInstance(t *testing.T, db *sql.DB, id, name, projectID, userID, flavorID, status string) {
	t.Helper()
	nowStr := time.Now().Format(time.RFC3339)
	_, err := db.ExecContext(context.Background(),
		database.Q(`INSERT INTO instances (id, name, project_id, user_id, flavor_id, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`),
		id, name, projectID, userID, flavorID, status, nowStr, nowStr,
	)
	require.NoError(t, err)
}

func insertQuota(t *testing.T, db *sql.DB, projectID, resource string, limit int) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		database.Q(`INSERT INTO quotas (id, project_id, resource, hard_limit)
			VALUES ($1, $2, $3, $4)`),
		fmt.Sprintf("quota-%s-%s", projectID, resource), projectID, resource, limit,
	)
	require.NoError(t, err)
}

func insertNetwork(t *testing.T, db *sql.DB, id, name, projectID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		database.Q(`INSERT INTO networks (id, name, project_id)
			VALUES ($1, $2, $3)`),
		id, name, projectID,
	)
	require.NoError(t, err)
}

func insertPort(t *testing.T, db *sql.DB, id, networkID, projectID, deviceID, fixedIPs string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		database.Q(`INSERT INTO ports (id, network_id, project_id, device_id, fixed_ips, mac_address)
			VALUES ($1, $2, $3, $4, $5, $6)`),
		id, networkID, projectID, deviceID, fixedIPs, "aa:bb:cc:dd:ee:ff",
	)
	require.NoError(t, err)
}

// TestCreateServerReturnsRequiredFields verifies that the server creation
// response contains all fields required by Terraform.
func TestCreateServerReturnsRequiredFields(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	insertImage(t, db, "img-cirros", "cirros", "test-project")
	svc := nova.NewServiceWithDB(db, "stub")
	c, w := novaGinContext(t, http.MethodPost, "/v2.1/servers", createServerBody("tf-test-vm"))

	svc.CreateServer(c)
	require.Equal(t, http.StatusAccepted, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	server, ok := resp["server"].(map[string]interface{})
	require.True(t, ok)

	for _, field := range []string{"id", "name", "status", "tenant_id", "user_id", "created", "updated", "flavor", "image", "links"} {
		assert.Contains(t, server, field, "response must include field %q", field)
	}
	assert.Equal(t, "BUILD", server["status"])
	assert.NotEmpty(t, server["id"])

	links, ok := server["links"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, links)
}

func TestCreateServerRejectsMissingImage(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	svc := nova.NewServiceWithDB(db, "stub")

	c, w := novaGinContext(t, http.MethodPost, "/v2.1/servers", createServerBody("missing-image-vm"))
	svc.CreateServer(c)

	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
	var count int
	require.NoError(t, db.QueryRowContext(context.Background(), database.Q("SELECT COUNT(*) FROM instances")).Scan(&count))
	assert.Zero(t, count, "server row must not be created when the image is missing")
}

func TestCreateServerRejectsInactiveImage(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	insertImageWithStatusFormat(t, db, "img-cirros", "cirros", "test-project", "queued", "qcow2")
	svc := nova.NewServiceWithDB(db, "stub")

	c, w := novaGinContext(t, http.MethodPost, "/v2.1/servers", createServerBody("inactive-image-vm"))
	svc.CreateServer(c)

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	var count int
	require.NoError(t, db.QueryRowContext(context.Background(), database.Q("SELECT COUNT(*) FROM instances")).Scan(&count))
	assert.Zero(t, count, "server row must not be created when the image is not active")
}

func TestCreateServerTaskPayloadUsesResolvedImageFormat(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	insertImageWithStatusFormat(t, db, "img-raw", "img-cirros", "test-project", "active", "raw")
	svc := nova.NewServiceWithDB(db, "stub")
	svc.SetDispatcher(tunnel.NewHub("test-secret"))

	c, w := novaGinContext(t, http.MethodPost, "/v2.1/servers", createServerBody("raw-image-vm"))
	svc.CreateServer(c)
	require.Equal(t, http.StatusAccepted, w.Code, "body: %s", w.Body.String())

	var payloadText string
	require.NoError(t, db.QueryRowContext(context.Background(), database.Q("SELECT payload FROM tasks WHERE type = 'VM_CREATE'")).Scan(&payloadText))
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(payloadText), &payload))
	assert.Equal(t, "/var/lib/o3k/images/img-raw.raw", payload["image_local_path"])
	assert.Equal(t, "raw", payload["image_format"])
}

// TestListServersDetailPagination verifies that listing with limit=2 returns
// exactly 2 entries plus a next-page link when the database has 5 servers.
func TestListServersDetailPagination(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	for i := range 5 {
		insertInstance(t, db,
			fmt.Sprintf("inst-%03d", i),
			fmt.Sprintf("pag-vm-%d", i),
			"test-project", "test-user", "flavor-m1-small", "ACTIVE",
		)
	}
	svc := nova.NewServiceWithDB(db, "stub")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v2.1/servers/detail?limit=2", nil)
	c.Set("project_id", "test-project")
	c.Set("user_id", "test-user")
	c.Set("roles", []string{"member"})

	svc.ListServersDetail(c)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	servers, ok := resp["servers"].([]interface{})
	require.True(t, ok)
	assert.Len(t, servers, 2, "expected exactly 2 servers with limit=2")
	assert.Contains(t, resp, "servers_links", "pagination link must be present when result hits the limit")
}

// TestListServersFilterByStatus verifies that ?status=ACTIVE filters correctly.
func TestListServersFilterByStatus(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	insertInstance(t, db, "inst-active", "active-vm", "test-project", "test-user", "flavor-m1-small", "ACTIVE")
	insertInstance(t, db, "inst-build", "build-vm", "test-project", "test-user", "flavor-m1-small", "BUILD")

	svc := nova.NewServiceWithDB(db, "stub")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v2.1/servers?status=ACTIVE", nil)
	c.Set("project_id", "test-project")
	c.Set("user_id", "test-user")
	c.Set("roles", []string{"member"})

	svc.ListServers(c)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	servers, ok := resp["servers"].([]interface{})
	require.True(t, ok)

	require.Len(t, servers, 1, "expected exactly 1 ACTIVE server")
	server := servers[0].(map[string]interface{})
	assert.Equal(t, "inst-active", server["id"])
}

// TestGetServerIncludesAddresses verifies that the server detail response
// includes an "addresses" key populated from the ports table.
func TestGetServerIncludesAddresses(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	insertNetwork(t, db, "net-1", "test-net", "test-project")
	insertInstance(t, db, "inst-addr", "addr-vm", "test-project", "test-user", "flavor-m1-small", "ACTIVE")
	insertPort(t, db, "port-1", "net-1", "test-project", "inst-addr", `[{"ip_address":"10.0.0.2"}]`)

	svc := nova.NewServiceWithDB(db, "stub")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v2.1/servers/detail", nil)
	c.Set("project_id", "test-project")
	c.Set("user_id", "test-user")
	c.Set("roles", []string{"member"})

	svc.ListServersDetail(c)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	servers, ok := resp["servers"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, servers)
	server := servers[0].(map[string]interface{})
	assert.Contains(t, server, "addresses", "server detail must include 'addresses' key")
	addresses, ok := server["addresses"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, addresses, "test-net")
}

// TestQuotaBlocksExcessCreation verifies that creating a server when the
// instance quota is already at its limit returns 413.
func TestQuotaBlocksExcessCreation(t *testing.T) {
	db := database.NewTestDB(t)
	insertFlavor(t, db, "flavor-m1-small", "test.small", 1, 512, 10)
	insertImage(t, db, "img-cirros", "cirros", "test-project")
	insertQuota(t, db, "test-project", "instances", 1)
	insertInstance(t, db, "inst-existing", "existing-vm", "test-project", "test-user", "flavor-m1-small", "ACTIVE")

	svc := nova.NewServiceWithDB(db, "stub")

	c, w := novaGinContext(t, http.MethodPost, "/v2.1/servers", createServerBody("quota-vm"))
	svc.CreateServer(c)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code,
		"expected 413 when quota exceeded; body: %s", w.Body.String())
}
