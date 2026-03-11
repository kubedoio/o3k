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

// TestCinderCreateVolumeTransfer_Contract tests POST /v3/:project_id/volume-transfers
func TestCinderCreateVolumeTransfer_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create a test volume first
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-transfer-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	// Test: Create volume transfer
	transfer := map[string]interface{}{
		"transfer": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-transfer",
		},
	}

	body, _ := json.Marshal(transfer)
	url := client.Endpoint + "volume-transfers"
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
		Transfer map[string]interface{} `json:"transfer"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Transfer["id"])
	assert.NotEmpty(t, result.Transfer["auth_key"])
	assert.Equal(t, volume.ID, result.Transfer["volume_id"])

	// Cleanup
	if transferID, ok := result.Transfer["id"].(string); ok {
		delURL := client.Endpoint + "volume-transfers/" + transferID
		delReq, _ := http.NewRequest("DELETE", delURL, nil)
		delReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(delReq)
	}
}

// TestCinderListVolumeTransfers_Contract tests GET /v3/:project_id/volume-transfers
func TestCinderListVolumeTransfers_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create volume and transfer
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-list-transfer-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	transfer := map[string]interface{}{
		"transfer": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-list-transfer",
		},
	}
	transferBody, _ := json.Marshal(transfer)
	createURL := client.Endpoint + "volume-transfers"
	createReq, _ := http.NewRequest("POST", createURL, bytes.NewReader(transferBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(createReq)

	// Test: List volume transfers
	url := client.Endpoint + "volume-transfers"
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Transfers []map[string]interface{} `json:"transfers"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Transfers)
}

// TestCinderGetVolumeTransfer_Contract tests GET /v3/:project_id/volume-transfers/:id
func TestCinderGetVolumeTransfer_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create volume and transfer
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-get-transfer-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	transfer := map[string]interface{}{
		"transfer": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-get-transfer",
		},
	}
	transferBody, _ := json.Marshal(transfer)
	createURL := client.Endpoint + "volume-transfers"
	createReq, _ := http.NewRequest("POST", createURL, bytes.NewReader(transferBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	defer createResp.Body.Close()

	createBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Transfer map[string]interface{} `json:"transfer"`
	}
	json.Unmarshal(createBody, &createResult)
	transferID := createResult.Transfer["id"].(string)

	// Test: Get volume transfer
	url := client.Endpoint + "volume-transfers/" + transferID
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Transfer map[string]interface{} `json:"transfer"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, transferID, result.Transfer["id"])
	assert.Equal(t, volume.ID, result.Transfer["volume_id"])

	// Cleanup
	delReq, _ := http.NewRequest("DELETE", url, nil)
	delReq.Header.Set("X-Auth-Token", client.TokenID)
	http.DefaultClient.Do(delReq)
}

// TestCinderDeleteVolumeTransfer_Contract tests DELETE /v3/:project_id/volume-transfers/:id
func TestCinderDeleteVolumeTransfer_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create volume and transfer
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-delete-transfer-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	transfer := map[string]interface{}{
		"transfer": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-delete-transfer",
		},
	}
	transferBody, _ := json.Marshal(transfer)
	createURL := client.Endpoint + "volume-transfers"
	createReq, _ := http.NewRequest("POST", createURL, bytes.NewReader(transferBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	defer createResp.Body.Close()

	createBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Transfer map[string]interface{} `json:"transfer"`
	}
	json.Unmarshal(createBody, &createResult)
	transferID := createResult.Transfer["id"].(string)

	// Test: Delete volume transfer
	url := client.Endpoint + "volume-transfers/" + transferID
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// TestCinderAcceptVolumeTransfer_Contract tests POST /v3/:project_id/volume-transfers/:id/accept
func TestCinderAcceptVolumeTransfer_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create volume and transfer
	volume, err := volumes.Create(client, volumes.CreateOpts{
		Size: 1,
		Name: "test-accept-transfer-volume",
	}).Extract()
	require.NoError(t, err)
	defer volumes.Delete(client, volume.ID, volumes.DeleteOpts{})

	transfer := map[string]interface{}{
		"transfer": map[string]interface{}{
			"volume_id": volume.ID,
			"name":      "test-accept-transfer",
		},
	}
	transferBody, _ := json.Marshal(transfer)
	createURL := client.Endpoint + "volume-transfers"
	createReq, _ := http.NewRequest("POST", createURL, bytes.NewReader(transferBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	defer createResp.Body.Close()

	createBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Transfer map[string]interface{} `json:"transfer"`
	}
	json.Unmarshal(createBody, &createResult)
	transferID := createResult.Transfer["id"].(string)
	authKey := createResult.Transfer["auth_key"].(string)

	// Test: Accept volume transfer
	accept := map[string]interface{}{
		"accept": map[string]interface{}{
			"auth_key": authKey,
		},
	}
	acceptBody, _ := json.Marshal(accept)
	url := client.Endpoint + "volume-transfers/" + transferID + "/accept"
	req, err := http.NewRequest("POST", url, bytes.NewReader(acceptBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Transfer map[string]interface{} `json:"transfer"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, transferID, result.Transfer["id"])
	assert.Equal(t, volume.ID, result.Transfer["volume_id"])
}
