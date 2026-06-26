package neutron

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cobaltcore-dev/o3k/internal/database"
)

// neutronTestDB creates an in-memory SQLite database with the minimal schema
// required for port lifecycle tests.
func neutronTestDB(t *testing.T) *sql.DB {
	t.Helper()
	require.NoError(t, database.ConnectSQLite(context.Background(), ":memory:"), "connecting to test SQLite database")
	db := database.DB
	t.Cleanup(database.Close)

	ctx := t.Context()

	// Minimal schema — only the tables the port handlers need.
	stmts := []string{
		`CREATE TABLE networks (
			id TEXT PRIMARY KEY,
			name TEXT,
			project_id TEXT,
			shared INTEGER NOT NULL DEFAULT 0,
			admin_state_up INTEGER,
			status TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE subnets (
			id TEXT PRIMARY KEY,
			name TEXT,
			network_id TEXT,
			project_id TEXT,
			cidr TEXT,
			gateway_ip TEXT,
			ip_version INTEGER DEFAULT 4,
			enable_dhcp INTEGER DEFAULT 1,
			dns_nameservers TEXT DEFAULT '[]',
			allocation_pools TEXT DEFAULT '[]',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE ports (
			id TEXT PRIMARY KEY,
			name TEXT,
			network_id TEXT,
			project_id TEXT,
			subnet_id TEXT,
			device_id TEXT,
			device_owner TEXT,
			mac_address TEXT,
			admin_state_up INTEGER DEFAULT 1,
			status TEXT,
			fixed_ips TEXT,
			allowed_address_pairs TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE security_groups (
			id TEXT PRIMARY KEY,
			name TEXT,
			project_id TEXT,
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE port_security_groups (
			port_id TEXT,
			security_group_id TEXT,
			PRIMARY KEY (port_id, security_group_id)
		)`,
		`CREATE TABLE floating_ips (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			floating_network_id TEXT,
			floating_ip_address TEXT,
			fixed_ip_address TEXT,
			port_id TEXT,
			router_id TEXT,
			status TEXT,
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE ip_allocations (
			subnet_id TEXT,
			ip_address TEXT,
			port_id TEXT,
			PRIMARY KEY (subnet_id, ip_address)
		)`,
	}

	for _, stmt := range stmts {
		_, err := db.ExecContext(ctx, stmt)
		require.NoError(t, err, "creating table: %s", stmt[:30])
	}

	return db
}

