package prereq_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
)

func newFakeCheckerWithRepoRoot(t *testing.T, root string) *prereq.Checker {
	t.Helper()
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(root + "\n"),
	})
	return &prereq.Checker{Dir: root, Git: git.New(fr), Config: config.Config{}}
}

func TestChecker_CheckHooks_NoHooksDirectory_OK(t *testing.T) {
	root := t.TempDir()
	checker := newFakeCheckerWithRepoRoot(t, root)

	check, err := checker.CheckHooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected no hooks directory to be OK, got %+v", check)
	}
}

func TestChecker_CheckHooks_OnlySampleHooks_OK(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit.sample"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("failed to seed sample hook: %v", err)
	}

	checker := newFakeCheckerWithRepoRoot(t, root)

	check, err := checker.CheckHooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected only .sample hooks to be OK (never installed), got %+v", check)
	}
}

func TestChecker_CheckHooks_InterferingHookPresent_WarnsWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "post-checkout"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("failed to seed post-checkout hook: %v", err)
	}

	checker := newFakeCheckerWithRepoRoot(t, root)

	check, err := checker.CheckHooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusWarning {
		t.Fatalf("expected an interfering hook to warn (informative, non-blocking), got %+v", check)
	}
	if check.Status == prereq.StatusBlocking {
		t.Fatal("git hooks detection must never block")
	}
}

func TestDetectInterferingHooks_MultipleHooks_ReturnsAllNonSampleMatches(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"pre-commit", "post-merge", "irrelevant-script", "pre-push.sample"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("failed to seed %s: %v", name, err)
		}
	}

	got, err := prereq.DetectInterferingHooks(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 interfering hooks (pre-commit, post-merge), got %d: %v", len(got), got)
	}
	found := map[string]bool{}
	for _, name := range got {
		found[name] = true
	}
	if !found["pre-commit"] || !found["post-merge"] {
		t.Fatalf("expected pre-commit and post-merge to be detected, got %v", got)
	}
	if found["irrelevant-script"] || found["pre-push.sample"] {
		t.Fatalf("expected non-hook and .sample files to be excluded, got %v", got)
	}
}
