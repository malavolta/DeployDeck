package app

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/gate"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// vcTimingBranch is the rendered promotion-branch name for vcTimingModel's
// fixed Ticket("PROJ-1")/TargetBranch("UAT") pair under
// config.DefaultBranchFormat — mirrors deploy_gate_test.go's own hardcoded
// "deploy/PROJ-1-to-UAT" (TestResolvePRURL, gateCheckModel/NoResolvablePR).
const vcTimingBranch = "deploy/PROJ-1-to-UAT"

// vcTimingModel builds a Model parked at StateValidationPolling on a
// gate-enabled, requireValidationComment-on UAT target, ready to drive
// reportDoneMsg/prCreatedMsg through onReportDone/onPrCreated — this change's
// own fixture, mirroring deploy_gate_test.go's gateCheckModel/postCommentModel
// idiom for the (previously untested) onReportDone/onPrCreated seam.
func vcTimingModel(t *testing.T, fr *execpkg.FakeRunner) Model {
	t.Helper()
	deps := Deps{
		Dir: "/repo",
		Config: config.Config{
			BranchFormat: config.DefaultBranchFormat,
			Gates: map[string]config.GateConfig{
				"UAT": {Enabled: true, RequireValidationComment: ptrBool(true)},
			},
		},
		GH: github.New(fr),
	}
	m := New(deps)
	m.plan.TargetBranch = "UAT"
	m.plan.Ticket = "PROJ-1"
	m.runID = "run-1"
	m.jobID = "0Af1"
	// onReportDone's stale-message guard requires StateValidationPolling —
	// without it, onReportDone would no-op before ever setting m.report or
	// evaluating the trigger, defeating every scenario below.
	m.state = StateValidationPolling
	return m
}

// vcCommentBody is the exact validation-comment body vcTimingModel's fixture
// produces: jobID "0Af1", runID "run-1", zero component/test errors, no
// CodeCoverage data (coverage unknown) — mirrors deploy_gate_test.go's
// postCommentModel report shape and TestPostValidationCommentCmd_
// PostsWhenNoMarkerPresent's own ValidationComment(...) call.
func vcCommentBody() string {
	_, body := gate.ValidationComment("0Af1", "run-1", 0, 0, 0, false)
	return body
}

// primeVcPostComment registers fr's canned `gh pr comment <url> --body
// <body>` success response via the probe-then-reprime technique
// (deploy_gate_test.go's TestPostValidationCommentCmd_PostsWhenNoMarkerPresent),
// so this file never hand-computes gh's exact arg shape.
func primeVcPostComment(t *testing.T, fr *execpkg.FakeRunner, url, body string) {
	t.Helper()
	probe := execpkg.NewFakeRunner()
	_, _ = github.New(probe).PostComment(context.Background(), url, body)
	if len(probe.Calls) != 1 {
		t.Fatalf("expected the probe to make exactly 1 call, got %d", len(probe.Calls))
	}
	fr.When(probe.Calls[0].Name, probe.Calls[0].Args, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("posted")})
}

// countPrComments counts `gh pr comment` calls recorded on calls.
func countPrComments(calls []execpkg.CommandRequest) int {
	n := 0
	for _, c := range calls {
		if len(c.Args) > 1 && c.Args[0] == "pr" && c.Args[1] == "comment" {
			n++
		}
	}
	return n
}

