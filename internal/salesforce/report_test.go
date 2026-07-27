package salesforce_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/salesforce"
)

func TestClient_ReportDeploy_ComposesArgs(t *testing.T) {
	fr := exec.NewFakeRunner()
	wantArgs := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000001EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", wantArgs, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{
			"status":"InProgress",
			"numberComponentsTotal":10,
			"numberComponentsDeployed":4,
			"numberComponentErrors":0,
			"numberTestsTotal":0,
			"numberTestsCompleted":0,
			"numberTestErrors":0,
			"details":{"componentFailures":[],"runTestResult":{"failures":[]}}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000001EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "InProgress" {
		t.Fatalf("expected Status %q, got %q", "InProgress", got.Status)
	}
	if got.NumberComponentsTotal != 10 || got.NumberComponentsDeployed != 4 {
		t.Fatalf("expected NumberComponentsTotal=10 NumberComponentsDeployed=4, got %+v", got)
	}

	if len(fr.Calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(fr.Calls))
	}
	if fr.Calls[0].Dir != "/repo" {
		t.Fatalf("expected Dir %q, got %q", "/repo", fr.Calls[0].Dir)
	}
}

func TestClient_ReportDeploy_ParsesComponentFailures(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000002EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{
			"status":"Failed",
			"numberComponentsTotal":10,
			"numberComponentsDeployed":9,
			"numberComponentErrors":1,
			"numberTestsTotal":0,
			"numberTestsCompleted":0,
			"numberTestErrors":0,
			"details":{
				"componentFailures":[
					{"fullName":"MyClass","componentType":"ApexClass","problem":"Compile error: unexpected token"}
				],
				"runTestResult":{"failures":[]}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000002EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.ComponentFailures) != 1 {
		t.Fatalf("expected 1 component failure, got %d: %+v", len(got.ComponentFailures), got.ComponentFailures)
	}
	f := got.ComponentFailures[0]
	if f.Component != "MyClass" || f.Type != "ApexClass" || f.Message != "Compile error: unexpected token" {
		t.Fatalf("expected {Component:MyClass Type:ApexClass Message:...}, got %+v", f)
	}
}

func TestClient_ReportDeploy_ParsesTestFailures(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000003EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{
			"status":"Failed",
			"numberComponentsTotal":10,
			"numberComponentsDeployed":10,
			"numberComponentErrors":0,
			"numberTestsTotal":5,
			"numberTestsCompleted":5,
			"numberTestErrors":1,
			"details":{
				"componentFailures":[],
				"runTestResult":{
					"failures":[
						{"name":"MyClassTest","methodName":"testSomething","message":"System.AssertException: Assertion Failed"}
					]
				}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000003EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.TestFailures) != 1 {
		t.Fatalf("expected 1 test failure, got %d: %+v", len(got.TestFailures), got.TestFailures)
	}
	f := got.TestFailures[0]
	if f.Class != "MyClassTest" || f.Method != "testSomething" || f.Message != "System.AssertException: Assertion Failed" {
		t.Fatalf("expected {Class:MyClassTest Method:testSomething Message:...}, got %+v", f)
	}
}

func TestClient_ReportDeploy_CLIErrorSurfacesMessageAndRaw(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000004EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stderr:   []byte("Error: No job found with id 0Af000000000004EAA\n"),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000004EAA", "UAT_SANDBOX", "/repo")
	if err == nil {
		t.Fatal("expected an error for a non-zero exit code")
	}
	if !strings.Contains(err.Error(), "No job found") {
		t.Fatalf("expected error to surface the CLI message, got: %v", err)
	}
	if !strings.Contains(got.Raw, "No job found") {
		t.Fatalf("expected Raw to be preserved on error, got %q", got.Raw)
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"Succeeded", true},
		{"SucceededPartial", true},
		{"Failed", true},
		{"Canceled", true},
		{"InProgress", false},
		{"Queued", false},
		{"Pending", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := salesforce.IsTerminal(tt.status); got != tt.want {
				t.Errorf("IsTerminal(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
