package git

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var gitVersionPattern = regexp.MustCompile(`git version (\S+)`)

// Version runs `git --version` and returns the parsed version token (e.g.
// "2.43.0"), stripping any platform-specific suffix such as
// "(Apple Git-146)". It needs no repository — git --version works from any
// directory — so it does not resolve a repo root first.
func (s *Service) Version(ctx context.Context) (string, error) {
	req := newRequest("", "--version")

	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("git: running git --version: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("git: git --version exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	out := strings.TrimSpace(string(result.Stdout))
	if m := gitVersionPattern.FindStringSubmatch(out); len(m) == 2 {
		return m[1], nil
	}
	return out, nil
}
