package git

import (
	"fmt"
	"strings"
	"time"
)

// Commit is a normalized commit as returned by every commit-listing method
// in this package (SearchCommits, CommitsInRange): SHA, author, date,
// subject, and parent SHAs. Parents is what IsMerge derives from — HU-002
// requires no separate `git log --parents` call, since %P already carries
// this per commit line via the shared commitLogFormat.
type Commit struct {
	SHA      string
	ShortSHA string
	Author   string
	Date     time.Time
	Parents  []string
	Subject  string
}

// IsMerge reports whether c has more than one parent (HU-002: merge
// commits are flagged and blocked from selection).
func (c Commit) IsMerge() bool {
	return len(c.Parents) > 1
}

// commitLogFormat is the shared `git log`/`git rev-list --format` layout
// used by every commit-listing command in this package (SearchCommits,
// CommitsInRange), so parseCommitLog serves both: full SHA, short SHA,
// author name, strict ISO 8601 author date, space-separated parent SHAs,
// subject — tab-delimited so subjects containing spaces parse safely.
const commitLogFormat = "%H%x09%h%x09%an%x09%aI%x09%P%x09%s"

// commitLogFieldCount is the number of tab-delimited fields commitLogFormat
// produces per line.
const commitLogFieldCount = 6

// parseCommitLog parses raw output produced by a git command using
// commitLogFormat into normalized Commits. Blank lines are skipped so
// trailing newlines never produce a spurious empty Commit.
func parseCommitLog(raw []byte) ([]Commit, error) {
	var commits []Commit

	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", commitLogFieldCount)
		if len(fields) != commitLogFieldCount {
			return nil, fmt.Errorf("git: malformed commit log line (expected %d tab-delimited fields, got %d): %q", commitLogFieldCount, len(fields), line)
		}

		date, err := time.Parse(time.RFC3339, fields[3])
		if err != nil {
			return nil, fmt.Errorf("git: malformed commit log line: parsing author date %q: %w", fields[3], err)
		}

		var parents []string
		if trimmed := strings.TrimSpace(fields[4]); trimmed != "" {
			parents = strings.Split(trimmed, " ")
		}

		commits = append(commits, Commit{
			SHA:      fields[0],
			ShortSHA: fields[1],
			Author:   fields[2],
			Date:     date,
			Parents:  parents,
			Subject:  fields[5],
		})
	}

	return commits, nil
}
