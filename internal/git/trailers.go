package git

import "regexp"

// cherryPickTrailerPattern matches the exact provenance trailer
// `git cherry-pick -x` writes into an applied commit's message
// (service_cherrypick.go's cherryPickArgs doc comment): a line containing
// "(cherry picked from commit <sha>)", capturing the referenced sha. A
// hex-only, 7-to-40-char capture accepts both the FULL sha `-x` always
// writes and any shorter form a differently-configured caller might
// produce; anything else (a missing/malformed sha, e.g. an empty or
// non-hex value) simply fails to match rather than panicking.
var cherryPickTrailerPattern = regexp.MustCompile(`\(cherry picked from commit ([0-9a-fA-F]{7,40})\)`)

// ParseCherryPickTrailers parses gitLog — the raw output of `git log
// <branch>`, whose commit bodies already carry any `-x` provenance trailer
// — into a set of every referenced source SHA (incremental-promotion spec:
// "Layered Already-On-Branch Detection" — the trailer signal). It is PURE
// (no I/O) so FilterNotOnBranch's caller fetches the log once and this
// function classifies every candidate commit against it, rather than one
// `git log --grep` per commit. A malformed or incomplete trailer line
// (no valid hex sha) is silently skipped — never a partial match, never a
// panic.
func ParseCherryPickTrailers(gitLog []byte) map[string]bool {
	present := make(map[string]bool)
	for _, m := range cherryPickTrailerPattern.FindAllSubmatch(gitLog, -1) {
		if len(m) < 2 || len(m[1]) == 0 {
			continue
		}
		present[string(m[1])] = true
	}
	return present
}
