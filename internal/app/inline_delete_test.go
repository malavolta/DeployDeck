package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// --- M-2: cross-screen cleanupPhase leak reset on terminal entry ------------

// TestOnReportDone_TerminalEntryResetsStaleCleanupState is the M-2 remediation
// (RED): entering a terminal state (here StateSucceeded via a Succeeded report)
// MUST reset any leaked delete-confirmation state, so a stray key on the
// terminal screen can never funnel into keyDeleteConfirm and delete
// plan.PromotionBranch at a wrong-branch confirm strength.
func TestOnReportDone_TerminalEntryResetsStaleCleanupState(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateValidationPolling
	// Simulate the M-2 leak: a stale non-idle cleanupPhase + pending delete
	// bled over from an earlier d+esc on the cleanup screen.
	m.cleanupPhase = cleanupStrongConfirm
	m.pendingDeleteCurrent = true
	m.deleteConfirm = "BOR"
	m.cleanupDeleteTarget = "deploy/PROJ-1-to-UAT"
	m.plan = git.DeploymentPlan{PromotionBranch: "deploy/PROJ-1-to-UAT"}

	next, _ := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded"}})
	nm := next.(Model)
	if nm.State() != StateSucceeded {
		t.Fatalf("a Succeeded report should reach StateSucceeded, got %v", nm.State())
	}
	if nm.cleanupPhase != cleanupIdle {
		t.Fatalf("entering a terminal state must reset the stale cleanupPhase, got %v", nm.cleanupPhase)
	}
	if nm.pendingDeleteCurrent {
		t.Error("terminal entry must reset pendingDeleteCurrent")
	}
	if nm.cleanupDeleteTarget != "" {
		t.Error("terminal entry must reset cleanupDeleteTarget")
	}

	// A stray 'y' must now be inert: cleanupIdle routes keySucceeded normally,
	// where 'y' is not a bound key, so no delete/quit command fires.
	fr := execpkg.NewFakeRunner() // uncanned: any git call fails loudly
	nm.deps.Git = git.New(fr)
	nm.deps.Dir = "/repo"
	next2, cmd := nm.Update(keyPress("y"))
	if next2.(Model).State() != StateSucceeded {
		t.Error("a stray y on a freshly-entered terminal must not change state")
	}
	if cmd != nil {
		t.Error("a stray y must not fire a delete/quit command")
	}
	if len(fr.Calls) != 0 {
		t.Errorf("a stray y must not invoke any git call, got %v", fr.Calls)
	}
}

// TestOnAborted_ResetsStaleCleanupState is M-2's StateAborted companion: the
// same reset happens when the flow reaches StateAborted.
func TestOnAborted_ResetsStaleCleanupState(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateCherryPickConflict
	m.cleanupPhase = cleanupStrongConfirm
	m.pendingDeleteCurrent = true
	m.deleteConfirm = "BOR"
	m.cleanupDeleteTarget = "deploy/PROJ-1-to-UAT"

	next, _ := m.Update(abortedMsg{})
	nm := next.(Model)
	if nm.State() != StateAborted {
		t.Fatalf("a confirmed abort should reach StateAborted, got %v", nm.State())
	}
	if nm.cleanupPhase != cleanupIdle {
		t.Fatalf("entering StateAborted must reset the stale cleanupPhase, got %v", nm.cleanupPhase)
	}
	if nm.pendingDeleteCurrent || nm.cleanupDeleteTarget != "" {
		t.Error("entering StateAborted must reset pendingDeleteCurrent and cleanupDeleteTarget")
	}
}

// --- 3.1: onPushDone sets currentPushed on success --------------------------

