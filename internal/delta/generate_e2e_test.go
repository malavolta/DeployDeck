package delta_test

// TestService_Generate_RealSgd_TempRepo is an "[I]" integration test (per
// tasks.md's HU-007 Phase 2): it seeds a temp git repo from the REAL
// test-e2e-org fixture's force-app/ + sfdx-project.json, commits a baseline
// then a change (an edit + a delete, to exercise both package.xml and
// destructiveChanges.xml), and runs the REAL `sf sgd source delta` (plugin
// installed) through internal/delta.Service — never a FakeRunner. Skips
// under -short since it shells out to real git and sf binaries, mirroring
// internal/git's newTempRepo convention.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/delta"
	"github.com/malavolta/DeployDeck/internal/exec"
)

// testE2EOrgFixtureDir is the real Salesforce Node fixture committed at the
// repo root (openspec/config.yaml: "Fixture: test-e2e-org/ is a Salesforce
// Node fixture for e2e only, not the app stack"), two levels up from this
// package.
const testE2EOrgFixtureDir = "../../test-e2e-org"

func runGit(t *testing.T, runner exec.Runner, dir string, args ...string) exec.CommandResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "git",
		Args: args,
		Dir:  dir,
		Env: []string{
			"GIT_EDITOR=true",
			"GIT_TERMINAL_PROMPT=0",
			"GIT_PAGER=cat",
			// Tool-created commits never need a signature and must never
			// hang waiting on a no-tty gpg prompt.
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=commit.gpgsign",
			"GIT_CONFIG_VALUE_0=false",
		},
	})
	if err != nil {
		t.Fatalf("git %v failed to run: %v (stderr: %s)", args, err, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("git %v exited %d: %s", args, result.ExitCode, result.Stderr)
	}
	return result
}

// copyFixtureTree copies src (a file OR a directory tree) to dst, used to
// seed the temp repo from the real, committed test-e2e-org fixture without
// ever mutating the fixture itself.
func copyFixtureTree(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("failed to copy fixture tree %s -> %s: %v", src, dst, err)
	}
}

