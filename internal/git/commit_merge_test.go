package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestCommit_IsMerge_TableDriven proves merge-commit detection is derived
// purely from Parents (parsed from canned `git log`/`git rev-list`
// --format output carrying %P), matching HU-002: "Un commit es merge si
// tiene mas de un parent."
func TestCommit_IsMerge_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		parents []string
		want    bool
	}{
		{name: "root commit (zero parents) is not a merge", parents: nil, want: false},
		{name: "normal commit (one parent) is not a merge", parents: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, want: false},
		{
			name: "two-parent commit is a merge",
			parents: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			want: true,
		},
		{
			name: "octopus merge (three parents) is still a merge",
			parents: []string{
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"cccccccccccccccccccccccccccccccccccccccc",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := git.Commit{SHA: "deadbeef", Parents: tt.parents}
			if got := c.IsMerge(); got != tt.want {
				t.Fatalf("IsMerge() with parents %v = %v, want %v", tt.parents, got, tt.want)
			}
		})
	}
}

// TestParseCommitLog_ParsesMultipleParentsFromLogOutput proves the %P
// field of a real `git log --format` line (space-separated parent SHAs
// for a merge commit) is parsed into Commit.Parents correctly — this is
// exercised through the package-level parser used by both SearchCommits
// and CommitsInRange, via the exported Commit constructor path (a merge
// commit line as git would actually emit it).
func TestParseCommitLog_ParsesMultipleParentsFromLogOutput(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("/repo\n")})
	runner.When("git", []string{"log", "--all", "--grep", "PROJ-1", "--format=%H%x09%h%x09%an%x09%aI%x09%P%x09%s"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(
			"mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm\tmmmmmmm\tRelease Bot\t2026-03-01T00:00:00Z\tpppppppppppppppppppppppppppppppppppppppp qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq\tPROJ-1: Merge branch 'feature/PROJ-1' into UAT\n",
		),
	})

	svc := git.New(runner)
	commits, err := svc.SearchCommits(context.Background(), "/repo", "PROJ-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if !commits[0].IsMerge() {
		t.Fatalf("expected the parsed commit to be flagged as a merge, got Parents=%v", commits[0].Parents)
	}
	if len(commits[0].Parents) != 2 {
		t.Fatalf("expected 2 parents parsed, got %d: %v", len(commits[0].Parents), commits[0].Parents)
	}
}
