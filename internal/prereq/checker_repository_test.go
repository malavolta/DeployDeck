package prereq_test

import (
	"context"
	"testing"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
)

func TestChecker_CheckRepository_OutsideAGitRepo_Blocks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}
	dir := t.TempDir() // no `git init` — not a repository

	checker := &prereq.Checker{Dir: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	checks, err := checker.CheckRepository(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoCheck := findCheck(t, checks, "git repository")
	if repoCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected repository check to block outside a git repo, got %+v", repoCheck)
	}
	if repoCheck.FixCommand == "" {
		t.Fatal("expected a corrective FixCommand for a missing repository")
	}
}

func TestChecker_CheckRepository_MissingOrigin_Blocks(t *testing.T) {
	dir := newTempRepo(t, false)

	checker := &prereq.Checker{Dir: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	checks, err := checker.CheckRepository(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repoCheck := findCheck(t, checks, "git repository")
	if repoCheck.Status != prereq.StatusOK {
		t.Fatalf("expected repository check to pass inside a real repo, got %+v", repoCheck)
	}

	originCheck := findCheck(t, checks, "origin remote")
	if originCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected origin check to block when no origin remote is configured, got %+v", originCheck)
	}
	if originCheck.FixCommand == "" {
		t.Fatal("expected a corrective FixCommand for a missing origin remote")
	}
}

func TestChecker_CheckRepository_WithOrigin_AllOK(t *testing.T) {
	dir := newTempRepo(t, true)

	checker := &prereq.Checker{Dir: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	checks, err := checker.CheckRepository(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{"git repository", "origin remote"} {
		c := findCheck(t, checks, name)
		if c.Status != prereq.StatusOK {
			t.Fatalf("expected %q to be OK, got %+v", name, c)
		}
	}
}
