package app

// standalone_modes_e2e_test.go is HU-018's consolidated Group 5 e2e (tasks.md
// "Group 5 — Consolidated HU-018 e2e", docs/HISTORIAS.md:1143-1149's "Test
// E2E"). Every test here drives the REAL Model.Update — never asserts against
// a hand-parked Model — proving the menu, standalone delta (real temp git +
// real `sf sgd`), and standalone validation (fake `sf`) wiring built in
// Groups 2-4 actually composes end-to-end, exactly as HU-018's own Test E2E
// section specifies: "Harness: repo git temporal (+ sfdx-git-delta) para el
// modo delta; sf falso / alias local para el modo validacion."

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/delta"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// freshAtMainMenu drives a brand-new Model from StatePrereqCheck through a
// synthetic OK prereqDoneMsg (fed directly, mirroring flow_e2e_test.go's
// convention for tests that exercise composition rather than the doctor
// screen itself), asserting it lands on HU-018's StateMainMenu.
func freshAtMainMenu(t *testing.T, deps Deps) Model {
	t.Helper()
	m := New(deps)
	if m.State() != StatePrereqCheck {
		t.Fatalf("start state = %v", m.State())
	}
	m = advance(t, m, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	if m.State() != StateMainMenu {
		t.Fatalf("after prereq: want StateMainMenu, got %v", m.State())
	}
	return m
}

// TestE2E_MenuRouting_AllThreeEntriesReachable is task 5.1 (RED, expected to
// land already-GREEN as a proof test — Groups 1-4 wired this): from a FRESH
// Model driven through prereqDoneMsg to StateMainMenu, each of the three
// cursor positions routes Enter to its documented target state. AC: "Main
// Menu As Post-Prereq Landing", "Promote entry enters the unchanged full
// flow".
func TestE2E_MenuRouting_AllThreeEntriesReachable(t *testing.T) {
	t.Run("cursor 0 Promocionar ticket -> StateTicketInput, standaloneMode empty", func(t *testing.T) {
		m := freshAtMainMenu(t, Deps{Dir: "/repo", Config: testConfig()})
		m = advance(t, m, keyPress("enter"))
		if m.State() != StateTicketInput {
			t.Fatalf("Enter on Promocionar should reach StateTicketInput, got %v", m.State())
		}
		if m.standaloneMode != "" {
			t.Errorf("the full flow must leave standaloneMode empty, got %q", m.standaloneMode)
		}
	})

	t.Run("cursor 1 Generar delta package -> StateDeltaSourceSelect, standaloneMode delta", func(t *testing.T) {
		m := freshAtMainMenu(t, Deps{Dir: "/repo", Config: testConfig()})
		m = advance(t, m, keyPress("down"))
		m = advance(t, m, keyPress("enter"))
		if m.State() != StateDeltaSourceSelect {
			t.Fatalf("Enter on Generar delta package should reach StateDeltaSourceSelect, got %v", m.State())
		}
		if m.standaloneMode != "delta" {
			t.Errorf("standaloneMode = %q, want %q", m.standaloneMode, "delta")
		}
	})

	t.Run("cursor 2 Validar package -> StatePackageSelect, standaloneMode validate", func(t *testing.T) {
		m := freshAtMainMenu(t, Deps{Dir: "/repo", Config: testConfig()})
		m = advance(t, m, keyPress("down"))
		m = advance(t, m, keyPress("down"))
		m = advance(t, m, keyPress("enter"))
		if m.State() != StatePackageSelect {
			t.Fatalf("Enter on Validar package should reach StatePackageSelect, got %v", m.State())
		}
		if m.standaloneMode != "validate" {
			t.Errorf("standaloneMode = %q, want %q", m.standaloneMode, "validate")
		}
	})
}

// --- Standalone delta: real temp git + real sf sgd --------------------------

// standaloneApexMeta is a minimal, org-independent ApexClass -meta.xml; sgd
// only diffs files and reads the SFDX source layout, never compiles against
// an org (mirrors internal/delta/multi_source_dir_e2e_test.go's apexClassMeta).
const standaloneApexMeta = `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>59.0</apiVersion>
    <status>Active</status>
</ApexClass>
`

// rawStandaloneDeltaConfig is a small config wiring exactly what standalone
// delta needs: a single "force-app" source dir for sgd (deltaCmd reads
// cfg.Delta.SourceDirs), no Branches/Sandboxes required (delta never
// resolves a sandbox).
func rawStandaloneDeltaConfig() config.Config {
	return config.Config{
		BranchFormat: config.DefaultBranchFormat,
		Delta:        config.DeltaConfig{SourceDirs: []string{"force-app"}},
	}
}

// setupStandaloneDeltaRepo builds a temp repo with a real "force-app" SFDX
// source dir: a baseline commit exposed as a fake self-pointing
// origin/<base> (mirroring internal/delta's real-sgd e2e convention — no
// actual network remote is needed for sgd's git diff to resolve the ref),
// then — when withChange is true — one further "hand-prepared" commit on
// the SAME branch (HEAD), never a separate ticket/feature branch: HU-018's
// standalone delta has no cherry-pick step, only "rama preparada a mano" vs
// a picked base. withChange=false seeds base==HEAD (used by the empty-delta
// variant, task 5.9). Returns the repo dir and the base branch name.
func setupStandaloneDeltaRepo(t *testing.T, withChange bool) (dir, base string) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test shells out to real git and sf sgd")
	}

	dir = t.TempDir()
	base = "UAT"

	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.name", "ana")
	gitRun(t, dir, "config", "user.email", "ana@example.com")

	writeFile(t, dir, "sfdx-project.json", `{
  "packageDirectories": [{ "path": "force-app", "default": true }],
  "name": "standalone-modes-e2e",
  "namespace": "",
  "sourceApiVersion": "59.0"
}
`)
	writeNestedFile(t, dir, "force-app/main/default/classes/Standalone.cls",
		"public with sharing class Standalone {\n    public static String ping() { return 'v1'; }\n}\n")
	writeNestedFile(t, dir, "force-app/main/default/classes/Standalone.cls-meta.xml", standaloneApexMeta)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "seed: baseline Apex class")

	gitRun(t, dir, "branch", base)
	gitRun(t, dir, "remote", "add", "origin", dir)
	gitRun(t, dir, "update-ref", "refs/remotes/origin/"+base, "refs/heads/"+base)

	if withChange {
		writeNestedFile(t, dir, "force-app/main/default/classes/Standalone.cls",
			"public with sharing class Standalone {\n    public static String ping() { return 'v2'; }\n}\n")
		gitRun(t, dir, "add", "-A")
		gitRun(t, dir, "commit", "-m", "hand-prepared: edit Standalone.cls")
	}

	return dir, base
}

