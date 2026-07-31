package delta_test

// TestService_Generate_RealSgd_MultiSourceDir is an "[I]" integration test
// closing the Phase 2 WARNING that the single multi-`--source-dir` invocation
// (design.md's "sgd multi-dir" decision: ONE call with repeated --source-dir,
// no per-dir merge fallback) was only ever exercised with a SINGLE source
// dir. Here a temp repo carries Salesforce metadata in TWO distinct source
// directories, both changed between origin/UAT and HEAD; the REAL
// `sf sgd source delta` runs with BOTH `--source-dir` flags, and the single
// merged package.xml is asserted to contain members from BOTH directories.
//
// Skips under -short (shells out to real git + sf) and when the sgd plugin /
// fixture are unavailable, mirroring generate_e2e_test.go's conventions. It
// reuses that file's runGit / memberOfType helpers (same delta_test package).

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/delta"
	"github.com/malavolta/DeployDeck/internal/exec"
)

// apexClass is a minimal, org-independent Apex class body; sgd only diffs the
// files and reads the SFDX source layout, it never compiles against an org.
func apexClass(name, body string) string {
	return "public with sharing class " + name + " {\n" +
		"    public static String ping() { return '" + body + "'; }\n" +
		"}\n"
}

const apexClassMeta = `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>59.0</apiVersion>
    <status>Active</status>
</ApexClass>
`

// writeSourceFile writes content to dir/rel, creating parent directories.
func writeSourceFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", rel, err)
	}
}

// seedMultiSourceDirRepo builds a temp repo with two SFDX package directories
// (packages/pkg-a, packages/pkg-b), each holding one Apex class. The baseline
// is exposed as origin/UAT; HEAD then edits BOTH classes, so a delta scoped to
// both --source-dir entries must surface a member from each directory.
func seedMultiSourceDirRepo(t *testing.T) (dir string, runner exec.Runner) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real-sgd multi-source-dir integration test in -short mode")
	}

	runner = exec.NewOSRunner()
	dir = t.TempDir()

	runGit(t, runner, dir, "init", "-b", "main")
	runGit(t, runner, dir, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")

	writeSourceFile(t, dir, "sfdx-project.json", `{
  "packageDirectories": [
    { "path": "packages/pkg-a", "default": true },
    { "path": "packages/pkg-b" }
  ],
  "name": "multi-source-dir",
  "namespace": "",
  "sourceApiVersion": "59.0"
}
`)
	writeSourceFile(t, dir, "packages/pkg-a/main/default/classes/AlphaService.cls", apexClass("AlphaService", "alpha"))
	writeSourceFile(t, dir, "packages/pkg-a/main/default/classes/AlphaService.cls-meta.xml", apexClassMeta)
	writeSourceFile(t, dir, "packages/pkg-b/main/default/classes/BetaService.cls", apexClass("BetaService", "beta"))
	writeSourceFile(t, dir, "packages/pkg-b/main/default/classes/BetaService.cls-meta.xml", apexClassMeta)

	runGit(t, runner, dir, "add", "-A")
	runGit(t, runner, dir, "commit", "-m", "seed: two SFDX package dirs, one Apex class each")

	// origin/UAT is the pre-change baseline (HU-007's --from origin/<target>).
	runGit(t, runner, dir, "branch", "UAT")
	runGit(t, runner, dir, "remote", "add", "origin", dir)
	runGit(t, runner, dir, "update-ref", "refs/remotes/origin/UAT", "refs/heads/UAT")

	// Change a class in BOTH source dirs so the delta must span both.
	writeSourceFile(t, dir, "packages/pkg-a/main/default/classes/AlphaService.cls", apexClass("AlphaService", "alpha-v2"))
	writeSourceFile(t, dir, "packages/pkg-b/main/default/classes/BetaService.cls", apexClass("BetaService", "beta-v2"))

	runGit(t, runner, dir, "add", "-A")
	runGit(t, runner, dir, "commit", "-m", "edit AlphaService (pkg-a) and BetaService (pkg-b)")

	return dir, runner
}

func TestService_Generate_RealSgd_MultiSourceDir(t *testing.T) {
	dir, runner := seedMultiSourceDirRepo(t)

	outputDir := filepath.Join(dir, ".deploydeck", "manifest", "delta", "PROJ-9-to-UAT")
	svc := delta.New(runner)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := svc.Generate(ctx, delta.Request{
		Dir:        dir,
		From:       "origin/UAT",
		To:         "HEAD",
		OutputDir:  outputDir,
		SourceDirs: []string{"packages/pkg-a", "packages/pkg-b"},
	})
	if err != nil {
		t.Fatalf("Generate() with two --source-dir flags failed against real sgd: %v", err)
	}
	if result.PackageXMLPath == "" {
		t.Fatal("expected a non-empty PackageXMLPath")
	}

	pkgData, err := os.ReadFile(result.PackageXMLPath)
	if err != nil {
		t.Fatalf("failed to read the merged package.xml: %v", err)
	}
	pkg, err := delta.ParsePackage(pkgData)
	if err != nil {
		t.Fatalf("ParsePackage() on the merged sgd output: %v", err)
	}

	// The single package.xml must merge members from BOTH source dirs.
	if !memberOfType(pkg, "ApexClass", "AlphaService") {
		t.Errorf("merged package.xml missing AlphaService (from packages/pkg-a); got %+v", pkg.Types)
	}
	if !memberOfType(pkg, "ApexClass", "BetaService") {
		t.Errorf("merged package.xml missing BetaService (from packages/pkg-b); got %+v", pkg.Types)
	}
}
