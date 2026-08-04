package salesforce

import (
	"context"
	"fmt"
	"strings"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// ComponentFailure is one metadata deploy error (HU-011 AC: "se muestran
// con componente, tipo y mensaje"), decoded from the report's
// details.componentFailures entries (fullName, componentType, problem,
// fileName, lineNumber, columnNumber, problemType). FileName/LineNumber/
// ColumnNumber/ProblemType are additive, backward-compatible fields
// (deploy-error-detail gap 1): an entry without them still decodes fine,
// leaving these at their zero value. ProblemType is "Error" or "Warning".
type ComponentFailure struct {
	Component string
	Type      string
	Message   string

	FileName     string
	LineNumber   int
	ColumnNumber int
	ProblemType  string
}

// TestFailure is one failed Apex test (HU-011 AC: "se muestran ... con
// clase, metodo y mensaje"), decoded from the report's
// details.runTestResult.failures entries (name, methodName, message,
// stackTrace). StackTrace is additive (deploy-error-detail gap 2): an
// entry without it still decodes fine, leaving it "".
type TestFailure struct {
	Class   string
	Method  string
	Message string

	StackTrace string
}

// CodeCoverageResult is one class/trigger's per-entity coverage, decoded
// from the report's details.runTestResult.codeCoverage[] entries
// (deploy-error-detail gap 3): distinct from CodeCoverageWarning (an
// org/class-level shortfall message), this carries the raw location counts
// needed to compute a percentage.
type CodeCoverageResult struct {
	Name      string
	Namespace string

	NumLocations           int
	NumLocationsNotCovered int
}

// Percent returns the covered-locations percentage, 0 when NumLocations is
// 0 (never divides by zero).
func (c CodeCoverageResult) Percent() int {
	if c.NumLocations == 0 {
		return 0
	}
	covered := c.NumLocations - c.NumLocationsNotCovered
	return covered * 100 / c.NumLocations
}

// FlowCoverageWarning is one Flow with insufficient test coverage, decoded
// from the report's details.runTestResult.flowCoverageWarnings[] entries
// (deploy-error-detail gap 3).
type FlowCoverageWarning struct {
	FlowName string
	Message  string
}

// CodeCoverageWarning is one code-coverage shortfall reported by the test
// run (bug fix: a Failed status can be caused by insufficient coverage with
// zero componentFailures/testFailures — this is the field that carries the
// real reason), decoded from the report's
// details.runTestResult.codeCoverageWarnings entries (name, namespace,
// message). Name is empty when the API reports a null/empty name, which
// means the warning is org-wide rather than tied to one class.
type CodeCoverageWarning struct {
	Name      string
	Namespace string
	Message   string
}

// DeployReport is a parsed `sf project deploy report --json` response.
type DeployReport struct {
	// Status is the job's current state, e.g. "Queued", "InProgress",
	// "Succeeded", "SucceededPartial", "Failed", "Canceled". See
	// IsTerminal for the subset that stops polling.
	Status string

	NumberComponentsTotal    int
	NumberComponentsDeployed int
	NumberComponentErrors    int

	NumberTestsTotal     int
	NumberTestsCompleted int
	NumberTestErrors     int

	// ErrorMessage is the org-level error message for a Failed report that
	// has no per-component/per-test detail (e.g. an errorStatusCode
	// condition). Often empty even on a real failure.
	ErrorMessage string
	// ErrorStatusCode is the status code paired with ErrorMessage.
	ErrorStatusCode string
	// CanceledByName is who canceled the job, populated when Status ==
	// "Canceled".
	CanceledByName string
	// StateDetail is the job's current-step description (e.g. "Deploying
	// Metadata"), additive (deploy-error-detail gap 4): a report without it
	// still decodes fine, leaving this "".
	StateDetail string

	ComponentFailures    []ComponentFailure
	TestFailures         []TestFailure
	CodeCoverageWarnings []CodeCoverageWarning
	// CodeCoverage is additive (deploy-error-detail gap 3): per-class
	// coverage results, distinct from CodeCoverageWarnings (shortfall
	// messages).
	CodeCoverage []CodeCoverageResult
	// FlowCoverageWarnings is additive (deploy-error-detail gap 3).
	FlowCoverageWarnings []FlowCoverageWarning

	// Raw is the command's captured stdout+stderr, kept for diagnostics
	// and display even when the call errored (HU-011 AC: "Guardar cada
	// respuesta raw relevante").
	Raw string
}

// reportResultEnvelope mirrors the sf CLI's `deploy report --json` result
// object shape (the same Metadata API DeployResult/RunTestsResult layout
// `sf project deploy validate`/`report` share): top-level counters plus a
// nested details object holding componentFailures and the test run's
// failures.
//
// Shape confirmed against REAL `sf project deploy report --json` output (sf
// CLI 2.135.7) by TestE2ERealOrg_ValidateAndReport: a Failed validate reported
// numberComponentErrors:3 under result.details.componentFailures[] with exactly
// {fullName, componentType, problem}, and result.details.runTestResult.failures
// as the test-failure container — i.e. the nesting inferred here matches the
// live CLI verbatim, no tag adjustment required.
type reportResultEnvelope struct {
	Status string `json:"status"`

	NumberComponentsTotal    int `json:"numberComponentsTotal"`
	NumberComponentsDeployed int `json:"numberComponentsDeployed"`
	NumberComponentErrors    int `json:"numberComponentErrors"`

	NumberTestsTotal     int `json:"numberTestsTotal"`
	NumberTestsCompleted int `json:"numberTestsCompleted"`
	NumberTestErrors     int `json:"numberTestErrors"`

	// ErrorMessage/ErrorStatusCode/CanceledByName are additive, backward-
	// compatible fields (bug fix): a report without them still decodes fine,
	// leaving these at their zero value ("").
	ErrorMessage    string `json:"errorMessage"`
	ErrorStatusCode string `json:"errorStatusCode"`
	CanceledByName  string `json:"canceledByName"`
	// StateDetail is additive (deploy-error-detail gap 4).
	StateDetail string `json:"stateDetail"`

	Details struct {
		ComponentFailures []struct {
			FullName      string `json:"fullName"`
			ComponentType string `json:"componentType"`
			Problem       string `json:"problem"`
			// FileName/LineNumber/ColumnNumber/ProblemType are additive
			// (deploy-error-detail gap 1): an entry without them still
			// decodes fine, leaving these at their zero value.
			FileName     string `json:"fileName"`
			LineNumber   int    `json:"lineNumber"`
			ColumnNumber int    `json:"columnNumber"`
			ProblemType  string `json:"problemType"`
		} `json:"componentFailures"`
		RunTestResult struct {
			Failures []struct {
				Name       string `json:"name"`
				MethodName string `json:"methodName"`
				Message    string `json:"message"`
				// StackTrace is additive (deploy-error-detail gap 2).
				StackTrace string `json:"stackTrace"`
			} `json:"failures"`
			// CodeCoverageWarnings is additive (bug fix): a report without it
			// still decodes fine, leaving this nil. Name is nullable in the
			// API and left "" (org-wide) when null.
			CodeCoverageWarnings []struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Message   string `json:"message"`
			} `json:"codeCoverageWarnings"`
			// CodeCoverage is additive (deploy-error-detail gap 3): per-class
			// coverage results, a report without it still decodes fine,
			// leaving this nil.
			CodeCoverage []struct {
				Name                   string `json:"name"`
				Namespace              string `json:"namespace"`
				NumLocations           int    `json:"numLocations"`
				NumLocationsNotCovered int    `json:"numLocationsNotCovered"`
			} `json:"codeCoverage"`
			// FlowCoverageWarnings is additive (deploy-error-detail gap 3).
			FlowCoverageWarnings []struct {
				FlowName string `json:"flowName"`
				Message  string `json:"message"`
			} `json:"flowCoverageWarnings"`
		} `json:"runTestResult"`
	} `json:"details"`
}

