package storage

import (
	"context"
	"testing"
	"time"
)

func TestNewCephClient(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.pool != "test-pool" {
		t.Errorf("Expected pool test-pool, got %s", client.pool)
	}

	if client.timeout != 1*time.Second {
		t.Errorf("Expected timeout 1s, got %v", client.timeout)
	}
}

func TestCreateVolumeStub(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	err := client.CreateVolume(ctx, "test-volume-123", 10)
	if err != nil {
		t.Errorf("CreateVolume failed: %v", err)
	}
}

func TestDeleteVolumeStub(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	err := client.DeleteVolume(ctx, "test-volume-123")
	if err != nil {
		t.Errorf("DeleteVolume failed: %v", err)
	}
}

func TestCreateSnapshotStub(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	err := client.CreateSnapshot(ctx, "test-volume-123", "test-snapshot-456")
	if err != nil {
		t.Errorf("CreateSnapshot failed: %v", err)
	}
}

func TestDeleteSnapshotStub(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	err := client.DeleteSnapshot(ctx, "test-volume-123", "test-snapshot-456")
	if err != nil {
		t.Errorf("DeleteSnapshot failed: %v", err)
	}
}

func TestVolumeExistsStub(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	exists, err := client.VolumeExists(ctx, "test-volume-123")
	if err != nil {
		t.Errorf("VolumeExists failed: %v", err)
	}

	// Stub always returns true
	if !exists {
		t.Error("Expected VolumeExists to return true (stub)")
	}
}

func TestGetVolumeSizeStub(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	size, err := client.GetVolumeSize(ctx, "test-volume-123")
	if err != nil {
		t.Errorf("GetVolumeSize failed: %v", err)
	}

	// Stub returns 0
	if size != 0 {
		t.Errorf("Expected size 0 (stub), got %d", size)
	}
}

func TestGetRBDPath(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")

	path := client.GetRBDPath("test-volume-123")
	expectedPath := "rbd:test-pool/volume-test-volume-123"

	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}
}

func TestHealthStub(t *testing.T) {
	client := NewCephClient("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	err := client.Health(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}
