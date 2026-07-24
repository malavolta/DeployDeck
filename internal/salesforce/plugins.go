package salesforce

import (
	"context"
	"fmt"
	"strings"

	"deploydeck/internal/exec"
)

// Plugins runs `sf plugins --json` and decodes the envelope's result array.
func (c *client) Plugins(ctx context.Context) ([]Plugin, error) {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{"plugins", "--json"},
	})
	if err != nil {
		return nil, fmt.Errorf("salesforce: running sf plugins --json: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("salesforce: sf plugins --json exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	var plugins []Plugin
	if err := decodeEnvelope(result.Stdout, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}