// terminalStatuses are the deploy-report states HU-011 treats as final:
// polling stops once one of these is reached.
var terminalStatuses = map[string]bool{
	"Succeeded":        true,
	"SucceededPartial": true,
	"Failed":           true,
	"Canceled":         true,
}

// IsTerminal reports whether status is one of the terminal deploy-report
// states {Succeeded, SucceededPartial, Failed, Canceled}. Any other value
// (InProgress, Queued, Pending, "", ...) is non-terminal.
func IsTerminal(status string) bool {
	return terminalStatuses[status]
}

// ReportDeploy runs `sf project deploy report --job-id <jobID>
// --target-org <targetOrg> --json` once and decodes the result. Callers
// drive polling themselves (HU-011's ValidationPolling state,
// internal/app) — this is a single call, not a loop, matching
// design.md's "polling driver" decision (tea.Tick fires one ReportDeploy
// per tick; no blocking loop lives in this package). Terminal detection is
// exit-code-INDEPENDENT: any output that decodes to a report with a
// recognized status is returned as a DeployReport with a nil error, even
// when the CLI exited non-zero (sf exits non-zero on a terminal Failed /
// Canceled deploy while still emitting the full report). Only output that
// is NOT a parseable report returns an error carrying Raw, which the poll
// loop treats as a transient failure retryable within its deadline.
func (c *client) ReportDeploy(ctx context.Context, jobID, targetOrg, dir string) (DeployReport, error) {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{
			"project", "deploy", "report",
			"--job-id", jobID,
			"--target-org", targetOrg,
			"--json",
		},
		Dir: dir,
	})
	if err != nil {
		return DeployReport{}, fmt.Errorf("salesforce: running sf project deploy report: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)

	// Decode the result envelope BEFORE consulting the exit code. `sf project
	// deploy report --json` exits non-zero on a genuinely TERMINAL deploy
	// (Failed / Canceled) while STILL emitting the full report under result, so
	// keying terminal detection off the exit code would misread a real failure
	// as a transient error and keep polling to the hard timeout with no detail.
	// A parseable report — a decoded result carrying a (recognized, non-empty)
	// status — is authoritative and returned as data with a nil error, whatever
	// the exit code. Only output that is NOT a parseable report (an error
	// envelope, empty output, or garbage) is surfaced as an error, with Raw
	// preserved for display.
	var decoded reportResultEnvelope
	decodeErr := decodeEnvelope(result.Stdout, &decoded)
	if decodeErr != nil || decoded.Status == "" {
		if result.ExitCode != 0 {
			return DeployReport{Raw: raw}, fmt.Errorf(
				"salesforce: sf project deploy report exited %d: %s",
				result.ExitCode, strings.TrimSpace(string(result.Stderr)),
			)
		}
		if decodeErr != nil {
			return DeployReport{Raw: raw}, decodeErr
		}
		// Exit 0 with a decoded-but-empty status: fall through and return the
		// (non-terminal) report unchanged, so the caller keeps polling — this
		// preserves the pre-existing zero-exit behavior verbatim.
	}

	report := DeployReport{
		Status:                   decoded.Status,
		NumberComponentsTotal:    decoded.NumberComponentsTotal,
		NumberComponentsDeployed: decoded.NumberComponentsDeployed,
		NumberComponentErrors:    decoded.NumberComponentErrors,
		NumberTestsTotal:         decoded.NumberTestsTotal,
		NumberTestsCompleted:     decoded.NumberTestsCompleted,
		NumberTestErrors:         decoded.NumberTestErrors,
		ErrorMessage:             decoded.ErrorMessage,
		ErrorStatusCode:          decoded.ErrorStatusCode,
		CanceledByName:           decoded.CanceledByName,
		StateDetail:              decoded.StateDetail,
		Raw:                      raw,
	}
	for _, f := range decoded.Details.ComponentFailures {
		report.ComponentFailures = append(report.ComponentFailures, ComponentFailure{
			Component:    f.FullName,
			Type:         f.ComponentType,
			Message:      f.Problem,
			FileName:     f.FileName,
			LineNumber:   f.LineNumber,
			ColumnNumber: f.ColumnNumber,
			ProblemType:  f.ProblemType,
		})
	}
	for _, f := range decoded.Details.RunTestResult.Failures {
		report.TestFailures = append(report.TestFailures, TestFailure{
			Class:      f.Name,
			Method:     f.MethodName,
			Message:    f.Message,
			StackTrace: f.StackTrace,
		})
	}
	for _, w := range decoded.Details.RunTestResult.CodeCoverageWarnings {
		report.CodeCoverageWarnings = append(report.CodeCoverageWarnings, CodeCoverageWarning{
			Name:      w.Name,
			Namespace: w.Namespace,
			Message:   w.Message,
		})
	}
	for _, c := range decoded.Details.RunTestResult.CodeCoverage {
		report.CodeCoverage = append(report.CodeCoverage, CodeCoverageResult{
			Name:                   c.Name,
			Namespace:              c.Namespace,
			NumLocations:           c.NumLocations,
			NumLocationsNotCovered: c.NumLocationsNotCovered,
		})
	}
	for _, w := range decoded.Details.RunTestResult.FlowCoverageWarnings {
		report.FlowCoverageWarnings = append(report.FlowCoverageWarnings, FlowCoverageWarning{
			FlowName: w.FlowName,
			Message:  w.Message,
		})
	}

	return report, nil
}
