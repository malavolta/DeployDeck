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

func TestChecker_CheckWorkingTree_Clean_OK(t *testing.T) {
	dir := newTempRepo(t, false)

	checker := &prereq.Checker{GitRoot: dir, ArtifactsRoot: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	check, err := checker.CheckWorkingTree(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected clean working tree to be OK, got %+v", check)
	}
}

func TestChecker_CheckWorkingTree_Dirty_Blocks(t *testing.T) {
	dir := newTempRepo(t, false)
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatalf("failed to seed an uncommitted file: %v", err)
	}

	checker := &prereq.Checker{GitRoot: dir, ArtifactsRoot: dir, Git: git.New(exec.NewOSRunner()), Config: config.Config{}}

	check, err := checker.CheckWorkingTree(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected a dirty working tree to block, got %+v", check)
	}
}
