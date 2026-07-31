package git_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

func TestService_RepoRoot_ResolvesViaRevParse(t *testing.T) {
	dir := newTempRepo(t)
	svc := git.New(exec.NewOSRunner())

	root, err := svc.RepoRoot(context.Background(), dir)
	if err != nil {
		t.Fatalf("expected RepoRoot to succeed inside a git repo, got error: %v", err)
	}

	wantRoot, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for comparison: %v", err)
	}
	gotRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("RepoRoot returned an unresolvable path %q: %v", root, err)
	}

	if gotRoot != wantRoot {
		t.Fatalf("RepoRoot() = %q, want %q", gotRoot, wantRoot)
	}
}

func TestService_RepoRoot_ErrorsOutsideRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}

	dir := t.TempDir() // deliberately NOT a git repo
	svc := git.New(exec.NewOSRunner())

	_, err := svc.RepoRoot(context.Background(), dir)
	if err == nil {
		t.Fatal("expected RepoRoot to error outside a git repository, got nil")
	}
}
