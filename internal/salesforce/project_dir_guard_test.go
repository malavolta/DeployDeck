package salesforce_test

import (
	"context"
	"errors"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// TestProjectDirGuard_EmptyDirIsRejectedBeforeAnyCommandRuns closes the
// verify phase's S-1 finding.
//
// Every `sf project deploy ...` subcommand must run with the SFDX project
// root as its working directory. internal/exec treats an EMPTY
// CommandRequest.Dir as "inherit the process cwd" — precisely the
// conflation the directory-resolution change removes. A caller that omits
// the directory would therefore silently reintroduce the original bug, in
// the one place where it is most expensive: a real deployment running
// against whatever directory the operator happened to launch from.
//
// The guard makes that unrepresentable: an empty dir is rejected up front,
// with NO command executed, so the failure is loud and local instead of
// silent and remote.
func TestProjectDirGuard_EmptyDirIsRejectedBeforeAnyCommandRuns(t *testing.T) {
	ctx := context.Background()

	// A runner with NO canned responses: reaching it at all is the failure
	// this test is about, and an unmatched request would also error loudly.
	// The guard must reject before the CLI is reached, not after.
	spy := exec.NewFakeRunner()
	guard := func(t *testing.T) {
		t.Helper()
		if len(spy.Calls) != 0 {
			t.Fatalf("guard did not short-circuit: %d command(s) executed with an empty dir: %+v", len(spy.Calls), spy.Calls)
		}
	}

	tests := []struct {
		name string
		call func(salesforce.Client) error
	}{
		{
			name: "ValidateDeploy",
			call: func(c salesforce.Client) error {
				_, err := c.ValidateDeploy(ctx, salesforce.ValidateRequest{
					Dir:          "",
					ManifestPath: "manifest/package.xml",
					TargetOrg:    "SANDBOX",
					TestLevel:    "RunLocalTests",
				})
				return err
			},
		},
		{
			name: "ReportDeploy",
			call: func(c salesforce.Client) error {
				_, err := c.ReportDeploy(ctx, "0Af000000000001", "SANDBOX", "")
				return err
			},
		},
		{
			name: "QuickDeploy",
			call: func(c salesforce.Client) error {
				_, err := c.QuickDeploy(ctx, "0Af000000000001", "SANDBOX", "")
				return err
			},
		},
		{
			name: "CancelDeploy",
			call: func(c salesforce.Client) error {
				_, err := c.CancelDeploy(ctx, "0Af000000000001", "SANDBOX", "")
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spy.Calls = nil
			client := salesforce.New(spy)

			err := tc.call(client)
			if err == nil {
				t.Fatalf("%s with an empty dir returned nil error; want ErrMissingProjectDir", tc.name)
			}
			if !errors.Is(err, salesforce.ErrMissingProjectDir) {
				t.Errorf("%s error = %v; want it to wrap ErrMissingProjectDir so callers can match it", tc.name, err)
			}
			guard(t)
		})
	}
}

// TestProjectDirGuard_NonEmptyDirStillRuns is the companion that stops the
// guard from being a blanket refusal: a real directory must still reach the
// CLI unchanged, threaded into CommandRequest.Dir and never into Args.
func TestProjectDirGuard_NonEmptyDirStillRuns(t *testing.T) {
	ctx := context.Background()
	const projectDir = "/repo/up_saln0001_giss_salesforce"

	runner := exec.NewFakeRunner()
	runner.When("sf", []string{
		"project", "deploy", "report",
		"--job-id", "0Af000000000001",
		"--target-org", "SANDBOX",
		"--json",
	}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})
	client := salesforce.New(runner)

	if _, err := client.ReportDeploy(ctx, "0Af000000000001", "SANDBOX", projectDir); err != nil {
		t.Fatalf("ReportDeploy with a real dir: %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected exactly 1 command, got %d", len(runner.Calls))
	}
	call := runner.Calls[0]
	if call.Dir != projectDir {
		t.Errorf("CommandRequest.Dir = %q, want the project root %q", call.Dir, projectDir)
	}
	for _, arg := range call.Args {
		if arg == projectDir {
			t.Errorf("project dir leaked into Args %v — it belongs only in CommandRequest.Dir (ADR-8)", call.Args)
		}
	}
}
