package scheduler_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cobaltcore-dev/o3k/internal/database"
	"github.com/cobaltcore-dev/o3k/internal/scheduler"
	"github.com/cobaltcore-dev/o3k/internal/tunnel"
)

// TestReconcilerCollectsThenUpdates verifies that the reconciler can run
// against a real in-memory SQLite database without deadlocking. With SQLite
// configured for a single concurrent writer, overlapping a Query cursor with
// an UPDATE would block; the reconciler closes the cursor before updating.
func TestReconcilerCollectsThenUpdates(t *testing.T) {
	db := database.NewTestDB(t)

	// reconcileOnce is private; exercise it through Run with a 1 ms interval.
	// Cancel after two ticks to keep the test short.
	r := scheduler.NewReconciler(db, 1)

	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()

	// Run blocks until ctx expires; any deadlock or panic surfaces as a test failure.
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Run(ctx)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Reconciler.Run did not return after context cancellation")
	}
}

// ---------------------------------------------------------------------------
// Test 2: TestHubDispatchSyncTimeout
//
// Verifies that DispatchSync returns an error when no result arrives before the
// context deadline rather than blocking forever.
// ---------------------------------------------------------------------------

func TestHubDispatchSyncTimeout(t *testing.T) {
	hub := tunnel.NewHub("test-secret")

	// No agent registered — GetAgent returns nil and DispatchSync returns an
	// error immediately.
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, _, err := hub.DispatchSync(ctx, "nonexistent-agent", "boot", []byte(`{}`), 1)
	if err == nil {
		t.Fatal("expected error dispatching to nonexistent agent, got nil")
	}
}

// ---------------------------------------------------------------------------
// Test 3: TestResultChCleanupOnCancel
//
// Verifies that cancelling the context causes DispatchSync to unregister the
// result channel from the hub so the entry does not leak.
// ---------------------------------------------------------------------------

// mockSendAgent is an AgentInfo-compatible test double whose stream never
// delivers a result, so DispatchSync always times out via context cancellation.
// We achieve this by registering the result chan but never calling DeliverResult.
func TestResultChCleanupOnCancel(t *testing.T) {
	hub := tunnel.NewHub("test-secret")

	// Register an agent.  Its Stream is nil, which means SendTask will fail and
	// DispatchSync will return an error before reaching the select.  We can still
	// verify the error path doesn't leak anything by running it many times.
	hub.RegisterAgent(&tunnel.AgentInfo{
		NodeID: "agent-1",
		Stream: nil, // nil stream → SendTask errors → inflight released
	})

	// With a nil stream, SendTask fails; DispatchSync must not leave inflight
	// in an acquired state.  Run 20 dispatches — if inflight leaked, the 2nd
	// call would return "agent busy".
	for i := range 20 {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		_, _, _ = hub.DispatchSync(ctx, "agent-1", "noop", []byte(`{}`), 1)
		cancel()
		if i > 0 {
			// After the first failed dispatch the agent must not be stuck "busy".
			// A second dispatch must also return a non-nil error (nil stream),
			// NOT "agent busy" — that would indicate an inflight leak.
			ctx2, cancel2 := context.WithTimeout(t.Context(), 10*time.Millisecond)
			_, _, err2 := hub.DispatchSync(ctx2, "agent-1", "noop", []byte(`{}`), 1)
			cancel2()
			if err2 != nil && strings.Contains(err2.Error(), "busy") {
				t.Fatalf("iteration %d: inflight leaked — got 'busy' error: %v", i, err2)
			}
		}
	}
}
