package hypervisor

import (
	"strings"
	"testing"
)

func TestGenerateVMXML(t *testing.T) {
	spec := VMSpec{
		Name:      "test-vm",
		UUID:      "550e8400-e29b-41d4-a716-446655440000",
		MemoryMB:  2048,
		VCPUs:     2,
		ImagePath: "/var/lib/libvirt/images/test.qcow2",
		Networks: []NetworkConfig{
			{
				BridgeName: "br-test",
				MACAddress: "52:54:00:12:34:56",
			},
		},
	}

	xml := GenerateVMXML(spec)

	if xml == "" {
		t.Fatal("Generated XML is empty")
	}

	// Check that key elements are present
	requiredElements := []string{
		"<name>test-vm</name>",
		"<uuid>550e8400-e29b-41d4-a716-446655440000</uuid>",
		"<memory unit='MiB'>2048</memory>",
		"<vcpu placement='static'>2</vcpu>",
		"<disk type='file' device='disk'>",
		"<source file='/var/lib/libvirt/images/test.qcow2'/>",
		"<interface type='bridge'>",
		"<source bridge='br-test'/>",
		"<mac address='52:54:00:12:34:56'/>",
	}

	for _, element := range requiredElements {
		if !strings.Contains(xml, element) {
			t.Errorf("XML missing required element: %s", element)
		}
	}
}

func TestGenerateVMXMLMultipleNetworks(t *testing.T) {
	spec := VMSpec{
		Name:      "multi-net-vm",
		UUID:      "550e8400-e29b-41d4-a716-446655440001",
		MemoryMB:  1024,
		VCPUs:     1,
		ImagePath: "/var/lib/libvirt/images/multi.qcow2",
		Networks: []NetworkConfig{
			{
				BridgeName: "br-net1",
				MACAddress: "52:54:00:12:34:56",
			},
			{
				BridgeName: "br-net2",
				MACAddress: "52:54:00:12:34:57",
			},
		},
	}

	xml := GenerateVMXML(spec)

	// Check both networks are present
	if !strings.Contains(xml, "br-net1") {
		t.Error("XML missing first network")
	}

	if !strings.Contains(xml, "br-net2") {
		t.Error("XML missing second network")
	}
}

func TestGenerateVMXMLWithVolumes(t *testing.T) {
	spec := VMSpec{
		Name:      "vm-with-volumes",
		UUID:      "550e8400-e29b-41d4-a716-446655440002",
		MemoryMB:  1024,
		VCPUs:     1,
		ImagePath: "/var/lib/libvirt/images/root.qcow2",
		Volumes: []VolumeConfig{
			{
				RBDPool:  "volumes",
				RBDImage: "volume-123",
				Device:   "vdb",
			},
		},
		Networks: []NetworkConfig{
			{
				BridgeName: "br-test",
				MACAddress: "52:54:00:12:34:56",
			},
		},
	}

	xml := GenerateVMXML(spec)

	// Check RBD volume is present
	requiredElements := []string{
		"<disk type='network' device='disk'>",
		"<source protocol='rbd' name='volumes/volume-123'>",
		"<target dev='vdb' bus='virtio'/>",
	}

	for _, element := range requiredElements {
		if !strings.Contains(xml, element) {
			t.Errorf("XML missing required volume element: %s", element)
		}
	}
}

func TestGenerateVMXMLWithRBDImage(t *testing.T) {
	spec := VMSpec{
		Name:      "rbd-vm",
		UUID:      "550e8400-e29b-41d4-a716-446655440003",
		MemoryMB:  1024,
		VCPUs:     1,
		ImagePath: "rbd:images/cirros",
		Networks: []NetworkConfig{
			{
				BridgeName: "br-test",
				MACAddress: "52:54:00:12:34:56",
			},
		},
	}

	xml := GenerateVMXML(spec)

	// Check RBD boot disk is present
	if !strings.Contains(xml, "<source protocol='rbd' name='images/cirros'>") {
		t.Error("XML missing RBD boot disk")
	}
}

// TestGenerateVMXML_RBDDefaultMonitor confirms the legacy 127.0.0.1:6789
// fallback fires when CephMonitors is empty. This guards backwards
// compatibility — single-node dev clusters don't need to configure
// monitors explicitly.
func TestGenerateVMXML_RBDDefaultMonitor(t *testing.T) {
	spec := VMSpec{
		Name:      "rbd-default",
		UUID:      "550e8400-e29b-41d4-a716-446655440010",
		MemoryMB:  512,
		VCPUs:     1,
		ImagePath: "rbd:images/cirros",
	}
	xml := GenerateVMXML(spec)
	if !strings.Contains(xml, "<host name='127.0.0.1' port='6789'/>") {
		t.Error("expected 127.0.0.1:6789 default monitor when CephMonitors is empty")
	}
}

