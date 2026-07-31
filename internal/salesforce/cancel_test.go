package salesforce_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// cancelArgs mirrors the exact `sf project deploy cancel` invocation
// CancelDeploy composes (HU-012 command, HISTORIAS.md:796-800), so the
// FakeRunner canned response matches the real args — and so the jobId and
// alias are asserted to be DISCRETE slice elements, never shell-interpolated.
func cancelArgs(jobID, alias string) []string {
	return []string{
		"project", "deploy", "cancel",
		"--job-id", jobID,
		"--target-org", alias,
		"--json",
	}
}

// TestClient_CancelDeploy_ComposesArgsAsSlice proves the jobId and alias are
// passed as SEPARATE slice elements to sf, never joined/interpolated
// (design.md Threat Matrix "PR / argument composition").
func TestClient_CancelDeploy_ComposesArgsAsSlice(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := cancelArgs("0AfXXX", "UAT_SANDBOX")
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"id":"0AfXXX","canceledBy":"me"}}`),
	})
	client := salesforce.New(fr)

	_, err := client.CancelDeploy(context.Background(), "0AfXXX", "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fr.Calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(fr.Calls))
	}
	call := fr.Calls[0]
	if len(call.Args) != len(args) {
		t.Fatalf("expected %d args, got %d: %v", len(args), len(call.Args), call.Args)
	}
	for i, want := range args {
		if call.Args[i] != want {
			t.Fatalf("arg[%d] = %q, want %q (jobId/alias must stay discrete args)", i, call.Args[i], want)
		}
	}
}

// TestClient_CancelDeploy_SuccessPreservesRaw covers the happy path: a zero
// exit returns a CancelResult whose Raw holds the verbatim response (persisted
// as cancel.json by the caller), with no error.
func TestClient_CancelDeploy_SuccessPreservesRaw(t *testing.T) {
	fr := exec.NewFakeRunner()
	stdout := `{"status":0,"result":{"id":"0AfYYY","done":true,"status":"Canceled"}}`
	fr.When("sf", cancelArgs("0AfYYY", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(stdout),
	})
	client := salesforce.New(fr)

	got, err := client.CancelDeploy(context.Background(), "0AfYYY", "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error on a successful cancel: %v", err)
	}
	if got.Raw != stdout {
		t.Fatalf("expected Raw to preserve the verbatim response, got %q", got.Raw)
	}
}

// TestClient_CancelDeploy_CLIErrorSurfacesDecodedMessageAndRaw mirrors
// ValidateDeploy's error handling: a non-zero exit returns a CancelResult
// carrying Raw together with an error surfacing the decoded oclif message.
func TestClient_CancelDeploy_CLIErrorSurfacesDecodedMessageAndRaw(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", cancelArgs("0AfZZZ", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"JobNotCancelable","message":"Job already completed","exitCode":1}`),
	})
	client := salesforce.New(fr)

	got, err := client.CancelDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error for a non-zero exit code")
	}
	if !strings.Contains(err.Error(), "Job already completed") {
		t.Fatalf("expected error to surface the decoded message, got: %v", err)
	}
	if !strings.Contains(got.Raw, "JobNotCancelable") {
		t.Fatalf("expected Raw to be preserved on error, got %q", got.Raw)
	}
}

// TestClient_CancelDeploy_CLIErrorSurfacesRawWhenUnparseable covers the
// stderr-only failure path: Raw and the error both fall back to the combined
// output when stdout is not parseable JSON.
func TestClient_CancelDeploy_CLIErrorSurfacesRawWhenUnparseable(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", cancelArgs("0AfZZZ", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(""),
		Stderr:   []byte("sf: network unreachable\n"),
	})
	client := salesforce.New(fr)

	got, err := client.CancelDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error for a non-zero exit code")
	}
	if !strings.Contains(err.Error(), "network unreachable") {
		t.Fatalf("expected error to surface raw stderr when stdout is unparseable, got: %v", err)
	}
	if !strings.Contains(got.Raw, "network unreachable") {
		t.Fatalf("expected Raw to preserve the raw output on error, got %q", got.Raw)
	}
}

// TestClient_CancelDeploy_RunnerErrorDoesNotPanic mirrors ValidateDeploy's
// runner-start failure: no canned response means Run errors — CancelDeploy must
// return an error, never panic.
func TestClient_CancelDeploy_RunnerErrorDoesNotPanic(t *testing.T) {
	fr := exec.NewFakeRunner() // no When() registered
	client := salesforce.New(fr)

	_, err := client.CancelDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error when the runner itself fails to start the command")
	}
}
