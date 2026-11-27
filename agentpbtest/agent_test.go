package agentpbtest_test

import (
	"context"
	"errors"
	"math"
	"net"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/allodium-co/contract/agentpb"
	"github.com/allodium-co/contract/agentpbtest"
)

// scriptedServer is a minimal control plane built on the plain agentpb
// server stub, so the FakeAgent tests do not depend on FakeControlPlane. It
// hands each stream to the test and ends the RPC with whatever the test
// sends on result.
type scriptedServer struct {
	streams chan agentpb.AgentService_StreamCommandsServer
	result  chan error
}

func (s *scriptedServer) StreamCommands(stream agentpb.AgentService_StreamCommandsServer) error {
	s.streams <- stream
	select {
	case err := <-s.result:
		return err
	case <-stream.Context().Done():
		return stream.Context().Err()
	}
}

// startScriptedServer serves on an in-memory listener, which also covers
// dialing through WithDialOptions.
func startScriptedServer(t *testing.T) (*scriptedServer, agentpbtest.AgentOption) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	ss := &scriptedServer{
		streams: make(chan agentpb.AgentService_StreamCommandsServer, 1),
		result:  make(chan error, 1),
	}
	agentpb.RegisterAgentServiceServer(srv, ss)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	dial := agentpbtest.WithDialOptions(grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}))
	return ss, dial
}

func (s *scriptedServer) accept(t *testing.T, ctx context.Context) agentpb.AgentService_StreamCommandsServer {
	t.Helper()
	select {
	case stream := <-s.streams:
		return stream
	case <-ctx.Done():
		t.Fatal("agent never opened a stream")
		return nil
	}
}

func TestFakeAgentSendsFramesWithMetadata(t *testing.T) {
	ctx := testCtx(t)
	ss, dial := startScriptedServer(t)
	agent := agentpbtest.NewFakeAgent(t, "passthrough:///bufnet", dial,
		agentpbtest.WithMetadata("agent-id", "a1"),
		agentpbtest.WithMetadata("workspace-id", "ws-1"))

	if err := agent.SendStatus(agentpbtest.StatusFrame().Status); err != nil {
		t.Fatal(err)
	}
	if err := agent.SendObserved(agentpbtest.ObservedFrame().Observed); err != nil {
		t.Fatal(err)
	}
	if err := agent.SendUsage(agentpbtest.UsageFrame().Usage); err != nil {
		t.Fatal(err)
	}

	stream := ss.accept(t, ctx)
	md, _ := metadata.FromIncomingContext(stream.Context())
	if got := md.Get("agent-id"); !reflect.DeepEqual(got, []string{"a1"}) {
		t.Errorf("agent-id = %v, want [a1]", got)
	}
	if got := md.Get("workspace-id"); !reflect.DeepEqual(got, []string{"ws-1"}) {
		t.Errorf("workspace-id = %v, want [ws-1]", got)
	}
	for _, want := range []*agentpb.AgentMessage{
		agentpbtest.StatusFrame(), agentpbtest.ObservedFrame(), agentpbtest.UsageFrame(),
	} {
		got, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("frame changed on the wire\n got: %+v\nwant: %+v", got, want)
		}
	}
}

func TestFakeAgentRecordsDesiredState(t *testing.T) {
	ctx := testCtx(t)
	ss, dial := startScriptedServer(t)
	agent := agentpbtest.NewFakeAgent(t, "passthrough:///bufnet", dial)

	stream := ss.accept(t, ctx)
	want := agentpbtest.DesiredFrame()
	if err := stream.Send(want); err != nil {
		t.Fatal(err)
	}
	got, err := agent.WaitForDesired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want.Desired) {
		t.Errorf("desired state changed on the wire\n got: %+v\nwant: %+v", got, want.Desired)
	}
	if n := len(agent.Received()); n != 1 {
		t.Errorf("Received() has %d frames, want 1", n)
	}
}

// A control plane rejects an agent by ending its stream; the fake must
// surface that status rather than leave the test waiting.
func TestFakeAgentSeesRejection(t *testing.T) {
	ctx := testCtx(t)
	ss, dial := startScriptedServer(t)
	agent := agentpbtest.NewFakeAgent(t, "passthrough:///bufnet", dial)

	ss.accept(t, ctx)
	ss.result <- status.Error(codes.PermissionDenied, "unknown enrollment token")

	_, err := agent.WaitForDesired(ctx)
	if !errors.Is(err, agentpbtest.ErrStreamEnded) {
		t.Fatalf("WaitForDesired = %v, want ErrStreamEnded", err)
	}
	if status.Code(agent.Err()) != codes.PermissionDenied {
		t.Errorf("Err() = %v, want PermissionDenied", agent.Err())
	}
	select {
	case <-agent.Done():
	default:
		t.Error("Done() not closed after the stream ended")
	}
}

func TestFakeAgentRejectsUnencodableFrame(t *testing.T) {
	_, dial := startScriptedServer(t)
	agent := agentpbtest.NewFakeAgent(t, "passthrough:///bufnet", dial)

	bad := agentpbtest.UsageFrame().Usage
	bad.Workloads[0].Resources[0].Quantity = math.Inf(1)
	if err := agent.SendUsage(bad); err == nil {
		t.Error("SendUsage of an infinite quantity succeeded, want an encoding error")
	}
}
