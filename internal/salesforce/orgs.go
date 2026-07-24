package salesforce

import (
	"context"
	"fmt"
	"strings"

	"deploydeck/internal/exec"
)

// Orgs runs `sf org list --json` and decodes the envelope's result object
// into all five org categories.
func (c *client) Orgs(ctx context.Context) (OrgList, error) {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{"org", "list", "--json"},
	})
	if err != nil {
		return OrgList{}, fmt.Errorf("salesforce: running sf org list --json: %w", err)
	}
	if result.ExitCode != 0 {
		return OrgList{}, fmt.Errorf("salesforce: sf org list --json exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	var list OrgList
	if err := decodeEnvelope(result.Stdout, &list); err != nil {
		return OrgList{}, err
	}
	return list, nil
}
