package prereq_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/salesforce"
)

func newFakeCheckerDeps(fr *exec.FakeRunner, cfg config.Config) *prereq.Checker {
	return &prereq.Checker{
		Dir:    "/repo",
		Git:    git.New(fr),
		SF:     salesforce.New(fr),
		Config: cfg,
	}
}

func TestChecker_CheckVersions_AllAboveMinimum_AllOK(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("git version 2.43.0\n")})
	fr.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":[{"name":"sfdx-git-delta","version":"5.35.0"}]}`)})

	cfg := config.Config{MinVersions: map[string]string{"git": "2.40.0", "sf": "2.60.0", "sfdx-git-delta": "5.30.0"}}
	checker := newFakeCheckerDeps(fr, cfg)

	checks, err := checker.CheckVersions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks (git, sf, sfdx-git-delta), got %d: %+v", len(checks), checks)
	}
	for _, c := range checks {
		if c.Status != prereq.StatusOK {
			t.Fatalf("expected check %q to be OK, got %+v", c.Name, c)
		}
	}
}

func TestChecker_CheckVersions_GitBelowMinimum_Blocks(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("git version 2.10.0\n")})
	fr.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":[{"name":"sfdx-git-delta","version":"5.35.0"}]}`)})

	cfg := config.Config{MinVersions: map[string]string{"git": "2.40.0"}}
	checker := newFakeCheckerDeps(fr, cfg)

	checks, err := checker.CheckVersions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitCheck := findCheck(t, checks, "git version")
	if gitCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected git version check to block, got %+v", gitCheck)
	}
	if !strings.Contains(gitCheck.Detail, "2.10.0") || !strings.Contains(gitCheck.Detail, "2.40.0") {
		t.Fatalf("expected detail to show current and required version, got %q", gitCheck.Detail)
	}
	if gitCheck.FixCommand == "" {
		t.Fatal("expected a non-empty FixCommand for a below-minimum git version")
	}
}

func TestChecker_CheckVersions_DeltaPluginBelowMinimum_BlocksWithFixCommand(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("git version 2.43.0\n")})
	fr.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":[{"name":"sfdx-git-delta","version":"5.10.0"}]}`)})

	cfg := config.Config{MinVersions: map[string]string{"sfdx-git-delta": "5.30.0"}}
	checker := newFakeCheckerDeps(fr, cfg)

	checks, err := checker.CheckVersions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deltaCheck := findCheck(t, checks, "sfdx-git-delta version")
	if deltaCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected sfdx-git-delta version check to block, got %+v", deltaCheck)
	}
	if !strings.Contains(deltaCheck.Detail, "5.10.0") || !strings.Contains(deltaCheck.Detail, "5.30.0") {
		t.Fatalf("expected detail to show current and required version, got %q", deltaCheck.Detail)
	}
	if deltaCheck.FixCommand == "" {
		t.Fatal("expected a non-empty FixCommand for a below-minimum sfdx-git-delta version")
	}
}

func TestChecker_CheckVersions_DeltaPluginMissing_ShowsInstallFixCommand(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("git version 2.43.0\n")})
	fr.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":[]}`)})

	checker := newFakeCheckerDeps(fr, config.Config{})

	checks, err := checker.CheckVersions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deltaCheck := findCheck(t, checks, "sfdx-git-delta plugin")
	if deltaCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected missing sfdx-git-delta plugin to block, got %+v", deltaCheck)
	}
	if !strings.Contains(deltaCheck.FixCommand, "install") {
		t.Fatalf("expected FixCommand to suggest installing the plugin, got %q", deltaCheck.FixCommand)
	}
}

func findCheck(t *testing.T, checks []prereq.PrereqCheck, name string) prereq.PrereqCheck {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("expected a check named %q, got: %+v", name, checks)
	return prereq.PrereqCheck{}
}
