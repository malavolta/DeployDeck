package github_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/github"
)

const gateTestPRURL = "https://github.com/org/repo/pull/42"

// --- PRReviews (tasks 4.3-4.4) ----------------------------------------------

// TestClient_PRReviews_MapsLatestReviews is task 4.3 (RED): PRReviews runs
// ONE `gh pr view <url> --json latestReviews`, asserted as an exact discrete
// arg slice (never shell-joined), and maps each entry's author.login/state
// into a Review.
func TestClient_PRReviews_MapsLatestReviews(t *testing.T) {
	runner := exec.NewFakeRunner()
	wantArgs := []string{"pr", "view", gateTestPRURL, "--json", "latestReviews"}
	runner.When("gh", wantArgs, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"latestReviews":[
			{"author":{"login":"alice"},"state":"APPROVED"},
			{"author":{"login":"bob"},"state":"CHANGES_REQUESTED"}
		]}`),
	})

	c := github.New(runner)
	reviews, err := c.PRReviews(context.Background(), gateTestPRURL)
	if err != nil {
		t.Fatalf("PRReviews: unexpected error: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("PRReviews() = %+v, want 2 entries", reviews)
	}
	if reviews[0].Login != "alice" || reviews[0].State != "APPROVED" {
		t.Fatalf("PRReviews()[0] = %+v, want {Login: alice, State: APPROVED}", reviews[0])
	}
	if reviews[1].Login != "bob" || reviews[1].State != "CHANGES_REQUESTED" {
		t.Fatalf("PRReviews()[1] = %+v, want {Login: bob, State: CHANGES_REQUESTED}", reviews[1])
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

// TestClient_PRReviews_Degrades is task 4.3 (RED)'s failure-path companion:
// a Runner error, a non-zero exit, or malformed JSON all return an error,
// never a silently empty/zero result.
func TestClient_PRReviews_Degrades(t *testing.T) {
	t.Run("runner error", func(t *testing.T) {
		runner := exec.NewFakeRunner() // no canned response
		c := github.New(runner)
		if _, err := c.PRReviews(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("PRReviews: expected an error when gh cannot be run at all")
		}
	})

	t.Run("non-zero exit", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		runner.When("gh", []string{"pr", "view", gateTestPRURL, "--json", "latestReviews"}, exec.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
		c := github.New(runner)
		if _, err := c.PRReviews(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("PRReviews: expected an error on a non-zero exit")
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		runner.When("gh", []string{"pr", "view", gateTestPRURL, "--json", "latestReviews"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("not json")})
		c := github.New(runner)
		if _, err := c.PRReviews(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("PRReviews: expected an error on malformed JSON")
		}
	})
}

// --- UnresolvedThreadCount (tasks 4.5-4.6) ----------------------------------

// TestClient_UnresolvedThreadCount_CountsUnresolved is task 4.5 (RED):
// UnresolvedThreadCount runs ONE `gh api graphql` call with discrete,
// never-shell-joined args, and counts the reviewThreads nodes whose
// isResolved is false.
func TestClient_UnresolvedThreadCount_CountsUnresolved(t *testing.T) {
	runner := exec.NewFakeRunner()
	c := github.New(runner)

	// The exact query text is an implementation detail; canning by Name only
	// (via a custom matcher) would be nicer, but FakeRunner matches on the
	// full arg slice — so this test first asserts the STABLE parts of the
	// call (api/graphql/-f owner/-f repo/-F number, all discrete elements)
	// via runner.Calls after driving the call through a wildcard-tolerant
	// canned response keyed on the runner's own recorded call.
	primeUnresolvedThreadsResponse(t, runner, "org", "repo", 42, []bool{false, true, false})

	count, err := c.UnresolvedThreadCount(context.Background(), gateTestPRURL)
	if err != nil {
		t.Fatalf("UnresolvedThreadCount: unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("UnresolvedThreadCount() = %d, want 2", count)
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(runner.Calls))
	}
	args := runner.Calls[0].Args
	if len(args) < 2 || args[0] != "api" || args[1] != "graphql" {
		t.Fatalf("expected the FIRST two args to be [api graphql], got %v", args)
	}
	// owner/repo MUST use -f (raw string, literal), never -F (typed): -F
	// interprets a leading "@" as file-read and ":owner" as a placeholder
	// reference — a repo or org literally named that way would be silently
	// mishandled by gh (remediation-pass risk SUGGESTION). -F is still
	// correct (and required) for number, which IS meant to be Int-typed.
	if !containsFlagValue(args, "-f", "owner=org") {
		t.Fatalf("expected a discrete -f owner=org arg (raw string, not -F), got %v", args)
	}
	if !containsFlagValue(args, "-f", "repo=repo") {
		t.Fatalf("expected a discrete -f repo=repo arg (raw string, not -F), got %v", args)
	}
	if !containsFlagValue(args, "-F", "number=42") {
		t.Fatalf("expected a discrete -F number=42 arg (Int-typed), got %v", args)
	}
}

// TestClient_UnresolvedThreadCount_AllResolvedIsZero triangulates the
// counting logic with a companion, all-resolved fixture: the count must be
// exactly 0, not merely "not 2" (deploy-gate spec: "All threads resolved
// passes the condition").
func TestClient_UnresolvedThreadCount_AllResolvedIsZero(t *testing.T) {
	runner := exec.NewFakeRunner()
	c := github.New(runner)
	primeUnresolvedThreadsResponse(t, runner, "org", "repo", 42, []bool{true, true})

	count, err := c.UnresolvedThreadCount(context.Background(), gateTestPRURL)
	if err != nil {
		t.Fatalf("UnresolvedThreadCount: unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("UnresolvedThreadCount() = %d, want 0", count)
	}
}

// TestClient_UnresolvedThreadCount_Degrades is task 4.5 (RED): a Runner
// error, a non-zero exit, a GraphQL `errors` envelope, or malformed JSON all
// return an error — thread-resolution status "cannot be determined" fails
// closed (deploy-gate spec: "Thread-resolution status unavailable fails
// closed"), never treated as zero/unknown-but-passing.
func TestClient_UnresolvedThreadCount_Degrades(t *testing.T) {
	t.Run("runner error", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		c := github.New(runner)
		if _, err := c.UnresolvedThreadCount(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("UnresolvedThreadCount: expected an error when gh cannot be run at all")
		}
	})

	t.Run("non-zero exit (auth failure)", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		primeUnresolvedThreadsCall(t, runner, "org", "repo", 42, exec.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
		c := github.New(runner)
		if _, err := c.UnresolvedThreadCount(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("UnresolvedThreadCount: expected an error on a non-zero exit")
		}
	})

	t.Run("graphql errors envelope", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		primeUnresolvedThreadsCall(t, runner, "org", "repo", 42, exec.CommandResult{
			ExitCode: 0,
			Stdout:   []byte(`{"data":null,"errors":[{"message":"Could not resolve to a PullRequest"}]}`),
		})
		c := github.New(runner)
		if _, err := c.UnresolvedThreadCount(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("UnresolvedThreadCount: expected an error on a GraphQL errors envelope")
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		primeUnresolvedThreadsCall(t, runner, "org", "repo", 42, exec.CommandResult{ExitCode: 0, Stdout: []byte("not json")})
		c := github.New(runner)
		if _, err := c.UnresolvedThreadCount(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("UnresolvedThreadCount: expected an error on malformed JSON")
		}
	})

	t.Run("truncated result (hasNextPage=true) fails closed", func(t *testing.T) {
		// A PR with more than 100 unresolved-eligible review threads truncates
		// the first page: returning the partial count would let the gate
		// fail-OPEN on exactly this PR (risk review W1). hasNextPage:true must
		// itself be treated as undeterminable and fail closed, mirroring the
		// other Degrades cases in this table.
		runner := exec.NewFakeRunner()
		primeUnresolvedThreadsCall(t, runner, "org", "repo", 42, exec.CommandResult{
			ExitCode: 0,
			Stdout:   []byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"isResolved":true}],"pageInfo":{"hasNextPage":true}}}}}}`),
		})
		c := github.New(runner)
		if _, err := c.UnresolvedThreadCount(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("UnresolvedThreadCount: expected an error when the review-threads page is truncated (hasNextPage=true)")
		}
	})

	t.Run("unparseable PR URL never calls gh", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		c := github.New(runner)
		if _, err := c.UnresolvedThreadCount(context.Background(), "not-a-pr-url"); err == nil {
			t.Fatal("UnresolvedThreadCount: expected an error for an unparseable URL")
		}
		if len(runner.Calls) != 0 {
			t.Fatalf("an unparseable URL must never call gh at all, calls: %v", runner.Calls)
		}
	})
}

