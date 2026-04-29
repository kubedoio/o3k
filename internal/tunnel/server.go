package tunnel

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	pb "github.com/cobaltcore-dev/o3k/proto/tunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// AgentInfo holds metadata and the active stream for a connected tunnel agent.
type AgentInfo struct {
	NodeID   string
	Hostname string
	TunnelIP string
	Stream   grpc.BidiStreamingServer[pb.AgentMessage, pb.ServerMessage]
}

// Hub tracks connected tunnel agents and provides agent selection.
type Hub struct {
	pb.UnimplementedTunnelHubServer
	tokenSecret string
	tlsConfig   *tls.Config
	mu          sync.RWMutex
	agents      map[string]*AgentInfo
}

// NewHub creates a new Hub with the given JWT token secret.
func NewHub(tokenSecret string) *Hub {
	return &Hub{
		tokenSecret: tokenSecret,
		agents:      make(map[string]*AgentInfo),
	}
}

// RegisterAgent adds or updates an agent entry in the hub.
func (h *Hub) RegisterAgent(info AgentInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.agents[info.NodeID] = &info
}

// RemoveAgent removes the agent with the given nodeID from the hub.
func (h *Hub) RemoveAgent(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.agents, nodeID)
}

// ListAgents returns a snapshot of all currently registered agents.
func (h *Hub) ListAgents() []AgentInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]AgentInfo, 0, len(h.agents))
	for _, a := range h.agents {
		out = append(out, *a)
	}
	return out
}

// PickAgent returns any one registered agent, or nil if none are available.
func (h *Hub) PickAgent() *AgentInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, a := range h.agents {
		return a
	}
	return nil
}

// VerifyJoin reports whether the given tokenHash is valid for nodeID.
// When tokenSecret is empty the hub is in open enrollment mode and all joins are accepted.
func (h *Hub) VerifyJoin(nodeID, tokenHash string) bool {
	if h.tokenSecret == "" {
		return true
	}
	return VerifyTokenHash(h.tokenSecret, nodeID, tokenHash)
}

// SetTLSConfig configures the Hub to use TLS when ListenAndServe is called.
// When cfg is nil the server starts without TLS (plain gRPC).
func (h *Hub) SetTLSConfig(cfg *tls.Config) {
	h.tlsConfig = cfg
}

// ListenAndServe starts the gRPC server on addr and blocks until it exits.
// When a TLS config has been set via SetTLSConfig the server uses mutual TLS.
func (h *Hub) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("tunnel listen %s: %w", addr, err)
	}

	var opts []grpc.ServerOption
	if h.tlsConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(h.tlsConfig)))
	}

	s := grpc.NewServer(opts...)
	pb.RegisterTunnelHubServer(s, h)
	fmt.Printf("TunnelHub listening on %s (tls=%v)\n", addr, h.tlsConfig != nil)
	return s.Serve(lis)
}

// AgentStream handles a bidirectional gRPC stream from a tunnel agent.
func (h *Hub) AgentStream(stream grpc.BidiStreamingServer[pb.AgentMessage, pb.ServerMessage]) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}
	join := msg.GetJoin()
	if join == nil {
		return fmt.Errorf("first message must be JoinMsg")
	}
	if !h.VerifyJoin(join.GetNodeId(), join.GetTokenHash()) {
		return fmt.Errorf("invalid join token for node %s", join.GetNodeId())
	}
	h.RegisterAgent(AgentInfo{
		NodeID:   join.GetNodeId(),
		Hostname: join.GetHostname(),
		TunnelIP: join.GetTunnelIp(),
		Stream:   stream,
	})
	defer h.RemoveAgent(join.GetNodeId())

	for {
		if _, err := stream.Recv(); err != nil {
			return err
		}
	}
}