// TestOnPushDone_SetsCurrentPushedTrueOnSuccess is task 3.1 (RED): a
// successful push sets m.currentPushed = true (design.md "Corrections Baked
// In" #4), so quitCmd's later inline-delete step knows to also delete the
// origin ref.
func TestOnPushDone_SetsCurrentPushedTrueOnSuccess(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: pushGit("/repo", branch, "git@github.com:org/repo.git", true)})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(pushDoneMsg{})
	nm := next.(Model)
	if !nm.currentPushed {
		t.Fatal("a successful push should set currentPushed = true")
	}
	if cmd == nil {
		t.Fatal("a successful push should still fire preparePRCmd")
	}
}

// TestOnPushDone_LeavesFalseOnFailure is task 3.1's companion: a failed push
// must NOT set currentPushed.
func TestOnPushDone_LeavesFalseOnFailure(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	next, cmd := m.Update(pushDoneMsg{err: errStub})
	nm := next.(Model)
	if nm.currentPushed {
		t.Fatal("a failed push must not set currentPushed")
	}
	if cmd != nil {
		t.Fatal("a failed push should not fire another command")
	}
}

// --- 3.3/3.4: d fires unpushedCountCmd, gating confirm strength -------------

// TestKeySucceeded_D_UnpushedRequiresStrongConfirm is task 3.3 (RED): `d`
// fires unpushedCountCmd(m.plan.PromotionBranch); a count > 0 gates the
// strong (typed BORRAR) confirmation.
func TestKeySucceeded_D_UnpushedRequiresStrongConfirm(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"rev-list", "origin/" + branch + ".." + branch, "--count"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("3\n")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateSucceeded
	m.plan = git.DeploymentPlan{PromotionBranch: branch}

	next, cmd := m.Update(keyPress("d"))
	nm := next.(Model)
	if nm.State() != StateSucceeded {
		t.Fatalf("d should stay on StateSucceeded while the count query is in flight, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("d should fire unpushedCountCmd")
	}
	msg := cmd()
	umsg, ok := msg.(unpushedMsg)
	if !ok {
		t.Fatalf("expected unpushedMsg, got %T", msg)
	}
	if umsg.err != nil {
		t.Fatalf("unpushedCountCmd errored: %v", umsg.err)
	}
	if umsg.count != 3 {
		t.Fatalf("count = %d, want 3", umsg.count)
	}

	next2, _ := nm.Update(umsg)
	nm2 := next2.(Model)
	if nm2.cleanupPhase != cleanupStrongConfirm {
		t.Fatalf("count>0 should gate the strong (typed BORRAR) confirmation, got %v", nm2.cleanupPhase)
	}
}

// TestKeySucceeded_D_PushedUsesNormalConfirm is task 3.4 (RED)'s companion: a
// count == 0 gates only the normal ('y') confirmation.
func TestKeySucceeded_D_PushedUsesNormalConfirm(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"rev-list", "origin/" + branch + ".." + branch, "--count"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("0\n")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateSucceeded
	m.plan = git.DeploymentPlan{PromotionBranch: branch}

	_, cmd := m.Update(keyPress("d"))
	msg := run(t, cmd)
	umsg, ok := msg.(unpushedMsg)
	if !ok {
		t.Fatalf("expected unpushedMsg, got %T", msg)
	}
	if umsg.count != 0 {
		t.Fatalf("count = %d, want 0", umsg.count)
	}

	next, _ := m.Update(umsg)
	nm := next.(Model)
	if nm.cleanupPhase != cleanupConfirm {
		t.Fatalf("count==0 should gate the normal 'y' confirmation, got %v", nm.cleanupPhase)
	}
}

// TestOnUnpushedCount_ErrorDefaultsToStrongConfirm is a triangulation case
// for 3.3/3.4: a resolution failure is treated conservatively, requiring the
// strong confirmation rather than silently defaulting to normal.
func TestOnUnpushedCount_ErrorDefaultsToStrongConfirm(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateSucceeded

	next, cmd := m.Update(unpushedMsg{err: errStub})
	nm := next.(Model)
	if nm.cleanupPhase != cleanupStrongConfirm {
		t.Fatalf("an unresolved count should conservatively require the strong confirmation, got %v", nm.cleanupPhase)
	}
	if cmd != nil {
		t.Error("landing the count result should not fire another command")
	}
}

// --- 3.5 (StateAborted companion) + regression: other terminals stay inert --

// TestKeyAborted_D_FiresUnpushedCountCmd proves keyAborted gains the same `d`
// wiring as keySucceeded (task 3.5's StateAborted half).
func TestKeyAborted_D_FiresUnpushedCountCmd(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"rev-list", "origin/" + branch + ".." + branch, "--count"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("0\n")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateAborted
	m.plan = git.DeploymentPlan{PromotionBranch: branch}

	next, cmd := m.Update(keyPress("d"))
	if next.(Model).State() != StateAborted {
		t.Fatalf("d on Aborted should stay on Aborted while the count query is in flight, got %v", next.(Model).State())
	}
	if cmd == nil {
		t.Fatal("d on Aborted should fire unpushedCountCmd, matching keySucceeded")
	}
	if _, ok := run(t, cmd).(unpushedMsg); !ok {
		t.Fatalf("expected unpushedMsg, got %T", run(t, cmd))
	}
}

// TestKeyAborted_QuitsNormally_DInertOnOtherTerminals is a regression guard:
// StateAborted still quits normally on q/enter/esc, and Failed/Canceled/Error
// stay quit-only — `d` is inert there (HU-017's inline delete is offered only
// on Succeeded/Aborted, per design.md's task-list wiring).
func TestKeyAborted_QuitsNormally_DInertOnOtherTerminals(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateAborted
	_, cmd := m.Update(keyPress("q"))
	if cmd == nil {
		t.Fatal("q from Aborted should still quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}

	for _, st := range []State{StateFailed, StateCanceled, StateError} {
		mm := New(Deps{Dir: "/repo", Config: testConfig()})
		mm.state = st
		next, _ := mm.Update(keyPress("d"))
		if next.(Model).State() != st {
			t.Errorf("d must not leave terminal state %v (inline delete offered only on Succeeded/Aborted)", st)
		}
	}
}

// --- 3.6: MANDATORY blocker fix — resumed run targets plan.PromotionBranch -

// TestInlineDelete_ResumedRun_TargetsPlanPromotionBranch_NotBranchName is
// task 3.6 (RED, MANDATORY blocker fix): on a RESUMED run, m.branchName is
// empty (resumeInto never sets it — only m.plan.PromotionBranch, reconstructed
// via git.RenderBranchName, exactly like viewRunHistory / the HU-014 push-fix
// regression test). The inline delete must target plan.PromotionBranch, never
// the empty m.branchName (which would silently no-op the delete / pass an
// invalid empty branch argument to git).
func TestInlineDelete_ResumedRun_TargetsPlanPromotionBranch_NotBranchName(t *testing.T) {
	wantBranch := "deploy/PROJ-1-to-UAT" // config.DefaultBranchFormat rendered

	clk := &fakeClock{t: time.Unix(1000, 0)}
	m := New(Deps{Dir: t.TempDir(), Config: testConfig(), SF: reportSF(t, "JOB1", "UAT_SBX", "InProgress"), Now: clk.now})
	m.repoState = git.RepoState{Clean: true}
	rec := runs.Record{RunID: "run-2", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "JOB1", Status: "InProgress", Phase: "validating"}

	next, _ := m.resumeInto(rec)
	m = next.(Model)
	if m.branchName != "" {
		t.Fatalf("test setup bug: expected branchName to stay empty on resume, got %q", m.branchName)
	}
	if m.plan.PromotionBranch != wantBranch {
		t.Fatalf("test setup bug: plan.PromotionBranch = %q, want %q", m.plan.PromotionBranch, wantBranch)
	}

	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(wantBranch)})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"checkout", "main", "--"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"branch", "-D", "--", wantBranch}, execpkg.CommandResult{ExitCode: 0})
	m.deps.Git = git.New(fr)
	m.deps.Dir = "/repo"
	m.originalBranch = "main"
	m.state = StateSucceeded
	m.pendingDeleteCurrent = true

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if !calledWith(fr, "git", "branch", "-D", "--", wantBranch) {
		t.Fatalf("delete should target plan.PromotionBranch %q (never the empty branchName); calls: %v", wantBranch, fr.Calls)
	}
	if calledWith(fr, "git", "branch", "-D", "--", "") {
		t.Fatal("delete must never target the empty branchName")
	}
}

