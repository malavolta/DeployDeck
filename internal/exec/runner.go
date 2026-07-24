// Package exec is the sole external-command execution boundary for
// deploydeck. internal/git, internal/salesforce and internal/prereq compose
// it through the Runner interface; internal/app (the TUI) never execs
// directly. No git-specific environment (GIT_EDITOR, GIT_TERMINAL_PROMPT,
// GIT_PAGER, etc.) lives here — that is layered by internal/git's request
// builder, never globally.
package exec

import (
	"context"
	"time"
)

// CommandRequest describes an external command to run. Args are passed as a
// slice, never through a shell. Env entries are layered on top of the
// inherited process environment by the OS-backed Runner implementation —
// never a bare replace.
type CommandRequest struct {
	Name       string
	Args       []string
	Dir        string
	Timeout    time.Duration
	Env        []string
	RedactArgs []string
	// Stdin, when non-nil, is piped into the child process's standard
	// input. This is needed for commands composed as a shell pipeline
	// elsewhere (e.g. `git show <sha> | git patch-id --stable`), which
	// internal/exec never runs via an actual shell: the caller runs the
	// first command, captures its Stdout, and passes it as Stdin to the
	// second CommandRequest. Nil (the zero value) preserves prior
	// behavior exactly: no stdin wired to the child.
	Stdin []byte
}

// CommandResult captures the outcome of a command that actually ran,
// including a non-zero exit. See the Runner doc comment for the
// non-zero-exit-as-data contract.
type CommandResult struct {
	Name      string
	Args      []string
	ExitCode  int
	Stdout    []byte
	Stderr    []byte
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
}

// Runner executes external commands.
//
// Non-zero-exit-as-data contract: a process that RAN but exited non-zero is
// NOT a Runner error — CommandResult.ExitCode is populated and the error
// return is nil. This is distinguishable from a start failure (binary
// missing/not executable → non-nil error, ExitCode == -1) and from a
// timeout/context-cancel (non-nil error with ctx.Err() set). Callers that
// use git exit codes as data (merge-base --is-ancestor, cherry-pick,
// diff --quiet, log --grep) interpret CommandResult per-command; they never
// treat a non-nil error from those semantics as a Runner failure.
type Runner interface {
	Run(ctx context.Context, req CommandRequest) (CommandResult, error)
}
