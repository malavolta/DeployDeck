package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestDeletedSourceBranchWarning_Integration seeds a ticket whose commit
// is still reachable via UAT (already merged) but whose originating
// feature branch has since been deleted, so branch-name candidate search
// finds nothing while message search still finds the commit — exactly the
// signal HU-002 warns about: "Avisar cuando la rama origen ya no exista y
// la busqueda dependa solo del grep de mensajes."
func TestDeletedSourceBranchWarning_Integration(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-1", "main")
	writeAndCommit(t, runner, dir, "feature.txt", "feature\n", "PROJ-1: add feature")
	runGit(t, runner, dir, "checkout", "main")
	runGit(t, runner, dir, "merge", "--no-ff", "-m", "Merge feature/PROJ-1", "feature/PROJ-1")
	runGit(t, runner, dir, "branch", "-D", "feature/PROJ-1")
	runGit(t, runner, dir, "push", "origin", "main")

	ctx := context.Background()

	matched, err := svc.SearchCommits(ctx, dir, "PROJ-1")
	if err != nil {
		t.Fatalf("SearchCommits: unexpected error: %v", err)
	}
	if len(matched) == 0 {
		t.Fatalf("expected the commit to still be found by message search after the branch was deleted")
	}

	candidates, err := svc.CandidateBranches(ctx, dir, "PROJ-1", config.Config{})
	if err != nil {
		t.Fatalf("CandidateBranches: unexpected error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidate branches (the source branch was deleted), got %+v", candidates)
	}

	if !git.DeletedSourceBranchWarning(matched, candidates) {
		t.Fatalf("expected DeletedSourceBranchWarning to fire: commits found by message but no candidate branch exists")
	}
}

// TestDeletedSourceBranchWarning_TableDriven covers the pure decision
// logic directly, including the "branch still exists" and "no results at
// all" cases where the warning must NOT fire.
func TestDeletedSourceBranchWarning_TableDriven(t *testing.T) {
	someCommit := []git.Commit{{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	someBranch := []git.Branch{{Name: "feature/PROJ-1"}}

	tests := []struct {
		name       string
		matched    []git.Commit
		candidates []git.Branch
		want       bool
	}{
		{name: "commits found but no candidate branch: warn", matched: someCommit, candidates: nil, want: true},
		{name: "commits found and a candidate branch exists: no warning", matched: someCommit, candidates: someBranch, want: false},
		{name: "no commits found at all: no warning (that's the no-results case, not deleted-branch)", matched: nil, candidates: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.DeletedSourceBranchWarning(tt.matched, tt.candidates); got != tt.want {
				t.Fatalf("DeletedSourceBranchWarning() = %v, want %v", got, tt.want)
			}
		})
	}
}
