package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_SearchCommits_Integration_TicketFoundInMessages seeds a real
// temp repo with commits mentioning a ticket (one of them mentioning a
// second ticket too) and an unrelated commit, then proves SearchCommits
// finds the ticket-related commits (via real `git log --all --grep`) for
// either ticket a dual-mention commit references, while excluding the
// unrelated one.
func TestService_SearchCommits_Integration_TicketFoundInMessages(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	writeAndCommit(t, runner, dir, "feature.txt", "feature work\n", "PROJ-1: add feature")
	writeAndCommit(t, runner, dir, "shared.txt", "shared work\n", "PROJ-1 PROJ-2: shared fix")
	writeAndCommit(t, runner, dir, "unrelated.txt", "unrelated\n", "chore: unrelated cleanup")

	commits, err := svc.SearchCommits(context.Background(), dir, "PROJ-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits for PROJ-1, got %d: %+v", len(commits), commits)
	}
	subjects := map[string]bool{}
	for _, c := range commits {
		subjects[c.Subject] = true
	}
	if !subjects["PROJ-1: add feature"] || !subjects["PROJ-1 PROJ-2: shared fix"] {
		t.Fatalf("expected both PROJ-1 commits present, got subjects: %v", subjects)
	}

	// The dual-ticket commit must also be found when searching by the
	// SECOND ticket it mentions.
	commitsForSecondTicket, err := svc.SearchCommits(context.Background(), dir, "PROJ-2")
	if err != nil {
		t.Fatalf("expected no error searching PROJ-2, got: %v", err)
	}
	if len(commitsForSecondTicket) != 1 || commitsForSecondTicket[0].Subject != "PROJ-1 PROJ-2: shared fix" {
		t.Fatalf("expected only the dual-ticket commit for PROJ-2, got: %+v", commitsForSecondTicket)
	}

	commitsForUnrelated, err := svc.SearchCommits(context.Background(), dir, "PROJ-999")
	if err != nil {
		t.Fatalf("expected no error for a ticket with no matches, got: %v", err)
	}
	if len(commitsForUnrelated) != 0 {
		t.Fatalf("expected 0 commits for an unmatched ticket, got %d", len(commitsForUnrelated))
	}
}

// writeAndCommit is a shared test helper (this file's integration suite)
// that writes a file at dir/name with content and commits it with message,
// via the real runner so the resulting commit is a genuine git object
// (real SHA, real parent chain) rather than canned data.
func writeAndCommit(t *testing.T, runner exec.Runner, dir, name, content, message string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	runGit(t, runner, dir, "add", name)
	runGit(t, runner, dir, "commit", "-m", message)

	result := runGit(t, runner, dir, "rev-parse", "HEAD")
	return trimNewline(string(result.Stdout))
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
