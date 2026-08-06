package app

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/gate"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

const deployGatePRURL = "https://github.com/org/repo/pull/42"

func ptrBool(b bool) *bool { return &b }

// --- 6.1: resolvePRURL -------------------------------------------------

// TestResolvePRURL is task 6.1 (RED): rec.PRUrl is used when present;
// empty PRUrl falls back to PRForBranch(RenderBranchName(BranchFormat,
// Ticket, Target)); neither resolves -> fail-closed (deploy-gate spec:
// "Fail-Closed PR Resolution").
func TestResolvePRURL(t *testing.T) {
	format := config.DefaultBranchFormat

	t.Run("rec.PRUrl used when present, gh never consulted", func(t *testing.T) {
		fr := execpkg.NewFakeRunner() // no canned response: any gh call would error
		gh := github.New(fr)
		rec := runs.Record{PRUrl: deployGatePRURL, Ticket: "PROJ-1", Target: "UAT"}

		url, ok := resolvePRURL(context.Background(), gh, rec, format)
		if !ok {
			t.Fatal("resolvePRURL() ok = false, want true")
		}
		if url != deployGatePRURL {
			t.Fatalf("resolvePRURL() = %q, want %q", url, deployGatePRURL)
		}
		if len(fr.Calls) != 0 {
			t.Fatalf("gh must never be consulted when rec.PRUrl is already set, calls: %v", fr.Calls)
		}
	})

	t.Run("empty PRUrl falls back to PRForBranch(RenderBranchName(...))", func(t *testing.T) {
		branch := "deploy/PROJ-1-to-UAT"
		fr := execpkg.NewFakeRunner()
		fr.When("gh", []string{"pr", "view", branch, "--json", "url,state"}, execpkg.CommandResult{
			ExitCode: 0,
			Stdout:   []byte(`{"url":"` + deployGatePRURL + `","state":"OPEN"}`),
		})
		gh := github.New(fr)
		rec := runs.Record{PRUrl: "", Ticket: "PROJ-1", Target: "UAT"}

		url, ok := resolvePRURL(context.Background(), gh, rec, format)
		if !ok {
			t.Fatal("resolvePRURL() ok = false, want true")
		}
		if url != deployGatePRURL {
			t.Fatalf("resolvePRURL() = %q, want %q", url, deployGatePRURL)
		}
	})

	t.Run("neither PRUrl nor PRForBranch resolve: fail-closed", func(t *testing.T) {
		branch := "deploy/PROJ-1-to-UAT"
		fr := execpkg.NewFakeRunner()
		fr.When("gh", []string{"pr", "view", branch, "--json", "url,state"}, execpkg.CommandResult{
			ExitCode: 1, Stderr: []byte("no pull requests found"),
		})
		gh := github.New(fr)
		rec := runs.Record{PRUrl: "", Ticket: "PROJ-1", Target: "UAT"}

		_, ok := resolvePRURL(context.Background(), gh, rec, format)
		if ok {
			t.Fatal("resolvePRURL() ok = true, want false (fail-closed: no PR resolvable)")
		}
	})
}

// --- 6.3: gateCheckCmd ---------------------------------------------------

// primeAppUnresolvedThreads discovers the exact arg slice UnresolvedThreadCount
// sends (via a throwaway probe call, mirroring internal/github's own test
// technique) and re-registers it on fr with the given canned result — so this
// test file never hardcodes the client's private graphql query text.
func primeAppUnresolvedThreads(t *testing.T, fr *execpkg.FakeRunner, result execpkg.CommandResult) {
	t.Helper()
	probe := execpkg.NewFakeRunner()
	_, _ = github.New(probe).UnresolvedThreadCount(context.Background(), deployGatePRURL)
	if len(probe.Calls) != 1 {
		t.Fatalf("expected the probe to make exactly 1 call, got %d", len(probe.Calls))
	}
	fr.When(probe.Calls[0].Name, probe.Calls[0].Args, result)
}

