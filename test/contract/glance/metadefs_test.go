package glance_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGlanceListMetadefNamespaces_Contract tests GET /v2/metadefs/namespaces
func TestGlanceListMetadefNamespaces_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	url := client.ServiceURL("metadefs", "namespaces")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Namespaces []map[string]interface{} `json:"namespaces"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Namespaces)
}

// TestGlanceCreateMetadefNamespace_Contract tests POST /v2/metadefs/namespaces
func TestGlanceCreateMetadefNamespace_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	// Create namespace
	payload := map[string]interface{}{
		"namespace":    "test-namespace",
		"display_name": "Test Namespace",
		"description":  "Test metadata namespace",
		"visibility":   "public",
		"resource_type_associations": []map[string]interface{}{
			{"name": "OS::Glance::Image"},
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("metadefs", "namespaces")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "test-namespace", result["namespace"])
	assert.Equal(t, "Test Namespace", result["display_name"])

	// Cleanup
	deleteURL := client.ServiceURL("metadefs", "namespaces", "test-namespace")
	deleteReq, _ := http.NewRequest("DELETE", deleteURL, nil)
	deleteReq.Header.Set("X-Auth-Token", client.TokenID)
	http.DefaultClient.Do(deleteReq)
}

// TestGlanceGetMetadefNamespace_Contract tests GET /v2/metadefs/namespaces/:namespace
func TestGlanceGetMetadefNamespace_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	// Create namespace first
	nsName := createTestMetadefNamespace(t, client, "test-get-namespace")

	url := client.ServiceURL("metadefs", "namespaces", nsName)
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

	assert.Equal(t, nsName, result["namespace"])

	// Cleanup
	cleanupTestMetadefNamespace(t, client, nsName)
}

// TestGlanceUpdateMetadefNamespace_Contract tests PUT /v2/metadefs/namespaces/:namespace
func TestGlanceUpdateMetadefNamespace_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	// Create namespace first
	nsName := createTestMetadefNamespace(t, client, "test-update-namespace")

	// Update namespace
	payload := map[string]interface{}{
		"namespace":    nsName,
		"display_name": "Updated Namespace",
		"description":  "Updated description",
		"visibility":   "public",
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("metadefs", "namespaces", nsName)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "Updated Namespace", result["display_name"])

	// Cleanup
	cleanupTestMetadefNamespace(t, client, nsName)
}

// TestGlanceDeleteMetadefNamespace_Contract tests DELETE /v2/metadefs/namespaces/:namespace
func TestGlanceDeleteMetadefNamespace_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	// Create namespace first
	nsName := createTestMetadefNamespace(t, client, "test-delete-namespace")

	url := client.ServiceURL("metadefs", "namespaces", nsName)
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestGlanceListMetadefResourceTypes_Contract tests GET /v2/metadefs/resource_types
func TestGlanceListMetadefResourceTypes_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupGlanceClient(t)

	url := client.ServiceURL("metadefs", "resource_types")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ResourceTypes []map[string]interface{} `json:"resource_types"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.ResourceTypes)
}

// Helper to create test metadef namespace
func createTestMetadefNamespace(t *testing.T, client *gophercloud.ServiceClient, name string) string {
	t.Helper()

	payload := map[string]interface{}{
		"namespace":    name,
		"display_name": name,
		"description":  "Test namespace",
		"visibility":   "public",
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("metadefs", "namespaces")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer resp.Body.Close()

	return name
}

// Helper to cleanup test metadef namespace
func cleanupTestMetadefNamespace(t *testing.T, client *gophercloud.ServiceClient, name string) {
	t.Helper()

	url := client.ServiceURL("metadefs", "namespaces", name)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Auth-Token", client.TokenID)
	http.DefaultClient.Do(req)
}