// seedNetwork inserts a network + subnet for the test project.
func seedNetwork(t *testing.T, db *sql.DB, networkID, subnetID, cidr, projectID string) {
	t.Helper()
	ctx := t.Context()

	_, err := db.ExecContext(ctx,
		database.Q(`INSERT INTO networks (id, name, project_id, shared, admin_state_up, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 0, 1, 'ACTIVE', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
		networkID, "test-net", projectID,
	)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		database.Q(`INSERT INTO subnets (id, name, network_id, project_id, cidr, gateway_ip, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
		subnetID, "test-subnet", networkID, projectID, cidr, firstIP(cidr),
	)
	require.NoError(t, err)
}

// firstIP returns the .1 address of a CIDR (used as gateway).
func firstIP(cidr string) string {
	ip, ipnet, _ := net.ParseCIDR(cidr)
	_ = ip
	ip = ipnet.IP
	ip[len(ip)-1]++
	return ip.String()
}

// portRequest builds the JSON body for a CreatePort call.
func portRequest(networkID string) string {
	return `{"port":{"network_id":"` + networkID + `"}}`
}

// neutronGinContext builds a gin context with project/user auth pre-set.
func neutronGinContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("project_id", "test-project")
	c.Set("user_id", "test-user")
	c.Set("roles", "member")
	return c, w
}

// TestCreatePortAllocatesIP verifies that creating a port on a subnet with
// a known CIDR results in a fixed_ips entry with a non-empty ip_address.
func TestCreatePortAllocatesIP(t *testing.T) {
	db := neutronTestDB(t)
	const networkID = "net-001"
	const subnetID = "sub-001"
	const cidr = "10.0.0.0/24"
	seedNetwork(t, db, networkID, subnetID, cidr, "test-project")

	svc := NewServiceWithDB(db, "stub", nil)

	c, w := neutronGinContext(t, http.MethodPost, "/v2/ports", portRequest(networkID))
	svc.CreatePort(c)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	port, ok := resp["port"].(map[string]interface{})
	require.True(t, ok)

	fixedIPs, ok := port["fixed_ips"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, fixedIPs, "expected at least one fixed IP")

	first := fixedIPs[0].(map[string]interface{})
	ipAddr, ok := first["ip_address"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, ipAddr, "ip_address must be non-empty")
}

// TestCreatePortRespectsCIDR verifies that the allocated IP falls within the
// subnet's CIDR range.
func TestCreatePortRespectsCIDR(t *testing.T) {
	db := neutronTestDB(t)
	const networkID = "net-002"
	const subnetID = "sub-002"
	const cidr = "192.168.42.0/24"
	seedNetwork(t, db, networkID, subnetID, cidr, "test-project")

	svc := NewServiceWithDB(db, "stub", nil)
	c, w := neutronGinContext(t, http.MethodPost, "/v2/ports", portRequest(networkID))
	svc.CreatePort(c)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	port := resp["port"].(map[string]interface{})
	fixedIPs := port["fixed_ips"].([]interface{})
	require.NotEmpty(t, fixedIPs)

	ipStr := fixedIPs[0].(map[string]interface{})["ip_address"].(string)
	ip := net.ParseIP(ipStr)
	require.NotNil(t, ip, "allocated IP must be parseable")

	_, ipnet, _ := net.ParseCIDR(cidr)
	assert.True(t, ipnet.Contains(ip), "allocated IP %s must be inside CIDR %s", ipStr, cidr)
}

// TestDeleteSubnetWithPortsFails409 verifies that deleting a subnet that has
// at least one port returns 409 Conflict.
func TestDeleteSubnetWithPortsFails409(t *testing.T) {
	db := neutronTestDB(t)
	const networkID = "net-003"
	const subnetID = "sub-003"
	const cidr = "172.16.0.0/24"
	seedNetwork(t, db, networkID, subnetID, cidr, "test-project")

	// Insert a port that references this subnet via subnet_id column.
	ctx := t.Context()
	_, err := db.ExecContext(ctx,
		database.Q(`INSERT INTO ports (id, name, network_id, project_id, subnet_id, mac_address, admin_state_up, status, fixed_ips, allowed_address_pairs, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 1, 'ACTIVE', '[]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
		"port-003", "test-port", networkID, "test-project", subnetID, "fa:16:3e:00:01:02",
	)
	require.NoError(t, err)

	svc := NewServiceWithDB(db, "stub", nil)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/v2/subnets/"+subnetID, nil)
	c.Params = gin.Params{{Key: "id", Value: subnetID}}
	c.Set("project_id", "test-project")

	svc.DeleteSubnet(c)
	assert.Equal(t, http.StatusConflict, w.Code, "expected 409 when ports exist; body: %s", w.Body.String())
}

// TestUpdatePortSecurityGroups verifies that updating a port's security groups
// issues the expected database writes without error.
func TestUpdatePortSecurityGroups(t *testing.T) {
	db := neutronTestDB(t)
	const networkID = "net-004"
	const subnetID = "sub-004"
	const cidr = "10.1.0.0/24"
	seedNetwork(t, db, networkID, subnetID, cidr, "test-project")

	ctx := t.Context()

	// Pre-insert a security group.
	const sgID = "sg-001"
	_, err := db.ExecContext(ctx,
		database.Q(`INSERT INTO security_groups (id, name, project_id, description, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
		sgID, "test-sg", "test-project", "",
	)
	require.NoError(t, err)

	// Create a port first.
	svc := NewServiceWithDB(db, "stub", nil)
	c, w := neutronGinContext(t, http.MethodPost, "/v2/ports", portRequest(networkID))
	svc.CreatePort(c)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var createResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	portID := createResp["port"].(map[string]interface{})["id"].(string)

	// Now update the port with an explicit security group.
	updateBody := `{"port":{"security_groups":["` + sgID + `"]}}`
	wUpd := httptest.NewRecorder()
	cUpd, _ := gin.CreateTestContext(wUpd)
	cUpd.Request, _ = http.NewRequest(http.MethodPut, "/v2/ports/"+portID, strings.NewReader(updateBody))
	cUpd.Request.Header.Set("Content-Type", "application/json")
	cUpd.Params = gin.Params{{Key: "id", Value: portID}}
	cUpd.Set("project_id", "test-project")
	cUpd.Set("user_id", "test-user")
	cUpd.Set("roles", "member")

	svc.UpdatePort(cUpd)

	// 200 or 201 means the update was accepted.
	assert.True(t, wUpd.Code == http.StatusOK || wUpd.Code == http.StatusCreated,
		"expected 200/201 on port update; got %d; body: %s", wUpd.Code, wUpd.Body.String())

	// Confirm the association exists in the DB.
	var count int
	err = db.QueryRowContext(ctx,
		database.Q(`SELECT COUNT(*) FROM port_security_groups WHERE port_id = $1 AND security_group_id = $2`),
		portID, sgID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "security group association should exist after update")
}

// TestFloatingIPAssociation verifies that a floating IP can be created and
// that the response includes the expected fields.
func TestFloatingIPAssociation(t *testing.T) {
	db := neutronTestDB(t)

	ctx := t.Context()
	const extNetID = "ext-net-001"

	// Insert the external network + subnet so allocateFloatingIP can pick an IP.
	_, err := db.ExecContext(ctx,
		database.Q(`INSERT INTO networks (id, name, project_id, shared, admin_state_up, status, created_at, updated_at)
		 VALUES ($1, 'ext-net', 'admin', 1, 1, 'ACTIVE', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
		extNetID,
	)
	require.NoError(t, err)

	const extSubnetID = "ext-sub-001"
	_, err = db.ExecContext(ctx,
		database.Q(`INSERT INTO subnets (id, name, network_id, project_id, cidr, gateway_ip, created_at, updated_at)
		 VALUES ($1, 'ext-subnet', $2, 'admin', '203.0.113.0/24', '203.0.113.1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`),
		extSubnetID, extNetID,
	)
	require.NoError(t, err)

	svc := NewServiceWithDB(db, "stub", nil)

	body := `{"floatingip":{"floating_network_id":"` + extNetID + `"}}`
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v2/floatingips", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("project_id", "test-project")
	c.Set("user_id", "test-user")
	c.Set("roles", "member")

	svc.CreateFloatingIP(c)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	fip, ok := resp["floatingip"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, fip["id"])
	assert.Equal(t, extNetID, fip["floating_network_id"])

	// The allocated address must fall inside the external subnet CIDR.
	fipAddr, ok := fip["floating_ip_address"].(string)
	require.True(t, ok)
	_, extNet, _ := net.ParseCIDR("203.0.113.0/24")
	assert.True(t, extNet.Contains(net.ParseIP(fipAddr)),
		"floating IP %s must be inside external CIDR", fipAddr)
}

func TestDeletePortByID(t *testing.T) {
	db := neutronTestDB(t)
	ctx := t.Context()

	svc := NewServiceWithDB(db, "stub", nil)

	const portID = "port-del-001"
	const projectID = "proj-del"
	_, err := db.ExecContext(ctx, database.Q(`
		INSERT INTO ports (id, name, network_id, project_id, mac_address, status, fixed_ips, allowed_address_pairs, created_at, updated_at)
		VALUES ($1, 'p', 'net-1', $2, 'fa:16:3e:00:00:01', 'DOWN', '[]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`), portID, projectID)
	require.NoError(t, err)

	require.NoError(t, svc.DeletePortByID(ctx, portID, projectID))

	var count int
	require.NoError(t, db.QueryRowContext(ctx, database.Q("SELECT COUNT(*) FROM ports WHERE id = $1"), portID).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestDeletePortByID_WrongProject(t *testing.T) {
	db := neutronTestDB(t)
	ctx := t.Context()

	svc := NewServiceWithDB(db, "stub", nil)

	const portID = "port-del-002"
	_, err := db.ExecContext(ctx, database.Q(`
		INSERT INTO ports (id, name, network_id, project_id, mac_address, status, fixed_ips, allowed_address_pairs, created_at, updated_at)
		VALUES ($1, 'p', 'net-1', 'owner-proj', 'fa:16:3e:00:00:02', 'DOWN', '[]', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`), portID)
	require.NoError(t, err)

	// Different project — should silently do nothing, not error.
	require.NoError(t, svc.DeletePortByID(ctx, portID, "other-proj"))

	var count int
	require.NoError(t, db.QueryRowContext(ctx, database.Q("SELECT COUNT(*) FROM ports WHERE id = $1"), portID).Scan(&count))
	assert.Equal(t, 1, count, "port owned by another project must not be deleted")
}
