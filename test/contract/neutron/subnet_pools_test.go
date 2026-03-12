package neutron_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNeutronListSubnetPools_Contract tests GET /v2.0/subnetpools
func TestNeutronListSubnetPools_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	url := client.ServiceURL("subnetpools")
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		SubnetPools []map[string]interface{} `json:"subnetpools"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.SubnetPools)
}

// TestNeutronCreateSubnetPool_Contract tests POST /v2.0/subnetpools
func TestNeutronCreateSubnetPool_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	// Create subnet pool
	createBody := `{
		"subnetpool": {
			"name": "test-pool",
			"prefixes": ["10.10.0.0/16"],
			"min_prefixlen": 24,
			"max_prefixlen": 28
		}
	}`

	url := client.ServiceURL("subnetpools")
	req, err := http.NewRequest("POST", url, strings.NewReader(createBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		SubnetPool map[string]interface{} `json:"subnetpool"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.SubnetPool["id"])
	assert.Equal(t, "test-pool", result.SubnetPool["name"])

	// Cleanup
	poolID := result.SubnetPool["id"].(string)
	cleanupTestSubnetPool(t, client, poolID)
}

// TestNeutronGetSubnetPool_Contract tests GET /v2.0/subnetpools/:id
func TestNeutronGetSubnetPool_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	// Create test subnet pool
	poolID := createTestSubnetPool(t, client)
	defer cleanupTestSubnetPool(t, client, poolID)

	// Get subnet pool details
	url := client.ServiceURL("subnetpools", poolID)
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		SubnetPool map[string]interface{} `json:"subnetpool"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, poolID, result.SubnetPool["id"])
	assert.Equal(t, "test-pool", result.SubnetPool["name"])
}

// TestNeutronUpdateSubnetPool_Contract tests PUT /v2.0/subnetpools/:id
func TestNeutronUpdateSubnetPool_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	// Create test subnet pool
	poolID := createTestSubnetPool(t, client)
	defer cleanupTestSubnetPool(t, client, poolID)

	// Update subnet pool
	updateBody := `{
		"subnetpool": {
			"name": "updated-pool"
		}
	}`

	url := client.ServiceURL("subnetpools", poolID)
	req, err := http.NewRequest("PUT", url, strings.NewReader(updateBody))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		SubnetPool map[string]interface{} `json:"subnetpool"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "updated-pool", result.SubnetPool["name"])
}

// TestNeutronDeleteSubnetPool_Contract tests DELETE /v2.0/subnetpools/:id
func TestNeutronDeleteSubnetPool_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	// Create test subnet pool
	poolID := createTestSubnetPool(t, client)

	// Delete subnet pool
	url := client.ServiceURL("subnetpools", poolID)
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// Helper function to create a test subnet pool
func createTestSubnetPool(t *testing.T, client *gophercloud.ServiceClient) string {
	t.Helper()

	createBody := `{
		"subnetpool": {
			"name": "test-pool",
			"prefixes": ["10.10.0.0/16"],
			"min_prefixlen": 24,
			"max_prefixlen": 28
		}
	}`

	url := client.ServiceURL("subnetpools")
	req, _ := http.NewRequest("POST", url, strings.NewReader(createBody))
	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		SubnetPool map[string]interface{} `json:"subnetpool"`
	}
	json.Unmarshal(respBody, &result)

	return result.SubnetPool["id"].(string)
}

// Helper function to cleanup a test subnet pool
func cleanupTestSubnetPool(t *testing.T, client *gophercloud.ServiceClient, poolID string) {
	t.Helper()

	url := client.ServiceURL("subnetpools", poolID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Auth-Token", client.TokenID)

	http.DefaultClient.Do(req)
}
