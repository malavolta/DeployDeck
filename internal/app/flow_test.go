package app

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
)

// pipelineConfig configures INT -> UAT as a two-stage pipeline, so a
// promotion to UAT has a pipeline-order default source (INT) — used to
// exercise SuggestDefaultSource's priority over the current-branch confirm.
func pipelineConfig() config.Config {
	return config.Config{
		Branches: map[string]string{"integration": "INT", "uat": "UAT"},
	}
}

// TestResolveSource is task 2.1 (RED): table-driven coverage of
// resolveSource's tri-state outcome. SuggestDefaultSource wins first;
// resolveNeedsConfirm fires only when it yields nothing, 2+ candidates
// remain, and currentBranch (eligible, non-"HEAD") matches one via
// sourceRefName; otherwise it falls through to the UNMODIFIED
// git.SelectSingleSource. A single deduped candidate always auto-resolves
// ready regardless of currentBranch (task 4.2's "no confirm needed").
func TestResolveSource(t *testing.T) {
	tests := []struct {
		name          string
		candidates    []git.Branch
		cfg           config.Config
		target        string
		currentBranch string
		wantOutcome   resolveOutcome
		wantName      string
	}{
		{
			name:          "zero candidates degrades",
			candidates:    nil,
			cfg:           pipelineConfig(),
			target:        "UAT",
			currentBranch: "feature/PROJ-1",
			wantOutcome:   resolveDegrade,
		},
		{
			name: "pipeline suggestion present resolves ready and ignores currentBranch",
			candidates: []git.Branch{
				{Name: "INT"},
				{Name: "feature/PROJ-1"},
			},
			cfg:           pipelineConfig(),
			target:        "UAT",
			currentBranch: "feature/PROJ-1", // matches a DIFFERENT candidate than INT; suggestion still wins
			wantOutcome:   resolveReady,
			wantName:      "INT",
		},
		{
			name: "no suggestion, currentBranch matches a candidate needs confirm",
			candidates: []git.Branch{
				{Name: "feature/PROJ-1"},
				{Name: "origin/hotfix/PROJ-1"},
			},
			cfg:           config.Config{}, // no pipeline branches configured -> no suggestion
			target:        "UAT",
			currentBranch: "feature/PROJ-1",
			wantOutcome:   resolveNeedsConfirm,
			wantName:      "feature/PROJ-1",
		},
		{
			name: "no suggestion, currentBranch matches a remote-tracking candidate via sourceRefName",
			candidates: []git.Branch{
				{Name: "origin/feature/PROJ-1", Remote: true},
				{Name: "hotfix/PROJ-1"},
			},
			cfg:           config.Config{},
			target:        "UAT",
			currentBranch: "feature/PROJ-1",
			wantOutcome:   resolveNeedsConfirm,
			wantName:      "origin/feature/PROJ-1",
		},
		{
			name: "empty currentBranch degrades (existing behavior, ambiguous candidates)",
			candidates: []git.Branch{
				{Name: "feature/PROJ-1"},
				{Name: "hotfix/PROJ-1"},
			},
			cfg:           config.Config{},
			target:        "UAT",
			currentBranch: "",
			wantOutcome:   resolveDegrade,
		},
		{
			name: "detached HEAD currentBranch degrades",
			candidates: []git.Branch{
				{Name: "feature/PROJ-1"},
				{Name: "hotfix/PROJ-1"},
			},
			cfg:           config.Config{},
			target:        "UAT",
			currentBranch: "HEAD",
			wantOutcome:   resolveDegrade,
		},
		{
			name: "currentBranch not among candidates degrades",
			candidates: []git.Branch{
				{Name: "feature/PROJ-1"},
				{Name: "hotfix/PROJ-1"},
			},
			cfg:           config.Config{},
			target:        "UAT",
			currentBranch: "unrelated-branch",
			wantOutcome:   resolveDegrade,
		},
		{
			name:          "single deduped candidate resolves ready without a currentBranch match",
			candidates:    []git.Branch{{Name: "feature/PROJ-1"}},
			cfg:           config.Config{},
			target:        "UAT",
			currentBranch: "",
			wantOutcome:   resolveReady,
			wantName:      "feature/PROJ-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, outcome := resolveSource(tt.candidates, tt.cfg, tt.target, tt.currentBranch)
			if outcome != tt.wantOutcome {
				t.Fatalf("resolveSource() outcome = %v, want %v (branch=%+v)", outcome, tt.wantOutcome, got)
			}
			if tt.wantOutcome != resolveDegrade && got.Name != tt.wantName {
				t.Fatalf("resolveSource() branch = %+v, want Name=%q", got, tt.wantName)
			}
		})
	}
}
