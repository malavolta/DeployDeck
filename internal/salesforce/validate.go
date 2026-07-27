package salesforce

import (
	"context"
	"encoding/json"
	"fmt"

	"deploydeck/internal/exec"
)

// testLevelRunSpecifiedTests is the one sf CLI TestLevel value that carries
// an explicit class list via repeated --tests flags (HU-010 AC: "Dado que
// el usuario elige RunSpecifiedTests, ... se incluye --tests con las clases
// indicadas"). Every other TestLevel (NoTestRun, RunLocalTests,
// RunAllTestsInOrg) never emits --tests, even if req.Tests happens to be
// non-empty.
const testLevelRunSpecifiedTests = "RunSpecifiedTests"

// ValidateRequest is one `sf project deploy validate` invocation's
// parameters.
type ValidateRequest struct {
	// Dir is the repository root the command runs in (the CommandRequest's
	// Dir) — an explicit, already-resolved root, never a trusted process
	// cwd (see design.md's Threat Matrix "Git repo selection" row).
	Dir string
	// ManifestPath is the generated package.xml (HU-007) validated against
	// TargetOrg.
	ManifestPath string
	// PostDestructivePath is the generated destructiveChanges.xml path.
	// When non-empty, --post-destructive-changes is included; when empty
	// (no destructive changes in the delta), the flag is omitted entirely.
	PostDestructivePath string
	// TargetOrg is the resolved Salesforce sandbox alias to validate
	// against.
	TargetOrg string
	// TestLevel is the sf CLI test level (NoTestRun, RunSpecifiedTests,
	// RunLocalTests, RunAllTestsInOrg); its default comes from
	// configuration, adjustable per run.
	TestLevel string
	// Tests lists the Apex test classes to run. Only consumed when
	// TestLevel == "RunSpecifiedTests" — each entry becomes a repeated
	// --tests flag; ignored otherwise.
	Tests []string
}

// ValidateResult is a `sf project deploy validate --async --json` call's
// outcome.
type ValidateResult struct {
	// JobID is the envelope's result.id — the async validation job to poll
	// via ReportDeploy.
	JobID string
	// Raw is the command's captured stdout+stderr, kept for diagnostics and
	// display even when JobID is empty (a CLI error still returns Raw so
	// callers can show "message + raw JSON" per HU-010's AC, not just the
	// decoded error text).
	Raw string
}

// validateResultEnvelope is the shape of the envelope's `result` object for
// a successful `sf project deploy validate --async --json` call; only id is
// consumed today, the rest of the object (done, state, ...) is preserved in
// Raw for display.
type validateResultEnvelope struct {
	ID string `json:"id"`
}

// ValidateDeploy runs `sf project deploy validate --async --json` and
// returns the parsed jobId. A runner-start failure (binary missing, etc.)
// returns a zero ValidateResult and an error — the command never ran, so
// there is no Raw to preserve. A non-zero CLI exit returns a
// ValidateResult carrying Raw (so the raw JSON stays available for display)
// together with an error: the decoded envelope's message when the CLI
// emitted parseable JSON, or the raw stdout+stderr otherwise. Either way
// the flow survives — this never panics and never blocks the caller from
// continuing (HU-010 AC: "se muestra mensaje y JSON raw si existe").
func (c *client) ValidateDeploy(ctx context.Context, req ValidateRequest) (ValidateResult, error) {
	cmdReq := exec.CommandRequest{
		Name: "sf",
		Args: buildValidateArgs(req),
		Dir:  req.Dir,
	}

	result, err := c.runner.Run(ctx, cmdReq)
	if err != nil {
		return ValidateResult{}, fmt.Errorf("salesforce: running sf project deploy validate: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		return ValidateResult{Raw: raw}, fmt.Errorf(
			"salesforce: sf project deploy validate exited %d: %s",
			result.ExitCode, validateErrorMessage(result.Stdout, raw),
		)
	}

	var decoded validateResultEnvelope
	if err := decodeEnvelope(result.Stdout, &decoded); err != nil {
		return ValidateResult{Raw: raw}, err
	}

	return ValidateResult{JobID: decoded.ID, Raw: raw}, nil
}

// buildValidateArgs composes the single validate invocation:
//
//	sf project deploy validate --manifest <ManifestPath>
//	  [--post-destructive-changes <PostDestructivePath>]
//	  --target-org <TargetOrg> --test-level <TestLevel>
//	  [--tests <class>]... (only when TestLevel == RunSpecifiedTests)
//	  --async --json
func buildValidateArgs(req ValidateRequest) []string {
	args := []string{
		"project", "deploy", "validate",
		"--manifest", req.ManifestPath,
	}
	if req.PostDestructivePath != "" {
		args = append(args, "--post-destructive-changes", req.PostDestructivePath)
	}
	args = append(args,
		"--target-org", req.TargetOrg,
		"--test-level", req.TestLevel,
	)
	if req.TestLevel == testLevelRunSpecifiedTests {
		for _, class := range req.Tests {
			args = append(args, "--tests", class)
		}
	}
	args = append(args, "--async", "--json")
	return args
}

// validateErrorMessage extracts the sf CLI's error envelope `message` field
// from stdout (the oclif --json error shape is
// {"status":1,"name":...,"message":...,"exitCode":...}, distinct from the
// success envelope's {"status":0,"result":...}). When stdout is empty or
// not parseable JSON with a non-empty message, it falls back to raw (the
// combined stdout+stderr) so the caller always gets a message — never a
// silent empty string.
func validateErrorMessage(stdout []byte, raw string) string {
	if len(stdout) > 0 {
		var errEnv struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(stdout, &errEnv); err == nil && errEnv.Message != "" {
			return errEnv.Message
		}
	}
	return raw
}

// combineOutput concatenates stdout and stderr for Raw/error-message
// display, mirroring internal/delta's combineOutput (kept as a private
// duplicate rather than a shared helper: internal/salesforce and
// internal/delta are deliberately separate packages per design.md's
// "delta placement" decision, and this one-function helper does not
// justify a shared dependency between them).
func combineOutput(stdout, stderr []byte) string {
	raw := string(stdout)
	if len(stderr) > 0 {
		if raw != "" {
			raw += "\n"
		}
		raw += string(stderr)
	}
	return raw
}
