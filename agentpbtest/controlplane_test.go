package agentpbtest_test

import (
	"errors"
	"io"
	"math"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/allodium-co/contract/agentpb"
	"github.com/allodium-co/contract/agentpbtest"
)

// openRawStream opens a stream with the plain agentpb client, so the
// FakeControlPlane tests do not depend on FakeAgent.
func openRawStream(t *testing.T, addr string, kv ...string) agentpb.AgentService_StreamCommandsClient {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx := metadata.NewOutgoingContext(testCtx(t), metadata.Pairs(kv...))
	stream, err := agentpb.NewAgentServiceClient(conn).StreamCommands(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	return stream
}

func TestFakeControlPlaneRecordsFramesAndMetadata(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)
	client := openRawStream(t, cp.Addr(), "agent-id", "a1")

	if err := client.Send(agentpbtest.StatusFrame()); err != nil {
		t.Fatalf("send status: %v", err)
	}
	if err := client.Send(agentpbtest.UsageFrame()); err != nil {
		t.Fatalf("send usage: %v", err)
	}

	s, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Metadata().Get("agent-id"); !reflect.DeepEqual(got, []string{"a1"}) {
		t.Errorf("metadata agent-id = %v, want [a1]", got)
	}
	usage, err := s.WaitFor(ctx, agentpbtest.HasUsage)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(usage, agentpbtest.UsageFrame()) {
		t.Errorf("usage frame changed on the wire: %+v", usage.Usage)
	}
	got := cp.Received()
	if len(got) != 2 || got[0].Status == nil || got[1].Usage == nil {
		t.Errorf("Received() = %+v, want the status frame then the usage frame", got)
	}
}

func TestFakeControlPlanePushesDesiredState(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)
	client := openRawStream(t, cp.Addr())

	s, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := agentpbtest.DesiredFrame()
	if err := s.SendDesired(want.Desired); err != nil {
		t.Fatalf("send desired: %v", err)
	}
	got, err := client.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("desired frame changed on the wire\n got: %+v\nwant: %+v", got.Desired, want.Desired)
	}
}

// A frame the agentpb codec cannot encode must fail in the fake as it would
// on the real wire.
func TestFakeControlPlaneRejectsUnencodableFrame(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)
	client := openRawStream(t, cp.Addr())
	s, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}

	bad := agentpbtest.UsageFrame()
	bad.Usage.Workloads[0].Resources[0].Quantity = math.NaN()
	if err := s.Send(bad); err == nil {
		t.Fatal("Send of a NaN quantity succeeded, want an encoding error")
	}
	<-s.Done()
	if code := status.Code(s.Err()); code != codes.Internal {
		t.Errorf("Err() = %v, want Internal", s.Err())
	}
	if _, err := client.Recv(); status.Code(err) != codes.Internal {
		t.Errorf("agent saw %v, want Internal", err)
	}
}

func TestFakeControlPlaneWaitForNextSkipsEarlierFrames(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)
	client := openRawStream(t, cp.Addr())
	s, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}

	early := agentpbtest.ObservedFrame()
	early.Observed.AgentVersion = "on-connect"
	if err := client.Send(early); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WaitFor(ctx, agentpbtest.HasObserved); err != nil {
		t.Fatal(err)
	}

	got := make(chan *agentpb.AgentMessage, 2)
	waitErr := make(chan error, 2)
	for _, wait := range []func() (*agentpb.AgentMessage, error){
		func() (*agentpb.AgentMessage, error) { return s.WaitForNext(ctx, agentpbtest.HasObserved) },
		func() (*agentpb.AgentMessage, error) { return cp.WaitForNext(ctx, agentpbtest.HasObserved) },
	} {
		ready := make(chan struct{})
		go func() {
			close(ready)
			m, err := wait()
			got <- m
			waitErr <- err
		}()
		<-ready
	}
	// The waiters take their starting point when they start running. The
	// pause lets them do so before the frame goes out; if one were late it
	// would miss the frame and time out, never pass by returning the early one.
	time.Sleep(50 * time.Millisecond)
	if err := client.Send(agentpbtest.ObservedFrame()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		m := <-got
		if err := <-waitErr; err != nil {
			t.Fatal(err)
		}
		if m.Observed.AgentVersion != "0.4.0" {
			t.Errorf("WaitForNext returned the %q frame, want the one sent after the call", m.Observed.AgentVersion)
		}
	}
}

func TestFakeControlPlaneCloseDropsAgents(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)
	client := openRawStream(t, cp.Addr())
	s, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cp.Close()
	select {
	case <-s.Done():
	default:
		t.Error("stream still open after Close returned")
	}
	if _, err := client.Recv(); status.Code(err) != codes.Unavailable {
		t.Errorf("agent saw %v, want Unavailable", err)
	}
}

func TestFakeControlPlaneDisconnectAndReconnect(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)

	first := openRawStream(t, cp.Addr(), "attempt", "1")
	s1, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s1.Close(status.Error(codes.Unavailable, "simulated disconnect"))
	if _, err := first.Recv(); status.Code(err) != codes.Unavailable {
		t.Errorf("agent saw %v, want Unavailable", err)
	}
	if err := s1.Send(agentpbtest.DesiredFrame()); !errors.Is(err, agentpbtest.ErrStreamEnded) {
		t.Errorf("Send on a closed stream = %v, want ErrStreamEnded", err)
	}

	openRawStream(t, cp.Addr(), "attempt", "2")
	s2, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Metadata().Get("attempt"); !reflect.DeepEqual(got, []string{"2"}) {
		t.Errorf("second stream attempt = %v, want [2]", got)
	}
}

func TestFakeControlPlaneSeesAgentHangUp(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)
	client := openRawStream(t, cp.Addr())
	s, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.CloseSend(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-ctx.Done():
		t.Fatal("stream did not end after the agent closed its send side")
	}
	if err := s.Err(); err != nil {
		t.Errorf("Err() = %v, want nil after a clean hang-up", err)
	}
	if _, err := s.WaitFor(ctx, agentpbtest.HasStatus); !errors.Is(err, agentpbtest.ErrStreamEnded) {
		t.Errorf("WaitFor on an ended stream = %v, want ErrStreamEnded", err)
	}
	if _, err := client.Recv(); !errors.Is(err, io.EOF) {
		t.Errorf("agent Recv = %v, want io.EOF", err)
	}
}
