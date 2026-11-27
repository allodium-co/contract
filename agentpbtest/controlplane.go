package agentpbtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/allodium-co/contract/agentpb"
	"github.com/allodium-co/contract/clusterspec"
)

// FakeControlPlane is the control-plane end of AgentService.StreamCommands: a
// real gRPC server on a loopback port, so the agent under test dials it the
// way it dials the real control plane, by address. It accepts any number of
// streams, records every frame the agent sends, and lets the test push frames
// down each stream.
type FakeControlPlane struct {
	srv    *grpc.Server
	lis    net.Listener
	frames *recorder

	mu       sync.Mutex
	streams  []*AgentStream
	accepted int
	arrived  chan struct{}
}

// NewFakeControlPlane starts a FakeControlPlane on 127.0.0.1 and stops it when
// the test ends. With no options it serves plaintext; pass grpc.Creds to serve
// TLS.
func NewFakeControlPlane(t testing.TB, opts ...grpc.ServerOption) *FakeControlPlane {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("agentpbtest: listen: %v", err)
	}
	f := &FakeControlPlane{
		srv:     grpc.NewServer(opts...),
		lis:     lis,
		frames:  newRecorder(),
		arrived: make(chan struct{}),
	}
	agentpb.RegisterAgentServiceServer(f.srv, (*fakeAgentService)(f))
	go func() { _ = f.srv.Serve(lis) }()
	t.Cleanup(f.Close)
	return f
}

// Addr is the host:port the agent under test dials.
func (f *FakeControlPlane) Addr() string { return f.lis.Addr().String() }

// Close stops the server and ends every open stream, as if the control plane
// went away. The agent sees its stream fail with codes.Unavailable.
func (f *FakeControlPlane) Close() {
	f.srv.Stop()
	f.frames.end(errors.New("fake control plane closed"))
}

// Accept returns the next stream an agent opened, in the order they were
// opened, waiting for one if need be. Each stream is returned once, so a test
// of reconnects calls Accept once per expected connection.
func (f *FakeControlPlane) Accept(ctx context.Context) (*AgentStream, error) {
	for {
		f.mu.Lock()
		if f.accepted < len(f.streams) {
			s := f.streams[f.accepted]
			f.accepted++
			f.mu.Unlock()
			return s, nil
		}
		arrived := f.arrived
		f.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for an agent stream: %w", ctx.Err())
		case <-arrived:
		}
	}
}

// Received returns every frame received so far, across all streams, in
// arrival order. The frames are shared with the fake; treat them as read-only.
func (f *FakeControlPlane) Received() []*agentpb.AgentMessage { return f.frames.snapshot() }

// WaitFor returns the first frame on any stream that match accepts, including
// frames received before the call. HasStatus, HasObserved and HasUsage are
// ready-made matchers.
func (f *FakeControlPlane) WaitFor(ctx context.Context, match func(*agentpb.AgentMessage) bool) (*agentpb.AgentMessage, error) {
	return f.frames.waitFor(ctx, match)
}

func (f *FakeControlPlane) register(s *AgentStream) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.streams = append(f.streams, s)
	close(f.arrived)
	f.arrived = make(chan struct{})
}

// fakeAgentService keeps StreamCommands off FakeControlPlane's exported API.
type fakeAgentService FakeControlPlane

func (svc *fakeAgentService) StreamCommands(stream agentpb.AgentService_StreamCommandsServer) error {
	f := (*FakeControlPlane)(svc)
	md, _ := metadata.FromIncomingContext(stream.Context())
	s := &AgentStream{
		md:      md.Copy(),
		frames:  newRecorder(),
		sends:   make(chan sendRequest),
		closing: make(chan error, 1),
		done:    make(chan struct{}),
	}
	f.register(s)

	recvErr := make(chan error, 1)
	go func() {
		for {
			m, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			s.frames.add(m)
			f.frames.add(m)
		}
	}()

	err := s.serve(stream, recvErr)
	s.frames.end(err)
	close(s.done)
	return err
}

// AgentStream is one stream an agent opened to a FakeControlPlane.
type AgentStream struct {
	md      metadata.MD
	frames  *recorder
	sends   chan sendRequest
	closing chan error
	done    chan struct{}
}

type sendRequest struct {
	msg    *agentpb.AgentMessage
	result chan error
}

// serve owns the server stream's send side until the stream ends, and returns
// the status the handler ends the RPC with.
func (s *AgentStream) serve(stream agentpb.AgentService_StreamCommandsServer, recvErr <-chan error) error {
	for {
		select {
		case req := <-s.sends:
			req.result <- stream.Send(req.msg)
		case err := <-s.closing:
			return err
		case err := <-recvErr:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// Metadata is the gRPC metadata the agent opened the stream with.
func (s *AgentStream) Metadata() metadata.MD { return s.md }

// Send pushes msg down the stream, encoded by the agentpb codec. It returns an
// error if the frame cannot be encoded or the stream has ended.
func (s *AgentStream) Send(msg *agentpb.AgentMessage) error {
	req := sendRequest{msg: msg, result: make(chan error, 1)}
	select {
	case s.sends <- req:
		return <-req.result
	case <-s.done:
		return endedError("before the frame was sent", s.frames.endErr())
	}
}

// SendDesired pushes spec down the stream as a desired-state frame.
func (s *AgentStream) SendDesired(spec *clusterspec.ClusterSpec) error {
	return s.Send(&agentpb.AgentMessage{Desired: spec})
}

// Received returns the frames received on this stream so far, in arrival
// order. The frames are shared with the fake; treat them as read-only.
func (s *AgentStream) Received() []*agentpb.AgentMessage { return s.frames.snapshot() }

// WaitFor returns the first frame on this stream that match accepts,
// including frames received before the call. It fails fast with
// ErrStreamEnded if the stream ends first.
func (s *AgentStream) WaitFor(ctx context.Context, match func(*agentpb.AgentMessage) bool) (*agentpb.AgentMessage, error) {
	return s.frames.waitFor(ctx, match)
}

// Close ends the stream from the control-plane side with err as the RPC
// status, then waits for it to end. A nil err ends the RPC with codes.OK;
// pass a status error, such as status.Error(codes.Unavailable, ...), to
// simulate a disconnect the agent should retry.
func (s *AgentStream) Close(err error) {
	select {
	case s.closing <- err:
	default:
	}
	<-s.done
}

// Done is closed once the stream has ended, from either side.
func (s *AgentStream) Done() <-chan struct{} { return s.done }

// Err is why the stream ended: nil if the agent closed its send side, the
// error passed to Close, or the transport error. It is nil while the stream
// is open.
func (s *AgentStream) Err() error {
	select {
	case <-s.done:
		return s.frames.endErr()
	default:
		return nil
	}
}
