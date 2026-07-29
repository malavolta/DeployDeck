package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/runs"
)

// --- 2.1/2.2: shouldRestore pure guard matrix -------------------------------

// TestShouldRestore_GuardMatrix is task 2.1 (RED): restore is skipped iff
// InProgress, the original branch is empty/detached, it already equals the
// current branch, or it no longer resolves — restore otherwise. The
// InProgress guard exists because git REFUSES checkout with unmerged paths
// — this is the rationale, NOT a resume-detection need (CHERRY_PICK_HEAD
// lives in .git/ and is worktree-scoped, so it survives regardless of which
// branch is checked out).
func TestShouldRestore_GuardMatrix(t *testing.T) {
	tests := []struct {
		name       string
		inProgress bool
		original   string
		current    string
		exists     bool
		want       bool
	}{
		{"restores when eligible", false, "main", "deploy/PROJ-1-to-UAT", true, true},
		{"skips mid-conflict: git refuses checkout with unmerged paths", true, "main", "deploy/PROJ-1-to-UAT", true, false},
		{"skips empty original (no capture / degrade)", false, "", "deploy/PROJ-1-to-UAT", true, false},
		{"skips detached HEAD", false, "HEAD", "deploy/PROJ-1-to-UAT", true, false},
		{"skips when original already equals current", false, "main", "main", true, false},
		{"skips when original no longer resolves", false, "main", "deploy/PROJ-1-to-UAT", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRestore(tt.inProgress, tt.original, tt.current, tt.exists)
			if got != tt.want {
				t.Errorf("shouldRestore(%v, %q, %q, %v) = %v, want %v",
					tt.inProgress, tt.original, tt.current, tt.exists, got, tt.want)
			}
		})
	}
}

// --- 2.5/2.6: originalBranchCmd captures CurrentBranch ----------------------

// TestOriginalBranchCmd_CapturesCurrentBranch is task 2.5 (RED):
// originalBranchCmd runs git.Service.CurrentBranch and its result lands on
// the model via originalBranchMsg / Update.
func TestOriginalBranchCmd_CapturesCurrentBranch(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("main")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	msg := run(t, m.originalBranchCmd())
	obm, ok := msg.(originalBranchMsg)
	if !ok {
		t.Fatalf("expected originalBranchMsg, got %T", msg)
	}
	if obm.err != nil {
		t.Fatalf("originalBranchCmd errored: %v", obm.err)
	}
	if obm.branch != "main" {
		t.Errorf("branch = %q, want %q", obm.branch, "main")
	}

	next, _ := m.Update(obm)
	if got := next.(Model).originalBranch; got != "main" {
		t.Errorf("Update(originalBranchMsg) should set m.originalBranch, got %q", got)
	}
}

// TestOriginalBranchCmd_NilGitDegradesToNoCommand mirrors resumeDetectCmd's
// best-effort nil degrade: a nil Git yields no command (never a panic).
func TestOriginalBranchCmd_NilGitDegradesToNoCommand(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	if cmd := m.originalBranchCmd(); cmd != nil {
		t.Error("originalBranchCmd with nil Git should return nil")
	}
}

// --- 2.3/2.4: onPrereqDone batches resumeDetectCmd + originalBranchCmd -----

// TestOnPrereqDone_BatchesResumeDetectAndOriginalBranch is tasks 2.3/2.4
// (RED): onPrereqDone fires BOTH resumeDetectCmd and originalBranchCmd,
// batched via tea.Batch; when Git/Runs are absent both degrade to nil and
// tea.Batch drops them (never a panic, never a phantom command).
func TestOnPrereqDone_BatchesResumeDetectAndOriginalBranch(t *testing.T) {
	t.Run("Git+Runs present: both commands fire", func(t *testing.T) {
		fr := execpkg.NewFakeRunner()
		fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
		fr.When("git", []string{"rev-parse", "--absolute-git-dir"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo/.git")})
		fr.When("git", []string{"status", "--porcelain", "-z"}, execpkg.CommandResult{ExitCode: 0})
		fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("main")})

		m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig(), Runs: runs.NewWriter(t.TempDir())})
		next, cmd := m.Update(prereqDoneMsg{})
		m = next.(Model)
		if m.State() != StateMainMenu {
			t.Fatalf("onPrereqDone should advance to StateMainMenu (HU-018 landing), got %v", m.State())
		}
		if cmd == nil {
			t.Fatal("onPrereqDone should return a non-nil batched command when Git+Runs are wired")
		}
		msg := cmd()
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			t.Fatalf("expected tea.BatchMsg (2 non-nil commands), got %T", msg)
		}
		if len(batch) != 2 {
			t.Fatalf("expected 2 batched commands, got %d", len(batch))
		}
		var sawResume, sawOriginal bool
		for _, c := range batch {
			switch c().(type) {
			case resumeDetectMsg:
				sawResume = true
			case originalBranchMsg:
				sawOriginal = true
			}
		}
		if !sawResume {
			t.Error("expected resumeDetectCmd's resumeDetectMsg in the batch")
		}
		if !sawOriginal {
			t.Error("expected originalBranchCmd's originalBranchMsg in the batch")
		}
	})

	t.Run("Git/Runs absent: both degrade to nil, tea.Batch drops them", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		_, cmd := m.Update(prereqDoneMsg{})
		if cmd != nil {
			t.Errorf("expected a nil command when Git/Runs are absent, got non-nil")
		}
	})
}

// --- 2.7: quitCmd restores on a deliberate terminal quit --------------------

