package prereq_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/exec"
)

func runGit(t *testing.T, runner exec.Runner, dir string, args ...string) exec.CommandResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "git",
		Args: args,
		Dir:  dir,
		Env: []string{
			"GIT_EDITOR=true",
			"GIT_TERMINAL_PROMPT=0",
			"GIT_PAGER=cat",
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

// newTempRepo inits a real temp git repository (mirroring internal/git's
// own newTempRepo helper, duplicated here since Go test helpers in _test.go
// files are not importable across packages). withOrigin controls whether a
// bare "origin" remote is wired, for HU-001's origin-check tests. Skips
// under -short since it shells out to a real git binary.
func newTempRepo(t *testing.T, withOrigin bool) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}

	runner := exec.NewOSRunner()
	dir := t.TempDir()

	runGit(t, runner, dir, "init", "-b", "main")
	runGit(t, runner, dir, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")

	if withOrigin {
		remoteDir := t.TempDir()
		runGit(t, runner, remoteDir, "init", "--bare")
		runGit(t, runner, dir, "remote", "add", "origin", remoteDir)
	}

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# deploydeck temp repo\n"), 0o644); err != nil {
		t.Fatalf("failed to seed README.md: %v", err)
	}
	runGit(t, runner, dir, "add", "README.md")
	runGit(t, runner, dir, "commit", "-m", "chore: initial commit")

	if withOrigin {
		runGit(t, runner, dir, "push", "origin", "main")
	}

	return dir
}

// newNestedTempRepo mirrors newTempRepo's exact init/config/(bare-origin)/
// README.md commit sequence, then adds a committed "project" subdirectory —
// simulating a repo whose SFDX project root is nested below the git root
// (directory-resolution design.md's "Test seams" — a sibling helper, never
// modifying newTempRepo, so every existing flat test keeps its exact
// harness). Returns BOTH paths so each assertion names the root it means.
// Skips under -short, same as newTempRepo.
func newNestedTempRepo(t *testing.T, withOrigin bool) (gitRoot, projectDir string) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}

	runner := exec.NewOSRunner()
	dir := t.TempDir()

	runGit(t, runner, dir, "init", "-b", "main")
	runGit(t, runner, dir, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")

	if withOrigin {
		remoteDir := t.TempDir()
		runGit(t, runner, remoteDir, "init", "--bare")
		runGit(t, runner, dir, "remote", "add", "origin", remoteDir)
	}

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# deploydeck temp repo\n"), 0o644); err != nil {
		t.Fatalf("failed to seed README.md: %v", err)
	}
	runGit(t, runner, dir, "add", "README.md")
	runGit(t, runner, dir, "commit", "-m", "chore: initial commit")

	project := filepath.Join(dir, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("failed to create nested project dir: %v", err)
	}
	sentinelPath := filepath.Join(project, "sfdx-project.json")
	if err := os.WriteFile(sentinelPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed nested sfdx-project.json: %v", err)
	}
	runGit(t, runner, dir, "add", "project/sfdx-project.json")
	runGit(t, runner, dir, "commit", "-m", "chore: seed nested project dir")

	if withOrigin {
		runGit(t, runner, dir, "push", "origin", "main")
	}

	return dir, project
}
