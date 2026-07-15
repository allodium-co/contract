package agentwire_test

// This is the CI backward-compatibility gate for the control-plane<->agent wire
// contract. The wire is hand-written JSON (no protobuf/buf), so the gate is a
// golden round-trip test rather than `buf breaking`.
//
// The testdata/*.json files are FROZEN samples of what an OLDER peer emits — one
// per AgentMessage payload variant, with every field of that generation
// populated. For each, the test decodes the golden into the CURRENT Go types,
// re-encodes, and asserts that every key/value present in the golden survived
// the round trip (a superset check). That direction is exactly additive-only
// discipline:
//
//   - Adding a field: the current types emit an extra key the golden lacks; the
//     superset check ignores it. PASS — additive changes are allowed.
//   - Removing / renaming a field: the golden's key is no longer produced by the
//     current types, so it vanishes from the round trip. FAIL.
//   - Retyping a field: the value fails to decode or comes back changed. FAIL.
//
// So the build breaks precisely on a breaking wire change, and old agents keep
// decoding on a new control plane. Do NOT edit an existing golden to make a
// failing test pass — that defeats the gate. To land an ADDITIVE change, leave
// the goldens as-is (they pass); add a NEW golden only to freeze a new field as
// a future baseline.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/allodium-co/contract/agentpb"
)

func TestWireGoldenBackwardCompat(t *testing.T) {
	files, err := filepath.Glob("testdata/agentmessage_*.json")
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no golden wire samples found under testdata/ — the compat gate would be a no-op")
	}

	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}

			// The old wire frame as a generic tree.
			var old map[string]any
			if err := json.Unmarshal(raw, &old); err != nil {
				t.Fatalf("golden %s is not valid JSON: %v", f, err)
			}

			// Decode into the current typed contract, then re-encode. A field that
			// was removed/renamed/retyped cannot make this round trip intact.
			var msg agentpb.AgentMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				t.Fatalf("current types cannot decode golden %s (breaking change?): %v", f, err)
			}
			reencoded, err := json.Marshal(&msg)
			if err != nil {
				t.Fatalf("re-encode %s: %v", f, err)
			}
			var got map[string]any
			if err := json.Unmarshal(reencoded, &got); err != nil {
				t.Fatalf("re-encoded %s is not valid JSON: %v", f, err)
			}

			if diff := missingFrom(old, got, ""); diff != "" {
				t.Errorf("breaking wire change: golden %s no longer round-trips through the current contract:\n%s\n\n"+
					"A field was removed, renamed, or retyped. This breaks older peers. See "+
					"docs/version-compatibility.md for the additive-only policy.", f, diff)
			}
		})
	}
}

// missingFrom reports the first path at which a key/value present in want is
// absent or changed in got. It ignores keys present only in got (additive
// fields). Empty string means want is fully contained in got.
func missingFrom(want, got any, path string) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: was an object, now %T", path, got)
		}
		for k, wv := range w {
			gv, present := g[k]
			if !present {
				return fmt.Sprintf("%s: key %q dropped from the wire", path, k)
			}
			if diff := missingFrom(wv, gv, path+"."+k); diff != "" {
				return diff
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: was an array, now %T", path, got)
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: array length changed %d -> %d", path, len(w), len(g))
		}
		for i := range w {
			if diff := missingFrom(w[i], g[i], fmt.Sprintf("%s[%d]", path, i)); diff != "" {
				return diff
			}
		}
		return ""
	default:
		if !reflect.DeepEqual(want, got) {
			return fmt.Sprintf("%s: value changed %v -> %v", path, want, got)
		}
		return ""
	}
}
