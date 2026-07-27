package salesforce_test

// TestE2ERealOrg_ValidateAndReport — opt-in end-to-end test against a REAL,
// developer-owned Salesforce org, following internal/prereq/real_org_e2e_test.go's
// convention EXACTLY: activated only via DEPLOYDECK_E2E_ORG=<alias>; skipped
// (never failed) when unset, so `go test ./...` stays green in CI and for any
// contributor without a connected org. No build tag — the env-var gate alone is
// the documented activation mechanism, and an unset var must produce a visible
// SKIP, not a silently excluded file.
//
// NON-DESTRUCTIVE: it runs `sf project deploy validate --async` (CheckOnly —
// NEVER a real deploy) over a delta package generated from the committed
// test-e2e-org fixture, then polls `sf project deploy report` to a terminal
// state. A validate is a check-only operation; no metadata is written to the
// org, and the destructive path is deliberately omitted.
//
// PURPOSE beyond smoke coverage: this test CONFIRMS internal/salesforce's
// reportResultEnvelope JSON shape against REAL `sf project deploy report --json`
// output. The nested details.componentFailures / details.runTestResult.failures
// mapping was inferred (see apply-progress deviation 7); the assertions below
// are strong enough to FAIL on a shape mismatch (a report claiming component or
// test errors MUST surface the parsed failures), so a mismatch is fixed in
// report.go's struct tags rather than hidden.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deploydeck/internal/delta"
	"deploydeck/internal/exec"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// e2eOrgAlias returns the DEPLOYDECK_E2E_ORG alias, skipping the calling test
// when it is unset (same gate as internal/prereq's real-org e2e).
func e2eOrgAlias(t *testing.T) string {
	t.Helper()
	alias := os.Getenv("DEPLOYDECK_E2E_ORG")
	if alias == "" {
		t.Skip("set DEPLOYDECK_E2E_ORG=<alias> to run the real-org validate+report e2e (local only)")
	}
	return alias
}

// e2eOrgFixtureDir is the committed Salesforce fixture, two levels up.
const e2eOrgFixtureDir = "../../test-e2e-org"

// runGitOrg runs a git command for test setup, failing on error.
func runGitOrg(t *testing.T, runner exec.Runner, dir string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "git",
		Args: args,
		Dir:  dir,
		Env: []string{
			"GIT_EDITOR=true",
			"GIT_TERMINAL_PROMPT=0",
			"GIT_PAGER=cat",
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=commit.gpgsign",
			"GIT_CONFIG_VALUE_0=false",
		},
	})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("git %v failed: err=%v exit=%d stderr=%s", args, err, result.ExitCode, result.Stderr)
	}
}

// copyTree copies a file or directory tree from src to dst without mutating
// the committed fixture.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

