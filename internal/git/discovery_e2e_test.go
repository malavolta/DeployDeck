package git_test

import (
	"context"
	"errors"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestHU002_Discover_E2E is the consolidated end-to-end scenario from
// docs/HISTORIAS.md HU-002's "Test E2E" section, run through the real
// discovery entry point (Service.Discover) against a real temp repo (no
// FakeRunner): a source branch whose commits have author dates deliberately
// inverted relative to topological order, a commit already merged into
// the target by identical SHA (via a real non-squash merge commit), a
// commit whose content was independently applied to the target under a
// different SHA (content equivalence), a merge commit within the source
// range, a commit mentioning two tickets, and a second candidate branch
// forcing single-source-branch selection.
func TestHU002_Discover_E2E(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// --- Seed: UAT (target) branches off main and is pushed. ---
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	runGit(t, runner, dir, "push", "origin", "UAT")

	// --- Seed: feature/PROJ-1 (source) branches off UAT. ---
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-1", "UAT")

	// A: will be merged directly into UAT below (identical-SHA
	// already-applied case). Kept OUT of the author-date inversion pair,
	// since a commit merged by identical SHA is correctly EXCLUDED from
	// the `origin/UAT..origin/feature/PROJ-1` delta by git's own range
	// semantics — it is proven separately below via IsAncestor, not via
	// OrderedCommits.
	aSHA := writeAndCommit(t, runner, dir, "a.txt", "a\n", "PROJ-1: base commit")

	// B -> C is the author-date inversion pair that stays in the delta:
	// B (parent) has a LATER author date than C (its own child), proving
	// CommitsInRange orders by topology, not date.
	bSHA := commitWithAuthorDate(t, runner, dir, "b.txt", "b\n", "PROJ-1: parent with later author date", "2026-06-01T10:00:00+00:00")
	cSHA := commitWithAuthorDate(t, runner, dir, "c.txt", "c\n", "PROJ-1: child with earlier author date", "2026-01-01T10:00:00+00:00")

	// D: content later duplicated on UAT under a different SHA (content
	// equivalence via git cherry).
	dSHA := writeAndCommit(t, runner, dir, "equiv.txt", "equivalent-payload\n", "PROJ-1: equivalent content commit")

	// A merge commit (M, parents D and S) within the source range, which
	// must be flagged and blocked from selection.
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-1-side", "feature/PROJ-1")
	sSHA := writeAndCommit(t, runner, dir, "side.txt", "side\n", "PROJ-1: side change")
	runGit(t, runner, dir, "checkout", "feature/PROJ-1")
	runGit(t, runner, dir, "merge", "--no-ff", "feature/PROJ-1-side", "-m", "PROJ-1: merge side branch")
	mResult := runGit(t, runner, dir, "rev-parse", "HEAD")
	mSHA := trimNewline(string(mResult.Stdout))

	// F: mentions a second ticket too (dual-ticket commit).
	fSHA := writeAndCommit(t, runner, dir, "shared.txt", "shared\n", "PROJ-1 PROJ-2: shared fix")

	runGit(t, runner, dir, "push", "origin", "feature/PROJ-1")

	// --- Seed: a second candidate branch for the same ticket. ---
	runGit(t, runner, dir, "checkout", "-b", "hotfix/PROJ-1", "UAT")
	writeAndCommit(t, runner, dir, "hotfix.txt", "hotfix\n", "PROJ-1: hotfix stub")
	runGit(t, runner, dir, "push", "origin", "hotfix/PROJ-1")

	// --- Seed: UAT advances independently — A merged by identical SHA
	// (real --no-ff merge, not a squash), plus D's content duplicated
	// under a different SHA and an unrelated message. ---
	runGit(t, runner, dir, "checkout", "UAT")
	runGit(t, runner, dir, "merge", "--no-ff", aSHA, "-m", "chore: fast-track a commit directly to UAT")
	writeAndCommit(t, runner, dir, "equiv.txt", "equivalent-payload\n", "chore: apply content update directly")
	runGit(t, runner, dir, "push", "origin", "UAT")

	ctx := context.Background()

	// === Assert: ticket message search lists the ticket's commits,
	// including the one already merged into target (--all scope is not
	// range-limited) and the dual-ticket commit found via either ticket. ===
	matched, err := svc.SearchCommits(ctx, dir, "PROJ-1")
	if err != nil {
		t.Fatalf("SearchCommits: unexpected error: %v", err)
	}
	matchedSHAs := commitSHASet(matched)
	for _, want := range []string{aSHA, bSHA, cSHA, dSHA, sSHA, mSHA, fSHA} {
		if !matchedSHAs[want] {
			t.Errorf("expected SearchCommits(PROJ-1) to include %s, got %d commits", want, len(matched))
		}
	}

	matchedForSecondTicket, err := svc.SearchCommits(ctx, dir, "PROJ-2")
	if err != nil {
		t.Fatalf("SearchCommits(PROJ-2): unexpected error: %v", err)
	}
	if !commitSHASet(matchedForSecondTicket)[fSHA] {
		t.Fatalf("expected the dual-ticket commit to also be found searching PROJ-2")
	}

	// === Assert: candidate branches are listed by name, and single-source
	// enforcement requires an explicit choice among them. ===
	candidates, err := svc.CandidateBranches(ctx, dir, "PROJ-1", config.Config{})
	if err != nil {
		t.Fatalf("CandidateBranches: unexpected error: %v", err)
	}
	if !branchNameSet(candidates)["feature/PROJ-1"] || !branchNameSet(candidates)["hotfix/PROJ-1"] {
		t.Fatalf("expected both feature/PROJ-1 and hotfix/PROJ-1 among candidates, got %+v", candidates)
	}
	if len(candidates) <= 1 {
		t.Fatalf("expected more than one candidate branch to force single-source selection, got %+v", candidates)
	}

	if _, err := git.SelectSingleSource(candidates, ""); !errors.Is(err, git.ErrMultipleSourceBranches) {
		t.Fatalf("expected continuing without a choice to block with ErrMultipleSourceBranches, got: %v", err)
	}
	resolvedSource, err := git.SelectSingleSource(candidates, "feature/PROJ-1")
	if err != nil {
		t.Fatalf("expected selecting feature/PROJ-1 to succeed, got error: %v", err)
	}
	if resolvedSource.Name != "feature/PROJ-1" {
		t.Fatalf("expected resolved source feature/PROJ-1, got %+v", resolvedSource)
	}

	// === Assert: A, already merged into UAT by identical SHA, is
	// detected via IsAncestor — proven directly rather than via
	// OrderedCommits, since a commit that is genuinely an ancestor of
	// target is (correctly) excluded from the origin/UAT..origin/source
	// delta by git's own range semantics; it simply is not "new" anymore. ===
	isAncestor, err := svc.IsAncestor(ctx, dir, aSHA, "origin/UAT")
	if err != nil {
		t.Fatalf("IsAncestor: unexpected error: %v", err)
	}
	if !isAncestor {
		t.Fatalf("expected the directly-merged commit to be an ancestor of origin/UAT")
	}
	if status := git.ClassifyEquivalence(isAncestor, 0, "", nil); status != git.AlreadyAppliedBySHA {
		t.Fatalf("expected AlreadyAppliedBySHA, got %v", status)
	}

	// === Assert: the full discovery entry point, run with a resolved
	// target+source, returns the delta topologically ordered, flags the
	// merge commit, and detects the content-equivalent commit. ===
	result, err := svc.Discover(ctx, dir, git.DiscoverOptions{
		Ticket: "PROJ-1",
		Target: "UAT",
		Source: resolvedSource.Name,
	})
	if err != nil {
		t.Fatalf("Discover: unexpected error: %v", err)
	}

	wantOrder := []string{bSHA, cSHA, dSHA, sSHA, mSHA, fSHA}
	if len(result.OrderedCommits) != len(wantOrder) {
		t.Fatalf("expected %d ordered commits, got %d: %+v", len(wantOrder), len(result.OrderedCommits), result.OrderedCommits)
	}
	for i, want := range wantOrder {
		if result.OrderedCommits[i].SHA != want {
			t.Errorf("OrderedCommits[%d].SHA = %s, want %s (topological order)", i, result.OrderedCommits[i].SHA, want)
		}
	}

	// The author-date inversion actually exists in the seeded data, so a
	// correct topo-order assertion above is not an accident of equal
	// dates.
	if !result.OrderedCommits[0].Date.After(result.OrderedCommits[1].Date) {
		t.Fatalf("test setup did not produce an author-date inversion between B and C")
	}

	byPosition := map[string]git.DiscoveredCommit{}
	for _, c := range result.OrderedCommits {
		byPosition[c.SHA] = c
	}

	if m := byPosition[mSHA]; !m.Merge || m.SelectableByDefault() {
		t.Fatalf("expected the merge commit %s flagged Merge=true and not selectable by default, got %+v", mSHA, m)
	}
	if d := byPosition[dSHA]; d.Equivalence != git.EquivalentByCherry || d.SelectableByDefault() {
		t.Fatalf("expected the content-equivalent commit %s classified EquivalentByCherry and not selectable by default, got %+v", dSHA, d)
	}
	for _, plain := range []string{bSHA, cSHA, sSHA, fSHA} {
		c := byPosition[plain]
		if c.Merge || c.AlreadyApplied() || !c.SelectableByDefault() {
			t.Errorf("expected commit %s to be a plain, selectable-by-default candidate, got %+v", plain, c)
		}
	}

	// The mixed applied/pending classification within the same ticket
	// (D applied, others pending) triggers the best-effort squash-merge
	// courtesy warning.
	if !containsString(result.Warnings, "squash-merge-history") {
		t.Errorf("expected a squash-merge-history warning given the mixed applied/pending classification, got %v", result.Warnings)
	}
	if containsString(result.Warnings, "deleted-source-branch") {
		t.Errorf("did not expect a deleted-source-branch warning: the source branch exists, got %v", result.Warnings)
	}
	if len(result.Alternatives) != 0 {
		t.Errorf("expected no no-results alternatives given real matches, got %v", result.Alternatives)
	}
}

// TestHU002_Discover_E2E_Variants covers the three named HU-002 E2E
// variants that don't fit the primary scenario's shared seed: a deleted
// source branch (grep-only warning), and a ticket with zero results
// (actionable alternatives).
func TestHU002_Discover_E2E_Variants(t *testing.T) {
	t.Run("deleted source branch narrows search to grep-only with a warning", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-9", "main")
		writeAndCommit(t, runner, dir, "f.txt", "f\n", "PROJ-9: feature work")
		runGit(t, runner, dir, "checkout", "main")
		runGit(t, runner, dir, "merge", "--no-ff", "-m", "Merge feature/PROJ-9", "feature/PROJ-9")
		runGit(t, runner, dir, "branch", "-D", "feature/PROJ-9")
		runGit(t, runner, dir, "push", "origin", "main")

		result, err := svc.Discover(context.Background(), dir, git.DiscoverOptions{Ticket: "PROJ-9"})
		if err != nil {
			t.Fatalf("Discover: unexpected error: %v", err)
		}
		if len(result.MatchedCommits) == 0 {
			t.Fatalf("expected the commit to still be found by message search")
		}
		if len(result.CandidateBranches) != 0 {
			t.Fatalf("expected no candidate branches after deletion, got %+v", result.CandidateBranches)
		}
		if !containsString(result.Warnings, "deleted-source-branch") {
			t.Fatalf("expected a deleted-source-branch warning, got %v", result.Warnings)
		}
	})

	t.Run("ticket with no results shows actionable alternatives", func(t *testing.T) {
		dir := newTempRepo(t)
		svc := git.New(exec.NewOSRunner())

		result, err := svc.Discover(context.Background(), dir, git.DiscoverOptions{Ticket: "NOPE-404"})
		if err != nil {
			t.Fatalf("Discover: unexpected error: %v", err)
		}
		if len(result.MatchedCommits) != 0 || len(result.CandidateBranches) != 0 {
			t.Fatalf("expected zero matches/candidates for an unmatched ticket, got %+v / %+v", result.MatchedCommits, result.CandidateBranches)
		}
		want := map[string]bool{
			git.AlternativeManualSearch: false,
			git.AlternativeChangeTicket: false,
			git.AlternativeSelectBranch: false,
		}
		if len(result.Alternatives) != len(want) {
			t.Fatalf("expected %d alternatives, got %v", len(want), result.Alternatives)
		}
		for _, a := range result.Alternatives {
			if _, ok := want[a]; !ok {
				t.Fatalf("unexpected alternative %q in %v", a, result.Alternatives)
			}
		}
	})
}

