package app

import (
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// --- 4.1/4.2: selectOrphans pure orphan-correlation rule --------------------

// TestSelectOrphans_ExcludesInProgressRun is task 4.1 (RED): a deploy/*
// branch tied to a live in-progress run (correlated via runs.List(), matched
// back to its branch name through the SAME git.RenderBranchName the rest of
// the app uses to derive a promotion branch from Ticket+Target) is excluded;
// every other branch is an orphan (branch-cleanup spec: "Orphan Deploy
// Branches Listed For Batch Cleanup" / "In-progress run's branch is
// excluded").
func TestSelectOrphans_ExcludesInProgressRun(t *testing.T) {
	format := config.DefaultBranchFormat
	branches := []git.DeployBranch{
		{Name: "deploy/PROJ-1-to-UAT"}, // tied to the live in-progress run
		{Name: "deploy/PROJ-2-to-UAT"}, // orphan
	}
	records := []runs.Record{
		{Ticket: "PROJ-1", Target: "UAT", Phase: "git-conflict"}, // live cherry-pick-phase run
		{Ticket: "PROJ-9", Target: "INT", Phase: "done"},         // terminal, irrelevant
	}

	orphans := selectOrphans(branches, records, format, true)
	if len(orphans) != 1 || orphans[0].Name != "deploy/PROJ-2-to-UAT" {
		t.Fatalf("expected only PROJ-2 as orphan, got %v", orphans)
	}
}

// TestSelectOrphans_NothingExcludedWhenNotInProgress triangulates 4.1: when
// the repo has NO live in-progress run, nothing is excluded — even a
// record whose Ticket/Target happens to render the same branch name (e.g. a
// terminal run whose branch was never cleaned up).
func TestSelectOrphans_NothingExcludedWhenNotInProgress(t *testing.T) {
	format := config.DefaultBranchFormat
	branches := []git.DeployBranch{
		{Name: "deploy/PROJ-1-to-UAT"},
		{Name: "deploy/PROJ-2-to-UAT"},
	}
	records := []runs.Record{
		{Ticket: "PROJ-1", Target: "UAT", Phase: "git-conflict"},
	}

	all := selectOrphans(branches, records, format, false)
	if len(all) != 2 {
		t.Fatalf("expected both branches when nothing is in progress, got %v", all)
	}
}

// TestSelectOrphans_TerminalPhaseRunNeverExcludes is a further triangulation:
// even while inProgress is true, a record whose Phase is NOT a cherry-pick
// phase (e.g. "done"/"validating") never excludes its branch — only the
// live cherry-pick/conflict-phase record does.
func TestSelectOrphans_TerminalPhaseRunNeverExcludes(t *testing.T) {
	format := config.DefaultBranchFormat
	branches := []git.DeployBranch{{Name: "deploy/PROJ-1-to-UAT"}}
	records := []runs.Record{{Ticket: "PROJ-1", Target: "UAT", Phase: "validating"}}

	orphans := selectOrphans(branches, records, format, true)
	if len(orphans) != 1 {
		t.Fatalf("a non-cherry-pick-phase record must never exclude its branch, got %v", orphans)
	}
}

// TestSelectOrphans_ExcludesNonTerminalValidatingRun is the H-2 remediation
// (RED): a run that finished cherry-picking and is async-validating on the org
// (JobID set, non-terminal Status) is a LIVE run — its deploy/* branch MUST be
// excluded from the orphan set even when repoState.InProgress==false and no
// cherry-pick phase is active (branch-cleanup spec: "orphan = not tied to a
// live in-progress run"). Before H-2, selectOrphans only excluded a live
// cherry-pick-phase branch, so an async-validating run's branch was mislabeled
// deletable.
func TestSelectOrphans_ExcludesNonTerminalValidatingRun(t *testing.T) {
	format := config.DefaultBranchFormat
	branches := []git.DeployBranch{
		{Name: "deploy/PROJ-1-to-UAT"}, // async-validating (non-terminal) -> live
		{Name: "deploy/PROJ-2-to-UAT"}, // terminal Succeeded -> orphan
	}
	for _, status := range []string{"InProgress", "Queued", "Pending", ""} {
		t.Run("non-terminal status "+status, func(t *testing.T) {
			records := []runs.Record{
				{Ticket: "PROJ-1", Target: "UAT", JobID: "JOB-1", Status: status, Phase: "validating"},
				{Ticket: "PROJ-2", Target: "UAT", JobID: "JOB-2", Status: "Succeeded", Phase: "done"},
			}
			// inProgress=false on purpose: the repo is NOT mid cherry-pick, yet
			// the async-validating run's branch must still be excluded.
			orphans := selectOrphans(branches, records, format, false)
			if len(orphans) != 1 || orphans[0].Name != "deploy/PROJ-2-to-UAT" {
				t.Fatalf("expected PROJ-1 (non-terminal jobId=%q) excluded and PROJ-2 (terminal) listed, got %v", status, orphans)
			}
		})
	}
}

// TestSelectOrphans_TerminalValidationRunStaysListed triangulates H-2: a run
// with a JobID but a TERMINAL status ({Succeeded, SucceededPartial, Failed,
// Canceled}, via salesforce.IsTerminal) is NOT live — its branch remains an
// orphan, so the existing terminal-run cases keep working.
func TestSelectOrphans_TerminalValidationRunStaysListed(t *testing.T) {
	format := config.DefaultBranchFormat
	branches := []git.DeployBranch{{Name: "deploy/PROJ-1-to-UAT"}}
	for _, status := range []string{"Succeeded", "SucceededPartial", "Failed", "Canceled"} {
		t.Run(status, func(t *testing.T) {
			records := []runs.Record{{Ticket: "PROJ-1", Target: "UAT", JobID: "JOB-1", Status: status, Phase: "done"}}
			orphans := selectOrphans(branches, records, format, false)
			if len(orphans) != 1 {
				t.Fatalf("a terminal (%s) validation run's branch must stay listable, got %v", status, orphans)
			}
		})
	}
}

// --- resolveMergeTarget / mergedLabel: merge-label target resolution -------

// TestResolveMergeTarget is a unit test for the pure correlation resolveMergeTarget
// uses to find a "likely merged" ancestor-check target for an orphan branch: it
// looks up the run whose Ticket/Target renders (via the SAME git.RenderBranchName)
// to the branch's name, and reports ok=false when no run correlates (branch-cleanup
// spec: "or 'unknown' when target can't be determined").
func TestResolveMergeTarget(t *testing.T) {
	format := config.DefaultBranchFormat
	records := []runs.Record{{Ticket: "PROJ-1", Target: "UAT"}}

	target, ok := resolveMergeTarget(records, format, "deploy/PROJ-1-to-UAT")
	if !ok || target != "UAT" {
		t.Fatalf("expected (UAT, true), got (%q, %v)", target, ok)
	}

	_, ok2 := resolveMergeTarget(records, format, "deploy/PROJ-9-to-UAT")
	if ok2 {
		t.Fatal("an uncorrelated branch name should not resolve a target")
	}
}

// TestMergedLabel is a unit test for the pure mergedLabel classifier: ok=false
// or a non-nil err both degrade to "unknown" (never a delete gate — spec:
// "Merged-Vs-Abandoned Is A Best-Effort Label Only"); otherwise the ancestor
// check's boolean result maps to "likely merged"/"abandoned".
func TestMergedLabel(t *testing.T) {
	tests := []struct {
		name   string
		ok     bool
		merged bool
		err    error
		want   string
	}{
		{"resolved + merged", true, true, nil, "likely merged"},
		{"resolved + not merged", true, false, nil, "abandoned"},
		{"unresolved target", false, false, nil, "unknown"},
		{"ancestor check errored", true, false, errStub, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergedLabel(tt.ok, tt.merged, tt.err); got != tt.want {
				t.Errorf("mergedLabel(%v, %v, %v) = %q, want %q", tt.ok, tt.merged, tt.err, got, tt.want)
			}
		})
	}
}

