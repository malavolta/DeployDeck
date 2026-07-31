package app

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// rePromoteConfig mirrors rePromoteEnvConfig() (internal/app/re_promote_test.go)
// with sandboxes for every pipeline environment, so target confirmation
// (ResolveSandbox) succeeds regardless of which destination the flow lands
// on.
func rePromoteConfig() config.Config {
	return config.Config{
		Branches: map[string]string{
			"integration": "INT",
			"uat":         "UAT",
			"production":  "main",
		},
		Sandboxes: map[string]config.SandboxConfig{
			"INT": {Alias: "INT_SBX", TestLevel: "NoTestRun"},
			"UAT": {Alias: "UAT_SBX", TestLevel: "RunLocalTests"},
		},
		TicketPatterns: []string{"PROJ-[0-9]+"},
		BranchFormat:   config.DefaultBranchFormat,
	}
}

// gitHead returns the current HEAD SHA in dir.
func gitHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// setupRePromoteRepo builds a bare remote pre-seeded with main + the
// NEXT-environment branch (UAT, no extra commits — the destination range
// this re-promotion remaps into) + the prior run's own environment branch
// (INT), which holds ONE real cherry-picked commit (a.cls) under a NEW SHA.
// After cloning into local, it commits the SAME content under a genuinely
// DIFFERENT SHA on an unpushed local feature branch — oldSHA, what a prior
// run.json would have recorded as Commits, mirroring the real cherry-pick
// shape (the original commit predates its own cherry-pick and its object
// only needs to be LOCALLY resolvable for PatchID) — plus one more unpushed
// commit (missingSHA) whose content was never promoted anywhere. Returns the
// local clone path, oldSHA, the real origin/INT equivalent newSHA, and
// missingSHA.
func setupRePromoteRepo(t *testing.T) (local, oldSHA, newSHA, missingSHA string) {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	local = filepath.Join(root, "local")

	gitRun(t, root, "init", "--bare", "-b", "main", remote)
	gitRun(t, root, "init", "-b", "main", seed)
	gitRun(t, seed, "config", "user.name", "ana")
	gitRun(t, seed, "config", "user.email", "ana@example.com")
	gitRun(t, seed, "remote", "add", "origin", remote)

	writeFile(t, seed, "base.txt", "base\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "base")
	gitRun(t, seed, "push", "origin", "main")

	gitRun(t, seed, "checkout", "-b", "UAT", "main")
	gitRun(t, seed, "push", "origin", "UAT")

	gitRun(t, seed, "checkout", "-b", "INT", "main")
	writeFile(t, seed, "a.cls", "public class A {}\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: add A (cherry-picked)")
	newSHA = gitHead(t, seed)
	gitRun(t, seed, "push", "origin", "INT")

	gitRun(t, seed, "checkout", "main")

	gitRun(t, root, "clone", remote, local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")

	// The ORIGINAL commit as it existed on the prior run's discovery source
	// (never pushed to origin — recorded in rec.Commits).
	gitRun(t, local, "checkout", "-b", "feature/PROJ-1", "main")
	writeFile(t, local, "a.cls", "public class A {}\n")
	gitRun(t, local, "add", ".")
	gitRun(t, local, "commit", "-m", "PROJ-1: add A")
	oldSHA = gitHead(t, local)

	// A prior-run commit whose content never made it into INT at all.
	gitRun(t, local, "checkout", "-b", "feature/PROJ-2", "main")
	writeFile(t, local, "b.cls", "public class B {}\n")
	gitRun(t, local, "add", ".")
	gitRun(t, local, "commit", "-m", "PROJ-2: never promoted")
	missingSHA = gitHead(t, local)

	gitRun(t, local, "checkout", "main")
	return local, oldSHA, newSHA, missingSHA
}

// setupRePromoteRepoTwoCommits mirrors setupRePromoteRepo but seeds TWO
// prior-run commits promoted to INT (a.cls, b.cls), each under its own
// distinct new SHA — used to prove a pre-loaded, multi-commit selection
// stays genuinely editable (re-promotion spec: "User edits the pre-loaded
// selection before continuing").
func setupRePromoteRepoTwoCommits(t *testing.T) (local string, oldSHAs, newSHAs []string) {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	local = filepath.Join(root, "local")

	gitRun(t, root, "init", "--bare", "-b", "main", remote)
	gitRun(t, root, "init", "-b", "main", seed)
	gitRun(t, seed, "config", "user.name", "ana")
	gitRun(t, seed, "config", "user.email", "ana@example.com")
	gitRun(t, seed, "remote", "add", "origin", remote)

	writeFile(t, seed, "base.txt", "base\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "base")
	gitRun(t, seed, "push", "origin", "main")

	gitRun(t, seed, "checkout", "-b", "UAT", "main")
	gitRun(t, seed, "push", "origin", "UAT")

	gitRun(t, seed, "checkout", "-b", "INT", "main")
	for _, f := range []struct{ name, content, msg string }{
		{"a.cls", "public class A {}\n", "PROJ-1: add A (cherry-picked)"},
		{"b.cls", "public class B {}\n", "PROJ-1: add B (cherry-picked)"},
	} {
		writeFile(t, seed, f.name, f.content)
		gitRun(t, seed, "add", ".")
		gitRun(t, seed, "commit", "-m", f.msg)
		newSHAs = append(newSHAs, gitHead(t, seed))
	}
	gitRun(t, seed, "push", "origin", "INT")
	gitRun(t, seed, "checkout", "main")

	gitRun(t, root, "clone", remote, local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")

	gitRun(t, local, "checkout", "-b", "feature/PROJ-1", "main")
	for _, f := range []struct{ name, content, msg string }{
		{"a.cls", "public class A {}\n", "PROJ-1: add A"},
		{"b.cls", "public class B {}\n", "PROJ-1: add B"},
	} {
		writeFile(t, local, f.name, f.content)
		gitRun(t, local, "add", ".")
		gitRun(t, local, "commit", "-m", f.msg)
		oldSHAs = append(oldSHAs, gitHead(t, local))
	}
	gitRun(t, local, "checkout", "main")
	return local, oldSHAs, newSHAs
}

// TestRePromote_E2E_FullACPath is task 6.1 (RED, then GREEN via 6.3's glue):
// the consolidated HU-016 e2e over a real temp git repo + a runs.Writer
// fixture (NO org): StateRunHistory -> r on an eligible Succeeded row ->
// interim StateCommitDiscovery while the real patch-id remap runs ->
// rePromoteSeededMsg/onRePromoteSeeded lands on StateCommitSelection with
// the differing-SHA equivalent pre-checked and the unmatched commit warned
// (never dropped) -> confirm selection -> target selection (the
// NextEnvironmentBranch default is pre-suggested and still overridable) ->
// plan preview -> branch creation (the new run persists SourceRunID) ->
// cherry-pick -> post-pick verification.
//
// Scope note: the re-promotion spec's own Purpose section states it covers
// ONLY the pre-seed behavior, not the downstream promotion states — this
// test's scope edge (PickVerification) mirrors the existing
// TestHU_FullFlow_PrereqToPickVerification precedent for the same reason: no
// delta/validate/push screen needs a Salesforce fixture to prove the pre-seed
// contract, and building one would be disproportionate to (and outside) this
// spec's actual acceptance criteria.
func TestRePromote_E2E_FullACPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local, oldSHA, newSHA, missingSHA := setupRePromoteRepo(t)
	writer := runs.NewWriter(t.TempDir())
	seededAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	priorRec := runs.Record{
		RunID: "prior-run", Ticket: "PROJ-1", Target: "INT", Status: "Succeeded",
		Commits: []string{oldSHA, missingSHA}, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	seedRunAt(t, writer, priorRec)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: rePromoteConfig(),
		Dir:    local,
		Runs:   writer,
	}
	m := New(deps)
	m.state = StateRunHistory
	m.runs = []runs.Record{priorRec}
	m.runsCursor = 0

	// StateRunHistory -> r -> interim StateCommitDiscovery, remap fired.
	next, cmd := m.Update(keyPress("r"))
	m = next.(Model)
	if m.State() != StateCommitDiscovery {
		t.Fatalf("r on the eligible prior run should enter the interim discovery state, got %v", m.State())
	}
	if cmd == nil {
		t.Fatal("r should fire the remap command")
	}

	// Run the REAL patch-id remap against the temp repo.
	seedMsg := run(t, cmd)
	m = advance(t, m, seedMsg)
	if m.State() != StateCommitSelection {
		t.Fatalf("the remap result should land on StateCommitSelection, got %v (err=%v)", m.State(), m.Err())
	}

	if len(m.items) != 1 {
		t.Fatalf("expected exactly 1 pre-loaded commit (the patch-id match), got %d: %+v", len(m.items), m.items)
	}
	got := m.items[0]
	if got.SHA != newSHA {
		t.Fatalf("matched commit SHA = %q, want the origin/INT equivalent %q (oldSHA=%q must never be returned — proves the source resolved to origin/INT, bypassing SuggestDefaultSource)", got.SHA, newSHA, oldSHA)
	}
	if !got.Selected {
		t.Error("the matched commit should be pre-checked")
	}
	if got.Disabled {
		t.Error("the matched commit should remain editable")
	}
	if len(m.rePromoteMissing) != 1 || m.rePromoteMissing[0] != missingSHA {
		t.Fatalf("rePromoteMissing = %v, want [%s] (warned, never dropped)", m.rePromoteMissing, missingSHA)
	}
	if !strings.Contains(m.View(), missingSHA) {
		t.Errorf("the missing commit should surface an explicit warning on the selection view:\n%s", m.View())
	}

	// Confirm selection -> TargetSelection: the next-environment default is
	// pre-suggested (commit-discovery spec).
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateTargetSelection {
		t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
	}
	if m.destinations[m.targetCursor].Branch != "UAT" {
		t.Fatalf("target cursor should default to the next-environment suggestion UAT, got %q", m.destinations[m.targetCursor].Branch)
	}
	// ...and still overridable (commit-discovery spec: "Suggested
	// next-environment default can be overridden").
	if overridden := advance(t, m, keyPress("up")); overridden.targetCursor == m.targetCursor {
		t.Error("the suggested target default should be overridable via navigation")
	}

	m = advance(t, m, keyPress("enter")) // confirm UAT target -> PlanPreview
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
	if m.runID == "" {
		t.Fatal("branch creation should have generated a runID")
	}

	// The new run durably links back to the prior run (re-promotion spec:
	// "Completed re-promotion records SourceRunID").
	gotRec, err := writer.Load(m.runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRec.SourceRunID != "prior-run" {
		t.Errorf("SourceRunID = %q, want prior-run", gotRec.SourceRunID)
	}

	// The pre-seeded run now composes verbatim with the unmodified promotion
	// flow.
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after cherry-pick, expected to stay in CherryPicking pending verify, got %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.verifyCmd()))
	if m.State() != StatePickVerification {
		t.Fatalf("after verify: %v (err=%v)", m.State(), m.Err())
	}
	if !m.Verification().OK() {
		t.Errorf("the reused, faithfully cherry-picked commit should verify OK; warnings: %v", m.Verification().Warnings())
	}
}

// TestStartRePromoteInto_NoNextEnvironment_DegradesToNormalDiscovery is the
// review remediation for Finding 1 (HIGH): a prior run with NO next
// environment (rec.Target is already the last pipeline stage) must degrade
// to the normal manual discovery flow, never firing rePromoteRemapCmd —
// which would otherwise build the ill-defined range
// "origin/..origin/<rec.Target>" (empty next-environment side), fail with a
// real git exit 128, and land the user on StateError. Run against a real
// temp repo: the returned command must yield a discoverDoneMsg (the normal
// discovery path), never a rePromoteSeededMsg with a non-nil err, and
// feeding that message through Update must never reach StateError.
func TestStartRePromoteInto_NoNextEnvironment_DegradesToNormalDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupCleanRepo(t)
	deps := Deps{Git: git.New(execpkg.NewOSRunner()), Config: rePromoteConfig(), Dir: local}
	m := New(deps)
	rec := runs.Record{
		RunID: "prior", Ticket: "PROJ-1", Target: "main", Status: "Succeeded",
		Commits: []string{"deadbeef"},
	}

	next, cmd := m.startRePromoteInto(rec)
	nm := next.(Model)

	if nm.sourceRunID != "" {
		t.Errorf("sourceRunID = %q, want empty (no next-env degrade has no well-defined provenance link)", nm.sourceRunID)
	}
	if nm.State() != StateCommitDiscovery {
		t.Fatalf("state = %v, want the interim StateCommitDiscovery", nm.State())
	}
	if cmd == nil {
		t.Fatal("expected a command even on degrade (the normal discovery command)")
	}

	msg := run(t, cmd)
	if seeded, ok := msg.(rePromoteSeededMsg); ok {
		t.Fatalf("no-next-environment degrade must never fire rePromoteRemapCmd, got rePromoteSeededMsg{Matched=%v, Unmatched=%v, err=%v}", seeded.Matched, seeded.Unmatched, seeded.err)
	}
	if _, ok := msg.(discoverDoneMsg); !ok {
		t.Fatalf("no-next-environment degrade fired %T, want discoverDoneMsg (the normal manual discovery)", msg)
	}

	final := advance(t, nm, msg)
	if final.State() == StateError {
		t.Fatalf("no-next-environment degrade must never reach StateError, err=%v", final.Err())
	}
}

// TestRePromote_E2E_Variants is task 6.2 (RED, then GREEN via 6.3's glue):
// the docs/HISTORIAS.md:1076 variants, consolidated at the e2e boundary.
func TestRePromote_E2E_Variants(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}

	t.Run("edited pre-loaded selection before continuing", func(t *testing.T) {
		local, oldSHAs, newSHAs := setupRePromoteRepoTwoCommits(t)
		writer := runs.NewWriter(t.TempDir())
		priorRec := runs.Record{
			RunID: "prior-run", Ticket: "PROJ-1", Target: "INT", Status: "Succeeded",
			Commits: oldSHAs,
		}
		seedRunAt(t, writer, priorRec)

		deps := Deps{Git: git.New(execpkg.NewOSRunner()), Config: rePromoteConfig(), Dir: local, Runs: writer}
		m := New(deps)
		m.state = StateRunHistory
		m.runs = []runs.Record{priorRec}
		m.runsCursor = 0

		next, cmd := m.Update(keyPress("r"))
		m = next.(Model)
		m = advance(t, m, run(t, cmd))
		if m.State() != StateCommitSelection {
			t.Fatalf("expected StateCommitSelection, got %v (err=%v)", m.State(), m.Err())
		}
		if len(m.items) != 2 {
			t.Fatalf("expected 2 pre-loaded commits, got %d", len(m.items))
		}

		// Toggle the FIRST item off before continuing.
		m = advance(t, m, keyPress(" "))
		if m.items[0].Selected {
			t.Fatal("toggling the first item should deselect it")
		}
		if !m.items[1].Selected {
			t.Fatal("the second item should remain selected")
		}

		m = advance(t, m, keyPress("enter")) // confirm the EDITED selection
		if m.State() != StateTargetSelection {
			t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
		}
		if len(m.plan.SelectedCommits) != 1 || m.plan.SelectedCommits[0].SHA != newSHAs[1] {
			t.Fatalf("plan.SelectedCommits = %+v, want only the still-selected commit %q — the edit must be honored through to the plan", m.plan.SelectedCommits, newSHAs[1])
		}
	})

	t.Run("failed prior run offers no pre-load", func(t *testing.T) {
		local, oldSHA, _, _ := setupRePromoteRepo(t)
		writer := runs.NewWriter(t.TempDir())
		priorRec := runs.Record{
			RunID: "prior-run", Ticket: "PROJ-1", Target: "INT", Status: "Failed",
			Commits: []string{oldSHA},
		}
		seedRunAt(t, writer, priorRec)

		deps := Deps{Git: git.New(execpkg.NewOSRunner()), Config: rePromoteConfig(), Dir: local, Runs: writer}
		m := New(deps)
		m.state = StateRunHistory
		m.runs = []runs.Record{priorRec}
		m.runsCursor = 0

		next, cmd := m.Update(keyPress("r"))
		nm := next.(Model)
		if nm.State() != StateRunHistory {
			t.Fatalf("r on a Failed prior run should be a no-op, got %v", nm.State())
		}
		if cmd != nil {
			t.Error("r on a Failed prior run must fire no command")
		}
	})

	t.Run("ticket without a prior run uses the manual flow", func(t *testing.T) {
		local := setupCleanRepo(t)
		writer := runs.NewWriter(t.TempDir()) // empty: no prior runs recorded

		deps := Deps{Git: git.New(execpkg.NewOSRunner()), Config: rePromoteConfig(), Dir: local, Runs: writer}
		m := New(deps)
		m.state = StateTicketInput

		m = typeString(m, "PROJ-1")
		m = advance(t, m, keyPress("enter"))
		if m.State() != StateCommitDiscovery {
			t.Fatalf("after ticket enter: %v", m.State())
		}
		if m.sourceRunID != "" || len(m.rePromoteMissing) != 0 {
			t.Errorf("the manual flow must not carry any re-promote seed, got sourceRunID=%q rePromoteMissing=%v", m.sourceRunID, m.rePromoteMissing)
		}

		m = advance(t, m, run(t, m.discoverCmd()))
		if m.State() != StateCommitSelection {
			t.Fatalf("after discovery: %v (err=%v)", m.State(), m.Err())
		}
		if len(m.items) != 0 {
			t.Errorf("a ticket with no matching commits should pre-load nothing, got %d items", len(m.items))
		}
	})
}
