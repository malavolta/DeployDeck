// Package version holds the build-time version identity of the deploydeck
// binary. Version, Commit, and Date are string variables (not constants) so
// they can be set at link time via:
//
//	-ldflags "-X deploydeck/internal/version.Version=1.2.3 \
//	          -X deploydeck/internal/version.Commit=abc123 \
//	          -X deploydeck/internal/version.Date=2026-01-01"
//
// A binary built without any -X injection keeps Version at its "dev"
// default, so `deploydeck --version` and the update-notification's
// no-nag-on-dev rule both stay inert-safe.
package version

import "fmt"

// Version is the semver tag (or "dev") this binary was built from.
var Version = "dev"

// Commit is the short git SHA this binary was built from. Empty when not
// injected.
var Commit = ""

// Date is the build timestamp. Empty when not injected.
var Date = ""

// String formats the version line reported by `deploydeck --version`. A
// "dev" Version (the default, unbuilt-with-ldflags case) is reported as-is;
// otherwise Commit and Date are appended when present.
func String() string {
	if Version == "dev" {
		return "dev"
	}

	s := Version
	if Commit != "" {
		s += fmt.Sprintf(" (commit %s", Commit)
		if Date != "" {
			s += fmt.Sprintf(", built %s", Date)
		}
		s += ")"
	} else if Date != "" {
		s += fmt.Sprintf(" (built %s)", Date)
	}
	return s
}
