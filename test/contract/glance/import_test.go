package glance_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGlanceStageImageData_Contract tests POST /v2/images/:id/stage
func TestGlanceStageImageData_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	// Create image for staging
	visibility := images.ImageVisibilityPrivate
	image, err := images.Create(client, images.CreateOpts{
		Name:            "stage-test-image",
		ContainerFormat: "bare",
		DiskFormat:      "qcow2",
		Visibility:      &visibility,
	}).Extract()
	require.NoError(t, err)
	defer images.Delete(client, image.ID)

	// Stage image data (dummy data for test)
	imageData := strings.NewReader("fake-qcow2-data-for-staging")
	url := client.ServiceURL("images", image.ID, "stage")
	req, err := http.NewRequest("POST", url, imageData)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 204 No Content
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestGlanceImportImage_Contract tests POST /v2/images/:id/import
func TestGlanceImportImage_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	// Create image for import
	visibility := images.ImageVisibilityPrivate
	image, err := images.Create(client, images.CreateOpts{
		Name:            "import-test-image",
		ContainerFormat: "bare",
		DiskFormat:      "qcow2",
		Visibility:      &visibility,
	}).Extract()
	require.NoError(t, err)
	defer images.Delete(client, image.ID)

	// Stage some data first
	imageData := strings.NewReader("fake-qcow2-data")
	stageURL := client.ServiceURL("images", image.ID, "stage")
	stageReq, _ := http.NewRequest("POST", stageURL, imageData)
	stageReq.Header.Set("X-Auth-Token", client.TokenID)
	stageReq.Header.Set("Content-Type", "application/octet-stream")
	http.DefaultClient.Do(stageReq)

	// Import image (move from staging to active)
	importBody := `{
		"method": {
			"name": "glance-direct"
		}
	}`

	url := client.ServiceURL("images", image.ID, "import")
	req, err := http.NewRequest("POST", url, strings.NewReader(importBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 202 Accepted
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// TestGlanceGetImageImportInfo_Contract tests GET /v2/images/:id/import
func TestGlanceGetImageImportInfo_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	// Create test image
	visibility := images.ImageVisibilityPrivate
	image, err := images.Create(client, images.CreateOpts{
		Name:            "import-info-test",
		ContainerFormat: "bare",
		DiskFormat:      "qcow2",
		Visibility:      &visibility,
	}).Extract()
	require.NoError(t, err)
	defer images.Delete(client, image.ID)

	// Get import info
	url := client.ServiceURL("images", image.ID, "import")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 OK with import methods info
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	// Should have import-methods array
	assert.NotNil(t, result["import-methods"])
}