// seedValidateRepo builds a temp SFDX project from the real test-e2e-org
// fixture: a baseline exposed as origin/UAT, then a HEAD commit that edits both
// Apex classes (additive only — no deletes, so no destructive validation), so
// the delta package.xml carries AccountService + AccountServiceTest to validate.
func seedValidateRepo(t *testing.T) (dir string, runner exec.Runner) {
	t.Helper()

	fixture, err := filepath.Abs(e2eOrgFixtureDir)
	if err != nil {
		t.Fatalf("resolve fixture: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture, "force-app")); err != nil {
		t.Skipf("test-e2e-org fixture not found at %s: %v", fixture, err)
	}

	runner = exec.NewOSRunner()
	dir = t.TempDir()

	runGitOrg(t, runner, dir, "init", "-b", "main")
	runGitOrg(t, runner, dir, "config", "user.name", "DeployDeck Test")
	runGitOrg(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")

	copyTree(t, filepath.Join(fixture, "force-app"), filepath.Join(dir, "force-app"))
	copyTree(t, filepath.Join(fixture, "sfdx-project.json"), filepath.Join(dir, "sfdx-project.json"))

	runGitOrg(t, runner, dir, "add", "-A")
	runGitOrg(t, runner, dir, "commit", "-m", "seed: test-e2e-org force-app baseline")

	runGitOrg(t, runner, dir, "branch", "UAT")
	runGitOrg(t, runner, dir, "remote", "add", "origin", dir)
	runGitOrg(t, runner, dir, "update-ref", "refs/remotes/origin/UAT", "refs/heads/UAT")

	classes := filepath.Join(dir, "force-app", "main", "default", "classes")
	for _, name := range []string{"AccountService.cls", "AccountServiceTest.cls"} {
		p := filepath.Join(classes, name)
		existing, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		edited := append(append([]byte{}, existing...), []byte("\n// delta-validation e2e edit\n")...)
		if err := os.WriteFile(p, edited, 0o644); err != nil {
			t.Fatalf("edit %s: %v", name, err)
		}
	}

	runGitOrg(t, runner, dir, "add", "-A")
	runGitOrg(t, runner, dir, "commit", "-m", "edit AccountService + AccountServiceTest (additive delta)")

	return dir, runner
}

func TestE2ERealOrg_ValidateAndReport(t *testing.T) {
	alias := e2eOrgAlias(t)
	dir, runner := seedValidateRepo(t)

	client := salesforce.New(runner)
	deltaSvc := delta.New(runner)
	writer := runs.NewWriter(dir)

	// 1) Real sgd delta over the fixture metadata -> package.xml.
	genCtx, genCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer genCancel()
	outputDir := filepath.Join(dir, ".deploydeck", "manifest", "delta", "E2E-to-UAT")
	deltaResult, err := deltaSvc.Generate(genCtx, delta.Request{
		Dir:        dir,
		From:       "origin/UAT",
		To:         "HEAD",
		OutputDir:  outputDir,
		SourceDirs: []string{"force-app"},
	})
	if err != nil {
		t.Fatalf("delta.Generate against real sgd: %v", err)
	}
	if deltaResult.PackageXMLPath == "" {
		t.Fatal("expected a package.xml from the delta")
	}
	pkgData, _ := os.ReadFile(deltaResult.PackageXMLPath)
	t.Logf("generated package.xml:\n%s", string(pkgData))

	// 2) Real async CheckOnly validate (NEVER a deploy). RunSpecifiedTests with
	// AccountServiceTest (in the package) keeps the tested surface self-contained.
	valCtx, valCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer valCancel()
	validateResult, err := client.ValidateDeploy(valCtx, salesforce.ValidateRequest{
		Dir:          dir,
		ManifestPath: deltaResult.PackageXMLPath,
		TargetOrg:    alias,
		TestLevel:    "RunSpecifiedTests",
		Tests:        []string{"AccountServiceTest"},
	})
	t.Logf("VERBATIM validate --async --json output:\n%s", validateResult.Raw)
	if err != nil {
		t.Fatalf("ValidateDeploy against real org %q: %v", alias, err)
	}
	if validateResult.JobID == "" {
		t.Fatalf("expected a non-empty jobId from async validate; raw:\n%s", validateResult.Raw)
	}
	t.Logf("captured jobId: %s", validateResult.JobID)

	// 3) Persist the run IMMEDIATELY on jobId receipt (HU-010).
	runID := "E2E-to-UAT-" + time.Now().Format("20060102150405")
	runDir, err := writer.Create(runs.Record{
		RunID:     runID,
		Ticket:    "E2E",
		Target:    "UAT",
		Alias:     alias,
		JobID:     validateResult.JobID,
		Status:    "Queued",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, []byte(validateResult.Raw))
	if err != nil {
		t.Fatalf("persisting run: %v", err)
	}
	assertPersistedJobID(t, runDir, validateResult.JobID)

	// 4) Poll the REAL report to a terminal state, persisting each raw report.
	var final salesforce.DeployReport
	deadline := time.Now().Add(15 * time.Minute)
	for {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 60*time.Second)
		report, err := client.ReportDeploy(pollCtx, validateResult.JobID, alias, dir)
		pollCancel()
		if err != nil {
			// Transient poll error: retry within the deadline (HU-011).
			if time.Now().After(deadline) {
				t.Fatalf("report poll kept erroring until timeout: %v", err)
			}
			t.Logf("transient report error, retrying: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if err := writer.AppendReport(runID, report.Status, []byte(report.Raw)); err != nil {
			t.Fatalf("AppendReport: %v", err)
		}
		t.Logf("poll status=%q components=%d/%d(err %d) tests=%d/%d(err %d)",
			report.Status,
			report.NumberComponentsDeployed, report.NumberComponentsTotal, report.NumberComponentErrors,
			report.NumberTestsCompleted, report.NumberTestsTotal, report.NumberTestErrors)
		if salesforce.IsTerminal(report.Status) {
			final = report
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("validation never reached a terminal state before the deadline; last status %q", report.Status)
		}
		time.Sleep(5 * time.Second)
	}

	t.Logf("VERBATIM terminal report --json output:\n%s", final.Raw)

	// 5) Shape confirmation — assertions strong enough to catch a struct-tag
	// mismatch in reportResultEnvelope (do NOT weaken these).
	if final.Status == "" {
		t.Fatalf("terminal report has an empty Status; the result envelope did not decode. Raw:\n%s", final.Raw)
	}
	if !salesforce.IsTerminal(final.Status) {
		t.Fatalf("expected a terminal status, got %q", final.Status)
	}
	// The top-level counter tags must parse: a real validate covers >=1 component.
	if final.NumberComponentsTotal < 1 {
		t.Errorf("NumberComponentsTotal parsed as %d; the top-level counter tags may not match the real JSON. Raw:\n%s", final.NumberComponentsTotal, final.Raw)
	}
	// A report claiming component errors MUST surface the parsed component
	// failures — otherwise the nested details.componentFailures path is wrong.
	if final.NumberComponentErrors > 0 && len(final.ComponentFailures) == 0 {
		t.Errorf("report reports %d component errors but parsed 0 ComponentFailures: details.componentFailures shape mismatch — fix report.go. Raw:\n%s", final.NumberComponentErrors, final.Raw)
	}
	for _, f := range final.ComponentFailures {
		if f.Component == "" && f.Type == "" && f.Message == "" {
			t.Errorf("a parsed ComponentFailure is entirely empty: fullName/componentType/problem tags mismatch. Raw:\n%s", final.Raw)
		}
	}
	// A report claiming test errors MUST surface the parsed test failures.
	if final.NumberTestErrors > 0 && len(final.TestFailures) == 0 {
		t.Errorf("report reports %d test errors but parsed 0 TestFailures: details.runTestResult.failures shape mismatch — fix report.go. Raw:\n%s", final.NumberTestErrors, final.Raw)
	}
	for _, f := range final.TestFailures {
		if f.Class == "" && f.Method == "" && f.Message == "" {
			t.Errorf("a parsed TestFailure is entirely empty: name/methodName/message tags mismatch. Raw:\n%s", final.Raw)
		}
	}

	// The final persisted run.json must reflect the terminal status.
	assertPersistedStatus(t, runDir, final.Status)
}

func assertPersistedJobID(t *testing.T, runDir, jobID string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		t.Fatalf("reading persisted run.json: %v", err)
	}
	var rec runs.Record
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}
	if rec.JobID != jobID {
		t.Errorf("persisted run.json JobID = %q, want %q", rec.JobID, jobID)
	}
	if rec.SchemaVersion != runs.SchemaVersion1 {
		t.Errorf("persisted run.json SchemaVersion = %d, want %d", rec.SchemaVersion, runs.SchemaVersion1)
	}
	if _, err := os.Stat(filepath.Join(runDir, "validate.json")); err != nil {
		t.Errorf("validate.json raw not persisted: %v", err)
	}
}

func assertPersistedStatus(t *testing.T, runDir, status string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		t.Fatalf("reading persisted run.json: %v", err)
	}
	var rec runs.Record
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}
	if rec.Status != status {
		t.Errorf("persisted run.json Status = %q, want terminal %q", rec.Status, status)
	}
	// At least one raw report was appended.
	entries, _ := os.ReadDir(runDir)
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "report-") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one persisted report-<NNN>.json")
	}
}
