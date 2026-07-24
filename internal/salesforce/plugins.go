package salesforce

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"deploydeck/internal/exec"
)

// oclifPlugin mirrors one entry from the TOP-LEVEL JSON ARRAY that
// `sf plugins --json` emits — an oclif plugin listing, NOT the
// `{"status":0,"result":...}` envelope other `sf ... --json` commands
// (e.g. `sf org list --json`, `sf --version`) use. A plugin such as
// sfdx-git-delta may appear either as a top-level entry or nested inside
// another plugin's "children" array. Verified against sf CLI 2.135.7.
type oclifPlugin struct {
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	Children []oclifPlugin `json:"children"`
}

// Plugins runs `sf plugins --json` and flattens its top-level array (each
// entry plus any nested children, recursively) into a flat []Plugin.
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

	var raw []oclifPlugin
	if err := json.Unmarshal(result.Stdout, &raw); err != nil {
		return nil, fmt.Errorf("salesforce: parsing sf plugins --json: %w", err)
	}

	var plugins []Plugin
	flattenOclifPlugins(raw, &plugins)
	return plugins, nil
}

// flattenOclifPlugins recursively appends each plugin (self, then children)
// into out, so a plugin nested under another plugin's "children" array is
// still surfaced as a flat Plugin entry.
func flattenOclifPlugins(raw []oclifPlugin, out *[]Plugin) {
	for _, p := range raw {
		*out = append(*out, Plugin{Name: p.Name, Version: p.Version})
		if len(p.Children) > 0 {
			flattenOclifPlugins(p.Children, out)
		}
	}
}
