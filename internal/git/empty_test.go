package git

import "testing"

// TestIsEmptyPickState pins the repo-state empty-pick signal: a pick is empty
// exactly when it is in progress with a clean tree and zero unmerged paths.
// A completed pick, a conflicting pick, and (defensively) a stray unmerged
// entry are all NOT empty — so the signal can never be spoofed by an echoed
// commit subject nor co-occur with a real conflict.
func TestIsEmptyPickState(t *testing.T) {
	tests := []struct {
		name  string
		state RepoState
		want  bool
	}{
		{name: "in progress, clean, no unmerged -> empty", state: RepoState{InProgress: true, Clean: true}, want: true},
		{name: "completed pick (not in progress) -> not empty", state: RepoState{InProgress: false, Clean: true}, want: false},
		{name: "in progress with conflict -> not empty", state: RepoState{InProgress: true, Clean: false, Unmerged: []ConflictFile{{Path: "a.cls", Kind: ConflictText}}}, want: false},
		{name: "in progress, clean flag, but unmerged present -> not empty", state: RepoState{InProgress: true, Clean: true, Unmerged: []ConflictFile{{Path: "a.cls", Kind: ConflictText}}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyPickState(tt.state); got != tt.want {
				t.Errorf("isEmptyPickState(%+v) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestOfferPartialBranchCleanup pins the pure cleanup-offer predicate.
func TestOfferPartialBranchCleanup(t *testing.T) {
	if OfferPartialBranchCleanup(0) {
		t.Errorf("no picks applied -> no cleanup offer")
	}
	if !OfferPartialBranchCleanup(1) {
		t.Errorf("one applied pick -> cleanup offer expected")
	}
}