// forEachDeployBranchesArgs is the exact for-each-ref invocation
// ListDeployBranches issues (mirrors internal/git's own tests).
var forEachDeployBranchesArgs = []string{
	"for-each-ref",
	"--format=%(refname:short)|%(committerdate:iso-strict)",
	"refs/heads/deploy/*",
	"refs/remotes/origin/deploy/*",
}

// --- 4.3/4.4: `b` from StateTicketInput enters StateBranchCleanup + loads --

// TestKeyTicket_B_EntersBranchCleanupAndLoads is task 4.3 (RED): pressing `b`
// on StateTicketInput enters StateBranchCleanup, parks cleanupPhase on
// cleanupLoading, and fires the list-load command.
func TestKeyTicket_B_EntersBranchCleanupAndLoads(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", forEachDeployBranchesArgs, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig(), Runs: runs.NewWriter(t.TempDir())})
	m.state = StateTicketInput

	next, cmd := m.Update(keyPress("b"))
	nm := next.(Model)
	if nm.State() != StateBranchCleanup {
		t.Fatalf("b should enter StateBranchCleanup, got %v", nm.State())
	}
	if nm.cleanupPhase != cleanupLoading {
		t.Fatalf("b should park cleanupPhase on cleanupLoading while the list loads, got %v", nm.cleanupPhase)
	}
	if cmd == nil {
		t.Fatal("b should fire the list-load command")
	}
	if _, ok := cmd().(deployBranchesMsg); !ok {
		t.Fatalf("expected deployBranchesMsg, got %T", cmd())
	}
}

// --- 4.5/4.6: q/esc from the cleanup screen returns to StateTicketInput ----

// TestKeyBranchCleanup_QEsc_ReturnsToTicketInput is task 4.5 (RED): q/esc
// from StateBranchCleanup (browsing) return to StateTicketInput.
func TestKeyBranchCleanup_QEsc_ReturnsToTicketInput(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		t.Run(key, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			m.state = StateBranchCleanup
			m.cleanupPhase = cleanupBrowsing

			next, cmd := m.Update(keyPress(key))
			nm := next.(Model)
			if nm.State() != StateTicketInput {
				t.Fatalf("%q should return to StateTicketInput, got %v", key, nm.State())
			}
			if cmd != nil {
				t.Errorf("%q should not fire a command", key)
			}
		})
	}
}

// --- 4.7/4.8: listDeployBranchesCmd loads + correlates orphans -------------

// TestListDeployBranchesCmd_LoadsAndSetsBrowsing is task 4.7 (RED):
// listDeployBranchesCmd calls git.ListDeployBranches + runs.List(),
// correlates orphans, and landing the result via Update sets cleanupPhase to
// cleanupBrowsing with the branches populated.
func TestListDeployBranchesCmd_LoadsAndSetsBrowsing(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	out := "deploy/PROJ-1-to-UAT|2026-07-10T00:00:00+00:00\n" +
		"origin/deploy/PROJ-1-to-UAT|2026-07-10T00:00:00+00:00\n" +
		"deploy/PROJ-2-to-UAT|2026-07-05T00:00:00+00:00\n"
	fr.When("git", forEachDeployBranchesArgs, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(out)})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig(), Runs: runs.NewWriter(t.TempDir())})

	msg := run(t, m.listDeployBranchesCmd())
	dmsg, ok := msg.(deployBranchesMsg)
	if !ok {
		t.Fatalf("expected deployBranchesMsg, got %T", msg)
	}
	if dmsg.err != nil {
		t.Fatalf("listDeployBranchesCmd errored: %v", dmsg.err)
	}
	if len(dmsg.branches) != 2 {
		t.Fatalf("expected 2 orphan branches (no records, nothing live), got %d: %v", len(dmsg.branches), dmsg.branches)
	}

	next, _ := m.Update(dmsg)
	nm := next.(Model)
	if nm.cleanupPhase != cleanupBrowsing {
		t.Fatalf("landing deployBranchesMsg should settle cleanupPhase on cleanupBrowsing, got %v", nm.cleanupPhase)
	}
	if len(nm.cleanupBranches) != 2 {
		t.Fatalf("expected cleanupBranches populated, got %d", len(nm.cleanupBranches))
	}
}