// gateCheckModel builds a Model parked on a seeded, gate-enabled run ready
// for gateCheckCmd — rec.PRUrl is pre-set so resolvePRURL never needs the
// PRForBranch fallback, isolating this test from that already-proven path.
func gateCheckModel(t *testing.T, gateCfg config.GateConfig) (Model, *execpkg.FakeRunner) {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	cfg := config.Config{
		Branches:     map[string]string{"uat": "UAT"},
		BranchFormat: config.DefaultBranchFormat,
		Gates:        map[string]config.GateConfig{"UAT": gateCfg},
	}
	deps := Deps{Dir: "/repo", Config: cfg, GH: github.New(fr)}
	m := New(deps)
	m.runs = []runs.Record{{RunID: "run-1", Ticket: "PROJ-1", Target: "UAT", PRUrl: deployGatePRURL}}
	m.runsCursor = 0
	return m, fr
}

// TestGateCheckCmd_MapsEachGHErrorIndependently is task 6.3 (RED):
// PRReviews/UnresolvedThreadCount/PRComments/provenance-fetch (PRDetails) gh
// errors are mapped INDEPENDENTLY into the matching Facts.*Err — a failure
// in ONE read never short-circuits or blanket-fails the others, and
// gateCheckCmd always returns a Result (never panics).
func TestGateCheckCmd_MapsEachGHErrorIndependently(t *testing.T) {
	gateCfg := config.GateConfig{
		Enabled:                  true,
		Approvers:                []string{"alice"},
		RequireResolvedThreads:   ptrBool(true),
		RequireValidationComment: ptrBool(true),
		RequireSignature:         ptrBool(true),
	}

	tests := []struct {
		name           string
		breakReviews   bool
		breakThreads   bool
		breakComments  bool
		breakDetails   bool
		wantFailedName string
	}{
		{name: "PRReviews failure fails approvals", breakReviews: true, wantFailedName: "approvals"},
		{name: "UnresolvedThreadCount failure fails threads", breakThreads: true, wantFailedName: "threads"},
		{name: "PRComments failure fails validation-comment", breakComments: true, wantFailedName: "validation-comment"},
		{name: "PRDetails failure fails signature", breakDetails: true, wantFailedName: "signature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fr := gateCheckModel(t, gateCfg)

			if tt.breakReviews {
				fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "latestReviews"}, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
			} else {
				fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "latestReviews"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"latestReviews":[{"author":{"login":"alice"},"state":"APPROVED"}]}`)})
			}

			if tt.breakThreads {
				primeAppUnresolvedThreads(t, fr, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
			} else {
				primeAppUnresolvedThreads(t, fr, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[]}}}}}`)})
			}

			if tt.breakComments {
				fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "comments"}, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
			} else {
				fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "comments"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"comments":[{"author":{"login":"bot"},"body":"` + gate.MarkerPrefix + ` job:0Af1 run:run-1 -->"}]}`)})
			}

			if tt.breakDetails {
				fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "headRefName,body"}, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
			} else {
				fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "headRefName,body"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"headRefName":"deploy/PROJ-1-to-UAT","body":"no marker here"}`)})
			}

			cmd := m.gateCheckCmd()
			if cmd == nil {
				t.Fatal("gateCheckCmd() returned a nil tea.Cmd")
			}
			msg := run(t, cmd)
			done, ok := msg.(gateCheckDoneMsg)
			if !ok {
				t.Fatalf("expected a gateCheckDoneMsg, got %T", msg)
			}

			if done.result.Passed {
				t.Fatal("expected Result.Passed=false: one condition failed via a gh error")
			}

			var found *gate.Condition
			for i := range done.result.Conditions {
				if done.result.Conditions[i].Name == tt.wantFailedName {
					found = &done.result.Conditions[i]
				}
			}
			if found == nil {
				t.Fatalf("expected a %q condition in the result, got %+v", tt.wantFailedName, done.result.Conditions)
			}
			if found.Passed {
				t.Fatalf("expected condition %q to fail (its gh read errored), got %+v", tt.wantFailedName, found)
			}

			// Every one of the 4 gh reads must have been ATTEMPTED regardless
			// of which one failed — proving independence, not a short-circuit.
			if len(fr.Calls) != 4 {
				t.Fatalf("expected all 4 gh reads to be attempted independently, got %d calls: %v", len(fr.Calls), fr.Calls)
			}
		})
	}
}

