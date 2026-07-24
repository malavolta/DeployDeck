package git

import "strings"

// EquivalenceStatus classifies whether a commit is already present in the
// target branch, and if so, how it was detected (HU-002: "Marcar commits
// ya presentes en destino por SHA y por equivalencia de contenido").
type EquivalenceStatus int

const (
	// NotApplied means none of the equivalence signals matched.
	NotApplied EquivalenceStatus = iota
	// AlreadyAppliedBySHA means the commit's own SHA is an ancestor of
	// the target (identical history, e.g. the branch was merged, not
	// cherry-picked).
	AlreadyAppliedBySHA
	// EquivalentByCherry means `git cherry` marked the commit '-':
	// equivalent content already present in the target under a different
	// SHA (the expected shape for this tool's own cherry-pick flow).
	EquivalentByCherry
	// EquivalentByPatchID means the commit's `git patch-id --stable`
	// output matches a commit already in the target, a fallback signal
	// for cases `git cherry` misses.
	EquivalentByPatchID
)

// AlreadyApplied reports whether s represents any form of already-applied
// detection (SHA ancestor or content equivalence). Neither case is
// selected by default (HU-002).
func (s EquivalenceStatus) AlreadyApplied() bool {
	return s != NotApplied
}

// ClassifyEquivalence determines whether a commit is already applied to
// the target, given precomputed signals: whether it is an ancestor of
// target (`git merge-base --is-ancestor`), its `git cherry` marker
// ('-' = equivalent content already upstream, '+' = unique to this
// branch), and its own patch-id compared against a set of the target's
// patch-ids (`git show <sha> | git patch-id --stable`) — a fallback for
// histories `git cherry` doesn't classify as equivalent. Checked in that
// priority order: SHA ancestry is authoritative when true; cherry's
// per-commit classification is checked next; patch-id is the last-resort
// fallback signal.
func ClassifyEquivalence(isAncestor bool, cherryMarker byte, patchID string, targetPatchIDs map[string]bool) EquivalenceStatus {
	if isAncestor {
		return AlreadyAppliedBySHA
	}
	if cherryMarker == '-' {
		return EquivalentByCherry
	}
	if patchID != "" && targetPatchIDs[patchID] {
		return EquivalentByPatchID
	}
	return NotApplied
}

// ParseCherryOutput parses `git cherry <upstream> <head>` output into a
// map from commit SHA to its marker: '+' (unique to head) or '-'
// (equivalent content already present in upstream under a different
// SHA). Blank lines are ignored.
func ParseCherryOutput(raw []byte) map[string]byte {
	markers := make(map[string]byte)

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 || len(parts[0]) == 0 {
			continue
		}
		markers[strings.TrimSpace(parts[1])] = parts[0][0]
	}

	return markers
}

// ParsePatchID parses `git show <sha> | git patch-id --stable` output
// ("<patch-id> <commit-sha>"), returning the patch-id. ok is false for
// empty output (e.g. an empty diff produces no patch-id line).
func ParsePatchID(raw []byte) (patchID string, ok bool) {
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return "", false
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
}
