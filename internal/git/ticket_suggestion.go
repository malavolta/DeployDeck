package git

import "regexp"

// TicketFromBranch returns the first ticket-pattern match found in branch,
// or "" when no pattern matches or a pattern fails to compile. patterns are
// tried in the given order (config.TicketPatterns), so the first configured
// pattern that matches wins; an invalid regex is skipped rather than
// aborting the search. Used to pre-fill StateTicketInput's buffer from the
// current branch name (app package).
func TicketFromBranch(branch string, patterns []string) string {
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		if match := re.FindString(branch); match != "" {
			return match
		}
	}
	return ""
}