// writeNestedFile writes content to dir/rel, creating parent directories —
// unlike flow_e2e_test.go's writeFile (flat names only), needed here for the
// nested SFDX source layout (force-app/main/default/classes/...).
func writeNestedFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", rel, err)
	}
}

// pickStandaloneBase loads the real branch list (standaloneBranchesCmd) and
// points m.branchCursor at the remote-tracking "origin/<base>" entry so the
// subsequent Enter exercises confirmDeltaSourceSelect's "origin/" stripping
// exactly as the unit test (TestModel_DeltaSourceSelect_CursorPickAndNormalize)
// does, but here against a REAL branch listing.
func pickStandaloneBase(t *testing.T, m Model, base string) Model {
	t.Helper()
	m = advance(t, m, run(t, m.standaloneBranchesCmd()))
	want := "origin/" + base
	for i, br := range m.branchList {
		if br.Name == want && br.Remote {
			m.branchCursor = i
			return m
		}
	}
	t.Fatalf("expected %q in the real branch list, got %+v", want, m.branchList)
	return m
}

// TestE2E_StandaloneDelta_TempGitAndRealSgd is task 5.3 (RED, -short-skippable):
// a real temp git repo + the real `sf sgd source delta` binary, driven
// through the menu's "Generar delta package" entry, the real branch picker,
// and the real deltaCmd. Asserts package.xml lands under
// .deploydeck/manifest/ with a non-empty per-type summary, NO cherry-pick was
// ever performed, and a local run is created tagged Mode="delta". AC:
// "Standalone Delta Generates A Package Without Cherry-Picks" (HU-018 Test
// E2E).
func TestE2E_StandaloneDelta_TempGitAndRealSgd(t *testing.T) {
	dir, base := setupStandaloneDeltaRepo(t, true)
	writer := runs.NewWriter(dir)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Delta:  delta.New(execpkg.NewOSRunner()),
		Config: rawStandaloneDeltaConfig(),
		Dir:    dir,
		Runs:   writer,
	}

	m := freshAtMainMenu(t, deps)

	// Menu -> "Generar delta package" (cursor 1).
	m = advance(t, m, keyPress("down"))
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateDeltaSourceSelect || m.standaloneMode != "delta" {
		t.Fatalf("menu delta entry should reach StateDeltaSourceSelect/delta, got state=%v mode=%q", m.State(), m.standaloneMode)
	}

	m = pickStandaloneBase(t, m, base)

	// Confirm the picked base -> StateDeltaGeneration, fires deltaCmd.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateDeltaGeneration {
		t.Fatalf("picking the base branch should enter StateDeltaGeneration, got %v", m.State())
	}
	if m.Plan().TargetBranch != base {
		t.Fatalf("origin/ prefix should be stripped, got TargetBranch=%q", m.Plan().TargetBranch)
	}

	// Run the REAL sgd delta -> StatePackageReview.
	m = advance(t, m, run(t, m.deltaCmd()))
	if m.State() != StatePackageReview {
		t.Fatalf("after real sgd delta: %v (deltaErr=%v)", m.State(), m.deltaErr)
	}

	pkgPath := m.Plan().PackageXMLPath
	if pkgPath == "" {
		t.Fatal("expected a non-empty PackageXMLPath")
	}
	wantPrefix := filepath.Join(dir, ".deploydeck", "manifest")
	if !strings.HasPrefix(pkgPath, wantPrefix) {
		t.Fatalf("package.xml should be generated under .deploydeck/manifest/, got %q", pkgPath)
	}
	if _, err := os.Stat(pkgPath); err != nil {
		t.Fatalf("generated package.xml should exist on disk: %v", err)
	}
	if len(m.summary.Types) == 0 || m.summary.Types[0].Name != "ApexClass" {
		t.Fatalf("expected the per-type summary to list ApexClass, got %+v", m.summary.Types)
	}
	if m.summary.Empty {
		t.Fatal("a real edit should not summarize as an empty delta")
	}

	// No cherry-pick was ever performed against this repo.
	if _, err := os.Stat(filepath.Join(dir, ".git", "CHERRY_PICK_HEAD")); !os.IsNotExist(err) {
		t.Errorf("standalone delta must never cherry-pick, but CHERRY_PICK_HEAD exists (stat err=%v)", err)
	}

	// A local run was created, tagged Mode=="delta".
	records, err := writer.List()
	if err != nil {
		t.Fatalf("writer.List(): %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly one persisted run, got %d", len(records))
	}
	rec := records[0]
	if rec.Mode != "delta" {
		t.Errorf("expected Mode=%q, got %q", "delta", rec.Mode)
	}
	if rec.ManifestPath != pkgPath {
		t.Errorf("expected ManifestPath=%q, got %q", pkgPath, rec.ManifestPath)
	}
	if rec.JobID != "" {
		t.Errorf("a standalone delta run must not carry a jobId, got %q", rec.JobID)
	}
}

