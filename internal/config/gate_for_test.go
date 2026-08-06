package config_test

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

// TestConfig_GateFor is tasks 1.3-1.5 (RED): GateFor resolves exact-match
// before glob (mirrors SandboxFor's own exact-then-glob order), reports
// ungated (false) for no match at all, and reports ungated (false) for a
// matched-but-disabled entry (deploy-gate spec: "Gated environment resolves
// its configuration by exact match then glob" / "An environment with no
// matching gate entry is ungated").
func TestConfig_GateFor(t *testing.T) {
	minTwo := 2
	cfg := config.Config{
		Gates: map[string]config.GateConfig{
			"UAT": {
				Enabled:      true,
				Approvers:    []string{"alice"},
				MinApprovals: &minTwo,
			},
			"Release/*": {
				Enabled:   true,
				Approvers: []string{"bob"},
			},
			"Disabled/*": {
				Enabled:   false,
				Approvers: []string{"carol"},
			},
		},
	}

	t.Run("exact match wins over a separate glob entry that would also match", func(t *testing.T) {
		got, ok := cfg.GateFor("UAT")
		if !ok {
			t.Fatalf("GateFor(%q) ok = false, want true", "UAT")
		}
		if got.MinApprovals == nil || *got.MinApprovals != 2 {
			t.Fatalf("GateFor(%q).MinApprovals = %v, want a pointer to 2 (the EXACT entry, not any glob)", "UAT", got.MinApprovals)
		}
	})

	t.Run("glob match resolves when no exact entry matches", func(t *testing.T) {
		got, ok := cfg.GateFor("Release/2024.1")
		if !ok {
			t.Fatalf("GateFor(%q) ok = false, want true", "Release/2024.1")
		}
		if len(got.Approvers) != 1 || got.Approvers[0] != "bob" {
			t.Fatalf("GateFor(%q).Approvers = %v, want [bob]", "Release/2024.1", got.Approvers)
		}
	})

	t.Run("no exact or glob match is ungated", func(t *testing.T) {
		_, ok := cfg.GateFor("feature/unrelated")
		if ok {
			t.Fatalf("GateFor(%q) ok = true, want false (no matching entry)", "feature/unrelated")
		}
	})

	t.Run("a matched but disabled entry is ungated", func(t *testing.T) {
		_, ok := cfg.GateFor("Disabled/x")
		if ok {
			t.Fatalf("GateFor(%q) ok = true, want false (enabled:false)", "Disabled/x")
		}
	})
}
