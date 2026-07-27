package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/config"
	"deploydeck/internal/delta"
	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// fakeClock is an injectable, mutable clock so ValidationPolling's deadline
// logic is testable without real time (design.md's "terminal timeout"
// decision: model pollDeadline from injected Deps.Now, never real
// time.Sleep). now() closes over the pointer, so advancing t is visible even
// across the value-copied Model that Bubble Tea's Update returns.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// validationConfig is a small valid config with fast poll settings for the
// deterministic (injected-clock) polling tests.
func validationConfig() config.Config {
	return config.Config{
		Branches:            map[string]string{"uat": "UAT"},
		Sandboxes:           map[string]config.SandboxConfig{"UAT": {Alias: "UAT_SBX", TestLevel: "RunLocalTests"}},
		TicketPatterns:      []string{"PROJ-[0-9]+"},
		BranchFormat:        config.DefaultBranchFormat,
		PollIntervalSeconds: 10,
		PollTimeoutSeconds:  3600,
	}
}

// reviewedModel returns a Model parked on PackageReview with the plan's delta
// artifacts already registered (as onDeltaDone would have done), ready to
// confirm into validation.
func reviewedModel(t *testing.T, deps Deps, empty bool) Model {
	t.Helper()
	m := New(deps)
	m.state = StatePackageReview
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SandboxAlias: "UAT_SBX",
		TestLevel:    "RunLocalTests",
	}
	m.plan = git.RegisterDeltaArtifacts(m.plan, "pkg/package.xml", "")
	m.summary = delta.PackageSummary{
		Types: []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 2}},
		Empty: empty,
	}
	if empty {
		m.summary = delta.PackageSummary{Empty: true}
	}
	return m
}

// --- 7.1 entry gate --------------------------------------------------------

// TestModel_PickVerification_ConfirmGatedByDeltaAllowed is task 7.1: a clean,
// non-aborted completion (DeltaAllowed) confirms into DeltaGeneration; a run
// that DeltaAllowed rejects (e.g. aborted) blocks entry and keeps the user on
// the verification screen.
func TestModel_PickVerification_ConfirmGatedByDeltaAllowed(t *testing.T) {
	t.Run("allowed confirm enters DeltaGeneration", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: validationConfig()})
		m.state = StatePickVerification
		m.repoState = git.RepoState{Clean: true, InProgress: false}

		next, cmd := m.Update(keyPress("enter"))
		nm := next.(Model)
		if nm.State() != StateDeltaGeneration {
			t.Fatalf("allowed confirm should enter DeltaGeneration, got %v", nm.State())
		}
		if cmd == nil {
			t.Fatal("entering DeltaGeneration should fire the delta command")
		}
	})

	t.Run("blocked run keeps user on verification", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: validationConfig()})
		m.state = StatePickVerification
		m.repoState = git.RepoState{Clean: true, InProgress: false}
		m.aborted = true // DeltaAllowed() -> false

		next, _ := m.Update(keyPress("enter"))
		nm := next.(Model)
		if nm.State() != StatePickVerification {
			t.Fatalf("a blocked run must not enter DeltaGeneration, got %v", nm.State())
		}
		if nm.notice == "" {
			t.Error("a blocked confirm should surface a notice")
		}
	})
}

// --- 7.3 delta generation transition ---------------------------------------

// TestModel_DeltaGeneration_Success_To_PackageReview is task 7.3 (success): a
// successful deltaDoneMsg registers the artifact paths on the plan and moves
// to PackageReview.
func TestModel_DeltaGeneration_Success_To_PackageReview(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: validationConfig()})
	m.state = StateDeltaGeneration
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	result := delta.Result{
		PackageXMLPath:         "/repo/.deploydeck/manifest/delta/PROJ-1-to-UAT/package/package.xml",
		DestructiveChangesPath: "/repo/.deploydeck/manifest/delta/PROJ-1-to-UAT/destructiveChanges/destructiveChanges.xml",
		Raw:                    "sgd ok",
	}
	summary := delta.PackageSummary{Types: []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 1}}}

	next, _ := m.Update(deltaDoneMsg{result: result, summary: summary})
	nm := next.(Model)

	if nm.State() != StatePackageReview {
		t.Fatalf("successful delta should reach PackageReview, got %v", nm.State())
	}
	if nm.Plan().PackageXMLPath != result.PackageXMLPath {
		t.Errorf("package.xml path not registered on plan: %q", nm.Plan().PackageXMLPath)
	}
	if nm.Plan().DestructiveChangesPath != result.DestructiveChangesPath {
		t.Errorf("destructiveChanges path not registered on plan: %q", nm.Plan().DestructiveChangesPath)
	}
}

