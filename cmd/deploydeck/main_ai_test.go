package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

// writeAIFixture seeds a minimal deploydeck.yaml with (or without) an `ai:`
// section, mirroring doctor_e2e_test.go's fixture-writing style.
func writeAIFixture(t *testing.T, dir, aiSection string) {
	t.Helper()
	content := "branches:\n  integration: INT\n" + aiSection
	if err := os.WriteFile(filepath.Join(dir, "deploydeck.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("seeding deploydeck.yaml: %v", err)
	}
}

// TestDefaultChecker_AIDisabledOrAbsent_CheckerAIIsNil is task 6.1 (RED):
// with no `ai:` section (or `enabled: false`), defaultChecker leaves
// Checker.AI nil — the nil-degrades convention CheckAI's skip branch relies
// on (design ADR-7: "main wires ... Checker.AI ONLY when cfg.AI.Enabled").
func TestDefaultChecker_AIDisabledOrAbsent_CheckerAIIsNil(t *testing.T) {
	tests := []struct {
		name      string
		aiSection string
	}{
		{name: "absent ai section", aiSection: ""},
		{name: "explicit enabled: false", aiSection: "ai:\n  enabled: false\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAIFixture(t, dir, tt.aiSection)

			checker, err := defaultChecker(dir)
			if err != nil {
				t.Fatalf("defaultChecker() unexpected error: %v", err)
			}
			if checker.AI != nil {
				t.Fatalf("expected Checker.AI to be nil when AI is disabled/absent, got %+v", checker.AI)
			}
		})
	}
}

// TestDefaultChecker_AIEnabled_CheckerAIIsWired is task 6.1 (RED): an
// explicit `ai: {enabled: true, endpoint, model}` wires a real,
// non-nil Checker.AI.
func TestDefaultChecker_AIEnabled_CheckerAIIsWired(t *testing.T) {
	dir := t.TempDir()
	writeAIFixture(t, dir, "ai:\n  enabled: true\n  endpoint: http://localhost:11434\n  model: qwen2.5-coder:3b\n")

	checker, err := defaultChecker(dir)
	if err != nil {
		t.Fatalf("defaultChecker() unexpected error: %v", err)
	}
	if checker.AI == nil {
		t.Fatal("expected Checker.AI to be wired when AI is enabled")
	}
}

// TestComposeGenerateSummary_DisabledOrAbsent_ReturnsNil is task 6.1 (RED):
// the composition helper backing app.Deps.GenerateSummary returns nil when
// AI is disabled/absent, so main never wires the AI-suggestion affordance
// unless explicitly enabled (spec: "No AI Config Leaves The Push/PR Flow
// Unchanged").
func TestComposeGenerateSummary_DisabledOrAbsent_ReturnsNil(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
	}{
		{name: "zero value", cfg: config.Config{}},
		{
			name: "explicit disabled with endpoint/model set",
			cfg:  config.Config{AI: config.AIConfig{Enabled: false, Endpoint: "http://localhost:11434", Model: "qwen2.5-coder:3b"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := composeGenerateSummary(tt.cfg); got != nil {
				t.Fatalf("composeGenerateSummary() = non-nil, want nil when AI is disabled/absent")
			}
		})
	}
}

// TestComposeGenerateSummary_Enabled_ReturnsNonNil is task 6.1 (RED): AI
// enabled wires a non-nil scalar closure.
func TestComposeGenerateSummary_Enabled_ReturnsNonNil(t *testing.T) {
	cfg := config.Config{AI: config.AIConfig{Enabled: true, Endpoint: "http://localhost:11434", Model: "qwen2.5-coder:3b"}}
	if got := composeGenerateSummary(cfg); got == nil {
		t.Fatal("composeGenerateSummary() = nil, want a non-nil closure when AI is enabled")
	}
}
