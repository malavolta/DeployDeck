package salesforce

import (
	"context"
	"fmt"
	"strings"

	"deploydeck/internal/exec"
)

// Version runs `sf --version` and parses its plain-text output.
func (c *client) Version(ctx context.Context) (VersionInfo, error) {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{"--version"},
	})
	if err != nil {
		return VersionInfo{}, fmt.Errorf("salesforce: running sf --version: %w", err)
	}
	if result.ExitCode != 0 {
		return VersionInfo{}, fmt.Errorf("salesforce: sf --version exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	return parseVersionOutput(string(result.Stdout)), nil
}
