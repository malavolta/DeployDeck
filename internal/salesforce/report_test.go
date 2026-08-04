package salesforce_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
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

// TestClient_ReportDeploy_ParsesCoverageWarningsAndOrgError is the HU-011 bug
// fix: a Failed report with zero componentFailures/testFailures can still
// carry the REAL failure reason under details.runTestResult.
// codeCoverageWarnings[] (a code-coverage shortfall) and/or the top-level
// errorMessage/errorStatusCode (an org-level error) — both must survive
// parsing, not be silently dropped.
func TestClient_ReportDeploy_ParsesCoverageWarningsAndOrgError(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000007EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout: []byte(`{"status":1,"result":{
			"status":"Failed",
			"numberComponentsTotal":0,
			"numberComponentsDeployed":0,
			"numberComponentErrors":0,
			"numberTestsTotal":0,
			"numberTestsCompleted":0,
			"numberTestErrors":0,
			"errorMessage":"INVALID_STATUS: metadata coverage failure",
			"errorStatusCode":"INVALID_STATUS",
			"details":{
				"componentFailures":[],
				"runTestResult":{
					"failures":[],
					"codeCoverageWarnings":[
						{"id":"","message":"Average test coverage across all Apex Classes and Triggers is 0%, at least 75% test coverage is required.","name":null,"namespace":""}
					]
				}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000007EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("a parseable terminal report must not error: %v", err)
	}
	if len(got.CodeCoverageWarnings) != 1 {
		t.Fatalf("expected 1 coverage warning, got %d: %+v", len(got.CodeCoverageWarnings), got.CodeCoverageWarnings)
	}
	w := got.CodeCoverageWarnings[0]
	if w.Name != "" {
		t.Errorf("expected empty Name for an org-wide coverage warning (null name), got %q", w.Name)
	}
	if !strings.Contains(w.Message, "test coverage") {
		t.Errorf("expected the coverage warning message to be parsed, got %q", w.Message)
	}
	if got.ErrorMessage != "INVALID_STATUS: metadata coverage failure" {
		t.Errorf("ErrorMessage not parsed, got %q", got.ErrorMessage)
	}
	if got.ErrorStatusCode != "INVALID_STATUS" {
		t.Errorf("ErrorStatusCode not parsed, got %q", got.ErrorStatusCode)
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

func TestClient_ReportDeploy_NonZeroExitTerminalReportIsParsed(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000005EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	// `sf project deploy report --json` exits NON-ZERO on a genuinely terminal
	// Failed deploy while STILL emitting the full report under result. Terminal
	// detection must be exit-code-INDEPENDENT: this must decode to a terminal
	// DeployReport with a nil error (NOT a transient error that would keep the
	// poll loop running to the hard timeout with no failure detail).
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout: []byte(`{"status":1,"result":{
			"status":"Failed",
			"numberComponentsTotal":3,
			"numberComponentsDeployed":0,
			"numberComponentErrors":3,
			"numberTestsTotal":0,
			"numberTestsCompleted":0,
			"numberTestErrors":0,
			"details":{
				"componentFailures":[
					{"fullName":"AccountService","componentType":"ApexClass","problem":"line 3: unexpected token"}
				],
				"runTestResult":{"failures":[]}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000005EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("a parseable terminal report must NOT be a transient error, got: %v", err)
	}
	if got.Status != "Failed" {
		t.Fatalf("expected terminal Status %q regardless of exit code, got %q", "Failed", got.Status)
	}
	if !salesforce.IsTerminal(got.Status) {
		t.Fatalf("Status %q should be terminal so polling stops", got.Status)
	}
	if got.NumberComponentErrors != 3 || len(got.ComponentFailures) != 1 {
		t.Fatalf("terminal report detail lost on a non-zero exit: %+v", got)
	}
	if f := got.ComponentFailures[0]; f.Component != "AccountService" || f.Type != "ApexClass" {
		t.Fatalf("component failure not parsed from a non-zero-exit report: %+v", f)
	}
}

func TestClient_ReportDeploy_NonZeroExitUnparseableStillErrors(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000006EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	// A non-zero exit whose output is NOT a parseable report (an oclif error
	// envelope with no result object) stays an error, with Raw preserved for
	// display — only a genuine report short-circuits the error path.
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"GenericTimeoutError","message":"The request timed out","exitCode":1}`),
		Stderr:   []byte("Warning: request timed out\n"),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000006EAA", "UAT_SANDBOX", "/repo")
	if err == nil {
		t.Fatal("a non-zero exit that is not a parseable report must still error")
	}
	if got.Status != "" {
		t.Fatalf("an unparseable error must not fabricate a status, got %q", got.Status)
	}
	if !strings.Contains(got.Raw, "timed out") {
		t.Fatalf("Raw must be preserved on an unparseable error, got %q", got.Raw)
	}
}

// TestClient_ReportDeploy_ParsesComponentFailureFileLineColumnAndProblemType
// is the deploy-error-detail RED (task 1.1): componentFailures entries carry
// fileName/lineNumber/columnNumber/problemType (Error AND Warning), single-
// and multi-entry, and a legacy entry missing these fields decodes with them
// zero-valued rather than erroring.
func TestClient_ReportDeploy_ParsesComponentFailureFileLineColumnAndProblemType(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000010EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout: []byte(`{"status":1,"result":{
			"status":"Failed",
			"numberComponentsTotal":3,
			"numberComponentsDeployed":0,
			"numberComponentErrors":3,
			"numberTestsTotal":0,
			"numberTestsCompleted":0,
			"numberTestErrors":0,
			"details":{
				"componentFailures":[
					{"fullName":"MyClass","componentType":"ApexClass","problem":"Compile error: unexpected token","fileName":"classes/MyClass.cls","lineNumber":12,"columnNumber":5,"problemType":"Error"},
					{"fullName":"MyTrigger","componentType":"ApexTrigger","problem":"Unused variable","fileName":"triggers/MyTrigger.trigger","lineNumber":3,"problemType":"Warning"},
					{"fullName":"LegacyComponent","componentType":"ApexClass","problem":"legacy entry, no location fields"}
				],
				"runTestResult":{"failures":[]}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000010EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("a parseable terminal report must not error: %v", err)
	}
	if len(got.ComponentFailures) != 3 {
		t.Fatalf("expected 3 component failures, got %d: %+v", len(got.ComponentFailures), got.ComponentFailures)
	}

	errEntry := got.ComponentFailures[0]
	if errEntry.FileName != "classes/MyClass.cls" || errEntry.LineNumber != 12 || errEntry.ColumnNumber != 5 || errEntry.ProblemType != "Error" {
		t.Fatalf("expected Error entry {FileName:classes/MyClass.cls LineNumber:12 ColumnNumber:5 ProblemType:Error}, got %+v", errEntry)
	}

	warnEntry := got.ComponentFailures[1]
	if warnEntry.FileName != "triggers/MyTrigger.trigger" || warnEntry.LineNumber != 3 || warnEntry.ColumnNumber != 0 || warnEntry.ProblemType != "Warning" {
		t.Fatalf("expected Warning entry {FileName:triggers/MyTrigger.trigger LineNumber:3 ColumnNumber:0 ProblemType:Warning}, got %+v", warnEntry)
	}

	legacyEntry := got.ComponentFailures[2]
	if legacyEntry.FileName != "" || legacyEntry.LineNumber != 0 || legacyEntry.ColumnNumber != 0 || legacyEntry.ProblemType != "" {
		t.Fatalf("expected legacy entry to stay zero-valued for absent location fields, got %+v", legacyEntry)
	}
}

// TestClient_ReportDeploy_ParsesTestFailureStackTrace is the deploy-error-detail
// RED (task 1.2): TestFailure.StackTrace is captured when the report provides
// it and stays empty when absent.
func TestClient_ReportDeploy_ParsesTestFailureStackTrace(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000011EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout: []byte(`{"status":1,"result":{
			"status":"Failed",
			"numberComponentsTotal":0,
			"numberComponentsDeployed":0,
			"numberComponentErrors":0,
			"numberTestsTotal":5,
			"numberTestsCompleted":5,
			"numberTestErrors":2,
			"details":{
				"componentFailures":[],
				"runTestResult":{
					"failures":[
						{"name":"MyClassTest","methodName":"testSomething","message":"System.AssertException: Assertion Failed","stackTrace":"Class.MyClassTest.testSomething: line 10, column 1"},
						{"name":"OtherClassTest","methodName":"testOther","message":"System.NullPointerException"}
					]
				}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000011EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("a parseable terminal report must not error: %v", err)
	}
	if len(got.TestFailures) != 2 {
		t.Fatalf("expected 2 test failures, got %d: %+v", len(got.TestFailures), got.TestFailures)
	}
	if got.TestFailures[0].StackTrace != "Class.MyClassTest.testSomething: line 10, column 1" {
		t.Fatalf("expected StackTrace captured, got %q", got.TestFailures[0].StackTrace)
	}
	if got.TestFailures[1].StackTrace != "" {
		t.Fatalf("expected StackTrace to stay empty when absent, got %q", got.TestFailures[1].StackTrace)
	}
}

// TestCodeCoverageResult_Percent is the deploy-error-detail RED (task 1.3,
// pure-function half): Percent() computes the covered-locations percentage,
// with num==0 degrading to 0 rather than dividing by zero.
func TestCodeCoverageResult_Percent(t *testing.T) {
	tests := []struct {
		name string
		c    salesforce.CodeCoverageResult
		want int
	}{
		{"zero locations does not divide by zero", salesforce.CodeCoverageResult{NumLocations: 0, NumLocationsNotCovered: 0}, 0},
		{"normal division", salesforce.CodeCoverageResult{NumLocations: 100, NumLocationsNotCovered: 30}, 70},
		{"fully covered", salesforce.CodeCoverageResult{NumLocations: 50, NumLocationsNotCovered: 0}, 100},
		{"fully uncovered", salesforce.CodeCoverageResult{NumLocations: 20, NumLocationsNotCovered: 20}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Percent(); got != tt.want {
				t.Errorf("Percent() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestClient_ReportDeploy_ParsesCodeCoverageResults is the deploy-error-detail
// RED (task 1.3, envelope half): codeCoverage[] entries decode into
// CodeCoverageResult{Name,Namespace,NumLocations,NumLocationsNotCovered}.
func TestClient_ReportDeploy_ParsesCodeCoverageResults(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000012EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout: []byte(`{"status":1,"result":{
			"status":"Failed",
			"details":{
				"componentFailures":[],
				"runTestResult":{
					"failures":[],
					"codeCoverage":[
						{"name":"MyClass","namespace":"","numLocations":100,"numLocationsNotCovered":40},
						{"name":"OtherClass","namespace":"ns","numLocations":10,"numLocationsNotCovered":0}
					]
				}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000012EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("a parseable terminal report must not error: %v", err)
	}
	if len(got.CodeCoverage) != 2 {
		t.Fatalf("expected 2 code coverage results, got %d: %+v", len(got.CodeCoverage), got.CodeCoverage)
	}
	c := got.CodeCoverage[0]
	if c.Name != "MyClass" || c.NumLocations != 100 || c.NumLocationsNotCovered != 40 {
		t.Fatalf("expected {Name:MyClass NumLocations:100 NumLocationsNotCovered:40}, got %+v", c)
	}
	if c.Percent() != 60 {
		t.Fatalf("expected Percent() 60, got %d", c.Percent())
	}
	if got.CodeCoverage[1].Namespace != "ns" {
		t.Fatalf("expected Namespace ns, got %+v", got.CodeCoverage[1])
	}
}

// TestClient_ReportDeploy_ParsesFlowCoverageWarningsAndStateDetail is the
// deploy-error-detail RED (task 1.4): flowCoverageWarnings[] decode into
// FlowCoverageWarning{FlowName,Message} and top-level stateDetail decodes
// into DeployReport.StateDetail.
func TestClient_ReportDeploy_ParsesFlowCoverageWarningsAndStateDetail(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000013EAA",
		"--target-org", "UAT_SANDBOX",
		"--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{
			"status":"InProgress",
			"stateDetail":"Deploying Metadata",
			"details":{
				"componentFailures":[],
				"runTestResult":{
					"failures":[],
					"flowCoverageWarnings":[
						{"flowName":"My_Flow","message":"Flow coverage below threshold"}
					]
				}
			}
		}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ReportDeploy(context.Background(), "0Af000000000013EAA", "UAT_SANDBOX", "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.FlowCoverageWarnings) != 1 {
		t.Fatalf("expected 1 flow coverage warning, got %d: %+v", len(got.FlowCoverageWarnings), got.FlowCoverageWarnings)
	}
	w := got.FlowCoverageWarnings[0]
	if w.FlowName != "My_Flow" || w.Message != "Flow coverage below threshold" {
		t.Fatalf("expected {FlowName:My_Flow Message:...}, got %+v", w)
	}
	if got.StateDetail != "Deploying Metadata" {
		t.Fatalf("expected StateDetail %q, got %q", "Deploying Metadata", got.StateDetail)
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
