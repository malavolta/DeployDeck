package exec

import (
	"bytes"
	"context"
	"errors"
	"os"
	osexec "os/exec"
	"time"
)

// OSRunner is the real, process-backed Runner implementation. It wraps
// os/exec.CommandContext.
type OSRunner struct{}

// NewOSRunner returns a Runner backed by real OS processes.
func NewOSRunner() *OSRunner {
	return &OSRunner{}
}

// Run executes req as a real OS process. See Runner's doc comment for the
// non-zero-exit-as-data contract this implementation upholds.
func (r *OSRunner) Run(ctx context.Context, req CommandRequest) (CommandResult, error) {
	cmd := osexec.CommandContext(ctx, req.Name, req.Args...)
	cmd.Dir = req.Dir

	// Layer req.Env ON TOP of the inherited process environment. Go's
	// os/exec REPLACES the whole environment whenever cmd.Env != nil, so a
	// bare `cmd.Env = req.Env` would strip inherited PATH/HOME/
	// SSH_AUTH_SOCK/GIT_* and break commands that rely on the parent
	// environment (e.g. `git fetch` over SSH) while unit tests using
	// FakeRunner stayed green. Append instead of assign.
	if len(req.Env) > 0 {
		cmd.Env = append(os.Environ(), req.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now()
	err := cmd.Run()
	endedAt := time.Now()

	result := CommandResult{
		Name:      req.Name,
		Args:      req.Args,
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Duration:  endedAt.Sub(startedAt),
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	// Timeout/cancel takes priority: the caller distinguishes this case via
	// ctx.Err() on the context it passed in, regardless of what the
	// underlying process error looked like (killed-by-signal often surfaces
	// as an *exec.ExitError too, but it must NOT be reported as a clean
	// non-zero exit here).
	if ctx.Err() != nil {
		result.ExitCode = -1
		return result, err
	}

	// Non-zero-exit-as-data: the process RAN and exited non-zero. This is
	// NOT a Runner error — populate ExitCode from *exec.ExitError and
	// return a nil error so callers read git/sf exit codes as data
	// (merge-base --is-ancestor, cherry-pick, diff --quiet, log --grep).
	var exitErr *osexec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	// Anything else is a start failure (missing/non-executable binary, etc).
	result.ExitCode = -1
	return result, err
}
