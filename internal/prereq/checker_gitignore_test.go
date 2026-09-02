package prereq_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
)

func TestChecker_CheckGitignore_MissingFile_Blocks(t *testing.T) {
	dir := newTempRepo(t, false)

	checker := &prereq.Checker{GitRoot: dir, ArtifactsRoot: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

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

	checker := &prereq.Checker{GitRoot: dir, ArtifactsRoot: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

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

	checker := &prereq.Checker{GitRoot: dir, ArtifactsRoot: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

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

	checker := &prereq.Checker{GitRoot: dir, ArtifactsRoot: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

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

// TestChecker_CheckGitignore_Nested is task 2.1 (RED): the gitignore check
// reads the .gitignore governing the ARTIFACTS root, falling back to the
// git root's .gitignore when the entry is not there (directory-resolution
// spec: "Gitignore Check Anchored At The Artifacts Root"; ADR-9's
// two-candidate read).
func TestChecker_CheckGitignore_Nested(t *testing.T) {
	t.Run("entry only in the artifacts-root .gitignore passes", func(t *testing.T) {
		gitRoot, projectDir := newNestedTempRepo(t, false)
		if err := os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(".deploydeck/\n"), 0o644); err != nil {
			t.Fatalf("failed to seed artifacts-root .gitignore: %v", err)
		}

		checker := &prereq.Checker{GitRoot: gitRoot, ArtifactsRoot: projectDir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

		check, err := checker.CheckGitignore(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if check.Status != prereq.StatusOK {
			t.Fatalf("expected the artifacts-root entry alone to pass, got %+v", check)
		}
	})

	t.Run("entry only in the git-root .gitignore passes via fallback", func(t *testing.T) {
		gitRoot, projectDir := newNestedTempRepo(t, false)
		if err := os.WriteFile(filepath.Join(gitRoot, ".gitignore"), []byte(".deploydeck/\n"), 0o644); err != nil {
			t.Fatalf("failed to seed git-root .gitignore: %v", err)
		}

		checker := &prereq.Checker{GitRoot: gitRoot, ArtifactsRoot: projectDir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

		check, err := checker.CheckGitignore(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if check.Status != prereq.StatusOK {
			t.Fatalf("expected the git-root fallback entry to pass, got %+v", check)
		}
	})

	t.Run("entry in neither blocks naming the artifacts path", func(t *testing.T) {
		gitRoot, projectDir := newNestedTempRepo(t, false)

		checker := &prereq.Checker{GitRoot: gitRoot, ArtifactsRoot: projectDir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

		check, err := checker.CheckGitignore(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if check.Status != prereq.StatusBlocking {
			t.Fatalf("expected neither candidate having the entry to block, got %+v", check)
		}
		if !strings.Contains(check.FixCommand, projectDir) {
			t.Fatalf("expected FixCommand %q to name the artifacts path %q", check.FixCommand, projectDir)
		}
	})

	t.Run("AddGitignoreEntry writes the artifacts root and creates no file at the git root", func(t *testing.T) {
		gitRoot, projectDir := newNestedTempRepo(t, false)

		checker := &prereq.Checker{GitRoot: gitRoot, ArtifactsRoot: projectDir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

		if err := checker.AddGitignoreEntry(context.Background()); err != nil {
			t.Fatalf("AddGitignoreEntry failed: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
		if err != nil {
			t.Fatalf("expected the artifacts-root .gitignore to exist: %v", err)
		}
		if !containsLine(string(data), ".deploydeck/") {
			t.Fatalf("expected the artifacts-root .gitignore to contain .deploydeck/, got: %q", data)
		}

		if _, err := os.Stat(filepath.Join(gitRoot, ".gitignore")); !os.IsNotExist(err) {
			t.Fatalf("expected NO .gitignore to be created at the git root, stat error: %v", err)
		}
	})
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
