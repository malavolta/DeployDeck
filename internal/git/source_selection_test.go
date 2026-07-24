package git_test

import (
	"errors"
	"testing"

	"deploydeck/internal/git"
)

func TestSelectSingleSource_TableDriven(t *testing.T) {
	feature1 := git.Branch{Name: "feature/PROJ-1"}
	feature2 := git.Branch{Name: "hotfix/PROJ-1", Remote: true}

	tests := []struct {
		name       string
		candidates []git.Branch
		selected   string
		wantBranch git.Branch
		wantErr    error
	}{
		{
			name:       "no candidates blocks with ErrNoSourceBranch",
			candidates: nil,
			selected:   "",
			wantErr:    git.ErrNoSourceBranch,
		},
		{
			name:       "exactly one candidate auto-resolves without requiring a selection",
			candidates: []git.Branch{feature1},
			selected:   "",
			wantBranch: feature1,
		},
		{
			name:       "more than one candidate with no explicit choice blocks continuing",
			candidates: []git.Branch{feature1, feature2},
			selected:   "",
			wantErr:    git.ErrMultipleSourceBranches,
		},
		{
			name:       "more than one candidate with an explicit choice resolves to it",
			candidates: []git.Branch{feature1, feature2},
			selected:   "hotfix/PROJ-1",
			wantBranch: feature2,
		},
		{
			name:       "a selection naming a branch outside the candidate pool is rejected",
			candidates: []git.Branch{feature1, feature2},
			selected:   "does-not-exist",
			wantErr:    git.ErrUnknownSourceBranch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := git.SelectSingleSource(tt.candidates, tt.selected)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SelectSingleSource() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SelectSingleSource() unexpected error: %v", err)
			}
			if got != tt.wantBranch {
				t.Fatalf("SelectSingleSource() = %+v, want %+v", got, tt.wantBranch)
			}
		})
	}
}
