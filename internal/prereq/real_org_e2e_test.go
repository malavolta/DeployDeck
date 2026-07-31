package prereq_test

// TestE2ERealOrg_* — opt-in end-to-end test against a REAL, developer-owned
// Salesforce org. Per docs/ARQUITECTURA.md ("Tests E2E Local (Contra Org
// Real)"): activated only via DEPLOYDECK_E2E_ORG=<alias>; skipped (never
// failed) when unset, so `go test ./...` stays green in CI and for any
// contributor without a connected org. NEVER runs in CI — this file has no
// build tag because the env-var gate alone is the documented activation
// mechanism, and an unset var must produce a visible SKIP, not a silently
// excluded file.
//
// Non-destructive: every call here is read-only (`sf --version`,
// `sf plugins --json`, `sf org list --json`). No deploy, no validate, no
// login, no org mutation.

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// e2eOrgAlias returns the DEPLOYDECK_E2E_ORG alias, skipping the calling
// test when it is unset. Called once per top-level test (not shared state)
// so each test reports its own explicit skip reason under -v.
func e2eOrgAlias(t *testing.T) string {
	t.Helper()
	alias := os.Getenv("DEPLOYDECK_E2E_ORG")
	if alias == "" {
		t.Skip("set DEPLOYDECK_E2E_ORG=<alias> to run the real-org e2e (local only)")
	}
	return alias
}

var cliVersionShape = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// TestE2ERealOrg_Smoke exercises the slice's real read-only Salesforce
// surface (internal/salesforce.Client + internal/prereq.Checker.CheckAliases)
// against the live org identified by DEPLOYDECK_E2E_ORG, using the real
// exec.NewOSRunner() (real sf/git), never FakeRunner.
func TestE2ERealOrg_Smoke(t *testing.T) {
	alias := e2eOrgAlias(t)

	runner := exec.NewOSRunner()
	client := salesforce.New(runner)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("Version parses real sf --version", func(t *testing.T) {
		info, err := client.Version(ctx)
		if err != nil {
			t.Fatalf("sf --version: unexpected error: %v", err)
		}
		if info.Raw == "" {
			t.Fatal("expected non-empty Raw sf --version output")
		}
		if !cliVersionShape.MatchString(info.CLIVersion) {
			t.Fatalf("expected CLIVersion to look like a semver (e.g. 2.135.7), got %q (Raw: %q)", info.CLIVersion, info.Raw)
		}
	})

	t.Run("Plugins parses real sf plugins --json top-level array", func(t *testing.T) {
		// This is the exact integration a FakeRunner cannot validate: real
		// `sf plugins --json` emits a TOP-LEVEL JSON ARRAY, not the
		// {"status":0,"result":...} envelope every other sf command uses.
		// The earlier field-mapping bug (see internal/salesforce/plugins.go)
		// lived exactly in this shape mismatch.
		plugins, err := client.Plugins(ctx)
		if err != nil {
			t.Fatalf("sf plugins --json: unexpected error: %v", err)
		}
		if len(plugins) == 0 {
			t.Fatal("expected at least one installed sf plugin to be parsed from the real CLI")
		}
		for _, p := range plugins {
			if p.Name == "" {
				t.Fatalf("expected every parsed plugin to have a non-empty Name, got %+v", p)
			}
		}
	})

	t.Run("Orgs parses real sf org list --json and FindByAlias resolves the connected alias", func(t *testing.T) {
		orgs, err := client.Orgs(ctx)
		if err != nil {
			t.Fatalf("sf org list --json: unexpected error: %v", err)
		}

		org, found := orgs.FindByAlias(alias)
		if !found {
			t.Fatalf("expected alias %q to be found across all five sf org list --json categories (nonScratchOrgs/scratchOrgs/sandboxes/devHubs/other); got OrgList: %+v", alias, orgs)
		}
		if org.Alias != alias {
			t.Fatalf("expected resolved org.Alias == %q, got %q", alias, org.Alias)
		}
		if org.Username == "" {
			t.Fatalf("expected the resolved org to have a non-empty Username, got %+v", org)
		}
	})

	t.Run("prereq CheckAliases resolves the connected alias as OK, not blocking", func(t *testing.T) {
		// Real classification note: sf org list --json currently returns this
		// connected DevHub/scratch-less org under nonScratchOrgs/other, not
		// sandboxes (verified via `sf org list --json` against AM-DEV-EDITION).
		// CheckAliases/FindByAlias scan all five categories regardless, so this
		// assertion still proves real alias resolution against the live org —
		// it does not assume a "sandboxes" category classification.
		const branch = "e2e-real-org"
		checker := &prereq.Checker{
			SF: client,
			Config: config.Config{
				Sandboxes: map[string]config.SandboxConfig{
					branch: {Alias: alias, TestLevel: "NoTestRun"},
				},
			},
		}

		checks, err := checker.CheckAliases(ctx)
		if err != nil {
			t.Fatalf("CheckAliases: unexpected error: %v", err)
		}

		var found *prereq.PrereqCheck
		for i := range checks {
			if checks[i].Name == "sandbox alias ("+branch+")" {
				found = &checks[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("expected a %q check in the report, got %+v", "sandbox alias ("+branch+")", checks)
		}
		if found.Status != prereq.StatusOK {
			t.Fatalf("expected the connected alias %q to resolve as OK (no unauthenticated-sandbox warning/block), got %+v", alias, *found)
		}
	})
}
