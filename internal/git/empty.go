package git

// emptyPickExplanation is the user-facing message shown when a cherry-pick
// becomes empty because the change is already present in the target (the
// squash-merge case). Offering `--skip` is the safety-net (DEC-001 is process
// context only — this net is implemented regardless of merge/squash policy).
const emptyPickExplanation = "the change is already present in the target (empty cherry-pick); skip it with `git cherry-pick --skip`"

// isEmptyPickState reports whether the reconciled repository state is a
// genuine empty cherry-pick. A genuine empty pick is exactly: a pick is in
// progress (CHERRY_PICK_HEAD is set) AND the working tree is clean AND there
// are zero unmerged paths — the commit currently being applied produced no
// change (its content is already present) and no new commit was created for
// it, so git stopped for `--skip`/`--continue`.
//
// This is derived from REPOSITORY STATE, never from the echoed command
// output. A successful pick prints each applied commit's subject, and a
// conflicting pick prints "could not apply <sha>... <subject>", so a substring
// match on the output for git's "is now empty" advice is spoofable by a commit
// whose subject contains that phrase. Worse, on a CONFLICTING pick that
// substring would set Empty=true alongside real unmerged paths, letting an
// Empty-first consumer `--skip` and silently drop the conflicting commit.
func isEmptyPickState(state RepoState) bool {
	return state.InProgress && state.Clean && len(state.Unmerged) == 0
}
