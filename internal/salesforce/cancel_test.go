package salesforce_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// cancelArgs mirrors the exact `sf project deploy cancel` invocation
// CancelDeploy composes (HU-012 command, HISTORIAS.md:796-800), so the
// FakeRunner canned response matches the real args — and so the jobId and
// alias are asserted to be DISCRETE slice elements, never shell-interpolated.
//
// testProjectDir (defined in quick_test.go, shared package-level test data)
// is the dir CancelDeploy's tests pass as the new trailing parameter
// (design.md ADR-8), asserted against exec.CommandRequest.Dir.
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

	_, err := client.CancelDeploy(context.Background(), "0AfXXX", "UAT_SANDBOX", testProjectDir)
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
	// dir reaches the child ONLY as CommandRequest.Dir, never appended to
	// Args (design.md Threat Matrix "PR commands / argument composition").
	if call.Dir != testProjectDir {
		t.Fatalf("Dir = %q, want %q", call.Dir, testProjectDir)
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

	got, err := client.CancelDeploy(context.Background(), "0AfYYY", "UAT_SANDBOX", testProjectDir)
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

	got, err := client.CancelDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX", testProjectDir)
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

	got, err := client.CancelDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX", testProjectDir)
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

// TestClient_CancelDeploy_ParsesStatusAndCanceledByName is the
// deploy-error-detail RED (task 1.9): a successful cancel decodes
// Status/CanceledByName from the envelope, and Raw stays byte-identical to
// stdout (D6, design.md Interfaces/Contracts).
func TestClient_CancelDeploy_ParsesStatusAndCanceledByName(t *testing.T) {
	fr := exec.NewFakeRunner()
	stdout := `{"status":0,"result":{"id":"0Af123","status":"Canceled","canceledByName":"Jane Doe"}}`
	fr.When("sf", cancelArgs("0Af123", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(stdout),
	})
	client := salesforce.New(fr)

	got, err := client.CancelDeploy(context.Background(), "0Af123", "UAT_SANDBOX", testProjectDir)
	if err != nil {
		t.Fatalf("unexpected error on a successful cancel: %v", err)
	}
	if got.Status != "Canceled" {
		t.Fatalf("expected Status %q, got %q", "Canceled", got.Status)
	}
	if got.CanceledByName != "Jane Doe" {
		t.Fatalf("expected CanceledByName %q, got %q", "Jane Doe", got.CanceledByName)
	}
	if got.Raw != stdout {
		t.Fatalf("expected Raw to stay byte-identical to stdout, got %q", got.Raw)
	}
}

// TestClient_CancelDeploy_AlreadyTerminalPreClassifiedAsSentinel is the
// deploy-error-detail RED (task 1.9, D6): an error envelope named
// CannotCancelDeployPre classifies via errors.Is(ErrCancelAlreadyTerminal).
func TestClient_CancelDeploy_AlreadyTerminalPreClassifiedAsSentinel(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", cancelArgs("0AfPRE", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"CannotCancelDeployPre","message":"Cannot cancel: job already completed","exitCode":1}`),
	})
	client := salesforce.New(fr)

	_, err := client.CancelDeploy(context.Background(), "0AfPRE", "UAT_SANDBOX", testProjectDir)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, salesforce.ErrCancelAlreadyTerminal) {
		t.Fatalf("expected ErrCancelAlreadyTerminal, got: %v", err)
	}
}

// TestClient_CancelDeploy_AlreadyTerminalRacedClassifiedAsSentinel is the
// deploy-error-detail RED (task 1.9, D6): an error envelope named
// CannotCancelDeploy (the raced/in-flight variant) also classifies via the
// same sentinel — both mean "job already terminal, nothing to cancel".
func TestClient_CancelDeploy_AlreadyTerminalRacedClassifiedAsSentinel(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", cancelArgs("0AfRACE", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"CannotCancelDeploy","message":"Cannot cancel: job already completed","exitCode":1}`),
	})
	client := salesforce.New(fr)

	_, err := client.CancelDeploy(context.Background(), "0AfRACE", "UAT_SANDBOX", testProjectDir)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, salesforce.ErrCancelAlreadyTerminal) {
		t.Fatalf("expected ErrCancelAlreadyTerminal, got: %v", err)
	}
}

// TestClient_CancelDeploy_OtherNameStaysGenericError is the
// deploy-error-detail RED (task 1.9, D6): an error envelope name OTHER than
// the two already-terminal names must stay a generic error, never
// misclassified as ErrCancelAlreadyTerminal.
func TestClient_CancelDeploy_OtherNameStaysGenericError(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", cancelArgs("0AfGEN", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"UNKNOWN_EXCEPTION","message":"An unexpected error occurred","exitCode":1}`),
	})
	client := salesforce.New(fr)

	_, err := client.CancelDeploy(context.Background(), "0AfGEN", "UAT_SANDBOX", testProjectDir)
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, salesforce.ErrCancelAlreadyTerminal) {
		t.Fatal("a non-already-terminal failure must not be classified as ErrCancelAlreadyTerminal")
	}
	if !strings.Contains(err.Error(), "An unexpected error occurred") {
		t.Fatalf("expected the generic error message to be surfaced, got: %v", err)
	}
}

// TestClient_CancelDeploy_RunnerErrorDoesNotPanic mirrors ValidateDeploy's
// runner-start failure: no canned response means Run errors — CancelDeploy must
// return an error, never panic.
func TestClient_CancelDeploy_RunnerErrorDoesNotPanic(t *testing.T) {
	fr := exec.NewFakeRunner() // no When() registered
	client := salesforce.New(fr)

	_, err := client.CancelDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX", testProjectDir)
	if err == nil {
		t.Fatal("expected an error when the runner itself fails to start the command")
	}
}
