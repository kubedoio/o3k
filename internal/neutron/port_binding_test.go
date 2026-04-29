package neutron_test

import (
	"testing"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/neutron"
	"github.com/stretchr/testify/assert"
)

func TestBindPortStubMode(t *testing.T) {
	mock := database.NewMockDB()
	svc := neutron.NewServiceWithDB(mock, "stub", nil)

	err := svc.BindPort("port-123", "fa:16:3e:aa:bb:cc", "192.168.1.50", "net-abcdef12", "test-vm")
	assert.NoError(t, err)
}

func TestUnbindPortStubMode(t *testing.T) {
	mock := database.NewMockDB()
	svc := neutron.NewServiceWithDB(mock, "stub", nil)

	err := svc.UnbindPort("port-123", "fa:16:3e:aa:bb:cc", "net-abcdef12")
	assert.NoError(t, err)
}