// --- 3.7: strong-confirm typed BORRAR gate -----------------------------------

// strongConfirmModel returns a Model parked on StateSucceeded mid a strong
// (typed BORRAR) delete confirmation, with a FakeRunner canned for the
// restore-checkout + local delete quitCmd will run on a correct confirmation.
func strongConfirmModel(t *testing.T) (Model, *execpkg.FakeRunner, string) {
	t.Helper()
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(branch)})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"checkout", "main", "--"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.state = StateSucceeded
	m.cleanupPhase = cleanupStrongConfirm
	return m, fr, branch
}

// TestStrongConfirm_BORRAR_TypedGate is task 3.7 (RED): wrong text is refused
// with a notice and deletes nothing; the exact case-sensitive literal BORRAR
// confirms and deletes. Mirrors keyCancelConfirm's typed-input idiom, but on
// the dedicated m.deleteConfirm field.
func TestStrongConfirm_BORRAR_TypedGate(t *testing.T) {
	t.Run("wrong text is refused, nothing deleted", func(t *testing.T) {
		m, fr, _ := strongConfirmModel(t)
		m = typeString(m, "BORRA") // incomplete

		next, cmd := m.Update(keyPress("enter"))
		nm := next.(Model)
		if nm.cleanupPhase != cleanupStrongConfirm {
			t.Fatalf("a wrong confirmation must stay on the strong-confirm screen, got %v", nm.cleanupPhase)
		}
		if cmd != nil {
			t.Error("a wrong confirmation must NOT fire the delete/quit command")
		}
		if nm.notice == "" {
			t.Error("a wrong confirmation should surface a notice")
		}
		if len(fr.Calls) != 0 {
			t.Errorf("no git call should have been made yet, got %v", fr.Calls)
		}
	})

	t.Run("lowercase does not match (case-sensitive)", func(t *testing.T) {
		m, _, _ := strongConfirmModel(t)
		m = typeString(m, "borrar")

		next, cmd := m.Update(keyPress("enter"))
		if next.(Model).cleanupPhase != cleanupStrongConfirm {
			t.Fatal("lowercase 'borrar' must not match the exact literal BORRAR")
		}
		if cmd != nil {
			t.Error("a case-mismatched confirmation must NOT fire a command")
		}
	})

	t.Run("backspace edits the buffer, then the corrected text confirms", func(t *testing.T) {
		m, _, branch := strongConfirmModel(t)
		m = typeString(m, "BORRARX")

		next, _ := m.Update(keyPress("backspace"))
		nm := next.(Model)
		if nm.deleteConfirm != "BORRAR" {
			t.Fatalf("backspace should drop the trailing rune, got %q", nm.deleteConfirm)
		}

		next2, cmd := nm.Update(keyPress("enter"))
		if cmd == nil {
			t.Fatal("the corrected BORRAR should fire the delete/quit command")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Fatalf("expected tea.QuitMsg, got %T", msg)
		}
		if next2.(Model).cleanupPhase != cleanupIdle {
			t.Errorf("a successful confirmation should reset cleanupPhase to idle, got %v", next2.(Model).cleanupPhase)
		}
		_ = branch
	})

	t.Run("exact BORRAR deletes locally", func(t *testing.T) {
		m, fr, branch := strongConfirmModel(t)
		m = typeString(m, "BORRAR")

		next, cmd := m.Update(keyPress("enter"))
		if cmd == nil {
			t.Fatal("the exact BORRAR confirmation should fire the delete/quit command")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Fatalf("expected tea.QuitMsg, got %T", msg)
		}
		if !calledWith(fr, "git", "branch", "-D", "--", branch) {
			t.Fatalf("expected the local branch to be deleted; calls: %v", fr.Calls)
		}
		if !next.(Model).pendingDeleteCurrent {
			t.Error("confirming should have set pendingDeleteCurrent")
		}
	})

	t.Run("esc backs out without deleting", func(t *testing.T) {
		m, fr, _ := strongConfirmModel(t)
		m = typeString(m, "BOR")

		next, cmd := m.Update(keyPress("esc"))
		nm := next.(Model)
		if nm.cleanupPhase != cleanupIdle {
			t.Fatalf("esc should back out to cleanupIdle, got %v", nm.cleanupPhase)
		}
		if nm.deleteConfirm != "" {
			t.Errorf("esc should clear the typed buffer, got %q", nm.deleteConfirm)
		}
		if cmd != nil {
			t.Error("esc must not fire a command")
		}
		if len(fr.Calls) != 0 {
			t.Errorf("esc must NOT invoke any git call, got %v", fr.Calls)
		}
	})
}

