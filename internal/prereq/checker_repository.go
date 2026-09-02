package prereq

import (
	"context"
	"fmt"
)

// CheckRepository validates that GitRoot belongs to a Git repository and
// that an `origin` remote is configured (HU-001: "Repository Membership And
// Remote Validation"). When GitRoot is not inside a repository, only the
// repository-membership check is returned — origin cannot be evaluated.
func (c *Checker) CheckRepository(ctx context.Context) ([]PrereqCheck, error) {
	root, err := c.Git.RepoRoot(ctx, c.GitRoot)
	if err != nil {
		return []PrereqCheck{
			{
				Name:       "git repository",
				Status:     StatusBlocking,
				Detail:     fmt.Sprintf("%s is not inside a git repository", c.GitRoot),
				FixCommand: "git init  # or run deploydeck from inside an existing repository",
			},
		}, nil
	}

	checks := []PrereqCheck{
		{Name: "git repository", Status: StatusOK, Detail: fmt.Sprintf("repository root: %s", root)},
	}

	hasOrigin, err := c.Git.HasRemote(ctx, root, "origin")
	if err != nil {
		return nil, fmt.Errorf("prereq: checking origin remote: %w", err)
	}
	if !hasOrigin {
		checks = append(checks, PrereqCheck{
			Name:       "origin remote",
			Status:     StatusBlocking,
			Detail:     "no origin remote is configured for this repository",
			FixCommand: "git remote add origin <url>",
		})
	} else {
		checks = append(checks, PrereqCheck{Name: "origin remote", Status: StatusOK, Detail: "origin remote is configured"})
	}

	return checks, nil
}
