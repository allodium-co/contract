package clusterspec

import "strings"

// Phase is the observed lifecycle phase of a compute workload.
// It is a closed set — only the four constants below are valid.
type Phase string

const (
	PhaseStarting Phase = "starting"
	PhaseRunning  Phase = "running"
	PhaseStopped  Phase = "stopped"
	PhaseError    Phase = "error"
)

// Valid reports whether p is one of the four known Phase values.
func (p Phase) Valid() bool {
	switch p {
	case PhaseStarting, PhaseRunning, PhaseStopped, PhaseError:
		return true
	}
	return false
}

// ParsePhase normalises a raw string (e.g. from a CRD status field that may be
// capitalised or use an alias such as "Pending") into a typed Phase.
// Unrecognised values map to PhaseStopped.
func ParsePhase(s string) Phase {
	switch strings.ToLower(s) {
	case "running":
		return PhaseRunning
	case "starting", "pending":
		return PhaseStarting
	case "error":
		return PhaseError
	default:
		return PhaseStopped
	}
}

// ClusterStatus is the observed state the agent writes back (to the CRD status,
// read by the control plane). It is metadata only — phases, counts, and the
// agent's version — never query data.
type ClusterStatus struct {
	Warehouses        []WarehouseStatus    `json:"warehouses,omitempty"`
	SparkClusters     []SparkClusterStatus `json:"sparkClusters,omitempty"`
	LastReconcileTime string               `json:"lastReconcileTime,omitempty"`
	AgentVersion      string               `json:"agentVersion,omitempty"`
}

type WarehouseStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Phase   Phase  `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

type SparkClusterStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Phase   Phase  `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}
