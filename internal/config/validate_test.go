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
		// AIConfig validation is task 2.1 (RED): absent ai (zero value) is
		// off/valid; enabled:true with an empty endpoint or model fails;
		// both set passes (ai-pr-summary spec: "AI Configuration Is
		// Optional And Zero-Value-Safe").
		{
			name:    "absent ai block (zero value) is off and valid",
			mutate:  func(c *config.Config) {},
			wantErr: false,
		},
		{
			name: "ai enabled with empty endpoint fails",
			mutate: func(c *config.Config) {
				c.AI = config.AIConfig{Enabled: true, Model: "qwen2.5-coder:3b"}
			},
			wantErr: true,
		},
		{
			name: "ai enabled with empty model fails",
			mutate: func(c *config.Config) {
				c.AI = config.AIConfig{Enabled: true, Endpoint: "http://localhost:11434"}
			},
			wantErr: true,
		},
		{
			name: "ai enabled with endpoint and model passes",
			mutate: func(c *config.Config) {
				c.AI = config.AIConfig{Enabled: true, Endpoint: "http://localhost:11434", Model: "qwen2.5-coder:3b"}
			},
			wantErr: false,
		},
		{
			name: "ai disabled with empty endpoint/model still passes",
			mutate: func(c *config.Config) {
				c.AI = config.AIConfig{Enabled: false}
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

// TestConfig_Validate_Gates is tasks 1.7-1.9 (RED): an enabled gate with an
// explicit minApprovals < 1 is rejected; an enabled gate with an empty
// approvers list is rejected UNCONDITIONALLY, even when minApprovals is
// omitted; an enabled gate with minApprovals omitted (nil) and a non-empty
// approvers list passes (nil is never treated as "< 1") — deploy-gate spec:
// "An enabled gate with invalid approval settings is rejected".
func TestConfig_Validate_Gates(t *testing.T) {
	zero := 0
	negative := -1
	one := 1

	tests := []struct {
		name    string
		gate    config.GateConfig
		wantErr bool
	}{
		{
			name:    "enabled gate with explicit minApprovals < 1 (zero) is rejected",
			gate:    config.GateConfig{Enabled: true, Approvers: []string{"alice"}, MinApprovals: &zero},
			wantErr: true,
		},
		{
			name:    "enabled gate with explicit negative minApprovals is rejected",
			gate:    config.GateConfig{Enabled: true, Approvers: []string{"alice"}, MinApprovals: &negative},
			wantErr: true,
		},
		{
			name:    "enabled gate with empty approvers is rejected even with minApprovals omitted",
			gate:    config.GateConfig{Enabled: true, Approvers: nil},
			wantErr: true,
		},
		{
			name:    "enabled gate with empty approvers and explicit minApprovals is still rejected",
			gate:    config.GateConfig{Enabled: true, Approvers: []string{}, MinApprovals: &one},
			wantErr: true,
		},
		{
			name:    "enabled gate with minApprovals omitted and a non-empty approvers list passes",
			gate:    config.GateConfig{Enabled: true, Approvers: []string{"alice"}},
			wantErr: false,
		},
		{
			name:    "enabled gate with explicit valid minApprovals and approvers passes",
			gate:    config.GateConfig{Enabled: true, Approvers: []string{"alice", "bob"}, MinApprovals: &one},
			wantErr: false,
		},
		{
			name:    "disabled gate with invalid settings is NOT validated (ungated, deploys as today)",
			gate:    config.GateConfig{Enabled: false, Approvers: nil, MinApprovals: &zero},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Gates = map[string]config.GateConfig{"UAT": tt.gate}

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

// TestConfig_Validate_Gates_DeterministicErrorTarget is the remediation-pass
// readability fix (Fix 7): with MULTIPLE invalid gates, Validate() must name
// the SAME target on every call — Go's map iteration order is randomized per
// range statement (even across repeated ranges over the SAME map), so an
// unsorted `for target, gate := range c.Gates` makes the reported error
// target (and therefore the whole error message) flaky. Sorting the keys
// first makes the alphabetically-first invalid target win, deterministically,
// every time.
func TestConfig_Validate_Gates_DeterministicErrorTarget(t *testing.T) {
	cfg := validConfig()
	cfg.Gates = map[string]config.GateConfig{
		"ZETA":   {Enabled: true, Approvers: nil},
		"ALPHA":  {Enabled: true, Approvers: nil},
		"MIDDLE": {Enabled: true, Approvers: nil},
	}

	var first string
	for i := 0; i < 20; i++ {
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if i == 0 {
			first = err.Error()
			continue
		}
		if err.Error() != first {
			t.Fatalf("Validate() error is non-deterministic across repeated calls on the SAME map: run 0 got %q, run %d got %q", first, i, err.Error())
		}
	}
	if !strings.Contains(first, `"ALPHA"`) {
		t.Fatalf("expected the deterministic error to always name the alphabetically-first invalid gate %q, got: %q", "ALPHA", first)
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
