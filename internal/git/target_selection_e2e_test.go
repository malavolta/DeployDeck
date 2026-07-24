package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/salesforce"
)

// TestHU004_TargetSelection_E2E is the consolidated end-to-end scenario
// from docs/HISTORIAS.md HU-004's "Test E2E" section, run through the real
// HU-004 target-selection entry points (ListDestinations, ResolveSandbox,
// Service.RemoteHead, Service.BranchExists, IsProductionBranch,
// SandboxAuthWarning, ConfirmTargetSelection) against the documented
// harness: "config YAML fixture + repo git temporal + sf org list falso"
// (fs config fixture, a real temp git repo with origin refs, and a
// FakeRunner-canned `sf org list --json`).
func TestHU004_TargetSelection_E2E(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// --- fs: write the config fixture into the repo root, per HU-004's
	// documented "Config esperada", plus a deliberately unmapped
	// destination (INT — no sandbox entry) and one destination whose
	// branch never gets created (STAGING, seeded below). ---
	writeConfigFixture(t, dir, `
branches:
  integration: INT
  uat: UAT
  staging: STAGING
  production: main

sandboxes:
  UAT:
    alias: UAT_SANDBOX
    testLevel: RunLocalTests
  Release/*:
    alias: PREPROD_SANDBOX
    testLevel: RunLocalTests
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: unexpected error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("cfg.Validate: unexpected error: %v", err)
	}

	// --- temp: seed UAT at a known HEAD, INT (existing branch, but no
	// sandbox mapping), and a Release/* branch that resolves its sandbox by
	// pattern. STAGING is deliberately left uncreated — the
	// nonexistent-destination variant, below. ---
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeAndCommit(t, runner, dir, "uat.txt", "uat\n", "chore: seed UAT")
	runGit(t, runner, dir, "push", "origin", "UAT")
	knownUATHead := trimNewline(string(runGit(t, runner, dir, "rev-parse", "origin/UAT").Stdout))

	runGit(t, runner, dir, "checkout", "-b", "INT", "main")
	runGit(t, runner, dir, "push", "origin", "INT")

	runGit(t, runner, dir, "checkout", "-b", "Release/Julio2026", "main")
	runGit(t, runner, dir, "push", "origin", "Release/Julio2026")

	ctx := context.Background()

	// === Assert: each configured destination resolves its alias and test
	// level, or is flagged unresolved when unmapped. ===
	destinations := git.ListDestinations(cfg)
	byEnv := make(map[string]git.Destination, len(destinations))
	for _, d := range destinations {
		byEnv[d.Environment] = d
	}

	uatDest, ok := byEnv["uat"]
	if !ok || !uatDest.SandboxResolved || uatDest.Sandbox.Alias != "UAT_SANDBOX" || uatDest.Sandbox.TestLevel != "RunLocalTests" {
		t.Fatalf("expected uat destination resolved to UAT_SANDBOX/RunLocalTests, got %+v (ok=%v)", uatDest, ok)
	}

	intDest, ok := byEnv["integration"]
	if !ok || intDest.SandboxResolved {
		t.Fatalf("expected the integration destination (INT) listed but unresolved (no sandbox mapping), got %+v (ok=%v)", intDest, ok)
	}

	// === Assert: Release/* resolves its sandbox via the configured glob
	// pattern; the unmapped INT destination blocks with an actionable
	// message. ===
	releaseSandbox, releaseBlock, err := git.ResolveSandbox(cfg, "Release/Julio2026")
	if err != nil {
		t.Fatalf("ResolveSandbox(Release/Julio2026): unexpected error: %v", err)
	}
	if releaseBlock != "" || releaseSandbox.Alias != "PREPROD_SANDBOX" || releaseSandbox.TestLevel != "RunLocalTests" {
		t.Fatalf("expected Release/Julio2026 to resolve PREPROD_SANDBOX/RunLocalTests via pattern, got %+v (block=%q)", releaseSandbox, releaseBlock)
	}

	_, intBlock, err := git.ResolveSandbox(cfg, "INT")
	if err == nil {
		t.Fatalf("expected ResolveSandbox(INT) to error: no sandbox mapping is configured for it")
	}
	if intBlock == "" {
		t.Fatalf("expected an actionable block message for the unmapped INT destination")
	}

	// === Assert: the destination branch's remote HEAD is shown via
	// `git rev-parse origin/<target>`. ===
	uatHead, uatExists, err := svc.RemoteHead(ctx, dir, "UAT")
	if err != nil {
		t.Fatalf("RemoteHead(UAT): unexpected error: %v", err)
	}
	if !uatExists || uatHead != knownUATHead {
		t.Fatalf("expected RemoteHead(UAT) = %q (exists), got %q (exists=%v)", knownUATHead, uatHead, uatExists)
	}

	// === Assert: main marks the production-environment warning; UAT does
	// not. ===
	if !git.IsProductionBranch("main") {
		t.Errorf("expected main to trigger the production-environment warning")
	}
	if git.IsProductionBranch("UAT") {
		t.Errorf("did not expect UAT to trigger the production-environment warning")
	}

	// === Assert: confirming a valid UAT + resolved sandbox selection saves
	// branch/alias/testLevel into the DeploymentPlan. ===
	plan := git.DeploymentPlan{Ticket: "PROJ-1"}
	plan = git.ConfirmTargetSelection(plan, uatDest.Branch, uatDest.Sandbox.Alias, uatDest.Sandbox.TestLevel)
	if plan.TargetBranch != "UAT" || plan.SandboxAlias != "UAT_SANDBOX" || plan.TestLevel != "RunLocalTests" {
		t.Fatalf("expected the DeploymentPlan to carry the confirmed selection, got %+v", plan)
	}
	if plan.Ticket != "PROJ-1" {
		t.Fatalf("expected ConfirmTargetSelection to preserve the plan's existing Ticket, got %q", plan.Ticket)
	}

	// === Variant: a configured destination branch that does not exist in
	// the repo blocks continuing. ===
	t.Run("nonexistent destination branch blocks the flow", func(t *testing.T) {
		exists, err := svc.BranchExists(ctx, dir, "STAGING")
		if err != nil {
			t.Fatalf("BranchExists(STAGING): unexpected error: %v", err)
		}
		if exists {
			t.Fatalf("expected STAGING (never created) to be reported as nonexistent")
		}

		stillExists, err := svc.BranchExists(ctx, dir, "UAT")
		if err != nil {
			t.Fatalf("BranchExists(UAT): unexpected error: %v", err)
		}
		if !stillExists {
			t.Fatalf("expected UAT (created above) to be reported as existing")
		}
	})

	// === Variant: an unauthenticated sandbox alias shows a non-blocking
	// warning (sf-fake: UAT_SANDBOX is authenticated, PREPROD_SANDBOX is
	// not). ===
	t.Run("unauthenticated sandbox shows a warning", func(t *testing.T) {
		fr := exec.NewFakeRunner()
		fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{
			ExitCode: 0,
			Stdout: []byte(`{"status":0,"result":{
				"sandboxes":[{"alias":"UAT_SANDBOX","username":"a@b.com"}]
			}}`),
		})
		sf := salesforce.New(fr)

		uatWarned, err := git.SandboxAuthWarning(ctx, sf, "UAT_SANDBOX")
		if err != nil {
			t.Fatalf("SandboxAuthWarning(UAT_SANDBOX): unexpected error: %v", err)
		}
		if uatWarned {
			t.Errorf("did not expect a warning for the authenticated UAT_SANDBOX alias")
		}

		releaseWarned, err := git.SandboxAuthWarning(ctx, sf, "PREPROD_SANDBOX")
		if err != nil {
			t.Fatalf("SandboxAuthWarning(PREPROD_SANDBOX): unexpected error: %v", err)
		}
		if !releaseWarned {
			t.Errorf("expected a warning for the unauthenticated PREPROD_SANDBOX alias")
		}
	})
}

func writeConfigFixture(t *testing.T, dir, contents string) {
	t.Helper()
	path := filepath.Join(dir, config.FileName)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write config fixture: %v", err)
	}
}
