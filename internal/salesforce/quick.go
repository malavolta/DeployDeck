package salesforce

import (
	"context"
	"fmt"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// QuickDeployResult is a `sf project deploy quick --json` call's outcome.
// Only the raw stdout+stderr is preserved (Raw): the caller persists it
// verbatim as quick.json (run-persistence spec) and shows it for
// diagnostics — no field is decoded out of the response body, so a shape
// change in the CLI's quick-deploy output never breaks this call.
type QuickDeployResult struct {
	// Raw is the command's captured stdout+stderr, kept for persistence and
	// display on both success and failure.
	Raw string
}

// QuickDeploy runs `sf project deploy quick --job-id <jobID> --target-org
// <targetOrg> --json` and returns the raw response. jobID and targetOrg are
// passed as DISCRETE slice elements (never shell-joined/interpolated —
// design.md Threat Matrix "PR / argument composition"); jobID is always the
// eligible run's own job, never user-typed. Error handling mirrors
// CancelDeploy: a runner-start failure (binary missing, etc.) returns a zero
// QuickDeployResult and an error — the command never ran, so there is no Raw
// to preserve. A non-zero CLI exit returns a QuickDeployResult carrying Raw
// (so the raw JSON stays available for display) together with an error: the
// decoded envelope's message when the CLI emitted parseable JSON, or the raw
// stdout+stderr otherwise. The flow survives either way — this never panics.
func (c *client) QuickDeploy(ctx context.Context, jobID, targetOrg string) (QuickDeployResult, error) {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{
			"project", "deploy", "quick",
			"--job-id", jobID,
			"--target-org", targetOrg,
			"--json",
		},
	})
	if err != nil {
		return QuickDeployResult{}, fmt.Errorf("salesforce: running sf project deploy quick: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		return QuickDeployResult{Raw: raw}, fmt.Errorf(
			"salesforce: sf project deploy quick exited %d: %s",
			result.ExitCode, validateErrorMessage(result.Stdout, raw),
		)
	}

	return QuickDeployResult{Raw: raw}, nil
}
