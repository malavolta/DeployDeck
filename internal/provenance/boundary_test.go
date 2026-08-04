package provenance_test

import (
	"go/build"
	"testing"
)

// TestProvenance_NeverImportsExecNetGithub is task 1.4 (RED): the
// architecture-invariant boundary test for the PURE provenance leaf
// (design.md "Provenance home" decision): its own (non-test) source files
// must never import os/exec, the internal/exec seam, net/http, or
// internal/github — any of those would pull exec transitively (github wraps
// exec) or otherwise turn provenance into an I/O-bearing package, breaking
// the "safe to link into both internal/app and cmd/deploydeck" purity
// guarantee the design relies on. Modeled on
// internal/app/boundary_test.go's TestApp_NeverImportsExecSeam.
func TestProvenance_NeverImportsExecNetGithub(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("importing internal/provenance package: %v", err)
	}

	// Non-vacuous guard: provenance must actually compose internal/version
	// (proving this test inspects a real, wired package — not an empty one).
	if !contains(pkg.Imports, "github.com/malavolta/DeployDeck/internal/version") {
		t.Fatalf("internal/provenance must import github.com/malavolta/DeployDeck/internal/version; direct imports were: %v", pkg.Imports)
	}

	forbidden := []string{
		"os/exec",
		"github.com/malavolta/DeployDeck/internal/exec",
		"net/http",
		"github.com/malavolta/DeployDeck/internal/github",
	}
	for _, f := range forbidden {
		if contains(pkg.Imports, f) {
			t.Errorf("internal/provenance must NOT import %q (keep this package a pure leaf); direct imports were: %v", f, pkg.Imports)
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