// TestModel_DeltaGeneration_SgdFailure_KeepsUserAndBlocksValidation is task 7.3
// (failure): an sgd failure keeps the user on DeltaGeneration with the raw
// output shown and NEVER launches validation.
func TestModel_DeltaGeneration_SgdFailure_KeepsUserAndBlocksValidation(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: validationConfig()})
	m.state = StateDeltaGeneration

	next, cmd := m.Update(deltaDoneMsg{err: errStub})
	nm := next.(Model)

	if nm.State() != StateDeltaGeneration {
		t.Fatalf("sgd failure must keep the user on DeltaGeneration, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("sgd failure must not launch any command (no validation)")
	}
	if nm.deltaErr == nil {
		t.Error("sgd failure should record the error for display")
	}
	if !strings.Contains(nm.View(), "stub error") {
		t.Errorf("DeltaGeneration failure view should surface the raw sgd error, got:\n%s", nm.View())
	}
}

// --- 7.5 empty-package block + override ------------------------------------

// TestModel_PackageReview_EmptyBlocksUntilOverride is tasks 7.5/7.6: an empty
// package blocks confirm until an explicit override, then proceeds — since
// HU-009 (Phase 2), "proceeds" means entering the real QueueReview stop
// (queueCmd fires), not ValidationStart directly.
func TestModel_PackageReview_EmptyBlocksUntilOverride(t *testing.T) {
	deps := Deps{Dir: "/repo", Config: validationConfig(), SF: exec_sf(t), Runs: runs.NewWriter(t.TempDir()), Now: (&fakeClock{t: time.Unix(0, 0)}).now}

	// Empty package: confirm is blocked, no command fired.
	m := reviewedModel(t, deps, true)
	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StatePackageReview {
		t.Fatalf("empty package must block confirm, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("blocked empty package must not fire validation")
	}
	if nm.notice == "" {
		t.Error("empty-package block should surface a notice")
	}

	// Explicit override, then confirm -> QueueReview (HU-009's real stop).
	next2, _ := nm.Update(keyPress("o"))
	nm2 := next2.(Model)
	if !nm2.emptyConfirmed {
		t.Fatal("override key should set emptyConfirmed")
	}
	next3, cmd3 := nm2.Update(keyPress("enter"))
	nm3 := next3.(Model)
	if nm3.State() != StateQueueReview {
		t.Fatalf("override then confirm should reach QueueReview, got %v", nm3.State())
	}
	if cmd3 == nil {
		t.Error("entering QueueReview should fire queueCmd")
	}
}

// --- 7.7 QueueReview entry (superseded by HU-009: see queue_review_test.go) -
//
// QueueReview was originally an inert pass-through (tasks 7.7/7.8): confirm
// landed directly on ValidationStart with zero queue query. HU-009 (Phase 2)
// turns it into a real stop — confirm now enters StateQueueReview and fires
// queueCmd; ValidationStart is reached afterward via keyQueueReview's `enter`
// or the ErrQueuePermission auto-skip. That new behavior, plus the
// permission/generic-error branches and own-job highlight, is covered by
// queue_review_test.go's TestModel_PackageReview_Confirm_
// EntersQueueReviewAndFiresQueueCmd and its siblings.

// --- 7.9/7.10 validate + immediate persistence -----------------------------

// exec_sf builds a Salesforce client over a FakeRunner canned for the happy
// validate + report sequence used by the wiring tests.
func exec_sf(t *testing.T) salesforce.Client {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", "pkg/package.xml",
		"--target-org", "UAT_SBX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"id":"0Af000000000042EAA","done":false,"state":"Queued"}}`),
	})
	return salesforce.New(fr)
}

