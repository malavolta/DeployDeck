package git_test

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
)

func discoveredCommit(sha string, merge bool, status git.EquivalenceStatus) git.DiscoveredCommit {
	return git.DiscoveredCommit{
		Commit:      git.Commit{SHA: sha},
		Merge:       merge,
		Equivalence: status,
	}
}

// TestSquashMergeWarning_TableDriven proves the best-effort squash-merge
// courtesy heuristic (HU-002/DEC-001: content equivalence for squash
// merges is not statically detectable): it fires when a ticket's matched
// commits are a MIX of already-applied and still-pending, which is
// atypical for a normal cherry-pick promotion flow (a ticket's commits
// are normally all-pending or all-promoted together) and suggests some
// commits were absorbed into a squashed/rewritten history that
// individual-commit equivalence can no longer see.
func TestSquashMergeWarning_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		in   []git.DiscoveredCommit
		want bool
	}{
		{
			name: "all pending: no warning",
			in: []git.DiscoveredCommit{
				discoveredCommit("a", false, git.NotApplied),
				discoveredCommit("b", false, git.NotApplied),
			},
			want: false,
		},
		{
			name: "all already applied: no warning",
			in: []git.DiscoveredCommit{
				discoveredCommit("a", false, git.AlreadyAppliedBySHA),
				discoveredCommit("b", false, git.EquivalentByCherry),
			},
			want: false,
		},
		{
			name: "mixed applied and pending: warns (squash-merge courtesy signal)",
			in: []git.DiscoveredCommit{
				discoveredCommit("a", false, git.EquivalentByCherry),
				discoveredCommit("b", false, git.NotApplied),
			},
			want: true,
		},
		{
			name: "empty result set: no warning",
			in:   nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.SquashMergeWarning(tt.in); got != tt.want {
				t.Fatalf("SquashMergeWarning() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNoResultsAlternatives_TableDriven proves the actionable-alternatives
// list (manual search, change ticket, select branch directly) is offered
// only when a ticket search finds NOTHING at all, and is empty whenever
// any result (message match or candidate branch) exists.
func TestNoResultsAlternatives_TableDriven(t *testing.T) {
	someCommit := []git.Commit{{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	someBranch := []git.Branch{{Name: "feature/PROJ-1"}}

	tests := []struct {
		name       string
		matched    []git.Commit
		candidates []git.Branch
		wantEmpty  bool
	}{
		{name: "no matches and no candidates: alternatives offered", matched: nil, candidates: nil, wantEmpty: false},
		{name: "matches exist: no alternatives needed", matched: someCommit, candidates: nil, wantEmpty: true},
		{name: "candidates exist: no alternatives needed", matched: nil, candidates: someBranch, wantEmpty: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.NoResultsAlternatives(tt.matched, tt.candidates)
			if tt.wantEmpty && len(got) != 0 {
				t.Fatalf("expected no alternatives, got %v", got)
			}
			if !tt.wantEmpty {
				want := map[string]bool{
					git.AlternativeManualSearch: false,
					git.AlternativeChangeTicket: false,
					git.AlternativeSelectBranch: false,
				}
				if len(got) != len(want) {
					t.Fatalf("expected %d alternatives, got %d: %v", len(want), len(got), got)
				}
				for _, a := range got {
					if _, ok := want[a]; !ok {
						t.Fatalf("unexpected alternative %q in %v", a, got)
					}
					want[a] = true
				}
				for a, seen := range want {
					if !seen {
						t.Fatalf("expected alternative %q to be offered, got %v", a, got)
					}
				}
			}
		})
	}
}
