package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// realTempDir returns a fresh t.TempDir(), resolved through
// filepath.EvalSymlinks — on macOS, t.TempDir() lives under /var, itself a
// symlink to /private/var, and `git rev-parse --show-toplevel` always
// returns the FULLY-RESOLVED path (mirrors internal/git/service_root_test.go's
// established convention). Without this, every git-backed assertion below
// would spuriously fail comparing an unresolved expected path against
// RepoRoot's resolved one.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", t.TempDir(), err)
	}
	return dir
}

// writeRootsConfig writes deploydeck.yaml with the given body under dir.
func writeRootsConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "deploydeck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing deploydeck.yaml: %v", err)
	}
}

// TestResolveRoots_FlatRepo is task 6.1 (RED): with deploydeck.yaml at the
// git root and no projectDir configured, all three roots coincide
// (directory-resolution spec: "Flat repo collapses all three roots to one
// directory").
func TestResolveRoots_FlatRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: shells out to a real git binary")
	}
	runner := exec.NewOSRunner()
	dir := realTempDir(t)
	gitCmd(t, runner, dir, "init", "-b", "main")
	gitCmd(t, runner, dir, "config", "user.name", "DeployDeck Test")
	gitCmd(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")
	writeRootsConfig(t, dir, "branches:\n  integration: INT\n")
	gitCmd(t, runner, dir, "add", "deploydeck.yaml")
	gitCmd(t, runner, dir, "commit", "-m", "chore: seed config")

	g := git.New(runner)
	r, cfg, err := resolveRoots(context.Background(), g, dir)
	if err != nil {
		t.Fatalf("resolveRoots() unexpected error: %v", err)
	}
	if r.GitRoot != dir || r.ProjectDir != dir || r.ArtifactsRoot != dir {
		t.Fatalf("flat repo roots = %+v, want all three == %q", r, dir)
	}
	// config-validation-wiring task 1.1 (RED): a flat repo collapses
	// ConfigDir into the same directory as the other three roots too —
	// deploydeck.yaml sits at the git root, so that is where it was found.
	if r.ConfigDir != dir {
		t.Errorf("ConfigDir = %q, want %q (flat repo: config file lives at the git root)", r.ConfigDir, dir)
	}
	if cfg.Branches["integration"] != "INT" {
		t.Fatalf("expected the loaded Config to carry parsed fields, got %+v", cfg)
	}
}

// TestResolveRoots_NestedRepo is task 6.1 (RED): with deploydeck.yaml found
// in a subdirectory of the git root and no explicit projectDir, the SFDX
// project root and artifacts root default to that subdirectory while the
// git root is the true repository top level — three DISTINCT roots
// (directory-resolution spec: "Nested repo resolves three distinct roots").
func TestResolveRoots_NestedRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: shells out to a real git binary")
	}
	runner := exec.NewOSRunner()
	gitRoot := realTempDir(t)
	gitCmd(t, runner, gitRoot, "init", "-b", "main")
	gitCmd(t, runner, gitRoot, "config", "user.name", "DeployDeck Test")
	gitCmd(t, runner, gitRoot, "config", "user.email", "deploydeck-test@example.com")

	projectDir := filepath.Join(gitRoot, "up_saln0001_giss_salesforce")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRootsConfig(t, projectDir, "branches:\n  integration: INT\n")
	gitCmd(t, runner, gitRoot, "add", "up_saln0001_giss_salesforce/deploydeck.yaml")
	gitCmd(t, runner, gitRoot, "commit", "-m", "chore: seed nested config")

	g := git.New(runner)
	// Invoked from inside the nested project subdirectory — the realistic
	// entry point (the operator cd'd into their SFDX project).
	r, _, err := resolveRoots(context.Background(), g, projectDir)
	if err != nil {
		t.Fatalf("resolveRoots() unexpected error: %v", err)
	}
	if r.GitRoot != gitRoot {
		t.Errorf("GitRoot = %q, want the true repository top level %q", r.GitRoot, gitRoot)
	}
	if r.ProjectDir != projectDir {
		t.Errorf("ProjectDir = %q, want the config file's directory %q", r.ProjectDir, projectDir)
	}
	if r.ArtifactsRoot != projectDir {
		t.Errorf("ArtifactsRoot = %q, want %q", r.ArtifactsRoot, projectDir)
	}
	if r.GitRoot == r.ProjectDir {
		t.Fatal("expected GitRoot and ProjectDir to be DISTINCT for a nested repo")
	}
}