// --- 3.9: normal confirm sets pendingDeleteCurrent, then quits --------------

// TestConfirmDelete_SetsPendingDeleteCurrent_ThenQuits is task 3.9 (RED): a
// normal ('y') confirmation sets m.pendingDeleteCurrent and fires quitCmd,
// which performs the restore checkout + delete.
func TestConfirmDelete_SetsPendingDeleteCurrent_ThenQuits(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(branch)})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"checkout", "main", "--"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.state = StateSucceeded
	m.cleanupPhase = cleanupConfirm

	next, cmd := m.Update(keyPress("y"))
	nm := next.(Model)
	if !nm.pendingDeleteCurrent {
		t.Fatal("confirming delete should set pendingDeleteCurrent")
	}
	if nm.cleanupPhase != cleanupIdle {
		t.Errorf("confirming should reset cleanupPhase to idle, got %v", nm.cleanupPhase)
	}
	if cmd == nil {
		t.Fatal("confirming delete should fire the quit/delete command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if !calledWith(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("expected the local branch to be deleted; calls: %v", fr.Calls)
	}
}

// TestKeyDeleteConfirm_NormalConfirm_NDeclinesWithoutDeleting is 3.9's
// negative companion: declining ('n'/'esc') the normal confirmation deletes
// nothing and returns to cleanupIdle.
func TestKeyDeleteConfirm_NormalConfirm_NDeclinesWithoutDeleting(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		t.Run(key, func(t *testing.T) {
			fr := execpkg.NewFakeRunner() // no canned responses: any git call fails loudly
			m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
			m.state = StateSucceeded
			m.cleanupPhase = cleanupConfirm
			m.plan = git.DeploymentPlan{PromotionBranch: "deploy/PROJ-1-to-UAT"}

			next, cmd := m.Update(keyPress(key))
			nm := next.(Model)
			if nm.cleanupPhase != cleanupIdle {
				t.Fatalf("%q should decline back to cleanupIdle, got %v", key, nm.cleanupPhase)
			}
			if cmd != nil {
				t.Errorf("%q must not fire a command", key)
			}
			if nm.pendingDeleteCurrent {
				t.Errorf("%q must not set pendingDeleteCurrent", key)
			}
			if len(fr.Calls) != 0 {
				t.Errorf("%q must not invoke any git call, got %v", key, fr.Calls)
			}
		})
	}
}

