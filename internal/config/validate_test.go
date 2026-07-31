package config_test

import (
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

func validConfig() config.Config {
	return config.Config{
		Branches: map[string]string{"integration": "INT"},
		Sandboxes: map[string]config.SandboxConfig{
			"INT": {Alias: "INT_SANDBOX", TestLevel: "RunLocalTests"},
		},
		TicketPatterns:      []string{"OTACUPYR-[0-9]+"},
		BranchFormat:        "deploy/{{ticket}}-to-{{target}}",
		MinVersions:         map[string]string{"git": "2.30.0"},
		Runs:                config.RunsConfig{KeepLast: 30, KeepDays: 90},
		PollIntervalSeconds: 10,
		PollTimeoutSeconds:  3600,
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *config.Config)
		wantErr bool
	}{
		{
			name:    "valid config passes",
			mutate:  func(c *config.Config) {},
			wantErr: false,
		},
		{
			name: "invalid ticketPatterns regex fails",
			mutate: func(c *config.Config) {
				c.TicketPatterns = []string{"[invalid("}
			},
			wantErr: true,
		},
		{
			name: "sandbox missing alias fails",
			mutate: func(c *config.Config) {
				c.Sandboxes["UAT"] = config.SandboxConfig{TestLevel: "RunLocalTests"}
			},
			wantErr: true,
		},
		{
			name: "branchFormat token outside allow-list fails",
			mutate: func(c *config.Config) {
				c.BranchFormat = "deploy/{{ticket}}-to-{{bogus}}"
			},
			wantErr: true,
		},
		{
			name: "pollIntervalSeconds <= 0 fails",
			mutate: func(c *config.Config) {
				c.PollIntervalSeconds = 0
			},
			wantErr: true,
		},
		{
			name: "negative pollIntervalSeconds fails",
			mutate: func(c *config.Config) {
				c.PollIntervalSeconds = -1
			},
			wantErr: true,
		},
		{
			name: "pollTimeoutSeconds <= 0 fails",
			mutate: func(c *config.Config) {
				c.PollTimeoutSeconds = 0
			},
			wantErr: true,
		},
		{
			name: "delta configured with empty sourceDirs fails",
			mutate: func(c *config.Config) {
				c.Delta = config.DeltaConfig{OutputDir: ".deploydeck/manifest/delta"}
			},
			wantErr: true,
		},
		{
			name: "delta configured with non-empty sourceDirs passes",
			mutate: func(c *config.Config) {
				c.Delta = config.DeltaConfig{
					OutputDir:  ".deploydeck/manifest/delta",
					SourceDirs: []string{"force-app"},
				}
			},
			wantErr: false,
		},
		{
			name: "negative runs.keepLast fails",
			mutate: func(c *config.Config) {
				c.Runs.KeepLast = -1
			},
			wantErr: true,
		},
		{
			name: "negative runs.keepDays fails",
			mutate: func(c *config.Config) {
				c.Runs.KeepDays = -1
			},
			wantErr: true,
		},
		{
			name: "runs.keepLast==0 passes (a valid, if aggressive, retention window)",
			mutate: func(c *config.Config) {
				c.Runs.KeepLast = 0
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected Validate() to return an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected Validate() to return nil, got: %v", err)
			}
		})
	}
}

// TestConfig_Validate_RunsBoundsIdentifyField is task 2.1 (RED): the
// run-retention spec requires each bounds error to identify the offending
// field (run-retention spec: "KeepLast/KeepDays Config Bounds Are
// Validated").
func TestConfig_Validate_RunsBoundsIdentifyField(t *testing.T) {
	keepLastCfg := validConfig()
	keepLastCfg.Runs.KeepLast = -5
	if err := keepLastCfg.Validate(); err == nil || !strings.Contains(err.Error(), "keepLast") {
		t.Fatalf("expected an error identifying keepLast, got %v", err)
	}

	keepDaysCfg := validConfig()
	keepDaysCfg.Runs.KeepDays = -5
	if err := keepDaysCfg.Validate(); err == nil || !strings.Contains(err.Error(), "keepDays") {
		t.Fatalf("expected an error identifying keepDays, got %v", err)
	}
}
