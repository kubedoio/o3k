package keystone_test

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

// TestKeystoneListCredentials_Contract tests GET /v3/credentials
func TestKeystoneListCredentials_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	url := client.ServiceURL("credentials")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Credentials []map[string]interface{} `json:"credentials"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Credentials)
}

// TestKeystoneCreateCredential_Contract tests POST /v3/credentials
func TestKeystoneCreateCredential_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	payload := map[string]interface{}{
		"credential": map[string]interface{}{
			"user_id":    "00000000-0000-0000-0000-000000000001",
			"project_id": "00000000-0000-0000-0000-000000000002",
			"type":       "ec2",
			"blob":       `{"access": "test-access", "secret": "test-secret"}`,
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("credentials")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Credential map[string]interface{} `json:"credential"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Credential["id"])
	assert.Equal(t, "ec2", result.Credential["type"])

	// Cleanup
	credID := result.Credential["id"].(string)
	cleanupTestCredential(t, client, credID)
}

// TestKeystoneGetCredential_Contract tests GET /v3/credentials/:id
func TestKeystoneGetCredential_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create test credential
	credID := createTestCredential(t, client)

	url := client.ServiceURL("credentials", credID)
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Credential map[string]interface{} `json:"credential"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, credID, result.Credential["id"])

	// Cleanup
	cleanupTestCredential(t, client, credID)
}

// TestKeystoneUpdateCredential_Contract tests PATCH /v3/credentials/:id
func TestKeystoneUpdateCredential_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create test credential
	credID := createTestCredential(t, client)

	payload := map[string]interface{}{
		"credential": map[string]interface{}{
			"blob": `{"access": "updated-access", "secret": "updated-secret"}`,
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("credentials", credID)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Credential map[string]interface{} `json:"credential"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Contains(t, result.Credential["blob"], "updated-access")

	// Cleanup
	cleanupTestCredential(t, client, credID)
}

// TestKeystoneDeleteCredential_Contract tests DELETE /v3/credentials/:id
func TestKeystoneDeleteCredential_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create test credential
	credID := createTestCredential(t, client)

	url := client.ServiceURL("credentials", credID)
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// Helper to create test credential
func createTestCredential(t *testing.T, client *gophercloud.ServiceClient) string {
	t.Helper()

	payload := map[string]interface{}{
		"credential": map[string]interface{}{
			"user_id":    "00000000-0000-0000-0000-000000000001",
			"project_id": "00000000-0000-0000-0000-000000000002",
			"type":       "ec2",
			"blob":       `{"access": "test-access", "secret": "test-secret"}`,
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("credentials")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create credential: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Credential map[string]interface{} `json:"credential"`
	}
	json.Unmarshal(respBody, &result)

	return result.Credential["id"].(string)
}

// Helper to cleanup test credential
func cleanupTestCredential(t *testing.T, client *gophercloud.ServiceClient, credID string) {
	t.Helper()

	url := client.ServiceURL("credentials", credID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Auth-Token", client.TokenID)
	http.DefaultClient.Do(req)
}
