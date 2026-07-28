package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// This file is the consolidated HU-015 Test E2E group (Group 5,
// docs/HISTORIAS.md:1031-1037): a real runs.Writer fixture (t.TempDir()),
// an injected clock for the 10-day quick-deploy window, and a FakeRunner
// standing in for `sf project deploy quick` — no real Salesforce org, no
// real git. Unlike Group 4's quickDeployModel helper (which hand-authors
// m.runs in memory), every test here seeds through the real Writer.Create
// path and reloads via Writer.List(), proving Groups 1-4 compose correctly
// through the actual on-disk round trip. Pure Model.Update + fake-runner
// coverage needs no testing.Short() skip (no real git/subprocess is used).

// runIndex returns the index of the record whose RunID matches runID
// within records, failing the test if absent — needed because
// runs.Writer.List() reorders newest-first by CreatedAt, not by
// seed/insertion order.
func runIndex(t *testing.T, records []runs.Record, runID string) int {
	t.Helper()
	for i, r := range records {
		if r.RunID == runID {
			return i
		}
	}
	t.Fatalf("run %q not found among %d loaded records", runID, len(records))
	return -1
}

// seedQuickDeployRuns creates every rec under writer via the real Create
// path (run.json + validate.json on disk, exactly what StateRunHistory
// loads from in production), then reloads the whole set through
// writer.List() so the driven Model.runs is a genuine round trip through
// the runs fixture.
func seedQuickDeployRuns(t *testing.T, writer *runs.Writer, recs ...runs.Record) []runs.Record {
	t.Helper()
	for _, rec := range recs {
		if _, err := writer.Create(rec, []byte(`{"raw":"validate"}`)); err != nil {
			t.Fatalf("seeding run %s: %v", rec.RunID, err)
		}
	}
	records, err := writer.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return records
}

// TestQuickDeploy_E2E_EligibleShowsCommand is task 5.1 (RED): driving
// StateRunHistory -> x on an eligible seeded run (Succeeded, RunLocalTests,
// age under 10 days) opens StateQuickDeploy showing that run's own JobID +
// target-org alias in the suggested command, firing no sf call
// (quick-deploy spec: "Eligible run shows its own job-id in the command" +
// "Default configuration never executes").
func TestQuickDeploy_E2E_EligibleShowsCommand(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}

	records := seedQuickDeployRuns(t, writer, runs.Record{
		RunID: "eligible-run", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX",
		JobID: "0AfELIGIBLE", Status: "Succeeded", TestLevel: "RunLocalTests",
		CreatedAt: clk.t.Add(-24 * time.Hour),
	})

	fr := execpkg.NewFakeRunner()
	deps := Deps{Dir: dir, Config: quickDeployConfig(true, false), SF: salesforce.New(fr), Runs: writer, Now: clk.now}
	m := New(deps)
	m.state = StateRunHistory
	m.runs = records
	m.runsCursor = runIndex(t, records, "eligible-run")

	next, cmd := m.Update(keyPress("x"))
	nm := next.(Model)
	if nm.State() != StateQuickDeploy {
		t.Fatalf("x on the eligible run should open StateQuickDeploy, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("opening the quick-deploy view should not fire a command yet")
	}

	v := nm.View()
	if !strings.Contains(v, "sf project deploy quick") {
		t.Errorf("view should show the suggested command, got:\n%s", v)
	}
	if !strings.Contains(v, "0AfELIGIBLE") {
		t.Errorf("view should show the eligible run's own JobID, got:\n%s", v)
	}
	if !strings.Contains(v, "UAT_SBX") {
		t.Errorf("view should show the target org alias, got:\n%s", v)
	}
	if len(fr.Calls) != 0 {
		t.Errorf("no sf call should fire merely from opening the view, got %v", fr.Calls)
	}
}

