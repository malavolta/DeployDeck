package git_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestService_CherryPick_TextConflict_StopsAndClassifies pins AC2 (tasks
// 10.11/10.12): a conflicting commit stops the flow and the conflicting file
// is surfaced classified by type (text).
func TestService_CherryPick_TextConflict_StopsAndClassifies(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, feature2 := seedConflictingFeature(t, runner, dir)
	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: feature1}},
		{Commit: git.Commit{SHA: feature2}},
	}

	outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if !outcome.State.InProgress {
		t.Fatalf("expected the flow to stop mid-pick (InProgress=true)")
	}
	if len(outcome.State.Unmerged) != 1 || outcome.State.Unmerged[0].Path != "a.cls" {
		t.Fatalf("expected one unmerged conflict on a.cls, got %+v", outcome.State.Unmerged)
	}
	if outcome.State.Unmerged[0].Kind != git.ConflictText {
		t.Errorf("expected a.cls classified as text, got %s", outcome.State.Unmerged[0].Kind)
	}
}

// TestService_CherryPick_ModifyDeleteConflict_KeepOrDelete pins tasks
// 10.13/10.14: a modify/delete conflict is classified as such and resolves
// via an explicit keep (`git add`) or delete (`git rm`) choice.
func TestService_CherryPick_ModifyDeleteConflict_KeepOrDelete(t *testing.T) {
	t.Run("keep the file with git add", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature := seedModifyDeleteConflict(t, runner, dir)
		outcome, err := svc.CherryPick(context.Background(), dir, []git.DiscoveredCommit{{Commit: git.Commit{SHA: feature}}}, true)
		if err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		if len(outcome.State.Unmerged) != 1 || outcome.State.Unmerged[0].Kind != git.ConflictModifyDelete {
			t.Fatalf("expected one modify/delete conflict, got %+v", outcome.State.Unmerged)
		}

		if err := svc.KeepConflictFile(context.Background(), dir, "f.cls"); err != nil {
			t.Fatalf("KeepConflictFile error: %v", err)
		}
		state, _ := svc.RepoState(context.Background(), dir)
		if len(state.Unmerged) != 0 {
			t.Errorf("expected no unmerged after keep, got %+v", state.Unmerged)
		}
		if _, err := os.Stat(filepath.Join(dir, "f.cls")); err != nil {
			t.Errorf("expected f.cls kept on disk after `git add`, stat error: %v", err)
		}
	})

	t.Run("delete the file with git rm", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature := seedModifyDeleteConflict(t, runner, dir)
		if _, err := svc.CherryPick(context.Background(), dir, []git.DiscoveredCommit{{Commit: git.Commit{SHA: feature}}}, true); err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}

		if err := svc.DeleteConflictFile(context.Background(), dir, "f.cls"); err != nil {
			t.Fatalf("DeleteConflictFile error: %v", err)
		}
		state, _ := svc.RepoState(context.Background(), dir)
		if len(state.Unmerged) != 0 {
			t.Errorf("expected no unmerged after delete, got %+v", state.Unmerged)
		}
		if _, err := os.Stat(filepath.Join(dir, "f.cls")); !os.IsNotExist(err) {
			t.Errorf("expected f.cls removed after `git rm`, stat err: %v", err)
		}
	})
}

// TestService_CherryPick_BinaryConflict_TheirsOrOurs pins tasks 10.15/10.16:
// a binary conflict is classified as binary and resolves via a
// `git checkout --theirs`/`--ours` selection.
func TestService_CherryPick_BinaryConflict_TheirsOrOurs(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature, theirs := seedBinaryConflict(t, runner, dir)
	outcome, err := svc.CherryPick(context.Background(), dir, []git.DiscoveredCommit{{Commit: git.Commit{SHA: feature}}}, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if len(outcome.State.Unmerged) != 1 || outcome.State.Unmerged[0].Kind != git.ConflictBinary {
		t.Fatalf("expected one binary conflict on img.png, got %+v", outcome.State.Unmerged)
	}

	// Resolve by taking THEIRS (the incoming feature version).
	if err := svc.ResolveBinaryConflict(context.Background(), dir, "img.png", git.SideTheirs); err != nil {
		t.Fatalf("ResolveBinaryConflict error: %v", err)
	}
	state, _ := svc.RepoState(context.Background(), dir)
	if len(state.Unmerged) != 0 {
		t.Errorf("expected no unmerged after binary resolution, got %+v", state.Unmerged)
	}
	got, err := os.ReadFile(filepath.Join(dir, "img.png"))
	if err != nil {
		t.Fatalf("reading resolved img.png: %v", err)
	}
	if !bytes.Equal(got, theirs) {
		t.Errorf("expected img.png to equal the theirs (feature) bytes after --theirs resolution")
	}
}
