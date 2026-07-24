package git

import "errors"

// Single-source-branch enforcement errors (HU-002: "Forzar una unica rama
// origen por ejecucion: no se mezclan commits de ramas distintas").
var (
	// ErrNoSourceBranch means ticket search found zero candidate branches
	// (the caller has nothing to select a source from at all).
	ErrNoSourceBranch = errors.New("git: no candidate source branch found")
	// ErrMultipleSourceBranches means more than one candidate branch
	// exists and the caller must explicitly choose exactly one before
	// continuing — commits from different branches are never mixed in one
	// run.
	ErrMultipleSourceBranches = errors.New("git: multiple candidate source branches found; select exactly one before continuing")
	// ErrUnknownSourceBranch means the caller's selection does not name
	// any of the discovered candidates.
	ErrUnknownSourceBranch = errors.New("git: selected source branch is not among the candidate branches")
)

// SelectSingleSource resolves the single source branch to use for a
// discovery run, enforcing that commits from more than one candidate
// branch are never mixed. With zero candidates there is nothing to
// select. With exactly one candidate it is returned regardless of
// selected — nothing to disambiguate. With more than one candidate,
// selected MUST name one of them (RF-002's suggested default source, or
// any override the user picks instead, are both valid values here): an
// empty selected blocks with ErrMultipleSourceBranches, and a selected
// value outside the candidate pool blocks with ErrUnknownSourceBranch.
func SelectSingleSource(candidates []Branch, selected string) (Branch, error) {
	switch len(candidates) {
	case 0:
		return Branch{}, ErrNoSourceBranch
	case 1:
		return candidates[0], nil
	}

	if selected == "" {
		return Branch{}, ErrMultipleSourceBranches
	}

	for _, c := range candidates {
		if c.Name == selected {
			return c, nil
		}
	}
	return Branch{}, ErrUnknownSourceBranch
}