// TestListDeployBranchesCmd_ExcludesLiveRun triangulates 4.7: a branch
// correlated to a live in-progress run (via runs.List() + RepoState) is
// excluded from the loaded list.
func TestListDeployBranchesCmd_ExcludesLiveRun(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	if err := writer.Save(runs.Record{RunID: "live", Ticket: "PROJ-1", Target: "UAT", Phase: "git-conflict", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seeding live run: %v", err)
	}

	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	out := "deploy/PROJ-1-to-UAT|2026-07-10T00:00:00+00:00\ndeploy/PROJ-2-to-UAT|2026-07-05T00:00:00+00:00\n"
	fr.When("git", forEachDeployBranchesArgs, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(out)})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig(), Runs: writer})
	m.repoState = git.RepoState{InProgress: true}

	msg := run(t, m.listDeployBranchesCmd())
	dmsg := msg.(deployBranchesMsg)
	if len(dmsg.branches) != 1 || dmsg.branches[0].Name != "deploy/PROJ-2-to-UAT" {
		t.Fatalf("expected PROJ-1's branch excluded (live run), got %v", dmsg.branches)
	}
}

// TestListDeployBranchesCmd_GitErrorSurfaces + TestOnDeployBranches cover the
// load-error and empty-list cases explicitly (task 4's error-handling note).

func TestListDeployBranchesCmd_GitErrorSurfaces(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", forEachDeployBranchesArgs, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("boom")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	msg := run(t, m.listDeployBranchesCmd())
	dmsg, ok := msg.(deployBranchesMsg)
	if !ok {
		t.Fatalf("expected deployBranchesMsg, got %T", msg)
	}
	if dmsg.err == nil {
		t.Fatal("a git for-each-ref failure should surface as an error")
	}

	next, _ := m.Update(dmsg)
	nm := next.(Model)
	if nm.cleanupNotice == "" {
		t.Error("a load error should surface a notice")
	}
	if nm.cleanupPhase != cleanupBrowsing {
		t.Fatalf("an errored load should still settle on cleanupBrowsing (empty list + notice), got %v", nm.cleanupPhase)
	}
}

func TestOnDeployBranches_EmptyListRendersEmptyState(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupLoading

	next, cmd := m.Update(deployBranchesMsg{})
	nm := next.(Model)
	if nm.cleanupPhase != cleanupBrowsing {
		t.Fatalf("landing an empty list should settle on cleanupBrowsing, got %v", nm.cleanupPhase)
	}
	if cmd != nil {
		t.Error("landing the list should not fire another command")
	}
	if !strings.Contains(nm.View(), "sin ramas") {
		t.Errorf("an empty branch list should render an explicit empty-state message:\n%s", nm.View())
	}
}

func TestViewBranchCleanup_LoadingState(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupLoading
	if v := m.View(); v == "" {
		t.Error("the loading state should still render a screen")
	}
}

// --- 4.9/4.10: cursor navigation --------------------------------------------

// TestKeyBranchCleanup_Nav_MovesCursor is task 4.9 (RED): ↑/↓ (and k/j) move
// cleanupCursor, clamped at both ends — mirrors keyRunHistory's nav.
func TestKeyBranchCleanup_Nav_MovesCursor(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: "deploy/A"}},
		{DeployBranch: git.DeployBranch{Name: "deploy/B"}},
	}

	down := advance(t, m, keyPress("down"))
	if down.cleanupCursor != 1 {
		t.Fatalf("down should move cursor to 1, got %d", down.cleanupCursor)
	}
	up := advance(t, down, keyPress("up"))
	if up.cleanupCursor != 0 {
		t.Fatalf("up should move cursor back to 0, got %d", up.cleanupCursor)
	}
	if advance(t, m, keyPress("up")).cleanupCursor != 0 {
		t.Error("up at the top should stay at 0")
	}
	if advance(t, down, keyPress("down")).cleanupCursor != 1 {
		t.Error("down at the bottom should stay at the last row")
	}
}

// --- 4.11/4.12: `d` fires the per-row unpushedCountCmd gate -----------------

// TestKeyBranchCleanup_D_UnpushedGatesStrongConfirm is task 4.11 (RED): `d`
// on a row fires unpushedCountCmd(row.Name); a count>0 gates the strong
// (typed BORRAR) confirmation — reusing onUnpushedCount UNCHANGED, exactly
// like keySucceeded's inline delete.
func TestKeyBranchCleanup_D_UnpushedGatesStrongConfirm(t *testing.T) {
	branch := "deploy/PROJ-2-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"rev-list", "origin/" + branch + ".." + branch, "--count"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("2\n")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{{DeployBranch: git.DeployBranch{Name: branch}}}

	next, cmd := m.Update(keyPress("d"))
	if next.(Model).State() != StateBranchCleanup {
		t.Fatal("d should stay on StateBranchCleanup while the count query is in flight")
	}
	if cmd == nil {
		t.Fatal("d should fire unpushedCountCmd for the selected row")
	}
	msg := run(t, cmd)
	umsg, ok := msg.(unpushedMsg)
	if !ok {
		t.Fatalf("expected unpushedMsg, got %T", msg)
	}
	if umsg.count != 2 {
		t.Fatalf("count = %d, want 2", umsg.count)
	}

	next2, _ := next.(Model).Update(umsg)
	if next2.(Model).cleanupPhase != cleanupStrongConfirm {
		t.Fatalf("count>0 should gate the strong confirmation, got %v", next2.(Model).cleanupPhase)
	}
}

