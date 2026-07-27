package app

import (
	"strings"

	"deploydeck/internal/config"
	"deploydeck/internal/git"
)

// preliminaryTarget picks the default destination branch used to compute the
// discovery range and equivalence classification BEFORE the user reaches the
// target-selection screen (the mockup's "Destino preliminar"). It is the
// first configured destination in git.ListDestinations order (sorted by
// environment key for determinism); "" when no branches are configured.
func preliminaryTarget(cfg config.Config) string {
	dests := git.ListDestinations(cfg)
	if len(dests) == 0 {
		return ""
	}
	return dests[0].Branch
}

// sourceRefName strips a leading "origin/" from a candidate branch name so it
// can feed git.DiscoverOptions.Source, which the git layer re-prefixes as
// "origin/<source>". A local candidate ("feature/X") passes through unchanged.
func sourceRefName(b git.Branch) string {
	return strings.TrimPrefix(b.Name, "origin/")
}

// resolveSource picks the single source branch for a discovery run from the
// candidate branches, honoring RF-002's suggested default (the previous
// pipeline environment) while still enforcing single-source selection. ok is
// false when no unambiguous single source can be resolved (zero candidates,
// or multiple with no suggestion) — the caller then degrades to message-only
// results.
func resolveSource(candidates []git.Branch, cfg config.Config, target string) (git.Branch, bool) {
	if len(candidates) == 0 {
		return git.Branch{}, false
	}

	selected := ""
	if suggested, ok := git.SuggestDefaultSource(candidates, cfg, target); ok {
		selected = suggested.Name
	}

	branch, err := git.SelectSingleSource(candidates, selected)
	if err != nil {
		return git.Branch{}, false
	}
	return branch, true
}

// selectedCommits returns the DiscoveredCommits the user currently has
// selected, in item (topological) order. Pure.
func selectedCommits(items []git.CommitSelectionItem) []git.DiscoveredCommit {
	var selected []git.DiscoveredCommit
	for _, it := range items {
		if it.Selected {
			selected = append(selected, it.DiscoveredCommit)
		}
	}
	return selected
}

// selectedSHASet is the set of currently-selected commit SHAs. Pure.
func selectedSHASet(items []git.CommitSelectionItem) map[string]bool {
	set := map[string]bool{}
	for _, it := range items {
		if it.Selected {
			set[it.SHA] = true
		}
	}
	return set
}

// commitSHAs extracts the SHAs of commits, in order — the Commits field
// HU-013 persists on the run record so resume detection can match the repo's
// live CHERRY_PICK_HEAD against this run's original selection. Pure; an
// empty selection yields nil (mirroring selectedCommits' convention).
func commitSHAs(commits []git.DiscoveredCommit) []string {
	if len(commits) == 0 {
		return nil
	}
	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.SHA
	}
	return shas
}
