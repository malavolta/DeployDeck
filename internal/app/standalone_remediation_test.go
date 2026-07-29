package app

import (
	"strings"
	"testing"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// TestOnValidateDone_ErrorPreservesPreCreatedRunID is the adversarial-review
// remediation for Finding 2 (MED): onValidateDone must NOT clobber a
// pre-created Mode="validate" run's id on a validate error. A validate error
// carries runID=="", and unconditionally assigning it would orphan the
// original run and push a retry down validateCmd's fallback branch (empty
// Ticket/Target, no Mode/ManifestPath), creating a malformed second run.
func TestOnValidateDone_ErrorPreservesPreCreatedRunID(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: standaloneValidateConfig()})
	m.state = StateValidationStart
	m.runID = "validate-xyz"

	// A validate error carries runID=="".
	next, _ := m.Update(validateDoneMsg{err: errStub})
	nm := next.(Model)

	if nm.runID != "validate-xyz" {
		t.Errorf("a validate error must not clobber the pre-created runID, got %q", nm.runID)
	}
	if nm.State() != StateValidationStart {
		t.Errorf("a validate error should keep the flow on StateValidationStart, got %v", nm.State())
	}
	if nm.validateErr == nil {
		t.Error("the validate error should be recorded")
	}

	// A success WITH a runID still sets it (regression guard for the guard).
	next2, _ := nm.Update(validateDoneMsg{runID: "run-success", result: salesforce.ValidateResult{JobID: "0Af000000000099EAA"}})
	nm2 := next2.(Model)
	if nm2.runID != "run-success" {
		t.Errorf("a successful validateDoneMsg with a runID should still set m.runID, got %q", nm2.runID)
	}
}

// TestKeySucceeded_StandaloneValidate_NoPushOrDeleteWithoutPromotionBranch is
// the adversarial-review remediation for Finding 3 (MED): a standalone
// validate reaches StateSucceeded with an EMPTY PromotionBranch, so the
// success screen must NOT offer `p` (push) or `d` (delete) — pushing/deleting
// an empty branch runs `git push -u origin ""` / deletes "" (nonsensical). The
// footer must not advertise push either. q/enter/esc still quit.
func TestKeySucceeded_StandaloneValidate_NoPushOrDeleteWithoutPromotionBranch(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: standaloneValidateConfig()})
	m.state = StateSucceeded
	m.standaloneMode = "validate"
	// A standalone validate never created a promotion branch.
	m.plan = git.DeploymentPlan{PackageXMLPath: "pkg/package.xml", SandboxAlias: "UAT_SBX", TestLevel: "RunLocalTests"}

	// `p` must NOT enter push preparation.
	nextP, cmdP := m.Update(keyPress("p"))
	nmP := nextP.(Model)
	if nmP.State() == StatePushPreparation {
		t.Error("standalone validate success must not offer push with an empty PromotionBranch")
	}
	if nmP.State() != StateSucceeded {
		t.Errorf("`p` on a promotion-less success should be a no-op, got state %v", nmP.State())
	}
	if cmdP != nil {
		t.Error("`p` on a promotion-less success must not fire a command")
	}

	// `d` must NOT begin a delete confirmation / fire the unpushed-count gate.
	nextD, cmdD := m.Update(keyPress("d"))
	nmD := nextD.(Model)
	if cmdD != nil {
		t.Error("`d` on a promotion-less success must not start a delete (no unpushedCountCmd)")
	}
	if nmD.cleanupPhase != cleanupIdle {
		t.Error("`d` must not begin a delete confirmation on a promotion-less success")
	}

	// The footer must not advertise push.
	if strings.Contains(m.View(), "preparar push") {
		t.Errorf("standalone validate success footer must not advertise push:\n%s", m.View())
	}

	// q/enter/esc still quit.
	if _, cmdQ := m.Update(keyPress("q")); cmdQ == nil {
		t.Error("q should still quit on a promotion-less success")
	}
	if _, cmdEsc := m.Update(keyPress("esc")); cmdEsc == nil {
		t.Error("esc should still quit on a promotion-less success")
	}
}

// TestKeySucceeded_FullFlow_PushAndDeleteStillWork is Finding 3's regression
// guard: a full-flow success with a REAL PromotionBranch must keep offering
// both `p` (push) and `d` (delete), and the footer must still advertise push —
// proving the promotion-branch gate never touches the full-flow deliverable.
func TestKeySucceeded_FullFlow_PushAndDeleteStillWork(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: validationConfig(), Git: git.New(execpkg.NewFakeRunner())})
	m.state = StateSucceeded
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "PROJ-1-to-UAT"}

	if nextP, _ := m.Update(keyPress("p")); nextP.(Model).State() != StatePushPreparation {
		t.Errorf("full-flow success must still offer push with a real promotion branch, got %v", nextP.(Model).State())
	}

	if _, cmdD := m.Update(keyPress("d")); cmdD == nil {
		t.Error("full-flow success `d` must still fire the unpushed-count gate")
	}

	if !strings.Contains(m.View(), "preparar push") {
		t.Errorf("full-flow success footer must still advertise push:\n%s", m.View())
	}
}

// TestKeyPrereq_Continue_IsSymmetricWithOnPrereqDone is the adversarial-review
// remediation for Finding 4 (LOW, latent HU-017 trap): keyPrereq's `c`-continue
// lands StateMainMenu but returned nil, while onPrereqDone returns
// tea.Batch(resumeDetectCmd(), originalBranchCmd()). Without those, the
// `c`-continue landing captures no original branch (HU-017 restore) and runs no
// HU-013 resume-detection. The `c` branch must return the same batch as
// onPrereqDone.
func TestKeyPrereq_Continue_IsSymmetricWithOnPrereqDone(t *testing.T) {
	// A Git-backed Deps makes originalBranchCmd() (and resumeDetectCmd(), given
	// Runs) non-nil, so the batch is observably non-nil — the exact shape
	// onPrereqDone produces.
	fr := execpkg.NewFakeRunner()
	deps := Deps{Dir: "/repo", Git: git.New(fr), Runs: runs.NewWriter(t.TempDir())}

	m := New(deps)
	m.state = StatePrereqCheck
	m.checks = []prereq.PrereqCheck{{Name: "alias", Status: prereq.StatusWarning}} // warnings do NOT block `c`

	next, cmd := m.Update(keyPress("c"))
	nm := next.(Model)
	if nm.State() != StateMainMenu {
		t.Fatalf("`c` past warnings should land on the main menu, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("`c`-continue must fire resume-detect + original-branch capture (symmetric with onPrereqDone), got nil")
	}

	// Symmetry sanity: onPrereqDone for the same deps also returns a non-nil
	// batch, so the `c` landing is now the same shape as the other post-prereq
	// landing site.
	if _, doneCmd := New(deps).Update(prereqDoneMsg{checks: m.checks}); doneCmd == nil {
		t.Fatal("onPrereqDone should return a non-nil batch for these deps (symmetry baseline)")
	}
}
