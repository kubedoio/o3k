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

// TestKeystoneListDomains_Contract tests GET /v3/domains
func TestKeystoneListDomains_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	url := client.ServiceURL("domains")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Domains []map[string]interface{} `json:"domains"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Domains)
	// Should have at least "default" domain
	assert.GreaterOrEqual(t, len(result.Domains), 1)
}

// TestKeystoneCreateDomain_Contract tests POST /v3/domains
func TestKeystoneCreateDomain_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create domain
	payload := map[string]interface{}{
		"domain": map[string]interface{}{
			"name":        "test-domain",
			"description": "Test domain",
			"enabled":     true,
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("domains")
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
		Domain map[string]interface{} `json:"domain"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Domain["id"])
	assert.Equal(t, "test-domain", result.Domain["name"])

	// Cleanup
	domainID := result.Domain["id"].(string)
	cleanupTestDomain(t, client, domainID)
}

// TestKeystoneGetDomain_Contract tests GET /v3/domains/:id
func TestKeystoneGetDomain_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create domain first
	domainID := createTestDomain(t, client, "test-get-domain")

	url := client.ServiceURL("domains", domainID)
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Domain map[string]interface{} `json:"domain"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, domainID, result.Domain["id"])

	// Cleanup
	cleanupTestDomain(t, client, domainID)
}

// TestKeystoneUpdateDomain_Contract tests PATCH /v3/domains/:id
func TestKeystoneUpdateDomain_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create domain first
	domainID := createTestDomain(t, client, "test-update-domain")

	// Update domain
	payload := map[string]interface{}{
		"domain": map[string]interface{}{
			"description": "Updated description",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("domains", domainID)
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
		Domain map[string]interface{} `json:"domain"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "Updated description", result.Domain["description"])

	// Cleanup
	cleanupTestDomain(t, client, domainID)
}

// TestKeystoneDeleteDomain_Contract tests DELETE /v3/domains/:id
func TestKeystoneDeleteDomain_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create domain first
	domainID := createTestDomain(t, client, "test-delete-domain")

	// Disable domain first (required before delete)
	disablePayload := map[string]interface{}{
		"domain": map[string]interface{}{
			"enabled": false,
		},
	}
	disableBody, _ := json.Marshal(disablePayload)
	disableURL := client.ServiceURL("domains", domainID)
	disableReq, _ := http.NewRequest("PATCH", disableURL, bytes.NewReader(disableBody))
	disableReq.Header.Set("X-Auth-Token", client.TokenID)
	disableReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(disableReq)

	// Delete domain
	url := client.ServiceURL("domains", domainID)
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestKeystoneGetDomainConfig_Contract tests GET /v3/domains/:id/config
func TestKeystoneGetDomainConfig_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupKeystoneClient(t)

	// Create a test domain
	domainID := createTestDomain(t, client, "test-config-domain")

	url := client.ServiceURL("domains", domainID, "config")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Config map[string]interface{} `json:"config"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Config)

	// Cleanup
	cleanupTestDomain(t, client, domainID)
}

// Helper to create test domain
func createTestDomain(t *testing.T, client *gophercloud.ServiceClient, name string) string {
	t.Helper()

	payload := map[string]interface{}{
		"domain": map[string]interface{}{
			"name":    name,
			"enabled": true,
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("domains")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create domain: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Domain map[string]interface{} `json:"domain"`
	}
	json.Unmarshal(respBody, &result)

	return result.Domain["id"].(string)
}

// Helper to cleanup test domain
func cleanupTestDomain(t *testing.T, client *gophercloud.ServiceClient, domainID string) {
	t.Helper()

	// Disable first
	disablePayload := map[string]interface{}{
		"domain": map[string]interface{}{
			"enabled": false,
		},
	}
	disableBody, _ := json.Marshal(disablePayload)
	disableURL := client.ServiceURL("domains", domainID)
	disableReq, _ := http.NewRequest("PATCH", disableURL, bytes.NewReader(disableBody))
	disableReq.Header.Set("X-Auth-Token", client.TokenID)
	disableReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(disableReq)

	// Delete
	url := client.ServiceURL("domains", domainID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Auth-Token", client.TokenID)
	http.DefaultClient.Do(req)
}
