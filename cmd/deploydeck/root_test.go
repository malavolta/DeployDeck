package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/salesforce"
	"github.com/malavolta/DeployDeck/internal/version"
)

// TestNewRootCmd_NoSubcommand_LaunchesTUI proves that running `deploydeck`
// with no subcommand launches the Bubble Tea TUI (via the injected RunTUI),
// resolving the working directory to run it in.
func TestNewRootCmd_NoSubcommand_LaunchesTUI(t *testing.T) {
	var launched bool
	var launchedDir string
	deps := Deps{
		RunTUI: func(dir string) error {
			launched = true
			launchedDir = dir
			return nil
		},
		NewChecker: func(string) (*prereq.Checker, error) {
			t.Fatal("bare `deploydeck` must launch the TUI, not run doctor")
			return nil, nil
		},
	}

	cmd := newRootCmd(deps)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bare deploydeck should launch the TUI cleanly, got: %v", err)
	}
	if !launched {
		t.Fatal("bare deploydeck did not launch the TUI")
	}
	if launchedDir == "" {
		t.Errorf("TUI should be launched with a resolved working directory")
	}
}

// TestNewRootCmd_Doctor_DoesNotLaunchTUI proves the doctor subcommand routes
// to the checker, never the TUI.
func TestNewRootCmd_Doctor_DoesNotLaunchTUI(t *testing.T) {
	var launched bool
	deps := Deps{
		RunTUI: func(string) error { launched = true; return nil },
		NewChecker: func(string) (*prereq.Checker, error) {
			return nil, errors.New("checker unavailable")
		},
	}

	cmd := newRootCmd(deps)
	cmd.SetArgs([]string{"doctor"})
	_ = cmd.Execute() // error is expected (checker unavailable); routing is what matters

	if launched {
		t.Fatal("`deploydeck doctor` must not launch the TUI")
	}
}

func TestNewRootCmd_RegistersDoctorSubcommand(t *testing.T) {
	cmd := newRootCmd(Deps{})

	doctorCmd, _, err := cmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("expected root command to register a doctor subcommand, got error: %v", err)
	}
	if doctorCmd.Name() != "doctor" {
		t.Fatalf("expected doctor subcommand name %q, got %q", "doctor", doctorCmd.Name())
	}
}

// TestNewRootCmd_Version_Default proves that with no ldflags injection
// (internal/version.Version at its "dev" default), `deploydeck --version`
// reports "dev" instead of Cobra's default inert output (release-versioning:
// CLI Version Exposure).
func TestNewRootCmd_Version_Default(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd(Deps{})
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version should exit cleanly, got: %v", err)
	}
	if !strings.Contains(buf.String(), "dev") {
		t.Fatalf("--version output = %q, want it to contain %q", buf.String(), "dev")
	}
}

// TestNewRootCmd_Version_Injected proves that root.Version reads
// internal/version.String() at construction time: setting Version before
// newRootCmd is called flows through to `deploydeck --version` output. This
// mutates the package-level version.Version global, so it must not run with
// t.Parallel.
func TestNewRootCmd_Version_Injected(t *testing.T) {
	orig := version.Version
	defer func() { version.Version = orig }()
	version.Version = "9.9.9"

	var buf bytes.Buffer
	cmd := newRootCmd(Deps{})
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version should exit cleanly, got: %v", err)
	}
	if !strings.Contains(buf.String(), "9.9.9") {
		t.Fatalf("--version output = %q, want it to contain %q", buf.String(), "9.9.9")
	}
}

// fakeAliveProber never reports a lock owner as alive, so doctor's own
// acquire in these tests always succeeds.
type fakeAliveProber struct{}

func (fakeAliveProber) Alive(int, time.Time) bool { return false }

func TestNewRootCmd_Doctor_AllChecksPass_ExitsZero(t *testing.T) {
	// CheckGitignore/CheckHooks read the filesystem directly at the
	// RepoRoot path, so the canned "repo root" must be a real directory,
	// not just a string. Seed a real .gitignore there.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".deploydeck/\n"), 0o644); err != nil {
		t.Fatalf("failed to seed .gitignore: %v", err)
	}

	fr := exec.NewFakeRunner()
	fr.When("git", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("git version 2.43.0\n")})
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(root + "\n")})
	fr.When("git", []string{"remote", "get-url", "origin"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("git@example.com:org/repo.git\n")})
	fr.When("git", []string{"status", "--porcelain"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("")})
	fr.When("git", []string{"config", "--get", "commit.gpgsign"}, exec.CommandResult{ExitCode: 1})
	fr.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`[{"name":"sfdx-git-delta","version":"5.35.0","children":[]}]`)})
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})

	lockPath := t.TempDir() + "/lock"
	deps := Deps{
		// The dir param is deliberately IGNORED: every git-backed check
		// resolves through the FakeRunner (Dir-blind, keyed on Name+Args
		// only), but CheckGitignore is now pure filesystem (ADR-9) and
		// reads ArtifactsRoot directly — it must be the seeded `root`
		// regardless of the real process cwd RunE's os.Getwd() happens to
		// report during `go test`.
		NewChecker: func(string) (*prereq.Checker, error) {
			return &prereq.Checker{
				GitRoot:       root,
				ArtifactsRoot: root,
				Git:           git.New(fr),
				SF:            salesforce.New(fr),
				// baselineConfig() (config-validation-wiring R2), not a
				// bare config.Config{} literal: the new "config file"
				// check blocks on pollIntervalSeconds otherwise, and this
				// test's whole point is that EVERY check passes.
				Config: baselineConfig(),
				Lock:   prereq.NewLock(lockPath, prereq.LockInfo{PID: 1, PName: "deploydeck"}, fakeAliveProber{}),
			}, nil
		},
	}

	cmd := newRootCmd(deps)
	cmd.SetArgs([]string{"doctor"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected doctor to exit 0 (nil error) when every check passes, got: %v", err)
	}
}

func TestNewRootCmd_Doctor_BlockingCheck_ExitsNonZero(t *testing.T) {
	fr := exec.NewFakeRunner() // no canned git responses: every git invocation "fails to start"
	sfr := exec.NewFakeRunner()
	sfr.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	sfr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`[]`)})
	sfr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})

	deps := Deps{
		NewChecker: func(dir string) (*prereq.Checker, error) {
			return &prereq.Checker{
				GitRoot:       dir,
				ArtifactsRoot: dir,
				Git:           git.New(fr),
				SF:            salesforce.New(sfr),
				Config:        config.Config{},
			}, nil
		},
	}

	cmd := newRootCmd(deps)
	cmd.SetArgs([]string{"doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected doctor to exit non-zero (a non-nil error) when a blocking check fails")
	}
}