// TestGateCheckCmd_NoResolvablePR_SkipsAllFourReads is task 6.3 (RED): when
// resolvePRURL cannot resolve a PR at all, gateCheckCmd sets
// Facts.PRResolved=false (a single failed pr-resolution condition) and
// skips the 4 gh reads entirely — only the PRForBranch fallback attempt
// itself is ever called.
func TestGateCheckCmd_NoResolvablePR_SkipsAllFourReads(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	branch := "deploy/PROJ-1-to-UAT"
	fr.When("gh", []string{"pr", "view", branch, "--json", "url,state"}, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("no pull requests found")})

	cfg := config.Config{
		Branches:     map[string]string{"uat": "UAT"},
		BranchFormat: config.DefaultBranchFormat,
		Gates:        map[string]config.GateConfig{"UAT": {Enabled: true, Approvers: []string{"alice"}}},
	}
	deps := Deps{Dir: "/repo", Config: cfg, GH: github.New(fr)}
	m := New(deps)
	m.runs = []runs.Record{{RunID: "run-1", Ticket: "PROJ-1", Target: "UAT", PRUrl: ""}}
	m.runsCursor = 0

	msg := run(t, m.gateCheckCmd())
	done, ok := msg.(gateCheckDoneMsg)
	if !ok {
		t.Fatalf("expected a gateCheckDoneMsg, got %T", msg)
	}
	if done.result.Passed {
		t.Fatal("expected Result.Passed=false when no PR resolves")
	}
	if len(done.result.Conditions) != 1 || done.result.Conditions[0].Name != "pr-resolution" {
		t.Fatalf("expected exactly 1 pr-resolution condition, got %+v", done.result.Conditions)
	}

	// Only the PRForBranch fallback attempt itself — none of the 4 gh reads.
	if len(fr.Calls) != 1 {
		t.Fatalf("expected only the PRForBranch attempt (1 call), got %d: %v", len(fr.Calls), fr.Calls)
	}
}

// ctxCapturingRunner wraps a Runner and records the FIRST context.Context it
// receives, so a test can assert on that context's Deadline() without
// depending on FakeRunner's own ctx-blind Run signature (FakeRunner.Run
// deliberately ignores ctx — it can only prove WHAT was sent, not the ctx it
// was sent under).
type ctxCapturingRunner struct {
	inner    execpkg.Runner
	firstCtx context.Context
}

func (r *ctxCapturingRunner) Run(ctx context.Context, req execpkg.CommandRequest) (execpkg.CommandResult, error) {
	if r.firstCtx == nil {
		r.firstCtx = ctx
	}
	return r.inner.Run(ctx, req)
}

// TestGateCheckCmd_BoundsGhCallsWithATimeout is the WARNING remediation-pass
// fix (resilience review): gateCheckCmd ran its (up to 4, sequential) gh
// reads under context.Background() — no timeout at all — so a single hung gh
// invocation could stall gateCheckDoneMsg forever, leaving the confirm
// screen looking frozen (design.md's "gate re-checked at point of no
// return"). The gh reads must run under a bounded context.WithTimeout, so a
// hung gh fails closed instead of hanging indefinitely.
func TestGateCheckCmd_BoundsGhCallsWithATimeout(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	capturing := &ctxCapturingRunner{inner: fr}
	gh := github.New(capturing)

	cfg := config.Config{
		Branches:     map[string]string{"uat": "UAT"},
		BranchFormat: config.DefaultBranchFormat,
		Gates:        map[string]config.GateConfig{"UAT": {Enabled: true, Approvers: []string{"alice"}}},
	}
	deps := Deps{Dir: "/repo", Config: cfg, GH: gh}
	m := New(deps)
	m.runs = []runs.Record{{RunID: "run-1", Ticket: "PROJ-1", Target: "UAT", PRUrl: deployGatePRURL}}
	m.runsCursor = 0

	// No canned responses at all: every gh read errors (Runner "no canned
	// response" error) — gateCheckCmd must still return a Result rather than
	// panic, and every attempted call must have gone through capturingRunner.
	run(t, m.gateCheckCmd())

	if capturing.firstCtx == nil {
		t.Fatal("expected at least one gh call to have reached the Runner")
	}
	if _, ok := capturing.firstCtx.Deadline(); !ok {
		t.Fatal("expected the ctx passed to gateCheckCmd's gh calls to carry a deadline (bounded context.WithTimeout), got one with no deadline")
	}
}

