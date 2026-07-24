package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrPromotionBranchExists signals CreatePromotionBranch found branchName
// already resolving locally or on origin — discovered via the SAME
// Service.BranchExists primitive HU-004 uses, run AFTER the mandatory
// fetch so an origin-only collision pushed after the last fetch is still
// caught (HU-005 Existing-Temp-Branch-Collision-Handling requirement:
// specs/promotion-branch/spec.md — "se pide accion al usuario"). Creation
// stops before any git state changes so the caller (internal/app, Phase
// 11) can prompt the user for an action — overwrite, rename, or reuse the
// existing branch — rather than this layer silently picking one.
var ErrPromotionBranchExists = errors.New("git: promotion branch already exists")

// CreatePromotionBranch creates branchName from the freshly fetched
// origin/<target> (HU-005 AC: "Dado un target valido, cuando se crea la
// rama, entonces parte de origin/<target>"). It ALWAYS runs
// `git fetch origin` FIRST (Fetch-Before-Branch-Creation-Ordering
// requirement) and resolves the base ref as the literal "origin/<target>"
// string, letting git itself resolve that ref AT CHECKOUT TIME — never a
// SHA captured before the fetch. This is the critical correctness property
// this HU exists for (Branch-Base-From-Fetched-Remote requirement): if
// origin/<target> advanced after the local clone/last fetch, and only this
// fresh fetch retrieves it, the new branch follows the ADVANCED tip, not a
// stale local tracking ref.
//
// It stops WITHOUT creating anything when:
//   - the fetch itself fails (Fetch-Failure-Halts-Flow requirement): the
//     current branch is left completely unchanged, no checkout runs.
//   - branchName already exists locally or on origin: ErrPromotionBranchExists
//     is returned (see its doc comment).
func (s *Service) CreatePromotionBranch(ctx context.Context, dir, target, branchName string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}

	if err := s.fetchOrigin(ctx, root); err != nil {
		return err
	}

	exists, err := s.BranchExists(ctx, root, branchName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w: %q", ErrPromotionBranchExists, branchName)
	}

	baseRef := "origin/" + target
	req := newRequest(root, "checkout", "-b", branchName, baseRef)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: creating promotion branch %q from %s in %s: %w", branchName, baseRef, root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: creating promotion branch %q from %s in %s: %s", branchName, baseRef, root, strings.TrimSpace(string(result.Stderr)))
	}

	return nil
}

// fetchOrigin runs `git fetch origin` in root, the ONLY place
// CreatePromotionBranch fetches from — every call site funnels through
// here so the fetch-before-branch-creation ordering holds by construction.
func (s *Service) fetchOrigin(ctx context.Context, root string) error {
	req := newRequest(root, "fetch", "origin")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: fetching origin in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: fetching origin in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}
