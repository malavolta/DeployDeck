package prereq_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
)

func newFakeCheckerWithGitConfig(t *testing.T, root string, configResult exec.CommandResult) *prereq.Checker {
	t.Helper()
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(root + "\n")})
	fr.When("git", []string{"config", "--get", "commit.gpgsign"}, configResult)
	return &prereq.Checker{Dir: root, Git: git.New(fr), Config: config.Config{}}
}

func TestChecker_CheckGpgSign_Unset_OK(t *testing.T) {
	checker := newFakeCheckerWithGitConfig(t, "/repo", exec.CommandResult{ExitCode: 1})

	check, err := checker.CheckGpgSign(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected unset commit.gpgsign to be OK, got %+v", check)
	}
}

func TestChecker_CheckGpgSign_False_OK(t *testing.T) {
	checker := newFakeCheckerWithGitConfig(t, "/repo", exec.CommandResult{ExitCode: 0, Stdout: []byte("false\n")})

	check, err := checker.CheckGpgSign(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected commit.gpgsign=false to be OK, got %+v", check)
	}
}

func TestChecker_CheckGpgSign_True_WarnsWithoutBlocking(t *testing.T) {
	checker := newFakeCheckerWithGitConfig(t, "/repo", exec.CommandResult{ExitCode: 0, Stdout: []byte("true\n")})

	check, err := checker.CheckGpgSign(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusWarning {
		t.Fatalf("expected commit.gpgsign=true to warn (non-blocking), got %+v", check)
	}
	if check.Status == prereq.StatusBlocking {
		t.Fatal("commit.gpgsign check must never block")
	}
}
