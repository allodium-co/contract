package clusterspec_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/allodium-co/contract/clusterspec"
)

func validToolSession() clusterspec.ToolSessionSpec {
	return clusterspec.ToolSessionSpec{
		ID:           "k5cq3ucoi4mxvwcbbq7rsgfz3e",
		OwnerSub:     "f3b1c2d4-5e6f-4a1b-8c9d-0e1f2a3b4c5d",
		Tool:         "marimo",
		Size:         "small",
		DesiredPhase: clusterspec.PhaseRunning,
	}
}

func TestToolSessionSpecValidateAccepts(t *testing.T) {
	cases := map[string]func(*clusterspec.ToolSessionSpec){
		"running":          func(*clusterspec.ToolSessionSpec) {},
		"stopped":          func(s *clusterspec.ToolSessionSpec) { s.DesiredPhase = clusterspec.PhaseStopped },
		"uuid id":          func(s *clusterspec.ToolSessionSpec) { s.ID = "0b9e4c1a-7f3d-4e2b-9a6c-5d8f1e2a3b4c" },
		"longest id":       func(s *clusterspec.ToolSessionSpec) { s.ID = strings.Repeat("a", clusterspec.MaxToolSessionIDLen) },
		"longest sub":      func(s *clusterspec.ToolSessionSpec) { s.OwnerSub = strings.Repeat("x", 255) },
		"sub with symbols": func(s *clusterspec.ToolSessionSpec) { s.OwnerSub = "auth0|5f7c8ec7c33c6c004bbafe82" },
		"hyphenated size":  func(s *clusterspec.ToolSessionSpec) { s.Size = "gpu-large" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := validToolSession()
			mutate(&s)
			if err := s.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestToolSessionSpecValidateRejects(t *testing.T) {
	cases := map[string]func(*clusterspec.ToolSessionSpec){
		"empty id":           func(s *clusterspec.ToolSessionSpec) { s.ID = "" },
		"short id":           func(s *clusterspec.ToolSessionSpec) { s.ID = strings.Repeat("a", clusterspec.MinToolSessionIDLen-1) },
		"id too long":        func(s *clusterspec.ToolSessionSpec) { s.ID = strings.Repeat("a", clusterspec.MaxToolSessionIDLen+1) },
		"uppercase id":       func(s *clusterspec.ToolSessionSpec) { s.ID = "K5CQ3UCOI4MXVWCBBQ7RSGFZ3E" },
		"leading hyphen id":  func(s *clusterspec.ToolSessionSpec) { s.ID = "-5cq3ucoi4mxvwcbbq7rsgfz3e" },
		"trailing hyphen id": func(s *clusterspec.ToolSessionSpec) { s.ID = "k5cq3ucoi4mxvwcbbq7rsgfz3-" },
		"dotted id":          func(s *clusterspec.ToolSessionSpec) { s.ID = "k5cq3ucoi4mxv.cbbq7rsgfz3e" },
		"empty sub":          func(s *clusterspec.ToolSessionSpec) { s.OwnerSub = "" },
		"sub too long":       func(s *clusterspec.ToolSessionSpec) { s.OwnerSub = strings.Repeat("x", 256) },
		"sub with space":     func(s *clusterspec.ToolSessionSpec) { s.OwnerSub = "user one" },
		"sub with newline":   func(s *clusterspec.ToolSessionSpec) { s.OwnerSub = "user\none" },
		"non-ascii sub":      func(s *clusterspec.ToolSessionSpec) { s.OwnerSub = "användare" },
		"empty tool":         func(s *clusterspec.ToolSessionSpec) { s.Tool = "" },
		"image as tool":      func(s *clusterspec.ToolSessionSpec) { s.Tool = "ghcr.io/evil/miner:latest" },
		"uppercase tool":     func(s *clusterspec.ToolSessionSpec) { s.Tool = "Marimo" },
		"empty size":         func(s *clusterspec.ToolSessionSpec) { s.Size = "" },
		"size too long":      func(s *clusterspec.ToolSessionSpec) { s.Size = strings.Repeat("s", 64) },
		"empty phase":        func(s *clusterspec.ToolSessionSpec) { s.DesiredPhase = "" },
		"starting phase":     func(s *clusterspec.ToolSessionSpec) { s.DesiredPhase = clusterspec.PhaseStarting },
		"error phase":        func(s *clusterspec.ToolSessionSpec) { s.DesiredPhase = clusterspec.PhaseError },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := validToolSession()
			mutate(&s)
			if err := s.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %+v", s)
			}
		})
	}
}

func TestNewToolSessionIDIsValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id, err := clusterspec.NewToolSessionID()
		if err != nil {
			t.Fatalf("NewToolSessionID: %v", err)
		}
		if len(id) != clusterspec.MinToolSessionIDLen {
			t.Fatalf("NewToolSessionID() = %q, length %d, want %d", id, len(id), clusterspec.MinToolSessionIDLen)
		}
		if !clusterspec.ValidToolSessionID(id) {
			t.Fatalf("NewToolSessionID() = %q, which ValidToolSessionID rejects", id)
		}
		if seen[id] {
			t.Fatalf("NewToolSessionID() repeated %q", id)
		}
		seen[id] = true
	}
}

func TestToolSessionWireFieldsAreOnlyShapeAndIdentity(t *testing.T) {
	cases := []struct {
		typ  reflect.Type
		want []string
	}{
		{reflect.TypeFor[clusterspec.ToolSessionSpec](), []string{"desiredPhase", "id", "ownerSub", "size", "tool"}},
		{reflect.TypeFor[clusterspec.ToolSessionStatus](), []string{"id", "message", "phase"}},
	}
	for _, tc := range cases {
		var got []string
		for i := range tc.typ.NumField() {
			name, _, _ := strings.Cut(tc.typ.Field(i).Tag.Get("json"), ",")
			got = append(got, name)
		}
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s wire fields = %v, want %v", tc.typ.Name(), got, tc.want)
		}
	}
}
