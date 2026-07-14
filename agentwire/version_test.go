package agentwire

import "testing"

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
		ok   bool
	}{
		{"1.2.3", [3]int{1, 2, 3}, true},
		{"v1.2.3", [3]int{1, 2, 3}, true},
		{"0.1.0", [3]int{0, 1, 0}, true},
		{"v2.0.0-rc1", [3]int{2, 0, 0}, true},
		{"1.4.2+build.7", [3]int{1, 4, 2}, true},
		{"  v1.2.3  ", [3]int{1, 2, 3}, true},
		{"latest", [3]int{}, false},
		{"", [3]int{}, false},
		{"1.2", [3]int{}, false},
		{"1.2.3.4", [3]int{}, false},
		{"a1b2c3d", [3]int{}, false},
		{"1.x.3", [3]int{}, false},
		{"-1.2.3", [3]int{}, false},
	}
	for _, tc := range cases {
		got, ok := parseSemver(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseSemver(%q) = %v,%v; want %v,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestClassifyAgentVersion(t *testing.T) {
	// Floor is MinSupportedAgentVersion ("0.1.0").
	cases := []struct {
		in   string
		want Support
	}{
		{"0.1.0", SupportOK},     // exactly the floor
		{"v0.3.0", SupportOK},    // inside the window
		{"1.0.0", SupportOK},     // newer than the control plane — additive-only makes this safe
		{"0.0.9", SupportTooOld}, // below the floor
		{"latest", SupportUnknown},
		{"", SupportUnknown},
		{"deadbeef", SupportUnknown},
	}
	for _, tc := range cases {
		if got := ClassifyAgentVersion(tc.in); got != tc.want {
			t.Errorf("ClassifyAgentVersion(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

// TestMinSupportedIsParseable guards against a mis-edit of the floor constant
// that would make ClassifyAgentVersion fail open for every agent.
func TestMinSupportedIsParseable(t *testing.T) {
	if _, ok := parseSemver(MinSupportedAgentVersion); !ok {
		t.Fatalf("MinSupportedAgentVersion %q is not a valid semver", MinSupportedAgentVersion)
	}
}
