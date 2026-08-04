package salesforce_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// deployQueueSOQL mirrors the exact query internal/salesforce's
// ListDeployQueue composes (HU-009 command, EPICA.md:371-375 /
// HISTORIAS.md:606-614), so FakeRunner.When matches the real args.
// StateDetail/ErrorMessage/ErrorStatusCode are additive columns
// (deploy-error-detail D8): the WHERE clause stays Status IN
// ('Pending','InProgress') unchanged (design.md Open Questions).
const deployQueueSOQL = "SELECT Id,Status,CheckOnly,CreatedDate,StartDate,CompletedDate,CreatedBy.Name,CreatedBy.Username,StateDetail,ErrorMessage,ErrorStatusCode,NumberComponentsTotal,NumberComponentsDeployed,NumberComponentErrors,NumberTestsTotal,NumberTestsCompleted,NumberTestErrors FROM DeployRequest WHERE Status IN ('Pending','InProgress') ORDER BY CreatedDate ASC"

func deployQueueArgs(alias string) []string {
	return []string{
		"data", "query",
		"--target-org", alias,
		"--use-tooling-api",
		"--json",
		"--query", deployQueueSOQL,
	}
}

// TestClient_ListDeployQueue_ComposesSOQLAsOneSliceArg proves the SOQL is
// passed as a SINGLE slice element — never shell-joined/interpolated
// (design.md Threat Matrix "PR / argument composition").
func TestClient_ListDeployQueue_ComposesSOQLAsOneSliceArg(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := deployQueueArgs("UAT_SANDBOX")
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"totalSize":0,"done":true,"records":[]}}`),
	})
	client := salesforce.New(fr)

	_, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
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
			t.Fatalf("arg[%d] = %q, want %q (SOQL must stay ONE arg)", i, call.Args[i], want)
		}
	}
}

// TestClient_ListDeployQueue_ParsesMultiUserOrderedInclOwn covers a queue
// with several users' jobs, incl. the current run's own job, and asserts the
// envelope's {totalSize,done,records[]} shape decodes into DeployQueueEntry
// with CreatedBy.Name and CreatedBy.Username both parsed (own-job identity
// depends on Username).
func TestClient_ListDeployQueue_ParsesMultiUserOrderedInclOwn(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", deployQueueArgs("UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{"totalSize":3,"done":true,"records":[
			{"attributes":{"type":"DeployRequest"},"Id":"0AfAAA","Status":"InProgress","CheckOnly":true,"CreatedDate":"2026-07-27T10:00:00.000+0000","StartDate":"2026-07-27T10:00:05.000+0000","CompletedDate":null,"CreatedBy":{"Name":"Maria Garcia","Username":"maria@example.com"},"NumberComponentsTotal":34,"NumberComponentsDeployed":12,"NumberComponentErrors":0,"NumberTestsTotal":142,"NumberTestsCompleted":80,"NumberTestErrors":0},
			{"attributes":{"type":"DeployRequest"},"Id":"0AfBBB","Status":"Pending","CheckOnly":false,"CreatedDate":"2026-07-27T10:10:00.000+0000","StartDate":null,"CompletedDate":null,"CreatedBy":{"Name":"Luis Perez","Username":"luis@example.com"},"NumberComponentsTotal":0,"NumberComponentsDeployed":0,"NumberComponentErrors":0,"NumberTestsTotal":0,"NumberTestsCompleted":0,"NumberTestErrors":0},
			{"attributes":{"type":"DeployRequest"},"Id":"0AfCCC","Status":"Pending","CheckOnly":true,"CreatedDate":"2026-07-27T10:15:00.000+0000","StartDate":null,"CompletedDate":null,"CreatedBy":{"Name":"Own User","Username":"own@example.com"},"NumberComponentsTotal":0,"NumberComponentsDeployed":0,"NumberComponentErrors":0,"NumberTestsTotal":0,"NumberTestsCompleted":0,"NumberTestErrors":0}
		]}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}
	wantOrder := []string{"0AfAAA", "0AfBBB", "0AfCCC"}
	for i, id := range wantOrder {
		if got[i].JobID != id {
			t.Fatalf("record[%d].JobID = %q, want %q (CreatedDate ASC ordering not preserved)", i, got[i].JobID, id)
		}
	}
	own := got[2]
	if own.Username != "own@example.com" {
		t.Fatalf("own record Username not parsed: %+v", own)
	}
	if own.CreatedBy != "Own User" {
		t.Fatalf("own record CreatedBy.Name not parsed: %+v", own)
	}
	if !own.CheckOnly {
		t.Error("own record CheckOnly should be true (a validation, not a deploy)")
	}
	first := got[0]
	if first.Components.Total != 34 || first.Components.Deployed != 12 || first.Components.Errors != 0 {
		t.Fatalf("component counts not parsed: %+v", first.Components)
	}
	if first.Tests.Total != 142 || first.Tests.Completed != 80 || first.Tests.Errors != 0 {
		t.Fatalf("test counts not parsed: %+v", first.Tests)
	}
	if first.CreatedDate.IsZero() {
		t.Error("CreatedDate should parse to a non-zero time")
	}
	if first.StartDate.IsZero() {
		t.Error("StartDate should parse to a non-zero time when present")
	}
}

// TestClient_ListDeployQueue_EmptyQueueReturnsEmptySlice covers HU-009's
// "clear empty state" AC at the client layer: zero records must not error.
func TestClient_ListDeployQueue_EmptyQueueReturnsEmptySlice(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", deployQueueArgs("UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"totalSize":0,"done":true,"records":[]}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected an empty queue, got %d entries", len(got))
	}
}

// TestClient_ListDeployQueue_CheckOnlyDistinguishesValidationFromDeploy
// covers HU-009's CheckOnly AC for both true and false records.
func TestClient_ListDeployQueue_CheckOnlyDistinguishesValidationFromDeploy(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", deployQueueArgs("UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{"totalSize":2,"done":true,"records":[
			{"Id":"0AfV","Status":"InProgress","CheckOnly":true,"CreatedDate":"2026-07-27T10:00:00.000+0000","CreatedBy":{"Name":"A","Username":"a@example.com"}},
			{"Id":"0AfD","Status":"InProgress","CheckOnly":false,"CreatedDate":"2026-07-27T10:01:00.000+0000","CreatedBy":{"Name":"B","Username":"b@example.com"}}
		]}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || !got[0].CheckOnly || got[1].CheckOnly {
		t.Fatalf("CheckOnly not distinguishing validate vs deploy: %+v", got)
	}
}

// TestClient_ListDeployQueue_PermissionErrorReturnsErrQueuePermission covers
// the non-blocking-degrade branch: a Tooling-API-permission CLI error must be
// classified distinctly via the typed sentinel.
func TestClient_ListDeployQueue_PermissionErrorReturnsErrQueuePermission(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", deployQueueArgs("UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"INSUFFICIENT_ACCESS","message":"Insufficient access to Tooling API object DeployRequest","exitCode":1}`),
	})
	client := salesforce.New(fr)

	_, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, salesforce.ErrQueuePermission) {
		t.Fatalf("expected ErrQueuePermission, got: %v", err)
	}
}

