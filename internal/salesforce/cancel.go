package salesforce

import (
	"context"
	"fmt"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// CancelResult is a `sf project deploy cancel --json` call's outcome. Only the
// raw stdout+stderr is preserved (Raw): the caller persists it verbatim as
// cancel.json (run-persistence spec) and shows it for diagnostics — no field is
// decoded out of the response body, so a shape change in the CLI's cancel
// output never breaks this call.
type CancelResult struct {
	// Raw is the command's captured stdout+stderr, kept for persistence and
	// display on both success and failure.
	Raw string
}

// CancelDeploy runs `sf project deploy cancel --job-id <jobID> --target-org
// <targetOrg> --json` and returns the raw response. jobID and targetOrg are
// passed as DISCRETE slice elements (never shell-joined/interpolated —
// design.md Threat Matrix "PR / argument composition"); jobID is always the
// current run's own job, never user-typed. Error handling mirrors
// ValidateDeploy: a runner-start failure (binary missing, etc.) returns a zero
// CancelResult and an error — the command never ran, so there is no Raw to
// preserve. A non-zero CLI exit returns a CancelResult carrying Raw (so the raw
// JSON stays available for display and the failed-cancel screen) together with
// an error: the decoded envelope's message when the CLI emitted parseable JSON,
// or the raw stdout+stderr otherwise. The flow survives either way — this never
// panics (HU-012 AC: "un cancel fallido muestra error y no marca el run").
func (c *client) CancelDeploy(ctx context.Context, jobID, targetOrg string) (CancelResult, error) {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{
			"project", "deploy", "cancel",
			"--job-id", jobID,
			"--target-org", targetOrg,
			"--json",
		},
	})
	if err != nil {
		return CancelResult{}, fmt.Errorf("salesforce: running sf project deploy cancel: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		return CancelResult{Raw: raw}, fmt.Errorf(
			"salesforce: sf project deploy cancel exited %d: %s",
			result.ExitCode, validateErrorMessage(result.Stdout, raw),
		)
	}

	return CancelResult{Raw: raw}, nil
}
