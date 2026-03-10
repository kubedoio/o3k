package neutron_test

import (
	"testing"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/quotas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeutronGetQuota_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	// Get quota for current project
	quota, err := quotas.Get(client, client.ProviderClient.TokenID).Extract()
	require.NoError(t, err)
	require.NotNil(t, quota)

	// Verify quota structure
	assert.GreaterOrEqual(t, quota.Network, 0, "Network quota should be >= 0")
	assert.GreaterOrEqual(t, quota.Subnet, 0, "Subnet quota should be >= 0")
	assert.GreaterOrEqual(t, quota.Port, 0, "Port quota should be >= 0")
	assert.GreaterOrEqual(t, quota.Router, 0, "Router quota should be >= 0")
	assert.GreaterOrEqual(t, quota.FloatingIP, 0, "FloatingIP quota should be >= 0")
	assert.GreaterOrEqual(t, quota.SecurityGroup, 0, "SecurityGroup quota should be >= 0")
	assert.GreaterOrEqual(t, quota.SecurityGroupRule, 0, "SecurityGroupRule quota should be >= 0")
}
