package git

import "strings"

// emptyPickExplanation is the user-facing message shown when a cherry-pick
// becomes empty because the change is already present in the target (the
// squash-merge case). Offering `--skip` is the safety-net (DEC-001 is process
// context only — this net is implemented regardless of merge/squash policy).
const emptyPickExplanation = "the change is already present in the target (empty cherry-pick); skip it with `git cherry-pick --skip`"

// isEmptyPickMessage reports whether cherry-pick command output signals an
// empty pick. Git prints "The previous cherry-pick is now empty" (exit 1)
// both for a plain already-present change and after a conflict resolves to
// nothing — either way the correct response is to offer `--skip`.
func isEmptyPickMessage(output string) bool {
	return strings.Contains(output, "is now empty")
}
