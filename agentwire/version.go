// Package agentwire pins the version identity of the control-plane<->agent wire
// contract and the compatibility policy that governs skew between the two planes
// once they ship on independent cadences (repo split af9z.19). In the monorepo
// the two planes upgrade together and skew is invisible; after the split a
// customer pins a data-plane version and upgrades on their own schedule, so the
// control plane must tolerate older agents on the gRPC stream.
//
// Two orthogonal versions live here:
//
//   - SchemaVersion is the generation of the JSON message schema (the agentpb
//     and clusterspec types). It is additive-only: fields may be added, but
//     removing, renaming, or retyping one is a BREAKING change forbidden without
//     the deprecation process in docs/repo-split/version-compatibility.md. The
//     golden round-trip gate (golden_test.go) fails CI on a breaking schema edit,
//     so old peers keep decoding.
//
//   - MinSupportedAgentVersion is the oldest AGENT RELEASE the control plane
//     tolerates on the stream — the floor of the N / N-1 / N-2 support window.
//     The agent reports its release in gRPC metadata ("agent-version") and in
//     ClusterStatus.AgentVersion; the control plane classifies it at handshake
//     with ClassifyAgentVersion.
//
// The contract module is the single source of truth both planes import, so these
// constants cannot drift between control plane and agent.
package agentwire

// SchemaVersion is the current generation of the JSON wire schema. Bump it by
// exactly one whenever the agentpb / clusterspec types change ADDITIVELY (a new
// field). It is published alongside the neutral JSON Schema (af9z.2) so a BYO
// agent can tell which generation it targets. A breaking change is not a bump —
// it is forbidden (see package doc).
const SchemaVersion = 1

// MinSupportedAgentVersion is the floor of the tolerated agent-release window:
// the control plane accepts this release and everything newer, and warns on
// anything older (outside N-2). Release engineering raises it as old releases
// age out of the window; the value here is the machine-readable expression of
// the policy documented in docs/repo-split/version-compatibility.md.
const MinSupportedAgentVersion = "0.1.0"

// SupportWindowMinors documents the width of the support window in minor
// releases: the control plane supports agent N, N-1, and N-2, i.e. two minors
// back from the current release. It is descriptive — the enforced floor is
// MinSupportedAgentVersion.
const SupportWindowMinors = 2

// Support is the outcome of classifying an agent's reported release version
// against the tolerated-version window.
type Support int

const (
	// SupportUnknown means the reported version could not be parsed as a semantic
	// version — an empty value, a dev tag like "latest", or a git SHA. The control
	// plane admits these (they are dev/BYO builds) but cannot vouch for skew.
	SupportUnknown Support = iota
	// SupportOK means the agent's release is at or above MinSupportedAgentVersion
	// (inside the window, or newer than the control plane — additive-only schema
	// makes a newer agent safe for an older control plane).
	SupportOK
	// SupportTooOld means the agent's release is below MinSupportedAgentVersion —
	// outside the N-2 window. The control plane still admits it for MVP but flags
	// the skew (see docs/repo-split/version-compatibility.md).
	SupportTooOld
)

func (s Support) String() string {
	switch s {
	case SupportOK:
		return "ok"
	case SupportTooOld:
		return "too_old"
	default:
		return "unknown"
	}
}

// ClassifyAgentVersion reports whether an agent's self-reported release version
// falls inside the tolerated-version window. An unparseable value (empty,
// "latest", a git SHA, a custom BYO tag) is SupportUnknown, not an error: the
// control plane admits it but cannot reason about skew.
func ClassifyAgentVersion(reported string) Support {
	v, ok := parseSemver(reported)
	if !ok {
		return SupportUnknown
	}
	floor, ok := parseSemver(MinSupportedAgentVersion)
	if !ok {
		// MinSupportedAgentVersion is a compile-time constant we control; an
		// unparseable floor means it was mis-edited. Fail open rather than reject
		// every agent.
		return SupportUnknown
	}
	if compareSemver(v, floor) < 0 {
		return SupportTooOld
	}
	return SupportOK
}
