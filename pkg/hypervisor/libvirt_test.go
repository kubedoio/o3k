package hypervisor

import (
	"context"
	"testing"
)

func TestNewVMManager(t *testing.T) {
	manager, err := NewVMManager("test:///default")
	if err != nil {
		t.Fatalf("Failed to create VM manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.libvirtURI != "test:///default" {
		t.Errorf("Expected URI test:///default, got %s", manager.libvirtURI)
	}
}

func TestCreateVMStub(t *testing.T) {
	manager, _ := NewVMManager("test:///default")
	ctx := context.Background()

	// The stub implementation returns an error
	_, err := manager.CreateVM(ctx, "<domain></domain>")
	if err == nil {
		t.Error("Expected error from stub implementation, got nil")
	}
}

func TestDeleteVMStub(t *testing.T) {
	manager, _ := NewVMManager("test:///default")
	ctx := context.Background()

	err := manager.DeleteVM(ctx, "test-uuid")
	if err == nil {
		t.Error("Expected error from stub implementation, got nil")
	}
}

func TestRebootVMStub(t *testing.T) {
	manager, _ := NewVMManager("test:///default")
	ctx := context.Background()

	err := manager.RebootVM(ctx, "test-uuid")
	if err == nil {
		t.Error("Expected error from stub implementation, got nil")
	}
}

func TestStopVMStub(t *testing.T) {
	manager, _ := NewVMManager("test:///default")
	ctx := context.Background()

	err := manager.StopVM(ctx, "test-uuid")
	if err == nil {
		t.Error("Expected error from stub implementation, got nil")
	}
}

func TestStartVMStub(t *testing.T) {
	manager, _ := NewVMManager("test:///default")
	ctx := context.Background()

	err := manager.StartVM(ctx, "test-uuid")
	if err == nil {
		t.Error("Expected error from stub implementation, got nil")
	}
}