// seedSgdRepo initializes a temp git repo seeded from test-e2e-org's real
// force-app/ + sfdx-project.json, with a baseline commit exposed as
// origin/UAT, then a second commit that edits AccountService.cls (produces
// package.xml) and deletes AccountServiceTest.cls + its -meta.xml (produces
// destructiveChanges.xml) — real Salesforce metadata, real deletions.
func seedSgdRepo(t *testing.T) (dir string, runner exec.Runner) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real-sgd integration test in -short mode")
	}

	fixture, err := filepath.Abs(testE2EOrgFixtureDir)
	if err != nil {
		t.Fatalf("failed to resolve fixture path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture, "force-app")); err != nil {
		t.Skipf("test-e2e-org fixture not found at %s: %v", fixture, err)
	}

	runner = exec.NewOSRunner()
	dir = t.TempDir()

	runGit(t, runner, dir, "init", "-b", "main")
	runGit(t, runner, dir, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")

	copyFixtureTree(t, filepath.Join(fixture, "force-app"), filepath.Join(dir, "force-app"))
	copyFixtureTree(t, filepath.Join(fixture, "sfdx-project.json"), filepath.Join(dir, "sfdx-project.json"))

	runGit(t, runner, dir, "add", "-A")
	runGit(t, runner, dir, "commit", "-m", "seed: test-e2e-org force-app baseline")

	// origin/UAT is the pre-change baseline every sgd --from comparison
	// runs against, mirroring the real app's promotion flow (HU-007's
	// `--from origin/<target>`). No real remote is needed: a self-pointing
	// origin + a manually written origin/UAT ref is enough for sgd's git
	// diff to resolve the ref.
	runGit(t, runner, dir, "branch", "UAT")
	runGit(t, runner, dir, "remote", "add", "origin", dir)
	runGit(t, runner, dir, "update-ref", "refs/remotes/origin/UAT", "refs/heads/UAT")

	classesDir := filepath.Join(dir, "force-app", "main", "default", "classes")
	accountServicePath := filepath.Join(classesDir, "AccountService.cls")
	existing, err := os.ReadFile(accountServicePath)
	if err != nil {
		t.Fatalf("failed to read AccountService.cls fixture: %v", err)
	}
	edited := append(append([]byte{}, existing...), []byte("\n// delta-validation integration test edit\n")...)
	if err := os.WriteFile(accountServicePath, edited, 0o644); err != nil {
		t.Fatalf("failed to edit AccountService.cls: %v", err)
	}
	if err := os.Remove(filepath.Join(classesDir, "AccountServiceTest.cls")); err != nil {
		t.Fatalf("failed to delete AccountServiceTest.cls: %v", err)
	}
	if err := os.Remove(filepath.Join(classesDir, "AccountServiceTest.cls-meta.xml")); err != nil {
		t.Fatalf("failed to delete AccountServiceTest.cls-meta.xml: %v", err)
	}

	runGit(t, runner, dir, "add", "-A")
	runGit(t, runner, dir, "commit", "-m", "edit AccountService.cls, delete AccountServiceTest.cls")

	return dir, runner
}

// seedNestedSgdRepo mirrors seedSgdRepo's EXACT init/config/baseline/edit/
// delete sequence, but nests the SFDX project one level below the git root
// (<gitRoot>/project/force-app, <gitRoot>/project/sfdx-project.json)
// instead of at the git root itself — the regression guard for this
// change's worst failure mode: a silently empty-but-successful delta
// package from sgd's implicit "./" repo-dir default (proposal R5, design.md
// "Budget": "not a lever"). Returns gitRoot so callers pass it as both the
// child cwd AND --repo-dir.
func seedNestedSgdRepo(t *testing.T) (gitRoot string, runner exec.Runner) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real-sgd nested integration test in -short mode")
	}

	fixture, err := filepath.Abs(testE2EOrgFixtureDir)
	if err != nil {
		t.Fatalf("failed to resolve fixture path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture, "force-app")); err != nil {
		t.Skipf("test-e2e-org fixture not found at %s: %v", fixture, err)
	}

	runner = exec.NewOSRunner()
	gitRoot = t.TempDir()
	project := filepath.Join(gitRoot, "project")

	runGit(t, runner, gitRoot, "init", "-b", "main")
	runGit(t, runner, gitRoot, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, gitRoot, "config", "user.email", "deploydeck-test@example.com")

	copyFixtureTree(t, filepath.Join(fixture, "force-app"), filepath.Join(project, "force-app"))
	copyFixtureTree(t, filepath.Join(fixture, "sfdx-project.json"), filepath.Join(project, "sfdx-project.json"))

	runGit(t, runner, gitRoot, "add", "-A")
	runGit(t, runner, gitRoot, "commit", "-m", "seed: nested test-e2e-org force-app baseline")

	runGit(t, runner, gitRoot, "branch", "UAT")
	runGit(t, runner, gitRoot, "remote", "add", "origin", gitRoot)
	runGit(t, runner, gitRoot, "update-ref", "refs/remotes/origin/UAT", "refs/heads/UAT")

	classesDir := filepath.Join(project, "force-app", "main", "default", "classes")
	accountServicePath := filepath.Join(classesDir, "AccountService.cls")
	existing, err := os.ReadFile(accountServicePath)
	if err != nil {
		t.Fatalf("failed to read AccountService.cls fixture: %v", err)
	}
	edited := append(append([]byte{}, existing...), []byte("\n// nested delta-validation integration test edit\n")...)
	if err := os.WriteFile(accountServicePath, edited, 0o644); err != nil {
		t.Fatalf("failed to edit AccountService.cls: %v", err)
	}

	runGit(t, runner, gitRoot, "add", "-A")
	runGit(t, runner, gitRoot, "commit", "-m", "edit project/force-app/.../AccountService.cls")

	return gitRoot, runner
}

