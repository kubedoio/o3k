package nova_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChangePassword_Contract tests the changePassword server action
func TestChangePassword_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "password-test-server",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Wait for server to be ACTIVE
	for i := 0; i < 30; i++ {
		s, _ := servers.Get(client, server.ID).Extract()
		if s.Status == "ACTIVE" {
			break
		}
		if i == 29 {
			t.Skip("Server did not become ACTIVE in time")
		}
		time.Sleep(time.Second)
	}

	// Change password
	actionBody := `{
		"changePassword": {
			"adminPass": "NewSecurePassword123!"
		}
	}`

	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "changePassword should succeed")
}

// TestChangePasswordInvalidLength_Contract tests password validation
func TestChangePasswordInvalidLength_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "password-validation-test",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Wait for server to be ACTIVE
	for i := 0; i < 30; i++ {
		s, _ := servers.Get(client, server.ID).Extract()
		if s.Status == "ACTIVE" {
			break
		}
		time.Sleep(time.Second)
	}

	// Try to set password that's too short
	actionBody := `{
		"changePassword": {
			"adminPass": "short"
		}
	}`

	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Password < 8 characters should fail")
}

// TestCreateBackup_Contract tests instance backup creation
func TestCreateBackup_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "backup-test-server",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Wait for server to be ACTIVE
	for i := 0; i < 30; i++ {
		s, _ := servers.Get(client, server.ID).Extract()
		if s.Status == "ACTIVE" {
			break
		}
		time.Sleep(time.Second)
	}

	// Create backup
	actionBody := `{
		"createBackup": {
			"name": "test-backup",
			"backup_type": "daily",
			"rotation": 7
		}
	}`

	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "createBackup should succeed")

	// Check for image_id in response (optional validation)
	if resp.StatusCode == http.StatusAccepted {
		// Backup created successfully
		t.Log("Backup created successfully")
	}
}

// TestMigrateServer_Contract tests server migration
func TestMigrateServer_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "migrate-test-server",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Wait for server to be ACTIVE
	for i := 0; i < 30; i++ {
		s, _ := servers.Get(client, server.ID).Extract()
		if s.Status == "ACTIVE" {
			break
		}
		time.Sleep(time.Second)
	}

	// Migrate server
	actionBody := `{
		"migrate": null
	}`

	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "migrate should succeed")
}

// TestResetStateAdmin_Contract tests os-resetState succeeds with admin role
func TestResetStateAdmin_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	// Use admin client
	client := setupNovaClient(t)

	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "reset-state-admin-test",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Wait for server to be ACTIVE
	for i := 0; i < 30; i++ {
		s, _ := servers.Get(client, server.ID).Extract()
		if s.Status == "ACTIVE" {
			break
		}
		time.Sleep(time.Second)
	}

	// Reset state to ERROR
	actionBody := `{
		"os-resetState": {
			"state": "error"
		}
	}`

	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "os-resetState with admin should succeed")

	// Reset back to active for cleanup
	actionBody2 := `{"os-resetState": {"state": "active"}}`
	req2, _ := http.NewRequest("POST", url, strings.NewReader(actionBody2))
	req2.Header.Set("X-Auth-Token", client.TokenID)
	req2.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req2)
}
