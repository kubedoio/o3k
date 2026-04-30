package networking_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cobaltcore-dev/o3k/pkg/networking"
	"github.com/stretchr/testify/assert"
)

func TestAddStaticLease(t *testing.T) {
	mgr := networking.NewDHCPManager("stub")

	dir := t.TempDir()
	hostsFile := filepath.Join(dir, "hosts")
	os.WriteFile(hostsFile, []byte{}, 0644)

	err := mgr.AddStaticLease(hostsFile, "fa:16:3e:aa:bb:cc", "192.168.1.50", "test-vm")
	assert.NoError(t, err)

	content, _ := os.ReadFile(hostsFile)
	assert.Contains(t, string(content), "fa:16:3e:aa:bb:cc,192.168.1.50,test-vm")
}

func TestRemoveStaticLease(t *testing.T) {
	mgr := networking.NewDHCPManager("stub")

	dir := t.TempDir()
	hostsFile := filepath.Join(dir, "hosts")
	os.WriteFile(hostsFile, []byte("fa:16:3e:aa:bb:cc,192.168.1.50,test-vm\nfa:16:3e:dd:ee:ff,192.168.1.51,other-vm\n"), 0644)

	err := mgr.RemoveStaticLease(hostsFile, "fa:16:3e:aa:bb:cc")
	assert.NoError(t, err)

	content, _ := os.ReadFile(hostsFile)
	assert.NotContains(t, string(content), "fa:16:3e:aa:bb:cc")
	assert.Contains(t, string(content), "fa:16:3e:dd:ee:ff")
}

func TestReloadConfigStubMode(t *testing.T) {
	mgr := networking.NewDHCPManager("stub")
	err := mgr.ReloadConfig("net-123")
	assert.NoError(t, err)
}
