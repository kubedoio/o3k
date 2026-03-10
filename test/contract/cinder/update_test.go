package cinder

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCinderVolumeUpdate_Contract tests PATCH /v3/{project_id}/volumes/{id}
func TestCinderVolumeUpdate_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Setup: Create volume
	volumeName := "test-volume-update-" + uuid.New().String()[:8]
	volume, err := volumes.Create(context.Background(), client, volumes.CreateOpts{
		Size: 1,
		Name: volumeName,
	}, nil).Extract()
	require.NoError(t, err, "Setup: CreateVolume should succeed")
	defer volumes.Delete(context.Background(), client, volume.ID, volumes.DeleteOpts{})

	// Test: Update volume name and description
	newName := volumeName + "-updated"
	newDesc := "Updated description"
	updateOpts := volumes.UpdateOpts{
		Name:        &newName,
		Description: &newDesc,
	}

	updated, err := volumes.Update(context.Background(), client, volume.ID, updateOpts).Extract()

	// Assertions
	require.NoError(t, err, "UpdateVolume should succeed")
	assert.Equal(t, newName, updated.Name, "Volume name should be updated")
	assert.Equal(t, newDesc, updated.Description, "Volume description should be updated")
	assert.Equal(t, volume.ID, updated.ID, "Volume ID should not change")
}

// TestCinderSnapshotUpdate_Contract tests PATCH /v3/{project_id}/snapshots/{id}
func TestCinderSnapshotUpdate_Contract(t *testing.T) {
	skipIfO3KNotRunning(t)

	client := setupCinderClient(t)

	// Setup: Create volume and snapshot
	volumeName := "test-volume-for-snap-" + uuid.New().String()[:8]
	volume, err := volumes.Create(context.Background(), client, volumes.CreateOpts{
		Size: 1,
		Name: volumeName,
	}, nil).Extract()
	require.NoError(t, err, "Setup: CreateVolume should succeed")
	defer volumes.Delete(context.Background(), client, volume.ID, volumes.DeleteOpts{})

	snapName := "test-snapshot-update-" + uuid.New().String()[:8]
	snapshot, err := snapshots.Create(context.Background(), client, snapshots.CreateOpts{
		VolumeID: volume.ID,
		Name:     snapName,
	}).Extract()
	require.NoError(t, err, "Setup: CreateSnapshot should succeed")
	defer snapshots.Delete(context.Background(), client, snapshot.ID)

	// Test: Update snapshot name and description
	newName := snapName + "-updated"
	newDesc := "Updated snapshot description"
	updateOpts := snapshots.UpdateOpts{
		Name:        &newName,
		Description: &newDesc,
	}

	updated, err := snapshots.Update(context.Background(), client, snapshot.ID, updateOpts).Extract()

	// Assertions
	require.NoError(t, err, "UpdateSnapshot should succeed")
	assert.Equal(t, newName, updated.Name, "Snapshot name should be updated")
	assert.Equal(t, newDesc, updated.Description, "Snapshot description should be updated")
	assert.Equal(t, snapshot.ID, updated.ID, "Snapshot ID should not change")
}
