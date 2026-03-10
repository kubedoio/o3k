package nova_test

import (
	"testing"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/hypervisors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNovaListHypervisors_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNovaClient(t)

	// List hypervisors
	allPages, err := hypervisors.List(client, hypervisors.ListOpts{}).AllPages()
	require.NoError(t, err)

	hypervisorList, err := hypervisors.ExtractHypervisors(allPages)
	require.NoError(t, err)

	// Should have at least one hypervisor
	assert.NotEmpty(t, hypervisorList)

	// Verify hypervisor structure
	if len(hypervisorList) > 0 {
		h := hypervisorList[0]
		assert.NotEmpty(t, h.HypervisorHostname)
		assert.NotEmpty(t, h.HypervisorType)
		assert.NotEmpty(t, h.ID)
	}
}