// TestModel_ValidationStart_PersistsRunOnJobId is tasks 7.9/7.10: the validate
// command routes through salesforce.ValidateDeploy, and the run is persisted
// to .deploydeck/runs/ IMMEDIATELY on jobId receipt, before polling starts.
func TestModel_ValidationStart_PersistsRunOnJobId(t *testing.T) {
	dir := t.TempDir()
	clk := &fakeClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	writer := runs.NewWriter(dir)

	deps := Deps{Dir: dir, Config: validationConfig(), SF: exec_sf(t), Runs: writer, Now: clk.now}
	m := reviewedModel(t, deps, false)
	m.plan = git.RegisterDeltaArtifacts(m.plan, "pkg/package.xml", "")

	// Run the real validate command (through the FakeRunner-backed client).
	msg := run(t, m.validateCmd())
	vmsg, ok := msg.(validateDoneMsg)
	if !ok {
		t.Fatalf("expected a validateDoneMsg, got %T", msg)
	}
	if vmsg.err != nil {
		t.Fatalf("validate command errored: %v", vmsg.err)
	}
	if vmsg.result.JobID != "0Af000000000042EAA" {
		t.Fatalf("jobId not captured: %q", vmsg.result.JobID)
	}

	// The run must already be persisted on disk at this point.
	runID := "PROJ-1-to-UAT-20260102030405"
	runJSON := filepath.Join(dir, ".deploydeck", "runs", runID, "run.json")
	data, err := os.ReadFile(runJSON)
	if err != nil {
		t.Fatalf("run.json should be persisted immediately on jobId: %v", err)
	}
	var rec runs.Record
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("run.json parse: %v", err)
	}
	if rec.JobID != "0Af000000000042EAA" {
		t.Errorf("persisted run.json JobID = %q", rec.JobID)
	}
	if rec.SchemaVersion != runs.SchemaVersion1 {
		t.Errorf("persisted schemaVersion = %d, want %d", rec.SchemaVersion, runs.SchemaVersion1)
	}
	if _, err := os.Stat(filepath.Join(dir, ".deploydeck", "runs", runID, "validate.json")); err != nil {
		t.Errorf("validate.json raw should be persisted: %v", err)
	}

	// Feeding the validateDoneMsg back moves to polling with the deadline set.
	next, cmd := m.Update(vmsg)
	nm := next.(Model)
	if nm.State() != StateValidationPolling {
		t.Fatalf("after jobId the flow should poll, got %v", nm.State())
	}
	if nm.jobID != "0Af000000000042EAA" {
		t.Errorf("jobID not held on the model: %q", nm.jobID)
	}
	if cmd == nil {
		t.Error("entering polling should fire the first report + tick")
	}
	wantDeadline := clk.t.Add(time.Duration(deps.Config.PollTimeoutSeconds) * time.Second)
	if !nm.pollDeadline.Equal(wantDeadline) {
		t.Errorf("pollDeadline = %v, want %v", nm.pollDeadline, wantDeadline)
	}
}

// TestModel_ValidationStart_CLIError_FlowStaysAlive is tasks 7.9/7.10 (error):
// a CLI validate error surfaces the message + raw and keeps the flow alive on
// ValidationStart — never a crash or a terminal error state.
func TestModel_ValidationStart_CLIError_FlowStaysAlive(t *testing.T) {
	dir := t.TempDir()
	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", "pkg/package.xml",
		"--target-org", "UAT_SBX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}, execpkg.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"InvalidManifest","message":"package.xml is malformed","exitCode":1}`),
	})
	deps := Deps{Dir: dir, Config: validationConfig(), SF: salesforce.New(fr), Runs: runs.NewWriter(dir), Now: (&fakeClock{t: time.Unix(0, 0)}).now}
	m := reviewedModel(t, deps, false)
	m.plan = git.RegisterDeltaArtifacts(m.plan, "pkg/package.xml", "")

	msg := run(t, m.validateCmd())
	vmsg := msg.(validateDoneMsg)
	if vmsg.err == nil {
		t.Fatal("a CLI validate error should surface an error")
	}

	next, cmd := m.Update(vmsg)
	nm := next.(Model)
	if nm.State() != StateValidationStart {
		t.Fatalf("a CLI error must keep the flow on ValidationStart (alive), got %v", nm.State())
	}
	if nm.State() == StateError {
		t.Fatal("a CLI validate error must not become a terminal error state")
	}
	if nm.validateErr == nil {
		t.Error("validate error should be recorded for display")
	}
	if cmd != nil {
		t.Error("a CLI validate error must not start polling")
	}
	if !strings.Contains(nm.View(), "malformed") {
		t.Errorf("ValidationStart error view should surface the decoded message, got:\n%s", nm.View())
	}
}

