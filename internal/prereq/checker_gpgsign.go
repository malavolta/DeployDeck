package prereq

import (
	"context"
	"fmt"
)

// CheckGpgSign warns (never blocks) when commit.gpgsign=true, noting that
// DeployDeck neutralizes it with `-c commit.gpgsign=false` on its own
// cherry-pick/--continue commits so a no-tty gpg prompt can never hang a
// run.
func (c *Checker) CheckGpgSign(ctx context.Context) (PrereqCheck, error) {
	value, ok, err := c.Git.ConfigGet(ctx, c.Dir, "commit.gpgsign")
	if err != nil {
		return PrereqCheck{}, fmt.Errorf("prereq: checking commit.gpgsign: %w", err)
	}

	if !ok || value != "true" {
		return PrereqCheck{Name: "commit.gpgsign", Status: StatusOK, Detail: "commit.gpgsign is not enabled"}, nil
	}

	return PrereqCheck{
		Name:   "commit.gpgsign",
		Status: StatusWarning,
		Detail: "commit.gpgsign=true; DeployDeck neutralizes it with -c commit.gpgsign=false on its own cherry-pick/--continue commits so a no-tty gpg prompt can never hang the run",
	}, nil
}
