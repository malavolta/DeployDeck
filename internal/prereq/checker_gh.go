package prereq

import (
	"context"

	"github.com/malavolta/DeployDeck/internal/github"
)

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

	switch c.GH.AuthStatus(ctx) {
	case github.AuthAuthenticated:
		return PrereqCheck{Name: "gh CLI", Status: StatusOK, Detail: "gh is installed and authenticated"}, nil
	case github.AuthUnauthenticated:
		return PrereqCheck{
			Name:       "gh CLI",
			Status:     StatusWarning,
			Detail:     "gh is installed but not authenticated; PR creation from the TUI will be unavailable (the compare URL fallback still works)",
			FixCommand: "gh auth login",
		}, nil
	default: // github.AuthAbsent
		return PrereqCheck{
			Name:       "gh CLI",
			Status:     StatusWarning,
			Detail:     "gh is not installed; PR creation from the TUI will be unavailable (the compare URL fallback still works)",
			FixCommand: "https://cli.github.com/",
		}, nil
	}
}
