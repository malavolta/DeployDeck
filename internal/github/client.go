package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// AuthState classifies gh's availability/authentication, derived from ONE
// `gh auth status` invocation (design.md's "gh 3-state via
// non-zero-exit-as-data" decision): a Runner error (the binary is missing
// or cannot even start) means AuthAbsent, a nil error with a non-zero exit
// means AuthUnauthenticated, and a zero exit means AuthAuthenticated.
type AuthState int

const (
	AuthAbsent AuthState = iota
	AuthUnauthenticated
	AuthAuthenticated
)

// Client is the gh CLI shim: auth-status detection and PR creation.
type Client interface {
	// AuthStatus runs ONE `gh auth status` and classifies the result. It is
	// TOTAL — no error return — because every outcome (missing binary,
	// present-unauthenticated, present-authenticated) is itself a valid,
	// informative AuthState the caller branches on directly, never a
	// failure mode requiring separate error handling.
	AuthStatus(ctx context.Context) AuthState
	// CreatePR runs `gh pr create --base <base> --head <head> --title
	// <title> --body <body>` (fully non-interactive — --body forces it,
	// even when body is "" — args passed as a discrete slice, NEVER
	// through a shell, so a multi-line body is passed through safely). On
	// success it returns the created PR's URL parsed from stdout. Raw (the
	// command's combined stdout+stderr) is preserved on BOTH success and
	// failure so a caller can show it after a failed creation (HU-014
	// AC7). CreatePR NEVER runs unless the caller has already obtained
	// explicit user confirmation — this package enforces the non-shell,
	// non-interactive argument shape; the confirm gate itself lives in the
	// caller (internal/app's StatePushPreparation).
	CreatePR(ctx context.Context, base, head, title, body string) (url, raw string, err error)
	// PRForBranch runs `gh pr view <branch> --json url,state` (incremental-
	// promotion design: "PR detection") to discover whether a PR already
	// exists for branch, exit-code-as-data: a found PR parses url/open from
	// its JSON state (OPEN -> open=true; CLOSED/MERGED -> open=false, url
	// still set — the caller falls through to a normal create + notice); no
	// PR for the branch (gh's own non-zero exit, e.g. "no pull requests
	// found") degrades to url="", open=false, err=nil — never an error, so
	// the caller's normal create path is unaffected. A genuine Runner
	// failure (gh missing/cannot start) still surfaces as err.
	PRForBranch(ctx context.Context, branch string) (url string, open bool, err error)
}

// client is the Runner-backed Client implementation.
type client struct {
	runner exec.Runner
}

// New returns a Client backed by runner.
func New(runner exec.Runner) Client {
	return &client{runner: runner}
}

// AuthStatus implements Client.AuthStatus.
func (c *client) AuthStatus(ctx context.Context) AuthState {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "gh",
		Args: []string{"auth", "status"},
	})
	if err != nil {
		return AuthAbsent
	}
	if result.ExitCode != 0 {
		return AuthUnauthenticated
	}
	return AuthAuthenticated
}

// CreatePR implements Client.CreatePR.
func (c *client) CreatePR(ctx context.Context, base, head, title, body string) (string, string, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "create", "--base", base, "--head", head, "--title", title, "--body", body},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("github: running gh pr create: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		return "", raw, fmt.Errorf("github: gh pr create exited %d: %s", result.ExitCode, raw)
	}

	return strings.TrimSpace(string(result.Stdout)), raw, nil
}

// prView is the shape `gh pr view <branch> --json url,state` prints.
type prView struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// PRForBranch implements Client.PRForBranch.
func (c *client) PRForBranch(ctx context.Context, branch string) (string, bool, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "view", branch, "--json", "url,state"},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return "", false, fmt.Errorf("github: running gh pr view: %w", err)
	}
	if result.ExitCode != 0 {
		// No PR for this branch (or gh itself reported an error) — data, not
		// a failure: the caller degrades to its own normal create path.
		return "", false, nil
	}

	var parsed prView
	if err := json.Unmarshal(result.Stdout, &parsed); err != nil {
		return "", false, fmt.Errorf("github: parsing gh pr view output: %w", err)
	}

	return parsed.URL, parsed.State == "OPEN", nil
}

// combineOutput joins stdout and stderr the same way internal/delta's own
// combineOutput does, so Raw always carries every line gh printed
// regardless of which stream it used.
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

// SuggestedTitle composes HU-014's suggested PR title:
// "<ticket> - Promote changes to <target>" (HU-014 AC4).
func SuggestedTitle(ticket, target string) string {
	return fmt.Sprintf("%s - Promote changes to %s", ticket, target)
}
