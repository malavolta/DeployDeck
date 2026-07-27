package salesforce

import (
	"context"
	"fmt"
	"strings"

	"deploydeck/internal/exec"
)

// ComponentFailure is one metadata deploy error (HU-011 AC: "se muestran
// con componente, tipo y mensaje"), decoded from the report's
// details.componentFailures entries (fullName, componentType, problem).
type ComponentFailure struct {
	Component string
	Type      string
	Message   string
}

// TestFailure is one failed Apex test (HU-011 AC: "se muestran ... con
// clase, metodo y mensaje"), decoded from the report's
// details.runTestResult.failures entries (name, methodName, message).
type TestFailure struct {
	Class   string
	Method  string
	Message string
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

	ComponentFailures []ComponentFailure
	TestFailures      []TestFailure

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

	Details struct {
		ComponentFailures []struct {
			FullName      string `json:"fullName"`
			ComponentType string `json:"componentType"`
			Problem       string `json:"problem"`
		} `json:"componentFailures"`
		RunTestResult struct {
			Failures []struct {
				Name       string `json:"name"`
				MethodName string `json:"methodName"`
				Message    string `json:"message"`
			} `json:"failures"`
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
// per tick; no blocking loop lives in this package). A non-zero CLI exit
// returns a DeployReport carrying Raw together with an error describing
// the failure, so callers can still show the raw output while deciding
// whether the failure is transient (retryable within the poll's deadline)
// or terminal.
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
	if result.ExitCode != 0 {
		return DeployReport{Raw: raw}, fmt.Errorf(
			"salesforce: sf project deploy report exited %d: %s",
			result.ExitCode, strings.TrimSpace(string(result.Stderr)),
		)
	}

	var decoded reportResultEnvelope
	if err := decodeEnvelope(result.Stdout, &decoded); err != nil {
		return DeployReport{Raw: raw}, err
	}

	report := DeployReport{
		Status:                   decoded.Status,
		NumberComponentsTotal:    decoded.NumberComponentsTotal,
		NumberComponentsDeployed: decoded.NumberComponentsDeployed,
		NumberComponentErrors:    decoded.NumberComponentErrors,
		NumberTestsTotal:         decoded.NumberTestsTotal,
		NumberTestsCompleted:     decoded.NumberTestsCompleted,
		NumberTestErrors:         decoded.NumberTestErrors,
		Raw:                      raw,
	}
	for _, f := range decoded.Details.ComponentFailures {
		report.ComponentFailures = append(report.ComponentFailures, ComponentFailure{
			Component: f.FullName,
			Type:      f.ComponentType,
			Message:   f.Problem,
		})
	}
	for _, f := range decoded.Details.RunTestResult.Failures {
		report.TestFailures = append(report.TestFailures, TestFailure{
			Class:   f.Name,
			Method:  f.MethodName,
			Message: f.Message,
		})
	}

	return report, nil
}