// TestResolveRoots_NestedRepo_ExplicitProjectDir proves an explicit
// projectDir (relative to the git root) is honored even when
// deploydeck.yaml itself lives elsewhere (e.g. the git root).
func TestResolveRoots_NestedRepo_ExplicitProjectDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: shells out to a real git binary")
	}
	runner := exec.NewOSRunner()
	gitRoot := realTempDir(t)
	gitCmd(t, runner, gitRoot, "init", "-b", "main")
	gitCmd(t, runner, gitRoot, "config", "user.name", "DeployDeck Test")
	gitCmd(t, runner, gitRoot, "config", "user.email", "deploydeck-test@example.com")
	writeRootsConfig(t, gitRoot, "projectDir: project\nbranches:\n  integration: INT\n")
	gitCmd(t, runner, gitRoot, "add", "deploydeck.yaml")
	gitCmd(t, runner, gitRoot, "commit", "-m", "chore: seed config with explicit projectDir")

	g := git.New(runner)
	r, _, err := resolveRoots(context.Background(), g, gitRoot)
	if err != nil {
		t.Fatalf("resolveRoots() unexpected error: %v", err)
	}
	wantProjectDir := filepath.Join(gitRoot, "project")
	if r.ProjectDir != wantProjectDir {
		t.Errorf("ProjectDir = %q, want %q (explicit projectDir, relative to gitRoot)", r.ProjectDir, wantProjectDir)
	}
	if r.ArtifactsRoot != wantProjectDir {
		t.Errorf("ArtifactsRoot = %q, want %q", r.ArtifactsRoot, wantProjectDir)
	}
	// config-validation-wiring task 1.1 (RED): ConfigDir is a FOURTH,
	// genuinely distinct fact here — deploydeck.yaml was LOCATED at
	// gitRoot, while an explicit projectDir points the SFDX project root
	// somewhere else entirely (design.md ADR-3).
	if r.ConfigDir != gitRoot {
		t.Errorf("ConfigDir = %q, want %q (the directory deploydeck.yaml was actually found in)", r.ConfigDir, gitRoot)
	}
	if r.ConfigDir == r.ProjectDir {
		t.Fatal("expected ConfigDir and ProjectDir to be DISTINCT when projectDir is explicit")
	}
}

// TestResolveRoots_NotInsideGitRepo is task 6.1 (RED): outside any Git
// repository, the upward config search is skipped and Load(cwd) is
// attempted directly — so prereq's "not a git repository" diagnostic stays
// primary rather than being masked by a confusing search failure
// (directory-resolution spec: "cwd is outside any Git repository").
func TestResolveRoots_NotInsideGitRepo(t *testing.T) {
	// realTempDir, not t.TempDir: resolveRoots canonicalises its cwd before
	// resolving anything (so a symlinked checkout still searches upward), and
	// on macOS t.TempDir() hands back /var/... which is a symlink to
	// /private/var/.... The assertion below is about all three roots
	// COLLAPSING to the cwd, not about which spelling of that path is
	// returned, so the fixture uses the canonical form the same way every
	// other resolveRoots test in this file does.
	dir := realTempDir(t) // no git init at all
	writeRootsConfig(t, dir, "branches:\n  integration: INT\n")

	g := git.New(exec.NewOSRunner())
	r, _, err := resolveRoots(context.Background(), g, dir)
	if err != nil {
		t.Fatalf("resolveRoots() unexpected error: %v", err)
	}
	if r.GitRoot != dir || r.ProjectDir != dir || r.ArtifactsRoot != dir {
		t.Fatalf("outside-git-repo roots = %+v, want all three == %q (probes cwd only)", r, dir)
	}
}