// --- 7.11 polling: timeout, transient retry, persist each report -----------

// pollingModel parks a Model in ValidationPolling with a jobId, an injected
// clock, and a live run directory, as it would be right after ValidationStart
// succeeded.
func pollingModel(t *testing.T, dir string, clk *fakeClock) Model {
	t.Helper()
	writer := runs.NewWriter(dir)
	runID := "PROJ-1-to-UAT-poll"
	if _, err := writer.Create(runs.Record{RunID: runID, JobID: "JOB1", Status: "Queued"}, []byte(`{"raw":"validate"}`)); err != nil {
		t.Fatalf("seeding run: %v", err)
	}
	deps := Deps{Dir: dir, Config: validationConfig(), Runs: writer, Now: clk.now}
	m := New(deps)
	m.state = StateValidationPolling
	m.jobID = "JOB1"
	m.runID = runID
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", SandboxAlias: "UAT_SBX"}
	m.pollDeadline = clk.t.Add(time.Duration(deps.Config.PollTimeoutSeconds) * time.Second)
	return m
}

// TestModel_ValidationPolling_HardTimeout is task 7.11 (timeout): once the
// injected clock advances past pollDeadline, the next tick ends polling in
// StateFailed (timeout) rather than looping forever.
func TestModel_ValidationPolling_HardTimeout(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	m := pollingModel(t, t.TempDir(), clk)

	// Before the deadline: a tick reschedules polling.
	next, cmd := m.Update(pollTickMsg{})
	if next.(Model).State() != StateValidationPolling {
		t.Fatalf("a tick within the deadline should keep polling, got %v", next.(Model).State())
	}
	if cmd == nil {
		t.Fatal("a tick within the deadline should re-arm polling")
	}

	// Advance past the hard timeout: the next tick fails the run.
	clk.advance(time.Duration(m.deps.Config.PollTimeoutSeconds+1) * time.Second)
	next2, cmd2 := m.Update(pollTickMsg{})
	nm2 := next2.(Model)
	if nm2.State() != StateFailed {
		t.Fatalf("past the deadline the run should time out into StateFailed, got %v", nm2.State())
	}
	if cmd2 != nil {
		t.Error("a timed-out run must stop polling (nil cmd)")
	}
	if !nm2.timedOut {
		t.Error("a timeout should be flagged so the view can explain it")
	}
	if !strings.Contains(nm2.View(), "timeout") && !strings.Contains(nm2.View(), "timed out") {
		t.Errorf("a timed-out failure view should mention the timeout, got:\n%s", nm2.View())
	}
}

// TestModel_ValidationPolling_TransientErrorRetries is task 7.11 (retry): a
// transient report error keeps the flow in ValidationPolling and — under the
// sequential-poll model — schedules the next poll itself (the loop is driven by
// report completion, not a free-running tick), so the retry runs within the
// deadline.
func TestModel_ValidationPolling_TransientErrorRetries(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	m := pollingModel(t, t.TempDir(), clk)
	m.pollInFlight = true

	next, cmd := m.Update(reportDoneMsg{err: errStub})
	nm := next.(Model)
	if nm.State() != StateValidationPolling {
		t.Fatalf("a transient error must not end polling, got %v", nm.State())
	}
	if cmd == nil {
		t.Error("a transient error (within the deadline) should schedule the next poll")
	}
	if nm.pollInFlight {
		t.Error("a transient error should clear the in-flight guard so the retry can run")
	}

	// The scheduled tick (within the deadline) fires the retry.
	next2, cmd2 := nm.Update(pollTickMsg{})
	if next2.(Model).State() != StateValidationPolling {
		t.Fatalf("retry tick should stay polling, got %v", next2.(Model).State())
	}
	if cmd2 == nil {
		t.Error("a within-deadline retry tick should re-arm the poll")
	}
}