// TestKeyBranchCleanup_D_PushedGatesNormalConfirm is task 4.11's companion: a
// count==0 gates only the normal ('y') confirmation.
func TestKeyBranchCleanup_D_PushedGatesNormalConfirm(t *testing.T) {
	branch := "deploy/PROJ-2-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
	fr.When("git", []string{"rev-list", "origin/" + branch + ".." + branch, "--count"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("0\n")})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{{DeployBranch: git.DeployBranch{Name: branch}}}

	// d captures the target; the count result must be landed on the POST-d
	// model (which holds cleanupDeleteTarget) — onUnpushedCount drops a count
	// whose branch does not match the captured target (review H-1).
	nd, cmd := m.Update(keyPress("d"))
	msg := run(t, cmd)
	umsg := msg.(unpushedMsg)

	next, _ := nd.(Model).Update(umsg)
	if next.(Model).cleanupPhase != cleanupConfirm {
		t.Fatalf("count==0 should gate the normal confirmation, got %v", next.(Model).cleanupPhase)
	}
}

// TestKeyBranchCleanup_D_NoopWhenListEmpty is a regression guard: `d` with no
// rows (or an out-of-range cursor) must never fire a command.
func TestKeyBranchCleanup_D_NoopWhenListEmpty(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing

	_, cmd := m.Update(keyPress("d"))
	if cmd != nil {
		t.Error("d with an empty list must not fire a command")
	}
}

// --- H-1: cursor-move TOCTOU — d captures the row, freezes the cursor -------

// TestKeyBranchCleanup_D_CapturesTarget_FreezesCursor_DeletesCaptured is the
// H-1 remediation (RED): pressing `d` on row0 (pushed, 0 unpushed) CAPTURES
// row0 as the delete target and enters cleanupCounting, where cursor-move keys
// are no-ops. Even after an attempted move to row1 (unpushed), the confirm
// binds to the CAPTURED row0 and deletes row0 — NEVER row1 (which the pre-fix
// cursor TOCTOU would have force-deleted, bypassing BORRAR).
func TestKeyBranchCleanup_D_CapturesTarget_FreezesCursor_DeletesCaptured(t *testing.T) {
	row0 := "deploy/PROJ-0-to-UAT" // pushed, 0 unpushed
	row1 := "deploy/PROJ-1-to-UAT" // unpushed
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + row0}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha")})
	fr.When("git", []string{"rev-list", "origin/" + row0 + ".." + row0, "--count"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("0\n")})
	fr.When("git", []string{"branch", "-D", "--", row0}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"push", "origin", "--delete", "--", row0}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: row0, Pushed: true}},
		{DeployBranch: git.DeployBranch{Name: row1, Pushed: false}},
	}
	m.cleanupCursor = 0

	// d captures row0 and enters the counting phase.
	next, cmd := m.Update(keyPress("d"))
	nm := next.(Model)
	if nm.cleanupPhase != cleanupCounting {
		t.Fatalf("d should enter cleanupCounting while the count is in flight, got %v", nm.cleanupPhase)
	}
	if nm.cleanupDeleteTarget != row0 {
		t.Fatalf("d should capture row0 as the delete target, got %q", nm.cleanupDeleteTarget)
	}
	if cmd == nil {
		t.Fatal("d should fire unpushedCountCmd for the captured row")
	}

	// Cursor-move keys must be inert while counting.
	moved := advance(t, nm, keyPress("down"))
	if moved.cleanupCursor != 0 {
		t.Fatalf("cursor-move keys must be no-ops while counting, got cursor %d", moved.cleanupCursor)
	}

	// The count for the CAPTURED row0 lands (0 unpushed -> normal confirm).
	umsg := run(t, cmd).(unpushedMsg)
	if umsg.branch != row0 {
		t.Fatalf("the count message must carry the captured branch, got %q", umsg.branch)
	}
	confirmed := advance(t, moved, umsg)
	if confirmed.cleanupPhase != cleanupConfirm {
		t.Fatalf("count==0 on the captured row should gate the normal confirm, got %v", confirmed.cleanupPhase)
	}

	// y deletes the CAPTURED row0, never row1.
	_, cmd2 := confirmed.Update(keyPress("y"))
	if cmd2 == nil {
		t.Fatal("y should fire the delete for the captured target")
	}
	if _, ok := run(t, cmd2).(deleteDoneMsg); !ok {
		t.Fatalf("expected deleteDoneMsg from the delete command")
	}
	if !calledWith(fr, "git", "branch", "-D", "--", row0) {
		t.Fatalf("must delete the CAPTURED row0; calls: %v", fr.Calls)
	}
	if calledWith(fr, "git", "branch", "-D", "--", row1) {
		t.Fatal("must NEVER delete row1 (the row the cursor tried to move to)")
	}
}

// TestConfirmDeleteOrphan_DeletesCapturedTarget_NotCursorRow is H-1's core
// safety assertion (RED): when the captured target is an UNPUSHED branch but
// the cursor now points at a different, PUSHED row, the strong (BORRAR)
// confirmation deletes the CAPTURED unpushed branch — proving the delete reads
// cleanupDeleteTarget/cleanupDeleteTargetPushed, never the live cursor row.
func TestConfirmDeleteOrphan_DeletesCapturedTarget_NotCursorRow(t *testing.T) {
	captured := "deploy/PROJ-9-to-UAT"  // unpushed, captured by d
	cursorRow := "deploy/PROJ-2-to-UAT" // pushed, where the cursor now sits
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"branch", "-D", "--", captured}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: cursorRow, Pushed: true}},
	}
	m.cleanupCursor = 0 // points at the PUSHED cursorRow
	m.cleanupDeleteTarget = captured
	m.cleanupDeleteTargetPushed = false // captured target was unpushed
	m.cleanupPhase = cleanupStrongConfirm
	m = typeString(m, "BORRAR")

	next, cmd := m.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatal("BORRAR should fire the delete for the captured target")
	}
	if _, ok := run(t, cmd).(deleteDoneMsg); !ok {
		t.Fatalf("expected deleteDoneMsg")
	}
	if !calledWith(fr, "git", "branch", "-D", "--", captured) {
		t.Fatalf("must delete the CAPTURED (unpushed) branch; calls: %v", fr.Calls)
	}
	if calledWith(fr, "git", "branch", "-D", "--", cursorRow) {
		t.Fatal("must NEVER delete the row the cursor happens to sit on")
	}
	// The captured target was unpushed, so no remote delete is attempted.
	if calledWith(fr, "git", "push", "origin", "--delete", "--", captured) {
		t.Fatal("an unpushed captured target must not attempt a remote delete")
	}
	_ = next
}

