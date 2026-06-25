package networking_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cobaltcore-dev/o3k/pkg/networking"
)

func TestFlatNetworkManagerStubMode(t *testing.T) {
	m := networking.NewFlatNetworkManager("stub", "br-o3k")

	if !m.BridgeExists() {
		t.Fatal("BridgeExists() should return true in stub mode")
	}

	cfg := networking.FlatDHCPConfig{
		SubnetID:   "test-subnet-id",
		BridgeName: "br-o3k",
		CIDR:       "192.168.100.0/24",
		GatewayIP:  "192.168.100.1",
		RangeStart: "192.168.100.10",
		RangeEnd:   "192.168.100.200",
		LeaseTime:  "24h",
		DNS:        "8.8.8.8",
	}
	if err := m.StartDHCP(cfg); err != nil {
		t.Fatalf("StartDHCP stub mode should not error: %v", err)
	}
	if err := m.StopDHCP("test-subnet-id"); err != nil {
		t.Fatalf("StopDHCP stub mode should not error: %v", err)
	}
	if err := m.AddDHCPReservation("test-subnet-id", "fa:16:3e:aa:bb:cc", "192.168.100.10", "test-vm"); err != nil {
		t.Fatalf("AddDHCPReservation stub mode should not error: %v", err)
	}
}

func TestFlatNetworkManagerHostsFile(t *testing.T) {
	dir := t.TempDir()
	m := networking.NewFlatNetworkManagerWithDir("stub", "br-o3k", dir)

	subnetID := "subnet-abc123"
	cfg := networking.FlatDHCPConfig{
		SubnetID:   subnetID,
		BridgeName: "br-o3k",
		CIDR:       "192.168.100.0/24",
		GatewayIP:  "192.168.100.1",
		RangeStart: "192.168.100.10",
		RangeEnd:   "192.168.100.200",
		LeaseTime:  "24h",
		DNS:        "8.8.8.8",
	}

	if err := m.StartDHCP(cfg); err != nil {
		t.Fatalf("StartDHCP: %v", err)
	}
	hostsFile := filepath.Join(dir, "dhcp-"+subnetID+".hosts")
	if _, err := os.Stat(hostsFile); err != nil {
		t.Fatalf("hosts file not created: %v", err)
	}

	if err := m.AddDHCPReservation(subnetID, "fa:16:3e:aa:bb:cc", "192.168.100.10", "test-vm"); err != nil {
		t.Fatalf("AddDHCPReservation: %v", err)
	}
	data, _ := os.ReadFile(hostsFile)
	if string(data) != "fa:16:3e:aa:bb:cc,192.168.100.10,test-vm\n" {
		t.Fatalf("unexpected hosts file content: %q", string(data))
	}
}