// TestClient_ListDeployQueue_GenericErrorIsNotSwallowed proves a
// non-permission failure stays a generic, actionable error — never silently
// swallowed and never misclassified as ErrQueuePermission.
func TestClient_ListDeployQueue_GenericErrorIsNotSwallowed(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", deployQueueArgs("UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"UNKNOWN_EXCEPTION","message":"An unexpected error occurred","exitCode":1}`),
	})
	client := salesforce.New(fr)

	_, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error for a generic query failure")
	}
	if errors.Is(err, salesforce.ErrQueuePermission) {
		t.Fatal("a generic, non-permission failure must not be classified as ErrQueuePermission")
	}
	if !strings.Contains(err.Error(), "An unexpected error occurred") {
		t.Fatalf("expected the generic error message to be surfaced, got: %v", err)
	}
}

// TestClient_ListDeployQueue_ParsesStateDetailErrorMessageErrorStatusCode is
// the deploy-error-detail RED (task 1.7): DeployQueueEntry maps
// StateDetail/ErrorMessage/ErrorStatusCode from the record, and a record
// without them stays zero-valued (D8, backward-compatible).
func TestClient_ListDeployQueue_ParsesStateDetailErrorMessageErrorStatusCode(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", deployQueueArgs("UAT_SANDBOX"), exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{"totalSize":2,"done":true,"records":[
			{"Id":"0AfERR","Status":"InProgress","CheckOnly":false,"CreatedDate":"2026-07-27T10:00:00.000+0000","CreatedBy":{"Name":"A","Username":"a@example.com"},"StateDetail":"Deploying Metadata","ErrorMessage":"Component failed to deploy","ErrorStatusCode":"COMPONENT_FAILURE"},
			{"Id":"0AfOK","Status":"Pending","CheckOnly":false,"CreatedDate":"2026-07-27T10:01:00.000+0000","CreatedBy":{"Name":"B","Username":"b@example.com"}}
		]}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	errored := got[0]
	if errored.StateDetail != "Deploying Metadata" || errored.ErrorMessage != "Component failed to deploy" || errored.ErrorStatusCode != "COMPONENT_FAILURE" {
		t.Fatalf("expected errored entry {StateDetail:Deploying Metadata ErrorMessage:Component failed to deploy ErrorStatusCode:COMPONENT_FAILURE}, got %+v", errored)
	}
	clean := got[1]
	if clean.StateDetail != "" || clean.ErrorMessage != "" || clean.ErrorStatusCode != "" {
		t.Fatalf("expected a record without these fields to stay zero-valued, got %+v", clean)
	}
}

// TestClient_ListDeployQueue_RunnerErrorDoesNotPanic mirrors
// TestClient_ValidateDeploy_RunnerErrorDoesNotPanicAndFlowSurvives: a
// runner-start failure (no canned response) must return an error, never
// panic.
func TestClient_ListDeployQueue_RunnerErrorDoesNotPanic(t *testing.T) {
	fr := exec.NewFakeRunner() // no When() registered
	client := salesforce.New(fr)

	_, err := client.ListDeployQueue(context.Background(), "UAT_SANDBOX")
	if err == nil {
		t.Fatal("expected an error when the runner itself fails to start the command")
	}
}
