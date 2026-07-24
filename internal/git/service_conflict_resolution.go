package git

import (
	"context"
	"fmt"
	"strings"

	"deploydeck/internal/exec"
)

// ResolutionSide selects which version of a binary conflict to keep.
type ResolutionSide int

const (
	// SideTheirs keeps the incoming commit's version (`git checkout --theirs`).
	SideTheirs ResolutionSide = iota
	// SideOurs keeps the current branch's version (`git checkout --ours`).
	SideOurs
)

// KeepConflictFile resolves a modify/delete conflict by KEEPING the file:
// it stages the working-tree version with `git add <path>` (HU-006
// modify/delete resolution — the explicit "conservar" choice).
func (s *Service) KeepConflictFile(ctx context.Context, dir, path string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}
	req := newRequest(root, "add", "--", path)
	return s.runResolution(ctx, req, "keeping", path, root)
}

// DeleteConflictFile resolves a modify/delete conflict by DELETING the file:
// it stages the removal with `git rm <path>` (HU-006 modify/delete
// resolution — the explicit "borrar" choice).
func (s *Service) DeleteConflictFile(ctx context.Context, dir, path string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}
	req := newRequest(root, "rm", "--", path)
	return s.runResolution(ctx, req, "deleting", path, root)
}

// ResolveBinaryConflict resolves a binary conflict by selecting one whole
// side with `git checkout --theirs/--ours -- <path>` and then staging it
// with `git add` (HU-006 binary resolution — static resources cannot be
// merged line-by-line, so the user picks a whole version).
func (s *Service) ResolveBinaryConflict(ctx context.Context, dir, path string, side ResolutionSide) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}

	sideFlag := "--theirs"
	if side == SideOurs {
		sideFlag = "--ours"
	}

	checkoutReq := newRequest(root, "checkout", sideFlag, "--", path)
	if err := s.runResolution(ctx, checkoutReq, "checking out "+sideFlag+" of", path, root); err != nil {
		return err
	}

	addReq := newRequest(root, "add", "--", path)
	return s.runResolution(ctx, addReq, "staging", path, root)
}

// runResolution runs a single conflict-resolution git command, mapping a
// non-zero exit to a descriptive error naming the action, path, and root.
func (s *Service) runResolution(ctx context.Context, req exec.CommandRequest, verb, path, root string) error {
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: %s %s in %s: %w", verb, path, root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: %s %s in %s: %s", verb, path, root, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}
