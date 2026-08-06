package gate_test

import (
	"go/build"
	"testing"
)

// TestGate_NeverImportsExecNetGithub is the architecture-invariant boundary
// test for the PURE internal/gate package (design.md "Gate package"
// decision: deps ONLY config+provenance, no exec/gh/net) — modeled on
// internal/provenance/boundary_test.go's TestProvenance_NeverImportsExecNetGithub.
// internal/gate's Evaluate must be exhaustively unit-testable without gh: it
// consumes Facts already mapped by internal/app's boundary, never fetching
// anything itself.
func TestGate_NeverImportsExecNetGithub(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("importing internal/gate package: %v", err)
	}

	// Non-vacuous guard: gate must actually compose config and provenance
	// (proving this test inspects a real, wired package — not an empty one).
	if !contains(pkg.Imports, "github.com/malavolta/DeployDeck/internal/config") {
		t.Fatalf("internal/gate must import github.com/malavolta/DeployDeck/internal/config; direct imports were: %v", pkg.Imports)
	}
	if !contains(pkg.Imports, "github.com/malavolta/DeployDeck/internal/provenance") {
		t.Fatalf("internal/gate must import github.com/malavolta/DeployDeck/internal/provenance; direct imports were: %v", pkg.Imports)
	}

	forbidden := []string{
		"os/exec",
		"github.com/malavolta/DeployDeck/internal/exec",
		"net/http",
		"github.com/malavolta/DeployDeck/internal/github",
	}
	for _, f := range forbidden {
		if contains(pkg.Imports, f) {
			t.Errorf("internal/gate must NOT import %q (keep this package a pure leaf); direct imports were: %v", f, pkg.Imports)
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
