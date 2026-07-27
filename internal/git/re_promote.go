package git

import "context"

// RemapResult is the outcome of RemapCommitsByPatchID: Matched holds the
// new-range DiscoveredCommit equivalents found for the prior run's commits
// (re-promotion spec: "Prior-Run Commits Pre-Loaded And Editable"), and
// Unmatched preserves the ORIGINAL prior SHAs that had no patch-id
// equivalent anywhere in the new range — never silently dropped, so the
// caller can warn on them explicitly (re-promotion spec: "Missing Commit
// Warned Explicitly").
type RemapResult struct {
	Matched   []DiscoveredCommit
	Unmatched []string
}

// RemapCommitsByPatchID maps each of priorSHAs to its patch-id equivalent
// commit in origin/<target>..origin/<source> — the SAME range CommitsInRange
// resolves for ordinary discovery — composing CommitsInRange + PatchID
// (re-promotion spec: "Patch-ID Remap Of Prior Commits"). Every matched
// DiscoveredCommit's Equivalence is fixed to NotApplied per design: a
// commit reused via patch-id remap is a fresh pick candidate in the new
// range, not something already applied there. A per-SHA PatchID lookup
// failure — an unresolvable/malformed SHA (e.g. a crafted leading "-" that
// could otherwise be misread as a `git show` option, design's threat-matrix
// row) or a genuinely absent match — degrades that SHA to Unmatched; it is
// NEVER propagated as a hard error and NEVER silently dropped.
func (s *Service) RemapCommitsByPatchID(ctx context.Context, dir string, priorSHAs []string, target, source string) (RemapResult, error) {
	rangeCommits, err := s.CommitsInRange(ctx, dir, target, source)
	if err != nil {
		return RemapResult{}, err
	}

	byPatchID := make(map[string][]DiscoveredCommit, len(rangeCommits))
	for _, c := range rangeCommits {
		patchID, err := s.PatchID(ctx, dir, c.SHA)
		if err != nil || patchID == "" {
			// A range commit whose own patch-id can't be computed (e.g. an
			// empty diff) simply has nothing for a prior SHA to match
			// against — not a hard error for the whole remap.
			continue
		}
		byPatchID[patchID] = append(byPatchID[patchID], DiscoveredCommit{
			Commit:      c,
			Merge:       c.IsMerge(),
			Equivalence: NotApplied,
		})
	}

	var result RemapResult
	for _, priorSHA := range priorSHAs {
		patchID, err := s.PatchID(ctx, dir, priorSHA)
		if err != nil || patchID == "" {
			result.Unmatched = append(result.Unmatched, priorSHA)
			continue
		}

		matches := byPatchID[patchID]
		if len(matches) == 0 {
			result.Unmatched = append(result.Unmatched, priorSHA)
			continue
		}
		result.Matched = append(result.Matched, matches[0])
	}

	return result, nil
}
