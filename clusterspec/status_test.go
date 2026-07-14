package clusterspec_test

import (
	"encoding/json"
	"testing"

	"github.com/allodium-co/contract/clusterspec"
)

// TestPhaseRoundTripJSON verifies that the four typed constants serialise to
// the expected lowercase strings and deserialise back to the same typed value.
func TestPhaseRoundTripJSON(t *testing.T) {
	cases := []struct {
		phase clusterspec.Phase
		json  string
	}{
		{clusterspec.PhaseRunning, `"running"`},
		{clusterspec.PhaseStarting, `"starting"`},
		{clusterspec.PhaseStopped, `"stopped"`},
		{clusterspec.PhaseError, `"error"`},
	}
	for _, tc := range cases {
		b, err := json.Marshal(tc.phase)
		if err != nil {
			t.Fatalf("Marshal(%q): %v", tc.phase, err)
		}
		if string(b) != tc.json {
			t.Errorf("Marshal(%q) = %s, want %s", tc.phase, b, tc.json)
		}

		var got clusterspec.Phase
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", b, err)
		}
		if got != tc.phase {
			t.Errorf("round-trip: got %q, want %q", got, tc.phase)
		}
	}
}

// TestPhaseValid verifies Valid() accepts the four known values and rejects anything else.
func TestPhaseValid(t *testing.T) {
	valid := []clusterspec.Phase{
		clusterspec.PhaseRunning,
		clusterspec.PhaseStarting,
		clusterspec.PhaseStopped,
		clusterspec.PhaseError,
	}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("Valid(%q) = false, want true", p)
		}
	}

	invalid := []clusterspec.Phase{"", "Running", "Pending", "unknown", "RUNNING"}
	for _, p := range invalid {
		if p.Valid() {
			t.Errorf("Valid(%q) = true, want false", p)
		}
	}
}

// TestParsePhase verifies ParsePhase normalises capitalised / aliased inputs and
// maps unknown values to PhaseStopped.
func TestParsePhase(t *testing.T) {
	cases := []struct {
		in   string
		want clusterspec.Phase
	}{
		{"running", clusterspec.PhaseRunning},
		{"Running", clusterspec.PhaseRunning},
		{"RUNNING", clusterspec.PhaseRunning},
		{"starting", clusterspec.PhaseStarting},
		{"Starting", clusterspec.PhaseStarting},
		{"pending", clusterspec.PhaseStarting}, // k8s alias
		{"Pending", clusterspec.PhaseStarting}, // k8s alias capitalised
		{"error", clusterspec.PhaseError},
		{"Error", clusterspec.PhaseError},
		{"stopped", clusterspec.PhaseStopped},
		{"Stopped", clusterspec.PhaseStopped},
		{"", clusterspec.PhaseStopped},      // empty → stopped
		{"weird", clusterspec.PhaseStopped}, // unknown → stopped
	}
	for _, tc := range cases {
		if got := clusterspec.ParsePhase(tc.in); got != tc.want {
			t.Errorf("ParsePhase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestWarehouseStatusPhaseFieldJSON verifies that WarehouseStatus and
// SparkClusterStatus serialise/deserialise Phase as the typed enum.
func TestWarehouseStatusPhaseFieldJSON(t *testing.T) {
	ws := clusterspec.WarehouseStatus{ID: "wh-1", Phase: clusterspec.PhaseRunning, Message: "ok"}
	b, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got clusterspec.WarehouseStatus
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Phase != clusterspec.PhaseRunning {
		t.Errorf("phase: want %q, got %q", clusterspec.PhaseRunning, got.Phase)
	}
	if !got.Phase.Valid() {
		t.Errorf("Valid() should be true for round-tripped phase %q", got.Phase)
	}
}
