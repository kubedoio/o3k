package neutron_test

import (
	"os"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNeutronClient(t *testing.T) *gophercloud.ServiceClient {
	t.Helper()

	authURL := getEnvOrDefault("OS_AUTH_URL", "http://localhost:35357/v3")
	username := getEnvOrDefault("OS_USERNAME", "admin")
	password := getEnvOrDefault("OS_PASSWORD", "secret")
	projectName := getEnvOrDefault("OS_PROJECT_NAME", "default")
	domainName := getEnvOrDefault("OS_USER_DOMAIN_NAME", "Default")

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       projectName,
		DomainName:       domainName,
	}

	provider, err := openstack.AuthenticatedClient(opts)
	require.NoError(t, err, "Failed to create authenticated client")

	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{})
	require.NoError(t, err, "Failed to create Neutron client")

	return client
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func skipIfO3KNotRunning(t *testing.T) {
	t.Helper()
	if os.Getenv("SKIP_CONTRACT_TESTS") == "1" {
		t.Skip("Skipping contract test (O3K not running)")
	}
}

func TestNeutronListExtensions_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupNeutronClient(t)

	// List API extensions
	allPages, err := extensions.List(client).AllPages()
	require.NoError(t, err)

	extensionsList, err := extensions.ExtractExtensions(allPages)
	require.NoError(t, err)

	// Should have at least some standard extensions
	assert.NotEmpty(t, extensionsList)

	// Verify extension structure
	foundExtensions := make(map[string]bool)
	for _, ext := range extensionsList {
		assert.NotEmpty(t, ext.Alias)
		assert.NotEmpty(t, ext.Name)
		foundExtensions[ext.Alias] = true
	}

	// Check for common extensions
	commonExtensions := []string{"security-group", "router", "port-security"}
	for _, extAlias := range commonExtensions {
		if foundExtensions[extAlias] {
			t.Logf("Found expected extension: %s", extAlias)
		}
	}
}
