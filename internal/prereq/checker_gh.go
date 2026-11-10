package prereq

import (
	"context"
	"time"

	"github.com/malavolta/DeployDeck/internal/github"
)

// defaultGHTimeout bounds the gh auth probe when Checker.GHTimeout is zero.
const defaultGHTimeout = 5 * time.Second

// CheckGH reports gh CLI availability/authentication as an informative,
// NON-BLOCKING PrereqCheck (HU-014, closing the "the gh check is
// intentionally deferred to HU-014" note in HU-001's Design Notes):
// absent, present-unauthenticated, and present-authenticated are all
// reported without ever using StatusBlocking — gh is an optional
// dependency, needed only to create a PR directly from the TUI; the
// compare-URL fallback needs no gh at all. A nil GH (every Checker built
// before HU-014, and every existing checker test that never sets GH) skips
// this check entirely, reporting OK.
func (c *Checker) CheckGH(ctx context.Context) (PrereqCheck, error) {
	if c.GH == nil {
		return PrereqCheck{Name: "gh CLI", Status: StatusOK, Detail: "gh check skipped (no gh client configured)"}, nil
	}

	timeout := c.GHTimeout
	if timeout <= 0 {
		timeout = defaultGHTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Scope the probe to the origin host so an unreachable extra host cannot
	// stall or flip the result; no origin (or Git) falls back to host-less.
	host := ""
	if c.Git != nil {
		if origin, err := c.Git.RemoteURL(ctx, c.GitRoot, "origin"); err == nil {
			host, _ = github.OriginHost(origin)
		}
	}

	const consequences = "PR creation falls back to the compare URL, the deploy gate and the validation comment are unavailable, push still works"
	switch c.GH.AuthStatus(ctx, host) {
	case github.AuthAuthenticated:
		return PrereqCheck{Name: "gh CLI", Status: StatusOK, Detail: "gh is installed and authenticated"}, nil
	case github.AuthUnauthenticated:
		return PrereqCheck{
			Name:       "gh CLI",
			Status:     StatusWarning,
			Detail:     "gh is installed but not authenticated; " + consequences,
			FixCommand: "gh auth login",
		}, nil
	case github.AuthTimedOut:
		return PrereqCheck{
			Name:       "gh CLI",
			Status:     StatusWarning,
			Detail:     "could not verify gh authentication (timed out); " + consequences,
			FixCommand: "gh auth status",
		}, nil
	default: // github.AuthAbsent
		return PrereqCheck{
			Name:       "gh CLI",
			Status:     StatusWarning,
			Detail:     "gh is not installed; " + consequences,
			FixCommand: "https://cli.github.com/",
		}, nil
	}
}
