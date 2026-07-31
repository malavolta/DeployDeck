package exec_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// TestOSRunner_FeedsRequestStdinToChildProcess proves CommandRequest.Stdin
// is actually piped into the child process, not merely accepted and
// ignored. This is required by internal/git's content-equivalence
// detection: `git show <sha> | git patch-id --stable` needs the first
// command's output fed as the second command's stdin.
func TestOSRunner_FeedsRequestStdinToChildProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	result, err := runner.Run(context.Background(), exec.CommandRequest{
		Name:  "cat",
		Dir:   t.TempDir(),
		Stdin: []byte("piped-deploydeck-content\n"),
	})
	if err != nil {
		t.Fatalf("expected no error running a real command, got: %v", err)
	}

	if got := strings.TrimSpace(string(result.Stdout)); got != "piped-deploydeck-content" {
		t.Fatalf("expected stdout %q (echoed from stdin), got %q", "piped-deploydeck-content", got)
	}
}

// TestOSRunner_NoStdin_DoesNotHang proves a request with no Stdin set
// behaves exactly as before this field existed: no hang waiting on input
// that will never arrive (`cat` with no args and no stdin would otherwise
// block forever on a terminal, but with cmd.Stdin left nil os/exec wires
// it to an already-closed/empty reader, not the parent's real stdin).
func TestOSRunner_NoStdin_DoesNotHang(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	result, err := runner.Run(context.Background(), exec.CommandRequest{
		Name: "cat",
		Dir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected no error running a real command, got: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if len(result.Stdout) != 0 {
		t.Fatalf("expected empty stdout with no Stdin set, got %q", result.Stdout)
	}
}
