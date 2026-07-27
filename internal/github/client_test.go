package github_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/github"
)

// TestClient_AuthStatus is task 2.3 (RED): a single `gh auth status`
// invocation classifies gh into 3 states via the Runner's
// non-zero-exit-as-data contract (HU-014 AC5/AC6): a Runner error (binary
// missing / cannot start) means Absent, a nil error with a non-zero exit
// means Unauthenticated, and a zero exit means Authenticated.
func TestClient_AuthStatus(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*exec.FakeRunner)
		want  github.AuthState
	}{
		{
			name: "gh binary missing returns AuthAbsent",
			// No canned response registered: FakeRunner.Run returns a
			// Runner error for any unmatched request, mirroring a
			// missing-binary start failure.
			setup: func(r *exec.FakeRunner) {},
			want:  github.AuthAbsent,
		},
		{
			name: "gh present but unauthenticated returns AuthUnauthenticated",
			setup: func(r *exec.FakeRunner) {
				r.When("gh", []string{"auth", "status"}, exec.CommandResult{ExitCode: 1, Stderr: []byte("You are not logged into any GitHub hosts")})
			},
			want: github.AuthUnauthenticated,
		},
		{
			name: "gh present and authenticated returns AuthAuthenticated",
			setup: func(r *exec.FakeRunner) {
				r.When("gh", []string{"auth", "status"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("Logged in to github.com as octocat")})
			},
			want: github.AuthAuthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := exec.NewFakeRunner()
			tt.setup(runner)
			c := github.New(runner)

			got := c.AuthStatus(context.Background())
			if got != tt.want {
				t.Fatalf("AuthStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestClient_CreatePR_Success_ParsesURLAndKeepsRaw is task 2.5 (RED):
// CreatePR runs a fully non-interactive, arg-slice `gh pr create` (never a
// shell), parses the resulting URL from stdout on success, and preserves
// Raw (HU-014 AC5/AC7).
func TestClient_CreatePR_Success_ParsesURLAndKeepsRaw(t *testing.T) {
	runner := exec.NewFakeRunner()
	prURL := "https://github.com/org/repo/pull/42"
	wantArgs := []string{"pr", "create", "--base", "main", "--head", "deploy/PROJ-1-to-main", "--title", "PROJ-1 - Promote changes to main", "--body", ""}
	runner.When("gh", wantArgs, exec.CommandResult{ExitCode: 0, Stdout: []byte(prURL + "\n")})

	c := github.New(runner)
	url, raw, err := c.CreatePR(context.Background(), "main", "deploy/PROJ-1-to-main", "PROJ-1 - Promote changes to main")
	if err != nil {
		t.Fatalf("CreatePR: unexpected error: %v", err)
	}
	if url != prURL {
		t.Fatalf("CreatePR() url = %q, want %q", url, prURL)
	}
	if !strings.Contains(raw, prURL) {
		t.Fatalf("expected Raw to preserve the command output, got %q", raw)
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(runner.Calls))
	}
	gotArgs := runner.Calls[0].Args
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(wantArgs), len(gotArgs), gotArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("arg %d = %q, want %q (full: %v)", i, gotArgs[i], wantArgs[i], gotArgs)
		}
	}
}

// TestClient_CreatePR_Failure_ReturnsErrorAndKeepsRaw is task 2.5 (RED)'s
// failure-path companion: a non-zero exit returns an error while still
// preserving Raw, so a caller can show the failed command's output
// (HU-014 AC7).
func TestClient_CreatePR_Failure_ReturnsErrorAndKeepsRaw(t *testing.T) {
	runner := exec.NewFakeRunner()
	args := []string{"pr", "create", "--base", "main", "--head", "deploy/PROJ-1-to-main", "--title", "title", "--body", ""}
	runner.When("gh", args, exec.CommandResult{ExitCode: 1, Stderr: []byte("pull request create failed: no commits between main and deploy/PROJ-1-to-main")})

	c := github.New(runner)
	url, raw, err := c.CreatePR(context.Background(), "main", "deploy/PROJ-1-to-main", "title")
	if err == nil {
		t.Fatal("CreatePR: expected an error on non-zero exit")
	}
	if url != "" {
		t.Fatalf("expected an empty url on failure, got %q", url)
	}
	if !strings.Contains(raw, "no commits between") {
		t.Fatalf("expected Raw to be preserved on failure, got %q", raw)
	}
}

// TestClient_CreatePR_RunnerError_KeepsEmptyRawAndReturnsError covers the
// gh-binary-missing path: a Runner error (never a shell) must still return
// an error, an empty url, and must not panic on an empty Raw.
func TestClient_CreatePR_RunnerError_KeepsEmptyRawAndReturnsError(t *testing.T) {
	runner := exec.NewFakeRunner() // no canned response: Runner error

	c := github.New(runner)
	url, _, err := c.CreatePR(context.Background(), "main", "head-branch", "title")
	if err == nil {
		t.Fatal("CreatePR: expected an error when gh cannot be run at all")
	}
	if url != "" {
		t.Fatalf("expected an empty url, got %q", url)
	}
}

// TestSuggestedTitle is task 2.7 (RED+GREEN): the suggested PR title format
// (HU-014 AC4): "<ticket> - Promote changes to <target>".
func TestSuggestedTitle(t *testing.T) {
	got := github.SuggestedTitle("PROJ-1", "UAT")
	want := "PROJ-1 - Promote changes to UAT"
	if got != want {
		t.Fatalf("SuggestedTitle() = %q, want %q", got, want)
	}
}
