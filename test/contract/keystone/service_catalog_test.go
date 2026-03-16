package keystone_test

import (
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
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

// TestServiceCatalogCompleteness_Contract verifies service catalog contains all required services
func TestServiceCatalogCompleteness_Contract(t *testing.T) {
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

	// Service catalog is available in ProviderClient.EndpointLocator
	// We'll test by trying to get endpoints for each service type
	requiredServices := map[string]string{
		"identity": "keystone",
		"compute":  "nova",
		"network":  "neutron",
		"volume":   "cinder",
		"image":    "glance",
	}

	foundServices := make(map[string]bool)

	// Test each service by trying to get its public endpoint
	for serviceType, serviceName := range requiredServices {
		endpoint, err := provider.EndpointLocator(gophercloud.EndpointOpts{
			Type:         serviceType,
			Availability: gophercloud.AvailabilityPublic,
		})

		if err != nil {
			t.Errorf("Service %s (%s) not found in catalog: %v", serviceName, serviceType, err)
		} else {
			assert.NotEmpty(t, endpoint, "Endpoint for service %s should not be empty", serviceName)
			foundServices[serviceType] = true
			t.Logf("Service %s (%s) found with endpoint: %s", serviceName, serviceType, endpoint)
		}
	}

	// Verify all required services were found
	for serviceType, serviceName := range requiredServices {
		assert.True(t, foundServices[serviceType],
			"Required service %s (%s) not found in catalog", serviceName, serviceType)
	}

	t.Logf("Service catalog validation passed: %d/%d services found",
		len(foundServices), len(requiredServices))
}

// TestServiceCatalogEndpoints_Contract verifies all services have public endpoints
func TestServiceCatalogEndpoints_Contract(t *testing.T) {
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

	serviceTypes := []string{"identity", "compute", "network", "volume", "image"}

	for _, serviceType := range serviceTypes {
		// Get public endpoint for service
		endpoint, err := provider.EndpointLocator(gophercloud.EndpointOpts{
			Type:         serviceType,
			Availability: gophercloud.AvailabilityPublic,
		})

		require.NoError(t, err, "Failed to get endpoint for service type %s", serviceType)
		require.NotEmpty(t, endpoint, "Endpoint should not be empty")

		// Verify URL is well-formed
		assert.Contains(t, endpoint, "http",
			"Public endpoint URL should contain http/https")
		assert.NotContains(t, endpoint, "$(",
			"Endpoint URL should not contain template variables")

		t.Logf("Service %s public endpoint: %s", serviceType, endpoint)
	}
}

// TestServiceCatalogHorizonCompatibility_Contract verifies catalog format matches Horizon expectations
func TestServiceCatalogHorizonCompatibility_Contract(t *testing.T) {
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

	// Test that all required service types can be located
	// This validates the service catalog is correctly formatted
	horizonServices := []string{"identity", "compute", "network", "volume", "image"}

	for _, serviceType := range horizonServices {
		endpoint, err := provider.EndpointLocator(gophercloud.EndpointOpts{
			Type:         serviceType,
			Availability: gophercloud.AvailabilityPublic,
		})

		assert.NoError(t, err, "Service type %s should be locatable (Horizon requirement)", serviceType)
		assert.NotEmpty(t, endpoint, "Service type %s should have endpoint", serviceType)
	}

	t.Log("Service catalog is Horizon-compatible")
}
