package neutron_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNeutronListNetworkIPAvailabilities_Contract tests GET /v2.0/network-ip-availabilities
func TestNeutronListNetworkIPAvailabilities_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	url := client.ServiceURL("network-ip-availabilities")
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

	// Should have network_ip_availabilities array
	assert.NotNil(t, result["network_ip_availabilities"])
}

// TestNeutronGetNetworkIPAvailability_Contract tests GET /v2.0/network-ip-availabilities/:id
func TestNeutronGetNetworkIPAvailability_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	// Create network first
	createBody := `{"network": {"name": "ip-avail-test", "admin_state_up": true}}`
	createURL := client.ServiceURL("networks")
	createReq, _ := http.NewRequest("POST", createURL, strings.NewReader(createBody))
	createReq.Header.Set("X-Auth-Token", client.TokenID)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	createRespBody, _ := io.ReadAll(createResp.Body)
	var createResult struct {
		Network struct {
			ID string `json:"id"`
		} `json:"network"`
	}
	json.Unmarshal(createRespBody, &createResult)
	networkID := createResult.Network.ID

	defer func() {
		deleteReq, _ := http.NewRequest("DELETE", client.ServiceURL("networks", networkID), nil)
		deleteReq.Header.Set("X-Auth-Token", client.TokenID)
		http.DefaultClient.Do(deleteReq)
	}()

	// Get network IP availability
	url := client.ServiceURL("network-ip-availabilities", networkID)
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		NetworkIPAvailability struct {
			NetworkID string `json:"network_id"`
		} `json:"network_ip_availability"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, networkID, result.NetworkIPAvailability.NetworkID)
}