// TestOnUnpushedCount_DropsStaleCountForDifferentBranch is H-1's stale-drop
// assertion (RED): a count landing for a branch that is no longer the captured
// target (a prior d+esc, then a new d on another row) must be dropped — never
// gating the confirm strength of the current target.
func TestOnUnpushedCount_DropsStaleCountForDifferentBranch(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupCounting
	m.cleanupDeleteTarget = "deploy/PROJ-2-to-UAT" // the CURRENT captured target

	// A stale count for a DIFFERENT branch lands.
	next, cmd := m.Update(unpushedMsg{branch: "deploy/PROJ-0-to-UAT", count: 7})
	nm := next.(Model)
	if nm.cleanupPhase != cleanupCounting {
		t.Fatalf("a stale count for a different branch must be dropped (phase unchanged), got %v", nm.cleanupPhase)
	}
	if cmd != nil {
		t.Error("dropping a stale count must not fire a command")
	}
}

// TestOnUnpushedCount_DroppedWhenOffDeleteScreens is the M-2 remediation
// (RED): onUnpushedCount must only apply its result while the model is on a
// delete-capable screen (StateSucceeded, StateAborted, StateBranchCleanup). A
// count that lands after the user left those screens — e.g. a d+esc on the
// cleanup screen with an in-flight count, then a normal promotion — must be
// dropped, never setting cleanupPhase on an unrelated screen.
func TestOnUnpushedCount_DroppedWhenOffDeleteScreens(t *testing.T) {
	cases := []struct {
		name  string
		state State
	}{
		{"ticket input", StateTicketInput},
		{"validation polling", StateValidationPolling},
		{"commit selection", StateCommitSelection},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			m.state = tc.state
			m.cleanupPhase = cleanupIdle

			next, cmd := m.Update(unpushedMsg{branch: "deploy/PROJ-2-to-UAT", count: 5})
			nm := next.(Model)
			if nm.cleanupPhase != cleanupIdle {
				t.Fatalf("a stale count off the delete screens must be dropped (phase stays idle), got %v", nm.cleanupPhase)
			}
			if cmd != nil {
				t.Error("dropping a stale count must not fire a command")
			}
		})
	}
}

// --- 4.13/4.14: deleteOrphanCmd + confirm wiring + reload -------------------

// branchCleanupStrongConfirmModel parks a Model on StateBranchCleanup mid a
// strong (typed BORRAR) per-row delete confirmation, with a FakeRunner canned
// for the local delete confirmDeleteOrphan will run on a correct confirmation.
func branchCleanupStrongConfirmModel(t *testing.T) (Model, *execpkg.FakeRunner, string) {
	t.Helper()
	branch := "deploy/PROJ-2-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupBranches = []cleanupRow{{DeployBranch: git.DeployBranch{Name: branch}}}
	m.cleanupCursor = 0
	// Review H-1: confirmDeleteOrphan deletes the CAPTURED target, populated by
	// `d`; set it directly here since this helper parks mid strong-confirm.
	m.cleanupDeleteTarget = branch
	m.cleanupDeleteTargetPushed = false
	m.cleanupPhase = cleanupStrongConfirm
	return m, fr, branch
}

// TestKeyBranchCleanup_StrongConfirm_BORRAR_TypedGate is task 4.11/4.12's
// strong-confirm companion: wrong text is refused, nothing deleted; the exact
// case-sensitive BORRAR deletes the SELECTED ROW and reloads; esc backs out.
func TestKeyBranchCleanup_StrongConfirm_BORRAR_TypedGate(t *testing.T) {
	t.Run("wrong text refused, nothing deleted", func(t *testing.T) {
		m, fr, _ := branchCleanupStrongConfirmModel(t)
		m = typeString(m, "BORRA")

		next, cmd := m.Update(keyPress("enter"))
		nm := next.(Model)
		if nm.cleanupPhase != cleanupStrongConfirm {
			t.Fatalf("wrong text must stay on the strong-confirm screen, got %v", nm.cleanupPhase)
		}
		if cmd != nil {
			t.Error("wrong text must NOT fire a command")
		}
		if nm.cleanupNotice == "" {
			t.Error("wrong text should surface a notice")
		}
		if len(fr.Calls) != 0 {
			t.Errorf("no git call should have been made yet, got %v", fr.Calls)
		}
	})

	t.Run("exact BORRAR deletes the row and reloads", func(t *testing.T) {
		m, fr, branch := branchCleanupStrongConfirmModel(t)
		m = typeString(m, "BORRAR")

		next, cmd := m.Update(keyPress("enter"))
		if cmd == nil {
			t.Fatal("BORRAR should fire the delete command")
		}
		msg := cmd()
		if _, ok := msg.(deleteDoneMsg); !ok {
			t.Fatalf("expected deleteDoneMsg, got %T", msg)
		}
		if !calledWith(fr, "git", "branch", "-D", "--", branch) {
			t.Fatalf("expected the selected row's local branch deleted; calls: %v", fr.Calls)
		}
		nm := next.(Model)
		if nm.cleanupPhase != cleanupBrowsing {
			t.Errorf("a successful confirm should return to cleanupBrowsing, got %v", nm.cleanupPhase)
		}
		if nm.deleteConfirm != "" {
			t.Errorf("a successful confirm should reset the typed buffer, got %q", nm.deleteConfirm)
		}

		// deleteDoneMsg then triggers a reload.
		next2, cmd2 := nm.Update(msg)
		nm2 := next2.(Model)
		if nm2.cleanupPhase != cleanupLoading {
			t.Fatalf("landing a successful delete should re-enter cleanupLoading to reload, got %v", nm2.cleanupPhase)
		}
		if cmd2 == nil {
			t.Fatal("a successful delete should fire the reload command")
		}
	})

	t.Run("esc backs out without deleting", func(t *testing.T) {
		m, fr, _ := branchCleanupStrongConfirmModel(t)
		m = typeString(m, "BOR")

		next, cmd := m.Update(keyPress("esc"))
		nm := next.(Model)
		if nm.cleanupPhase != cleanupBrowsing {
			t.Fatalf("esc should back out to cleanupBrowsing, got %v", nm.cleanupPhase)
		}
		if nm.deleteConfirm != "" {
			t.Errorf("esc should clear the typed buffer, got %q", nm.deleteConfirm)
		}
		if cmd != nil {
			t.Error("esc must not fire a command")
		}
		if len(fr.Calls) != 0 {
			t.Errorf("esc must NOT invoke any git call, got %v", fr.Calls)
		}
	})
}

