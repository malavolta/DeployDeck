package config_test

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

func TestConfig_SandboxFor(t *testing.T) {
	cfg := config.Config{
		Sandboxes: map[string]config.SandboxConfig{
			"INT":       {Alias: "INT_SANDBOX", TestLevel: "RunLocalTests"},
			"Release/*": {Alias: "PREPROD_SANDBOX", TestLevel: "RunLocalTests"},
		},
	}

	tests := []struct {
		name      string
		branch    string
		wantAlias string
		wantErr   bool
	}{
		{
			name:      "exact match",
			branch:    "INT",
			wantAlias: "INT_SANDBOX",
		},
		{
			name:      "Release/* glob match",
			branch:    "Release/2024.1",
			wantAlias: "PREPROD_SANDBOX",
		},
		{
			name:    "no match errors",
			branch:  "feature/unrelated",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox, err := cfg.SandboxFor(tt.branch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for branch %q, got nil", tt.branch)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for branch %q: %v", tt.branch, err)
			}
			if sandbox.Alias != tt.wantAlias {
				t.Fatalf("SandboxFor(%q).Alias = %q, want %q", tt.branch, sandbox.Alias, tt.wantAlias)
			}
		})
	}
}
