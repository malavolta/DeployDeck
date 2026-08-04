package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// validPackageXML is a minimal well-formed package.xml, reused across
// Group 4's pre-check tests (the same shape
// TestModel_DeltaCmd_ComposesGenerateAndSummarize already exercises via
// delta.ParsePackage).
const validPackageXML = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
  <types><members>AccountService</members><name>ApexClass</name></types>
  <version>59.0</version>
</Package>`

// standaloneValidateConfig is a small config with two sandbox entries so
// standaloneSandboxAliases's dedup+sort (task 4.1) is exercised
// deterministically.
func standaloneValidateConfig() config.Config {
	return config.Config{
		Branches: map[string]string{"uat": "UAT", "integration": "INT"},
		Sandboxes: map[string]config.SandboxConfig{
			"UAT": {Alias: "UAT_SBX", TestLevel: "RunLocalTests"},
			"INT": {Alias: "INT_SBX", TestLevel: "NoTestRun"},
		},
		BranchFormat: config.DefaultBranchFormat,
	}
}

// packageSelectModel parks a Model on StatePackageSelect, as the menu's
// "Validar package" entry would have left it (keyMainMenu's StatePackageSelect
// case).
func packageSelectModel(cfg config.Config) Model {
	m := New(Deps{Dir: "/repo", Config: cfg})
	m.state = StatePackageSelect
	m.standaloneMode = "validate"
	return m
}

// TestModel_PackageSelect_ValidPath_AdvancesToSandboxSelect is task 4.1 (RED):
// Enter on a path to an existing, parseable package.xml passes the pre-check
// (parsePackageFile, reused UNCHANGED from the delta step), holds the chosen
// path on packagePath, and advances to StateSandboxSelect with the picker
// built synchronously from cfg.Sandboxes (design ADR-2: "no exec" — unlike
// the delta base-branch picker, this needs no async command/message). AC:
// "Invalid Or Nonexistent Package Rejected Before Launch" (valid-path
// counterpart).
func TestModel_PackageSelect_ValidPath_AdvancesToSandboxSelect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.xml")
	if err := os.WriteFile(path, []byte(validPackageXML), 0o644); err != nil {
		t.Fatal(err)
	}

	m := packageSelectModel(standaloneValidateConfig())
	m.packagePath = path

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)

	if nm.State() != StateSandboxSelect {
		t.Fatalf("a valid package.xml should advance to StateSandboxSelect, got %v", nm.State())
	}
	if nm.packagePath != path {
		t.Errorf("packagePath should hold the chosen path, got %q", nm.packagePath)
	}
	want := []string{"INT_SBX", "UAT_SBX"}
	if !reflect.DeepEqual(nm.sandboxList, want) {
		t.Errorf("sandboxList = %v, want %v (deduped, sorted cfg.Sandboxes aliases)", nm.sandboxList, want)
	}
	if cmd != nil {
		t.Error("advancing to the sandbox picker should not fire any command (no exec, ADR-2)")
	}
}

// TestModel_PackageSelect_NonexistentOrInvalidPackage_BlocksAdvance is task
// 4.3 (RED): neither a nonexistent path nor a malformed package.xml ever
// advances past the pre-check — both stay on StatePackageSelect with an
// actionable notice, and NEVER fire any command (validateCmd/ValidateDeploy
// is unreachable from here). AC: "Invalid Or Nonexistent Package Rejected
// Before Launch".
func TestModel_PackageSelect_NonexistentOrInvalidPackage_BlocksAdvance(t *testing.T) {
	dir := t.TempDir()
	malformedPath := filepath.Join(dir, "malformed.xml")
	if err := os.WriteFile(malformedPath, []byte("not xml at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"nonexistent path", filepath.Join(dir, "does-not-exist.xml")},
		{"malformed XML", malformedPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := packageSelectModel(standaloneValidateConfig())
			m.packagePath = tt.path

			next, cmd := m.Update(keyPress("enter"))
			nm := next.(Model)

			if nm.State() != StatePackageSelect {
				t.Fatalf("an invalid package.xml must block the advance, got %v", nm.State())
			}
			if nm.notice == "" {
				t.Error("an invalid package.xml should surface an actionable notice")
			}
			if cmd != nil {
				t.Error("an invalid package.xml must not fire any command (validateCmd must never launch)")
			}
		})
	}
}

// sandboxSelectModel parks a Model on StateSandboxSelect with a preloaded
// sandbox list, as confirmPackageSelect would have left it.
func sandboxSelectModel(cfg config.Config, writer *runs.Writer, now func() time.Time) Model {
	m := New(Deps{Dir: "/repo", Config: cfg, Runs: writer, Now: now})
	m.state = StateSandboxSelect
	m.standaloneMode = "validate"
	m.packagePath = "pkg/package.xml"
	m.sandboxList = []string{"INT_SBX", "UAT_SBX"}
	m.sandboxCursor = 1
	return m
}

// TestModel_SandboxSelect_ConfirmPreCreatesRunAndFiresValidateCmd is task 4.5
// (RED): confirming the selected sandbox seeds the MINIMAL plan validateCmd's
// reuse branch reads (design ADR-2: PackageXMLPath/SandboxAlias/TestLevel —
// TestLevel from the matching SandboxConfig), pre-creates a local run tagged
// Mode="validate" with NO jobId yet (design ADR-4: "BEFORE firing
// validateCmd"), holds its id on m.runID, and enters StateValidationStart
// firing validateCmd UNCHANGED. AC: "Standalone Validation Launches And Polls
// Like The Full Flow", "Standalone Modes Create A Local Run" (validate).
func TestModel_SandboxSelect_ConfirmPreCreatesRunAndFiresValidateCmd(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	m := sandboxSelectModel(standaloneValidateConfig(), writer, clk.now)

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)

	if nm.State() != StateValidationStart {
		t.Fatalf("confirming the sandbox should enter StateValidationStart, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("confirming the sandbox should fire validateCmd")
	}
	if nm.Plan().PackageXMLPath != "pkg/package.xml" {
		t.Errorf("PackageXMLPath = %q, want %q", nm.Plan().PackageXMLPath, "pkg/package.xml")
	}
	if nm.Plan().SandboxAlias != "UAT_SBX" {
		t.Errorf("SandboxAlias = %q, want %q", nm.Plan().SandboxAlias, "UAT_SBX")
	}
	if nm.Plan().TestLevel != "RunLocalTests" {
		t.Errorf("TestLevel = %q, want %q (resolved from the matching SandboxConfig)", nm.Plan().TestLevel, "RunLocalTests")
	}
	if nm.runID == "" {
		t.Fatal("confirming the sandbox should pre-create a run and hold its id on m.runID")
	}

	records, err := writer.List()
	if err != nil {
		t.Fatalf("writer.List(): %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly one pre-created run, got %d", len(records))
	}
	rec := records[0]
	if rec.Mode != "validate" {
		t.Errorf("Mode = %q, want %q", rec.Mode, "validate")
	}
	if rec.ManifestPath != "pkg/package.xml" {
		t.Errorf("ManifestPath = %q, want %q", rec.ManifestPath, "pkg/package.xml")
	}
	if rec.JobID != "" {
		t.Errorf("a pre-created validate run must not carry a jobId yet, got %q", rec.JobID)
	}
}

// TestModel_SandboxSelect_Confirm_ResetsStalePlanBeforeValidate is the
// adversarial-review remediation for Finding 1 (HIGH): m.plan is NEVER reset
// for standalone modes, so a prior full-flow promotion's
// DestructiveChangesPath (and other fields) would bleed into a standalone
// validation — validateCmd reads plan.DestructiveChangesPath as
// PostDestructivePath and would silently include destructive deletions from
// an EARLIER promotion. confirmSandboxSelect must build a FRESH minimal plan
// so NO stale field survives, and the fired sf validate invocation must carry
// no post-destructive arg.
func TestModel_SandboxSelect_Confirm_ResetsStalePlanBeforeValidate(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}

	// Only the CLEAN args (no --post-destructive-changes) are canned; a stale
	// DestructiveChangesPath leaking into the plan would make validateCmd build
	// a different arg vector the FakeRunner has no response for.
	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", "pkg/package.xml",
		"--target-org", "UAT_SBX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"id":"0Af000000000099EAA","done":false,"state":"Queued"}}`),
	})

	m := New(Deps{Dir: dir, Config: standaloneValidateConfig(), SF: salesforce.New(fr), Runs: writer, Now: clk.now})
	m.state = StateSandboxSelect
	m.standaloneMode = "validate"
	m.packagePath = "pkg/package.xml"
	m.sandboxList = []string{"INT_SBX", "UAT_SBX"}
	m.sandboxCursor = 1
	// Pre-seed a DIRTY plan from a prior full-flow promotion.
	m.plan = git.DeploymentPlan{
		Ticket:                 "PROJ-9",
		SelectedCommits:        []git.DiscoveredCommit{{}},
		TargetBranch:           "UAT",
		SandboxAlias:           "OLD_SBX",
		TestLevel:              "NoTestRun",
		PromotionBranch:        "PROJ-9-to-UAT",
		PackageXMLPath:         "/stale/package.xml",
		DestructiveChangesPath: "/stale/destructiveChanges.xml",
	}

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)

	// The plan validateCmd reads must be FRESH — no stale field survives.
	if got := nm.Plan().DestructiveChangesPath; got != "" {
		t.Errorf("stale DestructiveChangesPath must be reset for standalone validate, got %q", got)
	}
	if got := nm.Plan().PromotionBranch; got != "" {
		t.Errorf("stale PromotionBranch must not survive standalone validate, got %q", got)
	}
	if got := nm.Plan().TargetBranch; got != "" {
		t.Errorf("stale TargetBranch must not survive standalone validate, got %q", got)
	}
	if got := nm.Plan().Ticket; got != "" {
		t.Errorf("stale Ticket must not survive standalone validate, got %q", got)
	}
	if len(nm.Plan().SelectedCommits) != 0 {
		t.Errorf("stale SelectedCommits must not survive standalone validate, got %v", nm.Plan().SelectedCommits)
	}
	// ...and the fresh minimal fields ARE seeded.
	if nm.Plan().PackageXMLPath != "pkg/package.xml" {
		t.Errorf("fresh PackageXMLPath = %q, want %q", nm.Plan().PackageXMLPath, "pkg/package.xml")
	}
	if nm.Plan().SandboxAlias != "UAT_SBX" {
		t.Errorf("fresh SandboxAlias = %q, want %q", nm.Plan().SandboxAlias, "UAT_SBX")
	}
	if nm.Plan().TestLevel != "RunLocalTests" {
		t.Errorf("fresh TestLevel = %q, want %q", nm.Plan().TestLevel, "RunLocalTests")
	}

	// The actual sf validate invocation must carry NO post-destructive arg and
	// no stale path. confirmSandboxSelect now batches validateCmd with the
	// progress spinnerCmd (task 5.8, design D5), so the returned command is a
	// tea.BatchMsg — unwrap it and run every sub-command (mirrors
	// original_branch_test.go's established unwrap pattern) so validateCmd's
	// side effect (the sf runner invocation) actually fires.
	if cmd == nil {
		t.Fatal("confirming the sandbox should fire validateCmd")
	}
	msg := run(t, cmd)
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg (validateCmd + spinnerCmd), got %T", msg)
	}
	for _, c := range batch {
		c()
	}
	if len(fr.Calls) == 0 {
		t.Fatal("expected validateCmd to invoke the sf runner")
	}
	args := fr.Calls[0].Args
	for _, a := range args {
		if a == "--post-destructive-changes" {
			t.Errorf("standalone validate must not include destructive changes from a prior plan: args=%v", args)
		}
	}
	if strings.Contains(strings.Join(args, " "), "/stale/") {
		t.Errorf("no stale path may leak into the sf validate args: %v", args)
	}
}