// TestResolveRoots_NotInsideGitRepo_NoConfig proves the fallback preserves
// config.Load's own, already-actionable error verbatim — no additional,
// worse "config not found while searching upward" message is introduced.
func TestResolveRoots_NotInsideGitRepo_NoConfig(t *testing.T) {
	dir := t.TempDir() // no git init, no deploydeck.yaml

	g := git.New(exec.NewOSRunner())
	_, _, err := resolveRoots(context.Background(), g, dir)
	if err == nil {
		t.Fatal("expected an error when no deploydeck.yaml exists anywhere")
	}
	if !strings.Contains(err.Error(), "deploydeck.yaml") {
		t.Fatalf("expected the error to preserve config.Load's own diagnostic, got: %v", err)
	}
}

// TestResolveRoots_InvalidProjectDir_AbortsBeforeAnyCommandRuns is task 6.1
// (RED): an absolute or ".."-bearing projectDir aborts resolution with an
// actionable error naming the offending value — BEFORE any external
// command (git/sf/sgd) ever runs, since resolveRoots is a pure resolution
// step with no side effects of its own (directory-resolution spec:
// "Absolute/parent-traversal projectDir is rejected on the resolution
// path").
func TestResolveRoots_InvalidProjectDir_AbortsBeforeAnyCommandRuns(t *testing.T) {
	tests := []struct {
		name       string
		projectDir string
	}{
		{name: "absolute projectDir", projectDir: "/etc/passwd"},
		{name: "projectDir with .. segment", projectDir: "../escape"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir() // no git init needed: the guard fires before any command
			writeRootsConfig(t, dir, "projectDir: "+tt.projectDir+"\nbranches:\n  integration: INT\n")

			g := git.New(exec.NewOSRunner())
			_, _, err := resolveRoots(context.Background(), g, dir)
			if err == nil {
				t.Fatal("expected resolveRoots() to reject the invalid projectDir")
			}
			if !strings.Contains(err.Error(), tt.projectDir) {
				t.Errorf("expected the error to name the offending value %q, got: %v", tt.projectDir, err)
			}
		})
	}
}

