package cinder

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCinderUpdateReadonlyFlag_Contract tests POST /v3/:project/volumes/:id/action (os-update_readonly_flag)
func TestCinderUpdateReadonlyFlag_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume using raw HTTP to avoid gophercloud time parsing bug
	createBody := `{"volume": {"size": 1, "name": "readonly-test"}}`
	createURL := client.ServiceURL("volumes")
	createReq, _ := http.NewRequest("POST", createURL, strings.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createRespBody, &createResult)
	volumeID := createResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Set readonly flag
	actionBody := `{
		"os-update_readonly_flag": {
			"readonly": true
		}
	}`

	url := client.ServiceURL("volumes", volumeID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// TestCinderSetImageMetadata_Contract tests POST /v3/:project/volumes/:id/action (os-set_image_metadata)
func TestCinderSetImageMetadata_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume using raw HTTP
	createBody := `{"volume": {"size": 1, "name": "image-metadata-test"}}`
	createURL := client.ServiceURL("volumes")
	createReq, _ := http.NewRequest("POST", createURL, strings.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createRespBody, &createResult)
	volumeID := createResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Set image metadata (make bootable)
	actionBody := `{
		"os-set_image_metadata": {
			"metadata": {
				"image_id": "fake-image-id",
				"image_name": "test-image"
			}
		}
	}`

	url := client.ServiceURL("volumes", volumeID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCinderForceDetach_Contract tests POST /v3/:project/volumes/:id/action (os-force_detach)
func TestCinderForceDetach_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume using raw HTTP
	createBody := `{"volume": {"size": 1, "name": "force-detach-test"}}`
	createURL := client.ServiceURL("volumes")
	createReq, _ := http.NewRequest("POST", createURL, strings.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createRespBody, &createResult)
	volumeID := createResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Force detach
	actionBody := `{
		"os-force_detach": {
			"attachment_id": "fake-attachment-id",
			"connector": {}
		}
	}`

	url := client.ServiceURL("volumes", volumeID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// TestCinderResetStatus_Contract tests POST /v3/:project/volumes/:id/action (os-reset_status)
func TestCinderResetStatus_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume using raw HTTP
	createBody := `{"volume": {"size": 1, "name": "reset-status-test"}}`
	createURL := client.ServiceURL("volumes")
	createReq, _ := http.NewRequest("POST", createURL, strings.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createRespBody, &createResult)
	volumeID := createResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Reset status to error (admin operation)
	actionBody := `{
		"os-reset_status": {
			"status": "error"
		}
	}`

	url := client.ServiceURL("volumes", volumeID, "action")
	req, err := http.NewRequest("POST", url, strings.NewReader(actionBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// TestCinderUnsetImageMetadata_Contract tests os-unset_image_metadata action
func TestCinderUnsetImageMetadata_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)
	client := setupCinderClient(t)

	// Create volume
	createBody := `{"volume": {"size": 1, "name": "unset-metadata-test"}}`
	createURL := client.ServiceURL("volumes")
	createReq, _ := http.NewRequest("POST", createURL, strings.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	var createResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.NewDecoder(createResp.Body).Decode(&createResult)
	volumeID := createResult.Volume.ID
	defer ensureVolumeDeleted(client, volumeID)

	// Set image metadata first
	setBody := `{"os-set_image_metadata": {"metadata": {"image_name": "cirros"}}}`
	setURL := client.ServiceURL("volumes", volumeID, "action")
	setReq, _ := http.NewRequest("POST", setURL, strings.NewReader(setBody))
	setReq.Header.Set("X-Auth-Token", client.TokenID)
	setReq.Header.Set("Content-Type", "application/json")

	setResp, _ := http.DefaultClient.Do(setReq)
	require.Equal(t, http.StatusOK, setResp.StatusCode)

	// Unset image metadata
	unsetBody := `{"os-unset_image_metadata": {"key": "image_name"}}`
	unsetURL := client.ServiceURL("volumes", volumeID, "action")
	unsetReq, _ := http.NewRequest("POST", unsetURL, strings.NewReader(unsetBody))
	unsetReq.Header.Set("X-Auth-Token", client.TokenID)
	unsetReq.Header.Set("Content-Type", "application/json")

	unsetResp, err := http.DefaultClient.Do(unsetReq)
	require.NoError(t, err)
	defer unsetResp.Body.Close()

	assert.Equal(t, http.StatusOK, unsetResp.StatusCode)
}

// TestCinderReimageVolume_Contract tests os-reimage action
func TestCinderReimageVolume_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)
	client := setupCinderClient(t)

	// Create volume
	createBody := `{"volume": {"size": 1, "name": "reimage-test"}}`
	createURL := client.ServiceURL("volumes")
	createReq, _ := http.NewRequest("POST", createURL, strings.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	var createResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.NewDecoder(createResp.Body).Decode(&createResult)
	volumeID := createResult.Volume.ID
	defer ensureVolumeDeleted(client, volumeID)

	// Reimage volume
	imageID := "00000000-0000-0000-0000-000000000001" // cirros test image
	reimageBody := fmt.Sprintf(`{"os-reimage": {"image_id": "%s"}}`, imageID)
	url := client.ServiceURL("volumes", volumeID, "action")
	req, _ := http.NewRequest("POST", url, strings.NewReader(reimageBody))
	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	// Verify volume is now bootable (would need GET /volumes/:id endpoint)
	// For now, just verify the action was accepted
}
