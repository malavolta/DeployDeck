package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// CancelResult is a `sf project deploy cancel --json` call's outcome.
// Status/CanceledByName are decoded from a successful response (D6,
// deploy-error-detail); Raw always preserves the verbatim stdout+stderr for
// persistence (cancel.json, run-persistence spec) and diagnostics on both
// success and failure.
type CancelResult struct {
	// Status is the job's status after cancellation (e.g. "Canceled").
	Status string
	// CanceledByName is who canceled the job.
	CanceledByName string
	// Raw is the command's captured stdout+stderr, kept for persistence and
	// display on both success and failure.
	Raw string
}

// ErrCancelAlreadyTerminal is the sentinel CancelDeploy returns when the sf
// CLI's error envelope `name` indicates the job already reached a terminal
// state on Salesforce before the cancel request landed — a friendly,
// non-fatal outcome, not a real failure (D6, mirrors the ErrQueuePermission
// idiom). Unverified against a real terminal-job cancel (design.md Open
// Questions): deliberately exact-name; any other name stays a generic
// error, never swallowed.
var ErrCancelAlreadyTerminal = errors.New("salesforce: deploy job already terminal; nothing to cancel")

// alreadyTerminalCancelNames are the sf CLI error-envelope `name` values
// that indicate the already-terminal outcome (D6): CannotCancelDeployPre
// (checked before the cancel call reaches Salesforce) and CannotCancelDeploy
// (the raced/in-flight variant). Both mean the same thing from the app's
// point of view.
var alreadyTerminalCancelNames = map[string]bool{
	"CannotCancelDeployPre": true,
	"CannotCancelDeploy":    true,
}

// cancelErrorEnvelope is the oclif error envelope's `name` field, decoded
// only to classify the already-terminal outcome (D6); the message itself is
// still extracted via validateErrorMessage.
type cancelErrorEnvelope struct {
	Name string `json:"name"`
}

// isAlreadyTerminalCancelError reports whether stdout decodes to an oclif
// error envelope whose `name` matches the already-terminal set.
func isAlreadyTerminalCancelError(stdout []byte) bool {
	if len(stdout) == 0 {
		return false
	}
	var env cancelErrorEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return false
	}
	return alreadyTerminalCancelNames[env.Name]
}

// cancelResultEnvelope mirrors the sf CLI's `deploy cancel --json` success
// result object shape; only status/canceledByName are consumed, the rest is
// preserved verbatim in Raw.
type cancelResultEnvelope struct {
	Status         string `json:"status"`
	CanceledByName string `json:"canceledByName"`
}

// CancelDeploy runs `sf project deploy cancel --job-id <jobID> --target-org
// <targetOrg> --json` and returns the raw response. jobID and targetOrg are
// passed as DISCRETE slice elements (never shell-joined/interpolated —
// design.md Threat Matrix "PR / argument composition"); jobID is always the
// current run's own job, never user-typed. dir is the SFDX project root the
// command runs in, threaded ONLY into CommandRequest.Dir, never appended to
// Args (design ADR-8). Error handling mirrors ValidateDeploy: a runner-start
// failure (binary missing, etc.) returns a zero CancelResult and an error —
// the command never ran, so there is no Raw to preserve. A non-zero CLI exit
// returns a CancelResult carrying Raw (so the raw JSON stays available for
// display and the failed-cancel screen) together with an error: the decoded
// envelope's message when the CLI emitted parseable JSON, or the raw
// stdout+stderr otherwise. The flow survives either way — this never
// panics (HU-012 AC: "un cancel fallido muestra error y no marca el run").
func (c *client) CancelDeploy(ctx context.Context, jobID, targetOrg, dir string) (CancelResult, error) {
	if err := requireProjectDir("sf project deploy cancel", dir); err != nil {
		return CancelResult{}, err
	}

	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{
			"project", "deploy", "cancel",
			"--job-id", jobID,
			"--target-org", targetOrg,
			"--json",
		},
		Dir: dir,
	})
	if err != nil {
		return CancelResult{}, fmt.Errorf("salesforce: running sf project deploy cancel: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		msg := validateErrorMessage(result.Stdout, raw)
		if isAlreadyTerminalCancelError(result.Stdout) {
			return CancelResult{Raw: raw}, fmt.Errorf("%w: %s", ErrCancelAlreadyTerminal, msg)
		}
		return CancelResult{Raw: raw}, fmt.Errorf(
			"salesforce: sf project deploy cancel exited %d: %s",
			result.ExitCode, msg,
		)
	}

	var decoded cancelResultEnvelope
	if err := decodeEnvelope(result.Stdout, &decoded); err != nil {
		return CancelResult{Raw: raw}, err
	}

	return CancelResult{
		Status:         decoded.Status,
		CanceledByName: decoded.CanceledByName,
		Raw:            raw,
	}, nil
}
