package cinder_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCinderGetSnapshotMetadata_Contract tests GET /v3/:project/snapshots/:id/metadata
func TestCinderGetSnapshotMetadata_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume first
	createVolBody := `{"volume": {"size": 1, "name": "snap-meta-vol"}}`
	createVolURL := client.ServiceURL("volumes")
	createVolReq, _ := http.NewRequest("POST", createVolURL, strings.NewReader(createVolBody))
	createVolReq.Header.Set("X-Auth-Token", client.TokenID)
	createVolReq.Header.Set("Content-Type", "application/json")

	createVolResp, err := http.DefaultClient.Do(createVolReq)
	require.NoError(t, err)
	defer createVolResp.Body.Close()

	createVolRespBody, _ := io.ReadAll(createVolResp.Body)
	var createVolResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createVolRespBody, &createVolResult)
	volumeID := createVolResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Create snapshot
	createSnapBody := `{"snapshot": {"volume_id": "` + volumeID + `", "name": "test-snap"}}`
	createSnapURL := client.ServiceURL("snapshots")
	createSnapReq, _ := http.NewRequest("POST", createSnapURL, strings.NewReader(createSnapBody))
	createSnapReq.Header.Set("X-Auth-Token", client.TokenID)
	createSnapReq.Header.Set("Content-Type", "application/json")

	createSnapResp, err := http.DefaultClient.Do(createSnapReq)
	require.NoError(t, err)
	defer createSnapResp.Body.Close()

	createSnapRespBody, _ := io.ReadAll(createSnapResp.Body)
	var createSnapResult struct {
		Snapshot struct {
			ID string `json:"id"`
		} `json:"snapshot"`
	}
	json.Unmarshal(createSnapRespBody, &createSnapResult)
	snapshotID := createSnapResult.Snapshot.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("snapshots", snapshotID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Get metadata (should be empty initially)
	url := client.ServiceURL("snapshots", snapshotID, "metadata")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	// Should have metadata object
	assert.NotNil(t, result["metadata"])
}

