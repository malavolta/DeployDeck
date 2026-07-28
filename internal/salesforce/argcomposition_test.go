package salesforce_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/salesforce"
)

// This file is the design.md Threat Matrix "PR / argument composition" proof:
// both new subprocess calls (ListDeployQueue's SOQL query and CancelDeploy's
// jobId) MUST reach the `sf` CLI as DISCRETE slice elements through the
// exec.Runner seam — never shell-joined, never string-interpolated into a
// single command line. FakeRunner records the exact CommandRequest.Args it
// receives, so asserting on Calls proves the values were passed positionally.
//
// The strongest assertion here feeds an ADVERSARIAL value carrying shell
// metacharacters and confirms it survives as ONE untouched arg: if the code
// ever shell-joined the args, such a value would be split or interpreted.

// argAfter returns the element immediately following flag in args, or "" when
// flag is absent or has no following element.
func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// TestThreatMatrix_ListDeployQueue_SOQLIsOneDiscreteArg proves the whole SOQL
// query is a SINGLE slice element following --query, not split across args and
// not concatenated with the flag.
func TestThreatMatrix_ListDeployQueue_SOQLIsOneDiscreteArg(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{
		"data", "query",
		"--target-org", "UAT_SANDBOX",
		"--use-tooling-api",
		"--json",
		"--query", deployQueueSOQL,
	}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{"totalSize":0,"done":true,"records":[]}}`)})

	if _, err := salesforce.New(fr).ListDeployQueue(context.Background(), "UAT_SANDBOX"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fr.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fr.Calls))
	}
	args := fr.Calls[0].Args

	// The SOQL must be exactly one arg, positioned right after --query.
	got := argAfter(args, "--query")
	if got != deployQueueSOQL {
		t.Fatalf("SOQL must be ONE discrete arg after --query; got %q", got)
	}
	// The SOQL must never appear fused with the flag into a single element.
	for _, a := range args {
		if a != deployQueueSOQL && strings.Contains(a, "SELECT") {
			t.Fatalf("SOQL leaked into another arg %q — it must stay a single discrete element", a)
		}
	}
	// The SOQL contains spaces; if the harness ever shell-joined, it would be
	// split into many args. Assert the count matches the discrete-slice form
	// (data, query, --target-org, <alias>, --use-tooling-api, --json, --query,
	// <SOQL> = 8 elements).
	if len(args) != 8 {
		t.Fatalf("expected 8 discrete args (SOQL not split), got %d: %v", len(args), args)
	}
}

// TestThreatMatrix_CancelDeploy_JobIDIsOneDiscreteArg proves the jobId is a
// single discrete arg after --job-id — even when it carries shell
// metacharacters, it is passed through untouched (arg-slice, never a shell).
func TestThreatMatrix_CancelDeploy_JobIDIsOneDiscreteArg(t *testing.T) {
	// An adversarial jobId that WOULD be dangerous under any shell
	// interpolation. It must survive as exactly one literal arg.
	const adversarialJobID = `0Af000; rm -rf / && echo "$(whoami)"`

	fr := exec.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "cancel",
		"--job-id", adversarialJobID,
		"--target-org", "UAT_SANDBOX",
		"--json",
	}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0}`)})

	if _, err := salesforce.New(fr).CancelDeploy(context.Background(), adversarialJobID, "UAT_SANDBOX"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fr.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fr.Calls))
	}
	args := fr.Calls[0].Args

	if got := argAfter(args, "--job-id"); got != adversarialJobID {
		t.Fatalf("jobId must be ONE untouched discrete arg after --job-id; got %q", got)
	}
	// The metacharacter-laden jobId must not have been split, dropped, or fused
	// with the alias: the discrete arg list is exactly the composed form.
	want := []string{"project", "deploy", "cancel", "--job-id", adversarialJobID, "--target-org", "UAT_SANDBOX", "--json"}
	if len(args) != len(want) {
		t.Fatalf("expected %d discrete args (jobId not split/interpolated), got %d: %v", len(want), len(args), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

// TestThreatMatrix_QuickDeploy_JobIDIsOneDiscreteArg proves the jobId is a
// single discrete arg after --job-id — even when it carries shell
// metacharacters, it is passed through untouched (arg-slice, never a shell).
func TestThreatMatrix_QuickDeploy_JobIDIsOneDiscreteArg(t *testing.T) {
	// An adversarial jobId that WOULD be dangerous under any shell
	// interpolation. It must survive as exactly one literal arg.
	const adversarialJobID = `0Af000; rm -rf / && echo "$(whoami)"`

	fr := exec.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "quick",
		"--job-id", adversarialJobID,
		"--target-org", "UAT_SANDBOX",
		"--json",
	}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0}`)})

	if _, err := salesforce.New(fr).QuickDeploy(context.Background(), adversarialJobID, "UAT_SANDBOX"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fr.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fr.Calls))
	}
	args := fr.Calls[0].Args

	if got := argAfter(args, "--job-id"); got != adversarialJobID {
		t.Fatalf("jobId must be ONE untouched discrete arg after --job-id; got %q", got)
	}
	// The metacharacter-laden jobId must not have been split, dropped, or fused
	// with the target org: the discrete arg list is exactly the composed form.
	want := []string{"project", "deploy", "quick", "--job-id", adversarialJobID, "--target-org", "UAT_SANDBOX", "--json"}
	if len(args) != len(want) {
		t.Fatalf("expected %d discrete args (jobId not split/interpolated), got %d: %v", len(want), len(args), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}
