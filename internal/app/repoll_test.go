package app

import (
	"testing"

	"deploydeck/internal/git"
)

// conflictModel builds a Model parked on the conflict screen with one
// unresolved text conflict and continue disabled, as it would be right after
// CherryPick stopped on a conflict.
func conflictModel() Model {
	m := New(Deps{Dir: "/repo"})
	m.state = StateCherryPickConflict
	m.repoState = git.RepoState{
		InProgress: true,
		CurrentSHA: "deadbeef",
		Unmerged:   []git.ConflictFile{{Path: "classes/AccountService.cls", Kind: git.ConflictText}},
	}
	m.continueEnabled = false
	return m
}

// TestModel_Repoll_ReflectsExternalResolution is HU-006 AC4 (deferred task
// 10.19/10.20): a resolution performed OUTSIDE the TUI step is reflected on the
// next re-poll. The poll delivers a fresh RepoState + continue-gate as a
// repoStateMsg; the model must adopt it — never keep its own stale copy (repo
// is the source of truth).
func TestModel_Repoll_ReflectsExternalResolution(t *testing.T) {
	m := conflictModel()

	// External resolution: the conflict was resolved and staged elsewhere,
	// so the fresh poll reports zero unmerged and an enabled gate — but the
	// multi-commit sequence is still in progress.
	resolved := git.RepoState{InProgress: true, CurrentSHA: "deadbeef", Clean: false}
	next, _ := m.Update(repoStateMsg{state: resolved, gate: git.ContinueGate{Enabled: true}})
	nm := next.(Model)

	if !nm.continueEnabled {
		t.Errorf("after external resolution the continue gate must be reflected as enabled")
	}
	if len(nm.repoState.Unmerged) != 0 {
		t.Errorf("stale unmerged list must be replaced by the fresh poll; got %d files", len(nm.repoState.Unmerged))
	}
	if nm.State() != StateCherryPickConflict {
		t.Errorf("still mid-sequence: should stay on the conflict screen, got %v", nm.State())
	}

	// Triangulation: a poll that still shows the conflict keeps continue
	// disabled (the model does not latch the previous enabled value).
	stillConflicted := git.RepoState{
		InProgress: true,
		Unmerged:   []git.ConflictFile{{Path: "x.cls", Kind: git.ConflictText}},
	}
	next2, _ := nm.Update(repoStateMsg{state: stillConflicted, gate: git.ContinueGate{Enabled: false, Pending: []string{"x.cls (text) is unmerged/unstaged"}}})
	nm2 := next2.(Model)
	if nm2.continueEnabled {
		t.Errorf("a re-conflicted poll must disable continue again")
	}
	if len(nm2.repoState.Unmerged) != 1 {
		t.Errorf("fresh poll should show the 1 unmerged file")
	}
}

// TestModel_Repoll_ReconcilesExternalCompletion is HU-006 AC6: an external
// `--continue`/`--abort` that finishes the sequence (no cherry-pick in
// progress) is detected on re-read and the screen resynchronizes off the
// conflict screen toward verification.
func TestModel_Repoll_ReconcilesExternalCompletion(t *testing.T) {
	m := conflictModel()

	done := git.RepoState{InProgress: false, Clean: true}
	next, cmd := m.Update(repoStateMsg{state: done, gate: git.ContinueGate{Enabled: true}})
	nm := next.(Model)

	if nm.State() == StateCherryPickConflict {
		t.Errorf("external completion must move the screen off the conflict view")
	}
	if cmd == nil {
		t.Errorf("reconciliation should trigger the verification command")
	}
}

// TestModel_Tick_PollsOnlyDuringCherryPick is the re-poll cadence (task
// 10.20): a tick during the cherry-pick screens arms the next poll; a tick in
// any other state stops (returns nil) so no background polling leaks past the
// cherry-pick phase.
func TestModel_Tick_PollsOnlyDuringCherryPick(t *testing.T) {
	polling := []State{StateCherryPicking, StateCherryPickConflict}
	for _, st := range polling {
		m := New(Deps{Dir: "/repo"})
		m.state = st
		if _, cmd := m.Update(tickMsg{}); cmd == nil {
			t.Errorf("tick in %v must arm the next poll (non-nil cmd)", st)
		}
	}

	idle := []State{StateTicketInput, StatePickVerification, StateAborted}
	for _, st := range idle {
		m := New(Deps{Dir: "/repo"})
		m.state = st
		if _, cmd := m.Update(tickMsg{}); cmd != nil {
			t.Errorf("tick in %v must NOT keep polling (want nil cmd)", st)
		}
	}
}