// TestKeyBranchCleanup_NormalConfirm_NDeclinesLeavingListUntouched is task
// 4.13's negative companion: declining ('n'/'esc') the normal confirmation
// deletes nothing and leaves the loaded list untouched.
func TestKeyBranchCleanup_NormalConfirm_NDeclinesLeavingListUntouched(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		t.Run(key, func(t *testing.T) {
			fr := execpkg.NewFakeRunner() // no canned responses: any git call fails loudly
			m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
			m.state = StateBranchCleanup
			m.cleanupPhase = cleanupConfirm
			m.cleanupBranches = []cleanupRow{{DeployBranch: git.DeployBranch{Name: "deploy/PROJ-2-to-UAT"}}}
			m.cleanupCursor = 0

			next, cmd := m.Update(keyPress(key))
			nm := next.(Model)
			if nm.cleanupPhase != cleanupBrowsing {
				t.Fatalf("%q should decline back to cleanupBrowsing, got %v", key, nm.cleanupPhase)
			}
			if cmd != nil {
				t.Errorf("%q must not fire a command", key)
			}
			if len(nm.cleanupBranches) != 1 || nm.cleanupBranches[0].Name != "deploy/PROJ-2-to-UAT" {
				t.Errorf("%q must leave the list untouched, got %v", key, nm.cleanupBranches)
			}
			if len(fr.Calls) != 0 {
				t.Errorf("%q must not invoke any git call, got %v", key, fr.Calls)
			}
		})
	}
}

// TestDeleteOrphanCmd_DeletesLocalAndRemoteIfPushed_ThenReloads is task 4.13
// (RED): deleteOrphanCmd deletes both the local branch and its origin ref
// when the row was pushed.
func TestDeleteOrphanCmd_DeletesLocalAndRemoteIfPushed_ThenReloads(t *testing.T) {
	branch := "deploy/PROJ-2-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"push", "origin", "--delete", "--", branch}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	msg := run(t, m.deleteOrphanCmd(branch, true))
	dmsg, ok := msg.(deleteDoneMsg)
	if !ok {
		t.Fatalf("expected deleteDoneMsg, got %T", msg)
	}
	if dmsg.err != nil {
		t.Fatalf("deleteOrphanCmd errored: %v", dmsg.err)
	}
	if !calledWith(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("expected local delete; calls: %v", fr.Calls)
	}
	if !calledWith(fr, "git", "push", "origin", "--delete", "--", branch) {
		t.Fatalf("expected remote delete for a pushed row; calls: %v", fr.Calls)
	}
}

// TestDeleteOrphanCmd_LocalOnly_WhenUnpushed is task 4.13's companion: an
// unpushed row's delete never attempts a remote delete.
func TestDeleteOrphanCmd_LocalOnly_WhenUnpushed(t *testing.T) {
	branch := "deploy/PROJ-3-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	msg := run(t, m.deleteOrphanCmd(branch, false))
	if _, ok := msg.(deleteDoneMsg); !ok {
		t.Fatalf("expected deleteDoneMsg, got %T", msg)
	}
	if calledWith(fr, "git", "push", "origin", "--delete", "--", branch) {
		t.Fatalf("an unpushed row must never attempt a remote delete; calls: %v", fr.Calls)
	}
}

// TestOnDeleteDone_ErrorSurfacesNotice_NoReload is task 4.13/4.14's error
// companion: a failed delete surfaces a notice and does NOT fire a reload.
func TestOnDeleteDone_ErrorSurfacesNotice_NoReload(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	next, cmd := m.Update(deleteDoneMsg{err: errStub})
	nm := next.(Model)
	if nm.cleanupNotice == "" {
		t.Error("a failed delete should surface a notice")
	}
	if cmd != nil {
		t.Error("a failed delete should not fire a reload command")
	}
}

// --- 4.15/4.16: the merged-label NEVER bypasses the delete confirm ---------

// TestMergedLabel_NeverBypassesConfirm is task 4.15 (RED, MANDATORY safety
// gate): a "likely merged" (or "abandoned", or "unknown") row still requires
// the EXACT SAME unpushedCountCmd gate as any other row — the label is
// display-only and never shortcuts, weakens, or skips confirmation (spec:
// "Merged-Vs-Abandoned Is A Best-Effort Label Only" / "Label never bypasses
// confirmation").
func TestMergedLabel_NeverBypassesConfirm(t *testing.T) {
	for _, label := range []string{"likely merged", "abandoned", "unknown"} {
		t.Run(label, func(t *testing.T) {
			branch := "deploy/PROJ-2-to-UAT"
			fr := execpkg.NewFakeRunner()
			fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
			fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("sha1")})
			fr.When("git", []string{"rev-list", "origin/" + branch + ".." + branch, "--count"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("1\n")})

			m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
			m.state = StateBranchCleanup
			m.cleanupPhase = cleanupBrowsing
			m.cleanupBranches = []cleanupRow{{DeployBranch: git.DeployBranch{Name: branch}, MergedLabel: label}}

			_, cmd := m.Update(keyPress("d"))
			if cmd == nil {
				t.Fatalf("a %q-labeled row must still fire the unpushed-count gate, never skip it", label)
			}
			msg := run(t, cmd)
			if _, ok := msg.(unpushedMsg); !ok {
				t.Fatalf("expected unpushedMsg regardless of label, got %T", msg)
			}
			// The label plays NO role in the delete path itself: deleteOrphanCmd
			// is only ever reached through confirmDeleteOrphan, which reads
			// row.Name/row.Pushed — never row.MergedLabel.
		})
	}
}

