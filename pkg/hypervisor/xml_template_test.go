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

