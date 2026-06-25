package neutron

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cobaltcore-dev/o3k/internal/database"
)

// BindPort prepares the host for a VM's network port.
// In stub mode: no-op. In flat mode: adds a dnsmasq reservation via FlatNetworkManager.
// In real mode: verifies bridge, adds DHCP lease, signals dnsmasq.
func (svc *Service) BindPort(portID, mac, ip, networkID, hostname string) error {
	if svc.mode == "stub" {
		return nil
	}

	// Flat mode: delegate to FlatNetworkManager.
	if svc.flatBridge != "" && svc.flatManager != nil {
		ctx := context.Background()
		var subnetID string
		_ = svc.activeDB().QueryRowContext(ctx,
			database.Q("SELECT id FROM subnets WHERE network_id::text = $1 LIMIT 1"),
			networkID,
		).Scan(&subnetID)
		if subnetID == "" {
			return nil
		}
		return svc.flatManager.AddDHCPReservation(subnetID, mac, ip, hostname)
	}

	if len(networkID) < 8 {
		return fmt.Errorf("network ID too short: %s", networkID)
	}
	bridgeName := fmt.Sprintf("br-%s", networkID[:8])

	if !svc.brManager.BridgeExists(bridgeName) {
		return fmt.Errorf("bridge %s does not exist for network %s", bridgeName, networkID)
	}

	hostsDir := "/var/lib/o3k/dhcp/hosts"
	if err := os.MkdirAll(hostsDir, 0755); err != nil {
		return fmt.Errorf("create hosts dir: %w", err)
	}

	hostsFile := filepath.Join(hostsDir, networkID)
	if err := svc.dhcpManager.AddStaticLease(hostsFile, mac, ip, hostname); err != nil {
		return fmt.Errorf("add DHCP lease for port %s: %w", portID, err)
	}

	if err := svc.dhcpManager.ReloadConfig(networkID); err != nil {
		fmt.Printf("warning: failed to reload dnsmasq for network %s: %v\n", networkID, err)
	}

	return nil
}

// UnbindPort removes the DHCP lease for a port.
func (svc *Service) UnbindPort(portID, mac, networkID string) error {
	if svc.mode == "stub" {
		return nil
	}

	hostsFile := filepath.Join("/var/lib/o3k/dhcp/hosts", networkID)
	if err := svc.dhcpManager.RemoveStaticLease(hostsFile, mac); err != nil {
		return fmt.Errorf("remove DHCP lease for port %s: %w", portID, err)
	}

	_ = svc.dhcpManager.ReloadConfig(networkID)
	return nil
}
