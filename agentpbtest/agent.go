package agentpbtest

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/allodium-co/contract/agentpb"
	"github.com/allodium-co/contract/clusterspec"
)

// FakeAgent is the agent end of AgentService.StreamCommands: a real gRPC
// client that opens one stream to the control plane under test, sends the
// frames the test asks for, and records every frame it receives.
type FakeAgent struct {
	conn   *grpc.ClientConn
	stream agentpb.AgentService_StreamCommandsClient
	cancel context.CancelFunc
	frames *recorder
	done   chan struct{}

	// grpc-go allows one goroutine at a time to send on a client stream.
	sendMu sync.Mutex
}

// AgentOption configures NewFakeAgent.
type AgentOption func(*agentConfig)

type agentConfig struct {
	md       metadata.MD
	dialOpts []grpc.DialOption
}

// WithMetadata adds key/value pairs to the metadata the stream opens with,
// where the real agent puts its identity.
func WithMetadata(kv ...string) AgentOption {
	return func(c *agentConfig) { c.md = metadata.Join(c.md, metadata.Pairs(kv...)) }
}

// WithDialOptions adds options to the dial. The default dials plaintext;
// grpc.WithTransportCredentials here overrides that, and grpc.WithContextDialer
// reaches a server on an in-memory listener.
func WithDialOptions(opts ...grpc.DialOption) AgentOption {
	return func(c *agentConfig) { c.dialOpts = append(c.dialOpts, opts...) }
}

// NewFakeAgent dials target, opens the stream and starts recording what the
// control plane sends. The stream is closed when the test ends. A control
// plane that rejects the agent does so by ending the stream, which shows in
// Done and Err rather than failing this call.
func NewFakeAgent(t testing.TB, target string, opts ...AgentOption) *FakeAgent {
	t.Helper()
	cfg := agentConfig{dialOpts: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}}
	for _, opt := range opts {
		opt(&cfg)
	}

	conn, err := grpc.NewClient(target, cfg.dialOpts...)
	if err != nil {
		t.Fatalf("agentpbtest: dial %s: %v", target, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if cfg.md != nil {
		ctx = metadata.NewOutgoingContext(ctx, cfg.md)
	}
	stream, err := agentpb.NewAgentServiceClient(conn).StreamCommands(ctx)
	if err != nil {
		cancel()
		_ = conn.Close()
		t.Fatalf("agentpbtest: open stream to %s: %v", target, err)
	}

	a := &FakeAgent{
		conn:   conn,
		stream: stream,
		cancel: cancel,
		frames: newRecorder(),
		done:   make(chan struct{}),
	}
	go a.recvLoop()
	t.Cleanup(a.Close)
	return a
}

func (a *FakeAgent) recvLoop() {
	defer close(a.done)
	for {
		m, err := a.stream.Recv()
		if err != nil {
			a.frames.end(err)
			return
		}
		a.frames.add(m)
	}
}

// Send sends msg up the stream, encoded by the agentpb codec. It returns an
// error if the frame cannot be encoded or the stream has ended.
func (a *FakeAgent) Send(msg *agentpb.AgentMessage) error {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()
	if err := a.stream.Send(msg); err != nil {
		return fmt.Errorf("agentpbtest: send: %w", err)
	}
	return nil
}

// SendStatus sends a liveness frame, which is the agent's heartbeat.
func (a *FakeAgent) SendStatus(st *agentpb.ClusterStatus) error {
	return a.Send(&agentpb.AgentMessage{Status: st})
}

// SendObserved sends a full observed-state frame.
func (a *FakeAgent) SendObserved(st *clusterspec.ClusterStatus) error {
	return a.Send(&agentpb.AgentMessage{Observed: st})
}

// SendUsage sends a usage sample.
func (a *FakeAgent) SendUsage(u *agentpb.UsageReport) error {
	return a.Send(&agentpb.AgentMessage{Usage: u})
}

// Received returns every frame received so far, in arrival order. The frames
// are shared with the fake; treat them as read-only.
func (a *FakeAgent) Received() []*agentpb.AgentMessage { return a.frames.snapshot() }

// WaitFor returns the first received frame that match accepts, including
// frames received before the call. It fails fast with ErrStreamEnded if the
// control plane ends the stream first.
func (a *FakeAgent) WaitFor(ctx context.Context, match func(*agentpb.AgentMessage) bool) (*agentpb.AgentMessage, error) {
	return a.frames.waitFor(ctx, match)
}

// WaitForDesired returns the desired state from the first desired-state frame
// received, including one received before the call.
func (a *FakeAgent) WaitForDesired(ctx context.Context) (*clusterspec.ClusterSpec, error) {
	m, err := a.WaitFor(ctx, HasDesired)
	if err != nil {
		return nil, err
	}
	return m.Desired, nil
}

// CloseSend half-closes the stream: the control plane's Recv returns io.EOF,
// and the agent can still receive.
func (a *FakeAgent) CloseSend() error {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()
	return a.stream.CloseSend()
}

// Close cancels the stream and closes the connection, as if the agent went
// away, and waits for the receive side to stop.
func (a *FakeAgent) Close() {
	a.cancel()
	<-a.done
	_ = a.conn.Close()
}

// Done is closed once the stream has ended.
func (a *FakeAgent) Done() <-chan struct{} { return a.done }

// Err is why the stream ended, as a gRPC status error or io.EOF when the
// control plane ended it with codes.OK. It is nil while the stream is open.
func (a *FakeAgent) Err() error {
	select {
	case <-a.done:
		return a.frames.endErr()
	default:
		return nil
	}
}
