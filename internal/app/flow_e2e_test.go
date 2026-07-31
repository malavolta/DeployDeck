package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// gitRun runs a real git command in dir for test setup, failing the test on
// error. Test code may exec freely — only the internal/app PRODUCTION code is
// forbidden from doing so (enforced by boundary_test.go, which inspects
// non-test imports only).
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=ana", "GIT_AUTHOR_EMAIL=ana@example.com",
		"GIT_COMMITTER_NAME=ana", "GIT_COMMITTER_EMAIL=ana@example.com",
		"GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

// setupFlowRepo builds a bare remote pre-seeded with main, UAT and a
// feature/PROJ-1 branch carrying two ticket commits (touching distinct files
// so the promotion is faithful), then a local clone whose origin/UAT and
// origin/feature/PROJ-1 refs are real. Returns the local clone path.
func setupFlowRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	local := filepath.Join(root, "local")

	gitRun(t, root, "init", "--bare", "-b", "main", remote)
	gitRun(t, root, "init", "-b", "main", seed)
	gitRun(t, seed, "config", "user.name", "ana")
	gitRun(t, seed, "config", "user.email", "ana@example.com")

	writeFile(t, seed, "base.txt", "base\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "base")

	gitRun(t, seed, "checkout", "-b", "UAT")
	gitRun(t, seed, "checkout", "-b", "feature/PROJ-1")
	writeFile(t, seed, "a.cls", "content A\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1 add A")
	writeFile(t, seed, "b.cls", "content B\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1 add B")

	gitRun(t, seed, "remote", "add", "origin", remote)
	gitRun(t, seed, "push", "origin", "main", "UAT", "feature/PROJ-1")

	gitRun(t, root, "clone", remote, local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")

	return local
}

// setupSourceResolutionRepo checks out feature/PROJ-1 LOCALLY on top of
// setupFlowRepo's clone (which stays on "main") — git's checkout DWIM
// creates a local tracking branch, reproducing D1's repro: a branch present
// both locally and as its origin/ twin.
func setupSourceResolutionRepo(t *testing.T) string {
	t.Helper()
	local := setupFlowRepo(t)
	gitRun(t, local, "checkout", "feature/PROJ-1")
	return local
}

// setupSourceConfirmRepo extends setupFlowRepo with a SECOND, genuinely
// distinct candidate branch (hotfix/PROJ-1, off UAT), pushed from the clone
// itself, then returns to feature/PROJ-1 — the "still ambiguous after
// dedupe" scenario D2's confirm prompt targets.
func setupSourceConfirmRepo(t *testing.T) string {
	t.Helper()
	local := setupFlowRepo(t)
	gitRun(t, local, "checkout", "-b", "hotfix/PROJ-1", "origin/UAT")
	writeFile(t, local, "h.cls", "content H\n")
	gitRun(t, local, "add", ".")
	gitRun(t, local, "commit", "-m", "PROJ-1 hotfix")
	gitRun(t, local, "push", "origin", "hotfix/PROJ-1")
	gitRun(t, local, "checkout", "feature/PROJ-1")
	return local
}

func flowConfig() config.Config {
	return config.Config{
		Branches: map[string]string{"uat": "UAT"},
		Sandboxes: map[string]config.SandboxConfig{
			"UAT": {Alias: "UAT_SBX", TestLevel: "RunLocalTests"},
		},
		TicketPatterns: []string{"PROJ-[0-9]+"},
		BranchFormat:   config.DefaultBranchFormat,
	}
}

// run executes a single tea.Cmd and returns its message. It rejects batched
// commands so the linear happy-path driver stays explicit about each step.
func run(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	return cmd()
}

func advance(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}

// TestHU_FullFlow_PrereqToPickVerification is task 11.4: the full TUI flow
// works on a real temp repo. It composes the real git service (against the
// temp clone) with the app state machine and drives PrereqCheck ->
// TicketInput -> Discovery -> Selection -> TargetSelection -> PlanPreview ->
// BranchCreation -> CherryPicking -> PickVerification, asserting the promotion
// is verified faithful (OK) at the scope edge.
func TestHU_FullFlow_PrereqToPickVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupFlowRepo(t)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: flowConfig(),
		Dir:    local,
		// NewChecker nil: prereqs are fed directly (already covered by the
		// Phase-5 doctor E2E); this test exercises the git composition.
	}

	m := New(deps)
	if m.State() != StatePrereqCheck {
		t.Fatalf("start state = %v", m.State())
	}

	// PrereqCheck -> StateMainMenu (HU-018 landing).
	m = advance(t, m, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	if m.State() != StateMainMenu {
		t.Fatalf("after prereq: %v", m.State())
	}

	// Main menu -> full flow: Enter on the default "Promocionar ticket" entry
	// (cursor 0) enters the UNCHANGED promotion flow at StateTicketInput.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateTicketInput {
		t.Fatalf("Enter on Promocionar should reach StateTicketInput: %v", m.State())
	}

	// TicketInput -> Discovery.
	m = typeString(m, "PROJ-1")
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateCommitDiscovery {
		t.Fatalf("after ticket enter: %v", m.State())
	}

	// Run real discovery -> Selection.
	m = advance(t, m, run(t, m.discoverCmd()))
	if m.State() != StateCommitSelection {
		t.Fatalf("after discovery: %v (err=%v)", m.State(), m.Err())
	}
	if len(m.items) != 2 {
		t.Fatalf("discovery should find 2 commits in origin/UAT..origin/feature/PROJ-1, got %d", len(m.items))
	}

	// Confirm selection (both selected) -> TargetSelection.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateTargetSelection {
		t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
	}

	// Confirm target (UAT is the only destination) -> PlanPreview.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StatePlanPreview {
		t.Fatalf("after target confirm: %v (notice=%q)", m.State(), m.notice)
	}
	if m.branchName != "deploy/PROJ-1-to-UAT" {
		t.Fatalf("branch name = %q", m.branchName)
	}

	// Confirm plan -> BranchCreation, then run real fetch+branch creation.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateBranchCreation {
		t.Fatalf("after plan confirm: %v", m.State())
	}
	m = advance(t, m, run(t, m.branchCreateCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after branch creation: %v (err=%v)", m.State(), m.Err())
	}
	if m.Plan().PromotionBranch != "deploy/PROJ-1-to-UAT" {
		t.Fatalf("promotion branch not registered on the plan: %q", m.Plan().PromotionBranch)
	}

	// Run the real cherry-pick -> (clean) -> verify command issued.
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after cherry-pick, expected to stay in CherryPicking pending verify, got %v (err=%v)", m.State(), m.Err())
	}

	// Run the real post-pick verification -> PickVerification (scope edge).
	m = advance(t, m, run(t, m.verifyCmd()))
	if m.State() != StatePickVerification {
		t.Fatalf("after verify: %v (err=%v)", m.State(), m.Err())
	}
	if !m.Verification().OK() {
		t.Errorf("faithful promotion should verify OK; warnings: %v", m.Verification().Warnings())
	}
	if !m.DeltaAllowed() {
		t.Errorf("a clean, non-aborted completion should allow delta downstream")
	}

	// The rendered verification screen confirms the promotion target.
	if !strings.Contains(m.View(), "origin/UAT") {
		t.Errorf("verification view should name the promotion target")
	}
}

