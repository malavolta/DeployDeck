package exec_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
)

func TestOSRunner_NonZeroExitIsDataNotRunnerError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	result, err := runner.Run(context.Background(), exec.CommandRequest{
		Name: "false",
		Dir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected a nil Runner error for a process that ran and exited non-zero, got: %v", err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected populated ExitCode 1 for `false`, got %d", result.ExitCode)
	}
}

func TestOSRunner_StartFailureIsDistinctFromNonZeroExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	result, err := runner.Run(context.Background(), exec.CommandRequest{
		Name: "deploydeck-nonexistent-binary-xyz",
		Dir:  t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected a non-nil error for a missing/non-executable binary (start failure), got nil")
	}
	if result.ExitCode != -1 {
		t.Fatalf("expected ExitCode -1 for a start failure, got %d", result.ExitCode)
	}
}
