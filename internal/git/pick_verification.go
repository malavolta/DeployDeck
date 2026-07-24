package git

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// PickVerification is the result of verifying the promoted (post-pick) branch
// against two references (HU-006 AC10, hardened). PartialFiles are selected
// files whose final content differs from the intended selected-commit content
// (a per-file partial-promotion warning). SpuriousFiles are files changed on
// the promotion branch that the selected commit set never touched — an
// unexpected/spurious promotion (e.g. a wrong pick dragging in an extra file).
type PickVerification struct {
	PartialFiles  []string
	SpuriousFiles []string
}

// OK reports whether the promotion is faithful: every selected file matches
// the intended content AND no unselected file was changed on the branch.
func (v PickVerification) OK() bool {
	return len(v.PartialFiles) == 0 && len(v.SpuriousFiles) == 0
}

// Warnings renders one warning per differing/spurious file.
func (v PickVerification) Warnings() []string {
	out := make([]string, 0, len(v.PartialFiles)+len(v.SpuriousFiles))
	for _, f := range v.PartialFiles {
		out = append(out, fmt.Sprintf("%s: partial promotion — final content differs from the selected source commit", f))
	}
	for _, f := range v.SpuriousFiles {
		out = append(out, fmt.Sprintf("%s: spurious promotion — changed on the branch but not part of the selected commit set", f))
	}
	return out
}

// VerifyPromotedContent verifies, once all picks complete, the promotion
// branch against two references (HU-006 AC10, hardened to close a false-pass
// and a false-fail in the naive "diff HEAD <source> -- <touched>" form):
//
//   - SPURIOUS check: every file that changed between the pre-pick base
//     (base, e.g. origin/<target>) and HEAD but that the selected commit set
//     never touched is flagged. This closes the false-pass where a wrong pick
//     drags in an extra file a touched-files-only diff would never inspect.
//   - PARTIAL check: every selected file whose HEAD content differs from the
//     intended selected-commit tip (selectedTip, the last selected commit in
//     source history) is flagged. Comparing against the SELECTED tip — not the
//     source branch tip — avoids false-failing an intentional subset promotion
//     whose files legitimately differ from later, deliberately-excluded
//     commits (which do sit on the source branch tip).
//
// Residual limitation (documented honestly): the partial check compares whole
// touched-file blobs at HEAD against selectedTip. If a conflict resolution
// happens to reproduce selectedTip's blob exactly, it is treated as faithful.
// This is the same content-equality basis AC10 specifies and is sufficient for
// the promote-a-selected-set flow. With no selected files the spurious check
// still runs; with an empty selectedTip the partial check is skipped.
func (s *Service) VerifyPromotedContent(ctx context.Context, dir, base, selectedTip string, selectedFiles []string) (PickVerification, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return PickVerification{}, err
	}

	selected := make(map[string]bool, len(selectedFiles))
	for _, f := range selectedFiles {
		selected[f] = true
	}

	// Spurious: files changed base..HEAD that are not in the selected set.
	changed, err := s.diffNameOnly(ctx, root, base, "HEAD", nil)
	if err != nil {
		return PickVerification{}, err
	}
	var spurious []string
	for _, f := range changed {
		if !selected[f] {
			spurious = append(spurious, f)
		}
	}

	// Partial: selected files whose HEAD content differs from the intended
	// selected-commit tip.
	var partial []string
	if len(selectedFiles) > 0 && selectedTip != "" {
		partial, err = s.diffNameOnly(ctx, root, "HEAD", selectedTip, selectedFiles)
		if err != nil {
			return PickVerification{}, err
		}
	}

	return PickVerification{PartialFiles: partial, SpuriousFiles: spurious}, nil
}

// diffNameOnly runs `git diff --name-only -z <a> <b> [-- paths...]` and returns
// the changed paths (NUL-split). `git diff` exits 0 whether or not files
// differ (no --exit-code), so a non-zero exit is a genuine error.
func (s *Service) diffNameOnly(ctx context.Context, root, a, b string, paths []string) ([]string, error) {
	args := []string{"diff", "--name-only", "-z", a, b}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	req := newRequest(root, args...)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: diff %s..%s in %s: %w", a, b, root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: diff %s..%s in %s: %s", a, b, root, strings.TrimSpace(string(result.Stderr)))
	}
	return splitNameOnlyZ(result.Stdout), nil
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
