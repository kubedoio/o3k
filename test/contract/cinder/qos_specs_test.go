package cinder

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCinderListQosSpecs_Contract tests GET /v3/:project/qos-specs
func TestCinderListQosSpecs_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	url := client.ServiceURL("qos-specs")
	t.Logf("Testing URL: %s", url)
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		QosSpecs []map[string]interface{} `json:"qos_specs"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.QosSpecs)
}

// TestCinderCreateQosSpec_Contract tests POST /v3/:project/qos-specs
func TestCinderCreateQosSpec_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	payload := map[string]interface{}{
		"qos_specs": map[string]interface{}{
			"name": "test-qos",
			"consumer": "back-end",
			"specs": map[string]interface{}{
				"read_iops_sec": "1000",
				"write_iops_sec": "800",
			},
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("qos-specs")
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
		QosSpecs map[string]interface{} `json:"qos_specs"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.QosSpecs["id"])
	assert.Equal(t, "test-qos", result.QosSpecs["name"])
	assert.Equal(t, "back-end", result.QosSpecs["consumer"])

	// Cleanup
	qosID := result.QosSpecs["id"].(string)
	cleanupTestQosSpec(t, client, qosID)
}

// TestCinderGetQosSpec_Contract tests GET /v3/:project/qos-specs/:id
func TestCinderGetQosSpec_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test QoS spec
	qosID := createTestQosSpec(t, client)

	url := client.ServiceURL("qos-specs", qosID)
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		QosSpecs map[string]interface{} `json:"qos_specs"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, qosID, result.QosSpecs["id"])

	// Cleanup
	cleanupTestQosSpec(t, client, qosID)
}

// TestCinderUpdateQosSpec_Contract tests PUT /v3/:project/qos-specs/:id
func TestCinderUpdateQosSpec_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test QoS spec
	qosID := createTestQosSpec(t, client)

	payload := map[string]interface{}{
		"qos_specs": map[string]interface{}{
			"read_iops_sec": "2000",
			"write_iops_sec": "1500",
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("qos-specs", qosID)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		QosSpecs map[string]interface{} `json:"qos_specs"`
	}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	specs := result.QosSpecs["specs"].(map[string]interface{})
	assert.Equal(t, "2000", specs["read_iops_sec"])

	// Cleanup
	cleanupTestQosSpec(t, client, qosID)
}

// TestCinderDeleteQosSpec_Contract tests DELETE /v3/:project/qos-specs/:id
func TestCinderDeleteQosSpec_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Create test QoS spec
	qosID := createTestQosSpec(t, client)

	url := client.ServiceURL("qos-specs", qosID)
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	req.Header.Set("X-Auth-Token", client.TokenID)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

// Helper to create test QoS spec
func createTestQosSpec(t *testing.T, client *gophercloud.ServiceClient) string {
	t.Helper()

	payload := map[string]interface{}{
		"qos_specs": map[string]interface{}{
			"name": "test-qos",
			"consumer": "back-end",
			"specs": map[string]interface{}{
				"read_iops_sec": "1000",
				"write_iops_sec": "800",
			},
		},
	}

	body, _ := json.Marshal(payload)
	url := client.ServiceURL("qos-specs")
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", client.TokenID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create QoS spec: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		QosSpecs map[string]interface{} `json:"qos_specs"`
	}
	json.Unmarshal(respBody, &result)

	return result.QosSpecs["id"].(string)
}

// Helper to cleanup test QoS spec
func cleanupTestQosSpec(t *testing.T, client *gophercloud.ServiceClient, qosID string) {
	t.Helper()

	url := client.ServiceURL("qos-specs", qosID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("X-Auth-Token", client.TokenID)
	http.DefaultClient.Do(req)
}
