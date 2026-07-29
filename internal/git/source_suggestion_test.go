package git_test

import (
	"testing"

	"deploydeck/internal/config"
	"deploydeck/internal/git"
)

func envConfig() config.Config {
	return config.Config{
		Branches: map[string]string{
			"integration": "INT",
			"uat":         "UAT",
			"production":  "main",
		},
	}
}

func TestSuggestDefaultSource_TableDriven(t *testing.T) {
	cfg := envConfig()

	tests := []struct {
		name       string
		candidates []git.Branch
		target     string
		wantBranch git.Branch
		wantOK     bool
	}{
		{
			name:       "env-to-env promotion suggests the previous validated environment, not a feature branch",
			candidates: []git.Branch{{Name: "feature/PROJ-1"}},
			target:     "UAT",
			wantBranch: git.Branch{Name: "INT"},
			wantOK:     true,
		},
		{
			name: "suggestion prefers the matching candidate entry (e.g. remote-tracking) over a bare synthesized branch",
			candidates: []git.Branch{
				{Name: "feature/PROJ-1"},
				{Name: "origin/INT", Remote: true},
			},
			target:     "UAT",
			wantBranch: git.Branch{Name: "origin/INT", Remote: true},
			wantOK:     true,
		},
		{
			name:       "the first pipeline stage has no previous environment to suggest",
			candidates: []git.Branch{{Name: "feature/PROJ-1"}},
			target:     "INT",
			wantOK:     false,
		},
		{
			name:       "a target that isn't a configured environment branch has no suggestion",
			candidates: []git.Branch{{Name: "feature/PROJ-1"}},
			target:     "feature/PROJ-1",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := git.SuggestDefaultSource(tt.candidates, cfg, tt.target)
			if ok != tt.wantOK {
				t.Fatalf("SuggestDefaultSource() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.wantBranch {
				t.Fatalf("SuggestDefaultSource() = %+v, want %+v", got, tt.wantBranch)
			}
		})
	}
}

// TestNextEnvironmentBranch_TableDriven is task 2.1 (RED): forward mirror of
// TestSuggestDefaultSource_TableDriven — given the prior environment,
// NextEnvironmentBranch suggests the NEXT stage in the pipeline as the
// plain configured branch name (never origin/-prefixed), or ok=false at the
// last stage / outside the fixed pipeline order / an unconfigured branch
// (commit-discovery spec: "Next environment is suggested as default
// target" + "No next environment falls back to manual target choice").
func TestNextEnvironmentBranch_TableDriven(t *testing.T) {
	cfg := envConfig()

	tests := []struct {
		name          string
		currentTarget string
		wantBranch    string
		wantOK        bool
	}{
		{
			name:          "INT has UAT as its next environment",
			currentTarget: "INT",
			wantBranch:    "UAT",
			wantOK:        true,
		},
		{
			name:          "UAT has prod (main) as its next environment",
			currentTarget: "UAT",
			wantBranch:    "main",
			wantOK:        true,
		},
		{
			name:          "prod is the last pipeline stage, no next environment",
			currentTarget: "main",
			wantBranch:    "",
			wantOK:        false,
		},
		{
			name:          "a Release/* glob target falls outside the fixed pipeline order",
			currentTarget: "Release/x",
			wantBranch:    "",
			wantOK:        false,
		},
		{
			name:          "an unconfigured branch has no next environment",
			currentTarget: "feature/PROJ-1",
			wantBranch:    "",
			wantOK:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := git.NextEnvironmentBranch(cfg, tt.currentTarget)
			if ok != tt.wantOK {
				t.Fatalf("NextEnvironmentBranch() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.wantBranch {
				t.Fatalf("NextEnvironmentBranch() = %q, want %q (must be the plain branch name, never origin/-prefixed)", got, tt.wantBranch)
			}
		})
	}
}

