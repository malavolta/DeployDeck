package github_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/github"
)

// TestClient_PRForBranch is task 1.7 (RED), mirroring client_test.go's
// exec.FakeRunner pattern: `gh pr view <branch> --json url,state` is
// exit-code-as-data (incremental-promotion design: "PR detection ... via
// `gh pr view <branch> --json url,state`") — an open PR reports open=true, a
// closed/merged PR reports open=false with its URL still set (the caller
// falls through to create + notice), no PR at all (gh's own non-zero exit)
// degrades to open=false/err=nil, and a genuine Runner failure (gh missing)
// surfaces as err.
func TestClient_PRForBranch(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	wantArgs := []string{"pr", "view", branch, "--json", "url,state"}

	tests := []struct {
		name     string
		setup    func(*exec.FakeRunner)
		wantURL  string
		wantOpen bool
		wantErr  bool
	}{
		{
			name: "open PR reports open=true and its URL",
			setup: func(r *exec.FakeRunner) {
				r.When("gh", wantArgs, exec.CommandResult{
					ExitCode: 0,
					Stdout:   []byte(`{"url":"https://github.com/org/repo/pull/7","state":"OPEN"}`),
				})
			},
			wantURL:  "https://github.com/org/repo/pull/7",
			wantOpen: true,
		},
		{
			name: "closed PR reports open=false but keeps the URL",
			setup: func(r *exec.FakeRunner) {
				r.When("gh", wantArgs, exec.CommandResult{
					ExitCode: 0,
					Stdout:   []byte(`{"url":"https://github.com/org/repo/pull/3","state":"CLOSED"}`),
				})
			},
			wantURL:  "https://github.com/org/repo/pull/3",
			wantOpen: false,
		},
		{
			name: "merged PR reports open=false but keeps the URL",
			setup: func(r *exec.FakeRunner) {
				r.When("gh", wantArgs, exec.CommandResult{
					ExitCode: 0,
					Stdout:   []byte(`{"url":"https://github.com/org/repo/pull/9","state":"MERGED"}`),
				})
			},
			wantURL:  "https://github.com/org/repo/pull/9",
			wantOpen: false,
		},
		{
			name: "no PR for the branch: gh's own non-zero exit degrades to open=false, err=nil",
			setup: func(r *exec.FakeRunner) {
				r.When("gh", wantArgs, exec.CommandResult{
					ExitCode: 1,
					Stderr:   []byte("no pull requests found for branch \"deploy/PROJ-1-to-UAT\""),
				})
			},
			wantURL:  "",
			wantOpen: false,
		},
		{
			name: "gh binary missing: Runner error surfaces as err",
			// No canned response: FakeRunner.Run returns a Runner error for
			// any unmatched request, mirroring a missing-binary start failure.
			setup:   func(r *exec.FakeRunner) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := exec.NewFakeRunner()
			tt.setup(runner)
			c := github.New(runner)

			url, open, err := c.PRForBranch(context.Background(), branch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PRForBranch: expected an error, got url=%q open=%v", url, open)
				}
				return
			}
			if err != nil {
				t.Fatalf("PRForBranch: unexpected error: %v", err)
			}
			if url != tt.wantURL {
				t.Errorf("PRForBranch() url = %q, want %q", url, tt.wantURL)
			}
			if open != tt.wantOpen {
				t.Errorf("PRForBranch() open = %v, want %v", open, tt.wantOpen)
			}
		})
	}
}
