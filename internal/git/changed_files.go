package git

import "context"

// ChangedFiles returns the files that differ between from and to (e.g.
// "origin/<target>" and "HEAD"). It resolves dir to its repo root first —
// like every other Service method (see RepoRoot) — so the underlying git
// call always runs with a root-relative Dir, never a trusted caller cwd,
// then delegates to diffNameOnly, the NUL-safe `git diff --name-only -z`
// helper VerifyPromotedContent already relies on.
// internal/delta.Summarize's OutsideSourceDirs check is this method's first
// consumer.
func (s *Service) ChangedFiles(ctx context.Context, dir, from, to string) ([]string, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}
	return s.diffNameOnly(ctx, root, from, to, nil)
}
