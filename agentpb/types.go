package agentpb

import (
	"time"

	"github.com/allodium-co/contract/clusterspec"
)

// AgentMessage is the envelope for all frames on the bidirectional stream.
type AgentMessage struct {
	// Desired is the full compute desired state pushed down by the control plane
	// (sourced from its Postgres). The agent applies it to its ClusterSpec CRD;
	// the control plane never touches the data-plane Kubernetes directly.
	Desired *clusterspec.ClusterSpec `json:"desired,omitempty"`
	// Status carries agent liveness (heartbeat) frames.
	Status *ClusterStatus `json:"status,omitempty"`
	// Observed is the full observed compute status the agent streams up: it reads
	// the ClusterSpec CRD's .status (written by the in-cluster ClusterSpec
	// operator) and forwards it, so the control plane learns real workload phases
	// without ever reading the data-plane Kubernetes itself. Sent periodically and
	// on (re)connect; last-wins.
	Observed *clusterspec.ClusterStatus `json:"observed,omitempty"`
	Usage    *UsageReport               `json:"usage,omitempty"`
}

// ClusterStatus is sent by the agent back to the control plane.
type ClusterStatus struct {
	CommandID string    `json:"command_id"`
	Phase     string    `json:"phase"` // stopped | starting | running | error
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// UsageReport is a periodic resource-usage sample the agent's Usage Reporter
// sends to the control plane on the existing stream (billing epic cff.1).
// It is metadata only — counts, quantities, and timestamps — never query data,
// so it stays on the MAY-side of the boundary invariant. Frames are size-limited
// per T6; a workload with many resource lines is split across reports.
//
// SampleID is stable for a given sample so control-plane ingestion is idempotent
// across buffered replay after a connection gap (billing-model.md §4). ObservedAt
// is the agent's local clock at sample time and IntervalSecs is the window this
// sample attributes (snapshot × interval, §3).
type UsageReport struct {
	SampleID     string          `json:"sample_id"`
	AgentID      string          `json:"agent_id"`
	WorkspaceID  string          `json:"workspace_id"`
	Region       string          `json:"region"` // geo+vendor code, e.g. eu-se-1
	ObservedAt   time.Time       `json:"observed_at"`
	IntervalSecs int             `json:"interval_secs"`
	Workloads    []WorkloadUsage `json:"workloads,omitempty"`
}

// WorkloadUsage is the usage of one running workload (a Trino warehouse or Spark
// session), broken down per component so a coordinator and its workers — which
// have different sizes and, for KEDA workers, different replica counts — are
// reported as distinct lines.
type WorkloadUsage struct {
	WorkloadID   string          `json:"workload_id"`   // e.g. the warehouse-id, or "spark-connect"
	WorkloadKind string          `json:"workload_kind"` // trino | spark
	Component    string          `json:"component"`     // coordinator | worker | driver
	Resources    []ResourceUsage `json:"resources,omitempty"`
}

// ResourceUsage is one metered resource for a component: the per-replica request
// and the number of replicas actually observed Running/Ready. Reserved-capacity
// model — Quantity is the k8s resource *request*, ReplicaCount is real observed
// pods (never desired/declared or KEDA min/max). ResourceType is an open set so
// GPU/VRAM for AI workloads needs no wire change (billing-model.md §2).
type ResourceUsage struct {
	ResourceType string  `json:"resource_type"` // CPU | MEMORY | GPU
	SKU          string  `json:"sku,omitempty"` // GPU model; empty for CPU/MEMORY
	Quantity     float64 `json:"quantity"`      // per-replica request
	Unit         string  `json:"unit"`          // vcpu | gib | gpu
	ReplicaCount int     `json:"replica_count"` // pods observed Running/Ready
}