// TestE2E_StandaloneDelta_EmptyDelta_WarningVariant is task 5.9 (RED,
// -short-skippable): the SAME real-sgd path, but the picked base equals HEAD
// (no diff). Proves the real sgd zero-diff output flows all the way through
// onDeltaDone into the existing HU-007/008 empty-delta warning, and that a
// local run is STILL created. AC: "Empty delta reuses the existing warning".
func TestE2E_StandaloneDelta_EmptyDelta_WarningVariant(t *testing.T) {
	dir, base := setupStandaloneDeltaRepo(t, false)
	writer := runs.NewWriter(dir)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Delta:  delta.New(execpkg.NewOSRunner()),
		Config: rawStandaloneDeltaConfig(),
		Dir:    dir,
		Runs:   writer,
	}

	m := freshAtMainMenu(t, deps)
	m = advance(t, m, keyPress("down"))
	m = advance(t, m, keyPress("enter"))
	m = pickStandaloneBase(t, m, base)
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateDeltaGeneration {
		t.Fatalf("picking the base branch should enter StateDeltaGeneration, got %v", m.State())
	}

	m = advance(t, m, run(t, m.deltaCmd()))
	if m.State() != StatePackageReview {
		t.Fatalf("after real sgd delta: %v (deltaErr=%v)", m.State(), m.deltaErr)
	}

	if !m.summary.Empty {
		t.Fatalf("a zero-diff base should summarize as empty, got %+v", m.summary)
	}
	if !strings.Contains(m.View(), "Package vacío") {
		t.Errorf("standalone delta should reuse the existing empty-delta warning verbatim, got:\n%s", m.View())
	}

	records, err := writer.List()
	if err != nil {
		t.Fatalf("writer.List(): %v", err)
	}
	if len(records) != 1 || records[0].Mode != "delta" {
		t.Fatalf("an empty delta should still create a local run tagged Mode=delta, got %+v", records)
	}
}