// TestIsProductionTarget_FailClosed_TableDriven is task 1.6 (RED), extended by
// the adversarial-review Finding M-2 remediation: IsProductionTarget must be
// FAIL-CLOSED for EVERY unclassifiable target, not just the literal "main".
// A target counts as NON-production ONLY when it positively maps to a known
// non-production pipeline environment (integration/uat); an empty target or a
// target that maps to NO configured environment (ok=false) is treated as
// production and therefore blocked unless AllowProduction is set (quick-deploy
// spec: "Production Target Blocked Without Explicit Configuration", BLOCKER
// correction 1 + Finding M-2 "production block must not fail OPEN for empty /
// unmapped targets").
func TestIsProductionTarget_FailClosed_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		cfg    config.Config
		target string
		want   bool
	}{
		{
			name:   "no production key configured at all: literal main still fail-closed to production",
			cfg:    config.Config{},
			target: "main",
			want:   true,
		},
		{
			name:   "configured production env-key matches the target branch",
			cfg:    config.Config{Branches: map[string]string{"production": "RELEASE"}},
			target: "RELEASE",
			want:   true,
		},
		{
			name:   "literal main is production even when the configured production branch is different (OR-condition)",
			cfg:    config.Config{Branches: map[string]string{"production": "RELEASE"}},
			target: "main",
			want:   true,
		},
		{
			// Finding M-2: this replaces the prior fail-OPEN case, which asserted
			// an UNMAPPED "UAT" (config had only a "production" key) → false. That
			// encoded the bug: an unmapped target must fail CLOSED. To keep a
			// meaningful "genuinely non-production target → false" case, UAT is now
			// actually mapped to the non-production "uat" environment key.
			name:   "a target that maps to the non-production uat env is not production",
			cfg:    config.Config{Branches: map[string]string{"production": "RELEASE", "uat": "UAT"}},
			target: "UAT",
			want:   false,
		},
		{
			// Finding M-2: a target that maps to the non-production integration env
			// is likewise not production.
			name:   "a target that maps to the non-production integration env is not production",
			cfg:    config.Config{Branches: map[string]string{"integration": "INT", "uat": "UAT", "production": "main"}},
			target: "INT",
			want:   false,
		},
		{
			// Finding M-2 (RED driver): an EMPTY target is unclassifiable, so it
			// must fail closed to production rather than slip through as non-prod.
			name:   "an empty target is unclassifiable and fails closed to production",
			cfg:    config.Config{Branches: map[string]string{"integration": "INT", "uat": "UAT", "production": "main"}},
			target: "",
			want:   true,
		},
		{
			// Finding M-2 (RED driver): a non-main target that maps to NO
			// configured environment is unclassifiable and fails closed.
			name:   "an unmapped non-main target is unclassifiable and fails closed to production",
			cfg:    config.Config{Branches: map[string]string{"production": "RELEASE"}},
			target: "feature/PROJ-1",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.IsProductionTarget(tt.cfg, tt.target)
			if got != tt.want {
				t.Fatalf("IsProductionTarget(%+v, %q) = %v, want %v", tt.cfg, tt.target, got, tt.want)
			}
		})
	}
}

// TestSuggestDefaultSource_OverrideStillEnforcesSingleSource proves the
// RF-002 suggestion never bypasses single-source enforcement: the user can
// override it with any other candidate, and SelectSingleSource still
// validates the final choice against the candidate pool.
func TestSuggestDefaultSource_OverrideStillEnforcesSingleSource(t *testing.T) {
	cfg := envConfig()
	candidates := []git.Branch{
		{Name: "feature/PROJ-1"},
		{Name: "origin/INT", Remote: true},
	}

	suggested, ok := git.SuggestDefaultSource(candidates, cfg, "UAT")
	if !ok || suggested.Name != "origin/INT" {
		t.Fatalf("expected origin/INT suggested, got %+v ok=%v", suggested, ok)
	}

	// Blocked without an explicit choice among >1 candidates, even though
	// a suggestion exists — the suggestion is a UI default, not an
	// automatic selection.
	if _, err := git.SelectSingleSource(candidates, ""); err == nil {
		t.Fatalf("expected SelectSingleSource to still require an explicit choice")
	}

	// Overriding with a different candidate is accepted.
	override, err := git.SelectSingleSource(candidates, "feature/PROJ-1")
	if err != nil {
		t.Fatalf("expected override to be accepted, got error: %v", err)
	}
	if override.Name != "feature/PROJ-1" {
		t.Fatalf("expected override to resolve to feature/PROJ-1, got %+v", override)
	}

	// Confirming the suggested branch itself is equally valid.
	confirmed, err := git.SelectSingleSource(candidates, suggested.Name)
	if err != nil {
		t.Fatalf("expected the suggested branch to be a valid selection, got error: %v", err)
	}
	if confirmed != suggested {
		t.Fatalf("expected confirming the suggestion to resolve to it, got %+v want %+v", confirmed, suggested)
	}
}