// TestNewChecker_LockPathUsesArtifactsRoot is task 6.4's VERIFY: the
// .deploydeck/lock path is composed from ArtifactsRoot, never GitRoot — the
// lock, like every other .deploydeck/ artifact, must never relocate for an
// existing install (design.md ADR-2).
func TestNewChecker_LockPathUsesArtifactsRoot(t *testing.T) {
	gitRoot := t.TempDir()
	artifactsRoot := filepath.Join(gitRoot, "project")
	if err := os.MkdirAll(artifactsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	checker, err := newChecker(config.Config{}, roots{GitRoot: gitRoot, ProjectDir: artifactsRoot, ArtifactsRoot: artifactsRoot})
	if err != nil {
		t.Fatalf("newChecker() unexpected error: %v", err)
	}

	wantPath := filepath.Join(artifactsRoot, ".deploydeck", "lock")
	if got := checker.Lock.Path(); got != wantPath {
		t.Errorf("Lock.Path() = %q, want %q (rooted at ArtifactsRoot, not GitRoot)", got, wantPath)
	}
	if checker.GitRoot != gitRoot {
		t.Errorf("Checker.GitRoot = %q, want %q", checker.GitRoot, gitRoot)
	}
	if checker.ArtifactsRoot != artifactsRoot {
		t.Errorf("Checker.ArtifactsRoot = %q, want %q", checker.ArtifactsRoot, artifactsRoot)
	}
}

// TestNewChecker_SetsConfigPathFromConfigDir is config-validation-wiring
// task 1.1 (RED): newChecker composes Checker.ConfigPath from
// roots.ConfigDir, not GitRoot or ProjectDir — config.Locate searches
// UPWARD, so in a nested layout a bare filename would leave the operator
// unable to tell which of two candidate files CheckConfig's FixCommand
// means (design.md ADR-3).
func TestNewChecker_SetsConfigPathFromConfigDir(t *testing.T) {
	gitRoot := t.TempDir()
	configDir := filepath.Join(gitRoot, "up_saln0001_giss_salesforce")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	checker, err := newChecker(config.Config{}, roots{GitRoot: gitRoot, ProjectDir: configDir, ArtifactsRoot: configDir, ConfigDir: configDir})
	if err != nil {
		t.Fatalf("newChecker() unexpected error: %v", err)
	}

	wantPath := filepath.Join(configDir, config.FileName)
	if checker.ConfigPath != wantPath {
		t.Errorf("checker.ConfigPath = %q, want %q", checker.ConfigPath, wantPath)
	}
}

// TestResolveRoots_SymlinkedCheckout_StillSearchesUpward is the regression
// test for the verify phase's W-1 finding.
//
// `git rev-parse --show-toplevel` reports the symlink-RESOLVED path, while
// os.Getwd() reports the LOGICAL one. On a symlinked checkout the two differ
// as plain strings, so config.Locate's lexical bound check (isAncestorOrSelf)
// sees stopDir as unrelated to startDir, collapses the search to startDir
// alone, and the upward deploydeck.yaml search silently never engages —
// the operator gets "config: reading <cwd>/deploydeck.yaml: no such file"
// even though the config sits one level up, inside the same repository.
//
// resolveRoots therefore canonicalises the cwd BEFORE resolving anything
// else, so both sides of every subsequent comparison share one path space.
func TestResolveRoots_SymlinkedCheckout_StillSearchesUpward(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: shells out to a real git binary")
	}
	runner := exec.NewOSRunner()

	// A real repo with the config at its root and a plain subdirectory.
	realRoot := realTempDir(t)
	gitRoot := filepath.Join(realRoot, "repo")
	if err := os.MkdirAll(gitRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, runner, gitRoot, "init", "-b", "main")
	gitCmd(t, runner, gitRoot, "config", "user.name", "DeployDeck Test")
	gitCmd(t, runner, gitRoot, "config", "user.email", "deploydeck-test@example.com")
	writeRootsConfig(t, gitRoot, "branches:\n  integration: INT\n")
	sub := filepath.Join(gitRoot, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, runner, gitRoot, "add", "deploydeck.yaml")
	gitCmd(t, runner, gitRoot, "commit", "-m", "chore: seed root config")

	// Reach that same subdirectory through a symlink, the way a macOS
	// operator with a symlinked workspace does.
	link := filepath.Join(realTempDir(t), "link")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}
	linkedSub := filepath.Join(link, "repo", "sub")

	g := git.New(runner)
	r, _, err := resolveRoots(context.Background(), g, linkedSub)
	if err != nil {
		t.Fatalf("resolveRoots() through a symlinked path: %v", err)
	}
	if r.GitRoot != gitRoot {
		t.Errorf("GitRoot = %q, want the resolved top level %q", r.GitRoot, gitRoot)
	}
	// The upward search must have engaged: the config lives at gitRoot, not
	// in the subdirectory we started from.
	if r.ProjectDir != gitRoot {
		t.Errorf("ProjectDir = %q, want %q — the upward search did not engage through the symlink", r.ProjectDir, gitRoot)
	}
	if r.ArtifactsRoot != r.ProjectDir {
		t.Errorf("ArtifactsRoot = %q, want it to track ProjectDir %q", r.ArtifactsRoot, r.ProjectDir)
	}
}
