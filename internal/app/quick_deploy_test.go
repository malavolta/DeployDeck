package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deploydeck/internal/config"
	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// --- 4.1/4.2: onBranchCreated threads plan.TestLevel onto the new run.Record --

// TestOnBranchCreated_PersistsPlanTestLevel is task 4.1 (RED): a run created
// from a plan carrying a TestLevel persists that value on the new run's
// record at creation time (run-persistence spec: "TestLevel persisted at run
// creation"), mirroring TestOnBranchCreated_RePromoteRun_PersistsSourceRunID.
func TestOnBranchCreated_PersistsPlanTestLevel(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	deps := Deps{Dir: dir, Config: rePromoteEnvConfig(), Runs: writer, Now: func() time.Time { return now }}
	m := New(deps)
	m.state = StateBranchCreation
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SandboxAlias: "UAT_SBX",
		TestLevel:    "RunLocalTests",
		SelectedCommits: []git.DiscoveredCommit{
			discovered("newA", "newA", "PROJ-1: add A", false),
		},
	}

	next, _ := m.Update(branchCreatedMsg{})
	nm := next.(Model)
	if nm.State() != StateCherryPicking {
		t.Fatalf("expected CherryPicking, got %v (err=%v)", nm.State(), nm.Err())
	}

	gotRec, err := writer.Load(nm.runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRec.TestLevel != "RunLocalTests" {
		t.Errorf("TestLevel = %q, want RunLocalTests", gotRec.TestLevel)
	}
}

// TestOnBranchCreated_NormalRun_TestLevelRoundTrips is the triangulation case
// (a second, different TestLevel value) proving the thread is a genuine
// pass-through of m.plan.TestLevel, not a hardcoded string.
func TestOnBranchCreated_NormalRun_TestLevelRoundTrips(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	deps := Deps{Dir: dir, Config: rePromoteEnvConfig(), Runs: writer, Now: func() time.Time { return now }}
	m := New(deps)
	m.state = StateBranchCreation
	m.branchName = "deploy/PROJ-2-to-INT"
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-2",
		TargetBranch: "INT",
		SandboxAlias: "INT_SBX",
		TestLevel:    "RunAllTestsInOrg",
		SelectedCommits: []git.DiscoveredCommit{
			discovered("newB", "newB", "PROJ-2: add B", false),
		},
	}

	next, _ := m.Update(branchCreatedMsg{})
	nm := next.(Model)

	gotRec, err := writer.Load(nm.runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRec.TestLevel != "RunAllTestsInOrg" {
		t.Errorf("TestLevel = %q, want RunAllTestsInOrg", gotRec.TestLevel)
	}
}

// --- 4.3/4.4/4.5: the `x` key gates entry into StateQuickDeploy on eligibility --