// TestListDeployBranchesCmd_LabelsMergedAndNotMerged is task 4.16 (RED):
// listDeployBranchesCmd labels an orphan whose correlating run's target it
// IS an ancestor of as "likely merged", one it is NOT as "abandoned", and one
// with no correlating run at all as "unknown" — proving the label is wired
// for DISPLAY only (4.15 already proves the delete path is unconditional).
func TestListDeployBranchesCmd_LabelsMergedAndNotMerged(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	if err := writer.Save(runs.Record{RunID: "r1", Ticket: "PROJ-1", Target: "UAT", Phase: "done", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seeding PROJ-1 run: %v", err)
	}
	if err := writer.Save(runs.Record{RunID: "r2", Ticket: "PROJ-2", Target: "UAT", Phase: "done", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seeding PROJ-2 run: %v", err)
	}

	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	out := "deploy/PROJ-1-to-UAT|2026-07-10T00:00:00+00:00\n" +
		"deploy/PROJ-2-to-UAT|2026-07-05T00:00:00+00:00\n" +
		"deploy/PROJ-3-to-UAT|2026-07-01T00:00:00+00:00\n" // no correlating run -> unknown
	fr.When("git", forEachDeployBranchesArgs, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(out)})
	fr.When("git", []string{"merge-base", "--is-ancestor", "deploy/PROJ-1-to-UAT", "origin/UAT"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"merge-base", "--is-ancestor", "deploy/PROJ-2-to-UAT", "origin/UAT"}, execpkg.CommandResult{ExitCode: 1})

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig(), Runs: writer})
	msg := run(t, m.listDeployBranchesCmd())
	dmsg := msg.(deployBranchesMsg)
	if dmsg.err != nil {
		t.Fatalf("listDeployBranchesCmd errored: %v", dmsg.err)
	}

	labels := map[string]string{}
	for _, row := range dmsg.branches {
		labels[row.Name] = row.MergedLabel
	}
	if labels["deploy/PROJ-1-to-UAT"] != "likely merged" {
		t.Errorf("PROJ-1 label = %q, want %q", labels["deploy/PROJ-1-to-UAT"], "likely merged")
	}
	if labels["deploy/PROJ-2-to-UAT"] != "abandoned" {
		t.Errorf("PROJ-2 label = %q, want %q", labels["deploy/PROJ-2-to-UAT"], "abandoned")
	}
	if labels["deploy/PROJ-3-to-UAT"] != "unknown" {
		t.Errorf("PROJ-3 (no correlating run) label = %q, want %q", labels["deploy/PROJ-3-to-UAT"], "unknown")
	}
	if calledWith(fr, "git", "merge-base", "--is-ancestor", "deploy/PROJ-3-to-UAT", "origin/UAT") {
		t.Error("an uncorrelated branch must never fire an ancestor check at all")
	}
}

// --- 4.17/4.18: `p` behind a confirm invokes the UNCHANGED retention Prune -

// TestKeyBranchCleanup_P_FiresPruneRunsCmd_ThenNotice is task 4.17 (RED): `p`
// enters cleanupPruneConfirm (no call yet); confirming ('y') invokes
// deps.Runs.Prune(cfg.Runs.KeepLast, cfg.Runs.KeepDays, m.now()) UNCHANGED,
// and landing the result reports the pruned count via a notice.
func TestKeyBranchCleanup_P_FiresPruneRunsCmd_ThenNotice(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	old := runs.Record{RunID: "old", Ticket: "PROJ-0", Target: "UAT", CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := writer.Create(old, nil); err != nil {
		t.Fatalf("seeding old run: %v", err)
	}
	recent := runs.Record{RunID: "recent", Ticket: "PROJ-1", Target: "UAT", CreatedAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)}
	if _, err := writer.Create(recent, nil); err != nil {
		t.Fatalf("seeding recent run: %v", err)
	}

	clk := &fakeClock{t: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)}
	cfg := testConfig()
	cfg.Runs = config.RunsConfig{KeepLast: 0, KeepDays: 1}
	m := New(Deps{Dir: "/repo", Config: cfg, Runs: writer, Now: clk.now})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing

	next, cmd := m.Update(keyPress("p"))
	nm := next.(Model)
	if nm.cleanupPhase != cleanupPruneConfirm {
		t.Fatalf("p should enter the prune confirmation, got %v", nm.cleanupPhase)
	}
	if cmd != nil {
		t.Error("p alone should not fire the prune command yet — it's gated behind a confirm")
	}

	next2, cmd2 := nm.Update(keyPress("y"))
	nm2 := next2.(Model)
	if nm2.cleanupPhase != cleanupBrowsing {
		t.Fatalf("confirming should return to cleanupBrowsing, got %v", nm2.cleanupPhase)
	}
	if cmd2 == nil {
		t.Fatal("confirming should fire pruneRunsCmd")
	}
	msg := run(t, cmd2)
	pmsg, ok := msg.(pruneDoneMsg)
	if !ok {
		t.Fatalf("expected pruneDoneMsg, got %T", msg)
	}
	if pmsg.err != nil {
		t.Fatalf("pruneRunsCmd errored: %v", pmsg.err)
	}
	if len(pmsg.removed) != 1 || pmsg.removed[0] != "old" {
		t.Fatalf("expected only the outside-window 'old' run pruned (recent kept), got %v", pmsg.removed)
	}

	next3, cmd3 := nm2.Update(pmsg)
	nm3 := next3.(Model)
	if nm3.cleanupNotice == "" {
		t.Error("landing pruneDoneMsg should report the pruned count via a notice")
	}
	if cmd3 != nil {
		t.Error("landing the prune result should not fire another command")
	}
}

