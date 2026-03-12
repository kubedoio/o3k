package cinder_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCinderVolumeUpdate_Contract tests PATCH /v3/{project_id}/volumes/{id}
func TestCinderVolumeUpdate_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Setup: Create volume using raw HTTP
	volumeName := "test-volume-update-" + uuid.New().String()[:8]
	createBody := map[string]interface{}{
		"volume": map[string]interface{}{
			"size": 1,
			"name": volumeName,
		},
	}
	createBodyJSON, _ := json.Marshal(createBody)
	createReq, _ := http.NewRequest("POST", client.ServiceURL("volumes"), bytes.NewReader(createBodyJSON))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err, "Setup: CreateVolume should succeed")
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

	// Test: Update volume name and description using PATCH
	newName := volumeName + "-updated"
	newDesc := "Updated description"
	updateBody := map[string]interface{}{
		"volume": map[string]interface{}{
			"name":        newName,
			"description": newDesc,
		},
	}
	updateBodyJSON, _ := json.Marshal(updateBody)
	updateReq, _ := http.NewRequest("PATCH", client.ServiceURL("volumes", volumeID), bytes.NewReader(updateBodyJSON))
	updateReq.Header.Set("X-Auth-Token", client.TokenID)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(updateReq)
	require.NoError(t, err, "UpdateVolume should succeed")
	defer updateResp.Body.Close()

	// Assertions
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	updateRespBody, _ := io.ReadAll(updateResp.Body)
	var updateResult struct {
		Volume struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"volume"`
	}
	err = json.Unmarshal(updateRespBody, &updateResult)
	require.NoError(t, err)

	assert.Equal(t, newName, updateResult.Volume.Name, "Volume name should be updated")
	assert.Equal(t, newDesc, updateResult.Volume.Description, "Volume description should be updated")
	assert.Equal(t, volumeID, updateResult.Volume.ID, "Volume ID should not change")
}

// TestCinderSnapshotUpdate_Contract tests PATCH /v3/{project_id}/snapshots/{id}
func TestCinderSnapshotUpdate_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Setup: Create volume using raw HTTP
	volumeName := "test-volume-for-snap-" + uuid.New().String()[:8]
	createVolBody := map[string]interface{}{
		"volume": map[string]interface{}{
			"size": 1,
			"name": volumeName,
		},
	}
	createVolBodyJSON, _ := json.Marshal(createVolBody)
	createVolReq, _ := http.NewRequest("POST", client.ServiceURL("volumes"), bytes.NewReader(createVolBodyJSON))
	createVolReq.Header.Set("X-Auth-Token", client.TokenID)
	createVolReq.Header.Set("Content-Type", "application/json")

	createVolResp, err := http.DefaultClient.Do(createVolReq)
	require.NoError(t, err, "Setup: CreateVolume should succeed")
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

	// Setup: Create snapshot using raw HTTP
	snapName := "test-snapshot-update-" + uuid.New().String()[:8]
	createSnapBody := map[string]interface{}{
		"snapshot": map[string]interface{}{
			"volume_id": volumeID,
			"name":      snapName,
		},
	}
	createSnapBodyJSON, _ := json.Marshal(createSnapBody)
	createSnapReq, _ := http.NewRequest("POST", client.ServiceURL("snapshots"), bytes.NewReader(createSnapBodyJSON))
	createSnapReq.Header.Set("X-Auth-Token", client.TokenID)
	createSnapReq.Header.Set("Content-Type", "application/json")

	createSnapResp, err := http.DefaultClient.Do(createSnapReq)
	require.NoError(t, err, "Setup: CreateSnapshot should succeed")
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

	// Test: Update snapshot name and description using PATCH
	newName := snapName + "-updated"
	newDesc := "Updated snapshot description"
	updateBody := map[string]interface{}{
		"snapshot": map[string]interface{}{
			"name":        newName,
			"description": newDesc,
		},
	}
	updateBodyJSON, _ := json.Marshal(updateBody)
	updateReq, _ := http.NewRequest("PATCH", client.ServiceURL("snapshots", snapshotID), bytes.NewReader(updateBodyJSON))
	updateReq.Header.Set("X-Auth-Token", client.TokenID)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(updateReq)
	require.NoError(t, err, "UpdateSnapshot should succeed")
	defer updateResp.Body.Close()

	// Assertions
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	updateRespBody, _ := io.ReadAll(updateResp.Body)
	var updateResult struct {
		Snapshot struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"snapshot"`
	}
	err = json.Unmarshal(updateRespBody, &updateResult)
	require.NoError(t, err)

	assert.Equal(t, newName, updateResult.Snapshot.Name, "Snapshot name should be updated")
	assert.Equal(t, newDesc, updateResult.Snapshot.Description, "Snapshot description should be updated")
	assert.Equal(t, snapshotID, updateResult.Snapshot.ID, "Snapshot ID should not change")
}
