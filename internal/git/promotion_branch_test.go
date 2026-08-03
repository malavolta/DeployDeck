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

// TestPromotionBranchMatcher_TableDriven is the D1 over-exclusion
// remediation (RED first): proves the promotion-branch exclusion matcher
// matches the FULL rendered shape of cfg.BranchFormat, not just its literal
// prefix — a prior prefix-only match wrongly excluded legitimate source
// branches that merely started with the same literal text (e.g.
// "deploy/DEMO-2", which lacks the "-to-<target>" suffix and is NOT a tool
// promotion branch). It also proves the nil-matcher guard: an empty format,
// or a format with no literal content once its tokens are stripped (a
// degenerate pattern that would match virtually any single-segment branch
// name), skips filtering entirely rather than dropping every candidate.
func TestPromotionBranchMatcher_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		wantNil    bool
		matches    []string
		nonMatches []string
	}{
		{
			name:   "default configured format matches the full promotion shape only",
			format: config.DefaultBranchFormat,
			// A target segment absorbing a hyphenated suffix (e.g.
			// "INT-extra") is a legitimate single-segment target name under
			// "[^/]+" and correctly still matches the full shape.
			matches:    []string{"deploy/DEMO-2-to-INT", "deploy/PROJ-1-to-UAT", "deploy/DEMO-2-to-INT-extra"},
			nonMatches: []string{"deploy/DEMO-2", "feature/DEMO-2", "deploy/DEMO-2-to-INT/nested"},
		},
		{
			name:       "custom single-token format still anchors on its literal prefix",
			format:     "promo/{{ticket}}",
			matches:    []string{"promo/PROJ-1"},
			nonMatches: []string{"promo", "promo/PROJ-1/extra", "other/PROJ-1"},
		},
		{
			name:       "token-leading format is usable when it still has literal content to anchor on",
			format:     "{{ticket}}-to-{{target}}",
			matches:    []string{"DEMO-2-to-INT"},
			nonMatches: []string{"DEMO-2", "feature/DEMO-2-to-INT"},
		},
		{
			name:    "empty format skips filtering (nil matcher)",
			format:  "",
			wantNil: true,
		},
		{
			name:    "token-only format with no literal content to anchor skips filtering (nil matcher)",
			format:  "{{ticket}}",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{BranchFormat: tt.format}
			got := git.PromotionBranchMatcher(cfg)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("PromotionBranchMatcher(%q) = %v, want nil", tt.format, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("PromotionBranchMatcher(%q) = nil, want a usable matcher", tt.format)
			}
			for _, m := range tt.matches {
				if !got.MatchString(m) {
					t.Errorf("PromotionBranchMatcher(%q): expected match for %q", tt.format, m)
				}
			}
			for _, nm := range tt.nonMatches {
				if got.MatchString(nm) {
					t.Errorf("PromotionBranchMatcher(%q): expected NO match for %q", tt.format, nm)
				}
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