// TestModel_ValidationPolling_PollsSequentially (H2) proves the poll loop never
// lets a slow report overlap with the next tick: at most ONE ReportDeploy is
// ever in flight. A tick (or a manual refresh) that arrives while a report is
// still running fires nothing; the next poll is scheduled only AFTER the
// current report returns — so ticks can never pile up into concurrent sf
// subprocesses hammering the org.
func TestModel_ValidationPolling_PollsSequentially(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}

	t.Run("an in-flight report blocks a concurrent tick and manual refresh", func(t *testing.T) {
		m := pollingModel(t, t.TempDir(), clk)
		m.pollInFlight = true // a reportCmd is currently executing

		next, cmd := m.Update(pollTickMsg{})
		if next.(Model).State() != StateValidationPolling {
			t.Fatalf("an in-flight tick should stay polling, got %v", next.(Model).State())
		}
		if cmd != nil {
			t.Error("a tick while a report is in flight must not fire a concurrent report")
		}

		next2, cmd2 := m.Update(keyPress("r"))
		if next2.(Model).State() != StateValidationPolling {
			t.Fatalf("an in-flight manual refresh should stay polling, got %v", next2.(Model).State())
		}
		if cmd2 != nil {
			t.Error("manual refresh while a report is in flight must not stack a second report")
		}
	})

	t.Run("the next poll is scheduled only after the prior report completes", func(t *testing.T) {
		m := pollingModel(t, t.TempDir(), clk)
		m.pollInFlight = true

		next, cmd := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: "InProgress", Raw: `{"status":"InProgress"}`}})
		nm := next.(Model)
		if nm.State() != StateValidationPolling {
			t.Fatalf("a non-terminal report should keep polling, got %v", nm.State())
		}
		if cmd == nil {
			t.Fatal("a completed non-terminal report should schedule the next poll")
		}
		if nm.pollInFlight {
			t.Error("a completed report should clear the in-flight guard")
		}

		// Guard cleared: a tick now fires exactly one next report and re-arms it.
		next2, cmd2 := nm.Update(pollTickMsg{})
		if cmd2 == nil {
			t.Error("a tick after the prior report completed should fire the next report")
		}
		if !next2.(Model).pollInFlight {
			t.Error("firing the next report should re-arm the in-flight guard")
		}
	})
}

// TestModel_ValidationPolling_PersistsRawOnTransientError (H3) proves an
// errored poll that still returned raw output persists it too — the spec saves
// EVERY raw report relevant to the run, not only the successful polls. (A
// non-parseable non-zero-exit report still carries Raw, per the salesforce fix.)
func TestModel_ValidationPolling_PersistsRawOnTransientError(t *testing.T) {
	dir := t.TempDir()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	m := pollingModel(t, dir, clk)
	m.pollInFlight = true

	_, _ = m.Update(reportDoneMsg{
		report: salesforce.DeployReport{Raw: `{"status":1,"message":"No job found"}`},
		err:    errStub,
	})

	runDir := filepath.Join(dir, ".deploydeck", "runs", "PROJ-1-to-UAT-poll")
	if _, err := os.Stat(filepath.Join(runDir, "report-001.json")); err != nil {
		t.Errorf("a transient error's raw output should still be persisted: %v", err)
	}
}

// TestModel_ValidationPolling_PersistsEachReport is task 7.11 (persist): every
// polled raw report is appended (never overwritten) via runs.AppendReport.
func TestModel_ValidationPolling_PersistsEachReport(t *testing.T) {
	dir := t.TempDir()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	m := pollingModel(t, dir, clk)

	m1, _ := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: "InProgress", Raw: `{"status":"InProgress"}`}})
	m2, _ := m1.(Model).Update(reportDoneMsg{report: salesforce.DeployReport{Status: "InProgress", Raw: `{"status":"InProgress","n":2}`}})
	_ = m2

	runDir := filepath.Join(dir, ".deploydeck", "runs", "PROJ-1-to-UAT-poll")
	for _, name := range []string{"report-001.json", "report-002.json"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Errorf("expected %s to be persisted: %v", name, err)
		}
	}
}

// --- 7.13 terminal mapping -------------------------------------------------

