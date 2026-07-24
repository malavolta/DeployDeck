package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/salesforce"
)

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
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":[{"name":"sfdx-git-delta","version":"5.35.0"}]}`)})
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})

	lockPath := t.TempDir() + "/lock"
	deps := Deps{
		NewChecker: func(dir string) (*prereq.Checker, error) {
			return &prereq.Checker{
				Dir:    dir,
				Git:    git.New(fr),
				SF:     salesforce.New(fr),
				Config: config.Config{},
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
	sfr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":[]}`)})
	sfr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})

	deps := Deps{
		NewChecker: func(dir string) (*prereq.Checker, error) {
			return &prereq.Checker{
				Dir:    dir,
				Git:    git.New(fr),
				SF:     salesforce.New(sfr),
				Config: config.Config{},
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
