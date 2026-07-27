package git

import (
	"context"
	"fmt"
	"strings"
)

// Push runs `git push -u origin <branch>` in the repository containing dir
// (HU-014 AC3: "se ejecuta git push -u origin <branch>"). It resolves the
// repository root via RepoRoot first, the same shape every other Service
// method uses, so a caller-provided dir is never trusted directly. -u sets
// the upstream tracking ref on the FIRST push, exactly as HU-014's push
// threat-matrix row requires.
func (s *Service) Push(ctx context.Context, dir, branch string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}

	req := newRequest(root, "push", "-u", "origin", branch)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: pushing %q to origin in %s: %w", branch, root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: pushing %q to origin in %s: %s", branch, root, strings.TrimSpace(string(result.Stderr)))
	}

	return nil
}
