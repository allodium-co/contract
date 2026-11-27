package agentpbtest

import (
	"time"

	"github.com/allodium-co/contract/agentpb"
	"github.com/allodium-co/contract/clusterspec"
)

// The frame builders below return the newest frozen golden of each payload
// variant in agentwire/testdata as Go values. A test pins each one to its
// golden, so a builder is always a frame the wire contract has committed to.
// Each call returns a fresh value: change any field to shape the frame a test
// needs without affecting other tests.

var goldenTime = time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)

// DesiredFrame is a control-plane-to-agent frame carrying full desired state:
// one KEDA-scaled Trino warehouse, one Spark cluster, and two tool sessions,
// one running and one stopped. Every spec in it passes its Validate method.
func DesiredFrame() *agentpb.AgentMessage {
	return &agentpb.AgentMessage{Desired: &clusterspec.ClusterSpec{
		Warehouses: []clusterspec.WarehouseSpec{{
			ID:                    "wh-1",
			TenantID:              "acme",
			Name:                  "analytics",
			Profile:               "medium",
			HeapSizeGb:            8,
			MaxQueryMemoryPerNode: "4GB",
			MaxTotalMemoryPerNode: "6GB",
			HeapHeadroom:          "2GB",
			WorkerMode:            "keda",
			WorkerProfile:         "medium",
			WorkerHeapSizeGb:      8,
			WorkerMinReplicas:     1,
			WorkerMaxReplicas:     5,
			IdleTimeoutMins:       30,
		}},
		SparkClusters: []clusterspec.SparkClusterSpec{{
			ID:                "spark-1",
			TenantID:          "acme",
			Name:              "etl",
			ExecutorInstances: 3,
			ExecutorMemory:    "4g",
			ExecutorCores:     2,
			DriverVCPUs:       2,
			DriverMemoryGB:    4,
			Packages:          []string{"org.apache.iceberg:iceberg-spark-runtime:1.5.0"},
		}},
		ToolSessions: []clusterspec.ToolSessionSpec{
			{
				ID:           "k5cq3ucoi4mxvwcbbq7rsgfz3e",
				OwnerSub:     "f3b1c2d4-5e6f-4a1b-8c9d-0e1f2a3b4c5d",
				Tool:         "marimo",
				Size:         "small",
				DesiredPhase: clusterspec.PhaseRunning,
			},
			{
				ID:           "q2w7nb4xk3ldyh6ru5vapzeoti",
				OwnerSub:     "f3b1c2d4-5e6f-4a1b-8c9d-0e1f2a3b4c5d",
				Tool:         "marimo",
				Size:         "medium",
				DesiredPhase: clusterspec.PhaseStopped,
			},
		},
	}}
}

// StatusFrame is an agent-to-control-plane liveness frame: the heartbeat.
func StatusFrame() *agentpb.AgentMessage {
	return &agentpb.AgentMessage{Status: &agentpb.ClusterStatus{
		CommandID: "cmd-1",
		Phase:     "running",
		Message:   "ok",
		Timestamp: goldenTime,
	}}
}

// ObservedFrame is an agent-to-control-plane frame carrying full observed
// state: a running warehouse, a starting Spark cluster, and two tool sessions,
// one running and one failed.
func ObservedFrame() *agentpb.AgentMessage {
	return &agentpb.AgentMessage{Observed: &clusterspec.ClusterStatus{
		Warehouses: []clusterspec.WarehouseStatus{
			{ID: "wh-1", Name: "analytics", Phase: clusterspec.PhaseRunning, Message: "ready"},
		},
		SparkClusters: []clusterspec.SparkClusterStatus{
			{ID: "spark-1", Name: "etl", Phase: clusterspec.PhaseStarting, Message: "scheduling"},
		},
		ToolSessions: []clusterspec.ToolSessionStatus{
			{ID: "k5cq3ucoi4mxvwcbbq7rsgfz3e", Phase: clusterspec.PhaseRunning, Message: "ready"},
			{ID: "q2w7nb4xk3ldyh6ru5vapzeoti", Phase: clusterspec.PhaseError, Message: `unknown size "huge" for tool "marimo"`},
		},
		LastReconcileTime: "2026-11-26T10:00:00Z",
		AgentVersion:      "0.4.0",
	}}
}

// UsageFrame is an agent-to-control-plane usage sample: a Trino coordinator's
// CPU and memory, and GPU workers with a SKU.
func UsageFrame() *agentpb.AgentMessage {
	return &agentpb.AgentMessage{Usage: &agentpb.UsageReport{
		SampleID:     "s-1",
		AgentID:      "agent-acme",
		WorkspaceID:  "ws-1",
		Region:       "eu-se-1",
		ObservedAt:   goldenTime,
		IntervalSecs: 60,
		Workloads: []agentpb.WorkloadUsage{
			{
				WorkloadID:   "wh-1",
				WorkloadKind: "trino",
				Component:    "coordinator",
				Resources: []agentpb.ResourceUsage{
					{ResourceType: "CPU", Quantity: 2, Unit: "vcpu", ReplicaCount: 1},
					{ResourceType: "MEMORY", Quantity: 8, Unit: "gib", ReplicaCount: 1},
				},
			},
			{
				WorkloadID:   "wh-1",
				WorkloadKind: "trino",
				Component:    "worker",
				Resources: []agentpb.ResourceUsage{
					{ResourceType: "GPU", SKU: "a100", Quantity: 1, Unit: "gpu", ReplicaCount: 3},
				},
			},
		},
	}}
}

// HasDesired matches a frame carrying desired state. It and the other Has
// functions are ready-made matchers for WaitFor.
func HasDesired(m *agentpb.AgentMessage) bool { return m.Desired != nil }

// HasStatus matches a liveness (heartbeat) frame.
func HasStatus(m *agentpb.AgentMessage) bool { return m.Status != nil }

// HasObserved matches a frame carrying observed state.
func HasObserved(m *agentpb.AgentMessage) bool { return m.Observed != nil }

// HasUsage matches a usage sample.
func HasUsage(m *agentpb.AgentMessage) bool { return m.Usage != nil }
