package git_test

import (
	"context"
	"errors"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestHU003_CommitSelection_E2E is the consolidated end-to-end scenario
// from docs/HISTORIAS.md HU-003's "Test E2E" section, run through the real
// HU-003 selection entry point (NewCommitSelectionItems +
// Service.DependencyWarnings + ValidateSelection + GenerateDeploymentPlan)
// against a real temp repo (no FakeRunner, no org): a feature branch with
// a merge commit, a commit already applied to the target by content
// equivalence, a commit mentioning several tickets, and a selection whose
// touched file has an earlier, unselected intermediate commit from another
// ticket.
func TestHU003_CommitSelection_E2E(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// --- Seed: UAT (target) branches off main and is pushed. ---
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	runGit(t, runner, dir, "push", "origin", "UAT")

	// --- Seed: feature/PROJ-20 (source) branches off UAT. ---
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-20", "UAT")

	// interleavedSHA: an EARLIER commit from ANOTHER ticket, touching a
	// file the ticket's own later commit also touches. Left unselected by
	// the user below, it must trigger the per-file dependency warning.
	interleavedSHA := writeAndCommit(t, runner, dir, "shared.txt", "v1\n", "PROJ-21: interleaved change to shared file")

	// sharedFileSHA: the ticket's own change to the SAME file, LATER in
	// the range. Selected by default.
	sharedFileSHA := writeAndCommit(t, runner, dir, "shared.txt", "v2\n", "PROJ-20: update shared file")

	// dualTicketSHA: mentions a second ticket besides the one searched.
	dualTicketSHA := writeAndCommit(t, runner, dir, "dual.txt", "dual\n", "PROJ-20 PROJ-22: dual ticket fix")

	// equivSHA: content later duplicated onto UAT under a different SHA
	// (content equivalence via git cherry) — the already-applied case.
	equivSHA := writeAndCommit(t, runner, dir, "equiv.txt", "equivalent-payload\n", "PROJ-20: equivalent content commit")

	// A merge commit within the source range, which must be flagged and
	// blocked from selection (cherry-pick -m unsupported in MVP).
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-20-side", "feature/PROJ-20")
	sideSHA := writeAndCommit(t, runner, dir, "side.txt", "side\n", "PROJ-20: side change")
	runGit(t, runner, dir, "checkout", "feature/PROJ-20")
	runGit(t, runner, dir, "merge", "--no-ff", "feature/PROJ-20-side", "-m", "PROJ-20: merge side branch")
	mergeResult := runGit(t, runner, dir, "rev-parse", "HEAD")
	mergeSHA := trimNewline(string(mergeResult.Stdout))

	runGit(t, runner, dir, "push", "origin", "feature/PROJ-20")

	// --- Seed: UAT advances independently — equiv.txt's content is
	// duplicated under a different SHA and an unrelated message. ---
	runGit(t, runner, dir, "checkout", "UAT")
	writeAndCommit(t, runner, dir, "equiv.txt", "equivalent-payload\n", "chore: apply content update directly")
	runGit(t, runner, dir, "push", "origin", "UAT")

	ctx := context.Background()

	// === Discover: the real HU-002 entry point this selection builds on. ===
	result, err := svc.Discover(ctx, dir, git.DiscoverOptions{
		Ticket: "PROJ-20",
		Target: "UAT",
		Source: "feature/PROJ-20",
	})
	if err != nil {
		t.Fatalf("Discover: unexpected error: %v", err)
	}

	wantOrder := []string{interleavedSHA, sharedFileSHA, dualTicketSHA, equivSHA, sideSHA, mergeSHA}
	if len(result.OrderedCommits) != len(wantOrder) {
		t.Fatalf("expected %d ordered commits, got %d: %+v", len(wantOrder), len(result.OrderedCommits), result.OrderedCommits)
	}
	for i, want := range wantOrder {
		if result.OrderedCommits[i].SHA != want {
			t.Fatalf("OrderedCommits[%d].SHA = %s, want %s (topological order)", i, result.OrderedCommits[i].SHA, want)
		}
	}

	// === Selection entry point: build the HU-003 model from discovery. ===
	items := git.NewCommitSelectionItems(result.OrderedCommits, "PROJ-20")

	byPosition := make(map[string]git.CommitSelectionItem, len(items))
	for _, it := range items {
		byPosition[it.SHA] = it
	}

	// --- Assert: merge commit is disabled with a reason and unselected. ---
	if m := byPosition[mergeSHA]; !m.Disabled || m.Selected || m.Reason != git.ReasonMergeCommit {
		t.Errorf("expected merge commit %s Disabled with ReasonMergeCommit, got %+v", mergeSHA, m)
	}

	// --- Assert: already-applied (content-equivalent) commit is disabled
	// with a reason and unselected. ---
	if e := byPosition[equivSHA]; !e.Disabled || e.Selected || e.Reason == "" {
		t.Errorf("expected already-applied commit %s Disabled with a Reason, got %+v", equivSHA, e)
	}

	// --- Assert: the dual-ticket commit shows a multi-ticket notice. ---
	dual := byPosition[dualTicketSHA]
	if !dual.MultiTicketNotice {
		t.Errorf("expected commit %s to carry a multi-ticket notice, got %+v", dualTicketSHA, dual)
	}
	if len(dual.OtherTickets) != 1 || dual.OtherTickets[0] != "PROJ-22" {
		t.Errorf("expected OtherTickets = [PROJ-22], got %v", dual.OtherTickets)
	}

	// --- Assert: plain commits (interleaved, sharedFile, side) are
	// selectable and selected by default. ---
	for _, plain := range []string{interleavedSHA, sharedFileSHA, sideSHA} {
		c := byPosition[plain]
		if c.Disabled || !c.Selected {
			t.Errorf("expected commit %s to be selected by default and not disabled, got %+v", plain, c)
		}
	}

	// === The user deselects the interleaved (other-ticket) commit,
	// keeping their own ticket's shared-file change selected. ===
	interleavedIndex := -1
	for i, it := range items {
		if it.SHA == interleavedSHA {
			interleavedIndex = i
		}
	}
	if interleavedIndex == -1 {
		t.Fatalf("interleaved commit %s not found in selection items", interleavedSHA)
	}
	items = git.ToggleSelection(items, interleavedIndex)
	if items[interleavedIndex].Selected {
		t.Fatalf("expected interleaved commit to be deselected after toggling")
	}

	selected := make(map[string]bool, len(items))
	for _, it := range items {
		if it.Selected {
			selected[it.SHA] = true
		}
	}

	// --- Assert: per-file dependency warning fires for the shared file,
	// naming the selected commit and the earlier unselected one. ---
	warnings, err := svc.DependencyWarnings(ctx, dir, result.OrderedCommits, selected)
	if err != nil {
		t.Fatalf("DependencyWarnings: unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 dependency warning, got %d: %+v", len(warnings), warnings)
	}
	if w := warnings[0]; w.File != "shared.txt" || w.SelectedSHA != sharedFileSHA || w.UnselectedSHA != interleavedSHA {
		t.Errorf("unexpected dependency warning: %+v", w)
	}

	// === Assert: confirming this valid, non-empty selection generates a
	// preliminary DeploymentPlan carrying exactly the selected commits, in
	// order. ===
	plan, err := git.GenerateDeploymentPlan("PROJ-20", items)
	if err != nil {
		t.Fatalf("GenerateDeploymentPlan: unexpected error: %v", err)
	}
	wantPlanOrder := []string{sharedFileSHA, dualTicketSHA, sideSHA}
	if len(plan.SelectedCommits) != len(wantPlanOrder) {
		t.Fatalf("expected %d commits in the plan, got %d: %+v", len(wantPlanOrder), len(plan.SelectedCommits), plan.SelectedCommits)
	}
	for i, want := range wantPlanOrder {
		if plan.SelectedCommits[i].SHA != want {
			t.Errorf("plan.SelectedCommits[%d].SHA = %s, want %s", i, plan.SelectedCommits[i].SHA, want)
		}
	}

	// === Assert: confirming with zero selected commits is blocked. ===
	t.Run("confirming with an empty selection is blocked", func(t *testing.T) {
		allDeselected := append([]git.CommitSelectionItem(nil), items...)
		for i, it := range allDeselected {
			if it.Selected {
				allDeselected = git.ToggleSelection(allDeselected, i)
			}
		}
		if err := git.ValidateSelection(allDeselected); !errors.Is(err, git.ErrEmptySelection) {
			t.Fatalf("expected ErrEmptySelection for an all-deselected selection, got: %v", err)
		}
		if _, err := git.GenerateDeploymentPlan("PROJ-20", allDeselected); !errors.Is(err, git.ErrEmptySelection) {
			t.Fatalf("expected GenerateDeploymentPlan to block an empty selection, got: %v", err)
		}
	})

	// === Variant: reordering is available ONLY in advanced mode, and
	// always carries the conflict-risk warning. ===
	t.Run("reordering in advanced mode shows a conflict-risk warning", func(t *testing.T) {
		if _, _, err := git.ReorderSelection(items, 0, 1, false); !errors.Is(err, git.ErrReorderRequiresAdvancedMode) {
			t.Fatalf("expected non-advanced reorder to be rejected, got: %v", err)
		}

		reordered, warning, err := git.ReorderSelection(items, 0, 1, true)
		if err != nil {
			t.Fatalf("ReorderSelection: unexpected error in advanced mode: %v", err)
		}
		if warning != git.ReorderConflictRiskWarning {
			t.Fatalf("expected the conflict-risk warning, got %q", warning)
		}
		if reordered[0].SHA != items[1].SHA || reordered[1].SHA != items[0].SHA {
			t.Fatalf("expected positions 0 and 1 swapped, got %+v", reordered[:2])
		}
	})
}
