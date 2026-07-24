package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

func TestService_Status_DistinguishesCleanVsDirty(t *testing.T) {
	dir := newTempRepo(t)
	svc := git.New(exec.NewOSRunner())

	clean, err := svc.Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("expected Status to succeed on a clean repo, got error: %v", err)
	}
	if !clean.Clean {
		t.Fatalf("expected freshly committed repo to be Clean, got %+v", clean)
	}

	untrackedPath := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(untrackedPath, []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("failed to write untracked file: %v", err)
	}

	dirty, err := svc.Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("expected Status to succeed on a dirty repo, got error: %v", err)
	}
	if dirty.Clean {
		t.Fatalf("expected repo with an untracked file to be dirty, got %+v", dirty)
	}
}
