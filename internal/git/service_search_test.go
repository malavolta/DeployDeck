package git_test

import (
	"context"
	"testing"
	"time"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// commitLogLine builds one raw `git log --format='%H%x09%h%x09%an%x09%aI%x09%P%x09%s'`
// line for canned FakeRunner responses in this file's tests.
func commitLogLine(sha, short, author, isoDate, parents, subject string) string {
	return sha + "\t" + short + "\t" + author + "\t" + isoDate + "\t" + parents + "\t" + subject
}

func TestService_SearchCommits_ParsesCannedLogOutput(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("/repo\n"),
	})
	runner.When("git", []string{"log", "--all", "--grep", "PROJ-1", "--format=%H%x09%h%x09%an%x09%aI%x09%P%x09%s"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(commitLogLine(
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"aaaaaaa",
			"Ada Lovelace",
			"2026-01-02T10:00:00+01:00",
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"PROJ-1: fix widget",
		) + "\n"),
	})

	svc := git.New(runner)
	commits, err := svc.SearchCommits(context.Background(), "/repo", "PROJ-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d: %+v", len(commits), commits)
	}

	got := commits[0]
	wantDate, err := time.Parse(time.RFC3339, "2026-01-02T10:00:00+01:00")
	if err != nil {
		t.Fatalf("test setup: bad date: %v", err)
	}

	if got.SHA != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("SHA = %q, want %q", got.SHA, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	}
	if got.ShortSHA != "aaaaaaa" {
		t.Errorf("ShortSHA = %q, want %q", got.ShortSHA, "aaaaaaa")
	}
	if got.Author != "Ada Lovelace" {
		t.Errorf("Author = %q, want %q", got.Author, "Ada Lovelace")
	}
	if !got.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", got.Date, wantDate)
	}
	if len(got.Parents) != 1 || got.Parents[0] != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Errorf("Parents = %v, want [bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb]", got.Parents)
	}
	if got.Subject != "PROJ-1: fix widget" {
		t.Errorf("Subject = %q, want %q", got.Subject, "PROJ-1: fix widget")
	}
	if got.IsMerge() {
		t.Errorf("expected a single-parent commit to not be a merge")
	}
}

// TestService_SearchCommits_CommitMentioningTwoTickets_MatchesEitherSearch
// proves a commit whose message references two tickets is returned when
// searching by EITHER ticket: the same canned commit data is registered
// under two different --grep arguments (one per ticket), simulating what
// git itself would return for a dual-ticket commit message, and both
// searches must resolve to that commit via the same parsing path.
func TestService_SearchCommits_CommitMentioningTwoTickets_MatchesEitherSearch(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("/repo\n")})

	dualTicketLine := commitLogLine(
		"cccccccccccccccccccccccccccccccccccccccc",
		"ccccccc",
		"Grace Hopper",
		"2026-02-01T09:00:00Z",
		"",
		"PROJ-1 PROJ-2: shared fix",
	) + "\n"

	runner.When("git", []string{"log", "--all", "--grep", "PROJ-1", "--format=%H%x09%h%x09%an%x09%aI%x09%P%x09%s"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(dualTicketLine),
	})
	runner.When("git", []string{"log", "--all", "--grep", "PROJ-2", "--format=%H%x09%h%x09%an%x09%aI%x09%P%x09%s"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(dualTicketLine),
	})

	svc := git.New(runner)

	for _, ticket := range []string{"PROJ-1", "PROJ-2"} {
		commits, err := svc.SearchCommits(context.Background(), "/repo", ticket)
		if err != nil {
			t.Fatalf("ticket %s: expected no error, got: %v", ticket, err)
		}
		if len(commits) != 1 {
			t.Fatalf("ticket %s: expected 1 commit, got %d", ticket, len(commits))
		}
		if commits[0].SHA != "cccccccccccccccccccccccccccccccccccccccc" {
			t.Errorf("ticket %s: SHA = %q, want the shared dual-ticket commit", ticket, commits[0].SHA)
		}
		if len(commits[0].Parents) != 0 {
			t.Errorf("ticket %s: expected a root commit (no parents), got %v", ticket, commits[0].Parents)
		}
		if commits[0].IsMerge() {
			t.Errorf("ticket %s: a root/single-parent commit must not be flagged as a merge", ticket)
		}
	}
}

func TestService_SearchCommits_NoMatches_ReturnsEmptyNotError(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("/repo\n")})
	runner.When("git", []string{"log", "--all", "--grep", "NOPE-1", "--format=%H%x09%h%x09%an%x09%aI%x09%P%x09%s"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(""),
	})

	svc := git.New(runner)
	commits, err := svc.SearchCommits(context.Background(), "/repo", "NOPE-1")
	if err != nil {
		t.Fatalf("expected no error for zero matches, got: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected 0 commits, got %d", len(commits))
	}
}