// primeUnresolvedThreadsResponse registers a canned success response for
// UnresolvedThreadCount's graphql call, keyed on the runner's OWN query
// text so this test file never hardcodes the client's internal query
// string. It works by first invoking a zero-behavior probe call sequence:
// since FakeRunner matches on the exact arg slice, we instead prime by args
// captured from a first (expected-to-fail) call, then re-register the exact
// same args with the desired canned result.
func primeUnresolvedThreadsResponse(t *testing.T, runner *exec.FakeRunner, owner, repo string, number int, resolved []bool) {
	t.Helper()
	primeUnresolvedThreadsCall(t, runner, owner, repo, number, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(buildReviewThreadsJSON(resolved)),
	})
}

// primeUnresolvedThreadsCall discovers the EXACT arg slice
// UnresolvedThreadCount sends (by making a throwaway probe call against an
// empty FakeRunner and capturing runner.Calls[0].Args), then re-registers a
// FRESH FakeRunner response for that exact slice with the given
// CommandResult — so this test file exercises the real client's argument
// composition without hardcoding its private query text.
func primeUnresolvedThreadsCall(t *testing.T, runner *exec.FakeRunner, owner, repo string, number int, result exec.CommandResult) {
	t.Helper()
	probe := exec.NewFakeRunner()
	probeClient := github.New(probe)
	_, _ = probeClient.UnresolvedThreadCount(context.Background(), gateTestPRURL)
	if len(probe.Calls) != 1 {
		t.Fatalf("expected the probe to make exactly 1 call, got %d", len(probe.Calls))
	}
	runner.When(probe.Calls[0].Name, probe.Calls[0].Args, result)
}

