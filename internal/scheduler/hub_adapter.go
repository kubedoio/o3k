package scheduler

import (
	"context"
	"fmt"

	"github.com/cobaltcore-dev/o3k/internal/tunnel"
)

// HubAdapter adapts tunnel.Hub to the scheduler.Dispatcher interface.
type HubAdapter struct {
	hub *tunnel.Hub
}

// NewHubAdapter wraps hub so it satisfies the scheduler.Dispatcher interface.
func NewHubAdapter(hub *tunnel.Hub) *HubAdapter {
	return &HubAdapter{hub: hub}
}

// Dispatch picks an available agent, validates the task, and sends it via the
// tunnel stream.  For v1 (single-server) it returns immediately after the send;
// the reconciler handles timeouts for tasks that never complete.
func (h *HubAdapter) Dispatch(ctx context.Context, agentID string, taskType string, payload []byte, timeoutSec int) ([]byte, string, error) {
	agent := h.hub.PickAgent()
	if agent == nil {
		return nil, "", fmt.Errorf("no agents connected")
	}
	if agent.Stream == nil {
		return nil, "", fmt.Errorf("agent %s has no active stream", agent.NodeID)
	}

	task := tunnel.Task{
		Type:    taskType,
		Payload: payload,
	}
	if err := task.Validate(); err != nil {
		return nil, "", err
	}

	// TODO: block until the matching TaskResult arrives on the stream (full spec).
	// For now, fire-and-return; the reconciler requeues tasks that exceed 2x timeout.
	d := tunnel.NewDispatcher(h.hub)
	if err := d.Dispatch(task); err != nil {
		return nil, err.Error(), err
	}

	return []byte(`{"dispatched":true}`), "", nil
}