// TestKeyRunHistory_X_OnEligibleRow_OpensStateQuickDeploy is task 4.3 (RED):
// x on a row that is quick-deploy-eligible (per runs.QuickDeployEligible)
// opens StateQuickDeploy with the confirm buffer reset (run-history spec:
// "Quick-deploy action on an eligible row opens the view").
func TestKeyRunHistory_X_OnEligibleRow_OpensStateQuickDeploy(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)}
	m := New(Deps{Dir: "/repo", Config: validationConfig(), Now: clk.now})
	m.state = StateRunHistory
	m.runs = []runs.Record{
		{RunID: "eligible", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfELIGIBLE", Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-24 * time.Hour)},
	}
	m.runsCursor = 0

	next, cmd := m.Update(keyPress("x"))
	nm := next.(Model)
	if nm.State() != StateQuickDeploy {
		t.Fatalf("x on an eligible row should open StateQuickDeploy, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("opening the quick-deploy view should not fire a command yet")
	}
	if nm.quickConfirm != "" {
		t.Errorf("quickConfirm should start empty, got %q", nm.quickConfirm)
	}
	if nm.quickErr != nil {
		t.Errorf("quickErr should start nil, got %v", nm.quickErr)
	}
}

// TestKeyRunHistory_X_OnIneligibleRow_IsNoOp is task 4.4 (RED): x on a row
// that fails ANY of runs.QuickDeployEligible's predicates is a strict no-op —
// the history view stays put (run-history spec: "Quick-deploy action on a
// non-eligible row has no effect").
func TestKeyRunHistory_X_OnIneligibleRow_IsNoOp(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)}
	cases := []struct {
		name string
		rec  runs.Record
	}{
		{"older than 10 days", runs.Record{RunID: "old", Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-11 * 24 * time.Hour)}},
		{"required tests not run", runs.Record{RunID: "notests", Status: "Succeeded", TestLevel: "RunSpecifiedTests", CreatedAt: clk.t.Add(-time.Hour)}},
		{"not a successful validation", runs.Record{RunID: "failed", Status: "Failed", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-time.Hour)}},
		{"already quick-deployed", runs.Record{RunID: "already", Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-time.Hour), QuickDeployedAt: clk.t.Add(-time.Minute)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: validationConfig(), Now: clk.now})
			m.state = StateRunHistory
			m.runs = []runs.Record{tc.rec}
			m.runsCursor = 0

			next, cmd := m.Update(keyPress("x"))
			nm := next.(Model)
			if nm.State() != StateRunHistory {
				t.Fatalf("x on an ineligible row (%s) should be a no-op, got %v", tc.name, nm.State())
			}
			if cmd != nil {
				t.Errorf("x on an ineligible row (%s) must fire no command", tc.name)
			}
		})
	}
}

// TestKeyRunHistory_EnterDAndR_RemainUnaffectedByX is task 4.5 (RED), a
// regression guard: the existing Enter (resume), d (detail toggle), and r
// (re-promote) actions are unaffected by the new x branch (run-history spec:
// "Enter, d, and r remain unaffected").
func TestKeyRunHistory_EnterDAndR_RemainUnaffectedByX(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	records := []runs.Record{
		{RunID: "run-1", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "JOB1", Status: "InProgress", Phase: "validating"},
	}

	m := New(Deps{Dir: "/repo", Config: validationConfig(), SF: reportSF(t, "JOB1", "UAT_SBX", "InProgress"), Now: clk.now})
	m.state = StateRunHistory
	m.runs = records
	m.runsCursor = 0
	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateValidationPolling {
		t.Fatalf("Enter should still resume a non-terminal jobId run unaffected by x, got %v", nm.State())
	}
	if cmd == nil {
		t.Error("resuming should fire a command")
	}

	m2 := New(Deps{Dir: "/repo", Config: validationConfig()})
	m2.state = StateRunHistory
	m2.runs = records
	toggled := advance(t, m2, keyPress("d"))
	if !toggled.runDetail {
		t.Fatal("d should still toggle the expanded detail, unaffected by x")
	}

	rp := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
	rp.state = StateRunHistory
	rp.runs = []runs.Record{{RunID: "prior", Ticket: "PROJ-1", Target: "INT", Status: "Succeeded", Commits: []string{"shaA"}}}
	rp.runsCursor = 0
	rNext, rCmd := rp.Update(keyPress("r"))
	if rNext.(Model).State() != StateCommitDiscovery {
		t.Fatalf("r should still start re-promote unaffected by x, got %v", rNext.(Model).State())
	}
	if rCmd == nil {
		t.Error("r should still fire the remap command")
	}
}

// --- 4.7/4.8: the pure execution gate -----------------------------------

