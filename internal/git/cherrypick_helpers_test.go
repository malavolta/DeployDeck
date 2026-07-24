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

// seedCleanFeature builds a "UAT" target and a "feature" branch whose two
// commits (touching DIFFERENT files than UAT) cherry-pick onto UAT with NO
// conflict. It leaves the repo checked out on UAT and returns the two
// feature commit SHAs (topological order). Used to prove the happy-path
// ordered application and content-equals-source property.
func seedCleanFeature(t *testing.T, runner exec.Runner, dir string) (feature1, feature2 string) {
	t.Helper()

	// Target branch UAT off main, unrelated to the files feature touches.
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "uat-only.txt", "uat\n")
	runGit(t, runner, dir, "add", "uat-only.txt")
	runGit(t, runner, dir, "commit", "-m", "UAT-only change")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	feature1 = writeAndCommit(t, runner, dir, "a.cls", "public class A {}\n", "PROJ-1: add A")
	feature2 = writeAndCommit(t, runner, dir, "b.cls", "public class B {}\n", "PROJ-1: add B")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")
	return feature1, feature2
}

// seedModifyDeleteConflict builds a UAT target that DELETES f.cls and a
// feature commit that MODIFIES it, so cherry-picking feature onto UAT
// produces a modify/delete (porcelain DU) conflict. Leaves the repo on UAT
// and returns the feature commit SHA.
func seedModifyDeleteConflict(t *testing.T, runner exec.Runner, dir string) (featureSHA string) {
	t.Helper()

	writeFileHelper(t, dir, "f.cls", "public class F {}\n")
	runGit(t, runner, dir, "add", "f.cls")
	runGit(t, runner, dir, "commit", "-m", "seed f.cls")
	runGit(t, runner, dir, "push", "origin", "main")

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	runGit(t, runner, dir, "rm", "f.cls")
	runGit(t, runner, dir, "commit", "-m", "UAT deletes f.cls")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	featureSHA = writeAndCommit(t, runner, dir, "f.cls", "public class F { int x; }\n", "PROJ-1: modify f.cls")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")
	return featureSHA
}

// seedBinaryConflict builds a UAT target and a feature commit that each
// change the SAME binary file (img.png) differently, so cherry-picking
// feature onto UAT produces a binary content conflict (porcelain UU on a
// binary blob). Leaves the repo on UAT and returns the feature commit SHA
// and the raw bytes of the feature ("theirs") version.
func seedBinaryConflict(t *testing.T, runner exec.Runner, dir string) (featureSHA string, theirs []byte) {
	t.Helper()

	base := []byte("\x00\x01\x02BIN\x00base\n")
	writeBytesHelper(t, dir, "img.png", base)
	runGit(t, runner, dir, "add", "img.png")
	runGit(t, runner, dir, "commit", "-m", "seed img.png")
	runGit(t, runner, dir, "push", "origin", "main")

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeBytesHelper(t, dir, "img.png", []byte("\x00\x01\x02BIN\x00uat\xaa\xbb\n"))
	runGit(t, runner, dir, "add", "img.png")
	runGit(t, runner, dir, "commit", "-m", "UAT changes img.png")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	theirs = []byte("\x00\x01\x02BIN\x00feature\xff\xfe\n")
	writeBytesHelper(t, dir, "img.png", theirs)
	runGit(t, runner, dir, "add", "img.png")
	runGit(t, runner, dir, "commit", "-m", "PROJ-1: feature changes img.png")
	featureSHA = trimNewline(string(runGit(t, runner, dir, "rev-parse", "HEAD").Stdout))
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")
	return featureSHA, theirs
}

// writeBytesHelper writes raw bytes to dir/name, failing the test on error.
func writeBytesHelper(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

// capturingRunner records every CommandRequest and returns a single canned
// CommandResult for all of them. It lets unit tests assert the exact Args a
// Service method issues (e.g. the `-c commit.gpgsign=false` flag) without a
// real git binary.
type capturingRunner struct {
	calls  []exec.CommandRequest
	result exec.CommandResult
}

func (r *capturingRunner) Run(_ context.Context, req exec.CommandRequest) (exec.CommandResult, error) {
	r.calls = append(r.calls, req)
	return r.result, nil
}

// findCall returns the first recorded request whose Args contain sub as a
// contiguous subslice-free token, matching on the presence of the token.
func (r *capturingRunner) callWithArg(token string) (exec.CommandRequest, bool) {
	for _, c := range r.calls {
		for _, a := range c.Args {
			if a == token {
				return c, true
			}
		}
	}
	return exec.CommandRequest{}, false
}

// writeFileHelper writes content to dir/name, failing the test on error.
func writeFileHelper(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

// argsContain reports whether args contains token.
func argsContain(args []string, token string) bool {
	for _, a := range args {
		if a == token {
			return true
		}
	}
	return false
}