// TestQuitCmd_RestoresOriginalBranch_OnTerminalQuit is task 2.7 (RED): a
// deliberate quit from a terminal state (StateSucceeded, q) restores the
// original branch — proving the 13-site keys.go wiring (task 2.14), not just
// quitCmd in isolation.
func TestQuitCmd_RestoresOriginalBranch_OnTerminalQuit(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("deploy/PROJ-1-to-UAT")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"checkout", "main", "--"}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.state = StateSucceeded

	_, cmd := m.Update(keyPress("q"))
	if cmd == nil {
		t.Fatal("q from StateSucceeded should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("quitCmd should ultimately return tea.QuitMsg, got %T", msg)
	}
	if !calledWith(fr, "git", "checkout", "main", "--") {
		t.Fatalf("quitCmd should restore the original branch via Checkout(main); calls: %v", fr.Calls)
	}
}

// --- 2.8: MANDATORY negative — mid-conflict quit never restores ------------

// TestQuitCmd_AbortMidConflict_NoRestore is task 2.8 (MANDATORY negative,
// protects HU-013): quitting mid-conflict (RepoState.InProgress) must NEVER
// attempt a checkout — git refuses checkout with unmerged paths, and
// CHERRY_PICK_HEAD must stay intact for the next run's resume-detection.
func TestQuitCmd_AbortMidConflict_NoRestore(t *testing.T) {
	fr := execpkg.NewFakeRunner() // no canned responses: any git call fails loudly

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.repoState = git.RepoState{InProgress: true, CurrentSHA: "sha-A"}
	m.state = StateCherryPickConflict

	_, cmd := m.Update(keyPress("q"))
	if cmd == nil {
		t.Fatal("q from StateCherryPickConflict should still return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("quitCmd should still return tea.QuitMsg, got %T", msg)
	}
	if len(fr.Calls) != 0 {
		t.Fatalf("mid-conflict quit must never attempt any git call (never mind a checkout); calls: %v", fr.Calls)
	}
}

// --- 2.9: no-op restore cases ------------------------------------------------

// TestQuitCmd_NoopWhenOriginalGoneDetachedOrCurrent is task 2.9: every no-op
// guard case still quits, but attempts zero checkouts.
func TestQuitCmd_NoopWhenOriginalGoneDetachedOrCurrent(t *testing.T) {
	tests := []struct {
		name           string
		originalBranch string
		currentBranch  string
		existsExit     int // exit code BranchExists' local rev-parse should return
	}{
		{"empty original (never captured)", "", "deploy/PROJ-1-to-UAT", 0},
		{"detached HEAD", "HEAD", "deploy/PROJ-1-to-UAT", 0},
		{"original already current", "main", "main", 0},
		{"original no longer exists", "main", "deploy/PROJ-1-to-UAT", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := execpkg.NewFakeRunner()
			fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
			fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(tt.currentBranch)})
			if tt.originalBranch != "" && tt.originalBranch != "HEAD" && tt.originalBranch != tt.currentBranch {
				fr.When("git", []string{"rev-parse", "--verify", "--quiet", tt.originalBranch}, execpkg.CommandResult{ExitCode: tt.existsExit, Stdout: []byte("sha1")})
				if tt.existsExit != 0 {
					fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + tt.originalBranch}, execpkg.CommandResult{ExitCode: 1})
				}
			}

			m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
			m.originalBranch = tt.originalBranch

			msg := run(t, m.quitCmd())
			if _, ok := msg.(tea.QuitMsg); !ok {
				t.Fatalf("quitCmd should still return tea.QuitMsg, got %T", msg)
			}
			if calledWith(fr, "git", "checkout", tt.originalBranch, "--") {
				t.Fatalf("%s must never checkout; calls: %v", tt.name, fr.Calls)
			}
		})
	}
}

// --- Regression: existing 13 keys.go quit sites still quit -----------------

// TestQuit_RegressionAcrossRepresentativeSites is a regression check that the
// keys.go 13-site tea.Quit -> m.quitCmd() swap (task 2.14) preserves the
// existing "the app quits" contract at a representative sample of sites: the
// shared terminal-state handler in handleKey (StateFailed), and keyPrereq's
// dedicated q.
func TestQuit_RegressionAcrossRepresentativeSites(t *testing.T) {
	t.Run("shared terminal handler (StateFailed)", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StateFailed
		_, cmd := m.Update(keyPress("q"))
		if cmd == nil {
			t.Fatal("q from StateFailed should return a quit command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("expected tea.QuitMsg, got %T", cmd())
		}
	})

	t.Run("keyPrereq", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StatePrereqCheck
		_, cmd := m.Update(keyPress("q"))
		if cmd == nil {
			t.Fatal("q from StatePrereqCheck should return a quit command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("expected tea.QuitMsg, got %T", cmd())
		}
	})
}

// TestCtrlC_StaysHardInterrupt_NoRestore proves update.go's ctrl+c path is
// left untouched (design decision, task list note on 2.14): it must never
// route through quitCmd, so it must never attempt a restore checkout even
// with a wired Git + non-empty originalBranch.
func TestCtrlC_StaysHardInterrupt_NoRestore(t *testing.T) {
	fr := execpkg.NewFakeRunner() // no canned responses

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.state = StateSucceeded

	_, cmd := m.Update(keyPress("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c should still return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c should return tea.QuitMsg, got %T", cmd())
	}
	if len(fr.Calls) != 0 {
		t.Errorf("ctrl+c must never attempt a restore checkout, calls: %v", fr.Calls)
	}
}
