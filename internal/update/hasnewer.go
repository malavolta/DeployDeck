// Package update checks whether a newer DeployDeck release is available: a
// GitHub Releases lookup (Checker) plus a hand-rolled, zero-dependency
// semver comparison (HasNewer). It never imports internal/app — main
// composes the two.
package update

import (
	"strconv"
	"strings"
)

// HasNewer reports whether latest is a semantically newer version than
// current. Versions are compared as MAJOR.MINOR.PATCH integers after
// stripping an optional leading "v". current == "dev" (the unbuilt-with
// -ldflags default) always reports false, so a dev build never nags about
// updates. Any parse failure on either side (missing/non-numeric segments)
// also reports false rather than panicking — a malformed remote tag must
// never crash the TUI.
func HasNewer(current, latest string) bool {
	if current == "dev" {
		return false
	}

	c, ok := parseSemver(current)
	if !ok {
		return false
	}
	l, ok := parseSemver(latest)
	if !ok {
		return false
	}

	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parseSemver strips an optional leading "v" and splits "MAJOR.MINOR.PATCH"
// into three integers. ok is false when the string doesn't have exactly
// three dot-separated, non-negative-integer segments.
func parseSemver(v string) (parts [3]int, ok bool) {
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}

	segs := strings.Split(v, ".")
	if len(segs) != 3 {
		return parts, false
	}

	for i, seg := range segs {
		n, err := strconv.Atoi(seg)
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}