// TestHU_FullFlow_SourceResolutionDedupe_NoConfirmNeeded is task 4.2's
// dedupe scenario: a local+origin twin, promoted to the first pipeline
// environment (no suggestion applies), reaches StateCommitSelection
// DIRECTLY — D1 alone fixes the repro, no D2 confirm needed.
func TestHU_FullFlow_SourceResolutionDedupe_NoConfirmNeeded(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupSourceResolutionRepo(t)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: flowConfig(),
		Dir:    local,
		Runs:   runs.NewWriter(local),
	}
	m := New(deps)
	m = driveOnPrereqDone(t, m)
	if m.originalBranch != "feature/PROJ-1" {
		t.Fatalf("originalBranch = %q, want feature/PROJ-1 (captured at startup)", m.originalBranch)
	}

	m = advance(t, m, keyPress("enter")) // main menu -> Promocionar
	m = typeString(m, "PROJ-1")
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateCommitDiscovery {
		t.Fatalf("after ticket enter: %v", m.State())
	}
	m = advance(t, m, run(t, m.discoverCmd()))
	if m.State() != StateCommitSelection {
		t.Fatalf("dedupe path should reach StateCommitSelection directly (no confirm), got %v (err=%v)", m.State(), m.Err())
	}
	if len(m.items) == 0 {
		t.Fatalf("expected a non-empty discovered range, got 0 items")
	}
}

// TestHU_FullFlow_SourceConfirm_AcceptAndDecline is task 4.2's confirm-prompt
// scenarios: 2 genuinely distinct candidates, no suggestion, checked out on
// one -> StateSourceConfirm. "s" accepts and reaches StateCommitSelection
// non-empty; "n" declines and degrades to zero items.
func TestHU_FullFlow_SourceConfirm_AcceptAndDecline(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}

	newModel := func(t *testing.T) Model {
		t.Helper()
		local := setupSourceConfirmRepo(t)
		deps := Deps{
			Git:    git.New(execpkg.NewOSRunner()),
			Config: flowConfig(),
			Dir:    local,
			Runs:   runs.NewWriter(local),
		}
		m := New(deps)
		m = driveOnPrereqDone(t, m)
		if m.originalBranch != "feature/PROJ-1" {
			t.Fatalf("originalBranch = %q, want feature/PROJ-1 (captured at startup)", m.originalBranch)
		}
		m = advance(t, m, keyPress("enter")) // main menu -> Promocionar
		m = typeString(m, "PROJ-1")
		m = advance(t, m, keyPress("enter"))
		if m.State() != StateCommitDiscovery {
			t.Fatalf("after ticket enter: %v", m.State())
		}
		m = advance(t, m, run(t, m.discoverCmd()))
		if m.State() != StateSourceConfirm {
			t.Fatalf("genuinely ambiguous + current-branch match should reach StateSourceConfirm, got %v (err=%v)", m.State(), m.Err())
		}
		if !strings.Contains(m.View(), "feature/PROJ-1") {
			t.Fatalf("confirm prompt should name the pending candidate feature/PROJ-1:\n%s", m.View())
		}
		return m
	}

	t.Run("accept (s) runs the ranged discover", func(t *testing.T) {
		m := newModel(t)
		m = advance(t, m, keyPress("s"))
		if m.State() != StateCommitDiscovery {
			t.Fatalf("s should re-enter StateCommitDiscovery pending confirmSourceCmd, got %v", m.State())
		}
		m = advance(t, m, run(t, m.confirmSourceCmd()))
		if m.State() != StateCommitSelection {
			t.Fatalf("accepted confirm should reach StateCommitSelection, got %v (err=%v)", m.State(), m.Err())
		}
		if len(m.items) == 0 {
			t.Fatalf("expected a non-empty discovered range after accepting, got 0 items")
		}
	})

	t.Run("decline (n) degrades with zero items", func(t *testing.T) {
		m := newModel(t)
		m = advance(t, m, keyPress("n"))
		if m.State() != StateCommitSelection {
			t.Fatalf("n should degrade straight to StateCommitSelection, got %v", m.State())
		}
		if len(m.items) != 0 {
			t.Fatalf("declined confirm must not select any range, got %d items", len(m.items))
		}
	})
}
