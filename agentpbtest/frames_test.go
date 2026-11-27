package agentpbtest_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/allodium-co/contract/agentpb"
	"github.com/allodium-co/contract/agentpbtest"
)

var builders = []struct {
	name   string
	build  func() *agentpb.AgentMessage
	golden string
}{
	{"DesiredFrame", agentpbtest.DesiredFrame, "agentmessage_desired_schema2.json"},
	{"StatusFrame", agentpbtest.StatusFrame, "agentmessage_status.json"},
	{"ObservedFrame", agentpbtest.ObservedFrame, "agentmessage_observed_schema2.json"},
	{"UsageFrame", agentpbtest.UsageFrame, "agentmessage_usage.json"},
}

// testCtx bounds every wait in these tests, so a fake that never delivers
// fails the test instead of hanging it.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// A builder is only useful if it is a frame the contract has frozen: each one
// must equal what the current types decode from its golden.
func TestFramesMatchGoldens(t *testing.T) {
	for _, b := range builders {
		t.Run(b.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "agentwire", "testdata", b.golden))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want agentpb.AgentMessage
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("decode golden: %v", err)
			}
			if got := b.build(); !reflect.DeepEqual(got, &want) {
				t.Errorf("%s() differs from %s\n got: %+v\nwant: %+v", b.name, b.golden, got, &want)
			}
		})
	}
}

var goldenName = regexp.MustCompile(`^agentmessage_([a-z]+)(?:_schema(\d+))?\.json$`)

// Each builder must follow the newest golden of its variant, so freezing a
// new schema generation moves the builders with it; and every variant with a
// golden must have a builder.
func TestFramesUseNewestGoldens(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "agentwire", "testdata", "agentmessage_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	newest := map[string]string{}
	generation := map[string]int{}
	for _, f := range files {
		m := goldenName.FindStringSubmatch(filepath.Base(f))
		if m == nil {
			t.Fatalf("golden %s does not follow agentmessage_<variant>[_schema<n>].json", f)
		}
		gen := 1
		if m[2] != "" {
			if gen, err = strconv.Atoi(m[2]); err != nil {
				t.Fatal(err)
			}
		}
		if gen > generation[m[1]] {
			generation[m[1]], newest[m[1]] = gen, filepath.Base(f)
		}
	}

	byGolden := map[string]string{}
	for _, b := range builders {
		byGolden[b.golden] = b.name
	}
	for variant, golden := range newest {
		if _, ok := byGolden[golden]; !ok {
			t.Errorf("newest %s golden is %s, but no builder is pinned to it", variant, golden)
		}
	}
}

// A payload variant added to AgentMessage without a builder would leave the
// planes writing their own frames for it again.
func TestFramesCoverEveryPayload(t *testing.T) {
	msgType := reflect.TypeFor[agentpb.AgentMessage]()
	for i := range msgType.NumField() {
		field := msgType.Field(i)
		covered := false
		for _, b := range builders {
			if !reflect.ValueOf(b.build()).Elem().Field(i).IsZero() {
				covered = true
			}
		}
		if !covered {
			t.Errorf("no frame builder sets AgentMessage.%s", field.Name)
		}
	}
}

func TestDesiredFrameIsValid(t *testing.T) {
	spec := agentpbtest.DesiredFrame().Desired
	for _, w := range spec.Warehouses {
		if err := w.Validate(); err != nil {
			t.Errorf("warehouse %s: %v", w.ID, err)
		}
	}
	for _, s := range spec.SparkClusters {
		if err := s.Validate(); err != nil {
			t.Errorf("spark cluster %s: %v", s.ID, err)
		}
	}
	for _, s := range spec.ToolSessions {
		if err := s.Validate(); err != nil {
			t.Errorf("tool session %s: %v", s.ID, err)
		}
	}
}

func TestFramesAreFresh(t *testing.T) {
	first := agentpbtest.UsageFrame()
	first.Usage.Workloads[0].Resources[0].Quantity = 99
	if got := agentpbtest.UsageFrame().Usage.Workloads[0].Resources[0].Quantity; got != 2 {
		t.Errorf("changing one frame changed the next: quantity = %v, want 2", got)
	}
}
