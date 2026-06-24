package scheduler_test

import (
	"testing"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/scheduler"
	"github.com/cobaltcore-dev/o3k/internal/tunnel"
	"github.com/stretchr/testify/assert"
)

func TestNewWorker(t *testing.T) {
	db := database.NewTestDB(t)
	hub := tunnel.NewHub("test-secret")
	w := scheduler.NewWorker(db, hub)
	assert.NotNil(t, w)
}

func TestWorker_ProcessOne_NoTasks(t *testing.T) {
	// Real in-memory DB: claimTask begins a transaction, finds no pending task,
	// rolls back, and returns. The hub must not be called; the test passes if
	// no panic occurs.
	db := database.NewTestDB(t)
	hub := tunnel.NewHub("test-secret")
	w := scheduler.NewWorker(db, hub)

	w.ProcessOne(t.Context())
}
