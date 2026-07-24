package git

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// PickVerification is the result of comparing the promoted (post-pick)
// content of the touched files against the source branch (HU-006 AC10).
// PartialFiles are the touched files whose final content still DIFFERS from
// the source — each is a per-file partial-promotion warning that must be
// shown before any delta-generation step.
type PickVerification struct {
	PartialFiles []string
}

// OK reports whether every touched file matches the source branch (no
// partial promotion).
func (v PickVerification) OK() bool {
	return len(v.PartialFiles) == 0
}

// Warnings renders one partial-promotion warning per differing file.
func (v PickVerification) Warnings() []string {
	out := make([]string, 0, len(v.PartialFiles))
	for _, f := range v.PartialFiles {
		out = append(out, fmt.Sprintf("%s: partial promotion — final content differs from the source branch", f))
	}
	return out
}

// VerifyPromotedContent compares the touched files at HEAD against the source
// branch via `git diff --name-only -z HEAD <source> -- <files...>` once all
// picks complete (HU-006 AC10). Any file listed still differs from the source
// and is reported as a partial promotion. With no touched files the result is
// trivially OK.
func (s *Service) VerifyPromotedContent(ctx context.Context, dir, source string, touchedFiles []string) (PickVerification, error) {
	if len(touchedFiles) == 0 {
		return PickVerification{}, nil
	}

	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return PickVerification{}, err
	}

	args := []string{"diff", "--name-only", "-z", "HEAD", source, "--"}
	args = append(args, touchedFiles...)
	req := newRequest(root, args...)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return PickVerification{}, fmt.Errorf("git: verifying promoted content against %s in %s: %w", source, root, err)
	}
	if result.ExitCode != 0 {
		return PickVerification{}, fmt.Errorf("git: verifying promoted content against %s in %s: %s", source, root, strings.TrimSpace(string(result.Stderr)))
	}

	return PickVerification{PartialFiles: splitNameOnlyZ(result.Stdout)}, nil
}

// splitNameOnlyZ splits NUL-delimited `git diff --name-only -z` output into
// paths, dropping the trailing empty element.
func splitNameOnlyZ(raw []byte) []string {
	var paths []string
	for _, p := range bytes.Split(raw, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		paths = append(paths, string(p))
	}
	return paths
}

// DeltaAndValidationAllowed reports whether the flow may proceed to delta
// generation and Salesforce validation (HU-006 AC11). It is FALSE whenever a
// cherry-pick is still in progress (conflict / mid-sequence) or the run was
// aborted — a failed cherry-pick never generates a delta or runs validation.
// Pure.
func DeltaAndValidationAllowed(state RepoState, aborted bool) bool {
	if aborted {
		return false
	}
	return !state.InProgress
}
