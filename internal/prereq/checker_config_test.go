package prereq_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/prereq"
)

// TestChecker_CheckConfig_Blocking is config-validation-wiring task 2.2
// (RED): a Config that fails Validate() reports StatusBlocking, Detail
// equal to the Validate() error VERBATIM, and a FixCommand naming the
// resolved ConfigPath — HU-001's PrereqCheck contract, applied to config
// validity (prereq-check spec: "Config Validity Check (Blocking)").
func TestChecker_CheckConfig_Blocking(t *testing.T) {
	cfg, path := loadMinimalConfig(t, "sandboxes:\n  UAT:\n    testLevel: RunLocalTests\n")

	checker := &prereq.Checker{Config: cfg, ConfigPath: path}

	check, err := checker.CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected StatusBlocking, got %+v", check)
	}

	wantDetail := cfg.Validate().Error()
	if check.Detail != wantDetail {
		t.Errorf("Detail = %q, want the Validate() error verbatim %q", check.Detail, wantDetail)
	}
	wantFix := "edit " + path
	if check.FixCommand != wantFix {
		t.Errorf("FixCommand = %q, want %q", check.FixCommand, wantFix)
	}
}

// TestChecker_CheckConfig_OK_LoadedMinimalConfig is task 2.2 (RED): a
// config.Load-produced (i.e. defaulted) minimal Config passes, and Detail
// discloses WHICH file was located.
func TestChecker_CheckConfig_OK_LoadedMinimalConfig(t *testing.T) {
	cfg, path := loadMinimalConfig(t, "branches:\n  integration: INT\n")

	checker := &prereq.Checker{Config: cfg, ConfigPath: path}

	check, err := checker.CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected StatusOK for a Load-produced minimal config, got %+v", check)
	}
	if check.Detail != path+" is valid" {
		t.Errorf("Detail = %q, want it to name the resolved path %q", check.Detail, path)
	}
}

// TestChecker_CheckConfig_ZeroValueConfigPath_DegradesToBareFileName is task
// 2.2 (RED): an unset ConfigPath (the zero value) never produces an empty
// FixCommand operand or a panic — it degrades to config.FileName.
func TestChecker_CheckConfig_ZeroValueConfigPath_DegradesToBareFileName(t *testing.T) {
	checker := &prereq.Checker{Config: config.Config{}} // deliberately un-defaulted: forces the block

	check, err := checker.CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected a bare config.Config{} literal to block (pollIntervalSeconds), got %+v", check)
	}
	if check.FixCommand != "edit "+config.FileName {
		t.Errorf("FixCommand = %q, want %q (zero-value ConfigPath degrades to the bare file name)", check.FixCommand, "edit "+config.FileName)
	}
}

// TestChecker_CheckConfig_DeltaConfiguredWithoutSourceDirs_BlocksNamingSourceDirs
// is task 2.2 (RED): the delta rule #8 that motivated this whole change —
// delta.outputDir set with an empty sourceDirs would otherwise silently
// scan nothing and produce an always-empty package (proposal "Intent").
func TestChecker_CheckConfig_DeltaConfiguredWithoutSourceDirs_BlocksNamingSourceDirs(t *testing.T) {
	cfg, path := loadMinimalConfig(t, "delta:\n  outputDir: .deploydeck/manifest/delta\n")

	checker := &prereq.Checker{Config: cfg, ConfigPath: path}

	check, err := checker.CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected delta configured with empty sourceDirs to block, got %+v", check)
	}
	if !strings.Contains(check.Detail, "sourceDirs") {
		t.Errorf("Detail = %q, want it to name sourceDirs", check.Detail)
	}
}