// TestValidateCmd_StandaloneValidate_MergesJobIdOntoPreCreatedRecord is task
// 4.7 (RED, expected to land already-GREEN — a pure-reuse pin mirroring
// task 3.9's delta-side pin): validateCmd's EXISTING m.runID!="" reuse branch
// (commands.go, UNCHANGED) merges the jobId onto the pre-created
// Mode="validate" record via Load+Save, and Mode/ManifestPath survive the
// round-trip.
func TestValidateCmd_StandaloneValidate_MergesJobIdOntoPreCreatedRecord(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}

	runID := "validate-UAT_SBX-20260102030405"
	if err := writer.Save(runs.Record{
		RunID:        runID,
		Mode:         "validate",
		ManifestPath: "pkg/package.xml",
		Alias:        "UAT_SBX",
		TestLevel:    "RunLocalTests",
		CreatedAt:    clk.t,
		UpdatedAt:    clk.t,
	}); err != nil {
		t.Fatalf("seeding the pre-created run: %v", err)
	}

	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", "pkg/package.xml",
		"--target-org", "UAT_SBX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"id":"0Af000000000099EAA","done":false,"state":"Queued"}}`),
	})

	m := New(Deps{Dir: dir, Config: standaloneValidateConfig(), SF: salesforce.New(fr), Runs: writer, Now: clk.now})
	m.plan.PackageXMLPath = "pkg/package.xml"
	m.plan.SandboxAlias = "UAT_SBX"
	m.plan.TestLevel = "RunLocalTests"
	m.runID = runID

	msg := run(t, m.validateCmd())
	vmsg, ok := msg.(validateDoneMsg)
	if !ok {
		t.Fatalf("expected validateDoneMsg, got %T", msg)
	}
	if vmsg.err != nil {
		t.Fatalf("validateCmd errored: %v", vmsg.err)
	}
	if vmsg.runID != runID {
		t.Fatalf("expected the pre-created runID reused, got %q, want %q", vmsg.runID, runID)
	}

	rec, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.JobID != "0Af000000000099EAA" {
		t.Errorf("JobID not merged, got %q", rec.JobID)
	}
	if rec.Mode != "validate" {
		t.Errorf("Mode should survive the round-trip, got %q", rec.Mode)
	}
	if rec.ManifestPath != "pkg/package.xml" {
		t.Errorf("ManifestPath should survive the round-trip, got %q", rec.ManifestPath)
	}

	entries, err := os.ReadDir(filepath.Join(dir, ".deploydeck", "runs"))
	if err != nil {
		t.Fatalf("reading runs dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 run dir (reused, not duplicated), got %d", len(entries))
	}
}

