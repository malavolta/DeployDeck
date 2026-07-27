package app

import (
	"path/filepath"
	"testing"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
)

// setupDoubleConflictRepo3 seeds a local clone with a "UAT" target and a
// three-commit "feature/PROJ-1" branch whose FIRST commit conflicts with UAT
// (both diverge a.cls), SECOND commit ALSO conflicts with UAT (both diverge
// x.cls), and THIRD commit is clean (adds e.cls). Cherry-picking all three
// onto UAT stops on pick 1, and — once pick 1 is resolved and continued —
// stops AGAIN on pick 2, so an in-app `c`-continue on a resumed run lands on
// the NEXT conflict. Returns the local clone path.
func setupDoubleConflictRepo3(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	local := filepath.Join(root, "local")

	gitRun(t, root, "init", "--bare", "-b", "main", remote)
	gitRun(t, root, "init", "-b", "main", seed)
	gitRun(t, seed, "config", "user.name", "ana")
	gitRun(t, seed, "config", "user.email", "ana@example.com")
	gitRun(t, seed, "remote", "add", "origin", remote)

	writeFile(t, seed, "a.cls", "l1\nBASE\nl3\n")
	writeFile(t, seed, "x.cls", "l1\nBASE\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "base")
	gitRun(t, seed, "push", "origin", "main")

	gitRun(t, seed, "checkout", "-b", "UAT", "main")
	writeFile(t, seed, "a.cls", "l1\nUAT\nl3\n")
	writeFile(t, seed, "x.cls", "l1\nUAT\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "UAT diverges a.cls and x.cls")
	gitRun(t, seed, "push", "origin", "UAT")

	gitRun(t, seed, "checkout", "-b", "feature/PROJ-1", "main")
	writeFile(t, seed, "a.cls", "l1\nFEATURE\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: feature changes a.cls (conflicts)")

	writeFile(t, seed, "x.cls", "l1\nFEATURE\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: feature changes x.cls (conflicts)")

	writeFile(t, seed, "e.cls", "public class E {}\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: add E (clean)")
	gitRun(t, seed, "push", "origin", "feature/PROJ-1")

	gitRun(t, root, "clone", remote, local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")

	return local
}

// TestResume_ContinueToNextConflict_PreservesPickProgress is HOLE B RED (a):
// after resuming a multi-commit conflict, an in-app `c`-continue that lands on
// the NEXT conflict must keep the correct "pick N of M" — both on the Model and
// in the persisted run.json. Before the fix, resumeInto never rehydrated
// m.plan.SelectedCommits, so onPickDone recomputed len==0 → pickTotal/pickIndex
// were permanently zeroed here.
func TestResume_ContinueToNextConflict_PreservesPickProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupDoubleConflictRepo3(t)

	seed, writer := driveToCherryPicking(t, local)
	seed = advance(t, seed, run(t, seed.cherryPickCmd()))
	if seed.State() != StateCherryPickConflict {
		t.Fatalf("seed: expected a conflict, got %v (err=%v)", seed.State(), seed.Err())
	}
	runID := seed.runID

	// Restart: a brand-new model resumes over the same repo + runs dir.
	fresh := New(Deps{Git: git.New(execpkg.NewOSRunner()), Config: pickIndexConfig(), Dir: local, Runs: writer})
	fresh = advance(t, fresh, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	fresh = advance(t, fresh, run(t, fresh.resumeDetectCmd()))
	if fresh.State() != StateRunHistory {
		t.Fatalf("startup detection should offer resume via StateRunHistory, got %v", fresh.State())
	}
	fresh = advance(t, fresh, keyPress("enter"))
	if fresh.State() != StateCherryPickConflict {
		t.Fatalf("accepting resume should route to StateCherryPickConflict, got %v", fresh.State())
	}
	// The resumed run must carry its full selected set again, so len()-driven
	// pick math survives the next step.
	if len(fresh.plan.SelectedCommits) != 3 {
		t.Fatalf("resume should rehydrate all 3 selected commits, got %d", len(fresh.plan.SelectedCommits))
	}
	if fresh.pickIndex != 1 || fresh.pickTotal != 3 {
		t.Fatalf("resumed conflict should be pick 1 of 3, got %d of %d", fresh.pickIndex, fresh.pickTotal)
	}

	// Resolve the FIRST conflict (a.cls) and stage it, then continue in-app.
	writeFile(t, local, "a.cls", "l1\nFEATURE\nl3\n")
	gitRun(t, local, "add", "a.cls")
	fresh.continueEnabled = true // the git-level gate is re-checked in ContinueCherryPick

	next, cmd := fresh.Update(keyPress("c"))
	fresh = next.(Model)
	if cmd == nil {
		t.Fatal("pressing c should fire the continue command")
	}
	fresh = advance(t, fresh, run(t, cmd))

	// The continue landed on the SECOND conflict: still pick N of M, now 2 of 3.
	if fresh.State() != StateCherryPickConflict {
		t.Fatalf("continuing should land on the next conflict, got %v (err=%v)", fresh.State(), fresh.Err())
	}
	if fresh.pickTotal != 3 {
		t.Errorf("Model pickTotal = %d, want 3 (must not be zeroed on a resumed continue)", fresh.pickTotal)
	}
	if fresh.pickIndex != 2 {
		t.Errorf("Model pickIndex = %d, want 2 (3-commit, now conflicting on the second pick)", fresh.pickIndex)
	}

	rec, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.PickTotal != 3 {
		t.Errorf("persisted PickTotal = %d, want 3 (the resumed continue must not zero it)", rec.PickTotal)
	}
	if rec.PickIndex != 2 {
		t.Errorf("persisted PickIndex = %d, want 2", rec.PickIndex)
	}
}

// TestResume_Completion_RunsPostPickVerification is HOLE B RED (b): a resumed
// run's completion must RUN post-pick verification against the selected set
// (HU-006's safety check), not silently skip it. The conflict is resolved with
// content that does NOT match the selected commit (a partial promotion), so a
// REAL verification flags a.cls — while a skipped/no-op verification (the bug:
// empty SelectedCommits short-circuits verifyCmd) would falsely report the
// promotion faithful and lose the safety net.
func TestResume_Completion_RunsPostPickVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupConflictRepo2(t)

	seed, writer := driveToCherryPicking(t, local)
	seed = advance(t, seed, run(t, seed.cherryPickCmd()))
	if seed.State() != StateCherryPickConflict {
		t.Fatalf("seed: expected a conflict, got %v (err=%v)", seed.State(), seed.Err())
	}

	fresh := New(Deps{Git: git.New(execpkg.NewOSRunner()), Config: pickIndexConfig(), Dir: local, Runs: writer})
	fresh = advance(t, fresh, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	fresh = advance(t, fresh, run(t, fresh.resumeDetectCmd()))
	fresh = advance(t, fresh, keyPress("enter"))
	if fresh.State() != StateCherryPickConflict {
		t.Fatalf("accepting resume should route to StateCherryPickConflict, got %v", fresh.State())
	}
	if len(fresh.plan.SelectedCommits) != 2 {
		t.Fatalf("resume should rehydrate both selected commits, got %d", len(fresh.plan.SelectedCommits))
	}

	// Resolve the conflict WRONG (not the selected commit's content) so a real
	// post-pick verification has something to flag, then continue to completion.
	writeFile(t, local, "a.cls", "l1\nWRONG\nl3\n")
	gitRun(t, local, "add", "a.cls")
	fresh.continueEnabled = true

	next, cmd := fresh.Update(keyPress("c"))
	fresh = next.(Model)
	if cmd == nil {
		t.Fatal("pressing c should fire the continue command")
	}
	// The continue applies the clean second pick and completes the sequence,
	// so onPickDone builds the post-pick verifyCmd.
	next, verifyCommand := fresh.Update(run(t, cmd))
	fresh = next.(Model)
	if fresh.State() != StateCherryPicking {
		t.Fatalf("a completed resumed pick should enter verification (StateCherryPicking), got %v", fresh.State())
	}
	if verifyCommand == nil {
		t.Fatal("completing the pick should fire the post-pick verify command")
	}

	msg := run(t, verifyCommand)
	vd, ok := msg.(verifyDoneMsg)
	if !ok {
		t.Fatalf("expected verifyDoneMsg, got %T", msg)
	}
	if vd.err != nil {
		t.Fatalf("verify errored: %v", vd.err)
	}
	// A real verification over the selected set catches the partial promotion.
	// The bug (skipped verify on empty SelectedCommits) would report OK here.
	if vd.verification.OK() {
		t.Fatal("resumed-run verification must NOT be skipped: the partial promotion should be flagged, not reported OK")
	}
	if !containsString(vd.verification.PartialFiles, "a.cls") {
		t.Errorf("post-pick verify should flag a.cls as a partial promotion, got %v", vd.verification.PartialFiles)
	}
}

// containsString reports whether s is in xs.
func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
