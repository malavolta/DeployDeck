package exec_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
)

func TestOSRunner_RunsRealCommandAndCapturesStdoutExitDuration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	result, err := runner.Run(context.Background(), exec.CommandRequest{
		Name: "echo",
		Args: []string{"hello-deploydeck"},
		Dir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected no error running a real command, got: %v", err)
	}

	if got := strings.TrimSpace(string(result.Stdout)); got != "hello-deploydeck" {
		t.Fatalf("expected stdout %q, got %q", "hello-deploydeck", got)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Duration <= 0 {
		t.Fatalf("expected a positive Duration, got %v", result.Duration)
	}
}
