package prereq_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func TestChecker_Check_AllPrerequisitesPass_AllCriticalChecksOK(t *testing.T) {
	dir := newTempRepo(t, true)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".deploydeck/\n"), 0o644); err != nil {
		t.Fatalf("failed to seed .gitignore: %v", err)
	}
	runner := exec.NewOSRunner()
	runGit(t, runner, dir, "add", ".gitignore")
	runGit(t, runner, dir, "commit", "-m", "chore: ignore .deploydeck/")

	sfRunner := exec.NewFakeRunner()
	sfRunner.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	sfRunner.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`[{"name":"sfdx-git-delta","version":"5.35.0","children":[]}]`)})
	sfRunner.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"sandboxes":[{"alias":"uat","username":"a@b.com"}]}}`)})

	lockPath := filepath.Join(t.TempDir(), "lock")
	self := prereq.LockInfo{PID: 4321, PName: "deploydeck", Host: "test-host"}
	lock := prereq.NewLock(lockPath, self, fakeProber{alive: func(int, time.Time) bool { return false }})

	// loadMinimalConfig (config-validation-wiring R3), not a bare struct
	// literal: the new "config file" check requires a Load-produced (i.e.
	// defaulted) Config, and this test's flagship assertion — ZERO checks
	// blocking — is exactly where that invariant must hold.
	cfg, path := loadMinimalConfig(t, "sandboxes:\n  UAT:\n    alias: uat\n    testLevel: RunLocalTests\n")

	checker := &prereq.Checker{
		GitRoot:       dir,
		ArtifactsRoot: dir,
		Git:           git.New(exec.NewOSRunner()),
		SF:            salesforce.New(sfRunner),
		Lock:          lock,
		Config:        cfg,
		ConfigPath:    path,
	}

	checks, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(checks) == 0 {
		t.Fatal("expected a non-empty prerequisite report")
	}
	for _, c := range checks {
		if c.Status == prereq.StatusBlocking {
			t.Fatalf("expected every check to pass in a correctly configured environment, but %q blocked: %+v", c.Name, c)
		}
	}

	// Every critical check named by HU-001's AC must be present and OK.
	for _, name := range []string{"config file", "git version", "sf version", "sfdx-git-delta version", "git repository", "origin remote", "working tree", ".deploydeck/ gitignore", "sandbox alias (UAT)", "instance lock"} {
		c := findCheck(t, checks, name)
		if c.Status != prereq.StatusOK {
			t.Fatalf("expected %q to be OK, got %+v", name, c)
		}
	}
}

// TestChecker_Check_ConfigFileRunsFirstBeforeCheckVersions is
// config-validation-wiring task 3.1 (RED): the "config file" entry must
// appear at index 0, before every CheckVersions entry — cfg.MinVersions
// feeds CheckVersions and a malformed config must surface before the check
// that consumes it (design.md ADR-2, prereq-check spec: "Config check
// runs before CheckVersions").
func TestChecker_Check_ConfigFileRunsFirstBeforeCheckVersions(t *testing.T) {
	cfg, path := loadMinimalConfig(t, "branches:\n  integration: INT\n")

	gitRunner := exec.NewFakeRunner() // no canned responses: git checks degrade to blocking, irrelevant to ordering
	sfRunner := exec.NewFakeRunner()
	sfRunner.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	sfRunner.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`[{"name":"sfdx-git-delta","version":"5.35.0","children":[]}]`)})
	sfRunner.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})

	checker := &prereq.Checker{
		GitRoot:       "/repo",
		ArtifactsRoot: "/repo",
		Git:           git.New(gitRunner),
		SF:            salesforce.New(sfRunner),
		Config:        cfg,
		ConfigPath:    path,
	}

	checks, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if len(checks) == 0 {
		t.Fatal("expected a non-empty report")
	}
	if checks[0].Name != "config file" {
		t.Fatalf("checks[0].Name = %q, want %q — config file must run FIRST", checks[0].Name, "config file")
	}

	// Every CheckVersions entry (git/sf/sfdx-git-delta) must come strictly
	// AFTER index 0 — reordering is otherwise free (Check() has no
	// status-based short-circuit, and findCheck locates by name, never
	// index), so this is the one ordering guarantee worth pinning.
	versionNames := map[string]bool{"git version": true, "sf version": true, "sfdx-git-delta version": true, "sfdx-git-delta plugin": true}
	for i, c := range checks {
		if versionNames[c.Name] && i == 0 {
			t.Fatalf("%q must not be at index 0 — config file must run before it", c.Name)
		}
	}
}

func TestChecker_Check_MissingGitBinary_BlocksWithFixCommand(t *testing.T) {
	gitRunner := exec.NewFakeRunner() // no canned responses: every git invocation "fails to start"

	sfRunner := exec.NewFakeRunner()
	sfRunner.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	sfRunner.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`[{"name":"sfdx-git-delta","version":"5.35.0","children":[]}]`)})
	sfRunner.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})

	checker := &prereq.Checker{
		GitRoot:       "/repo",
		ArtifactsRoot: "/repo",
		Git:           git.New(gitRunner),
		SF:            salesforce.New(sfRunner),
		Config:        config.Config{},
	}

	checks, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("expected Check() to render a full report even when git is unavailable, got error: %v", err)
	}

	gitCheck := findCheck(t, checks, "git version")
	if gitCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected a missing git binary to block, got %+v", gitCheck)
	}
	if gitCheck.FixCommand == "" {
		t.Fatal("expected a corrective FixCommand when git is unavailable")
	}

	repoCheck := findCheck(t, checks, "git repository")
	if repoCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected repo-membership to also block without a working git, got %+v", repoCheck)
	}
}
