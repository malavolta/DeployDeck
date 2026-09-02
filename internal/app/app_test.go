package app

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/gate"
)

// TestStateDeployGateBlocked_ModelFieldsExistAndAreWired is task 6.5 (RED):
// StateDeployGateBlocked plus the Model.gateCheckingRunID/gateConditions
// fields exist and are settable/readable — the minimal structural proof the
// rest of Phase 6's wiring (keys.go/update.go/view.go) builds on.
func TestStateDeployGateBlocked_ModelFieldsExistAndAreWired(t *testing.T) {
	m := New(Deps{})
	m.state = StateDeployGateBlocked
	m.gateCheckingRunID = "run-1"
	m.gateConditions = []gate.Condition{
		{Name: "approvals", Passed: false, Detail: "0/1 aprobaciones requeridas"},
	}

	if m.State() != StateDeployGateBlocked {
		t.Fatalf("State() = %v, want StateDeployGateBlocked", m.State())
	}
	if m.gateCheckingRunID != "run-1" {
		t.Fatalf("gateCheckingRunID = %q, want %q", m.gateCheckingRunID, "run-1")
	}
	if len(m.gateConditions) != 1 || m.gateConditions[0].Name != "approvals" {
		t.Fatalf("gateConditions = %+v, want 1 entry named approvals", m.gateConditions)
	}
}

// TestNew_NormalizeRoots_FlatLayoutInvariant is task 5.1 (RED):
// normalizeRoots seeds GitRoot/ProjectDir/ArtifactsRoot from the
// compatibility base Dir whenever they are left empty (design.md ADR-3) —
// so a Deps{Dir: d} literal alone (every one of the 241 existing test
// sites, unchanged) yields all three roots equal to d, exactly today's
// flat-layout behavior.
func TestNew_NormalizeRoots_FlatLayoutInvariant(t *testing.T) {
	const d = "/repo"

	m := New(Deps{Dir: d})

	if m.deps.GitRoot != d {
		t.Errorf("GitRoot = %q, want %q", m.deps.GitRoot, d)
	}
	if m.deps.ProjectDir != d {
		t.Errorf("ProjectDir = %q, want %q", m.deps.ProjectDir, d)
	}
	if m.deps.ArtifactsRoot != d {
		t.Errorf("ArtifactsRoot = %q, want %q", m.deps.ArtifactsRoot, d)
	}
}

// TestNew_NormalizeRoots_ExplicitRootsWin is task 5.1 (RED): when the three
// roots are set explicitly, normalizeRoots must NOT overwrite them from
// Dir — proving ADR-2's ArtifactsRoot != ProjectDir case is expressible and
// that Dir stays a pure fallback, never an override.
func TestNew_NormalizeRoots_ExplicitRootsWin(t *testing.T) {
	m := New(Deps{
		Dir:           "/should-not-be-used",
		GitRoot:       "/repo",
		ProjectDir:    "/repo/project",
		ArtifactsRoot: "/repo/project",
	})

	if m.deps.GitRoot != "/repo" {
		t.Errorf("GitRoot = %q, want %q", m.deps.GitRoot, "/repo")
	}
	if m.deps.ProjectDir != "/repo/project" {
		t.Errorf("ProjectDir = %q, want %q", m.deps.ProjectDir, "/repo/project")
	}
	if m.deps.ArtifactsRoot != "/repo/project" {
		t.Errorf("ArtifactsRoot = %q, want %q", m.deps.ArtifactsRoot, "/repo/project")
	}
}