// --- Standalone validation: fake sf ------------------------------------------

// fakeStandaloneValidateSF cans the exact validate + report sequence
// standalone validation drives: one async `sf project deploy validate`
// returning jobID, and one `sf project deploy report` immediately resolving
// to a terminal status (so the e2e drives a single deterministic poll,
// mirroring resume_test.go's reportSF for the report shape).
func fakeStandaloneValidateSF(t *testing.T, manifestPath, alias, testLevel, jobID, terminalStatus string) salesforce.Client {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", manifestPath,
		"--target-org", alias,
		"--test-level", testLevel,
		"--async", "--json",
	}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"id":"` + jobID + `","done":false,"state":"Queued"}}`),
	})
	fr.When("sf", []string{
		"project", "deploy", "report",
		"--job-id", jobID,
		"--target-org", alias,
		"--json",
	}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"status":"` + terminalStatus + `","numberComponentsTotal":5,"numberComponentsDeployed":5}}`),
	})
	return salesforce.New(fr)
}

// TestE2E_StandaloneValidate_FakeSFPollsToTerminal is task 5.5 (RED, expected
// to land already-GREEN): a real package.xml on disk, driven through the
// menu's "Validar package contra sandbox" entry, the typed path input, the
// sandbox picker, and the real validateCmd/reportCmd composed over a fake
// `sf`. Asserts the jobId is captured, polling reaches a terminal state, and
// a local run is created tagged Mode="validate". AC: "Standalone Validation
// Launches And Polls Like The Full Flow" (HU-018 Test E2E).
func TestE2E_StandaloneValidate_FakeSFPollsToTerminal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.xml")
	if err := os.WriteFile(path, []byte(validPackageXML), 0o644); err != nil {
		t.Fatal(err)
	}
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	const jobID = "0Af000000000STANDALONE"
	sf := fakeStandaloneValidateSF(t, path, "UAT_SBX", "RunLocalTests", jobID, "Succeeded")

	deps := Deps{Dir: dir, Config: standaloneValidateConfig(), SF: sf, Runs: writer, Now: clk.now}

	m := freshAtMainMenu(t, deps)
	m = advance(t, m, keyPress("down"))
	m = advance(t, m, keyPress("down"))
	m = advance(t, m, keyPress("enter"))
	if m.State() != StatePackageSelect || m.standaloneMode != "validate" {
		t.Fatalf("menu validate entry should reach StatePackageSelect/validate, got state=%v mode=%q", m.State(), m.standaloneMode)
	}

	m = typeString(m, path)
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateSandboxSelect {
		t.Fatalf("a valid package.xml should advance to StateSandboxSelect, got %v (notice=%q)", m.State(), m.notice)
	}

	idx := -1
	for i, alias := range m.sandboxList {
		if alias == "UAT_SBX" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("expected UAT_SBX in sandboxList, got %v", m.sandboxList)
	}
	m.sandboxCursor = idx

	m = advance(t, m, keyPress("enter"))
	if m.State() != StateValidationStart {
		t.Fatalf("confirming the sandbox should enter StateValidationStart, got %v", m.State())
	}
	if m.runID == "" {
		t.Fatal("confirming the sandbox should pre-create a run and hold its id on m.runID")
	}
	preCreatedRunID := m.runID

	msg := run(t, m.validateCmd())
	vmsg, ok := msg.(validateDoneMsg)
	if !ok {
		t.Fatalf("expected validateDoneMsg, got %T", msg)
	}
	if vmsg.err != nil {
		t.Fatalf("validateCmd errored: %v", vmsg.err)
	}
	if vmsg.result.JobID != jobID {
		t.Fatalf("jobId not captured: %q", vmsg.result.JobID)
	}
	if vmsg.runID != preCreatedRunID {
		t.Fatalf("validateCmd should reuse the pre-created run, got %q, want %q", vmsg.runID, preCreatedRunID)
	}

	m = advance(t, m, vmsg)
	if m.State() != StateValidationPolling {
		t.Fatalf("after jobId the flow should poll, got %v", m.State())
	}
	if m.jobID != jobID {
		t.Errorf("jobID not held on the model: %q", m.jobID)
	}

	// Drive the single, terminal real poll.
	m = advance(t, m, run(t, m.reportCmd()))
	if m.State() != StateSucceeded {
		t.Fatalf("a Succeeded report should reach the terminal StateSucceeded, got %v", m.State())
	}

	rec, err := writer.Load(preCreatedRunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.Mode != "validate" {
		t.Errorf("Mode = %q, want %q", rec.Mode, "validate")
	}
	if rec.ManifestPath != path {
		t.Errorf("ManifestPath = %q, want %q", rec.ManifestPath, path)
	}
	if rec.JobID != jobID {
		t.Errorf("JobID not merged onto the pre-created run, got %q, want %q", rec.JobID, jobID)
	}
}

