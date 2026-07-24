package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"deploydeck/internal/exec"
)

// runGitAllow runs a git command like runGit but does NOT fail the test on a
// non-zero exit code — only on a Runner start failure. Cherry-pick conflicts
// and empty picks exit non-zero as DATA, so seeding/driving those states
// needs a runner that treats a clean non-zero exit as expected.
func runGitAllow(t *testing.T, runner exec.Runner, dir string, args ...string) exec.CommandResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "git",
		Args: args,
		Dir:  dir,
		Env:  nonInteractiveGitEnv,
	})
	if err != nil {
		t.Fatalf("git %v failed to start: %v (stderr: %s)", args, err, result.Stderr)
	}
	return result
}

// seedConflictingFeature builds, on a fresh newTempRepo checkout, a target
// branch "UAT" and a "feature" branch with two commits: the first modifies
// a.cls in a way that conflicts with UAT, the second cleanly adds b.cls. It
// leaves the repo checked out on UAT (the promotion target), so a subsequent
// `git cherry-pick <feature1>^..<feature2>` reproduces a real multi-commit
// conflict with a populated .git/sequencer/todo. Returns the two feature
// commit SHAs in topological order (conflicting first).
func seedConflictingFeature(t *testing.T, runner exec.Runner, dir string) (feature1, feature2 string) {
	t.Helper()

	writeFileHelper(t, dir, "a.cls", "l1\nBASE\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "seed a.cls baseline")
	runGit(t, runner, dir, "push", "origin", "main")

	// Target branch UAT diverges a.cls so cherry-picking feature conflicts.
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "a.cls", "l1\nUAT\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "UAT diverges a.cls")
	runGit(t, runner, dir, "push", "origin", "UAT")

	// Feature branch off main: conflicting change then a clean add.
	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	writeFileHelper(t, dir, "a.cls", "l1\nFEATURE\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "PROJ-1: feature changes a.cls")
	feature1 = trimNewline(string(runGit(t, runner, dir, "rev-parse", "HEAD").Stdout))

	feature2 = writeAndCommit(t, runner, dir, "b.cls", "b\n", "PROJ-1: feature adds b.cls")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")
	return feature1, feature2
}

// writeFileHelper writes content to dir/name, failing the test on error.
func writeFileHelper(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}