// TestHU002_Discover_E2E_SourceResolutionDedupe is task 4.1: a local+origin
// twin, promoted to the first pipeline environment, resolves to exactly 1
// deduped candidate with a non-empty range; a distinct-candidates companion
// proves dedupe never over-collapses two genuinely different branches.
func TestHU002_Discover_E2E_SourceResolutionDedupe(t *testing.T) {
	t.Run("local+origin twin dedupes to 1 candidate with a non-empty range", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)
		ctx := context.Background()

		runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
		runGit(t, runner, dir, "push", "origin", "UAT")

		runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-3", "UAT")
		sha := writeAndCommit(t, runner, dir, "a.txt", "a\n", "PROJ-3: change")
		runGit(t, runner, dir, "push", "origin", "feature/PROJ-3")

		base, err := svc.Discover(ctx, dir, git.DiscoverOptions{Ticket: "PROJ-3"})
		if err != nil {
			t.Fatalf("Discover (pass 1): unexpected error: %v", err)
		}
		if len(base.CandidateBranches) != 1 {
			t.Fatalf("expected exactly 1 deduped candidate, got %d: %+v", len(base.CandidateBranches), base.CandidateBranches)
		}

		resolvedSource, err := git.SelectSingleSource(base.CandidateBranches, "")
		if err != nil {
			t.Fatalf("expected the single deduped candidate to auto-resolve without an explicit selection, got error: %v", err)
		}
		if resolvedSource.Name != "feature/PROJ-3" {
			t.Fatalf("resolved source = %q, want the bare/local form feature/PROJ-3", resolvedSource.Name)
		}

		full, err := svc.Discover(ctx, dir, git.DiscoverOptions{
			Ticket: "PROJ-3",
			Target: "UAT",
			Source: resolvedSource.Name,
		})
		if err != nil {
			t.Fatalf("Discover (pass 2): unexpected error: %v", err)
		}
		if len(full.OrderedCommits) != 1 || full.OrderedCommits[0].SHA != sha {
			t.Fatalf("expected exactly 1 ordered commit (%s), not a dead end, got %+v", sha, full.OrderedCommits)
		}
	})

	t.Run("distinct candidates are never over-collapsed", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)
		ctx := context.Background()

		runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
		runGit(t, runner, dir, "push", "origin", "UAT")

		runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-4-a", "UAT")
		writeAndCommit(t, runner, dir, "a.txt", "a\n", "PROJ-4: change a")
		runGit(t, runner, dir, "push", "origin", "feature/PROJ-4-a")

		runGit(t, runner, dir, "checkout", "-b", "hotfix/PROJ-4-b", "UAT")
		writeAndCommit(t, runner, dir, "b.txt", "b\n", "PROJ-4: change b")
		runGit(t, runner, dir, "push", "origin", "hotfix/PROJ-4-b")

		candidates, err := svc.CandidateBranches(ctx, dir, "PROJ-4", config.Config{})
		if err != nil {
			t.Fatalf("CandidateBranches: unexpected error: %v", err)
		}
		if len(candidates) != 2 {
			t.Fatalf("expected 2 genuinely distinct candidates, got %d: %+v", len(candidates), candidates)
		}

		if _, err := git.SelectSingleSource(candidates, ""); !errors.Is(err, git.ErrMultipleSourceBranches) {
			t.Fatalf("expected an unresolved ambiguous selection to block with ErrMultipleSourceBranches, got: %v", err)
		}
		resolved, err := git.SelectSingleSource(candidates, "feature/PROJ-4-a")
		if err != nil {
			t.Fatalf("expected selecting feature/PROJ-4-a to succeed, got error: %v", err)
		}
		if resolved.Name != "feature/PROJ-4-a" {
			t.Fatalf("resolved source = %q, want feature/PROJ-4-a", resolved.Name)
		}
	})
}

func commitSHASet(commits []git.Commit) map[string]bool {
	set := make(map[string]bool, len(commits))
	for _, c := range commits {
		set[c.SHA] = true
	}
	return set
}

func branchNameSet(branches []git.Branch) map[string]bool {
	set := make(map[string]bool, len(branches))
	for _, b := range branches {
		set[b.Name] = true
	}
	return set
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