// TestE2E_StandaloneValidate_InvalidPackage_ActionableErrorNoLaunch is task
// 5.7 (RED, expected to land already-GREEN): neither a nonexistent nor a
// malformed package.xml, typed via the real key handler, ever advances past
// StatePackageSelect — an actionable notice is shown and, load-bearingly,
// the fake `sf` runner records ZERO calls (validateCmd/ValidateDeploy is
// structurally unreachable from here, not merely untriggered by test
// bookkeeping). AC: "Invalid Or Nonexistent Package Rejected Before Launch".
func TestE2E_StandaloneValidate_InvalidPackage_ActionableErrorNoLaunch(t *testing.T) {
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
			fr := execpkg.NewFakeRunner() // no canned responses at all
			deps := Deps{Dir: "/repo", Config: standaloneValidateConfig(), SF: salesforce.New(fr)}

			m := freshAtMainMenu(t, deps)
			m = advance(t, m, keyPress("down"))
			m = advance(t, m, keyPress("down"))
			m = advance(t, m, keyPress("enter"))
			if m.State() != StatePackageSelect {
				t.Fatalf("menu validate entry should reach StatePackageSelect, got %v", m.State())
			}

			m = typeString(m, tt.path)
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
			if len(fr.Calls) != 0 {
				t.Errorf("sf must never be invoked for an invalid package.xml, but got calls: %+v", fr.Calls)
			}
		})
	}
}

// --- Resume-detection regression (Group 2's guard fix, full driver) --------

