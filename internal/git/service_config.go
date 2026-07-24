package git

import (
	"context"
	"fmt"
	"strings"
)

// ConfigGet reads a git config key in the repository containing dir via
// `git config --get <key>`. ok is false when the key is unset — a non-zero
// exit here is DATA (per the exit-code-as-data contract), never a Runner
// failure.
func (s *Service) ConfigGet(ctx context.Context, dir, key string) (value string, ok bool, err error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return "", false, err
	}

	req := newRequest(root, "config", "--get", key)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return "", false, fmt.Errorf("git: reading config %q in %s: %w", key, root, err)
	}
	if result.ExitCode != 0 {
		return "", false, nil
	}

	return strings.TrimSpace(string(result.Stdout)), true, nil
}
