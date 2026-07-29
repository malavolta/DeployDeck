package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"deploydeck/internal/config"
	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
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
