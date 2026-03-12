package nova_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNovaRestoreInstance_Contract tests POST /v2.1/servers/:id/action (restore)
func TestNovaRestoreInstance_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create and soft-delete a server
	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "restore-test",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)

	// Soft delete
	servers.Delete(client, server.ID)

	// Restore the instance
	actionBody := `{
		"restore": null
	}`

	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Restore should succeed or return not found (if hard deleted)
	assert.Contains(t, []int{http.StatusAccepted, http.StatusNotFound}, resp.StatusCode)

	// Cleanup
	servers.ForceDelete(client, server.ID)
}

// TestNovaCreateBackup_Contract tests POST /v2.1/servers/:id/action (createBackup)
func TestNovaCreateBackup_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "backup-test",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Create backup
	actionBody := `{
		"createBackup": {
			"name": "test-backup",
			"backup_type": "daily",
			"rotation": 3
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

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	// Response may contain image ID in Location header
	location := resp.Header.Get("Location")
	if location != "" {
		assert.Contains(t, location, "/images/")
	}
}

// TestNovaResetState_Contract tests POST /v2.1/servers/:id/action (os-resetState)
func TestNovaResetState_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "reset-state-test",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

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

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// TestNovaResetNetwork_Contract tests POST /v2.1/servers/:id/action (os-resetNetwork)
func TestNovaResetNetwork_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "reset-network-test",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Reset network
	actionBody := `{
		"os-resetNetwork": null
	}`

	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}
