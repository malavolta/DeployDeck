package app

// hu017_cleanup_e2e_test.go is Group 5 (tasks 5.1/5.2): the CONSOLIDATED
// HU-017 end-to-end coverage, driving the REAL Model.Update against a REAL
// temp git repo + bare local remote + .deoployeck/runs/ filesystem — never
// FakeRunner — mirroring docs/HISTORIAS.md:1108-1114's own Test E2E section
// verbatim:
//
//   - Harness: repo git temporal con remoto bare local + .deploydeck/runs/,
//     sin org.
//   - Seed: rama original del usuario; ramas deploy/* huerfanas con y sin
//     push; runs antiguos fuera de retencion.
//   - Asserta: al terminar/abortar se restaura la rama original; un PR
//     mergeado o abandono borra la rama temporal tras confirmar, y si la
//     rama se habia pusheado tambien se elimina su ref remota en el bare
//     local; las huerfanas se listan con antiguedad y estado de push; una
//     rama con commits sin push no se borra sin confirmacion fuerte; los
//     runs fuera de keepLast/keepDays se eliminan y los recientes se
//     conservan.
//   - Variantes: abort a mitad de secuencia.
//   - En CI: si.
//
// Groups 1-4 already implement and unit/integration-test every one of these
// mechanisms in isolation; this file proves they compose correctly end to
// end through the SAME production entry points a real user session hits
// (handleKey / Update), never re-deriving or duplicating their unit-level
// assertions.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// setupHU017Repo builds a bare "origin" remote seeded with main, UAT, and a
// feature/PROJ-1 branch carrying two ticket commits — the SAME shape
// setupFlowRepo builds (flow_e2e_test.go) — but ALSO returns the bare
// remote's own path: this consolidated e2e needs it to assert a pushed
// deploy branch's remote ref is REALLY gone (ls-remote against the bare
// repo itself), not just the local branch.
func setupHU017Repo(t *testing.T) (local, remote string) {
	t.Helper()
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	local = filepath.Join(root, "local")

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

	return local, remote
}

// commitAtDate writes name/content in dir and commits it with a FIXED
// committer/author date (unixSeconds, UTC) — mirrors internal/git's own
// commitAt test helper (service_cleanup_e2e_test.go) — so the seeded
// orphan branches' age assertions below never depend on real wall-clock
// timing between commits made moments apart.
func commitAtDate(t *testing.T, dir, name, content, message string, unixSeconds int64) {
	t.Helper()
	writeFile(t, dir, name, content)

	addCmd := exec.Command("git", "add", name)
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %s in %s: %v\n%s", name, dir, err, out)
	}

	fixedDate := fmt.Sprintf("@%d +0000", unixSeconds)
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = dir
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=ana", "GIT_AUTHOR_EMAIL=ana@example.com",
		"GIT_COMMITTER_NAME=ana", "GIT_COMMITTER_EMAIL=ana@example.com",
		"GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_DATE="+fixedDate, "GIT_COMMITTER_DATE="+fixedDate,
	)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit -m %q in %s: %v\n%s", message, dir, err, out)
	}
}

