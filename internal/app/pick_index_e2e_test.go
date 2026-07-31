package app

import (
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// pickIndexConfig mirrors flowConfig(): a single UAT destination, mapped to a
// sandbox, with the default branch format.
func pickIndexConfig() config.Config {
	return config.Config{
		Branches: map[string]string{"uat": "UAT"},
		Sandboxes: map[string]config.SandboxConfig{
			"UAT": {Alias: "UAT_SBX", TestLevel: "RunLocalTests"},
		},
		TicketPatterns: []string{"PROJ-[0-9]+"},
		BranchFormat:   config.DefaultBranchFormat,
	}
}

// setupConflictRepo2 seeds a local clone with a "UAT" target and a
// two-commit "feature/PROJ-1" branch whose FIRST commit conflicts with UAT
// (both diverge a.cls) and whose SECOND commit is clean (adds b.cls).
// Returns the local clone path.
func setupConflictRepo2(t *testing.T) string {
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
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "base")
	gitRun(t, seed, "push", "origin", "main")

	gitRun(t, seed, "checkout", "-b", "UAT", "main")
	writeFile(t, seed, "a.cls", "l1\nUAT\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "UAT diverges a.cls")
	gitRun(t, seed, "push", "origin", "UAT")

	gitRun(t, seed, "checkout", "-b", "feature/PROJ-1", "main")
	writeFile(t, seed, "a.cls", "l1\nFEATURE\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: feature changes a.cls (conflicts)")
	writeFile(t, seed, "b.cls", "b\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: feature adds b.cls (clean)")
	gitRun(t, seed, "push", "origin", "feature/PROJ-1")

	gitRun(t, root, "clone", remote, local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")

	return local
}

// setupConflictRepo3 seeds a local clone with a "UAT" target and a
// three-commit "feature/PROJ-1" branch whose FIRST commit is clean (adds
// c.cls), SECOND commit conflicts with UAT (both diverge a.cls), and THIRD
// commit is clean (adds d.cls). Returns the local clone path.
func setupConflictRepo3(t *testing.T) string {
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
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "base")
	gitRun(t, seed, "push", "origin", "main")

	gitRun(t, seed, "checkout", "-b", "UAT", "main")
	writeFile(t, seed, "a.cls", "l1\nUAT\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "UAT diverges a.cls")
	gitRun(t, seed, "push", "origin", "UAT")

	gitRun(t, seed, "checkout", "-b", "feature/PROJ-1", "main")
	writeFile(t, seed, "c.cls", "public class C {}\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: add C (clean)")

	writeFile(t, seed, "a.cls", "l1\nFEATURE\nl3\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: feature changes a.cls (conflicts)")

	writeFile(t, seed, "d.cls", "public class D {}\n")
	gitRun(t, seed, "add", ".")
	gitRun(t, seed, "commit", "-m", "PROJ-1: add D (clean)")
	gitRun(t, seed, "push", "origin", "feature/PROJ-1")

	gitRun(t, root, "clone", remote, local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")

	return local
}

// driveToCherryPicking runs the real discovery/selection/target/branch flow
// (mirroring TestHU_FullFlow_PrereqToPickVerification) up to StateCherryPicking
// on a real temp repo, with Runs persistence wired to local. Returns the
// model (holding the HU-013 runID generated at branch creation) and the
// writer.
func driveToCherryPicking(t *testing.T, local string) (Model, *runs.Writer) {
	t.Helper()
	writer := runs.NewWriter(local)
	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: pickIndexConfig(),
		Dir:    local,
		Runs:   writer,
	}
	m := New(deps)
	m = advance(t, m, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	// HU-018: prereq now lands on the main menu; Enter on the default
	// "Promocionar ticket" entry (cursor 0) enters the UNCHANGED full flow.
	if m.State() != StateMainMenu {
		t.Fatalf("after prereq: want StateMainMenu, got %v", m.State())
	}
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
	if m.runID == "" {
		t.Fatal("branch creation should have generated a runID")
	}

	return m, writer
}

// TestOnPickDone_PersistsPickIndex_RealCherryPickSequencer is task 3.5 (RED,
// load-bearing): drives a REAL git cherry-pick through the full app flow and
// PINS derivePickIndex's "+1" against actual sequencer state — the disambiguating
// test design.md calls for: 2-commit conflict-on-first -> PickIndex 1;
// 3-commit conflict-on-second -> PickIndex 2.
func TestOnPickDone_PersistsPickIndex_RealCherryPickSequencer(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}

	t.Run("2-commit selection conflicting on the first pick -> PickIndex 1", func(t *testing.T) {
		local := setupConflictRepo2(t)
		m, writer := driveToCherryPicking(t, local)

		m = advance(t, m, run(t, m.cherryPickCmd()))
		if m.State() != StateCherryPickConflict {
			t.Fatalf("expected a conflict, got %v (err=%v)", m.State(), m.Err())
		}

		rec, err := writer.Load(m.runID)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if rec.PickIndex != 1 {
			t.Errorf("PickIndex = %d, want 1 (2-commit, conflict on the first pick)", rec.PickIndex)
		}
		if rec.PickTotal != 2 {
			t.Errorf("PickTotal = %d, want 2", rec.PickTotal)
		}
		if rec.Phase != "git-conflict" {
			t.Errorf("Phase = %q, want git-conflict", rec.Phase)
		}
		if rec.CurrentCommit == "" {
			t.Error("CurrentCommit should be persisted as the conflicting SHA")
		}
	})

	t.Run("3-commit selection conflicting on the second pick -> PickIndex 2", func(t *testing.T) {
		local := setupConflictRepo3(t)
		m, writer := driveToCherryPicking(t, local)

		m = advance(t, m, run(t, m.cherryPickCmd()))
		if m.State() != StateCherryPickConflict {
			t.Fatalf("expected a conflict, got %v (err=%v)", m.State(), m.Err())
		}

		rec, err := writer.Load(m.runID)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if rec.PickIndex != 2 {
			t.Errorf("PickIndex = %d, want 2 (3-commit, conflict on the second pick)", rec.PickIndex)
		}
		if rec.PickTotal != 3 {
			t.Errorf("PickTotal = %d, want 3", rec.PickTotal)
		}
	})
}
