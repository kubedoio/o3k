package networking

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
)

// NetworkNamespaceManager manages Linux network namespaces
type NetworkNamespaceManager struct {
	nsPrefix string
}

// NewNetworkNamespaceManager creates a new namespace manager
func NewNetworkNamespaceManager() *NetworkNamespaceManager {
	return &NetworkNamespaceManager{
		nsPrefix: "light-ns-",
	}
}

// CreateNamespace creates a network namespace for a project
func (m *NetworkNamespaceManager) CreateNamespace(projectID string) error {
	nsName := m.nsPrefix + projectID

	// Check if namespace already exists
	if m.NamespaceExists(nsName) {
		return nil // Already exists
	}

	// Create namespace using ip netns add
	cmd := exec.Command("ip", "netns", "add", nsName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", nsName, err)
	}

	return nil
}

// DeleteNamespace deletes a network namespace
func (m *NetworkNamespaceManager) DeleteNamespace(projectID string) error {
	nsName := m.nsPrefix + projectID

	if !m.NamespaceExists(nsName) {
		return nil // Already deleted
	}

	cmd := exec.Command("ip", "netns", "delete", nsName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", nsName, err)
	}

	return nil
}

// NamespaceExists checks if a namespace exists
func (m *NetworkNamespaceManager) NamespaceExists(nsName string) bool {
	cmd := exec.Command("ip", "netns", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	namespaces := strings.Split(string(output), "\n")
	for _, ns := range namespaces {
		if strings.HasPrefix(ns, nsName) {
			return true
		}
	}

	return false
}

// GetNamespaceName returns the namespace name for a project
func (m *NetworkNamespaceManager) GetNamespaceName(projectID string) string {
	return m.nsPrefix + projectID
}

// ExecInNamespace executes a command in a namespace
func (m *NetworkNamespaceManager) ExecInNamespace(projectID string, args ...string) error {
	nsName := m.GetNamespaceName(projectID)
	fullArgs := append([]string{"netns", "exec", nsName}, args...)
	cmd := exec.Command("ip", fullArgs...)
	return cmd.Run()
}

// BridgeManager manages Linux bridges
type BridgeManager struct{}

// NewBridgeManager creates a new bridge manager
func NewBridgeManager() *BridgeManager {
	return &BridgeManager{}
}

// CreateBridge creates a bridge in the default or specified namespace
func (m *BridgeManager) CreateBridge(bridgeName string, inNamespace bool, nsName string) error {
	if inNamespace {
		// Create bridge in namespace using ip command
		cmd := exec.Command("ip", "netns", "exec", nsName, "ip", "link", "add", bridgeName, "type", "bridge")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create bridge %s in namespace %s: %w", bridgeName, nsName, err)
		}

		// Bring bridge up
		cmd = exec.Command("ip", "netns", "exec", nsName, "ip", "link", "set", bridgeName, "up")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to bring up bridge %s: %w", bridgeName, err)
		}
	} else {
		// Create bridge in default namespace using netlink
		bridge := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridgeName,
			},
		}

		if err := netlink.LinkAdd(bridge); err != nil {
			if !strings.Contains(err.Error(), "exists") {
				return fmt.Errorf("failed to create bridge %s: %w", bridgeName, err)
			}
		}

		// Bring bridge up
		link, err := netlink.LinkByName(bridgeName)
		if err != nil {
			return fmt.Errorf("failed to find bridge %s: %w", bridgeName, err)
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to bring up bridge %s: %w", bridgeName, err)
		}
	}

	return nil
}

// DeleteBridge deletes a bridge
func (m *BridgeManager) DeleteBridge(bridgeName string, inNamespace bool, nsName string) error {
	if inNamespace {
		cmd := exec.Command("ip", "netns", "exec", nsName, "ip", "link", "delete", bridgeName)
		return cmd.Run()
	}

	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil // Already deleted
	}

	return netlink.LinkDel(link)
}

// AttachToBridge attaches an interface to a bridge
func (m *BridgeManager) AttachToBridge(ifName, bridgeName string, inNamespace bool, nsName string) error {
	if inNamespace {
		cmd := exec.Command("ip", "netns", "exec", nsName, "ip", "link", "set", ifName, "master", bridgeName)
		return cmd.Run()
	}

	iface, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to find interface %s: %w", ifName, err)
	}

	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("failed to find bridge %s: %w", bridgeName, err)
	}

	return netlink.LinkSetMaster(iface, bridge.(*netlink.Bridge))
}

// TAPDeviceManager manages TAP devices
type TAPDeviceManager struct{}

// NewTAPDeviceManager creates a new TAP device manager
func NewTAPDeviceManager() *TAPDeviceManager {
	return &TAPDeviceManager{}
}

// CreateTAPDevice creates a TAP device
func (m *TAPDeviceManager) CreateTAPDevice(tapName string, inNamespace bool, nsName string) error {
	if inNamespace {
		cmd := exec.Command("ip", "netns", "exec", nsName, "ip", "tuntap", "add", tapName, "mode", "tap")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create TAP device %s: %w", tapName, err)
		}

		// Bring TAP device up
		cmd = exec.Command("ip", "netns", "exec", nsName, "ip", "link", "set", tapName, "up")
		return cmd.Run()
	}

	// Create TAP device in default namespace
	cmd := exec.Command("ip", "tuntap", "add", tapName, "mode", "tap")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create TAP device %s: %w", tapName, err)
	}

	// Bring TAP device up
	cmd = exec.Command("ip", "link", "set", tapName, "up")
	return cmd.Run()
}

// DeleteTAPDevice deletes a TAP device
func (m *TAPDeviceManager) DeleteTAPDevice(tapName string, inNamespace bool, nsName string) error {
	if inNamespace {
		cmd := exec.Command("ip", "netns", "exec", nsName, "ip", "link", "delete", tapName)
		return cmd.Run()
	}

	cmd := exec.Command("ip", "link", "delete", tapName)
	return cmd.Run()
}

// MoveTAPToNamespace moves a TAP device to a namespace
func (m *TAPDeviceManager) MoveTAPToNamespace(tapName, nsName string) error {
	cmd := exec.Command("ip", "link", "set", tapName, "netns", nsName)
	return cmd.Run()
}
