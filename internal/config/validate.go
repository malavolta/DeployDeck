package config

import (
	"fmt"
	"regexp"
	"strings"
)

var branchFormatTokenPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// Validate checks that ticketPatterns compile as regexes, every sandbox has
// a non-empty alias, branchFormat only uses tokens from
// AllowedBranchFormatTokens, PollIntervalSeconds/PollTimeoutSeconds are
// strictly positive, and — when any Delta field is configured — Delta.
// SourceDirs is non-empty (an sgd run with zero source dirs would scan
// nothing, silently producing an always-empty package).
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

	if c.PollIntervalSeconds <= 0 {
		return fmt.Errorf("config: pollIntervalSeconds must be > 0, got %d", c.PollIntervalSeconds)
	}
	if c.PollTimeoutSeconds <= 0 {
		return fmt.Errorf("config: pollTimeoutSeconds must be > 0, got %d", c.PollTimeoutSeconds)
	}

	if deltaConfigured(c.Delta) && len(c.Delta.SourceDirs) == 0 {
		return fmt.Errorf("config: delta is configured but sourceDirs is empty")
	}

	return nil
}

// deltaConfigured reports whether any DeltaConfig field was set, so
// Validate can require SourceDirs only once the user has opted into delta
// configuration at all (an entirely omitted delta section is valid — HU-007
// wiring is optional until the caller starts using it).
func deltaConfigured(d DeltaConfig) bool {
	return d.OutputDir != "" || len(d.SourceDirs) > 0 || d.IgnoreFile != "" || d.IgnoreDestructiveFile != ""
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
