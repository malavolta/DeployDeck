package exec_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
)

func TestOSRunner_MergesEnvOntoInheritedProcessEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	result, err := runner.Run(context.Background(), exec.CommandRequest{
		Name: "env",
		Dir:  t.TempDir(),
		Env:  []string{"DEPLOYDECK_TEST_VAR=custom-value"},
	})
	if err != nil {
		t.Fatalf("expected no error running env, got: %v", err)
	}

	output := string(result.Stdout)

	if !strings.Contains(output, "DEPLOYDECK_TEST_VAR=custom-value") {
		t.Fatalf("expected req.Env override to be present in child env, got:\n%s", output)
	}
	if !strings.Contains(output, "PATH=") {
		t.Fatalf("expected inherited PATH to still resolve in child env (env must be merged, not replaced), got:\n%s", output)
	}
}
