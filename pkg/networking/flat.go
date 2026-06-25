package networking

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const defaultFlatDataDir = "/var/lib/o3k"

// FlatDHCPConfig holds parameters for a dnsmasq instance on the flat bridge.
type FlatDHCPConfig struct {
	SubnetID   string
	BridgeName string
	CIDR       string
	GatewayIP  string
	RangeStart string
	RangeEnd   string
	LeaseTime  string
	DNS        string
}

// FlatNetworkManager manages dnsmasq DHCP for flat networking mode.
// In stub mode all operations succeed silently (no OS calls).
type FlatNetworkManager struct {
	mode    string
	bridge  string
	dataDir string
	mu      sync.Mutex
	pids    map[string]int // subnetID → dnsmasq pid
}

// NewFlatNetworkManager returns a manager using /var/lib/o3k as the data dir.
func NewFlatNetworkManager(mode, bridge string) *FlatNetworkManager {
	return NewFlatNetworkManagerWithDir(mode, bridge, defaultFlatDataDir)
}

// NewFlatNetworkManagerWithDir returns a manager using a custom data dir (tests).
func NewFlatNetworkManagerWithDir(mode, bridge, dataDir string) *FlatNetworkManager {
	return &FlatNetworkManager{
		mode:    mode,
		bridge:  bridge,
		dataDir: dataDir,
		pids:    make(map[string]int),
	}
}

// BridgeExists reports whether the flat bridge device exists on the host.
// Always returns true in stub mode.
func (m *FlatNetworkManager) BridgeExists() bool {
	if m.mode == "stub" {
		return true
	}
	_, err := exec.Command("ip", "link", "show", m.bridge).Output()
	return err == nil
}

// StartDHCP creates the hosts file and launches a dnsmasq instance for the
// given subnet. The hosts file is created in non-stub modes and in stub mode
// when a custom dataDir is configured. dnsmasq is launched only in non-stub
// mode.
func (m *FlatNetworkManager) StartDHCP(cfg FlatDHCPConfig) error {
	// Skip file creation in stub mode with the production data dir (no write
	// permission outside tests/real installs).
	if !(m.mode == "stub" && m.dataDir == defaultFlatDataDir) {
		if err := m.ensureHostsFile(cfg.SubnetID); err != nil {
			return err
		}
	}
	if m.mode == "stub" {
		return nil
	}

	hostsFile := m.hostsFilePath(cfg.SubnetID)
	pidFile := m.pidFilePath(cfg.SubnetID)

	args := []string{
		"--interface=" + cfg.BridgeName,
		"--bind-interfaces",
		"--dhcp-range=" + cfg.RangeStart + "," + cfg.RangeEnd + "," + cfg.LeaseTime,
		"--dhcp-option=3," + cfg.GatewayIP,
		"--dhcp-option=6," + cfg.DNS,
		"--pid-file=" + pidFile,
		"--dhcp-hostsfile=" + hostsFile,
		"--no-daemon",
	}

	cmd := exec.Command("dnsmasq", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnsmasq for subnet %s: %w", cfg.SubnetID, err)
	}

	m.mu.Lock()
	m.pids[cfg.SubnetID] = cmd.Process.Pid
	m.mu.Unlock()

	return nil
}

// StopDHCP terminates the dnsmasq instance for the given subnet.
// No-op in stub mode.
func (m *FlatNetworkManager) StopDHCP(subnetID string) error {
	if m.mode == "stub" {
		return nil
	}

	pidFile := m.pidFilePath(subnetID)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil // already stopped
	}
	pid := strings.TrimSpace(string(data))
	_ = exec.Command("kill", pid).Run()
	_ = os.Remove(pidFile)

	m.mu.Lock()
	delete(m.pids, subnetID)
	m.mu.Unlock()

	return nil
}

// AddDHCPReservation appends a MAC→IP reservation to the hosts file and
// signals dnsmasq to reload (SIGHUP). In stub mode with the default data dir
// the operation is a no-op; when a custom dataDir is set (e.g. in tests or
// real mode) the entry is written and dnsmasq is signalled if running.
func (m *FlatNetworkManager) AddDHCPReservation(subnetID, mac, ip, hostname string) error {
	hostsFile := m.hostsFilePath(subnetID)

	// In stub mode with the production data dir we have no write permission;
	// skip the file write.  Custom dirs (tests, real installs) proceed below.
	if m.mode == "stub" && m.dataDir == defaultFlatDataDir {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(hostsFile), 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	f, err := os.OpenFile(hostsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open hosts file %s: %w", hostsFile, err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s,%s,%s\n", mac, ip, hostname); err != nil {
		return fmt.Errorf("failed to write reservation: %w", err)
	}

	if m.mode == "stub" {
		return nil
	}

	pidFile := m.pidFilePath(subnetID)
	if data, err := os.ReadFile(pidFile); err == nil {
		pid := strings.TrimSpace(string(data))
		_ = exec.Command("kill", "-HUP", pid).Run()
	}

	return nil
}

func (m *FlatNetworkManager) hostsFilePath(subnetID string) string {
	return filepath.Join(m.dataDir, "dhcp-"+subnetID+".hosts")
}

func (m *FlatNetworkManager) pidFilePath(subnetID string) string {
	return filepath.Join(m.dataDir, "dhcp-"+subnetID+".pid")
}

func (m *FlatNetworkManager) ensureHostsFile(subnetID string) error {
	p := m.hostsFilePath(subnetID)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		f, err := os.Create(p)
		if err != nil {
			return fmt.Errorf("failed to create hosts file %s: %w", p, err)
		}
		return f.Close()
	}
	return nil
}