// TestQuickDeployExecAllowed_TableDriven is task 4.7 (RED): execution is
// allowed only when AllowExecution is true AND (the target is non-production
// OR AllowProduction is true) — quick-deploy spec: "Production Target
// Blocked Without Explicit Configuration" + "Suggest-Only By Default".
func TestQuickDeployExecAllowed_TableDriven(t *testing.T) {
	cases := []struct {
		name   string
		cfg    config.Config
		isProd bool
		want   bool
	}{
		{
			name:   "AllowExecution false blocks regardless of prod/AllowProduction",
			cfg:    config.Config{QuickDeploy: config.QuickDeployConfig{AllowExecution: false, AllowProduction: true}},
			isProd: true,
			want:   false,
		},
		{
			name:   "AllowExecution true, non-prod, allows",
			cfg:    config.Config{QuickDeploy: config.QuickDeployConfig{AllowExecution: true}},
			isProd: false,
			want:   true,
		},
		{
			name:   "AllowExecution true, prod, AllowProduction false blocks",
			cfg:    config.Config{QuickDeploy: config.QuickDeployConfig{AllowExecution: true, AllowProduction: false}},
			isProd: true,
			want:   false,
		},
		{
			name:   "AllowExecution true, prod, AllowProduction true allows",
			cfg:    config.Config{QuickDeploy: config.QuickDeployConfig{AllowExecution: true, AllowProduction: true}},
			isProd: true,
			want:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := quickDeployExecAllowed(tc.cfg, tc.isProd); got != tc.want {
				t.Errorf("quickDeployExecAllowed(%+v, isProd=%v) = %v, want %v", tc.cfg.QuickDeploy, tc.isProd, got, tc.want)
			}
		})
	}
}

// --- 4.9-4.15: keyQuickDeploy's execution path -----------------------------

// quickDeployConfig builds a small valid config (mirrors validationConfig)
// with a single non-production "uat" destination and the given QuickDeploy
// gate settings.
func quickDeployConfig(allow, allowProd bool) config.Config {
	return config.Config{
		Branches:       map[string]string{"uat": "UAT"},
		Sandboxes:      map[string]config.SandboxConfig{"UAT": {Alias: "UAT_SBX", TestLevel: "RunLocalTests"}},
		TicketPatterns: []string{"PROJ-[0-9]+"},
		BranchFormat:   config.DefaultBranchFormat,
		QuickDeploy:    config.QuickDeployConfig{AllowExecution: allow, AllowProduction: allowProd},
	}
}

// quickDeployProdConfig mirrors quickDeployConfig but the seeded run's
// target ("main") is the literal production branch (git.IsProductionTarget
// fail-closed arm), independent of any configured "production" Branches key.
func quickDeployProdConfig(allow, allowProd bool) config.Config {
	return config.Config{
		Branches:       map[string]string{"uat": "UAT"},
		Sandboxes:      map[string]config.SandboxConfig{"main": {Alias: "PROD_SBX", TestLevel: "RunLocalTests"}},
		TicketPatterns: []string{"PROJ-[0-9]+"},
		BranchFormat:   config.DefaultBranchFormat,
		QuickDeploy:    config.QuickDeployConfig{AllowExecution: allow, AllowProduction: allowProd},
	}
}

// quickDeployArgs mirrors the exact `sf project deploy quick` invocation
// quickDeployCmd composes, so the FakeRunner canned response matches AND the
// test can assert jobId/alias are discrete args.
func quickDeployArgs(jobID, alias string) []string {
	return []string{
		"project", "deploy", "quick",
		"--job-id", jobID,
		"--target-org", alias,
		"--json",
	}
}

