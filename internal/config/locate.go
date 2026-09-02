package config

import (
	"os"
	"path/filepath"
)

// Locate searches upward from startDir for deploydeck.yaml, probing
// startDir first and then each ancestor directory up to and including
// stopDir, returning the first directory that contains the file
// (design.md ADR-1). It is PURE: no git dependency, no exec — the caller
// (typically the composition root) supplies stopDir as the git root it
// already resolved via git.RepoRoot, so this package stays a leaf.
//
// WHEN stopDir is empty or is not an ancestor of (or equal to) startDir,
// the upward walk is skipped entirely and only startDir is probed — this
// keeps a misconfigured or absent bound from silently walking arbitrarily
// far up the filesystem.
func Locate(startDir, stopDir string) (dir string, found bool) {
	stop := filepath.Clean(startDir)
	if stopDir != "" && isAncestorOrSelf(stopDir, startDir) {
		stop = filepath.Clean(stopDir)
	}

	cur := filepath.Clean(startDir)
	for {
		if configExists(cur) {
			return cur, true
		}
		if cur == stop {
			return "", false
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the filesystem root without hitting stop — should be
			// unreachable given the isAncestorOrSelf guard above, but never
			// loop forever.
			return "", false
		}
		cur = parent
	}
}

// configExists reports whether dir contains deploydeck.yaml.
func configExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil
}

// isAncestorOrSelf reports whether ancestor is dir itself or a directory
// above it in the filesystem hierarchy.
func isAncestorOrSelf(ancestor, dir string) bool {
	ancestor = filepath.Clean(ancestor)
	cur := filepath.Clean(dir)
	for {
		if cur == ancestor {
			return true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return false
		}
		cur = parent
	}
}