// TestService_Generate_RealSgd_NestedRepo is tasks 7.1/7.2 (RED/GREEN): with
// the SFDX project nested one level below the git root, sourceDirs set to
// the repo-root-relative "project/force-app" (directory-resolution spec's
// sourceDirs INVARIANT — unaffected by projectDir), and Request.Dir ==
// gitRoot (both the child cwd AND the emitted --repo-dir), the REAL sgd
// invocation produces a NON-EMPTY package.xml — the only guard against a
// silently empty-but-successful delta package that validates green and
// deploys nothing (proposal R5).
func TestService_Generate_RealSgd_NestedRepo(t *testing.T) {
	gitRoot, runner := seedNestedSgdRepo(t)

	outputDir := filepath.Join(gitRoot, "project", ".deploydeck", "manifest", "delta", "PROJ-1-to-UAT")
	svc := delta.New(runner)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := svc.Generate(ctx, delta.Request{
		Dir:        gitRoot,
		From:       "origin/UAT",
		To:         "HEAD",
		OutputDir:  outputDir,
		SourceDirs: []string{"project/force-app"},
	})
	if err != nil {
		t.Fatalf("Generate() unexpected error against real sgd (nested layout): %v", err)
	}

	if result.PackageXMLPath == "" {
		t.Fatal("expected a non-empty PackageXMLPath")
	}
	pkgData, err := os.ReadFile(result.PackageXMLPath)
	if err != nil {
		t.Fatalf("failed to read generated package.xml: %v", err)
	}
	pkg, err := delta.ParsePackage(pkgData)
	if err != nil {
		t.Fatalf("ParsePackage() on real sgd output: %v", err)
	}
	// NON-EMPTY is the load-bearing assertion (task 7.2): a silent
	// implicit-cwd bug reproduces as sgd running with ZERO members found,
	// not as a hard error — this is the only guard that would catch it.
	if len(pkg.Types) == 0 {
		t.Fatal("expected a NON-EMPTY package.xml (<types> present) — an empty package here means sgd silently scanned the wrong repo root")
	}
	if !memberOfType(pkg, "ApexClass", "AccountService") {
		t.Fatalf("expected package.xml to list AccountService under ApexClass, got %+v", pkg.Types)
	}

	if result.Raw == "" {
		t.Error("expected Raw to capture the real sgd command output")
	}
}

// TestSgd_RepoDirIndependentOfCwd_RealSgd is config-validation-wiring task
// 9.1 (S-2): proves the ADR-7 (directory-resolution) separation is real —
// sgd's --repo-dir flag genuinely drives repository discovery
// independently of the child process's cwd, not merely "in addition to"
// it. It BYPASSES delta.Service entirely: Request.Dir drives both the
// child cwd AND the emitted --repo-dir from ONE field (ADR-7), so Service
// structurally cannot separate the two — TestService_Generate_RealSgd_NestedRepo
// above would still pass green even if --repo-dir were silently dropped
// from buildArgs, as long as cwd alone happened to be correct. This test
// sets the child cwd to the NESTED project dir while passing --repo-dir as
// the GIT root explicitly (a combination Service can never produce) — a
// passing result is explainable ONLY by sgd genuinely honoring --repo-dir
// over any cwd-based default.
func TestSgd_RepoDirIndependentOfCwd_RealSgd(t *testing.T) {
	gitRoot, runner := seedNestedSgdRepo(t)
	project := filepath.Join(gitRoot, "project")

	outputDir := filepath.Join(t.TempDir(), "delta")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("creating output dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{
			"sgd", "source", "delta",
			"--from", "origin/UAT",
			"--to", "HEAD",
			"--output-dir", outputDir,
			"--generate-delta",
			"--repo-dir", gitRoot,
			"--source-dir", "project/force-app",
		},
		// Dir is the NESTED project dir — deliberately DIFFERENT from
		// --repo-dir above, and never something delta.Service itself could
		// produce (its Request.Dir feeds both).
		Dir: project,
	})
	if err != nil {
		t.Fatalf("running sf sgd source delta directly: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("sf sgd source delta exited %d: %s", result.ExitCode, string(result.Stderr))
	}

	pkgData, err := os.ReadFile(filepath.Join(outputDir, "package", "package.xml"))
	if err != nil {
		t.Fatalf("reading generated package.xml: %v", err)
	}
	pkg, err := delta.ParsePackage(pkgData)
	if err != nil {
		t.Fatalf("ParsePackage() on real sgd output: %v", err)
	}
	// NON-EMPTY is the load-bearing assertion, same as the nested Service
	// test above: an empty package here means sgd silently scanned the
	// wrong repo root (fell back to cwd) rather than honoring --repo-dir.
	if len(pkg.Types) == 0 {
		t.Fatal("expected a NON-EMPTY package.xml — an empty package means sgd silently scanned the wrong repo root, ignoring --repo-dir")
	}
	if !memberOfType(pkg, "ApexClass", "AccountService") {
		t.Fatalf("expected package.xml to list AccountService under ApexClass, got %+v", pkg.Types)
	}
}

