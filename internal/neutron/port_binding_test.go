package neutron_test

import (
	"context"
	"testing"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/neutron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bindTestDB(t *testing.T) {
	t.Helper()
	require.NoError(t, database.ConnectSQLite(context.Background(), ":memory:"), "connecting to test SQLite database")
	t.Cleanup(database.Close)
}

func TestBindPortStubMode(t *testing.T) {
	bindTestDB(t)
	svc := neutron.NewServiceWithDB(database.DB, "stub", nil)

	err := svc.BindPort("port-123", "fa:16:3e:aa:bb:cc", "192.168.1.50", "net-abcdef12", "test-vm")
	assert.NoError(t, err)
}

func TestUnbindPortStubMode(t *testing.T) {
	bindTestDB(t)
	svc := neutron.NewServiceWithDB(database.DB, "stub", nil)

	err := svc.UnbindPort("port-123", "fa:16:3e:aa:bb:cc", "net-abcdef12")
	assert.NoError(t, err)
}
