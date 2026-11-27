package agentpbtest_test

import (
	"errors"
	"io"
	"math"
	"reflect"
	"testing"

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
	openRawStream(t, cp.Addr())
	s, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}

	bad := agentpbtest.UsageFrame()
	bad.Usage.Workloads[0].Resources[0].Quantity = math.NaN()
	if err := s.Send(bad); err == nil {
		t.Error("Send of a NaN quantity succeeded, want an encoding error")
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
