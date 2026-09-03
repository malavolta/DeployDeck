package prereq_test

import (
	"context"
	"os"
	"path/filepath"
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
	// ADR-6 GUARD (do not "tidy" this fixture into a valid config): a bare
	// literal is un-defaulted, so CheckConfig MUST block on pollIntervalSeconds.
	// This is currently the only test that fails if CheckConfig ever starts
	// applying defaults to a local copy — which would report "valid" for a
	// config the app then runs with zero poll values. See also
	// TestChecker_CheckConfig_BareLiteral_BlocksPerADR6 below.
	checker := &prereq.Checker{Config: config.Config{}}

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

// TestChecker_CheckConfig_MultipleViolations_ReportsOneAtATime closes the
// verify phase's CRITICAL-1: the spec scenario "Multiple violations require
// multiple fix cycles" had no covering test.
//
// The implementation already behaves correctly — Validate() returns a single
// error and Detail copies it verbatim — but nothing pinned that. Without this
// test, a future change could make Detail multi-line and stay green, silently
// breaking the single-line render contract that BOTH surfaces depend on
// (cmd/deploydeck/main.go's `[%s] %s: %s\n` and internal/app/view.go's
// prerequisite rendering).
func TestChecker_CheckConfig_MultipleViolations_ReportsOneAtATime(t *testing.T) {
	// Two independent rules violated at once: a sandbox with no alias, and a
	// delta block with no sourceDirs.
	twoProblems := "sandboxes:\n  UAT:\n    alias: \"\"\n    testLevel: RunLocalTests\ndelta:\n  outputDir: .deploydeck/manifest/delta\n"

	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)
	if err := os.WriteFile(path, []byte(twoProblems), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	checker := &prereq.Checker{Config: cfg, ConfigPath: path}
	check, err := checker.CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("a config violating two rules must block, got %+v", check)
	}

	// The render contract: one violation, on one line.
	if strings.Contains(check.Detail, "\n") {
		t.Errorf("Detail must stay single-line (both renderers use a one-line format), got %q", check.Detail)
	}
	first := check.Detail

	// Fixing the first violation must surface the SECOND one, not silence.
	// This is what makes the "N problems, N cycles" cost in the spec real
	// rather than merely asserted.
	fixed := "sandboxes:\n  UAT:\n    alias: uat\n    testLevel: RunLocalTests\ndelta:\n  outputDir: .deploydeck/manifest/delta\n"
	dir2 := t.TempDir()
	path2 := filepath.Join(dir2, config.FileName)
	if err := os.WriteFile(path2, []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg2, err := config.Load(dir2)
	if err != nil {
		t.Fatal(err)
	}
	check2, err := (&prereq.Checker{Config: cfg2, ConfigPath: path2}).CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() unexpected error: %v", err)
	}
	if check2.Status != prereq.StatusBlocking {
		t.Fatalf("the second violation must still block after the first is fixed, got %+v", check2)
	}
	if check2.Detail == first {
		t.Errorf("after fixing the first violation the check still reports it (%q) — the next one never surfaces", first)
	}
	if !strings.Contains(check2.Detail, "sourceDirs") {
		t.Errorf("Detail = %q, want the remaining sourceDirs violation", check2.Detail)
	}
}

// TestChecker_CheckConfig_BareLiteral_BlocksPerADR6 closes the verify phase's
// WARNING-3. The ADR-6 invariant — CheckConfig must never apply defaults to a
// local copy — had exactly one guardian, and that guardian's declared subject
// was FixCommand path degradation. A contributor tidying that test toward its
// stated purpose would have deleted the only guard while every other test
// stayed green.
//
// This test exists for the invariant and nothing else, so its name says so.
func TestChecker_CheckConfig_BareLiteral_BlocksPerADR6(t *testing.T) {
	// A hand-built literal never went through applyDefaults, so its poll
	// fields are zero. CheckConfig must surface that rather than paper over
	// it: reporting OK here would bless a config the app then runs with
	// PollIntervalSeconds == 0.
	checker := &prereq.Checker{Config: config.Config{}, ConfigPath: "/repo/deploydeck.yaml"}

	check, err := checker.CheckConfig(context.Background())
	if err != nil {
		t.Fatalf("CheckConfig() unexpected error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("ADR-6 violated: CheckConfig applied defaults to an un-defaulted Config and reported %+v", check)
	}
	if !strings.Contains(check.Detail, "pollIntervalSeconds") {
		t.Errorf("Detail = %q, want the un-defaulted poll field named", check.Detail)
	}
}