// TestKeyBranchCleanup_PruneConfirm_NDeclines is 4.17's negative companion:
// declining ('n'/'esc') never invokes Prune.
func TestKeyBranchCleanup_PruneConfirm_NDeclines(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		t.Run(key, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig(), Runs: runs.NewWriter(t.TempDir())})
			m.state = StateBranchCleanup
			m.cleanupPhase = cleanupPruneConfirm

			next, cmd := m.Update(keyPress(key))
			if next.(Model).cleanupPhase != cleanupBrowsing {
				t.Fatalf("%q should decline back to cleanupBrowsing, got %v", key, next.(Model).cleanupPhase)
			}
			if cmd != nil {
				t.Errorf("%q must not fire pruneRunsCmd", key)
			}
		})
	}
}

// TestOnPruneDone_ErrorSurfacesNotice is 4.17/4.18's error companion.
func TestOnPruneDone_ErrorSurfacesNotice(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	next, cmd := m.Update(pruneDoneMsg{err: errStub})
	nm := next.(Model)
	if nm.cleanupNotice == "" {
		t.Error("a failed prune should surface a notice")
	}
	if cmd != nil {
		t.Error("a failed prune should not fire another command")
	}
}

// --- 4.19/4.20: viewBranchCleanup renders age, push status, merged label ---

// TestViewBranchCleanup_RendersAgeAndPushStatus is task 4.19 (RED): the
// screen renders each row's name, age, pushed/local-only status, and
// merged-label — mirroring spec.md's own scenario numbers (pushed 10 days
// old vs local-only 2 days old).
func TestViewBranchCleanup_RendersAgeAndPushStatus(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	clk := &fakeClock{t: now}
	m := New(Deps{Dir: "/repo", Config: testConfig(), Now: clk.now})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: "deploy/DD-2", LastCommit: now.AddDate(0, 0, -10), Pushed: true}, MergedLabel: "abandoned"},
		{DeployBranch: git.DeployBranch{Name: "deploy/DD-3", LastCommit: now.AddDate(0, 0, -2), Pushed: false}, MergedLabel: "likely merged"},
	}

	v := m.View()
	for _, want := range []string{
		"deploy/DD-2", "10d", "pushed", "abandoned",
		"deploy/DD-3", "2d", "local", "likely merged",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("branch cleanup view missing %q:\n%s", want, v)
		}
	}
	// deploy/DD-2's row precedes deploy/DD-3's (list order preserved).
	if strings.Index(v, "deploy/DD-2") > strings.Index(v, "deploy/DD-3") {
		t.Errorf("rows should render in list order:\n%s", v)
	}
}

// TestViewBranchCleanup_ShowsCursor is a companion to 4.19/4.20: the selected
// row (deploy/B, cursor index 1) renders with a leading '>' marker; the
// unselected row (deploy/A) does not.
func TestViewBranchCleanup_ShowsCursor(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: "deploy/A"}},
		{DeployBranch: git.DeployBranch{Name: "deploy/B"}},
	}
	m.cleanupCursor = 1

	var cursorLine, otherLine string
	for _, l := range strings.Split(m.View(), "\n") {
		switch {
		case strings.Contains(l, "deploy/B"):
			cursorLine = l
		case strings.Contains(l, "deploy/A"):
			otherLine = l
		}
	}
	if cursorLine == "" || !strings.HasPrefix(strings.TrimLeft(cursorLine, " "), ">") {
		t.Errorf("selected row (deploy/B) should render a leading '>' marker, got %q", cursorLine)
	}
	if otherLine == "" || strings.HasPrefix(strings.TrimLeft(otherLine, " "), ">") {
		t.Errorf("unselected row (deploy/A) must NOT render the cursor marker, got %q", otherLine)
	}
}

// TestViewBranchCleanup_ConfirmPrompts is a companion to 4.19/4.20: the
// active confirm/strongConfirm/pruneConfirm sub-state renders its prompt.
func TestViewBranchCleanup_ConfirmPrompts(t *testing.T) {
	base := func() Model {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StateBranchCleanup
		m.cleanupBranches = []cleanupRow{{DeployBranch: git.DeployBranch{Name: "deploy/DD-2"}}}
		// The confirm prompts render the CAPTURED target (review H-1), populated
		// by `d`; set it directly here since these tests park mid-confirm.
		m.cleanupDeleteTarget = "deploy/DD-2"
		return m
	}

	t.Run("normal confirm", func(t *testing.T) {
		m := base()
		m.cleanupPhase = cleanupConfirm
		if !strings.Contains(m.View(), "Borrar") || !strings.Contains(m.View(), "deploy/DD-2") {
			t.Errorf("normal confirm should prompt to delete the captured branch:\n%s", m.View())
		}
	})

	t.Run("strong confirm echoes the typed buffer", func(t *testing.T) {
		m := base()
		m.cleanupPhase = cleanupStrongConfirm
		m.deleteConfirm = "BOR"
		if !strings.Contains(m.View(), "BORRAR") || !strings.Contains(m.View(), "BOR") {
			t.Errorf("strong confirm should prompt BORRAR and echo the typed buffer:\n%s", m.View())
		}
	})

	t.Run("prune confirm shows keepLast/keepDays", func(t *testing.T) {
		m := base()
		cfg := testConfig()
		cfg.Runs = config.RunsConfig{KeepLast: 30, KeepDays: 90}
		m.deps.Config = cfg
		m.cleanupPhase = cleanupPruneConfirm
		v := m.View()
		if !strings.Contains(v, "30") || !strings.Contains(v, "90") {
			t.Errorf("prune confirm should show keepLast/keepDays:\n%s", v)
		}
	})
}
