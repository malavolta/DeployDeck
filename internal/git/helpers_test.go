package git_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deploydeck/internal/exec"
)

// nonInteractiveGitEnv mirrors the env internal/git.Service injects into
// every git request (GIT_EDITOR/GIT_TERMINAL_PROMPT/GIT_PAGER). Test
// helpers use it directly since they run git through a bare exec.Runner,
// not through the Service under test.
var nonInteractiveGitEnv = []string{
	"GIT_EDITOR=true",
	"GIT_TERMINAL_PROMPT=0",
	"GIT_PAGER=cat",
	// Tool-created commits never need a signature and must never hang
	// waiting on a no-tty gpg prompt (see design.md's gpgsign decision).
	"GIT_CONFIG_COUNT=1",
	"GIT_CONFIG_KEY_0=commit.gpgsign",
	"GIT_CONFIG_VALUE_0=false",
}

func runGit(t *testing.T, runner exec.Runner, dir string, args ...string) exec.CommandResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "git",
		Args: args,
		Dir:  dir,
		Env:  nonInteractiveGitEnv,
	})
	if err != nil {
		t.Fatalf("git %v failed to run: %v (stderr: %s)", args, err, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("git %v exited %d: %s", args, result.ExitCode, result.Stderr)
	}
	return result
}

// newTempRepo initializes a real temp Git repository with a bare "origin"
// remote wired and pushed, so origin/<branch> refs exist for tests that
// need them (branch listing, discovery, target-selection). It skips under
// -short since it shells out to a real git binary.
//
// This is the shared reference harness reused by later phases (HU-002
// Phase 6, HU-004 Phase 8); Phase 9's newTempRepoWithRemote extends it with
// a remote that advances independently after clone.
func newTempRepo(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	dir := t.TempDir()
	remoteDir := t.TempDir()

	runGit(t, runner, dir, "init", "-b", "main")
	runGit(t, runner, dir, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")

	runGit(t, runner, remoteDir, "init", "--bare")
	runGit(t, runner, dir, "remote", "add", "origin", remoteDir)

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# deploydeck temp repo\n"), 0o644); err != nil {
		t.Fatalf("failed to seed README.md: %v", err)
	}
	runGit(t, runner, dir, "add", "README.md")
	runGit(t, runner, dir, "commit", "-m", "chore: initial commit")

	runGit(t, runner, dir, "push", "origin", "main")

	return dir
}

// newTempRepoWithRemote initializes a bare "origin" remote seeded with
// "main" and "UAT" (both pushed, one initial commit each), then returns a
// LOCAL CLONE of it — via `git clone`, unlike newTempRepo's init+push —
// plus the SEED working directory used to create the remote and the bare
// remote's own path. Tests advance remoteDir's branches independently
// AFTER localDir has already cloned by committing and pushing through
// seedDir, NEVER through localDir itself: this is exactly the "remote
// advances after the local clone, and only a fetch retrieves it" scenario
// HU-005's Test E2E section requires (docs/HISTORIAS.md HU-005 — "tras el
// clon local, el remoto avanza UAT a un nuevo HEAD que solo un fetch
// trae"). Skips on -short like newTempRepo, since it shells out to real
// git.
func newTempRepoWithRemote(t *testing.T) (localDir, remoteDir, seedDir string) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	remoteDir = t.TempDir()
	runGit(t, runner, remoteDir, "init", "--bare", "-b", "main")

	seedDir = t.TempDir()
	runGit(t, runner, seedDir, "init", "-b", "main")
	runGit(t, runner, seedDir, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, seedDir, "config", "user.email", "deploydeck-test@example.com")
	runGit(t, runner, seedDir, "remote", "add", "origin", remoteDir)

	readmePath := filepath.Join(seedDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# deploydeck temp repo\n"), 0o644); err != nil {
		t.Fatalf("failed to seed README.md: %v", err)
	}
	runGit(t, runner, seedDir, "add", "README.md")
	runGit(t, runner, seedDir, "commit", "-m", "chore: initial commit")
	runGit(t, runner, seedDir, "push", "origin", "main")

	runGit(t, runner, seedDir, "checkout", "-b", "UAT", "main")
	uatPath := filepath.Join(seedDir, "uat.txt")
	if err := os.WriteFile(uatPath, []byte("uat\n"), 0o644); err != nil {
		t.Fatalf("failed to seed uat.txt: %v", err)
	}
	runGit(t, runner, seedDir, "add", "uat.txt")
	runGit(t, runner, seedDir, "commit", "-m", "chore: seed UAT")
	runGit(t, runner, seedDir, "push", "origin", "UAT")
	runGit(t, runner, seedDir, "checkout", "main")

	localParent := t.TempDir()
	localDir = filepath.Join(localParent, "local")
	runGit(t, runner, localParent, "clone", remoteDir, localDir)
	runGit(t, runner, localDir, "config", "user.name", "DeployDeck Test")
	runGit(t, runner, localDir, "config", "user.email", "deploydeck-test@example.com")

	return localDir, remoteDir, seedDir
}

// callRecordingRunner wraps a real exec.Runner (typically NewOSRunner()),
// recording every request's Name+Args (in call order, as a single
// space-joined string) while still running it for real. Ordering
// assertions that must exercise actual git behavior — proving `git fetch`
// really runs before `git checkout -b` against a real repository, not a
// canned FakeRunner response — use this instead of FakeRunner.
type callRecordingRunner struct {
	inner exec.Runner
	calls []string
}

func (r *callRecordingRunner) Run(ctx context.Context, req exec.CommandRequest) (exec.CommandResult, error) {
	r.calls = append(r.calls, req.Name+" "+strings.Join(req.Args, " "))
	return r.inner.Run(ctx, req)
}
