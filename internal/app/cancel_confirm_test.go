package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// cancelJobID is the current run's own jobId the cancel flow must target — and
// only this value must ever reach `sf project deploy cancel --job-id`, never the
// typed CANCELAR confirmation literal.
const cancelJobID = "0Af000000000042EAA"

// cancelArgs mirrors the exact `sf project deploy cancel` invocation cancelCmd
// composes, so the FakeRunner canned response matches AND so the test can assert
// jobId/alias are discrete args.
func cancelArgs(jobID, alias string) []string {
	return []string{
		"project", "deploy", "cancel",
		"--job-id", jobID,
		"--target-org", alias,
		"--json",
	}
}

// cancelPollingModel parks a Model on ValidationPolling with a seeded run dir
// (Status=InProgress on disk) and the run's own jobId, exactly as it would be
// right after ValidationStart succeeded — ready to enter the cancel flow. It
// returns the model, the FakeRunner (to assert what cancelCmd sent), the base
// dir, and the runID.
func cancelPollingModel(t *testing.T, cancel execpkg.CommandResult) (Model, *execpkg.FakeRunner, string, string) {
	t.Helper()
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	runID := "PROJ-1-to-UAT-cancel"
	if _, err := writer.Create(runs.Record{RunID: runID, JobID: cancelJobID, Status: "InProgress"}, []byte(`{"raw":"validate"}`)); err != nil {
		t.Fatalf("seeding run: %v", err)
	}

	fr := execpkg.NewFakeRunner()
	fr.When("sf", cancelArgs(cancelJobID, "UAT_SBX"), cancel)

	clk := &fakeClock{t: time.Unix(1000, 0)}
	deps := Deps{Dir: dir, Config: validationConfig(), SF: salesforce.New(fr), Runs: writer, Now: clk.now}
	m := New(deps)
	m.state = StateValidationPolling
	m.jobID = cancelJobID
	m.runID = runID
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", SandboxAlias: "UAT_SBX"}
	m.pollDeadline = clk.t.Add(time.Hour)
	m.report = salesforce.DeployReport{Status: "InProgress"}
	return m, fr, dir, runID
}

