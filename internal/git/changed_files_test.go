package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_ChangedFiles_WrapsGitDiffNameOnly proves ChangedFiles
// resolves dir to its repo root (like every other Service method — see
// RepoRoot) and then wraps `git diff --name-only <from>..<to>` — reusing
// the same NUL-safe -z helper VerifyPromotedContent already relies on
// (internal/delta.Summarize's OutsideSourceDirs check is this method's
// first consumer).
func TestService_ChangedFiles_WrapsGitDiffNameOnly(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("/repo\n"),
	})
	runner.When("git", []string{"diff", "--name-only", "-z", "origin/UAT", "HEAD"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("force-app/main/default/classes/A.cls\x00README.md\x00"),
	})

	svc := git.New(runner)
	got, err := svc.ChangedFiles(context.Background(), "/repo/sub", "origin/UAT", "HEAD")
	if err != nil {
		t.Fatalf("ChangedFiles() unexpected error: %v", err)
	}

	want := []string{"force-app/main/default/classes/A.cls", "README.md"}
	if len(got) != len(want) {
		t.Fatalf("ChangedFiles() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ChangedFiles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestService_ChangedFiles_NoChanges proves an empty diff returns an empty
// (not error) result.
func TestService_ChangedFiles_NoChanges(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("/repo\n"),
	})
	runner.When("git", []string{"diff", "--name-only", "-z", "origin/UAT", "HEAD"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(""),
	})

	svc := git.New(runner)
	got, err := svc.ChangedFiles(context.Background(), "/repo", "origin/UAT", "HEAD")
	if err != nil {
		t.Fatalf("ChangedFiles() unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ChangedFiles() = %v, want empty", got)
	}
}

// TestService_ChangedFiles_PropagatesRepoRootError proves a directory
// outside a git repository surfaces RepoRoot's error rather than proceeding
// with an untrusted Dir.
func TestService_ChangedFiles_PropagatesRepoRootError(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{
		ExitCode: 128,
		Stderr:   []byte("fatal: not a git repository"),
	})

	svc := git.New(runner)
	if _, err := svc.ChangedFiles(context.Background(), "/tmp/not-a-repo", "origin/UAT", "HEAD"); err == nil {
		t.Fatal("expected an error when the directory is not inside a git repository")
	}
}
