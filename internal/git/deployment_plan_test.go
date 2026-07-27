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

// TestRegisterDeltaArtifacts_SetsPathsPreservesOtherFields proves
// RegisterDeltaArtifacts (HU-007 AC: "Guardar paths generados en
// DeploymentPlan") sets PackageXMLPath/DestructiveChangesPath while
// preserving every other field already on plan, mirroring
// RegisterPromotionBranch's gate-then-persist shape.
func TestRegisterDeltaArtifacts_SetsPathsPreservesOtherFields(t *testing.T) {
	plan := git.DeploymentPlan{
		Ticket:          "PROJ-1",
		SelectedCommits: []git.DiscoveredCommit{{Commit: git.Commit{SHA: "a"}}},
		TargetBranch:    "UAT",
		SandboxAlias:    "UAT_SANDBOX",
		TestLevel:       "RunLocalTests",
		PromotionBranch: "deploy/PROJ-1-to-UAT",
	}

	wantPackage := "/repo/.deploydeck/manifest/delta/PROJ-1-to-UAT/package/package.xml"
	wantDestructive := "/repo/.deploydeck/manifest/delta/PROJ-1-to-UAT/destructiveChanges/destructiveChanges.xml"

	got := git.RegisterDeltaArtifacts(plan, wantPackage, wantDestructive)

	if got.PackageXMLPath != wantPackage {
		t.Errorf("PackageXMLPath = %q, want %q", got.PackageXMLPath, wantPackage)
	}
	if got.DestructiveChangesPath != wantDestructive {
		t.Errorf("DestructiveChangesPath = %q, want %q", got.DestructiveChangesPath, wantDestructive)
	}

	// Every other field already on plan must survive untouched.
	if got.Ticket != plan.Ticket {
		t.Errorf("Ticket = %q, want %q (preserved)", got.Ticket, plan.Ticket)
	}
	if got.TargetBranch != plan.TargetBranch || got.SandboxAlias != plan.SandboxAlias || got.TestLevel != plan.TestLevel {
		t.Errorf("target/sandbox/testLevel fields not preserved: got %+v, want fields from %+v", got, plan)
	}
	if got.PromotionBranch != plan.PromotionBranch {
		t.Errorf("PromotionBranch = %q, want %q (preserved)", got.PromotionBranch, plan.PromotionBranch)
	}
	if len(got.SelectedCommits) != len(plan.SelectedCommits) || got.SelectedCommits[0].SHA != plan.SelectedCommits[0].SHA {
		t.Errorf("SelectedCommits not preserved: got %+v, want %+v", got.SelectedCommits, plan.SelectedCommits)
	}
}

// TestRegisterDeltaArtifacts_DestructiveChangesPathOptional proves an empty
// destructiveChangesPath (no deleted metadata in the diff) round-trips as
// an empty DestructiveChangesPath, while PackageXMLPath is always set.
func TestRegisterDeltaArtifacts_DestructiveChangesPathOptional(t *testing.T) {
	got := git.RegisterDeltaArtifacts(git.DeploymentPlan{Ticket: "PROJ-1"}, "/repo/.deploydeck/manifest/delta/PROJ-1-to-UAT/package/package.xml", "")

	if got.PackageXMLPath == "" {
		t.Error("expected a non-empty PackageXMLPath")
	}
	if got.DestructiveChangesPath != "" {
		t.Errorf("expected empty DestructiveChangesPath when no destructive changes were generated, got %q", got.DestructiveChangesPath)
	}
}
