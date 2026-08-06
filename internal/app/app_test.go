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
