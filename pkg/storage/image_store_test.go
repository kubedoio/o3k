package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestNewImageStore(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	if store == nil {
		t.Fatal("Expected non-nil store")
	}

	if store.cephPool != "test-pool" {
		t.Errorf("Expected pool test-pool, got %s", store.cephPool)
	}

	if store.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", store.timeout)
	}
}

func TestUploadImageStub(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	data := []byte("fake image data")
	reader := bytes.NewReader(data)

	size, err := store.UploadImage(ctx, "test-image-123", reader)
	if err != nil {
		t.Errorf("UploadImage failed: %v", err)
	}

	// Stub returns 0
	if size != 0 {
		t.Errorf("Expected size 0 (stub), got %d", size)
	}
}

func TestDownloadImageStub(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	var buf bytes.Buffer
	err := store.DownloadImage(ctx, "test-image-123", &buf)
	if err != nil {
		t.Errorf("DownloadImage failed: %v", err)
	}

	// Stub doesn't write anything
	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer (stub), got %d bytes", buf.Len())
	}
}

func TestDeleteImageStub(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	err := store.DeleteImage(ctx, "test-image-123")
	if err != nil {
		t.Errorf("DeleteImage failed: %v", err)
	}
}

func TestImageExistsStub(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	exists, err := store.ImageExists(ctx, "test-image-123")
	if err != nil {
		t.Errorf("ImageExists failed: %v", err)
	}

	// Stub returns false
	if exists {
		t.Error("Expected ImageExists to return false (stub)")
	}
}

func TestGetImageSizeStub(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	size, err := store.GetImageSize(ctx, "test-image-123")
	if err != nil {
		t.Errorf("GetImageSize failed: %v", err)
	}

	// Stub returns 0
	if size != 0 {
		t.Errorf("Expected size 0 (stub), got %d", size)
	}
}

func TestGetRBDPathImage(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")

	path := store.GetRBDPath("test-image-123")
	expectedPath := "rbd:test-pool/image-test-image-123"

	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}
}

func TestUploadImageWithLargeDataStub(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	// Create a large fake image (10MB)
	data := make([]byte, 10*1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	reader := bytes.NewReader(data)

	size, err := store.UploadImage(ctx, "large-image-123", reader)
	if err != nil {
		t.Errorf("UploadImage with large data failed: %v", err)
	}

	// Stub returns 0
	if size != 0 {
		t.Errorf("Expected size 0 (stub), got %d", size)
	}
}

func TestDownloadImageToNilWriterStub(t *testing.T) {
	store := NewImageStore("test-pool", "/etc/ceph/ceph.conf")
	ctx := context.Background()

	// Use a writer that discards data
	err := store.DownloadImage(ctx, "test-image-123", io.Discard)
	if err != nil {
		t.Errorf("DownloadImage to discard writer failed: %v", err)
	}
}
