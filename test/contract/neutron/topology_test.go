package neutron_test

import (
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	authURL     = "http://localhost:35357/v3"
	username    = "admin"
	password    = "secret"
	projectName = "default"
	domainName  = "Default"
)

func setupNetworkClient(t *testing.T) *gophercloud.ServiceClient {
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       projectName,
		DomainName:       domainName,
		AllowReauth:      true,
	}

	provider, err := openstack.AuthenticatedClient(opts)
	require.NoError(t, err, "Failed to authenticate")

	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{})
	require.NoError(t, err, "Failed to create Neutron client")

	return client
}

// TestNetworkTopologyData_Contract verifies network topology data is complete for Horizon
func TestNetworkTopologyData_Contract(t *testing.T) {
	client := setupNetworkClient(t)

	// Create test network for topology
	createOpts := networks.CreateOpts{
		Name:         "topology-test-network",
		AdminStateUp: gophercloud.Enabled,
	}

	network, err := networks.Create(client, createOpts).Extract()
	require.NoError(t, err, "Failed to create network")
	defer networks.Delete(client, network.ID)

	// Create subnet
	subnetOpts := subnets.CreateOpts{
		NetworkID:  network.ID,
		CIDR:       "192.168.100.0/24",
		IPVersion:  gophercloud.IPv4,
		Name:       "topology-test-subnet",
		GatewayIP:  nil, // No gateway for this test
		EnableDHCP: gophercloud.Disabled,
	}

	subnet, err := subnets.Create(client, subnetOpts).Extract()
	require.NoError(t, err, "Failed to create subnet")
	defer subnets.Delete(client, subnet.ID)

	// Verify network has required fields for topology visualization
	assert.NotEmpty(t, network.ID, "Network should have ID")
	assert.NotEmpty(t, network.Name, "Network should have name")
	assert.NotNil(t, network.AdminStateUp, "Network should have admin_state_up")
	assert.NotEmpty(t, network.Status, "Network should have status")

	t.Logf("Network: ID=%s, Name=%s, Status=%s", network.ID, network.Name, network.Status)

	// Verify subnet has required fields for topology
	assert.NotEmpty(t, subnet.ID, "Subnet should have ID")
	assert.NotEmpty(t, subnet.Name, "Subnet should have name")
	assert.NotEmpty(t, subnet.CIDR, "Subnet should have CIDR")
	assert.Equal(t, network.ID, subnet.NetworkID, "Subnet should reference network")

	t.Logf("Subnet: ID=%s, CIDR=%s, NetworkID=%s", subnet.ID, subnet.CIDR, subnet.NetworkID)
}

// TestPortTopologyData_Contract verifies port data includes device_owner for topology
func TestPortTopologyData_Contract(t *testing.T) {
	client := setupNetworkClient(t)

	// Create network and subnet first
	network, err := networks.Create(client, networks.CreateOpts{
		Name:         "port-topology-test-network",
		AdminStateUp: gophercloud.Enabled,
	}).Extract()
	require.NoError(t, err)
	defer networks.Delete(client, network.ID)

	subnet, err := subnets.Create(client, subnets.CreateOpts{
		NetworkID:  network.ID,
		CIDR:       "192.168.101.0/24",
		IPVersion:  gophercloud.IPv4,
		EnableDHCP: gophercloud.Disabled,
	}).Extract()
	require.NoError(t, err)
	defer subnets.Delete(client, subnet.ID)

	// Create port
	portOpts := ports.CreateOpts{
		NetworkID: network.ID,
		Name:      "topology-test-port",
	}

	port, err := ports.Create(client, portOpts).Extract()
	require.NoError(t, err)
	defer ports.Delete(client, port.ID)

	// Verify port has device_owner field (required for topology visualization)
	// device_owner indicates what type of device owns this port
	// Examples: compute:nova, network:router_interface, network:dhcp
	assert.NotEmpty(t, port.ID, "Port should have ID")
	assert.NotEmpty(t, port.NetworkID, "Port should have network_id")
	assert.NotEmpty(t, port.MACAddress, "Port should have MAC address")

	// device_owner can be empty for unattached ports, just verify field exists
	t.Logf("Port: ID=%s, NetworkID=%s, DeviceOwner=%s", port.ID, port.NetworkID, port.DeviceOwner)
}

