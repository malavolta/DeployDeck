package config

import (
	"fmt"
	"regexp"
	"strings"
)

var branchFormatTokenPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// Validate checks that ticketPatterns compile as regexes, every sandbox has
// a non-empty alias, and branchFormat only uses tokens from
// AllowedBranchFormatTokens.
func (c Config) Validate() error {
	for _, pattern := range c.TicketPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("config: invalid ticketPatterns entry %q: %w", pattern, err)
		}
	}

	for branch, sandbox := range c.Sandboxes {
		if sandbox.Alias == "" {
			return fmt.Errorf("config: sandbox %q is missing an alias", branch)
		}
	}

	if err := validateBranchFormatTokens(c.BranchFormat); err != nil {
		return err
	}

	return nil
}

func validateBranchFormatTokens(format string) error {
	for _, token := range branchFormatTokenPattern.FindAllString(format, -1) {
		if !isAllowedBranchFormatToken(token) {
			return fmt.Errorf(
				"config: branchFormat %q contains unsupported token %q (allowed: %s)",
				format, token, strings.Join(AllowedBranchFormatTokens, ", "),
			)
		}
	}
	return nil
}

func isAllowedBranchFormatToken(token string) bool {
	for _, allowed := range AllowedBranchFormatTokens {
		if token == allowed {
			return true
		}
	}
	return false
}
