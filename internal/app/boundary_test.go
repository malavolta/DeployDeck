package app_test

import (
	"go/build"
	"testing"
)

// TestApp_NeverImportsExecSeam is the architecture-invariant boundary test
// (HU-006 tasks 10.37/10.38, extended by 11.6 to cover the whole package):
// internal/app must NEVER exec git/sf directly. It reaches every external
// command ONLY through the services (internal/git.Service,
// internal/salesforce.Client), which themselves funnel through
// internal/exec. Therefore internal/app's OWN (non-test) source files must
// import neither:
//
//   - "os/exec"                — a raw process launch, bypassing the seam, and
//   - "deploydeck/internal/exec" — the seam itself (only services may hold it).
//
// This is a DIRECT-import check, not a transitive one: bubbletea's
// tea.ExecProcess transitively pulls in os/exec for interactive $EDITOR /
// mergetool handoff, which the design explicitly ALLOWS — so a `go list
// -deps` transitive scan for os/exec would wrongly fail. build.ImportDir
// reports exactly the packages internal/app's own files import, which is the
// invariant the design actually constrains.
func TestApp_NeverImportsExecSeam(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("importing internal/app package: %v", err)
	}

	// Non-vacuous guard: internal/app must actually compose the git service
	// (proving this test inspects a real, wired package — not an empty one).
	if !contains(pkg.Imports, "deploydeck/internal/git") {
		t.Fatalf("internal/app must compose deploydeck/internal/git; direct imports were: %v", pkg.Imports)
	}

	forbidden := []string{"os/exec", "deploydeck/internal/exec"}
	for _, f := range forbidden {
		if contains(pkg.Imports, f) {
			t.Errorf("internal/app must NOT import %q directly (route all external commands through the services); direct imports were: %v", f, pkg.Imports)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
