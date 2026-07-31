package app

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// setupInlineDeleteRepo builds a bare "origin" remote seeded with main, plus
// a local clone with a real deploy/* branch committed and pushed — so the
// restore-checkout + local/remote delete round trip (task 3.13) exercises
// real git, not a FakeRunner double. Returns the local clone dir, the bare
// remote dir, and the branch name (already checked out locally, mirroring
// where a live flow would leave HEAD right before a terminal quit).
func setupInlineDeleteRepo(t *testing.T) (local, remote, branch string) {
	t.Helper()
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	localDir := filepath.Join(root, "local")
	branch = "deploy/PROJ-1-to-UAT"

	gitRun(t, root, "init", "--bare", "-b", "main", remote)
	gitRun(t, root, "init", "-b", "main", seed)
	gitRun(t, seed, "config", "user.name", "ana")
	gitRun(t, seed, "config", "user.email", "ana@example.com")
	writeFile(t, seed, "base.txt", "base\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "base")
	gitRun(t, seed, "remote", "add", "origin", remote)
	gitRun(t, seed, "push", "origin", "main")

	gitRun(t, root, "clone", remote, localDir)
	gitRun(t, localDir, "config", "user.name", "ana")
	gitRun(t, localDir, "config", "user.email", "ana@example.com")

	gitRun(t, localDir, "checkout", "-b", branch)
	writeFile(t, localDir, "deploy.txt", "deploy\n")
	gitRun(t, localDir, "add", ".")
	gitRun(t, localDir, "commit", "-m", "deploy work")
	gitRun(t, localDir, "push", "-u", "origin", branch)

	return localDir, remote, branch
}

// remoteHeadsContain reports whether the bare remote at remoteDir still
// advertises branch among its heads, via `git ls-remote --heads`.
func remoteHeadsContain(t *testing.T, remoteDir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "ls-remote", "--heads", remoteDir, branch)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-remote --heads %s %s: %v", remoteDir, branch, err)
	}
	return strings.TrimSpace(string(out)) != ""
}

// localBranchExists reports whether branch still resolves locally in dir.
func localBranchExists(dir, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// TestHU017_InlineDelete_LocalAndRemoteRoundTrip is task 3.13 (RED,
// real-git+bare-remote): confirming the inline current-branch delete on a
// REAL repo checks out the original branch, then deletes BOTH the local
// branch and its pushed origin ref — proving the branch-cleanup spec's
// "Confirmed delete removes local and remote refs" scenario against real
// git, not a FakeRunner double.
func TestHU017_InlineDelete_LocalAndRemoteRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local, remote, branch := setupInlineDeleteRepo(t)

	if currentBranchOf(t, local) != branch {
		t.Fatalf("test setup bug: expected to be on %s", branch)
	}

	deps := Deps{Git: git.New(execpkg.NewOSRunner()), Config: testConfig(), Dir: local}
	m := New(deps)
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.pendingDeleteCurrent = true
	m.currentPushed = true

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}

	if head := currentBranchOf(t, local); head != "main" {
		t.Fatalf("HEAD after delete-confirm quit = %q, want restored to %q", head, "main")
	}
	if localBranchExists(local, branch) {
		t.Errorf("expected the local %s to be deleted", branch)
	}
	if remoteHeadsContain(t, remote, branch) {
		t.Errorf("expected origin/%s to be deleted", branch)
	}
}

// TestHU017_InlineDelete_LocalOnly_WhenNeverPushed is 3.13's companion on a
// real repo: an unpushed branch is deleted locally only — the remote is
// never touched (there's nothing there to delete).
func TestHU017_InlineDelete_LocalOnly_WhenNeverPushed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local, _, branch := setupInlineDeleteRepo(t)
	// Undo the push from setup so the branch is genuinely local-only.
	gitRun(t, local, "push", "origin", "--delete", branch)

	deps := Deps{Git: git.New(execpkg.NewOSRunner()), Config: testConfig(), Dir: local}
	m := New(deps)
	m.originalBranch = "main"
	m.plan = git.DeploymentPlan{PromotionBranch: branch}
	m.pendingDeleteCurrent = true
	m.currentPushed = false

	msg := run(t, m.quitCmd())
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
	if head := currentBranchOf(t, local); head != "main" {
		t.Fatalf("HEAD after delete-confirm quit = %q, want restored to %q", head, "main")
	}
	if localBranchExists(local, branch) {
		t.Errorf("expected the local %s to be deleted", branch)
	}
}
