package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

func writeFixture(t *testing.T, dir, contents string) {
	t.Helper()
	path := filepath.Join(dir, "deploydeck.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
}

func TestLoad_AppliesDefaultsForOmittedFields(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		wantBranchFormat string
		wantKeepLast     int
		wantKeepDays     int
	}{
		{
			name: "omitted branchFormat and runs get defaults",
			yaml: `
branches:
  integration: INT
`,
			wantBranchFormat: "deploy/{{ticket}}-to-{{target}}",
			wantKeepLast:     30,
			wantKeepDays:     90,
		},
		{
			name: "explicit branchFormat and runs override defaults",
			yaml: `
branchFormat: "promo/{{ticket}}"
runs:
  keepLast: 5
  keepDays: 14
`,
			wantBranchFormat: "promo/{{ticket}}",
			wantKeepLast:     5,
			wantKeepDays:     14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, tt.yaml)

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("Load() returned unexpected error: %v", err)
			}

			if cfg.BranchFormat != tt.wantBranchFormat {
				t.Errorf("BranchFormat = %q, want %q", cfg.BranchFormat, tt.wantBranchFormat)
			}
			if cfg.Runs.KeepLast != tt.wantKeepLast {
				t.Errorf("Runs.KeepLast = %d, want %d", cfg.Runs.KeepLast, tt.wantKeepLast)
			}
			if cfg.Runs.KeepDays != tt.wantKeepDays {
				t.Errorf("Runs.KeepDays = %d, want %d", cfg.Runs.KeepDays, tt.wantKeepDays)
			}
		})
	}
}

func TestLoad_ParsesConfiguredFields(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, `
branches:
  integration: INT
  production: main

sandboxes:
  INT:
    alias: INT_SANDBOX
    testLevel: RunLocalTests

ticketPatterns:
  - "OTACUPYR-[0-9]+"

minVersions:
  git: "2.30.0"
  sf: "2.0.0"
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if got := cfg.Branches["integration"]; got != "INT" {
		t.Errorf("Branches[integration] = %q, want %q", got, "INT")
	}
	if got := cfg.Sandboxes["INT"].Alias; got != "INT_SANDBOX" {
		t.Errorf("Sandboxes[INT].Alias = %q, want %q", got, "INT_SANDBOX")
	}
	if len(cfg.TicketPatterns) != 1 || cfg.TicketPatterns[0] != "OTACUPYR-[0-9]+" {
		t.Errorf("TicketPatterns = %v, want [OTACUPYR-[0-9]+]", cfg.TicketPatterns)
	}
	if got := cfg.MinVersions["git"]; got != "2.30.0" {
		t.Errorf("MinVersions[git] = %q, want %q", got, "2.30.0")
	}
}
