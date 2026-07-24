package config

import (
	"fmt"
	"path"
)

// SandboxFor resolves the SandboxConfig for branch, trying an exact
// Sandboxes key match first, then a glob match (e.g. "Release/*") via
// path.Match. It returns an error when no configured key matches.
func (c Config) SandboxFor(branch string) (SandboxConfig, error) {
	if sandbox, ok := c.Sandboxes[branch]; ok {
		return sandbox, nil
	}

	for pattern, sandbox := range c.Sandboxes {
		matched, err := path.Match(pattern, branch)
		if err != nil {
			continue
		}
		if matched {
			return sandbox, nil
		}
	}

	return SandboxConfig{}, fmt.Errorf("config: no sandbox configured for branch %q", branch)
}
