package prereq

import (
	"context"
	"fmt"
	"sort"
)

// CheckAliases validates every configured Salesforce sandbox alias via
// `sf org list --json`, scanned across all five org categories
// (salesforce.OrgList.FindByAlias), and blocks validation against a
// sandbox whose alias does not exist (HU-001: "Salesforce Alias
// Validation").
func (c *Checker) CheckAliases(ctx context.Context) ([]PrereqCheck, error) {
	orgs, err := c.SF.Orgs(ctx)
	if err != nil {
		return nil, fmt.Errorf("prereq: checking Salesforce aliases: %w", err)
	}

	branches := make([]string, 0, len(c.Config.Sandboxes))
	for branch := range c.Config.Sandboxes {
		branches = append(branches, branch)
	}
	sort.Strings(branches)

	checks := make([]PrereqCheck, 0, len(branches))
	for _, branch := range branches {
		sandbox := c.Config.Sandboxes[branch]
		name := fmt.Sprintf("sandbox alias (%s)", branch)

		if sandbox.Alias == "" {
			// config.Validate() already rejects a missing alias, and
			// Checker.CheckConfig now actually calls it FIRST in Check()
			// (config-validation-wiring ADR-1) — so this branch is
			// unreachable via that path today, not merely defensive. It
			// stays as a guard for any future direct construction of a
			// Checker bypassing CheckConfig, and skipping here (rather than
			// reporting a confusing empty-alias check) avoids double-
			// reporting the same violation CheckConfig already surfaced.
			continue
		}

		if _, ok := orgs.FindByAlias(sandbox.Alias); !ok {
			checks = append(checks, PrereqCheck{
				Name:       name,
				Status:     StatusBlocking,
				Detail:     fmt.Sprintf("Salesforce alias %q (branch %q) was not found via sf org list --json", sandbox.Alias, branch),
				FixCommand: fmt.Sprintf("sf org login web --alias %s", sandbox.Alias),
			})
			continue
		}

		checks = append(checks, PrereqCheck{Name: name, Status: StatusOK, Detail: fmt.Sprintf("alias %q found", sandbox.Alias)})
	}

	return checks, nil
}
