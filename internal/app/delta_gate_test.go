package app

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
)

// TestModel_DeltaAllowed_ThreadsAbortedFlag proves the run's real `aborted`
// flag is threaded into git.DeltaAndValidationAllowed from the lifecycle: a
// successful abort leaves a CLEAN working tree, but must still be treated as a
// failed run (delta/validation stay disabled), NOT a clean completion.
func TestModel_DeltaAllowed_ThreadsAbortedFlag(t *testing.T) {
	tests := []struct {
		name      string
		repoState git.RepoState
		aborted   bool
		want      bool
	}{
		{
			name:      "clean completion allows delta",
			repoState: git.RepoState{Clean: true, InProgress: false},
			aborted:   false,
			want:      true,
		},
		{
			name:      "successful abort (clean tree) is NOT clean completion",
			repoState: git.RepoState{Clean: true, InProgress: false},
			aborted:   true,
			want:      false,
		},
		{
			name:      "mid-conflict never allows delta",
			repoState: git.RepoState{InProgress: true},
			aborted:   false,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo"})
			m.repoState = tt.repoState
			m.aborted = tt.aborted
			if got := m.DeltaAllowed(); got != tt.want {
				t.Errorf("DeltaAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestModel_Abort_SetsAbortedTerminal proves the abort message sets the
// aborted terminal so the flag is live for DeltaAllowed even after the abort
// resets the tree to clean.
func TestModel_Abort_SetsAbortedTerminal(t *testing.T) {
	m := conflictModel()
	next, _ := m.Update(abortedMsg{})
	nm := next.(Model)

	if nm.State() != StateAborted {
		t.Fatalf("abort should reach StateAborted, got %v", nm.State())
	}
	if nm.DeltaAllowed() {
		t.Errorf("an aborted run must not allow delta/validation")
	}
}

// TestModel_VerifyDone_ComputesDeltaAllowed proves a normal completion path
// (verifyDoneMsg with no error, not aborted) records deltaAllowed=true and
// reaches PickVerification, the scope edge.
func TestModel_VerifyDone_ComputesDeltaAllowed(t *testing.T) {
	m := New(Deps{Dir: "/repo"})
	m.state = StateCherryPicking
	m.repoState = git.RepoState{Clean: true, InProgress: false}

	next, _ := m.Update(verifyDoneMsg{verification: git.PickVerification{}})
	nm := next.(Model)

	if nm.State() != StatePickVerification {
		t.Fatalf("verifyDone should reach StatePickVerification, got %v", nm.State())
	}
	if !nm.DeltaAllowed() {
		t.Errorf("clean, non-aborted completion should allow delta")
	}
}