// --- 3.11: quitCmd deletes local (+ remote iff pushed) -----------------------

// TestQuitCmd_DeletesCurrentBranch_LocalAndRemoteIfPushed is task 3.11 (RED):
// once pendingDeleteCurrent + currentPushed are set, quitCmd deletes BOTH the
// local branch and its origin ref, after the restore checkout succeeds.
func TestQuitCmd_DeletesCurrentBranch_LocalAndRemoteIfPushed(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(branch)})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"checkout", "main", "--"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"push", "origin", "--delete", "--", branch}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.pendingDeleteCurrent = true
	m.currentPushed = true

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if !calledWith(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("expected local delete; calls: %v", fr.Calls)
	}
	if !calledWith(fr, "git", "push", "origin", "--delete", "--", branch) {
		t.Fatalf("a pushed branch should also delete the remote ref; calls: %v", fr.Calls)
	}
}

// TestQuitCmd_DeletesConfirmedBranch_EvenWhenRestoreSkipped is the M-1
// remediation (RED): a resumed run reaching a terminal state can have
// current==originalBranch — the deploy branch was never checked out this
// session (resumeInto reconstructs plan.PromotionBranch but leaves HEAD on the
// original branch). A CONFIRMED inline delete MUST still fire even though the
// restore checkout is skipped (original already equals current). Before M-1
// the delete was nested under `if Checkout()==nil`, so it was silently dropped
// whenever restore was skipped, leaking the branch with no feedback.
func TestQuitCmd_DeletesConfirmedBranch_EvenWhenRestoreSkipped(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	// HEAD already sits on the original branch: no deploy branch was checked out.
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("main")})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})
	// Deliberately NO checkout canned: a restore must NOT be attempted, and the
	// delete must NOT depend on the checkout having run.

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.pendingDeleteCurrent = true
	m.currentPushed = false

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if !calledWith(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("a confirmed delete must fire even when restore is skipped (current==original); calls: %v", fr.Calls)
	}
	if calledWith(fr, "git", "checkout", "main", "--") {
		t.Fatalf("no restore checkout should happen when current already equals original; calls: %v", fr.Calls)
	}
}

