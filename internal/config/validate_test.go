package config_test

import (
	"testing"

	"deploydeck/internal/config"
)

func validConfig() config.Config {
	return config.Config{
		Branches: map[string]string{"integration": "INT"},
		Sandboxes: map[string]config.SandboxConfig{
			"INT": {Alias: "INT_SANDBOX", TestLevel: "RunLocalTests"},
		},
		TicketPatterns: []string{"OTACUPYR-[0-9]+"},
		BranchFormat:   "deploy/{{ticket}}-to-{{target}}",
		MinVersions:    map[string]string{"git": "2.30.0"},
		Runs:           config.RunsConfig{KeepLast: 30, KeepDays: 90},
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
