package git_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// TestListDestinations_TableDriven proves every configured destination
// branch (config.Config.Branches) is shown together with its resolved
// sandbox alias and test level (HU-004 AC: "cada destino se muestra con su
// sandbox asociada"), and that a branch with no matching sandbox entry is
// still listed but flagged unresolved rather than silently dropped.
// TestResolveSandbox_TableDriven proves ResolveSandbox wires
// config.SandboxFor into the target-selection flow: a Release/* branch
// resolves via its configured glob pattern, and a destination with no
// matching mapping blocks with an actionable message rather than a bare
// error (HU-004 AC: "si no hay mapeo, se bloquea con mensaje accionable").
func TestResolveSandbox_TableDriven(t *testing.T) {
	cfg := config.Config{
		Sandboxes: map[string]config.SandboxConfig{
			"UAT":       {Alias: "UAT_SANDBOX", TestLevel: "RunLocalTests"},
			"Release/*": {Alias: "PREPROD_SANDBOX", TestLevel: "RunLocalTests"},
		},
	}

	tests := []struct {
		name        string
		branch      string
		wantAlias   string
		wantBlocked bool
	}{
		{name: "exact configured branch resolves", branch: "UAT", wantAlias: "UAT_SANDBOX"},
		{name: "Release/* glob resolves by pattern", branch: "Release/Julio2026", wantAlias: "PREPROD_SANDBOX"},
		{name: "unmapped destination blocks with an actionable message", branch: "INT", wantBlocked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox, blockMessage, err := git.ResolveSandbox(cfg, tt.branch)

			if tt.wantBlocked {
				if err == nil {
					t.Fatalf("expected an error for unmapped branch %q, got nil", tt.branch)
				}
				if blockMessage == "" {
					t.Fatalf("expected a non-empty actionable block message for %q", tt.branch)
				}
				if !strings.Contains(blockMessage, tt.branch) {
					t.Fatalf("expected block message to name the branch %q, got %q", tt.branch, blockMessage)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.branch, err)
			}
			if blockMessage != "" {
				t.Fatalf("expected no block message for a resolved branch, got %q", blockMessage)
			}
			if sandbox.Alias != tt.wantAlias {
				t.Fatalf("ResolveSandbox(%q).Alias = %q, want %q", tt.branch, sandbox.Alias, tt.wantAlias)
			}
		})
	}
}

// TestUnauthenticatedSandboxWarning_TableDriven proves the pure decision:
// an alias present in ANY of the five sf org list --json categories does
// not warn, an absent one does, and an empty alias (an unmapped sandbox,
// ResolveSandbox's own concern) never warns here.
func TestUnauthenticatedSandboxWarning_TableDriven(t *testing.T) {
	orgs := salesforce.OrgList{
		Sandboxes: []salesforce.Org{{Alias: "UAT_SANDBOX"}},
		DevHubs:   []salesforce.Org{{Alias: "PREPROD_SANDBOX"}},
	}

	tests := []struct {
		name  string
		alias string
		want  bool
	}{
		{name: "alias found in sandboxes category does not warn", alias: "UAT_SANDBOX", want: false},
		{name: "alias found in a non-default category does not warn", alias: "PREPROD_SANDBOX", want: false},
		{name: "alias absent from every category warns", alias: "GHOST_SANDBOX", want: true},
		{name: "empty alias never warns", alias: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.UnauthenticatedSandboxWarning(orgs, tt.alias); got != tt.want {
				t.Errorf("UnauthenticatedSandboxWarning(%q) = %v, want %v", tt.alias, got, tt.want)
			}
		})
	}
}

