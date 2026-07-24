package git

import (
	"context"
	"fmt"
	"strings"
)

// IsAncestor reports whether sha is an ancestor of ref (e.g.
// "origin/<target>") via `git merge-base --is-ancestor <sha> <ref>`.
// Exit 0 means yes, exit 1 means no — both are DATA per the
// exit-code-as-data contract, never a Runner failure; any other exit
// (e.g. an unknown ref) is a genuine error.
func (s *Service) IsAncestor(ctx context.Context, dir, sha, ref string) (bool, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return false, err
	}

	req := newRequest(root, "merge-base", "--is-ancestor", sha, ref)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return false, fmt.Errorf("git: checking whether %s is an ancestor of %s in %s: %w", sha, ref, root, err)
	}

	switch result.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git: checking whether %s is an ancestor of %s in %s: %s", sha, ref, root, strings.TrimSpace(string(result.Stderr)))
	}
}

// Cherry runs `git cherry <upstream> <head>` and returns a map from
// commit SHA to its marker: '+' (unique to head) or '-' (equivalent
// content already present in upstream under a different SHA) — HU-002's
// content-equivalence detection.
func (s *Service) Cherry(ctx context.Context, dir, upstream, head string) (map[string]byte, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	req := newRequest(root, "cherry", upstream, head)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: cherry %s %s in %s: %w", upstream, head, root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: cherry %s %s in %s: %s", upstream, head, root, strings.TrimSpace(string(result.Stderr)))
	}

	return ParseCherryOutput(result.Stdout), nil
}

// PatchID computes the stable patch-id for sha, composing two
// CommandRequests (`git show <sha>` piped into `git patch-id --stable`)
// via CommandRequest.Stdin rather than an actual shell pipe — internal/exec
// never shells out. Returns "" when sha's diff is empty (no patch-id
// line), which is not an error: an empty diff simply has nothing to
// compare by content.
func (s *Service) PatchID(ctx context.Context, dir, sha string) (string, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return "", err
	}

	showReq := newRequest(root, "show", sha)
	showResult, err := s.runner.Run(ctx, showReq)
	if err != nil {
		return "", fmt.Errorf("git: show %s in %s: %w", sha, root, err)
	}
	if showResult.ExitCode != 0 {
		return "", fmt.Errorf("git: show %s in %s: %s", sha, root, strings.TrimSpace(string(showResult.Stderr)))
	}

	patchIDReq := newRequest(root, "patch-id", "--stable")
	patchIDReq.Stdin = showResult.Stdout
	patchIDResult, err := s.runner.Run(ctx, patchIDReq)
	if err != nil {
		return "", fmt.Errorf("git: patch-id --stable for %s in %s: %w", sha, root, err)
	}
	if patchIDResult.ExitCode != 0 {
		return "", fmt.Errorf("git: patch-id --stable for %s in %s: %s", sha, root, strings.TrimSpace(string(patchIDResult.Stderr)))
	}

	id, _ := ParsePatchID(patchIDResult.Stdout)
	return id, nil
}