// TestQuitCmd_ConfirmedDelete_NoopWhenStillOnTargetBranch is M-1's negative
// companion: when the restore could NOT move HEAD off the deploy branch (e.g.
// the original branch is gone) so current still equals the delete target, the
// delete is a SAFE no-op — `git branch -D` of the checked-out branch is refused
// anyway, and we never force a checkout to an invalid branch.
func TestQuitCmd_ConfirmedDelete_NoopWhenStillOnTargetBranch(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	// Still standing on the deploy branch; the original branch no longer resolves.
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(branch)})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 1})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/main"}, execpkg.CommandResult{ExitCode: 1})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.pendingDeleteCurrent = true
	m.currentPushed = true

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if calledWith(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("must never -D the branch we are still standing on; calls: %v", fr.Calls)
	}
	if calledWith(fr, "git", "checkout", "main", "--") {
		t.Fatalf("must never checkout an original branch that no longer resolves; calls: %v", fr.Calls)
	}
}

// TestQuitCmd_DeletesLocalOnly_WhenUnpushed is task 3.11's companion: when
// currentPushed is false, quitCmd deletes ONLY the local branch — never
// attempting a remote delete against a ref that was never pushed.
func TestQuitCmd_DeletesLocalOnly_WhenUnpushed(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(branch)})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"checkout", "main", "--"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.pendingDeleteCurrent = true
	m.currentPushed = false

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if !calledWith(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("expected local delete; calls: %v", fr.Calls)
	}
	if calledWith(fr, "git", "push", "origin", "--delete", "--", branch) {
		t.Fatalf("an unpushed branch must never attempt a remote delete; calls: %v", fr.Calls)
	}
}

// TestQuitCmd_NoDeleteWhenPendingDeleteCurrentUnset is a regression guard:
// the pre-existing (Group 2) quit-restore behavior deletes nothing when
// pendingDeleteCurrent is left at its zero value (every quit site that never
// entered the delete-confirm flow).
func TestQuitCmd_NoDeleteWhenPendingDeleteCurrentUnset(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(branch)})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "main"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"checkout", "main", "--"}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.currentPushed = true // pushed, but delete was never confirmed

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if calledWith(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("an unconfirmed quit must never delete the branch; calls: %v", fr.Calls)
	}
}
