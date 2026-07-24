package git_test

import (
	"errors"
	"testing"

	"deploydeck/internal/git"
)

// TestGenerateDeploymentPlan_TableDriven proves confirming a valid
// non-empty selection produces a preliminary DeploymentPlan carrying
// exactly the selected commits, in their original (topo) order, while an
// empty selection is blocked the same way ValidateSelection blocks it
// (HU-003 AC: "el usuario confirma una seleccion valida, entonces se
// genera un DeploymentPlan preliminar").
func TestGenerateDeploymentPlan_TableDriven(t *testing.T) {
	commitA := git.DiscoveredCommit{Commit: git.Commit{SHA: "a", Subject: "PROJ-1: first"}}
	commitB := git.DiscoveredCommit{Commit: git.Commit{SHA: "b", Subject: "PROJ-1: second"}}
	commitC := git.DiscoveredCommit{Commit: git.Commit{SHA: "c", Subject: "Merge branch"}, Merge: true}

	tests := []struct {
		name    string
		ticket  string
		items   []git.CommitSelectionItem
		want    git.DeploymentPlan
		wantErr error
	}{
		{
			name:   "valid non-empty selection generates a preliminary plan preserving order",
			ticket: "PROJ-1",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: commitA, Selected: true},
				{DiscoveredCommit: commitB, Selected: true},
				{DiscoveredCommit: commitC, Selected: false, Disabled: true, Reason: git.ReasonMergeCommit},
			},
			want: git.DeploymentPlan{
				Ticket:          "PROJ-1",
				SelectedCommits: []git.DiscoveredCommit{commitA, commitB},
			},
		},
		{
			name:   "a partial selection includes only the selected commits, in order",
			ticket: "PROJ-1",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: commitA, Selected: false},
				{DiscoveredCommit: commitB, Selected: true},
			},
			want: git.DeploymentPlan{
				Ticket:          "PROJ-1",
				SelectedCommits: []git.DiscoveredCommit{commitB},
			},
		},
		{
			name:   "empty selection is blocked, no plan generated",
			ticket: "PROJ-1",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: commitA, Selected: false},
				{DiscoveredCommit: commitC, Selected: false, Disabled: true, Reason: git.ReasonMergeCommit},
			},
			wantErr: git.ErrEmptySelection,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := git.GenerateDeploymentPlan(tt.ticket, tt.items)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("GenerateDeploymentPlan() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateDeploymentPlan() unexpected error: %v", err)
			}
			if got.Ticket != tt.want.Ticket {
				t.Errorf("Ticket = %q, want %q", got.Ticket, tt.want.Ticket)
			}
			if len(got.SelectedCommits) != len(tt.want.SelectedCommits) {
				t.Fatalf("SelectedCommits = %+v, want %+v", got.SelectedCommits, tt.want.SelectedCommits)
			}
			for i := range got.SelectedCommits {
				if got.SelectedCommits[i].SHA != tt.want.SelectedCommits[i].SHA {
					t.Errorf("SelectedCommits[%d].SHA = %q, want %q", i, got.SelectedCommits[i].SHA, tt.want.SelectedCommits[i].SHA)
				}
			}
		})
	}
}