// TestGenerateVMXML_RBDCustomMonitors verifies explicit monitor configuration
// produces one <host> element per entry, with port defaulting to 6789 when
// the entry omits a port.
func TestGenerateVMXML_RBDCustomMonitors(t *testing.T) {
	spec := VMSpec{
		Name:      "rbd-custom",
		UUID:      "550e8400-e29b-41d4-a716-446655440011",
		MemoryMB:  512,
		VCPUs:     1,
		ImagePath: "rbd:images/cirros",
		CephMonitors: []string{
			"mon1.ceph.local:6789",
			"10.0.0.5:6790",
			"mon3.ceph.local", // no port — should default to 6789
		},
	}
	xml := GenerateVMXML(spec)
	expected := []string{
		"<host name='mon1.ceph.local' port='6789'/>",
		"<host name='10.0.0.5' port='6790'/>",
		"<host name='mon3.ceph.local' port='6789'/>",
	}
	for _, e := range expected {
		if !strings.Contains(xml, e) {
			t.Errorf("XML missing expected monitor element: %s", e)
		}
	}
	if strings.Contains(xml, "<host name='127.0.0.1'") {
		t.Error("XML still contains hardcoded 127.0.0.1 fallback when CephMonitors is set")
	}
}

// TestGenerateVMXML_RBDVolumesUseMonitors confirms attached RBD volumes also
// pick up the configured monitors, not just the boot disk.
func TestGenerateVMXML_RBDVolumesUseMonitors(t *testing.T) {
	spec := VMSpec{
		Name:      "rbd-vol",
		UUID:      "550e8400-e29b-41d4-a716-446655440012",
		MemoryMB:  512,
		VCPUs:     1,
		ImagePath: "/var/lib/libvirt/images/root.qcow2",
		Volumes: []VolumeConfig{
			{RBDPool: "volumes", RBDImage: "vol-1", Device: "vdb"},
		},
		CephMonitors: []string{"mon1.example.com:6789"},
	}
	xml := GenerateVMXML(spec)
	if !strings.Contains(xml, "<host name='mon1.example.com' port='6789'/>") {
		t.Error("attached volume XML missing configured monitor")
	}
	if strings.Contains(xml, "<host name='127.0.0.1'") {
		t.Error("attached volume XML still uses hardcoded 127.0.0.1 fallback")
	}
}

// TestGenerateDiskXML_RBDCustomMonitors covers the disk-attachment path used
// by Nova volume_attachment.
func TestGenerateDiskXML_RBDCustomMonitors(t *testing.T) {
	xml := GenerateDiskXML(DiskSpec{
		Device:       "/dev/vdb",
		Type:         "network",
		Source:       "volumes/vol-9",
		Protocol:     "rbd",
		CephMonitors: []string{"mon-a:6789", "mon-b:6789"},
	})
	if !strings.Contains(xml, "<host name='mon-a' port='6789'/>") ||
		!strings.Contains(xml, "<host name='mon-b' port='6789'/>") {
		t.Errorf("disk XML missing expected monitors:\n%s", xml)
	}
	if strings.Contains(xml, "<host name='127.0.0.1'") {
		t.Error("disk XML still uses hardcoded fallback when monitors set")
	}
}

// TestGenerateDiskXML_RBDDefaultMonitor confirms the disk path also
// preserves the legacy default.
func TestGenerateDiskXML_RBDDefaultMonitor(t *testing.T) {
	xml := GenerateDiskXML(DiskSpec{
		Device:   "/dev/vdb",
		Type:     "network",
		Source:   "volumes/vol-9",
		Protocol: "rbd",
	})
	if !strings.Contains(xml, "<host name='127.0.0.1' port='6789'/>") {
		t.Error("expected default 127.0.0.1:6789 when DiskSpec.CephMonitors is empty")
	}
}

func TestDefaultCloudInitConfig(t *testing.T) {
	hostname := "test-vm"
	sshKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQtest"

	config := DefaultCloudInitConfig(hostname, sshKey)

	if config == nil {
		t.Fatal("Expected non-nil cloud-init config")
	}

	if !strings.Contains(config.MetaData, hostname) {
		t.Error("MetaData missing hostname")
	}

	if !strings.Contains(config.UserData, sshKey) {
		t.Error("UserData missing SSH key")
	}

	if !strings.Contains(config.UserData, "#cloud-config") {
		t.Error("UserData missing cloud-config header")
	}
}