// TestQuickDeploy_E2E_IneligibleVariants is task 5.2 (RED): x on a row
// failing any single QuickDeployEligible predicate is a strict no-op —
// the age variant (HISTORIAS "no elegible por antiguedad"), the test-level
// variant (HISTORIAS "no elegible por test level", including the
// backward-compat zero-value TestLevel case), and the correction-2
// already-quick-deployed double-deploy guard.
func TestQuickDeploy_E2E_IneligibleVariants(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	cases := []struct {
		name string
		rec  runs.Record
	}{
		{"older than 10 days", runs.Record{
			RunID: "aged-out", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfAGED",
			Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-11 * 24 * time.Hour),
		}},
		{"RunSpecifiedTests not sufficient", runs.Record{
			RunID: "specified-tests", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfSPEC",
			Status: "Succeeded", TestLevel: "RunSpecifiedTests", CreatedAt: clk.t.Add(-time.Hour),
		}},
		{"NoTestRun not sufficient", runs.Record{
			RunID: "no-test-run", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfNOTEST",
			Status: "Succeeded", TestLevel: "NoTestRun", CreatedAt: clk.t.Add(-time.Hour),
		}},
		{"zero-value TestLevel (pre-HU-015 run.json) fails safe", runs.Record{
			RunID: "pre-hu015", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfPRE",
			Status: "Succeeded", TestLevel: "", CreatedAt: clk.t.Add(-time.Hour),
		}},
		{"already quick-deployed (correction 2 double-deploy guard)", runs.Record{
			RunID: "already-deployed", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfALREADY",
			Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-time.Hour),
			QuickDeployedAt: clk.t.Add(-time.Minute),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writer := runs.NewWriter(dir)
			records := seedQuickDeployRuns(t, writer, tc.rec)

			fr := execpkg.NewFakeRunner()
			deps := Deps{Dir: dir, Config: quickDeployConfig(true, false), SF: salesforce.New(fr), Runs: writer, Now: clk.now}
			m := New(deps)
			m.state = StateRunHistory
			m.runs = records
			m.runsCursor = runIndex(t, records, tc.rec.RunID)

			next, cmd := m.Update(keyPress("x"))
			nm := next.(Model)
			if nm.State() != StateRunHistory {
				t.Fatalf("x on an ineligible row (%s) should be a no-op, got %v", tc.name, nm.State())
			}
			if cmd != nil {
				t.Errorf("x on an ineligible row (%s) must fire no command", tc.name)
			}
			if len(fr.Calls) != 0 {
				t.Errorf("x on an ineligible row (%s) must never call sf, got %v", tc.name, fr.Calls)
			}
		})
	}
}

// TestQuickDeploy_E2E_ProductionBlockedWithoutConfig is task 5.3 (RED): a
// quick-deploy-eligible run whose target resolves to production
// (git.IsProductionTarget's fail-closed literal-branch arm, no configured
// "production" Branches key), with AllowExecution true but AllowProduction
// false, refuses execution even with the exact typed DESPLEGAR
// confirmation — the fake sf runner is NEVER invoked and the run is never
// marked quick-deployed (quick-deploy spec: "Production target without
// AllowProduction is not executed").
func TestQuickDeploy_E2E_ProductionBlockedWithoutConfig(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}

	rec := runs.Record{
		RunID: "prod-run", Ticket: "PROJ-1", Target: "main", Alias: "PROD_SBX", JobID: "0AfPROD",
		Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-24 * time.Hour),
	}
	records := seedQuickDeployRuns(t, writer, rec)

	fr := execpkg.NewFakeRunner()
	fr.When("sf", quickDeployArgs(rec.JobID, rec.Alias), quickDeploySuccess(rec.JobID))
	deps := Deps{Dir: dir, Config: quickDeployProdConfig(true, false), SF: salesforce.New(fr), Runs: writer, Now: clk.now}
	m := New(deps)
	m.state = StateRunHistory
	m.runs = records
	m.runsCursor = runIndex(t, records, rec.RunID)

	next, _ := m.Update(keyPress("x"))
	m = next.(Model)
	if m.State() != StateQuickDeploy {
		t.Fatalf("setup: x should open StateQuickDeploy on the eligible seeded run, got %v", m.State())
	}
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
	got, err := writer.Load(rec.RunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.QuickDeployedAt.IsZero() {
		t.Error("the run must NOT be marked quick-deployed when production execution was blocked")
	}
}