// TestValidateCmd_LaunchFailureWithPreCreatedRunID_PersistsValidateJSONCompanion
// is the deploy-error-detail RED (task 3.7, D7, deploy-validation spec
// "Launch Error Shows An Actionable Message And The Persisted-Raw Path"): a
// launch failure with a pre-created runID (e.g. the standalone-validate
// Mode="validate" record) persists the raw failure envelope via
// SaveRawCompanion("validate.json") and returns its path on validateDoneMsg.
func TestValidateCmd_LaunchFailureWithPreCreatedRunID_PersistsValidateJSONCompanion(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}

	runID := "validate-UAT_SBX-20260102030405"
	if err := writer.Save(runs.Record{
		RunID:        runID,
		Mode:         "validate",
		ManifestPath: "pkg/package.xml",
		Alias:        "UAT_SBX",
		TestLevel:    "RunLocalTests",
		CreatedAt:    clk.t,
		UpdatedAt:    clk.t,
	}); err != nil {
		t.Fatalf("seeding the pre-created run: %v", err)
	}

	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", "pkg/package.xml",
		"--target-org", "UAT_SBX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}, execpkg.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"InvalidManifest","message":"package.xml is invalid","exitCode":1}`),
	})

	m := New(Deps{Dir: dir, Config: standaloneValidateConfig(), SF: salesforce.New(fr), Runs: writer, Now: clk.now})
	m.plan.PackageXMLPath = "pkg/package.xml"
	m.plan.SandboxAlias = "UAT_SBX"
	m.plan.TestLevel = "RunLocalTests"
	m.runID = runID

	msg := run(t, m.validateCmd())
	vmsg, ok := msg.(validateDoneMsg)
	if !ok {
		t.Fatalf("expected validateDoneMsg, got %T", msg)
	}
	if vmsg.err == nil {
		t.Fatal("expected a launch error")
	}
	wantPath := filepath.Join(dir, ".deploydeck", "runs", runID, "validate.json")
	if vmsg.rawPath != wantPath {
		t.Fatalf("expected rawPath %q, got %q", wantPath, vmsg.rawPath)
	}

	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected validate.json companion to exist: %v", err)
	}
	if !strings.Contains(string(got), "InvalidManifest") {
		t.Errorf("expected the persisted companion to hold the raw failure envelope, got %q", got)
	}
}