// TestOnReportDone_TerminalSuccess_PRAbsent_PostsNothing is task 1.2 (RED):
// a terminal-success validation on a gate-enabled target, with the run's PR
// not yet resolvable (branch unpushed / no PR yet), posts NO comment at
// validation time — the normal flow's first half (deploy-gate: "Validation
// completes before the PR exists, comment posts at PR creation").
func TestOnReportDone_TerminalSuccess_PRAbsent_PostsNothing(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("gh", []string{"pr", "view", vcTimingBranch, "--json", "url,state"}, execpkg.CommandResult{
		ExitCode: 1, Stderr: []byte("no pull requests found"),
	})
	m := vcTimingModel(t, fr)

	next, cmd := m.onReportDone(reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded"}})
	m = next.(Model)
	if m.report.Status != "Succeeded" {
		t.Fatalf("m.report.Status = %q, want %q", m.report.Status, "Succeeded")
	}
	if cmd == nil {
		t.Fatal("onReportDone() cmd = nil, want the validation-comment cmd (gate enabled, terminal success)")
	}
	run(t, cmd)

	if n := countPrComments(fr.Calls); n != 0 {
		t.Fatalf("expected zero `gh pr comment` calls (PR not yet resolvable), got %d: %v", n, fr.Calls)
	}
}

// TestOnPrCreated_AfterTerminalSuccess_PostsCommentExactlyOnce is task 1.3
// (RED, load-bearing): after a terminal-success validation completed this
// session with the PR not yet resolvable (mirrors 1.2's setup), the run's PR
// is subsequently created — onPrCreated's success path must (re)trigger the
// validation-comment post/upsert, reaching the PR that did not exist at
// validation time (deploy-gate: "posts at PR creation" + "exactly once
// across both trigger points"; validation-progress: "PR creation
// (re)triggers the comment"). Fails today: onPrCreated returns (m, nil)
// unconditionally on its success path, so cmd is nil here.
func TestOnPrCreated_AfterTerminalSuccess_PostsCommentExactlyOnce(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("gh", []string{"pr", "view", vcTimingBranch, "--json", "url,state"}, execpkg.CommandResult{
		ExitCode: 1, Stderr: []byte("no pull requests found"),
	})
	m := vcTimingModel(t, fr)
	next, cmd := m.onReportDone(reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded"}})
	m = next.(Model)
	run(t, cmd) // validation-time attempt: no PR yet, no post (pinned by 1.2)

	fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "comments"}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"comments":[{"author":{"login":"alice"},"body":"LGTM"}]}`),
	})
	primeVcPostComment(t, fr, deployGatePRURL, vcCommentBody())

	next, cmd = m.onPrCreated(prCreatedMsg{url: deployGatePRURL})
	m = next.(Model)
	if m.prURL != deployGatePRURL {
		t.Fatalf("m.prURL = %q, want %q", m.prURL, deployGatePRURL)
	}
	if cmd == nil {
		t.Fatal("onPrCreated() cmd = nil, want the validation-comment cmd: a terminal-success validation already completed this session with no PR yet, and the PR now exists")
	}
	run(t, cmd)

	if n := countPrComments(fr.Calls); n != 1 {
		t.Fatalf("expected exactly ONE `gh pr comment` call after PR creation, got %d: %v", n, fr.Calls)
	}
}

// TestOnPrCreated_MarkerAlreadyPresent_NoDoublePost is task 1.4 (RED): same
// setup as 1.3, but the PR already carries this job's marker comment —
// onPrCreated must NOT post a second comment (deploy-gate: "Re-validation
// does not duplicate the comment"; the exactly-once dedup applies across
// BOTH trigger points, not just within one).
func TestOnPrCreated_MarkerAlreadyPresent_NoDoublePost(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("gh", []string{"pr", "view", vcTimingBranch, "--json", "url,state"}, execpkg.CommandResult{
		ExitCode: 1, Stderr: []byte("no pull requests found"),
	})
	m := vcTimingModel(t, fr)
	next, cmd := m.onReportDone(reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded"}})
	m = next.(Model)
	run(t, cmd)

	existingBody := vcCommentBody()
	fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "comments"}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"comments":[{"author":{"login":"deploydeck-bot"},"body":` + jsonQuote(existingBody) + `}]}`),
	})

	next, cmd = m.onPrCreated(prCreatedMsg{url: deployGatePRURL})
	m = next.(Model)
	if cmd != nil {
		run(t, cmd)
	}

	if n := countPrComments(fr.Calls); n != 0 {
		t.Fatalf("expected zero `gh pr comment` calls (marker already present), got %d: %v", n, fr.Calls)
	}
}

// TestOnPrCreated_NoTerminalSuccessThisSession_PostsNothing is task 1.5
// (RED): PR creation with no prior terminal-success validation this session
// (report.Status empty/Failed/Canceled) never triggers the comment
// (validation-progress: "PR creation with no terminal-successful validation
// this session SHALL NOT trigger any comment either").
func TestOnPrCreated_NoTerminalSuccessThisSession_PostsNothing(t *testing.T) {
	for _, status := range []string{"", "Failed", "Canceled"} {
		t.Run("status="+status, func(t *testing.T) {
			fr := execpkg.NewFakeRunner()
			m := vcTimingModel(t, fr)
			m.report.Status = status

			_, cmd := m.onPrCreated(prCreatedMsg{url: deployGatePRURL})
			if cmd != nil {
				t.Fatalf("onPrCreated() cmd != nil for report.Status = %q, want nil (no terminal-success this session)", status)
			}
			if len(fr.Calls) != 0 {
				t.Fatalf("expected zero gh calls, got %d: %v", len(fr.Calls), fr.Calls)
			}
		})
	}
}

// TestOnReportDone_ReuseFlow_PRAlreadyOpenAtValidation_PostsOnce is task 1.6
// (RED): the reuse/incremental flow, where the run's PR is already open at
// validation time — onReportDone's existing trigger posts the comment
// exactly once, UNCHANGED by this fix (design: "Reuse flow ... unchanged";
// validation-progress: "successful CheckOnly triggers the comment").
func TestOnReportDone_ReuseFlow_PRAlreadyOpenAtValidation_PostsOnce(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	fr.When("gh", []string{"pr", "view", deployGatePRURL, "--json", "comments"}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"comments":[{"author":{"login":"alice"},"body":"LGTM"}]}`),
	})
	primeVcPostComment(t, fr, deployGatePRURL, vcCommentBody())

	m := vcTimingModel(t, fr)
	m.prURL = deployGatePRURL // PR already open before validation completes

	_, cmd := m.onReportDone(reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded"}})
	if cmd == nil {
		t.Fatal("onReportDone() cmd = nil, want the validation-comment cmd")
	}
	run(t, cmd)

	if n := countPrComments(fr.Calls); n != 1 {
		t.Fatalf("expected exactly ONE `gh pr comment` call, got %d: %v", n, fr.Calls)
	}
}