// TestQuickDeploy_E2E_NoConfirmDoesNotDeploy is task 5.4 (RED): the
// execution gate is otherwise fully permitted (AllowExecution true, a
// non-production target), but the typed confirmation is empty or wrong —
// enter must never call sf and must stay on StateQuickDeploy (quick-deploy
// spec: "Missing confirmation blocks execution").
func TestQuickDeploy_E2E_NoConfirmDoesNotDeploy(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	for _, confirm := range []string{"", "desplegar", "DESPLEGA"} {
		t.Run("confirm="+confirm, func(t *testing.T) {
			dir := t.TempDir()
			writer := runs.NewWriter(dir)
			rec := runs.Record{
				RunID: "noconfirm-run", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfNOCONFIRM",
				Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-24 * time.Hour),
			}
			records := seedQuickDeployRuns(t, writer, rec)

			fr := execpkg.NewFakeRunner()
			fr.When("sf", quickDeployArgs(rec.JobID, rec.Alias), quickDeploySuccess(rec.JobID))
			deps := Deps{Dir: dir, Config: quickDeployConfig(true, false), SF: salesforce.New(fr), Runs: writer, Now: clk.now}
			m := New(deps)
			m.state = StateRunHistory
			m.runs = records
			m.runsCursor = runIndex(t, records, rec.RunID)

			next, _ := m.Update(keyPress("x"))
			m = next.(Model)
			if confirm != "" {
				m = typeString(m, confirm)
			}

			next, cmd := m.Update(keyPress("enter"))
			nm := next.(Model)
			if nm.State() != StateQuickDeploy {
				t.Fatalf("missing/wrong confirmation must keep the user on StateQuickDeploy, got %v", nm.State())
			}
			if cmd != nil {
				t.Error("missing/wrong confirmation must NOT fire the quick-deploy command")
			}
			if len(fr.Calls) != 0 {
				t.Errorf("no sf call should have been made, got %v", fr.Calls)
			}
		})
	}
}

// TestQuickDeploy_E2E_GateAndConfirmExecutesAndRegisters is task 5.5 (RED):
// the full authorized path — AllowExecution true, a permitted (non-prod)
// target, the exact typed DESPLEGAR confirmation — fires `sf project deploy
// quick` with the SELECTED ROW's own JobID/Alias, and on success
// MarkQuickDeployed persists QuickDeployedAt + quick.json while leaving
// Status untouched (quick-deploy spec: "Opt-In Execution Runs Quick Deploy
// And Registers The Action", the full HU-015 Test E2E AC set,
// docs/HISTORIAS.md:1034).
func TestQuickDeploy_E2E_GateAndConfirmExecutesAndRegisters(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}

	rec := runs.Record{
		RunID: "execute-run", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfEXECUTE",
		Status: "Succeeded", TestLevel: "RunLocalTests", CreatedAt: clk.t.Add(-24 * time.Hour),
	}
	records := seedQuickDeployRuns(t, writer, rec)

	fr := execpkg.NewFakeRunner()
	fr.When("sf", quickDeployArgs(rec.JobID, rec.Alias), quickDeploySuccess(rec.JobID))
	deps := Deps{Dir: dir, Config: quickDeployConfig(true, false), SF: salesforce.New(fr), Runs: writer, Now: clk.now}
	m := New(deps)
	m.state = StateRunHistory
	m.runs = records
	m.runsCursor = runIndex(t, records, rec.RunID)

	next, _ := m.Update(keyPress("x"))
	m = next.(Model)
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
	want := quickDeployArgs(rec.JobID, rec.Alias)
	got := fr.Calls[0].Args
	if len(got) != len(want) {
		t.Fatalf("expected args %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q (quick deploy must target the selected row's own JobID/Alias)", i, got[i], want[i])
		}
	}

	after := advance(t, nm, qmsg)
	if after.quickErr != nil {
		t.Errorf("a successful quick deploy should carry no error, got %v", after.quickErr)
	}

	gotRec, err := writer.Load(rec.RunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRec.QuickDeployedAt.IsZero() {
		t.Error("QuickDeployedAt should be set after a successful quick deploy")
	}
	if gotRec.Status != "Succeeded" {
		t.Errorf("Status should stay Succeeded (ADR-4: never overwritten), got %q", gotRec.Status)
	}
	quickData, err := os.ReadFile(filepath.Join(dir, ".deploydeck", "runs", rec.RunID, "quick.json"))
	if err != nil {
		t.Fatalf("quick.json should be persisted on success: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(quickData, &decoded); err != nil {
		t.Fatalf("quick.json should hold the raw quick-deploy response: %v", err)
	}
}
