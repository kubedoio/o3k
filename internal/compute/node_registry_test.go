package compute_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/compute"
	"github.com/stretchr/testify/assert"
)

func TestNodeRegistryPersistsUUID(t *testing.T) {
	dir := t.TempDir()
	idFile := filepath.Join(dir, "node-id")

	r1, err := compute.NewNodeRegistryWithIDPath("auto", "127.0.0.1", time.Second, idFile)
	assert.NoError(t, err)
	id1 := r1.GetNodeID()
	assert.NotEmpty(t, id1)

	r2, err := compute.NewNodeRegistryWithIDPath("auto", "127.0.0.1", time.Second, idFile)
	assert.NoError(t, err)
	assert.Equal(t, id1, r2.GetNodeID(), "UUID must be stable across restarts")

	_, err = os.Stat(idFile)
	assert.NoError(t, err)
}