// TestSandboxAuthWarning_ComposesSalesforceClient proves the composed
// entry point runs `sf org list --json` (via a FakeRunner sf-fake) and
// reports the warning for an alias absent from every category, while a
// present alias reports no warning (HU-004 AC: "si la sandbox asociada no
// esta autenticada, se muestra advertencia antes de validar").
func TestSandboxAuthWarning_ComposesSalesforceClient(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{
			"sandboxes":[{"alias":"UAT_SANDBOX","username":"a@b.com"}]
		}}`),
	})
	sf := salesforce.New(fr)

	warned, err := git.SandboxAuthWarning(context.Background(), sf, "GHOST_SANDBOX")
	if err != nil {
		t.Fatalf("SandboxAuthWarning: unexpected error: %v", err)
	}
	if !warned {
		t.Errorf("expected a warning for an alias absent from sf org list --json, got warned=false")
	}

	notWarned, err := git.SandboxAuthWarning(context.Background(), sf, "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("SandboxAuthWarning: unexpected error: %v", err)
	}
	if notWarned {
		t.Errorf("expected no warning for an authenticated alias, got warned=true")
	}
}

// TestIsProductionBranch_TableDriven proves selecting "main" triggers
// HU-004's production-environment warning, while any other branch name
// (including one that merely CONTAINS "main") does not (HU-004 AC: "Dado
// que el usuario selecciona main, entonces la TUI muestra advertencia de
// entorno productivo").
func TestIsProductionBranch_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   bool
	}{
		{name: "main triggers the production warning", branch: "main", want: true},
		{name: "UAT does not trigger the production warning", branch: "UAT", want: false},
		{name: "a branch merely containing main does not trigger it", branch: "feature/maintenance", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.IsProductionBranch(tt.branch); got != tt.want {
				t.Errorf("IsProductionBranch(%q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

// TestConfirmTargetSelection_TableDriven proves confirming a valid
// destination+sandbox saves branch/alias/testLevel into the
// DeploymentPlan, preserving any fields the plan already carried from an
// earlier stage (HU-003's Ticket/SelectedCommits) rather than replacing
// the whole plan (HU-004 AC: "Guardar seleccion en DeploymentPlan").
func TestConfirmTargetSelection_TableDriven(t *testing.T) {
	basePlan := git.DeploymentPlan{
		Ticket:          "PROJ-1",
		SelectedCommits: []git.DiscoveredCommit{{Commit: git.Commit{SHA: "abc"}}},
	}

	tests := []struct {
		name         string
		branch       string
		sandboxAlias string
		testLevel    string
	}{
		{name: "UAT destination with RunLocalTests", branch: "UAT", sandboxAlias: "UAT_SANDBOX", testLevel: "RunLocalTests"},
		{name: "Release destination with NoTestRun", branch: "Release/Julio2026", sandboxAlias: "PREPROD_SANDBOX", testLevel: "NoTestRun"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.ConfirmTargetSelection(basePlan, tt.branch, tt.sandboxAlias, tt.testLevel)

			if got.TargetBranch != tt.branch {
				t.Errorf("TargetBranch = %q, want %q", got.TargetBranch, tt.branch)
			}
			if got.SandboxAlias != tt.sandboxAlias {
				t.Errorf("SandboxAlias = %q, want %q", got.SandboxAlias, tt.sandboxAlias)
			}
			if got.TestLevel != tt.testLevel {
				t.Errorf("TestLevel = %q, want %q", got.TestLevel, tt.testLevel)
			}
			if got.Ticket != basePlan.Ticket || len(got.SelectedCommits) != len(basePlan.SelectedCommits) {
				t.Errorf("expected prior plan fields preserved, got Ticket=%q SelectedCommits=%+v", got.Ticket, got.SelectedCommits)
			}
		})
	}
}

func TestListDestinations_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want []git.Destination
	}{
		{
			name: "two configured branches, both with a sandbox mapping",
			cfg: config.Config{
				Branches: map[string]string{
					"integration": "INT",
					"uat":         "UAT",
				},
				Sandboxes: map[string]config.SandboxConfig{
					"INT": {Alias: "INT_SANDBOX", TestLevel: "NoTestRun"},
					"UAT": {Alias: "UAT_SANDBOX", TestLevel: "RunLocalTests"},
				},
			},
			want: []git.Destination{
				{Environment: "integration", Branch: "INT", Sandbox: config.SandboxConfig{Alias: "INT_SANDBOX", TestLevel: "NoTestRun"}, SandboxResolved: true},
				{Environment: "uat", Branch: "UAT", Sandbox: config.SandboxConfig{Alias: "UAT_SANDBOX", TestLevel: "RunLocalTests"}, SandboxResolved: true},
			},
		},
		{
			name: "a configured branch with no sandbox mapping is listed but unresolved",
			cfg: config.Config{
				Branches: map[string]string{
					"integration": "INT",
				},
			},
			want: []git.Destination{
				{Environment: "integration", Branch: "INT", SandboxResolved: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.ListDestinations(tt.cfg)

			if len(got) != len(tt.want) {
				t.Fatalf("ListDestinations() = %+v, want %+v", got, tt.want)
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("ListDestinations()[%d] = %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}
