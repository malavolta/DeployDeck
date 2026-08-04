package salesforce_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// quickArgs mirrors the exact `sf project deploy quick` invocation QuickDeploy
// composes (HU-015 command, design.md Interfaces/Contracts), so the
// FakeRunner canned response matches the real args — and so jobID and
// targetOrg are asserted to be DISCRETE slice elements, never
// shell-interpolated.
func quickArgs(jobID, targetOrg string) []string {
	return []string{
		"project", "deploy", "quick",
		"--job-id", jobID,
		"--target-org", targetOrg,
		"--json",
	}
}

// TestClient_QuickDeploy_ComposesArgsAsSlice proves jobID and targetOrg are
// passed as SEPARATE slice elements to sf, never joined/interpolated
// (design.md Threat Matrix "PR/argument composition").
func TestClient_QuickDeploy_ComposesArgsAsSlice(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := quickArgs("0AfXXX", "UAT_SANDBOX")
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"id":"0AfXXX","status":"Succeeded"}}`),
	})
	client := salesforce.New(fr)

	_, err := client.QuickDeploy(context.Background(), "0AfXXX", "UAT_SANDBOX")
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
			t.Fatalf("arg[%d] = %q, want %q (jobID/targetOrg must stay discrete args)", i, call.Args[i], want)
		}
	}
}

// TestClient_QuickDeploy_SuccessPreservesRaw mirrors
// TestClient_CancelDeploy_SuccessPreservesRaw: a zero exit returns a
// QuickDeployResult whose Raw holds the verbatim response (persisted as
// quick.json by the caller), with no error.
func TestClient_QuickDeploy_SuccessPreservesRaw(t *testing.T) {
	fr := exec.NewFakeRunner()
	stdout := `{"status":0,"result":{"id":"0AfYYY","done":true,"status":"Succeeded"}}`
	fr.When("sf", quickArgs("0AfYYY", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(stdout),
	})
	client := salesforce.New(fr)

	got, err := client.QuickDeploy(context.Background(), "0AfYYY", "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error on a successful quick deploy: %v", err)
	}
	if got.Raw != stdout {
		t.Fatalf("expected Raw to preserve the verbatim response, got %q", got.Raw)
	}
}

// TestClient_QuickDeploy_CLIErrorSurfacesDecodedMessageAndRaw mirrors
// CancelDeploy's error handling: a non-zero exit returns a QuickDeployResult
// carrying Raw together with an error surfacing the decoded oclif message.
func TestClient_QuickDeploy_CLIErrorSurfacesDecodedMessageAndRaw(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", quickArgs("0AfZZZ", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"QuickDeployFailed","message":"Job outside quick deploy window","exitCode":1}`),
	})
	client := salesforce.New(fr)

	got, err := client.QuickDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error for a non-zero exit code")
	}
	if !strings.Contains(err.Error(), "Job outside quick deploy window") {
		t.Fatalf("expected error to surface the decoded message, got: %v", err)
	}
	if !strings.Contains(got.Raw, "QuickDeployFailed") {
		t.Fatalf("expected Raw to be preserved on error, got %q", got.Raw)
	}
}

// TestClient_QuickDeploy_ParsesStatus is the deploy-error-detail RED (task
// 1.11): a successful quick deploy decodes Status from the envelope; Raw
// stays byte-identical to stdout (MarkQuickDeployed's asserted bytes).
func TestClient_QuickDeploy_ParsesStatus(t *testing.T) {
	fr := exec.NewFakeRunner()
	stdout := `{"status":0,"result":{"id":"0Af123","status":"Succeeded"}}`
	fr.When("sf", quickArgs("0Af123", "UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(stdout),
	})
	client := salesforce.New(fr)

	got, err := client.QuickDeploy(context.Background(), "0Af123", "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error on a successful quick deploy: %v", err)
	}
	if got.Status != "Succeeded" {
		t.Fatalf("expected Status %q, got %q", "Succeeded", got.Status)
	}
	if got.Raw != stdout {
		t.Fatalf("expected Raw to stay byte-identical to stdout, got %q", got.Raw)
	}
}

// TestClient_QuickDeploy_RunnerErrorDoesNotPanic mirrors CancelDeploy's
// runner-start failure: no canned response means Run errors — QuickDeploy
// must return an error, never panic.
func TestClient_QuickDeploy_RunnerErrorDoesNotPanic(t *testing.T) {
	fr := exec.NewFakeRunner() // no When() registered
	client := salesforce.New(fr)

	_, err := client.QuickDeploy(context.Background(), "0AfZZZ", "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error when the runner itself fails to start the command")
	}
}
