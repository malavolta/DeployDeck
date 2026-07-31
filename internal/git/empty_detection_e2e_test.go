package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestHU006_EmptyDetection_SpoofSubjectCleanApply_NotEmpty reproduces the H1
// data-loss class: a genuine, clean cherry-pick of a commit whose SUBJECT
// literally contains "is now empty" must NOT be reported as an empty pick.
// A successful pick echoes each applied commit's subject to stdout, so a
// substring match on the echoed output is spoofable; the empty signal must
// come from repo state (a pick in progress with a clean tree), not the text.
func TestHU006_EmptyDetection_SpoofSubjectCleanApply_NotEmpty(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// UAT touches only its own file, so the feature commit applies cleanly.
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "uat-only.txt", "uat\n")
	runGit(t, runner, dir, "add", "uat-only.txt")
	runGit(t, runner, dir, "commit", "-m", "UAT-only change")
	runGit(t, runner, dir, "push", "origin", "UAT")

	// One feature commit whose SUBJECT contains "is now empty", touching a
	// file UAT never touches -> it applies with NO conflict and is NOT empty.
	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	spoof := writeAndCommit(t, runner, dir, "spoof.cls", "public class Spoof {}\n",
		"chore: the weekly report is now empty until a filter is applied")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")

	outcome, err := svc.CherryPick(context.Background(), dir,
		[]git.DiscoveredCommit{{Commit: git.Commit{SHA: spoof}}}, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if outcome.State.InProgress {
		t.Fatalf("expected a clean completed pick, got InProgress=true: %+v", outcome.State)
	}
	if outcome.Empty {
		t.Fatalf("a commit whose SUBJECT contains %q applied CLEANLY must not be reported empty; got Empty=true, EmptyMessage=%q",
			"is now empty", outcome.EmptyMessage)
	}
}

// TestHU006_EmptyDetection_SpoofSubjectConflict_NotEmptySurfacesConflict is
// the worst-case H1 scenario: a conflicting pick whose subject contains
// "is now empty". Git echoes the subject in its "could not apply <sha>...
// <subject>" line, so a substring match would set Empty=true ALONGSIDE a real
// unresolved conflict — and an Empty-first consumer would `--skip` and
// silently DROP the conflicting selected commit. Empty must be false and the
// conflict must be surfaced.
func TestHU006_EmptyDetection_SpoofSubjectConflict_NotEmptySurfacesConflict(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	writeFileHelper(t, dir, "a.cls", "l1\nBASE\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "seed a.cls baseline")
	runGit(t, runner, dir, "push", "origin", "main")

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "a.cls", "l1\nUAT\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "UAT diverges a.cls")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	spoof := writeAndCommit(t, runner, dir, "a.cls", "l1\nFEATURE\nl3\n",
		"fix: dashboard is now empty until a filter is chosen")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")

	outcome, err := svc.CherryPick(context.Background(), dir,
		[]git.DiscoveredCommit{{Commit: git.Commit{SHA: spoof}}}, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if len(outcome.State.Unmerged) != 1 || outcome.State.Unmerged[0].Path != "a.cls" {
		t.Fatalf("expected the real a.cls conflict surfaced, got %+v", outcome.State.Unmerged)
	}
	if outcome.Empty {
		t.Fatalf("DATA-LOSS GUARD: Empty=true reported alongside %d unresolved conflict(s) — an Empty-first consumer would --skip and drop the conflicting commit",
			len(outcome.State.Unmerged))
	}
}

// TestHU006_EmptyDetection_GenuinelyAlreadyApplied_IsEmpty guards that the
// repo-state signal still detects a real empty pick: UAT already contains the
// feature commit's change under a different SHA, so applying it produces no
// change and stops empty with a clean tree and zero unmerged paths.
func TestHU006_EmptyDetection_GenuinelyAlreadyApplied_IsEmpty(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	writeFileHelper(t, dir, "a.cls", "x\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "seed a.cls")
	runGit(t, runner, dir, "push", "origin", "main")

	// UAT already has the exact change the feature commit makes (different SHA).
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "a.cls", "x\nADDED\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "UAT already has the change (diff sha)")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	feat := writeAndCommit(t, runner, dir, "a.cls", "x\nADDED\n", "PROJ-1: add line (already in UAT)")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")

	outcome, err := svc.CherryPick(context.Background(), dir,
		[]git.DiscoveredCommit{{Commit: git.Commit{SHA: feat}}}, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if !outcome.Empty {
		t.Fatalf("a genuinely already-applied commit must be detected empty, got %+v", outcome)
	}
	if len(outcome.State.Unmerged) != 0 {
		t.Errorf("a genuine empty pick has no unmerged files, got %+v", outcome.State.Unmerged)
	}
}

// TestHU006_Rerere_SpoofSubject_NotFalseFlaggedAsAutoResolved reproduces the
// H1 sibling class in the rerere parser: a commit whose SUBJECT is exactly
// git's "Resolved '<path>' using previous resolution." line, applied cleanly
// with NO rerere involved, must not be parsed as an auto-resolved path. The
// subject echo lands on stdout prefixed by "[branch sha] "; the real rerere
// diagnostic is a line-anchored message on stderr.
func TestHU006_Rerere_SpoofSubject_NotFalseFlaggedAsAutoResolved(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "uat-only.txt", "uat\n")
	runGit(t, runner, dir, "add", "uat-only.txt")
	runGit(t, runner, dir, "commit", "-m", "UAT-only change")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	spoof := writeAndCommit(t, runner, dir, "clean.cls", "public class Clean {}\n",
		"Resolved 'evil.cls' using previous resolution.")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")

	outcome, err := svc.CherryPick(context.Background(), dir,
		[]git.DiscoveredCommit{{Commit: git.Commit{SHA: spoof}}}, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if len(outcome.RerereResolved) != 0 {
		t.Fatalf("a commit SUBJECT echoing git's rerere line must not be parsed as an auto-resolved path; got RerereResolved=%v",
			outcome.RerereResolved)
	}
}