// TestValidateCmd_LaunchFailureWithNoRunID_CreatesFailedRunFallback is the
// deploy-error-detail RED (task 3.7, D7): a launch failure with NO
// pre-created runID falls back to Create(rec{Status:"Failed"}, raw) —
// mirroring the success no-runID fallback — and returns the persisted
// validate.json path.
func TestValidateCmd_LaunchFailureWithNoRunID_CreatesFailedRunFallback(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}

	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", "pkg/package.xml",
		"--post-destructive-changes", "pkg/destructiveChanges.xml",
		"--target-org", "UAT_SBX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}, execpkg.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"AuthError","message":"session expired","exitCode":1}`),
	})

	m := New(Deps{Dir: dir, Config: validationConfig(), SF: salesforce.New(fr), Runs: writer, Now: clk.now})
	m.plan = git.DeploymentPlan{
		Ticket:                 "PROJ-9",
		TargetBranch:           "UAT",
		SandboxAlias:           "UAT_SBX",
		PackageXMLPath:         "pkg/package.xml",
		DestructiveChangesPath: "pkg/destructiveChanges.xml",
		TestLevel:              "RunLocalTests",
	}

	msg := run(t, m.validateCmd())
	vmsg, ok := msg.(validateDoneMsg)
	if !ok {
		t.Fatalf("expected validateDoneMsg, got %T", msg)
	}
	if vmsg.err == nil {
		t.Fatal("expected a launch error")
	}
	if vmsg.runID == "" {
		t.Fatal("expected a fallback runID to be created")
	}

	rec, err := writer.Load(vmsg.runID)
	if err != nil {
		t.Fatalf("expected the fallback run to be persisted: %v", err)
	}
	if rec.Status != "Failed" {
		t.Errorf("expected fallback run Status %q, got %q", "Failed", rec.Status)
	}

	wantPath := filepath.Join(dir, ".deploydeck", "runs", vmsg.runID, "validate.json")
	if vmsg.rawPath != wantPath {
		t.Fatalf("expected rawPath %q, got %q", wantPath, vmsg.rawPath)
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected validate.json to exist: %v", err)
	}
	if !strings.Contains(string(got), "AuthError") {
		t.Errorf("expected validate.json to hold the raw failure envelope, got %q", got)
	}
}
