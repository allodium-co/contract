package agentpbtest_test

import (
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/allodium-co/contract/agentpb"
	"github.com/allodium-co/contract/agentpbtest"
)

// TestFakeAgentAgainstFakeControlPlane runs a whole session between the two
// fakes: connect with an identity, stream every agent-to-control-plane
// payload up, push desired state down, then drop the agent.
func TestFakeAgentAgainstFakeControlPlane(t *testing.T) {
	ctx := testCtx(t)
	cp := agentpbtest.NewFakeControlPlane(t)
	agent := agentpbtest.NewFakeAgent(t, cp.Addr(),
		agentpbtest.WithMetadata("agent-id", "agent-acme", "workspace-id", "ws-1"))

	stream, err := cp.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := stream.Metadata().Get("workspace-id"); !reflect.DeepEqual(got, []string{"ws-1"}) {
		t.Errorf("workspace-id = %v, want [ws-1]", got)
	}

	if err := agent.SendStatus(agentpbtest.StatusFrame().Status); err != nil {
		t.Fatal(err)
	}
	if err := agent.SendObserved(agentpbtest.ObservedFrame().Observed); err != nil {
		t.Fatal(err)
	}
	if err := agent.SendUsage(agentpbtest.UsageFrame().Usage); err != nil {
		t.Fatal(err)
	}
	for _, up := range []struct {
		match func(*agentpb.AgentMessage) bool
		want  *agentpb.AgentMessage
	}{
		{agentpbtest.HasStatus, agentpbtest.StatusFrame()},
		{agentpbtest.HasObserved, agentpbtest.ObservedFrame()},
		{agentpbtest.HasUsage, agentpbtest.UsageFrame()},
	} {
		got, err := cp.WaitFor(ctx, up.match)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, up.want) {
			t.Errorf("control plane received\n %+v\nwant\n %+v", got, up.want)
		}
	}

	want := agentpbtest.DesiredFrame().Desired
	if err := stream.SendDesired(want); err != nil {
		t.Fatal(err)
	}
	got, err := agent.WaitForDesired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("agent received\n %+v\nwant\n %+v", got, want)
	}

	stream.Close(status.Error(codes.Unavailable, "control plane restarting"))
	<-agent.Done()
	if code := status.Code(agent.Err()); code != codes.Unavailable {
		t.Errorf("agent saw %v, want Unavailable", agent.Err())
	}
}
