package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/runs"
)

// currentBranchOf returns the real, currently checked-out short branch name
// in dir via `git symbolic-ref --short HEAD`, failing the test on error (a
// detached HEAD has no symbolic ref and is never expected in these tests).
func currentBranchOf(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git symbolic-ref --short HEAD in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// driveOnPrereqDone runs the REAL onPrereqDone batched command (never the
// advance-with-discarded-cmd shortcut) so m.originalBranch is captured from
// the actual startup branch exactly as the real app wires it (design.md:
// "onPrereqDone returns tea.Batch(resumeDetectCmd(), originalBranchCmd())").
func driveOnPrereqDone(t *testing.T, m Model) Model {
	t.Helper()
	next, cmd := m.Update(prereqDoneMsg{})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("onPrereqDone should return a non-nil batched command when Git+Runs are wired")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}
	for _, c := range batch {
		m = advance(t, m, c())
	}
	return m
}

// TestHU017_RestoreOnQuit_RealRepo is task 2.11 (RED, real-git): the flow
// starts on "main", checks out a real deploy/* promotion branch, reaches a
// terminal state, and quitting restores HEAD back to "main" — proving the
// branch-cleanup spec's "Finish or abort restores the original branch"
// scenario against a real repository, not a FakeRunner double.
func TestHU017_RestoreOnQuit_RealRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupFlowRepo(t)
	if got := currentBranchOf(t, local); got != "main" {
		t.Fatalf("test setup bug: expected the fresh clone to start on main, got %q", got)
	}

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: flowConfig(),
		Dir:    local,
		Runs:   runs.NewWriter(local),
	}
	m := New(deps)
	m = driveOnPrereqDone(t, m)
	if m.originalBranch != "main" {
		t.Fatalf("originalBranch = %q, want %q (captured at startup, before any branch switch)", m.originalBranch, "main")
	}

	m = typeString(m, "PROJ-1")
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateCommitDiscovery {
		t.Fatalf("after ticket enter: %v", m.State())
	}
	m = advance(t, m, run(t, m.discoverCmd()))
	if m.State() != StateCommitSelection {
		t.Fatalf("after discovery: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, keyPress("enter")) // confirm selection (all selected)
	if m.State() != StateTargetSelection {
		t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
	}
	m = advance(t, m, keyPress("enter")) // confirm UAT target
	if m.State() != StatePlanPreview {
		t.Fatalf("after target confirm: %v (notice=%q)", m.State(), m.notice)
	}
	m = advance(t, m, keyPress("enter")) // confirm plan -> BranchCreation
	if m.State() != StateBranchCreation {
		t.Fatalf("after plan confirm: %v", m.State())
	}
	m = advance(t, m, run(t, m.branchCreateCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after branch creation: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after cherry-pick, expected to stay in CherryPicking pending verify, got %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.verifyCmd()))
	if m.State() != StatePickVerification {
		t.Fatalf("expected a clean, verified pick, got %v (err=%v)", m.State(), m.Err())
	}
	if deployBranch := currentBranchOf(t, local); deployBranch == "main" {
		t.Fatal("test setup bug: expected to have checked out the deploy branch by now")
	}

	// Reach a terminal state (validation itself is out of this group's
	// scope): quitCmd's restore-on-terminal-quit path only needs the
	// terminal state + the real checked-out deploy branch.
	m.state = StateSucceeded

	_, quitCmd := m.Update(keyPress("q"))
	if quitCmd == nil {
		t.Fatal("q from StateSucceeded should return a quit command")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatal("quitCmd should return tea.QuitMsg")
	}

	if head := currentBranchOf(t, local); head != "main" {
		t.Fatalf("HEAD after quit = %q, want restored to %q", head, "main")
	}
}

// TestHU017_AbortMidConflict_NoRestore_RealRepo is task 2.12 (MANDATORY,
// real-git): quitting from a REAL unresolved cherry-pick conflict must NEVER
// restore — HEAD stays on the deploy branch and CHERRY_PICK_HEAD survives
// intact, so HU-013 resume-detection remains possible on the next run
// (branch-cleanup spec: "Unresolved conflict blocks restore").
func TestHU017_AbortMidConflict_NoRestore_RealRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupConflictRepo2(t)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: pickIndexConfig(),
		Dir:    local,
		Runs:   runs.NewWriter(local),
	}
	m := New(deps)
	m = driveOnPrereqDone(t, m)
	if m.originalBranch != "main" {
		t.Fatalf("originalBranch = %q, want %q", m.originalBranch, "main")
	}

	m = typeString(m, "PROJ-1")
	m = advance(t, m, keyPress("enter"))
	m = advance(t, m, run(t, m.discoverCmd()))
	if m.State() != StateCommitSelection {
		t.Fatalf("after discovery: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, keyPress("enter")) // confirm selection
	if m.State() != StateTargetSelection {
		t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
	}
	m = advance(t, m, keyPress("enter")) // confirm UAT target
	m = advance(t, m, keyPress("enter")) // confirm plan -> BranchCreation
	if m.State() != StateBranchCreation {
		t.Fatalf("after plan confirm: %v", m.State())
	}
	m = advance(t, m, run(t, m.branchCreateCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after branch creation: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPickConflict {
		t.Fatalf("expected a conflict, got %v (err=%v)", m.State(), m.Err())
	}
	if !m.repoState.InProgress {
		t.Fatal("expected RepoState.InProgress on a real unresolved conflict")
	}
	deployBranch := currentBranchOf(t, local)
	if deployBranch == "main" {
		t.Fatal("test setup bug: expected to be checked out on the deploy branch mid-conflict")
	}
	cherryPickHead := filepath.Join(local, ".git", "CHERRY_PICK_HEAD")
	if _, err := os.Stat(cherryPickHead); err != nil {
		t.Fatalf("test setup bug: expected a real CHERRY_PICK_HEAD, stat: %v", err)
	}

	_, quitCmd := m.Update(keyPress("q"))
	if quitCmd == nil {
		t.Fatal("q from StateCherryPickConflict should still return a quit command")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatal("quitCmd should still return tea.QuitMsg")
	}

	if head := currentBranchOf(t, local); head != deployBranch {
		t.Fatalf("HEAD after mid-conflict quit = %q, want unchanged %q (no restore)", head, deployBranch)
	}
	if _, err := os.Stat(cherryPickHead); err != nil {
		t.Fatalf("CHERRY_PICK_HEAD must remain intact for HU-013 resume-detection: %v", err)
	}
}