func buildReviewThreadsJSON(resolved []bool) string {
	var b strings.Builder
	b.WriteString(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[`)
	for i, r := range resolved {
		if i > 0 {
			b.WriteString(",")
		}
		if r {
			b.WriteString(`{"isResolved":true}`)
		} else {
			b.WriteString(`{"isResolved":false}`)
		}
	}
	b.WriteString(`]}}}}}`)
	return b.String()
}

func containsFlagValue(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// --- PRComments (tasks 4.7-4.8) ---------------------------------------------

// TestClient_PRComments_MapsComments is task 4.7 (RED): PRComments runs ONE
// `gh pr view <url> --json comments`, asserted as an exact discrete arg
// slice, and maps each entry's author.login/body into a Comment.
func TestClient_PRComments_MapsComments(t *testing.T) {
	runner := exec.NewFakeRunner()
	wantArgs := []string{"pr", "view", gateTestPRURL, "--json", "comments"}
	runner.When("gh", wantArgs, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"comments":[
			{"author":{"login":"deploydeck-bot"},"body":"<!-- deploydeck-validation: job:0Af1 run:run-1 -->\nValidation passed."},
			{"author":{"login":"alice"},"body":"LGTM"}
		]}`),
	})

	c := github.New(runner)
	comments, err := c.PRComments(context.Background(), gateTestPRURL)
	if err != nil {
		t.Fatalf("PRComments: unexpected error: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("PRComments() = %+v, want 2 entries", comments)
	}
	if comments[0].Login != "deploydeck-bot" || !strings.Contains(comments[0].Body, "deploydeck-validation") {
		t.Fatalf("PRComments()[0] = %+v, want a deploydeck-bot marker comment", comments[0])
	}
	if comments[1].Login != "alice" || comments[1].Body != "LGTM" {
		t.Fatalf("PRComments()[1] = %+v, want {Login: alice, Body: LGTM}", comments[1])
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(runner.Calls))
	}
	gotArgs := runner.Calls[0].Args
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("arg %d = %q, want %q (full: %v)", i, gotArgs[i], wantArgs[i], gotArgs)
		}
	}
}

