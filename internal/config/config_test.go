package config_test

import (
	"testing"

	"deploydeck/internal/config"
)

// TestLoad_DeltaConfig_ParsesConfiguredFields proves HU-007's delta section
// (outputDir, sourceDirs, ignoreFile, ignoreDestructiveFile) round-trips
// through Load exactly as configured.
func TestLoad_DeltaConfig_ParsesConfiguredFields(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, `
branches:
  integration: INT

delta:
  outputDir: .deploydeck/manifest/delta
  sourceDirs:
    - force-app
    - unpackaged
  ignoreFile: .sgdignore
  ignoreDestructiveFile: .sgdignoredestructive
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Delta.OutputDir != ".deploydeck/manifest/delta" {
		t.Errorf("Delta.OutputDir = %q, want %q", cfg.Delta.OutputDir, ".deploydeck/manifest/delta")
	}

	wantDirs := []string{"force-app", "unpackaged"}
	if len(cfg.Delta.SourceDirs) != len(wantDirs) {
		t.Fatalf("Delta.SourceDirs = %v, want %v", cfg.Delta.SourceDirs, wantDirs)
	}
	for i := range wantDirs {
		if cfg.Delta.SourceDirs[i] != wantDirs[i] {
			t.Errorf("Delta.SourceDirs[%d] = %q, want %q", i, cfg.Delta.SourceDirs[i], wantDirs[i])
		}
	}

	if cfg.Delta.IgnoreFile != ".sgdignore" {
		t.Errorf("Delta.IgnoreFile = %q, want %q", cfg.Delta.IgnoreFile, ".sgdignore")
	}
	if cfg.Delta.IgnoreDestructiveFile != ".sgdignoredestructive" {
		t.Errorf("Delta.IgnoreDestructiveFile = %q, want %q", cfg.Delta.IgnoreDestructiveFile, ".sgdignoredestructive")
	}
}

// TestLoad_PollSecondsAndDeltaOutputDir_DefaultsWhenOmitted proves
// pollIntervalSeconds/pollTimeoutSeconds always default to 10/3600, and
// delta.outputDir defaults to DefaultDeltaOutputDir ONLY once the user has
// opted into delta via a non-empty sourceDirs — an entirely omitted delta
// section must stay entirely empty post-Load (never silently
// "configured"), or Validate's "any Delta field set requires non-empty
// sourceDirs" rule would self-trigger on every config that never mentions
// delta at all. An explicit outputDir always overrides the default
// (mirrors TestLoad_AppliesDefaultsForOmittedFields's branchFormat/runs
// coverage).
func TestLoad_PollSecondsAndDeltaOutputDir_DefaultsWhenOmitted(t *testing.T) {
	tests := []struct {
		name               string
		yaml               string
		wantInterval       int
		wantTimeout        int
		wantDeltaOutputDir string
	}{
		{
			name: "omitted poll fields default; delta.outputDir stays empty when delta is unconfigured",
			yaml: `
branches:
  integration: INT
`,
			wantInterval:       10,
			wantTimeout:        3600,
			wantDeltaOutputDir: "",
		},
		{
			name: "delta configured via sourceDirs gets a default outputDir when omitted",
			yaml: `
delta:
  sourceDirs:
    - force-app
`,
			wantInterval:       10,
			wantTimeout:        3600,
			wantDeltaOutputDir: ".deploydeck/manifest/delta",
		},
		{
			name: "explicit poll fields and delta.outputDir override defaults",
			yaml: `
pollIntervalSeconds: 5
pollTimeoutSeconds: 120

delta:
  sourceDirs:
    - force-app
  outputDir: custom/output
`,
			wantInterval:       5,
			wantTimeout:        120,
			wantDeltaOutputDir: "custom/output",
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

			if cfg.PollIntervalSeconds != tt.wantInterval {
				t.Errorf("PollIntervalSeconds = %d, want %d", cfg.PollIntervalSeconds, tt.wantInterval)
			}
			if cfg.PollTimeoutSeconds != tt.wantTimeout {
				t.Errorf("PollTimeoutSeconds = %d, want %d", cfg.PollTimeoutSeconds, tt.wantTimeout)
			}
			if cfg.Delta.OutputDir != tt.wantDeltaOutputDir {
				t.Errorf("Delta.OutputDir = %q, want %q", cfg.Delta.OutputDir, tt.wantDeltaOutputDir)
			}

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() unexpected error on a Load()-produced config: %v", err)
			}
		})
	}
}