// TestModel_ValidationPolling_TerminalMapping is tasks 7.13/7.14: terminal
// statuses map to the right terminal state and stop polling.
func TestModel_ValidationPolling_TerminalMapping(t *testing.T) {
	tests := []struct {
		status string
		want   State
	}{
		{"Succeeded", StateSucceeded},
		{"SucceededPartial", StateSucceeded},
		{"Failed", StateFailed},
		{"Canceled", StateCanceled},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			clk := &fakeClock{t: time.Unix(1000, 0)}
			m := pollingModel(t, t.TempDir(), clk)

			next, _ := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: tt.status, Raw: "{}"}})
			nm := next.(Model)
			if nm.State() != tt.want {
				t.Fatalf("status %q should map to %v, got %v", tt.status, tt.want, nm.State())
			}

			// A tick after terminal must not resume polling.
			_, cmd := nm.Update(pollTickMsg{})
			if cmd != nil {
				t.Errorf("polling must stop once terminal (%q), got a non-nil cmd", tt.status)
			}
		})
	}
}

// --- 7.15 user exit leaves the job active ----------------------------------

// TestModel_ValidationPolling_UserExitLeavesJobActive is task 7.15: exiting
// the progress screen quits without issuing any cancel/abort, leaving the
// Salesforce job active and the persisted run resumable on disk.
func TestModel_ValidationPolling_UserExitLeavesJobActive(t *testing.T) {
	dir := t.TempDir()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	m := pollingModel(t, dir, clk)

	next, cmd := m.Update(keyPress("q"))
	if next.(Model).State() != StateValidationPolling {
		t.Errorf("quitting should not change the run state (job stays active), got %v", next.(Model).State())
	}
	if cmd == nil {
		t.Fatal("q should quit the program")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q should return tea.Quit, got %T", cmd())
	}

	// The run remains on disk (resumable): no cancel wiped it.
	if _, err := os.Stat(filepath.Join(dir, ".deploydeck", "runs", "PROJ-1-to-UAT-poll", "run.json")); err != nil {
		t.Errorf("run should stay persisted (resumable) after exit: %v", err)
	}
}

// TestModel_ValidationPolling_UserExitCancelsInFlightReport (H3) proves user
// exit cancels the polling session's context so an in-flight `sf project deploy
// report` subprocess is torn down — WITHOUT touching the Salesforce job (report
// is read-only; no `deploy cancel` is ever issued and the run stays resumable).
func TestModel_ValidationPolling_UserExitCancelsInFlightReport(t *testing.T) {
	dir := t.TempDir()
	clk := &fakeClock{t: time.Unix(1000, 0)}

	// Enter ValidationPolling exactly as ValidationStart success does, so the
	// cancelable poll context is armed by real code, not the test.
	m := pollingModel(t, dir, clk)
	entered, _ := m.Update(validateDoneMsg{result: salesforce.ValidateResult{JobID: "JOB1"}, runID: m.runID})
	pm := entered.(Model)
	if pm.State() != StateValidationPolling {
		t.Fatalf("expected ValidationPolling, got %v", pm.State())
	}
	if pm.pollCtx == nil {
		t.Fatal("entering polling should arm a cancelable poll context")
	}

	next, cmd := pm.Update(keyPress("q"))
	if cmd == nil {
		t.Fatal("q should quit the program")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q should return tea.Quit, got %T", cmd())
	}
	if err := pm.pollCtx.Err(); err != context.Canceled {
		t.Errorf("user exit should cancel the in-flight poll context, got %v", err)
	}
	_ = next

	// The run stays persisted (resumable): cancelling the local read-only query
	// never wiped the run and never cancelled the Salesforce job.
	if _, err := os.Stat(filepath.Join(dir, ".deploydeck", "runs", "PROJ-1-to-UAT-poll", "run.json")); err != nil {
		t.Errorf("run should stay persisted (resumable) after exit: %v", err)
	}
}

// --- deltaCmd composition (routes through the delta + git services) --------