// TestHU017_E2E_FullCleanupFlow is task 5.1 (RED): the consolidated HU-017
// scenario, driven end to end against a real repo. It seeds the original
// branch (main), two orphan deploy/* branches (one pushed+old, one
// local-only+newer), and two runs (one outside retention, one recent);
// drives the real flow to a terminal success; confirms the inline
// current-branch delete (local + remote, since it was pushed) alongside the
// restore-on-quit; then opens the batch cleanup screen in a FRESH session to
// prove the orphans list with age/push-status, the unpushed orphan refuses a
// normal confirm but accepts the typed BORRAR, and retention prunes the old
// run while keeping the recent one.
func TestHU017_E2E_FullCleanupFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local, remote := setupHU017Repo(t)

	// Seed two orphan deploy/* branches BEFORE the app ever runs, with
	// pinned, differing commit dates (branch-cleanup spec: "Orphan Deploy
	// Branches Listed For Batch Cleanup" — age + push status).
	oldSeconds := time.Now().AddDate(0, 0, -10).Unix()
	newSeconds := time.Now().AddDate(0, 0, -2).Unix()

	gitRun(t, local, "checkout", "-b", "deploy/DD-2")
	commitAtDate(t, local, "dd2.txt", "dd2\n", "chore: dd2", oldSeconds)
	gitRun(t, local, "push", "-u", "origin", "deploy/DD-2")
	gitRun(t, local, "checkout", "main")

	gitRun(t, local, "checkout", "-b", "deploy/DD-3")
	commitAtDate(t, local, "dd3.txt", "dd3\n", "chore: dd3", newSeconds)
	// Deliberately never pushed: this is the unpushed orphan the strong
	// (typed BORRAR) confirmation gate below must protect.
	gitRun(t, local, "checkout", "main")

	// Seed retention: one run well outside keepLast/keepDays, one recent.
	writer := runs.NewWriter(local)
	if _, err := writer.Create(runs.Record{
		RunID: "old-run", Ticket: "OLD-1", Target: "UAT",
		CreatedAt: time.Now().AddDate(0, 0, -100),
	}, nil); err != nil {
		t.Fatalf("seeding old run: %v", err)
	}
	if _, err := writer.Create(runs.Record{
		RunID: "recent-run", Ticket: "REC-1", Target: "UAT",
		CreatedAt: time.Now(),
	}, nil); err != nil {
		t.Fatalf("seeding recent run: %v", err)
	}

	cfg := flowConfig()
	cfg.Runs = config.RunsConfig{KeepLast: 0, KeepDays: 1}

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: cfg,
		Dir:    local,
		Runs:   writer,
	}

	// --- Drive the real flow to a terminal success (delta/validation are
	// out of this slice's scope, exactly like Group 2/3's own e2e tests).
	m := New(deps)
	m = driveOnPrereqDone(t, m)
	if m.originalBranch != "main" {
		t.Fatalf("originalBranch = %q, want %q (captured at startup)", m.originalBranch, "main")
	}

	// HU-018: post-prereq landing is the main menu; Enter on the default
	// "Promocionar ticket" entry reaches the unchanged full flow.
	m = advance(t, m, keyPress("enter"))

	m = typeString(m, "PROJ-1")
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateCommitDiscovery {
		t.Fatalf("after ticket enter: %v", m.State())
	}
	m = advance(t, m, run(t, m.discoverCmd()))
	if m.State() != StateCommitSelection {
		t.Fatalf("after discovery: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, keyPress("enter")) // confirm selection (all selected)
	if m.State() != StateTargetSelection {
		t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
	}
	m = advance(t, m, keyPress("enter")) // confirm UAT target
	if m.State() != StatePlanPreview {
		t.Fatalf("after target confirm: %v (notice=%q)", m.State(), m.notice)
	}
	m = advance(t, m, keyPress("enter")) // confirm plan -> BranchCreation
	if m.State() != StateBranchCreation {
		t.Fatalf("after plan confirm: %v", m.State())
	}
	m = advance(t, m, run(t, m.branchCreateCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after branch creation: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after cherry-pick, expected to stay in CherryPicking pending verify, got %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.verifyCmd()))
	if m.State() != StatePickVerification {
		t.Fatalf("expected a clean, verified pick, got %v (err=%v)", m.State(), m.Err())
	}

	deployBranch := m.Plan().PromotionBranch
	if deployBranch != "deploy/PROJ-1-to-UAT" {
		t.Fatalf("unexpected promotion branch: %q", deployBranch)
	}
	if currentBranchOf(t, local) != deployBranch {
		t.Fatalf("expected to be checked out on %s before finishing", deployBranch)
	}

	// Push the deploy branch (out-of-slice HU-014 push itself is not driven
	// here — only its currentPushed effect on the delete gate matters, same
	// posture Group 3's own e2e uses) so the inline-delete round trip below
	// also deletes its remote ref (branch-cleanup spec: "Confirmed delete
	// removes local and remote refs").
	gitRun(t, local, "push", "-u", "origin", deployBranch)
	m.currentPushed = true

	// --- Finish -> restore original branch + inline current-branch delete
	// (confirmed): keySucceeded's `d` -> unpushedCountCmd -> a fully pushed
	// branch (0 unpushed) gates the NORMAL confirm -> `y` -> quitCmd.
	m.state = StateSucceeded
	mAfterD, cmdD := m.Update(keyPress("d"))
	m = mAfterD.(Model)
	if cmdD == nil {
		t.Fatal("d should fire unpushedCountCmd for the current run's branch")
	}
	uMsg := run(t, cmdD)
	mAfterU, _ := m.Update(uMsg)
	m = mAfterU.(Model)
	if m.cleanupPhase != cleanupConfirm {
		t.Fatalf("a fully pushed, fully in-sync branch should gate the normal confirm, got %v", m.cleanupPhase)
	}

	mAfterY, quitCmd := m.Update(keyPress("y"))
	m = mAfterY.(Model)
	if quitCmd == nil {
		t.Fatal("confirming the delete should fire quitCmd")
	}
	if _, ok := run(t, quitCmd).(tea.QuitMsg); !ok {
		t.Fatal("quitCmd should return tea.QuitMsg")
	}

	if head := currentBranchOf(t, local); head != "main" {
		t.Fatalf("HEAD after finish+delete = %q, want restored to %q", head, "main")
	}
	if localBranchExists(local, deployBranch) {
		t.Errorf("expected %s deleted locally", deployBranch)
	}
	if remoteHeadsContain(t, remote, deployBranch) {
		t.Errorf("expected origin/%s deleted from the bare remote", deployBranch)
	}

	// --- A NEW session (simulating relaunching DeployDeck) opens the batch
	// cleanup screen: `b` lists the two seeded orphans with age + push
	// status (branch-cleanup spec: "Orphan Deploy Branches Listed For Batch
	// Cleanup").
	m2 := New(deps)
	m2.state = StateTicketInput

	mAfterB, cmdB := m2.Update(keyPress("b"))
	m2 = mAfterB.(Model)
	if m2.State() != StateBranchCleanup || m2.cleanupPhase != cleanupLoading {
		t.Fatalf("b should enter StateBranchCleanup/cleanupLoading, got state=%v phase=%v", m2.State(), m2.cleanupPhase)
	}
	if cmdB == nil {
		t.Fatal("b should fire the list-load command")
	}
	dMsg := run(t, cmdB)
	mAfterList, _ := m2.Update(dMsg)
	m2 = mAfterList.(Model)
	if m2.cleanupPhase != cleanupBrowsing {
		t.Fatalf("landing the list should settle on cleanupBrowsing, got %v", m2.cleanupPhase)
	}
	if len(m2.cleanupBranches) != 2 {
		t.Fatalf("expected exactly the 2 seeded orphans (the deleted current branch must NOT reappear), got %d: %+v", len(m2.cleanupBranches), m2.cleanupBranches)
	}

	byName := map[string]cleanupRow{}
	for _, row := range m2.cleanupBranches {
		byName[row.Name] = row
	}
	dd2, ok := byName["deploy/DD-2"]
	if !ok || !dd2.Pushed {
		t.Fatalf("expected deploy/DD-2 listed and marked pushed, got %+v (found=%v)", dd2, ok)
	}
	dd3, ok := byName["deploy/DD-3"]
	if !ok || dd3.Pushed {
		t.Fatalf("expected deploy/DD-3 listed and marked local-only, got %+v (found=%v)", dd3, ok)
	}
	if !dd3.LastCommit.After(dd2.LastCommit) {
		t.Errorf("expected deploy/DD-3 (seeded newer) LastCommit %v to be after deploy/DD-2's %v", dd3.LastCommit, dd2.LastCommit)
	}

	view := m2.View()
	for _, want := range []string{"deploy/DD-2", "pushed", "deploy/DD-3", "local"} {
		if !strings.Contains(view, want) {
			t.Errorf("branch cleanup view missing %q:\n%s", want, view)
		}
	}

	// Select the unpushed orphan (deploy/DD-3).
	dd3Idx := -1
	for i, row := range m2.cleanupBranches {
		if row.Name == "deploy/DD-3" {
			dd3Idx = i
		}
	}
	if dd3Idx < 0 {
		t.Fatal("deploy/DD-3 not found in the loaded list")
	}
	m2.cleanupCursor = dd3Idx

	// --- Unpushed orphan: `d` must gate the STRONG (typed BORRAR)
	// confirmation, never the normal one (branch-cleanup spec: "Strong
	// Confirmation For Unpushed Work").
	mAfterD2, cmdD2 := m2.Update(keyPress("d"))
	m2 = mAfterD2.(Model)
	if cmdD2 == nil {
		t.Fatal("d should fire unpushedCountCmd for the selected row")
	}
	uMsg2 := run(t, cmdD2)
	mAfterU2, _ := m2.Update(uMsg2)
	m2 = mAfterU2.(Model)
	if m2.cleanupPhase != cleanupStrongConfirm {
		t.Fatalf("an unpushed orphan should gate the STRONG confirm, got %v", m2.cleanupPhase)
	}

	// An incomplete/wrong typed confirmation is refused: the branch must
	// stay untouched (spec: "Normal confirm refused for unpushed work" —
	// here exercised as "any non-exact confirmation").
	m2 = typeString(m2, "BORRA")
	mAfterWrong, cmdWrong := m2.Update(keyPress("enter"))
	m2 = mAfterWrong.(Model)
	if cmdWrong != nil {
		t.Error("an incomplete confirmation must never fire the delete")
	}
	if m2.cleanupPhase != cleanupStrongConfirm {
		t.Fatalf("wrong/incomplete text must stay on the strong-confirm screen, got %v", m2.cleanupPhase)
	}
	if !localBranchExists(local, "deploy/DD-3") {
		t.Fatal("deploy/DD-3 must still exist after a refused confirmation")
	}

	// Completing the exact literal BORRAR deletes it.
	m2 = typeString(m2, "R") // buffer is now the exact "BORRAR"
	mAfterBorrar, cmdBorrar := m2.Update(keyPress("enter"))
	m2 = mAfterBorrar.(Model)
	if cmdBorrar == nil {
		t.Fatal("the exact typed BORRAR should fire the delete")
	}
	ddMsg := run(t, cmdBorrar)
	if _, ok := ddMsg.(deleteDoneMsg); !ok {
		t.Fatalf("expected deleteDoneMsg, got %T", ddMsg)
	}
	if localBranchExists(local, "deploy/DD-3") {
		t.Error("expected deploy/DD-3 deleted after typing the exact BORRAR")
	}
	mAfterDelete, cmdReload := m2.Update(ddMsg) // a successful delete reloads the list
	m2 = mAfterDelete.(Model)
	if cmdReload == nil {
		t.Fatal("a successful delete should fire the reload command")
	}
	reloadMsg := run(t, cmdReload)
	mAfterReload, _ := m2.Update(reloadMsg)
	m2 = mAfterReload.(Model)

	// --- Retention prune: outside-window runs removed, recent kept
	// (branch-cleanup spec: "Run Retention Applied From The Cleanup
	// Surface").
	mAfterP, cmdP := m2.Update(keyPress("p"))
	m2 = mAfterP.(Model)
	if m2.cleanupPhase != cleanupPruneConfirm {
		t.Fatalf("p should enter the prune confirmation, got %v", m2.cleanupPhase)
	}
	if cmdP != nil {
		t.Error("p alone must not fire the prune yet — it is gated behind a confirm")
	}
	mAfterPruneY, cmdPruneY := m2.Update(keyPress("y"))
	m2 = mAfterPruneY.(Model)
	if cmdPruneY == nil {
		t.Fatal("confirming should fire pruneRunsCmd")
	}
	pMsg := run(t, cmdPruneY)
	pdMsg, ok := pMsg.(pruneDoneMsg)
	if !ok {
		t.Fatalf("expected pruneDoneMsg, got %T", pMsg)
	}
	if pdMsg.err != nil {
		t.Fatalf("pruneRunsCmd errored: %v", pdMsg.err)
	}
	if !containsString(pdMsg.removed, "old-run") {
		t.Errorf("expected old-run pruned (outside keepDays=1), got removed=%v", pdMsg.removed)
	}
	if containsString(pdMsg.removed, "recent-run") {
		t.Errorf("expected recent-run KEPT (within keepDays=1), got removed=%v", pdMsg.removed)
	}
	if _, err := os.Stat(filepath.Join(local, ".deploydeck", "runs", "old-run")); !os.IsNotExist(err) {
		t.Errorf("expected old-run's directory removed from disk, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(local, ".deploydeck", "runs", "recent-run")); err != nil {
		t.Errorf("expected recent-run's directory to remain on disk: %v", err)
	}
}

// TestHU017_E2E_AbortMidSequence_NoRestore is task 5.2 (RED, MANDATORY
// variant): "abort a mitad de secuencia" (docs/HISTORIAS.md:1113) — quitting
// from a REAL unresolved cherry-pick conflict must NEVER restore the
// original branch (the load-bearing HU-013-protection assertion), and a
// FRESH session started afterwards on the same repo must still offer to
// resume the conflicted run — proving CHERRY_PICK_HEAD truly survived intact
// and resume-detection remains possible, not just that the file exists.
func TestHU017_E2E_AbortMidSequence_NoRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupConflictRepo2(t)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: pickIndexConfig(),
		Dir:    local,
		Runs:   runs.NewWriter(local),
	}

	m := New(deps)
	m = driveOnPrereqDone(t, m)
	if m.originalBranch != "main" {
		t.Fatalf("originalBranch = %q, want %q", m.originalBranch, "main")
	}

	// HU-018: main-menu landing -> Promocionar (full flow).
	m = advance(t, m, keyPress("enter"))

	m = typeString(m, "PROJ-1")
	m = advance(t, m, keyPress("enter"))
	m = advance(t, m, run(t, m.discoverCmd()))
	if m.State() != StateCommitSelection {
		t.Fatalf("after discovery: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, keyPress("enter")) // confirm selection
	if m.State() != StateTargetSelection {
		t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
	}
	m = advance(t, m, keyPress("enter")) // confirm UAT target
	m = advance(t, m, keyPress("enter")) // confirm plan -> BranchCreation
	if m.State() != StateBranchCreation {
		t.Fatalf("after plan confirm: %v", m.State())
	}
	m = advance(t, m, run(t, m.branchCreateCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after branch creation: %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPickConflict {
		t.Fatalf("expected a conflict, got %v (err=%v)", m.State(), m.Err())
	}
	if !m.repoState.InProgress {
		t.Fatal("expected RepoState.InProgress on a real unresolved conflict")
	}
	deployBranch := currentBranchOf(t, local)
	if deployBranch == "main" {
		t.Fatal("test setup bug: expected to be checked out on the deploy branch mid-conflict")
	}
	cherryPickHead := filepath.Join(local, ".git", "CHERRY_PICK_HEAD")
	if _, err := os.Stat(cherryPickHead); err != nil {
		t.Fatalf("test setup bug: expected a real CHERRY_PICK_HEAD, stat: %v", err)
	}

	// Abort mid-sequence: quitting mid-conflict must NEVER restore
	// (branch-cleanup spec: "Unresolved conflict blocks restore").
	_, quitCmd := m.Update(keyPress("q"))
	if quitCmd == nil {
		t.Fatal("q from StateCherryPickConflict should still return a quit command")
	}
	if _, ok := run(t, quitCmd).(tea.QuitMsg); !ok {
		t.Fatal("quitCmd should still return tea.QuitMsg")
	}
	if head := currentBranchOf(t, local); head != deployBranch {
		t.Fatalf("HEAD after mid-conflict quit = %q, want unchanged %q (no restore)", head, deployBranch)
	}
	if _, err := os.Stat(cherryPickHead); err != nil {
		t.Fatalf("CHERRY_PICK_HEAD must remain intact for HU-013 resume-detection: %v", err)
	}

	// A NEW session on the same repo (simulating relaunching DeployDeck
	// after the aborted-without-restore quit) must still offer to resume
	// the live conflict — proving the no-restore guard did not silently
	// break resume-detection.
	m2 := New(deps)
	m2 = driveOnPrereqDone(t, m2)
	if m2.State() != StateRunHistory {
		t.Fatalf("a fresh session should offer resume on the live conflict, got %v (notice=%q)", m2.State(), m2.notice)
	}
	if m2.runsCursor < 0 || m2.runsCursor >= len(m2.runs) {
		t.Fatalf("resume offer should pre-select a valid run, cursor=%d len=%d", m2.runsCursor, len(m2.runs))
	}
	if !isResumable(m2.runs[m2.runsCursor], m2.repoState) {
		t.Fatalf("the pre-selected run must be resumable, got %+v", m2.runs[m2.runsCursor])
	}
}
