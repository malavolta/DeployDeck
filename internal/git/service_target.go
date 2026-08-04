package git

import (
	"context"
	"fmt"
	"strings"
)

// revParseVerify resolves ref to its SHA via
// `git rev-parse --verify --quiet <ref>`. Exit 0 = found (SHA on stdout),
// exit 1 = ref does not resolve — both DATA per the exit-code-as-data
// contract, never a Runner failure; any other exit is a genuine error.
func (s *Service) revParseVerify(ctx context.Context, dir, ref string) (sha string, ok bool, err error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return "", false, err
	}

	req := newRequest(root, "rev-parse", "--verify", "--quiet", ref)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return "", false, fmt.Errorf("git: resolving %s in %s: %w", ref, root, err)
	}

	switch result.ExitCode {
	case 0:
		return strings.TrimSpace(string(result.Stdout)), true, nil
	case 1:
		return "", false, nil
	default:
		return "", false, fmt.Errorf("git: resolving %s in %s: %s", ref, root, strings.TrimSpace(string(result.Stderr)))
	}
}

// RemoteHead resolves the destination branch's remote HEAD via
// `git rev-parse origin/<target>` (HU-004 AC: "Mostrar HEAD remoto de la
// rama destino"). ok is false when origin/<target> does not resolve
// (the branch does not exist on origin), not an error.
func (s *Service) RemoteHead(ctx context.Context, dir, target string) (sha string, ok bool, err error) {
	return s.revParseVerify(ctx, dir, "origin/"+target)
}

// BranchExists reports whether branch resolves locally or as an origin
// remote-tracking branch (HU-004 AC: destination/custom-branch existence
// check — "se bloquea el flujo" when neither resolves). Local is checked
// first since it is the more common case and avoids a second call whenever
// it already resolves.
func (s *Service) BranchExists(ctx context.Context, dir, branch string) (bool, error) {
	if _, ok, err := s.revParseVerify(ctx, dir, branch); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	_, ok, err := s.revParseVerify(ctx, dir, "origin/"+branch)
	return ok, err
}

// LocalBranchExists reports whether branch resolves as a LOCAL branch
// (unlike BranchExists, it never also checks origin) — the exists-guard
// StateBranchCollision's "delete & recreate" action (internal/app/commands.go
// deleteAndRecreateBranchCmd) uses to skip DeleteLocalBranch (a `-D` force
// delete that itself errors on an absent branch) when branch was never
// created locally, e.g. a purely origin-only collision.
func (s *Service) LocalBranchExists(ctx context.Context, dir, branch string) (bool, error) {
	_, ok, err := s.revParseVerify(ctx, dir, branch)
	return ok, err
}

// RemoteBranchExists reports whether branch resolves as an origin
// remote-tracking branch (unlike BranchExists, it never also checks local)
// — the exists-guard deleteAndRecreateBranchCmd uses to skip
// DeleteRemoteBranch (which itself errors when the remote ref was never
// pushed — see its own doc comment) when branch was never pushed to origin.
func (s *Service) RemoteBranchExists(ctx context.Context, dir, branch string) (bool, error) {
	_, ok, err := s.revParseVerify(ctx, dir, "origin/"+branch)
	return ok, err
}
