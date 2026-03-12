package nova_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNovaGetConsoleOutput_Contract tests POST /v2.1/servers/:id/action (os-getConsoleOutput)
func TestNovaGetConsoleOutput_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	createOpts := servers.CreateOpts{
		Name:      "test-console-output",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}
	server, err := servers.Create(client, createOpts).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Get console output via action
	payload := map[string]interface{}{
		"os-getConsoleOutput": map[string]interface{}{
			"length": 50,
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
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

	assert.NotNil(t, result["output"])
}

// TestNovaGetVNCConsole_Contract tests POST /v2.1/servers/:id/action (os-getVNCConsole)
func TestNovaGetVNCConsole_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	createOpts := servers.CreateOpts{
		Name:      "test-vnc-console",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}
	server, err := servers.Create(client, createOpts).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Get VNC console
	payload := map[string]interface{}{
		"os-getVNCConsole": map[string]interface{}{
			"type": "novnc",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Console struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"console"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "novnc", result.Console.Type)
	assert.NotEmpty(t, result.Console.URL)
}

// TestNovaCreateRemoteConsole_Contract tests POST /v2.1/servers/:id/remote-consoles
func TestNovaCreateRemoteConsole_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	createOpts := servers.CreateOpts{
		Name:      "test-remote-console",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}
	server, err := servers.Create(client, createOpts).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Create remote console (microversion 2.6+)
	payload := map[string]interface{}{
		"remote_console": map[string]interface{}{
			"protocol": "vnc",
			"type":     "novnc",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("servers", server.ID, "remote-consoles")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenStack-API-Version", "compute 2.6")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		RemoteConsole struct {
			Protocol string `json:"protocol"`
			Type     string `json:"type"`
			URL      string `json:"url"`
		} `json:"remote_console"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "vnc", result.RemoteConsole.Protocol)
	assert.Equal(t, "novnc", result.RemoteConsole.Type)
	assert.NotEmpty(t, result.RemoteConsole.URL)
}

// TestNovaGetSerialConsole_Contract tests POST /v2.1/servers/:id/action (os-getSerialConsole)
func TestNovaGetSerialConsole_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	createOpts := servers.CreateOpts{
		Name:      "test-serial-console",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}
	server, err := servers.Create(client, createOpts).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Get serial console
	payload := map[string]interface{}{
		"os-getSerialConsole": map[string]interface{}{
			"type": "serial",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Console struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"console"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "serial", result.Console.Type)
	assert.NotEmpty(t, result.Console.URL)
}

// TestNovaGetSPICEConsole_Contract tests POST /v2.1/servers/:id/action (os-getSPICEConsole)
func TestNovaGetSPICEConsole_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	createOpts := servers.CreateOpts{
		Name:      "test-spice-console",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}
	server, err := servers.Create(client, createOpts).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Get SPICE console
	payload := map[string]interface{}{
		"os-getSPICEConsole": map[string]interface{}{
			"type": "spice-html5",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Console struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"console"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "spice-html5", result.Console.Type)
	assert.NotEmpty(t, result.Console.URL)
	assert.Contains(t, result.Console.URL, "6082") // SPICE port
}

// TestNovaGetRDPConsole_Contract tests POST /v2.1/servers/:id/action (os-getRDPConsole)
func TestNovaGetRDPConsole_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// Create test server
	createOpts := servers.CreateOpts{
		Name:      "test-rdp-console",
		FlavorRef: "00000000-0000-0000-0000-000000000010",
		ImageRef:  "00000000-0000-0000-0000-000000000001",
	}
	server, err := servers.Create(client, createOpts).Extract()
	require.NoError(t, err)
	defer servers.Delete(client, server.ID)

	// Get RDP console
	payload := map[string]interface{}{
		"os-getRDPConsole": map[string]interface{}{
			"type": "rdp-html5",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("servers", server.ID, "action")
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Console struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"console"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, "rdp-html5", result.Console.Type)
	assert.NotEmpty(t, result.Console.URL)
	assert.Contains(t, result.Console.URL, "6084") // RDP port
}