func quickDeploySuccess(jobID string) execpkg.CommandResult {
	return execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"id":"` + jobID + `","status":"Succeeded"}}`)}
}

// quickDeployModel parks a Model on StateQuickDeploy (via a real `x` press)
// on a seeded, quick-deploy-eligible run persisted under a temp runs dir
// (ready for MarkQuickDeployed to write), with a FakeRunner primed to answer
// that run's own JobID/Alias quick-deploy call. Returns the model, the
// FakeRunner (to assert what quickDeployCmd sent, or that it sent nothing),
// the base dir, and the runID.
func quickDeployModel(t *testing.T, cfg config.Config, target, alias, jobID string) (Model, *execpkg.FakeRunner, string, string) {
	t.Helper()
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	runID := "PROJ-1-to-" + target + "-quick"
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	rec := runs.Record{
		RunID: runID, Ticket: "PROJ-1", Target: target, Alias: alias, JobID: jobID,
		Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: now.Add(-24 * time.Hour),
	}
	if _, err := writer.Create(rec, []byte(`{"raw":"validate"}`)); err != nil {
		t.Fatalf("seeding run: %v", err)
	}

	fr := execpkg.NewFakeRunner()
	fr.When("sf", quickDeployArgs(jobID, alias), quickDeploySuccess(jobID))

	deps := Deps{Dir: dir, Config: cfg, SF: salesforce.New(fr), Runs: writer, Now: func() time.Time { return now }}
	m := New(deps)
	m.state = StateRunHistory
	m.runs = []runs.Record{rec}
	m.runsCursor = 0

	next, _ := m.Update(keyPress("x"))
	nm := next.(Model)
	if nm.State() != StateQuickDeploy {
		t.Fatalf("setup: x should open StateQuickDeploy on an eligible seeded run, got %v", nm.State())
	}
	return nm, fr, dir, runID
}

// TestKeyQuickDeploy_Enter_WrongOrMissingConfirm_DoesNotFireExec is task 4.9
// (RED): the gate is otherwise permitted (AllowExecution true, non-prod
// target), but the typed confirmation is empty/wrong — enter must never
// call QuickDeploy and must stay on StateQuickDeploy (quick-deploy spec:
// "Missing confirmation blocks execution").
func TestKeyQuickDeploy_Enter_WrongOrMissingConfirm_DoesNotFireExec(t *testing.T) {
	for _, confirm := range []string{"", "desplegar", "DESPLEGA"} {
		t.Run("confirm="+confirm, func(t *testing.T) {
			m, fr, _, _ := quickDeployModel(t, quickDeployConfig(true, false), "UAT", "UAT_SBX", "0AfQUICK1")
			m.quickConfirm = confirm

			next, cmd := m.Update(keyPress("enter"))
			nm := next.(Model)
			if nm.State() != StateQuickDeploy {
				t.Fatalf("wrong/missing confirmation must keep the user on StateQuickDeploy, got %v", nm.State())
			}
			if cmd != nil {
				t.Error("wrong/missing confirmation must NOT fire the quick-deploy command")
			}
			if len(fr.Calls) != 0 {
				t.Errorf("no sf call should have been made, got %v", fr.Calls)
			}
		})
	}
}

// TestKeyQuickDeploy_Enter_ProductionWithoutAllowProduction_DoesNotFireExec
// is task 4.10 (RED): a production target with AllowProduction false blocks
// execution even with AllowExecution true and the exact typed DESPLEGAR
// confirmation (quick-deploy spec: "Production target without
// AllowProduction is not executed").
func TestKeyQuickDeploy_Enter_ProductionWithoutAllowProduction_DoesNotFireExec(t *testing.T) {
	m, fr, dir, runID := quickDeployModel(t, quickDeployProdConfig(true, false), "main", "PROD_SBX", "0AfQUICK2")
	m = typeString(m, "DESPLEGAR")

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateQuickDeploy {
		t.Fatalf("a production target without AllowProduction must keep the user on StateQuickDeploy, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("a production target without AllowProduction must NOT fire the quick-deploy command")
	}
	if len(fr.Calls) != 0 {
		t.Errorf("no sf call should have been made, got %v", fr.Calls)
	}
	rec, err := runs.NewWriter(dir).Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !rec.QuickDeployedAt.IsZero() {
		t.Error("the run must NOT be marked quick-deployed when execution was blocked")
	}
}

// TestKeyQuickDeploy_Enter_DefaultSuggestOnly_NeverExecutes is task 4.11
// (RED): a zero-value config (AllowExecution false, the default) never
// executes, even with the exact DESPLEGAR confirmation typed on an eligible,
// non-production row (quick-deploy spec: "Default configuration never
// executes").
func TestKeyQuickDeploy_Enter_DefaultSuggestOnly_NeverExecutes(t *testing.T) {
	m, fr, _, _ := quickDeployModel(t, quickDeployConfig(false, false), "UAT", "UAT_SBX", "0AfQUICK3")
	m = typeString(m, "DESPLEGAR")

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateQuickDeploy {
		t.Fatalf("the default suggest-only config must keep the user on StateQuickDeploy, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("the default suggest-only config must NOT fire the quick-deploy command under any confirmation input")
	}
	if len(fr.Calls) != 0 {
		t.Errorf("no sf call should have been made, got %v", fr.Calls)
	}
}

// TestKeyQuickDeploy_Enter_GatePermittedAndConfirmed_FiresQuickDeployAndMarks
// is task 4.12 (RED): AllowExecution true, a permitted (non-production)
// target, and the exact typed DESPLEGAR confirmation together fire
// QuickDeploy with the SELECTED ROW's own JobID/Alias (never m.jobID/
// m.plan.SandboxAlias) — and on success MarkQuickDeployed persists
// QuickDeployedAt + quick.json (quick-deploy spec: "Opt-In Execution Runs
// Quick Deploy And Registers The Action").
func TestKeyQuickDeploy_Enter_GatePermittedAndConfirmed_FiresQuickDeployAndMarks(t *testing.T) {
	m, fr, dir, runID := quickDeployModel(t, quickDeployConfig(true, false), "UAT", "UAT_SBX", "0AfQUICK4")
	m = typeString(m, "DESPLEGAR")

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateQuickDeploy {
		t.Fatalf("the confirm screen holds until quickDeployDoneMsg lands, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("a fully authorized confirmation should fire the quick-deploy command")
	}

	msg := run(t, cmd)
	qmsg, ok := msg.(quickDeployDoneMsg)
	if !ok {
		t.Fatalf("expected a quickDeployDoneMsg, got %T", msg)
	}
	if qmsg.err != nil {
		t.Fatalf("quickDeployCmd errored: %v", qmsg.err)
	}

	if len(fr.Calls) != 1 {
		t.Fatalf("expected exactly 1 quick-deploy call, got %d: %v", len(fr.Calls), fr.Calls)
	}
	want := quickDeployArgs("0AfQUICK4", "UAT_SBX")
	got := fr.Calls[0].Args
	if len(got) != len(want) {
		t.Fatalf("expected args %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q (quick deploy must target the selected row's own JobID/Alias)", i, got[i], want[i])
		}
	}
	for _, a := range got {
		if a == "DESPLEGAR" {
			t.Fatal("the typed confirmation literal must NEVER be passed to sf as an argument")
		}
	}

	after, cmd2 := nm.Update(qmsg)
	am := after.(Model)
	if cmd2 != nil {
		t.Error("landing a successful quickDeployDoneMsg should not fire another command")
	}
	if am.quickErr != nil {
		t.Errorf("a successful quick deploy should carry no error, got %v", am.quickErr)
	}

	rec, err := runs.NewWriter(dir).Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.QuickDeployedAt.IsZero() {
		t.Error("QuickDeployedAt should be set after a successful quick deploy")
	}
	if rec.Status != "Succeeded" {
		t.Errorf("Status should stay Succeeded (ADR-4: never overwritten), got %q", rec.Status)
	}
	quickData, err := os.ReadFile(filepath.Join(dir, ".deploydeck", "runs", runID, "quick.json"))
	if err != nil {
		t.Fatalf("quick.json should be persisted on success: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(quickData, &decoded); err != nil {
		t.Fatalf("quick.json should hold the raw quick-deploy response: %v", err)
	}
}

// TestKeyQuickDeploy_QOrEsc_ReturnsToRunHistoryAndClearsBuffer proves q/esc
// back out of StateQuickDeploy to StateRunHistory, clearing the typed
// confirm buffer and any surfaced error, and firing no command.
func TestKeyQuickDeploy_QOrEsc_ReturnsToRunHistoryAndClearsBuffer(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		t.Run(key, func(t *testing.T) {
			m, fr, _, _ := quickDeployModel(t, quickDeployConfig(true, false), "UAT", "UAT_SBX", "0AfQUICK5")
			m = typeString(m, "DESP")
			m.quickErr = errStub

			next, cmd := m.Update(keyPress(key))
			nm := next.(Model)
			if nm.State() != StateRunHistory {
				t.Fatalf("%q should return to StateRunHistory, got %v", key, nm.State())
			}
			if nm.quickConfirm != "" {
				t.Errorf("%q should clear the typed confirm buffer, got %q", key, nm.quickConfirm)
			}
			if nm.quickErr != nil {
				t.Errorf("%q should clear any surfaced quick-deploy error, got %v", key, nm.quickErr)
			}
			if cmd != nil {
				t.Errorf("%q must not fire a command", key)
			}
			if len(fr.Calls) != 0 {
				t.Errorf("%q must NOT invoke any quick deploy, got %v", key, fr.Calls)
			}
		})
	}
}

// --- 4.16: viewQuickDeploy -------------------------------------------------

// TestViewQuickDeploy_ShowsCommandAndConfirmPrompt is task 4.16 (RED): the
// screen always shows the suggested `sf project deploy quick` command for
// the selected row's own JobID/target-org alias, and echoes the typed
// DESPLEGAR confirmation prompt (quick-deploy spec: "Eligible run shows its
// own job-id in the command").
func TestViewQuickDeploy_ShowsCommandAndConfirmPrompt(t *testing.T) {
	m, _, _, _ := quickDeployModel(t, quickDeployConfig(true, false), "UAT", "UAT_SBX", "0AfQUICK6")
	m = typeString(m, "DES")

	v := m.View()
	if !strings.Contains(v, "sf project deploy quick") {
		t.Errorf("view should show the suggested command, got:\n%s", v)
	}
	if !strings.Contains(v, "0AfQUICK6") {
		t.Errorf("view should show the selected run's own JobID, got:\n%s", v)
	}
	if !strings.Contains(v, "UAT_SBX") {
		t.Errorf("view should show the target org alias, got:\n%s", v)
	}
	if !strings.Contains(v, quickDeployConfirmWord) {
		t.Errorf("view should show the DESPLEGAR prompt, got:\n%s", v)
	}
	if !strings.Contains(v, "DES") {
		t.Errorf("view should echo what the user has typed so far, got:\n%s", v)
	}
}

// TestViewQuickDeploy_ShowsSuggestOnlyNote_WhenAllowExecutionFalse is task
// 4.16's suggest-only-gated-off case: the command is still shown, but a
// note explains execution is disabled (quick-deploy spec: "Default
// configuration never executes" — the view must make this visible, not just
// the underlying gate).
func TestViewQuickDeploy_ShowsSuggestOnlyNote_WhenAllowExecutionFalse(t *testing.T) {
	m, _, _, _ := quickDeployModel(t, quickDeployConfig(false, false), "UAT", "UAT_SBX", "0AfQUICK7")

	v := m.View()
	if !strings.Contains(v, "0AfQUICK7") {
		t.Errorf("view should still show the suggested command's JobID even when suggest-only, got:\n%s", v)
	}
	if !strings.Contains(v, "solo sugerido") {
		t.Errorf("view should note execution is gated off (suggest-only), got:\n%s", v)
	}
}

// TestViewQuickDeploy_ShowsQuickErr proves a failed quick deploy's error is
// surfaced on the screen (mirrors viewCancelConfirm's cancelErr display).
func TestViewQuickDeploy_ShowsQuickErr(t *testing.T) {
	m, _, _, _ := quickDeployModel(t, quickDeployConfig(true, false), "UAT", "UAT_SBX", "0AfQUICK8")
	m.quickErr = errStub

	v := m.View()
	if !strings.Contains(v, "stub error") {
		t.Errorf("view should surface the quick-deploy error, got:\n%s", v)
	}
}
