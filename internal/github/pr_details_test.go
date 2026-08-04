package github_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/github"
)

// TestClient_PRDetails_Success is task 2.3 (RED): PRDetails runs
// `gh pr view <url> --json headRefName,body` and parses headBranch+body from
// its JSON output.
func TestClient_PRDetails_Success(t *testing.T) {
	url := "https://github.com/org/repo/pull/42"
	runner := exec.NewFakeRunner()
	runner.When("gh", []string{"pr", "view", url, "--json", "headRefName,body"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"headRefName":"deploy/PROJ-1-to-UAT","body":"Some description\n\n<!-- deploydeck: v1 run:run-1 sig:dev -->"}`),
	})

	c := github.New(runner)
	headBranch, body, err := c.PRDetails(context.Background(), url)
	if err != nil {
		t.Fatalf("PRDetails: unexpected error: %v", err)
	}
	if headBranch != "deploy/PROJ-1-to-UAT" {
		t.Fatalf("PRDetails() headBranch = %q, want %q", headBranch, "deploy/PROJ-1-to-UAT")
	}
	if body == "" {
		t.Fatal("PRDetails() body should not be empty")
	}
}

// TestClient_PRDetails_GHMissing_ReturnsErr is task 2.3 (RED): a Runner
// error (gh binary missing/cannot start) returns an error, never a panic.
func TestClient_PRDetails_GHMissing_ReturnsErr(t *testing.T) {
	runner := exec.NewFakeRunner() // no canned response: Runner error

	c := github.New(runner)
	_, _, err := c.PRDetails(context.Background(), "https://github.com/org/repo/pull/42")
	if err == nil {
		t.Fatal("PRDetails: expected an error when gh cannot be run at all")
	}
}

// TestClient_PRDetails_NonZeroExit_ReturnsErr is task 2.3 (RED): unlike
// PRForBranch's non-zero-exit-as-data ("no PR found" degrades to a normal
// create fallthrough), an unfetchable PR here is a genuine degrade — verify
// has no create-fallthrough analog — so a non-zero exit returns an error.
func TestClient_PRDetails_NonZeroExit_ReturnsErr(t *testing.T) {
	runner := exec.NewFakeRunner()
	url := "https://github.com/org/repo/pull/999"
	runner.When("gh", []string{"pr", "view", url, "--json", "headRefName,body"}, exec.CommandResult{
		ExitCode: 1,
		Stderr:   []byte("no pull requests found"),
	})

	c := github.New(runner)
	_, _, err := c.PRDetails(context.Background(), url)
	if err == nil {
		t.Fatal("PRDetails: expected an error on a non-zero exit (unlike PRForBranch, verify has no create-fallthrough)")
	}
}

// TestClient_PRDetails_MalformedJSON_ReturnsErr is task 2.3 (RED): a
// zero-exit but unparseable stdout returns an error rather than a
// zero-valued, silently-wrong result.
func TestClient_PRDetails_MalformedJSON_ReturnsErr(t *testing.T) {
	runner := exec.NewFakeRunner()
	url := "https://github.com/org/repo/pull/42"
	runner.When("gh", []string{"pr", "view", url, "--json", "headRefName,body"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("not json at all"),
	})

	c := github.New(runner)
	_, _, err := c.PRDetails(context.Background(), url)
	if err == nil {
		t.Fatal("PRDetails: expected an error on malformed JSON output")
	}
}