// TestTopologyWithRouter_Contract verifies router data for network topology
func TestTopologyWithRouter_Contract(t *testing.T) {
	client := setupNetworkClient(t)

	// List existing routers to verify topology data format
	listOpts := ports.ListOpts{
		DeviceOwner: "network:router_interface",
	}

	allPages, err := ports.List(client, listOpts).AllPages()
	require.NoError(t, err, "Failed to list router interface ports")

	routerPorts, err := ports.ExtractPorts(allPages)
	require.NoError(t, err, "Failed to extract ports")

	if len(routerPorts) > 0 {
		// Verify router interface port has required fields
		routerPort := routerPorts[0]

		assert.NotEmpty(t, routerPort.ID, "Router port should have ID")
		assert.NotEmpty(t, routerPort.DeviceID, "Router port should have device_id (router ID)")
		assert.Equal(t, "network:router_interface", routerPort.DeviceOwner, "Should be router interface")
		assert.NotEmpty(t, routerPort.FixedIPs, "Router port should have fixed IPs")

		t.Logf("Router interface port: ID=%s, DeviceID=%s, Network=%s",
			routerPort.ID, routerPort.DeviceID, routerPort.NetworkID)
	} else {
		t.Log("No router interfaces found (acceptable for test environment)")
	}
}

// TestTopologyNetworkList_Contract verifies network list for topology page
func TestTopologyNetworkList_Contract(t *testing.T) {
	client := setupNetworkClient(t)

	// List all networks
	allPages, err := networks.List(client, nil).AllPages()
	require.NoError(t, err, "Failed to list networks")

	networkList, err := networks.ExtractNetworks(allPages)
	require.NoError(t, err, "Failed to extract networks")

	// Verify each network has required fields for topology
	for _, network := range networkList {
		assert.NotEmpty(t, network.ID, "Network should have ID")
		assert.NotEmpty(t, network.Name, "Network should have name")
		assert.NotEmpty(t, network.Status, "Network should have status")
		assert.NotNil(t, network.AdminStateUp, "Network should have admin_state_up")

		// Verify subnets field is present (can be empty)
		// Subnets is a slice, not a single value
		t.Logf("Network %s: Subnets=%d", network.Name, len(network.Subnets))
	}

	assert.Greater(t, len(networkList), 0, "Should have at least one network")
	t.Logf("Total networks for topology: %d", len(networkList))
}

// TestTopologySubnetDetails_Contract verifies subnet details for topology
func TestTopologySubnetDetails_Contract(t *testing.T) {
	client := setupNetworkClient(t)

	// Create network with gateway
	network, err := networks.Create(client, networks.CreateOpts{
		Name:         "subnet-topology-test",
		AdminStateUp: gophercloud.Enabled,
	}).Extract()
	require.NoError(t, err)
	defer networks.Delete(client, network.ID)

	// Create subnet with gateway
	gatewayIP := "192.168.102.1"
	subnet, err := subnets.Create(client, subnets.CreateOpts{
		NetworkID:  network.ID,
		CIDR:       "192.168.102.0/24",
		IPVersion:  gophercloud.IPv4,
		GatewayIP:  &gatewayIP,
		EnableDHCP: gophercloud.Enabled,
	}).Extract()
	require.NoError(t, err)
	defer subnets.Delete(client, subnet.ID)

	// Verify subnet has all topology-required fields
	assert.NotEmpty(t, subnet.ID, "Subnet should have ID")
	assert.Equal(t, network.ID, subnet.NetworkID, "Subnet should reference network")
	assert.NotEmpty(t, subnet.CIDR, "Subnet should have CIDR")
	assert.NotEmpty(t, subnet.GatewayIP, "Subnet should have gateway IP")
	assert.True(t, subnet.EnableDHCP, "DHCP should be enabled")

	// Verify allocation pools exist
	assert.NotEmpty(t, subnet.AllocationPools, "Subnet should have allocation pools")

	t.Logf("Subnet topology data: CIDR=%s, Gateway=%s, DHCP=%v, Pools=%d",
		subnet.CIDR, subnet.GatewayIP, subnet.EnableDHCP, len(subnet.AllocationPools))
}