// --- 6.17: postValidationCommentCmd skip-if-present -------------------------

// postCommentModel builds a Model parked past a terminal-success validation
// (m.prURL/m.jobID/m.runID/m.report already set), ready for
// postValidationCommentCmd.
func postCommentModel(fr *execpkg.FakeRunner) Model {
	deps := Deps{Dir: "/repo", Config: config.Config{}, GH: github.New(fr)}
	m := New(deps)
	m.prURL = deployGatePRURL
	m.jobID = "0Af1"
	m.runID = "run-1"
	m.report = salesforce.DeployReport{
		Status:                "Succeeded",
		NumberComponentErrors: 0,
		NumberTestErrors:      0,
	}
	return m
}

// TestPostValidationCommentCmd_SkipsWhenMarkerAlreadyPresent is task 6.17
// (RED): the PR already carries THIS job's marker comment -> no `gh pr
// comment` call is made (no duplicate), per deploy-gate spec's
// "Re-validation does not duplicate the comment".
func TestPostValidationCommentCmd_SkipsWhenMarkerAlreadyPresent(t *testing.T) {
	_, existingBody := gate.ValidationComment("0Af1", "run-1", 0, 0, 100, true)

	fr := execpkg.NewFakeRunner()
	fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "comments"}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"comments":[{"author":{"login":"deploydeck-bot"},"body":` + jsonQuote(existingBody) + `}]}`),
	})
	m := postCommentModel(fr)

	msg := run(t, m.postValidationCommentCmd())
	if _, ok := msg.(postCommentDoneMsg); !ok {
		t.Fatalf("expected a postCommentDoneMsg, got %T", msg)
	}

	for _, c := range fr.Calls {
		if len(c.Args) > 0 && c.Args[0] == "pr" && len(c.Args) > 1 && c.Args[1] == "comment" {
			t.Fatalf("expected NO gh pr comment call when the marker is already present, got calls: %v", fr.Calls)
		}
	}
}

// TestPostValidationCommentCmd_PostsWhenNoMarkerPresent is the triangulation
// companion: no existing marker comment -> a comment IS posted, carrying
// the marker + report scalars.
func TestPostValidationCommentCmd_PostsWhenNoMarkerPresent(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "comments"}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"comments":[{"author":{"login":"alice"},"body":"LGTM"}]}`),
	})
	m := postCommentModel(fr)

	// Register a matching `gh pr comment` response regardless of the exact
	// body text (probe-then-reprime), so this test does not need to
	// hand-compute ValidationComment's exact byte-for-byte output.
	probe := execpkg.NewFakeRunner()
	_, body := gate.ValidationComment("0Af1", "run-1", 0, 0, 0, false)
	_, _ = github.New(probe).PostComment(context.Background(), deployGatePRURL, body)
	fr.When(probe.Calls[0].Name, probe.Calls[0].Args, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("posted")})

	run(t, m.postValidationCommentCmd())

	posted := false
	for _, c := range fr.Calls {
		if len(c.Args) > 1 && c.Args[0] == "pr" && c.Args[1] == "comment" {
			posted = true
		}
	}
	if !posted {
		t.Fatalf("expected a gh pr comment call when no marker is present, got calls: %v", fr.Calls)
	}
}

// jsonQuote minimally escapes s for embedding as a JSON string literal in a
// hand-built fixture (only \\ and " and newlines matter for this file's
// fixtures).
func jsonQuote(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for _, r := range s {
		switch r {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, string(r)...)
		}
	}
	out = append(out, '"')
	return string(out)
}