func cancelSuccess() execpkg.CommandResult {
	return execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"id":"` + cancelJobID + `","status":"Canceled"}}`)}
}

// --- entry: ValidationPolling --c--> StateCancelConfirm (q untouched) --------

// TestModel_ValidationPolling_CEntersCancelConfirm is the distinct-cancel-entry
// scenario (validation-progress delta): `c` opens the typed-confirm cancel flow.
func TestModel_ValidationPolling_CEntersCancelConfirm(t *testing.T) {
	m, _, _, _ := cancelPollingModel(t, cancelSuccess())

	next, cmd := m.Update(keyPress("c"))
	nm := next.(Model)
	if nm.State() != StateCancelConfirm {
		t.Fatalf("c should enter StateCancelConfirm, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("entering the confirm screen should not fire a command yet")
	}
	if nm.cancelInput != "" {
		t.Errorf("the cancel input should start empty, got %q", nm.cancelInput)
	}
}

// TestModel_ValidationPolling_QLeavesJobActiveAndDoesNotCancel proves the `q`
// invariant is UNCHANGED by the new cancel action: q exits without transitioning
// to the cancel flow and without invoking CancelDeploy (validation-progress
// spec: "exit and cancel remain distinct actions").
func TestModel_ValidationPolling_QLeavesJobActiveAndDoesNotCancel(t *testing.T) {
	m, fr, _, _ := cancelPollingModel(t, cancelSuccess())

	next, cmd := m.Update(keyPress("q"))
	nm := next.(Model)
	if nm.State() == StateCancelConfirm {
		t.Fatal("q must NOT enter the cancel flow")
	}
	if cmd == nil {
		t.Fatal("q should return a quit command")
	}
	for _, c := range fr.Calls {
		if len(c.Args) > 2 && c.Args[2] == "cancel" {
			t.Fatalf("q must NOT invoke sf project deploy cancel, but got call: %v", c.Args)
		}
	}
}

// --- typed CANCELAR gate: wrong / lowercase / backspace / esc ----------------

// TestModel_CancelConfirm_WrongTextDoesNotCancel is the mismatched-input
// scenario (validation-cancel spec): confirming with text != CANCELAR does not
// execute the cancel and leaves the run untouched.
func TestModel_CancelConfirm_WrongTextDoesNotCancel(t *testing.T) {
	m, fr, _, _ := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm
	m = typeString(m, "CANCEL") // incomplete

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateCancelConfirm {
		t.Fatalf("a wrong confirmation must keep the user on the confirm screen, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("a wrong confirmation must NOT fire the cancel command")
	}
	if nm.notice == "" {
		t.Error("a wrong confirmation should surface a notice")
	}
	if len(fr.Calls) != 0 {
		t.Errorf("no cancel call should have been made, got %v", fr.Calls)
	}
}

// TestModel_CancelConfirm_LowercaseDoesNotMatch proves the gate is exact and
// case-sensitive (design.md "no normalization").
func TestModel_CancelConfirm_LowercaseDoesNotMatch(t *testing.T) {
	m, _, _, _ := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm
	m = typeString(m, "cancelar")

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateCancelConfirm {
		t.Fatalf("lowercase 'cancelar' must not match the exact literal CANCELAR, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("a case-mismatched confirmation must NOT fire the cancel command")
	}
}

// TestModel_CancelConfirm_BackspaceEditsInput proves backspace edits the typed
// confirmation, and the corrected text then confirms.
func TestModel_CancelConfirm_BackspaceEditsInput(t *testing.T) {
	m, _, _, _ := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm
	m = typeString(m, "CANCELARX")

	next, _ := m.Update(keyPress("backspace"))
	nm := next.(Model)
	if nm.cancelInput != "CANCELAR" {
		t.Fatalf("backspace should drop the trailing rune, got %q", nm.cancelInput)
	}

	next2, cmd := nm.Update(keyPress("enter"))
	if next2.(Model).State() != StateCancelConfirm {
		t.Fatalf("correct confirmation stays on confirm until cancelDoneMsg lands, got %v", next2.(Model).State())
	}
	if cmd == nil {
		t.Fatal("the corrected CANCELAR should fire the cancel command")
	}
}

// TestModel_CancelConfirm_EscReturnsToPolling is the esc scenario: backing out
// returns to ValidationPolling and clears the typed input.
func TestModel_CancelConfirm_EscReturnsToPolling(t *testing.T) {
	m, fr, _, _ := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm
	m = typeString(m, "CANC")

	next, _ := m.Update(keyPress("esc"))
	nm := next.(Model)
	if nm.State() != StateValidationPolling {
		t.Fatalf("esc should return to ValidationPolling, got %v", nm.State())
	}
	if nm.cancelInput != "" {
		t.Errorf("esc should clear the typed input, got %q", nm.cancelInput)
	}
	if len(fr.Calls) != 0 {
		t.Errorf("esc must NOT invoke any cancel, got %v", fr.Calls)
	}
}

// --- correct confirmation fires cancelCmd for the run's OWN job only ---------

// TestModel_CancelConfirm_CorrectTextCancelsOwnJobOnly is the
// correct-confirmation scenario: typing CANCELAR fires cancelCmd, which targets
// the run's own jobId (m.jobID) — NEVER the typed literal or another user's job.
func TestModel_CancelConfirm_CorrectTextCancelsOwnJobOnly(t *testing.T) {
	m, fr, _, _ := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm
	m = typeString(m, "CANCELAR")

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateCancelConfirm {
		t.Fatalf("the confirm screen holds until cancelDoneMsg lands, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("a correct confirmation should fire the cancel command")
	}

	msg := run(t, cmd)
	cmsg, ok := msg.(cancelDoneMsg)
	if !ok {
		t.Fatalf("expected a cancelDoneMsg, got %T", msg)
	}
	if cmsg.err != nil {
		t.Fatalf("cancelCmd errored: %v", cmsg.err)
	}

	if len(fr.Calls) != 1 {
		t.Fatalf("expected exactly 1 cancel call, got %d: %v", len(fr.Calls), fr.Calls)
	}
	want := cancelArgs(cancelJobID, "UAT_SBX")
	got := fr.Calls[0].Args
	if len(got) != len(want) {
		t.Fatalf("expected args %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q (cancel must target the run's own jobId)", i, got[i], want[i])
		}
	}
	for _, a := range got {
		if a == "CANCELAR" {
			t.Fatal("the typed confirmation literal must NEVER be passed to sf as an argument")
		}
	}
}

// --- onCancelDone: success -> Canceled + persist ; failure -> stay, unmarked --

// TestModel_OnCancelDone_Success_MovesToCanceledAndPersists is the successful
// cancel scenario (validation-cancel spec): the run moves to Canceled and the
// cancel result is persisted (cancel.json + run.json Status=Canceled).
func TestModel_OnCancelDone_Success_MovesToCanceledAndPersists(t *testing.T) {
	m, _, dir, runID := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm

	raw := `{"status":0,"result":{"status":"Canceled"}}`
	next, cmd := m.Update(cancelDoneMsg{result: salesforce.CancelResult{Raw: raw}})
	nm := next.(Model)
	if nm.State() != StateCanceled {
		t.Fatalf("a successful cancel should move to StateCanceled, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("landing on the terminal Canceled screen should not fire another command")
	}
	if nm.cancelErr != nil {
		t.Errorf("a successful cancel should carry no error, got %v", nm.cancelErr)
	}

	runRoot := filepath.Join(dir, ".deploydeck", "runs", runID)
	cancelData, err := os.ReadFile(filepath.Join(runRoot, "cancel.json"))
	if err != nil {
		t.Fatalf("cancel.json should be persisted on success: %v", err)
	}
	if string(cancelData) != raw {
		t.Errorf("cancel.json should hold the raw cancel response verbatim, got %q", cancelData)
	}
	runData, err := os.ReadFile(filepath.Join(runRoot, "run.json"))
	if err != nil {
		t.Fatalf("run.json should exist: %v", err)
	}
	var rec runs.Record
	if err := json.Unmarshal(runData, &rec); err != nil {
		t.Fatalf("run.json parse: %v", err)
	}
	if rec.Status != "Canceled" {
		t.Errorf("run.json Status should be Canceled, got %q", rec.Status)
	}
}

// TestModel_OnCancelDone_Failure_StaysAndLeavesRunUnmarked is the failed-cancel
// scenario (validation-cancel spec): an error is shown and the run is NOT marked
// canceled.
func TestModel_OnCancelDone_Failure_StaysAndLeavesRunUnmarked(t *testing.T) {
	m, _, dir, runID := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm

	next, cmd := m.Update(cancelDoneMsg{err: errStub})
	nm := next.(Model)
	if nm.State() != StateCancelConfirm {
		t.Fatalf("a failed cancel must keep the user on the confirm screen, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("a failed cancel should not fire another command")
	}
	if nm.cancelErr == nil {
		t.Error("a failed cancel should record the error for display")
	}
	if !strings.Contains(nm.View(), "stub error") {
		t.Errorf("the confirm view should surface the cancel error, got:\n%s", nm.View())
	}

	runRoot := filepath.Join(dir, ".deploydeck", "runs", runID)
	if _, err := os.Stat(filepath.Join(runRoot, "cancel.json")); err == nil {
		t.Error("a failed cancel must NOT write cancel.json (run left untouched)")
	}
	runData, _ := os.ReadFile(filepath.Join(runRoot, "run.json"))
	var rec runs.Record
	_ = json.Unmarshal(runData, &rec)
	if rec.Status == "Canceled" {
		t.Error("a failed cancel must NOT mark the run Canceled")
	}
}

// --- stale-message guards ----------------------------------------------------

// TestModel_OnCancelDone_StaleGuard_IgnoredOutsideCancelConfirm proves a late or
// duplicate cancelDoneMsg arriving after we already left the confirm screen
// (e.g. already terminal Canceled) is dropped and cannot re-mark/clobber the
// run (symmetric to onReportDone's guard).
func TestModel_OnCancelDone_StaleGuard_IgnoredOutsideCancelConfirm(t *testing.T) {
	m, _, dir, runID := cancelPollingModel(t, cancelSuccess())
	m.state = StateCanceled // already terminal

	next, cmd := m.Update(cancelDoneMsg{result: salesforce.CancelResult{Raw: "late"}})
	nm := next.(Model)
	if nm.State() != StateCanceled {
		t.Fatalf("a late cancel must not change the terminal state, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("a dropped late cancel should not fire a command")
	}
	if _, err := os.Stat(filepath.Join(dir, ".deploydeck", "runs", runID, "cancel.json")); err == nil {
		t.Error("a late cancel must NOT persist a cancel.json over the terminal state")
	}
}

// TestModel_LateReportDoneMsg_AfterCancel_IsDropped proves a report poll landing
// after cancel success (now StateCanceled) is dropped by onReportDone's existing
// state != StateValidationPolling guard, so it can't clobber the terminal state.
func TestModel_LateReportDoneMsg_AfterCancel_IsDropped(t *testing.T) {
	m, _, _, _ := cancelPollingModel(t, cancelSuccess())
	m.state = StateCanceled // reached via a successful cancel

	next, cmd := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded", Raw: "late"}})
	nm := next.(Model)
	if nm.State() != StateCanceled {
		t.Fatalf("a late report after cancel must be dropped, state should stay Canceled, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("a dropped late report should not reschedule polling")
	}
}

// TestModel_ReportDoneMsg_DuringCancelConfirm_DoesNotFreezePolling is the MEDIUM
// liveness regression guard: a report that lands while the user is on
// StateCancelConfirm (the NORMAL case — the in-flight poll returns while the user
// is deciding whether to type CANCELAR) is still DROPPED by the stale-message
// guard, but it MUST clear pollInFlight — the report goroutine has genuinely
// returned. If it stayed stuck true, a later esc back to StateValidationPolling
// could never re-arm the loop (onPollTick no-ops while pollInFlight), permanently
// freezing live progress. This test drives the full path and asserts the loop
// actually resumes: the re-armed tick fires a fresh report instead of no-op'ing
// on a wedged in-flight guard.
func TestModel_ReportDoneMsg_DuringCancelConfirm_DoesNotFreezePolling(t *testing.T) {
	m, _, _, _ := cancelPollingModel(t, cancelSuccess())
	m.pollInFlight = true // a reportCmd is currently outstanding

	// Enter the cancel-confirm modal; the in-flight report is still outstanding.
	confirm, _ := m.Update(keyPress("c"))
	cm := confirm.(Model)
	if cm.State() != StateCancelConfirm {
		t.Fatalf("c should enter StateCancelConfirm, got %v", cm.State())
	}

	// The outstanding report lands on the confirm screen: dropped (state stays
	// CancelConfirm, no reschedule) BUT pollInFlight must be cleared.
	dropped, cmd := cm.Update(reportDoneMsg{report: salesforce.DeployReport{Status: "InProgress", Raw: `{"status":"InProgress"}`}})
	dm := dropped.(Model)
	if dm.State() != StateCancelConfirm {
		t.Fatalf("a report during cancel-confirm must be dropped (stay on confirm), got %v", dm.State())
	}
	if cmd != nil {
		t.Error("a dropped report must not reschedule polling from the confirm screen")
	}
	if dm.pollInFlight {
		t.Fatal("the returned report must clear pollInFlight even when dropped, or the poll loop wedges (liveness bug)")
	}

	// esc back to polling re-arms the poll loop.
	resumed, escCmd := dm.Update(keyPress("esc"))
	rm := resumed.(Model)
	if rm.State() != StateValidationPolling {
		t.Fatalf("esc should return to ValidationPolling, got %v", rm.State())
	}
	if escCmd == nil {
		t.Fatal("esc should re-arm the poll loop (pollTickCmd)")
	}

	// The scheduled tick must actually fire a fresh report — proving live
	// progress resumed and is not frozen on a stuck in-flight guard.
	ticked, tickCmd := rm.Update(pollTickMsg{})
	tm := ticked.(Model)
	if tickCmd == nil {
		t.Fatal("the resumed tick must fire the next report (frozen poll loop otherwise)")
	}
	if !tm.pollInFlight {
		t.Error("firing the resumed report should re-arm the in-flight guard")
	}
}

// TestModel_CancelConfirm_EscAfterInFlightReport_ManualRefreshWorks proves the
// manual-refresh promise also survives the drop: after a report lands during
// cancel-confirm and the user escs back to polling, pressing `r` fires a fresh
// report immediately. It would be a dead key if pollInFlight were left stuck true
// by the dropped report.
func TestModel_CancelConfirm_EscAfterInFlightReport_ManualRefreshWorks(t *testing.T) {
	m, _, _, _ := cancelPollingModel(t, cancelSuccess())
	m.pollInFlight = true

	confirm, _ := m.Update(keyPress("c"))
	dropped, _ := confirm.(Model).Update(reportDoneMsg{report: salesforce.DeployReport{Status: "InProgress", Raw: `{"status":"InProgress"}`}})
	resumed, _ := dropped.(Model).Update(keyPress("esc"))
	rm := resumed.(Model)

	next, cmd := rm.Update(keyPress("r"))
	nm := next.(Model)
	if cmd == nil {
		t.Fatal("manual refresh after esc should fire a report (dead key if pollInFlight is stuck true)")
	}
	if !nm.pollInFlight {
		t.Error("manual refresh should arm the in-flight guard")
	}
}

// --- view --------------------------------------------------------------------

// TestModel_CancelConfirm_ViewShowsJobAndTypedPrompt is task 5.6: the confirm
// screen shows the run's own job/org and echoes the typed prompt.
func TestModel_CancelConfirm_ViewShowsJobAndTypedPrompt(t *testing.T) {
	m, _, _, _ := cancelPollingModel(t, cancelSuccess())
	m.state = StateCancelConfirm
	m = typeString(m, "CAN")

	view := m.View()
	if !strings.Contains(view, cancelJobID) {
		t.Errorf("the confirm view should show the run's own jobId, got:\n%s", view)
	}
	if !strings.Contains(view, "UAT_SBX") {
		t.Errorf("the confirm view should show the target org, got:\n%s", view)
	}
	if !strings.Contains(view, "CANCELAR") {
		t.Errorf("the confirm view should show the CANCELAR prompt, got:\n%s", view)
	}
	if !strings.Contains(view, "CAN") {
		t.Errorf("the confirm view should echo what the user has typed so far, got:\n%s", view)
	}
}