// TestClient_PRComments_Degrades is task 4.7 (RED)'s failure-path companion.
func TestClient_PRComments_Degrades(t *testing.T) {
	t.Run("runner error", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		c := github.New(runner)
		if _, err := c.PRComments(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("PRComments: expected an error when gh cannot be run at all")
		}
	})

	t.Run("non-zero exit", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		runner.When("gh", []string{"pr", "view", gateTestPRURL, "--json", "comments"}, exec.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
		c := github.New(runner)
		if _, err := c.PRComments(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("PRComments: expected an error on a non-zero exit")
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		runner.When("gh", []string{"pr", "view", gateTestPRURL, "--json", "comments"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("not json")})
		c := github.New(runner)
		if _, err := c.PRComments(context.Background(), gateTestPRURL); err == nil {
			t.Fatal("PRComments: expected an error on malformed JSON")
		}
	})
}

// --- PostComment (tasks 4.9-4.10) -------------------------------------------

// TestClient_PostComment_Success is task 4.9 (RED): PostComment runs
// `gh pr comment <url> --body <body>` as discrete, never-shell-joined args
// (a multi-line body proves this — a shell would mangle embedded newlines)
// and returns the raw combined output on success.
func TestClient_PostComment_Success(t *testing.T) {
	runner := exec.NewFakeRunner()
	body := "**Validación de DeployDeck**\n\n- Job: 0Af1\n- Cobertura: 92%"
	wantArgs := []string{"pr", "comment", gateTestPRURL, "--body", body}
	runner.When("gh", wantArgs, exec.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/42#issuecomment-1")})

	c := github.New(runner)
	raw, err := c.PostComment(context.Background(), gateTestPRURL, body)
	if err != nil {
		t.Fatalf("PostComment: unexpected error: %v", err)
	}
	if !strings.Contains(raw, "issuecomment") {
		t.Fatalf("PostComment() raw = %q, want it to preserve the command output", raw)
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

// TestClient_PostComment_Degrades is task 4.9 (RED)'s failure-path
// companion: a Runner error or a non-zero exit both return an error while
// still preserving Raw on the non-zero-exit path.
func TestClient_PostComment_Degrades(t *testing.T) {
	t.Run("runner error", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		c := github.New(runner)
		_, err := c.PostComment(context.Background(), gateTestPRURL, "body")
		if err == nil {
			t.Fatal("PostComment: expected an error when gh cannot be run at all")
		}
	})

	t.Run("non-zero exit keeps raw", func(t *testing.T) {
		runner := exec.NewFakeRunner()
		runner.When("gh", []string{"pr", "comment", gateTestPRURL, "--body", "body"}, exec.CommandResult{ExitCode: 1, Stderr: []byte("pull request comment failed")})
		c := github.New(runner)
		raw, err := c.PostComment(context.Background(), gateTestPRURL, "body")
		if err == nil {
			t.Fatal("PostComment: expected an error on a non-zero exit")
		}
		if !strings.Contains(raw, "pull request comment failed") {
			t.Fatalf("expected raw to preserve the failure output, got %q", raw)
		}
	})
}
