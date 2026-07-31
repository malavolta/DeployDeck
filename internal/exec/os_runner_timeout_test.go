package exec_test

import (
	"context"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/exec"
)

func TestOSRunner_ContextTimeoutReturnsErrorWithoutHanging(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	started := time.Now()
	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "sleep",
		Args: []string{"5"},
		Dir:  t.TempDir(),
	})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("expected an error when the context deadline is exceeded, got nil")
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("expected ctx.Err() == context.DeadlineExceeded, got %v", ctx.Err())
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("expected Run to return promptly on timeout, took %v (command would have taken 5s)", elapsed)
	}
	if result.ExitCode == 0 {
		t.Fatalf("expected a non-zero/failure ExitCode on timeout, got 0")
	}
}

func TestOSRunner_ContextCancelReturnsErrorWithoutHanging(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping OS-level integration test in -short mode")
	}

	runner := exec.NewOSRunner()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	started := time.Now()
	_, err := runner.Run(ctx, exec.CommandRequest{
		Name: "sleep",
		Args: []string{"5"},
		Dir:  t.TempDir(),
	})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("expected an error when the context is canceled, got nil")
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("expected ctx.Err() == context.Canceled, got %v", ctx.Err())
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("expected Run to return promptly on cancel, took %v (command would have taken 5s)", elapsed)
	}
}
