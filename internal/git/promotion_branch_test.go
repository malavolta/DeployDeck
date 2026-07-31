package git_test

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestRenderBranchName_TableDriven proves branchFormat is rendered via a
// literal token substitution (strings.NewReplacer), NOT Go text/template —
// which would fail parsing "{{ticket}}" as a function node
// (design.md's Interfaces/Contracts decision) — for the default format, a
// custom format, and a user-edited override (HU-005 AC: "Ofrecer editar
// nombre antes de crear" — the caller passes whatever format the user
// confirmed, default or edited, through the same renderer).
func TestRenderBranchName_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		format string
		ticket string
		target string
		want   string
	}{
		{
			name:   "default configured format",
			format: "deploy/{{ticket}}-to-{{target}}",
			ticket: "PROJ-1",
			target: "UAT",
			want:   "deploy/PROJ-1-to-UAT",
		},
		{
			name:   "custom configured format with tokens reordered",
			format: "promo/{{target}}/{{ticket}}",
			ticket: "PROJ-42",
			target: "production",
			want:   "promo/production/PROJ-42",
		},
		{
			name:   "user-edited literal override with no tokens is passed through unchanged",
			format: "PROJ-1-manual-branch-name",
			ticket: "PROJ-1",
			target: "UAT",
			want:   "PROJ-1-manual-branch-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.RenderBranchName(tt.format, tt.ticket, tt.target)
			if got != tt.want {
				t.Errorf("RenderBranchName(%q, %q, %q) = %q, want %q", tt.format, tt.ticket, tt.target, got, tt.want)
			}
		})
	}
}

// TestIsProtectedBranch_TableDriven proves the "protected branch" list is
// config-driven — every branch config.Config.Branches maps an environment
// to (HU-005 AC: "Validar que no se esta actualmente sobre rama protegida
// para modificarla directamente") — not a hardcoded name and not every
// branch in the repo.
func TestIsProtectedBranch_TableDriven(t *testing.T) {
	cfg := config.Config{
		Branches: map[string]string{
			"integration": "main",
			"uat":         "UAT",
		},
	}

	tests := []struct {
		name   string
		branch string
		want   bool
	}{
		{name: "a configured environment branch is protected", branch: "main", want: true},
		{name: "another configured environment branch is protected", branch: "UAT", want: true},
		{name: "an unconfigured feature branch is not protected", branch: "feature/PROJ-1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.IsProtectedBranch(cfg, tt.branch); got != tt.want {
				t.Errorf("IsProtectedBranch(cfg, %q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

// TestRegisterPromotionBranch_TableDriven proves a successfully created
// promotion branch's final name is recorded into the DeploymentPlan (HU-005
// AC: "Dado que la rama se crea correctamente, entonces el plan registra el
// nombre final"), preserving every field the plan already carried from
// earlier stages (HU-003's Ticket/SelectedCommits, HU-004's
// TargetBranch/SandboxAlias/TestLevel) — the same gate-then-persist shape
// ConfirmTargetSelection uses for HU-004.
func TestRegisterPromotionBranch_TableDriven(t *testing.T) {
	basePlan := git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SandboxAlias: "UAT_SANDBOX",
		TestLevel:    "RunLocalTests",
	}

	tests := []struct {
		name       string
		branchName string
	}{
		{name: "default-format branch name", branchName: "deploy/PROJ-1-to-UAT"},
		{name: "custom-format branch name", branchName: "promo/UAT/PROJ-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.RegisterPromotionBranch(basePlan, tt.branchName)

			if got.PromotionBranch != tt.branchName {
				t.Errorf("PromotionBranch = %q, want %q", got.PromotionBranch, tt.branchName)
			}
			if got.Ticket != basePlan.Ticket || got.TargetBranch != basePlan.TargetBranch || got.SandboxAlias != basePlan.SandboxAlias || got.TestLevel != basePlan.TestLevel {
				t.Errorf("expected prior plan fields preserved, got %+v", got)
			}
		})
	}
}