// TestCinderSetSnapshotMetadataKey_Contract tests PUT /v3/:project/snapshots/:id/metadata/:key
func TestCinderSetSnapshotMetadataKey_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume
	createVolBody := `{"volume": {"size": 1, "name": "snap-meta-key-vol"}}`
	createVolURL := client.ServiceURL("volumes")
	createVolReq, _ := http.NewRequest("POST", createVolURL, strings.NewReader(createVolBody))
	createVolReq.Header.Set("X-Auth-Token", client.TokenID)
	createVolReq.Header.Set("Content-Type", "application/json")

	createVolResp, err := http.DefaultClient.Do(createVolReq)
	require.NoError(t, err)
	defer createVolResp.Body.Close()

	createVolRespBody, _ := io.ReadAll(createVolResp.Body)
	var createVolResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createVolRespBody, &createVolResult)
	volumeID := createVolResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Create snapshot
	createSnapBody := `{"snapshot": {"volume_id": "` + volumeID + `", "name": "test-snap"}}`
	createSnapURL := client.ServiceURL("snapshots")
	createSnapReq, _ := http.NewRequest("POST", createSnapURL, strings.NewReader(createSnapBody))
	createSnapReq.Header.Set("X-Auth-Token", client.TokenID)
	createSnapReq.Header.Set("Content-Type", "application/json")

	createSnapResp, err := http.DefaultClient.Do(createSnapReq)
	require.NoError(t, err)
	defer createSnapResp.Body.Close()

	createSnapRespBody, _ := io.ReadAll(createSnapResp.Body)
	var createSnapResult struct {
		Snapshot struct {
			ID string `json:"id"`
		} `json:"snapshot"`
	}
	json.Unmarshal(createSnapRespBody, &createSnapResult)
	snapshotID := createSnapResult.Snapshot.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("snapshots", snapshotID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Set metadata key
	metadataBody := `{"meta": {"environment": "production"}}`
	url := client.ServiceURL("snapshots", snapshotID, "metadata", "environment")
	req, err := http.NewRequest("PUT", url, strings.NewReader(metadataBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCinderGetSnapshotMetadataKey_Contract tests GET /v3/:project/snapshots/:id/metadata/:key
func TestCinderGetSnapshotMetadataKey_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume
	createVolBody := `{"volume": {"size": 1, "name": "snap-get-key-vol"}}`
	createVolURL := client.ServiceURL("volumes")
	createVolReq, _ := http.NewRequest("POST", createVolURL, strings.NewReader(createVolBody))
	createVolReq.Header.Set("X-Auth-Token", client.TokenID)
	createVolReq.Header.Set("Content-Type", "application/json")

	createVolResp, err := http.DefaultClient.Do(createVolReq)
	require.NoError(t, err)
	defer createVolResp.Body.Close()

	createVolRespBody, _ := io.ReadAll(createVolResp.Body)
	var createVolResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createVolRespBody, &createVolResult)
	volumeID := createVolResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Create snapshot
	createSnapBody := `{"snapshot": {"volume_id": "` + volumeID + `", "name": "test-snap"}}`
	createSnapURL := client.ServiceURL("snapshots")
	createSnapReq, _ := http.NewRequest("POST", createSnapURL, strings.NewReader(createSnapBody))
	createSnapReq.Header.Set("X-Auth-Token", client.TokenID)
	createSnapReq.Header.Set("Content-Type", "application/json")

	createSnapResp, err := http.DefaultClient.Do(createSnapReq)
	require.NoError(t, err)
	defer createSnapResp.Body.Close()

	createSnapRespBody, _ := io.ReadAll(createSnapResp.Body)
	var createSnapResult struct {
		Snapshot struct {
			ID string `json:"id"`
		} `json:"snapshot"`
	}
	json.Unmarshal(createSnapRespBody, &createSnapResult)
	snapshotID := createSnapResult.Snapshot.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("snapshots", snapshotID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Set a metadata key first
	metadataBody := `{"meta": {"test-key": "test-value"}}`
	setURL := client.ServiceURL("snapshots", snapshotID, "metadata", "test-key")
	setReq, _ := http.NewRequest("PUT", setURL, strings.NewReader(metadataBody))
	setReq.Header.Set("X-Auth-Token", client.TokenID)
	setReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(setReq)

	// Get the metadata key
	url := client.ServiceURL("snapshots", snapshotID, "metadata", "test-key")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	// Should have meta object with the key
	assert.NotNil(t, result["meta"])
}

// TestCinderUpdateAllSnapshotMetadata_Contract tests POST /v3/:project/snapshots/:id/metadata
func TestCinderUpdateAllSnapshotMetadata_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume
	createVolBody := `{"volume": {"size": 1, "name": "snap-all-meta-vol"}}`
	createVolURL := client.ServiceURL("volumes")
	createVolReq, _ := http.NewRequest("POST", createVolURL, strings.NewReader(createVolBody))
	createVolReq.Header.Set("X-Auth-Token", client.TokenID)
	createVolReq.Header.Set("Content-Type", "application/json")

	createVolResp, err := http.DefaultClient.Do(createVolReq)
	require.NoError(t, err)
	defer createVolResp.Body.Close()

	createVolRespBody, _ := io.ReadAll(createVolResp.Body)
	var createVolResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createVolRespBody, &createVolResult)
	volumeID := createVolResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Create snapshot
	createSnapBody := `{"snapshot": {"volume_id": "` + volumeID + `", "name": "test-snap"}}`
	createSnapURL := client.ServiceURL("snapshots")
	createSnapReq, _ := http.NewRequest("POST", createSnapURL, strings.NewReader(createSnapBody))
	createSnapReq.Header.Set("X-Auth-Token", client.TokenID)
	createSnapReq.Header.Set("Content-Type", "application/json")

	createSnapResp, err := http.DefaultClient.Do(createSnapReq)
	require.NoError(t, err)
	defer createSnapResp.Body.Close()

	createSnapRespBody, _ := io.ReadAll(createSnapResp.Body)
	var createSnapResult struct {
		Snapshot struct {
			ID string `json:"id"`
		} `json:"snapshot"`
	}
	json.Unmarshal(createSnapRespBody, &createSnapResult)
	snapshotID := createSnapResult.Snapshot.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("snapshots", snapshotID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Update all metadata
	metadataBody := `{"metadata": {"key1": "value1", "key2": "value2"}}`
	url := client.ServiceURL("snapshots", snapshotID, "metadata")
	req, err := http.NewRequest("POST", url, strings.NewReader(metadataBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 OK
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCinderDeleteSnapshotMetadataKey_Contract tests DELETE /v3/:project/snapshots/:id/metadata/:key
func TestCinderDeleteSnapshotMetadataKey_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test volume
	createVolBody := `{"volume": {"size": 1, "name": "snap-del-meta-vol"}}`
	createVolURL := client.ServiceURL("volumes")
	createVolReq, _ := http.NewRequest("POST", createVolURL, strings.NewReader(createVolBody))
	createVolReq.Header.Set("X-Auth-Token", client.TokenID)
	createVolReq.Header.Set("Content-Type", "application/json")

	createVolResp, err := http.DefaultClient.Do(createVolReq)
	require.NoError(t, err)
	defer createVolResp.Body.Close()

	createVolRespBody, _ := io.ReadAll(createVolResp.Body)
	var createVolResult struct {
		Volume struct {
			ID string `json:"id"`
		} `json:"volume"`
	}
	json.Unmarshal(createVolRespBody, &createVolResult)
	volumeID := createVolResult.Volume.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("volumes", volumeID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Create snapshot
	createSnapBody := `{"snapshot": {"volume_id": "` + volumeID + `", "name": "test-snap"}}`
	createSnapURL := client.ServiceURL("snapshots")
	createSnapReq, _ := http.NewRequest("POST", createSnapURL, strings.NewReader(createSnapBody))
	createSnapReq.Header.Set("X-Auth-Token", client.TokenID)
	createSnapReq.Header.Set("Content-Type", "application/json")

	createSnapResp, err := http.DefaultClient.Do(createSnapReq)
	require.NoError(t, err)
	defer createSnapResp.Body.Close()

	createSnapRespBody, _ := io.ReadAll(createSnapResp.Body)
	var createSnapResult struct {
		Snapshot struct {
			ID string `json:"id"`
		} `json:"snapshot"`
	}
	json.Unmarshal(createSnapRespBody, &createSnapResult)
	snapshotID := createSnapResult.Snapshot.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("snapshots", snapshotID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Set a metadata key first
	metadataBody := `{"meta": {"delete-me": "value"}}`
	setURL := client.ServiceURL("snapshots", snapshotID, "metadata", "delete-me")
	setReq, _ := http.NewRequest("PUT", setURL, strings.NewReader(metadataBody))
	setReq.Header.Set("X-Auth-Token", client.TokenID)
	setReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(setReq)

	// Delete the metadata key
	url := client.ServiceURL("snapshots", snapshotID, "metadata", "delete-me")
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 204 No Content
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
