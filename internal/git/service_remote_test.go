package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_RemoteURL_ReturnsTrimmedOriginURL is task 1.3 (RED): RemoteURL
// returns the trimmed stdout of `git remote get-url <name>`, tested against
// both SSH and HTTPS origin URL forms (HU-014's CompareURL normalizer
// consumes exactly this value).
func TestService_RemoteURL_ReturnsTrimmedOriginURL(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)
	ctx := context.Background()

	cases := []struct {
		name string
		url  string
	}{
		{"SSH origin", "git@github.com:org/repo.git"},
		{"HTTPS origin", "https://github.com/org/repo.git"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runGit(t, runner, dir, "remote", "set-url", "origin", tc.url)

			got, err := svc.RemoteURL(ctx, dir, "origin")
			if err != nil {
				t.Fatalf("RemoteURL: unexpected error: %v", err)
			}
			if got != tc.url {
				t.Fatalf("RemoteURL() = %q, want %q", got, tc.url)
			}
		})
	}
}

// TestService_RemoteURL_UnknownRemoteErrors is RemoteURL's failure-path
// companion, sibling to HasRemote's own non-zero-exit handling: an unknown
// remote name must return an error, not a silent empty string.
func TestService_RemoteURL_UnknownRemoteErrors(t *testing.T) {
	dir := newTempRepo(t)
	svc := git.New(exec.NewOSRunner())

	if _, err := svc.RemoteURL(context.Background(), dir, "does-not-exist"); err == nil {
		t.Fatal("expected RemoteURL to error for an unconfigured remote")
	}
}