func TestService_Generate_RealSgd_TempRepo(t *testing.T) {
	dir, runner := seedSgdRepo(t)

	outputDir := filepath.Join(dir, ".deploydeck", "manifest", "delta", "PROJ-1-to-UAT")
	svc := delta.New(runner)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := svc.Generate(ctx, delta.Request{
		Dir:        dir,
		From:       "origin/UAT",
		To:         "HEAD",
		OutputDir:  outputDir,
		SourceDirs: []string{"force-app"},
	})
	if err != nil {
		t.Fatalf("Generate() unexpected error against real sgd: %v", err)
	}

	if result.PackageXMLPath == "" {
		t.Fatal("expected a non-empty PackageXMLPath")
	}
	pkgData, err := os.ReadFile(result.PackageXMLPath)
	if err != nil {
		t.Fatalf("failed to read generated package.xml: %v", err)
	}
	pkg, err := delta.ParsePackage(pkgData)
	if err != nil {
		t.Fatalf("ParsePackage() on real sgd output: %v", err)
	}
	if !memberOfType(pkg, "ApexClass", "AccountService") {
		t.Fatalf("expected package.xml to list AccountService under ApexClass, got %+v", pkg.Types)
	}

	if result.DestructiveChangesPath == "" {
		t.Fatal("expected a non-empty DestructiveChangesPath: the seed deleted AccountServiceTest.cls")
	}
	destructiveData, err := os.ReadFile(result.DestructiveChangesPath)
	if err != nil {
		t.Fatalf("failed to read generated destructiveChanges.xml: %v", err)
	}
	destructive, err := delta.ParseDestructive(destructiveData)
	if err != nil {
		t.Fatalf("ParseDestructive() on real sgd output: %v", err)
	}
	if !memberOfType(destructive, "ApexClass", "AccountServiceTest") {
		t.Fatalf("expected destructiveChanges.xml to list AccountServiceTest under ApexClass, got %+v", destructive.Types)
	}

	if result.Raw == "" {
		t.Error("expected Raw to capture the real sgd command output")
	}

	// Working tree stays clean: every entry in git status is untracked and
	// confined to .deploydeck/ — no TRACKED file was modified.
	status := runGit(t, runner, dir, "status", "--porcelain")
	for _, line := range strings.Split(strings.TrimRight(string(status.Stdout), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "??") {
			t.Fatalf("expected only untracked entries in git status, got a tracked change: %q", line)
		}
		if !strings.Contains(line, ".deploydeck/") {
			t.Fatalf("expected the only untracked entry to be under .deploydeck/, got: %q", line)
		}
	}
}

func memberOfType(pkg delta.Package, typeName, member string) bool {
	for _, ty := range pkg.Types {
		if ty.Name != typeName {
			continue
		}
		for _, m := range ty.Members {
			if m == member {
				return true
			}
		}
	}
	return false
}
