package agentwire

import (
	"strconv"
	"strings"
)

// parseSemver parses a MAJOR.MINOR.PATCH string into its three numeric
// components. It accepts an optional leading "v" and ignores any pre-release or
// build-metadata suffix ("-rc1", "+build.7"), matching the release tags the
// release pipeline stamps ("v*.*.*"). It returns ok
// == false for anything that is not three dotted integers, so callers treat dev
// tags ("latest"), empty values, and git SHAs as SupportUnknown rather than a
// hard parse error.
func parseSemver(s string) (v [3]int, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	// Drop pre-release / build metadata; only the core X.Y.Z is ordered.
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// compareSemver returns -1, 0, or +1 as a is less than, equal to, or greater
// than b, comparing major, then minor, then patch.
func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}
