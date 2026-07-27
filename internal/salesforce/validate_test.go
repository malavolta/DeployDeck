package salesforce_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/salesforce"
)

func TestClient_ValidateDeploy_ComposesArgs(t *testing.T) {
	tests := []struct {
		name string
		req  salesforce.ValidateRequest
		want []string
	}{
		{
			name: "no destructive, default test level, no tests",
			req: salesforce.ValidateRequest{
				Dir:          "/repo",
				ManifestPath: ".deploydeck/manifest/delta/TICKET-to-UAT/package/package.xml",
				TargetOrg:    "UAT_SANDBOX",
				TestLevel:    "RunLocalTests",
			},
			want: []string{
				"project", "deploy", "validate",
				"--manifest", ".deploydeck/manifest/delta/TICKET-to-UAT/package/package.xml",
				"--target-org", "UAT_SANDBOX",
				"--test-level", "RunLocalTests",
				"--async", "--json",
			},
		},
		{
			name: "with destructive changes",
			req: salesforce.ValidateRequest{
				Dir:                 "/repo",
				ManifestPath:        "package/package.xml",
				PostDestructivePath: "destructiveChanges/destructiveChanges.xml",
				TargetOrg:           "UAT_SANDBOX",
				TestLevel:           "RunLocalTests",
			},
			want: []string{
				"project", "deploy", "validate",
				"--manifest", "package/package.xml",
				"--post-destructive-changes", "destructiveChanges/destructiveChanges.xml",
				"--target-org", "UAT_SANDBOX",
				"--test-level", "RunLocalTests",
				"--async", "--json",
			},
		},
		{
			name: "RunSpecifiedTests includes repeated --tests",
			req: salesforce.ValidateRequest{
				Dir:          "/repo",
				ManifestPath: "package/package.xml",
				TargetOrg:    "UAT_SANDBOX",
				TestLevel:    "RunSpecifiedTests",
				Tests:        []string{"MyClassTest", "OtherClassTest"},
			},
			want: []string{
				"project", "deploy", "validate",
				"--manifest", "package/package.xml",
				"--target-org", "UAT_SANDBOX",
				"--test-level", "RunSpecifiedTests",
				"--tests", "MyClassTest",
				"--tests", "OtherClassTest",
				"--async", "--json",
			},
		},
		{
			name: "TestLevel other than RunSpecifiedTests never emits --tests even if Tests is set",
			req: salesforce.ValidateRequest{
				Dir:          "/repo",
				ManifestPath: "package/package.xml",
				TargetOrg:    "UAT_SANDBOX",
				TestLevel:    "NoTestRun",
				Tests:        []string{"IgnoredTest"},
			},
			want: []string{
				"project", "deploy", "validate",
				"--manifest", "package/package.xml",
				"--target-org", "UAT_SANDBOX",
				"--test-level", "NoTestRun",
				"--async", "--json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := exec.NewFakeRunner()
			fr.When("sf", tt.want, exec.CommandResult{
				ExitCode: 0,
				Stdout:   []byte(`{"status":0,"result":{"id":"0Af000000000001EAA","done":false}}`),
			})
			client := salesforce.New(fr)

			got, err := client.ValidateDeploy(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.JobID != "0Af000000000001EAA" {
				t.Fatalf("expected JobID %q, got %q", "0Af000000000001EAA", got.JobID)
			}

			if len(fr.Calls) != 1 {
				t.Fatalf("expected exactly 1 call, got %d", len(fr.Calls))
			}
			call := fr.Calls[0]
			if call.Dir != tt.req.Dir {
				t.Fatalf("expected Dir %q, got %q", tt.req.Dir, call.Dir)
			}
		})
	}
}

func TestClient_ValidateDeploy_JobIDFromEnvelopeResultID(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "validate",
		"--manifest", "package/package.xml",
		"--target-org", "UAT_SANDBOX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"id":"0Af000000000002EAA","done":false,"state":"Queued"}}`),
	})
	client := salesforce.New(fr)

	got, err := client.ValidateDeploy(context.Background(), salesforce.ValidateRequest{
		Dir:          "/repo",
		ManifestPath: "package/package.xml",
		TargetOrg:    "UAT_SANDBOX",
		TestLevel:    "RunLocalTests",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.JobID != "0Af000000000002EAA" {
		t.Fatalf("expected JobID %q, got %q", "0Af000000000002EAA", got.JobID)
	}
	if !strings.Contains(got.Raw, "0Af000000000002EAA") {
		t.Fatalf("expected Raw to preserve the full stdout, got %q", got.Raw)
	}
}

func TestClient_ValidateDeploy_CLIErrorSurfacesDecodedMessage(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "validate",
		"--manifest", "package/package.xml",
		"--target-org", "UAT_SANDBOX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(`{"status":1,"name":"InvalidManifest","message":"package.xml is malformed","exitCode":1}`),
	})
	client := salesforce.New(fr)

	got, err := client.ValidateDeploy(context.Background(), salesforce.ValidateRequest{
		Dir:          "/repo",
		ManifestPath: "package/package.xml",
		TargetOrg:    "UAT_SANDBOX",
		TestLevel:    "RunLocalTests",
	})
	if err == nil {
		t.Fatal("expected an error for a non-zero exit code")
	}
	if !strings.Contains(err.Error(), "package.xml is malformed") {
		t.Fatalf("expected error to surface the decoded message, got: %v", err)
	}
	if got.JobID != "" {
		t.Fatalf("expected empty JobID on error, got %q", got.JobID)
	}
	if !strings.Contains(got.Raw, "InvalidManifest") {
		t.Fatalf("expected Raw to be preserved on error, got %q", got.Raw)
	}
}

func TestClient_ValidateDeploy_CLIErrorSurfacesRawWhenUnparseable(t *testing.T) {
	fr := exec.NewFakeRunner()
	args := []string{
		"project", "deploy", "validate",
		"--manifest", "package/package.xml",
		"--target-org", "UAT_SANDBOX",
		"--test-level", "RunLocalTests",
		"--async", "--json",
	}
	fr.When("sf", args, exec.CommandResult{
		ExitCode: 1,
		Stdout:   []byte(""),
		Stderr:   []byte("sf: command not found in PATH context\n"),
	})
	client := salesforce.New(fr)

	got, err := client.ValidateDeploy(context.Background(), salesforce.ValidateRequest{
		Dir:          "/repo",
		ManifestPath: "package/package.xml",
		TargetOrg:    "UAT_SANDBOX",
		TestLevel:    "RunLocalTests",
	})
	if err == nil {
		t.Fatal("expected an error for a non-zero exit code")
	}
	if !strings.Contains(err.Error(), "command not found in PATH context") {
		t.Fatalf("expected error to surface raw stderr when stdout is unparseable, got: %v", err)
	}
	if !strings.Contains(got.Raw, "command not found in PATH context") {
		t.Fatalf("expected Raw to preserve the raw output on error, got %q", got.Raw)
	}
}

func TestClient_ValidateDeploy_RunnerErrorDoesNotPanicAndFlowSurvives(t *testing.T) {
	fr := exec.NewFakeRunner() // no When() registered — Run returns an explicit error
	client := salesforce.New(fr)

	_, err := client.ValidateDeploy(context.Background(), salesforce.ValidateRequest{
		Dir:          "/repo",
		ManifestPath: "package/package.xml",
		TargetOrg:    "UAT_SANDBOX",
		TestLevel:    "RunLocalTests",
	})
	if err == nil {
		t.Fatal("expected an error when the runner itself fails to start the command")
	}
}
