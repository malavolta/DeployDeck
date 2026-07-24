package app

import (
	"testing"

	"deploydeck/internal/prereq"
)

// TestModel_PrereqCheck_To_TicketInput drives the first transition directly
// through Model.Update (design's state diagram: PrereqCheck --> TicketInput on
// OK). It triangulates the two branches that decide it: an all-OK report
// advances to ticket input; a report containing a blocking check keeps the
// user on the doctor screen so they can fix and retry.
func TestModel_PrereqCheck_To_TicketInput(t *testing.T) {
	tests := []struct {
		name      string
		checks    []prereq.PrereqCheck
		wantState State
	}{
		{
			name: "all OK advances to ticket input",
			checks: []prereq.PrereqCheck{
				{Name: "git", Status: prereq.StatusOK},
				{Name: "sf", Status: prereq.StatusOK},
				{Name: "alias", Status: prereq.StatusWarning}, // a warning does NOT block
			},
			wantState: StateTicketInput,
		},
		{
			name: "a blocking check keeps the user on the doctor screen",
			checks: []prereq.PrereqCheck{
				{Name: "git", Status: prereq.StatusOK},
				{Name: "repo", Status: prereq.StatusBlocking},
			},
			wantState: StatePrereqCheck,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo"})
			if m.State() != StatePrereqCheck {
				t.Fatalf("New should start in StatePrereqCheck, got %v", m.State())
			}

			next, _ := m.Update(prereqDoneMsg{checks: tt.checks})
			got := next.(Model).State()
			if got != tt.wantState {
				t.Errorf("after prereqDoneMsg: state = %v, want %v", got, tt.wantState)
			}
		})
	}
}

// TestModel_PrereqCheck_Error routes a checker error to the error state.
func TestModel_PrereqCheck_Error(t *testing.T) {
	m := New(Deps{Dir: "/repo"})
	next, _ := m.Update(prereqDoneMsg{err: errStub})
	nm := next.(Model)
	if nm.State() != StateError {
		t.Fatalf("prereq error should route to StateError, got %v", nm.State())
	}
	if nm.Err() == nil {
		t.Errorf("expected the error to be recorded")
	}
}

// TestModel_PrereqScreen_ContinueBlockedByBlocker proves the `c` key cannot
// bypass a blocking prerequisite.
func TestModel_PrereqScreen_ContinueBlockedByBlocker(t *testing.T) {
	m := New(Deps{Dir: "/repo"})
	m.checks = []prereq.PrereqCheck{{Name: "repo", Status: prereq.StatusBlocking}}

	next, _ := m.Update(keyPress("c"))
	if next.(Model).State() != StatePrereqCheck {
		t.Errorf("`c` must not advance past a blocking prereq")
	}

	// With only warnings, `c` continues.
	m.checks = []prereq.PrereqCheck{{Name: "alias", Status: prereq.StatusWarning}}
	next, _ = m.Update(keyPress("c"))
	if next.(Model).State() != StateTicketInput {
		t.Errorf("`c` should continue past warnings, got %v", next.(Model).State())
	}
}
