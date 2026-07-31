package prereq_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
)

func TestChecker_CheckGitignore_MissingFile_Blocks(t *testing.T) {
	dir := newTempRepo(t, false)

	checker := &prereq.Checker{Dir: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	check, err := checker.CheckGitignore(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected a missing .gitignore to block, got %+v", check)
	}
	if check.FixCommand == "" {
		t.Fatal("expected a FixCommand offering to add the entry")
	}
}

func TestChecker_CheckGitignore_PresentButMissingEntry_Blocks(t *testing.T) {
	dir := newTempRepo(t, false)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("failed to seed .gitignore: %v", err)
	}

	checker := &prereq.Checker{Dir: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	check, err := checker.CheckGitignore(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected .gitignore without .deploydeck/ to block, got %+v", check)
	}
}

func TestChecker_CheckGitignore_EntryPresent_OK(t *testing.T) {
	dir := newTempRepo(t, false)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n.deploydeck/\n"), 0o644); err != nil {
		t.Fatalf("failed to seed .gitignore: %v", err)
	}

	checker := &prereq.Checker{Dir: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	check, err := checker.CheckGitignore(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected .gitignore with .deploydeck/ present to pass, got %+v", check)
	}
}

func TestChecker_AddGitignoreEntry_AddsEntryAndCheckThenPasses(t *testing.T) {
	dir := newTempRepo(t, false)

	checker := &prereq.Checker{Dir: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	before, err := checker.CheckGitignore(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if before.Status != prereq.StatusBlocking {
		t.Fatalf("expected initial state to block, got %+v", before)
	}

	if err := checker.AddGitignoreEntry(context.Background()); err != nil {
		t.Fatalf("AddGitignoreEntry failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore to exist after AddGitignoreEntry: %v", err)
	}
	if !containsLine(string(data), ".deploydeck/") {
		t.Fatalf("expected .gitignore to contain .deploydeck/, got: %q", data)
	}

	after, err := checker.CheckGitignore(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if after.Status != prereq.StatusOK {
		t.Fatalf("expected the check to pass after AddGitignoreEntry, got %+v", after)
	}
}

func containsLine(content, want string) bool {
	for _, line := range splitLines(content) {
		if line == want {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
