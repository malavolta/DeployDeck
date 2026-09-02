package prereq

import (
	"context"
	"fmt"
)

// CheckWorkingTree detects a dirty working tree, blocking branch-modifying
// operations (HU-001: "Working Tree Cleanliness Check").
func (c *Checker) CheckWorkingTree(ctx context.Context) (PrereqCheck, error) {
	state, err := c.Git.Status(ctx, c.GitRoot)
	if err != nil {
		return PrereqCheck{}, fmt.Errorf("prereq: checking working tree: %w", err)
	}

	if !state.Clean {
		return PrereqCheck{
			Name:       "working tree",
			Status:     StatusBlocking,
			Detail:     "the working tree has uncommitted changes; branch-modifying operations are blocked",
			FixCommand: "git status --short  # commit or stash your changes",
		}, nil
	}

	return PrereqCheck{Name: "working tree", Status: StatusOK, Detail: "working tree is clean"}, nil
}
