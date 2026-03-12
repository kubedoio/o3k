package cinder_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCinderListBackups_Contract tests GET /v3/:project_id/backups
func TestCinderListBackups_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	url := client.Endpoint + "backups"
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Backups []map[string]interface{} `json:"backups"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Backups)
}

// TestCinderCreateBackup_Contract tests POST /v3/:project_id/backups
func TestCinderCreateBackup_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume first
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-backup-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	// Create backup
	payload := map[string]interface{}{
		"backup": map[string]interface{}{
			"volume_id":   volume.ID,
			"name":        "test-backup",
			"description": "Test backup",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.Endpoint + "backups"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Backup map[string]interface{} `json:"backup"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Backup["id"])
	assert.Equal(t, "test-backup", result.Backup["name"])
	assert.Equal(t, volume.ID, result.Backup["volume_id"])

	// Cleanup backup
	backupID := result.Backup["id"].(string)
	delReq, _ := http.NewRequest("DELETE", client.Endpoint+"backups/"+backupID, nil)
	delReq.Header.Set("X-Auth-Token", client.TokenID)
	http.DefaultClient.Do(delReq)
}

// TestCinderGetBackup_Contract tests GET /v3/:project_id/backups/:id
func TestCinderGetBackup_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-backup-get-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	// Create test backup
	createPayload := map[string]interface{}{
		"backup": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-backup-get",
		},
	}

	createBody, _ := json.Marshal(createPayload)
	createReq, _ := http.NewRequest("POST", client.Endpoint+"backups", bytes.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := http.DefaultClient.Do(createReq)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Backup map[string]interface{} `json:"backup"`
	}
	json.Unmarshal(createRespBody, &createResult)
	backupID := createResult.Backup["id"].(string)

	defer func() {
		delReq, _ := http.NewRequest("DELETE", client.Endpoint+"backups/"+backupID, nil)
		delReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(delReq)
	}()

	// Get backup
	url := client.Endpoint + "backups/" + backupID
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Backup map[string]interface{} `json:"backup"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, backupID, result.Backup["id"])
	assert.Equal(t, volume.ID, result.Backup["volume_id"])
}

// TestCinderDeleteBackup_Contract tests DELETE /v3/:project_id/backups/:id
func TestCinderDeleteBackup_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-backup-delete-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	// Create test backup
	createPayload := map[string]interface{}{
		"backup": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-backup-delete",
		},
	}

	createBody, _ := json.Marshal(createPayload)
	createReq, _ := http.NewRequest("POST", client.Endpoint+"backups", bytes.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := http.DefaultClient.Do(createReq)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Backup map[string]interface{} `json:"backup"`
	}
	json.Unmarshal(createRespBody, &createResult)
	backupID := createResult.Backup["id"].(string)

	// Delete backup
	url := client.Endpoint + "backups/" + backupID
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// TestCinderRestoreBackup_Contract tests POST /v3/:project_id/backups/:id/action (restore)
func TestCinderRestoreBackup_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-backup-restore-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	// Create test backup
	createPayload := map[string]interface{}{
		"backup": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-backup-restore",
		},
	}

	createBody, _ := json.Marshal(createPayload)
	createReq, _ := http.NewRequest("POST", client.Endpoint+"backups", bytes.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := http.DefaultClient.Do(createReq)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Backup map[string]interface{} `json:"backup"`
	}
	json.Unmarshal(createRespBody, &createResult)
	backupID := createResult.Backup["id"].(string)

	defer func() {
		delReq, _ := http.NewRequest("DELETE", client.Endpoint+"backups/"+backupID, nil)
		delReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(delReq)
	}()

	// Restore backup
	payload := map[string]interface{}{
		"restore": map[string]interface{}{
			"volume_id": volume.ID,
		},
	}

	body, _ := json.Marshal(payload)
	url := client.Endpoint + "backups/" + backupID + "/action"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Restore map[string]interface{} `json:"restore"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, backupID, result.Restore["backup_id"])
	assert.Equal(t, volume.ID, result.Restore["volume_id"])
}
