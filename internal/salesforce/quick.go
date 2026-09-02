package salesforce

import (
	"context"
	"fmt"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// QuickDeployResult is a `sf project deploy quick --json` call's outcome.
// Status is decoded from a successful response (deploy-error-detail gap 7);
// Raw always preserves the verbatim stdout+stderr — the caller persists it
// as quick.json (run-persistence spec) and shows it for diagnostics on both
// success and failure.
type QuickDeployResult struct {
	// Status is the job's status after the quick deploy (e.g. "Succeeded").
	Status string
	// Raw is the command's captured stdout+stderr, kept for persistence and
	// display on both success and failure.
	Raw string
}

// quickDeployResultEnvelope mirrors the sf CLI's `deploy quick --json`
// success result object shape; only status is consumed, the rest is
// preserved verbatim in Raw.
type quickDeployResultEnvelope struct {
	Status string `json:"status"`
}

// QuickDeploy runs `sf project deploy quick --job-id <jobID> --target-org
// <targetOrg> --json` and returns the raw response. jobID and targetOrg are
// passed as DISCRETE slice elements (never shell-joined/interpolated —
// design.md Threat Matrix "PR / argument composition"); jobID is always the
// eligible run's own job, never user-typed. dir is the SFDX project root
// the command runs in, threaded ONLY into CommandRequest.Dir, never
// appended to Args (design ADR-8). Error handling mirrors CancelDeploy: a
// runner-start failure (binary missing, etc.) returns a zero
// QuickDeployResult and an error — the command never ran, so there is no Raw
// to preserve. A non-zero CLI exit returns a QuickDeployResult carrying Raw
// (so the raw JSON stays available for display) together with an error: the
// decoded envelope's message when the CLI emitted parseable JSON, or the raw
// stdout+stderr otherwise. The flow survives either way — this never panics.
func (c *client) QuickDeploy(ctx context.Context, jobID, targetOrg, dir string) (QuickDeployResult, error) {
	if err := requireProjectDir("sf project deploy quick", dir); err != nil {
		return QuickDeployResult{}, err
	}

	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{
			"project", "deploy", "quick",
			"--job-id", jobID,
			"--target-org", targetOrg,
			"--json",
		},
		Dir: dir,
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

	var decoded quickDeployResultEnvelope
	if err := decodeEnvelope(result.Stdout, &decoded); err != nil {
		return QuickDeployResult{Raw: raw}, err
	}

	return QuickDeployResult{Status: decoded.Status, Raw: raw}, nil
}
