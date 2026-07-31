package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestService_CommitsInRange_OrdersTopologically_OverridingInvertedAuthorDates
// seeds two commits on a source branch, off UAT (the target), where the
// CHILD commit's author date is EARLIER than its PARENT's — proving that
// a naive chronological-by-author-date sort would invert them, while
// `git rev-list --reverse --topo-order` (what CommitsInRange must use)
// respects the real parent-before-child history instead (HU-002:
// "las fechas de autor no sobreviven fiablemente a rebases y amends").
func TestService_CommitsInRange_OrdersTopologically_OverridingInvertedAuthorDates(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-1", "UAT")

	// Parent commit: author date LATER (2026-06-01).
	parentSHA := commitWithAuthorDate(t, runner, dir, "a.txt", "a\n", "PROJ-1: parent (later author date)", "2026-06-01T10:00:00+00:00")
	// Child commit: author date EARLIER than its own parent (2026-01-01) —
	// this is the inversion under test.
	childSHA := commitWithAuthorDate(t, runner, dir, "b.txt", "b\n", "PROJ-1: child (earlier author date)", "2026-01-01T10:00:00+00:00")

	runGit(t, runner, dir, "push", "origin", "feature/PROJ-1")

	commits, err := svc.CommitsInRange(context.Background(), dir, "UAT", "feature/PROJ-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits in range, got %d: %+v", len(commits), commits)
	}

	if commits[0].SHA != parentSHA {
		t.Errorf("expected the PARENT commit first (topological order), got %+v as commits[0]", commits[0])
	}
	if commits[1].SHA != childSHA {
		t.Errorf("expected the CHILD commit second (topological order), got %+v as commits[1]", commits[1])
	}

	// Sanity: prove the inversion actually exists in the seeded data, so a
	// passing assertion above is not an accident of equal dates.
	if !commits[0].Date.After(commits[1].Date) {
		t.Fatalf("test setup did not produce an author-date inversion: parent.Date=%v, child.Date=%v", commits[0].Date, commits[1].Date)
	}
}

// commitWithAuthorDate writes a file and commits it with an explicit
// author date (via `git commit --date`), so tests can construct histories
// where author-date order deliberately diverges from topological order.
func commitWithAuthorDate(t *testing.T, runner exec.Runner, dir, name, content, message, isoDate string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	runGit(t, runner, dir, "add", name)
	runGit(t, runner, dir, "commit", "--date="+isoDate, "-m", message)

	result := runGit(t, runner, dir, "rev-parse", "HEAD")
	return trimNewline(string(result.Stdout))
}
