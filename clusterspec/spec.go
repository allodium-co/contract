// Package clusterspec is the canonical desired-state + status schema exchanged
// between the control plane and the data-plane agent. The control plane owns
// desired state (in its Postgres) and speaks only these types; the agent maps
// this desired state onto its internal ClusterSpec CRD. This is the single
// source of truth — both planes reference it, so the schema cannot drift.
//
// Boundary invariant: only compute shape is permitted here (sizes, replica
// bounds, package coordinates). No table/schema/column references, query text,
// or results ever appear in these types.
//
// The agent Validate()s every spec before building Kubernetes objects — its
// defense against a malformed or compromised control plane (architecture
// test T2).
package clusterspec

import "fmt"

var validWorkerModes = map[string]bool{"": true, "none": true, "static": true, "keda": true}

// ClusterSpec is the compute desired state: the named Trino warehouses and Spark
// clusters a tenant has deployed. There is no always-on default compute — every
// cluster is tenant-declared from a t-shirt size or a custom shape. Only compute
// shape is permitted here — no table, schema, or query-result fields (boundary
// invariant, T1).
type ClusterSpec struct {
	Warehouses    []WarehouseSpec    `json:"warehouses,omitempty"`
	SparkClusters []SparkClusterSpec `json:"sparkClusters,omitempty"`
}

// WarehouseSpec is the constrained compute schema the control plane may send to
// start a Trino warehouse.
type WarehouseSpec struct {
	ID                    string `json:"id"`
	TenantID              string `json:"tenant_id,omitempty"`
	Name                  string `json:"name"`
	Profile               string `json:"profile,omitempty"`
	HeapSizeGb            int    `json:"heap_size_gb,omitempty"`
	MaxQueryMemoryPerNode string `json:"max_query_memory_per_node,omitempty"`
	MaxTotalMemoryPerNode string `json:"max_total_memory_per_node,omitempty"`
	HeapHeadroom          string `json:"heap_headroom,omitempty"`
	WorkerMode            string `json:"worker_mode,omitempty"`
	WorkerProfile         string `json:"worker_profile,omitempty"`
	WorkerHeapSizeGb      int    `json:"worker_heap_size_gb,omitempty"`
	WorkerMinReplicas     int    `json:"worker_min_replicas,omitempty"`
	WorkerMaxReplicas     int    `json:"worker_max_replicas,omitempty"`
	IdleTimeoutMins       int    `json:"idle_timeout_mins,omitempty"`
}

// Validate rejects anything outside the constrained compute schema. This is
// the agent's defense against a compromised or malformed control plane
// (architecture test T2): the executor must never build k8s objects from an
// unvalidated spec.
func (w WarehouseSpec) Validate() error {
	if w.ID == "" {
		return fmt.Errorf("id is required")
	}
	if w.Name == "" {
		return fmt.Errorf("name is required")
	}
	// Profile is a free-form label (the t-shirt size the tenant picked, or
	// "custom"); the control plane owns the size roster in trino_defaults and has
	// already resolved it to the concrete numbers below, so the agent does not
	// validate it against a fixed enum.
	if !validWorkerModes[w.WorkerMode] {
		return fmt.Errorf("invalid worker_mode %q", w.WorkerMode)
	}
	// A warehouse must carry a real coordinator heap. The control plane resolves
	// every t-shirt size to concrete numbers before sending, and a custom shape
	// must specify one, so a zero heap is a malformed spec — rejected here rather
	// than silently floored.
	if w.HeapSizeGb < 1 {
		return fmt.Errorf("heap_size_gb must be at least 1")
	}
	hasWorkers := w.WorkerMode == "static" || w.WorkerMode == "keda"
	if hasWorkers && w.WorkerHeapSizeGb < 1 {
		return fmt.Errorf("worker_heap_size_gb must be at least 1 when workers are enabled")
	}
	if w.WorkerMinReplicas < 0 {
		return fmt.Errorf("worker_min_replicas must be non-negative")
	}
	if w.WorkerMaxReplicas < 0 {
		return fmt.Errorf("worker_max_replicas must be non-negative")
	}
	if w.WorkerMaxReplicas > 0 && w.WorkerMinReplicas > w.WorkerMaxReplicas {
		return fmt.Errorf("worker_min_replicas (%d) exceeds worker_max_replicas (%d)", w.WorkerMinReplicas, w.WorkerMaxReplicas)
	}
	if w.IdleTimeoutMins < 0 {
		return fmt.Errorf("idle_timeout_mins must be non-negative")
	}
	return nil
}

// SparkClusterSpec is the desired state for a named Spark Connect cluster.
// Only compute shape and package coordinates are permitted — no data or query
// fields (boundary invariant, T1).
type SparkClusterSpec struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id,omitempty"`
	Name              string `json:"name,omitempty"`
	ExecutorInstances int    `json:"executorInstances,omitempty"`
	ExecutorMemory    string `json:"executorMemory,omitempty"`
	ExecutorCores     int    `json:"executorCores,omitempty"`
	DriverVCPUs       int    `json:"driverVCPUs,omitempty"`
	DriverMemoryGB    int    `json:"driverMemoryGB,omitempty"`
	// Packages lists Maven coordinates (groupId:artifactId:version) or JAR URLs
	// to make available in the Spark Connect session.
	Packages []string `json:"packages,omitempty"`
}

func (s SparkClusterSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}