// TestModel_DeltaCmd_ComposesGenerateAndSummarize proves deltaCmd routes
// through delta.Service.Generate (arg composition) and delta.Summarize,
// returning a deltaDoneMsg — without any real sgd (FakeRunner) — matching the
// design's "app never execs directly" invariant.
func TestModel_DeltaCmd_ComposesGenerateAndSummarize(t *testing.T) {
	dir := t.TempDir()
	cfg := validationConfig()
	cfg.Delta = config.DeltaConfig{SourceDirs: []string{"force-app"}}

	outputDir := filepath.Join(dir, config.DefaultDeltaOutputDir, "PROJ-1-to-UAT")
	if err := os.MkdirAll(filepath.Join(outputDir, "package"), 0o755); err != nil {
		t.Fatal(err)
	}
	pkgXML := `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
  <types><members>AccountService</members><name>ApexClass</name></types>
  <version>59.0</version>
</Package>`
	if err := os.WriteFile(filepath.Join(outputDir, "package", "package.xml"), []byte(pkgXML), 0o644); err != nil {
		t.Fatal(err)
	}

	sgdRunner := execpkg.NewFakeRunner()
	sgdRunner.When("sf", []string{
		"sgd", "source", "delta",
		"--from", "origin/UAT",
		"--to", "HEAD",
		"--output-dir", outputDir,
		"--generate-delta",
		"--source-dir", "force-app",
	}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sgd done")})

	deps := Deps{
		Dir:    dir,
		Config: cfg,
		Delta:  delta.New(sgdRunner),
		Git:    git.New(execpkg.NewFakeRunner()), // ChangedFiles best-effort: no When -> degrades to no outside-dirs
	}
	m := New(deps)
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	msg := run(t, m.deltaCmd())
	dmsg, ok := msg.(deltaDoneMsg)
	if !ok {
		t.Fatalf("expected deltaDoneMsg, got %T", msg)
	}
	if dmsg.err != nil {
		t.Fatalf("deltaCmd errored: %v", dmsg.err)
	}
	if len(dmsg.summary.Types) != 1 || dmsg.summary.Types[0].Name != "ApexClass" || dmsg.summary.Types[0].Count != 1 {
		t.Fatalf("summary not composed from the parsed package.xml: %+v", dmsg.summary.Types)
	}
	if dmsg.result.PackageXMLPath == "" {
		t.Error("delta result should carry the package.xml path")
	}
}

// --- view coverage for the live-progress screens ---------------------------

// TestModel_View_RendersValidationScreens asserts the live-progress and
// result screens surface their load-bearing HU-011 content: component/test
// counts, metadata errors (component/type/message), and failed tests
// (class/method/message).
func TestModel_View_RendersValidationScreens(t *testing.T) {
	// Live polling with progress.
	poll := New(Deps{Dir: "/repo", Config: validationConfig()})
	poll.state = StateValidationPolling
	poll.jobID = "0Af000000000042EAA"
	poll.report = salesforce.DeployReport{
		Status:                   "InProgress",
		NumberComponentsTotal:    10,
		NumberComponentsDeployed: 4,
		NumberTestsTotal:         6,
		NumberTestsCompleted:     2,
	}
	pv := poll.View()
	for _, want := range []string{"0Af000000000042EAA", "InProgress", "4", "10", "2", "6"} {
		if !strings.Contains(pv, want) {
			t.Errorf("polling view missing %q\n%s", want, pv)
		}
	}

	// Terminal failure with a metadata error and a failed test.
	fail := New(Deps{Dir: "/repo", Config: validationConfig()})
	fail.state = StateFailed
	fail.report = salesforce.DeployReport{
		Status:                "Failed",
		NumberComponentErrors: 1,
		NumberTestErrors:      1,
		ComponentFailures:     []salesforce.ComponentFailure{{Component: "MyClass", Type: "ApexClass", Message: "Compile error: unexpected token"}},
		TestFailures:          []salesforce.TestFailure{{Class: "MyClassTest", Method: "testSomething", Message: "System.AssertException"}},
	}
	fv := fail.View()
	for _, want := range []string{"MyClass", "ApexClass", "Compile error: unexpected token", "MyClassTest", "testSomething", "System.AssertException"} {
		if !strings.Contains(fv, want) {
			t.Errorf("failure view missing %q\n%s", want, fv)
		}
	}

	// Package review with a sensitive-type + outside-source-dir warning.
	rev := New(Deps{Dir: "/repo", Config: validationConfig()})
	rev.state = StatePackageReview
	rev.summary = delta.PackageSummary{
		Types:             []delta.MetadataTypeSummary{{Name: "Flow", Count: 1}},
		SensitiveTypes:    []string{"Flow"},
		OutsideSourceDirs: []string{"docs/README.md"},
	}
	rv := rev.View()
	for _, want := range []string{"Flow", "docs/README.md"} {
		if !strings.Contains(rv, want) {
			t.Errorf("package review view missing %q\n%s", want, rv)
		}
	}
}