// TestE2E_ResumeDetection_StillOfferedAfterMenuLanding_Regression is task
// 5.11 (RED, expected to land already-GREEN — full-stack proof of 2.13/2.14):
// a REAL in-progress multi-commit cherry-pick + a matching run.json, driven
// from a FRESH Model's startup (New -> prereqDoneMsg -> resumeDetectCmd),
// still offers resume via StateRunHistory pre-selected DESPITE HU-018's
// StateMainMenu landing sitting between them, and accepting the offer
// resumes straight into the live StateCherryPickConflict — never swallowed by
// the new menu. AC: "Resume-Detection Preserved After Menu Landing".
func TestE2E_ResumeDetection_StillOfferedAfterMenuLanding_Regression(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupConflictRepo2(t)
	seed, writer := driveToCherryPicking(t, local)
	seed = advance(t, seed, run(t, seed.cherryPickCmd()))
	if seed.State() != StateCherryPickConflict {
		t.Fatalf("expected a conflict, got %v (err=%v)", seed.State(), seed.Err())
	}
	runID := seed.runID

	// Restart: a brand-new Model over the same repo + runs dir.
	fresh := New(Deps{Git: git.New(execpkg.NewOSRunner()), Config: pickIndexConfig(), Dir: local, Runs: writer})
	fresh = advance(t, fresh, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	if fresh.State() != StateMainMenu {
		t.Fatalf("prereq should land on StateMainMenu before detection runs, got %v", fresh.State())
	}

	fresh = advance(t, fresh, run(t, fresh.resumeDetectCmd()))
	if fresh.State() != StateRunHistory {
		t.Fatalf("startup detection should still offer resume via StateRunHistory despite the StateMainMenu landing, got %v", fresh.State())
	}
	if fresh.runs[fresh.runsCursor].RunID != runID {
		t.Fatalf("the in-progress run should be pre-selected, got %q", fresh.runs[fresh.runsCursor].RunID)
	}

	fresh = advance(t, fresh, keyPress("enter"))
	if fresh.State() != StateCherryPickConflict {
		t.Fatalf("accepting resume should route to StateCherryPickConflict, got %v", fresh.State())
	}
}

// TestModel_NestedDeps_SFCommands_UseProjectDir is task 5.4 (RED): with a
// NESTED Deps (ProjectDir != GitRoot), validateCmd/reportCmd/quickDeployCmd/
// cancelCmd — all four `sf project deploy *` commands — must run with cwd ==
// ProjectDir, never GitRoot or the old conflated Dir (directory-resolution
// spec: "`sf project` Commands Bind To The SFDX Project Root"), captured
// directly on exec.CommandRequest.Dir via FakeRunner (never inferred from
// Args, which stay byte-identical to before this change).
func TestModel_NestedDeps_SFCommands_UseProjectDir(t *testing.T) {
	gitRoot := t.TempDir()
	projectDir := filepath.Join(gitRoot, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(projectDir, "package.xml")

	const jobID = "0Af000000000NESTED"
	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "validate",
		"--manifest", manifestPath,
		"--target-org", "UAT_SBX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"id":"` + jobID + `","done":false,"state":"Queued"}}`)})
	fr.When("sf", []string{
		"project", "deploy", "report",
		"--job-id", jobID,
		"--target-org", "UAT_SBX",
		"--json",
	}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"status":"Succeeded"}}`)})
	fr.When("sf", []string{
		"project", "deploy", "quick",
		"--job-id", jobID,
		"--target-org", "UAT_SBX",
		"--json",
	}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"id":"` + jobID + `","status":"Succeeded"}}`)})
	fr.When("sf", []string{
		"project", "deploy", "cancel",
		"--job-id", jobID,
		"--target-org", "UAT_SBX",
		"--json",
	}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"id":"` + jobID + `","status":"Canceled"}}`)})

	deps := Deps{
		GitRoot:       gitRoot,
		ProjectDir:    projectDir,
		ArtifactsRoot: projectDir,
		Config:        standaloneValidateConfig(),
		SF:            salesforce.New(fr),
	}
	m := New(deps)
	m.plan = git.DeploymentPlan{
		PackageXMLPath: manifestPath,
		SandboxAlias:   "UAT_SBX",
		TestLevel:      "RunLocalTests",
	}
	m.jobID = jobID
	m.runs = []runs.Record{{JobID: jobID, Alias: "UAT_SBX"}}
	m.runsCursor = 0

	run(t, m.validateCmd())
	run(t, m.reportCmd())
	run(t, m.quickDeployCmd())
	run(t, m.cancelCmd())

	if len(fr.Calls) != 4 {
		t.Fatalf("expected exactly 4 sf calls, got %d: %+v", len(fr.Calls), fr.Calls)
	}
	for _, call := range fr.Calls {
		if call.Dir != projectDir {
			t.Errorf("call %v Dir = %q, want ProjectDir %q (never GitRoot)", call.Args, call.Dir, projectDir)
		}
	}
}
