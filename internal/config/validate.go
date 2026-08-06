package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var branchFormatTokenPattern = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// Validate checks that ticketPatterns compile as regexes, every sandbox has
// a non-empty alias, branchFormat only uses tokens from
// AllowedBranchFormatTokens, PollIntervalSeconds/PollTimeoutSeconds are
// strictly positive, Runs.KeepLast/Runs.KeepDays are non-negative (HU-013
// run-retention: a negative window is nonsensical), and — when any Delta
// field is configured — Delta.SourceDirs is non-empty (an sgd run with zero
// source dirs would scan nothing, silently producing an always-empty
// package).
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

	if c.Runs.KeepLast < 0 {
		return fmt.Errorf("config: runs.keepLast must be >= 0, got %d", c.Runs.KeepLast)
	}
	if c.Runs.KeepDays < 0 {
		return fmt.Errorf("config: runs.keepDays must be >= 0, got %d", c.Runs.KeepDays)
	}

	if deltaConfigured(c.Delta) && len(c.Delta.SourceDirs) == 0 {
		return fmt.Errorf("config: delta is configured but sourceDirs is empty")
	}

	if c.AI.Enabled && (c.AI.Endpoint == "" || c.AI.Model == "") {
		return fmt.Errorf("config: ai is enabled but endpoint or model is empty")
	}

	// Sorted iteration (remediation-pass readability fix): Go's map iteration
	// order is randomized per range statement, so an unsorted range here made
	// the reported error target — and therefore the whole error message —
	// flaky whenever more than one gate was invalid at once.
	gateTargets := make([]string, 0, len(c.Gates))
	for target := range c.Gates {
		gateTargets = append(gateTargets, target)
	}
	sort.Strings(gateTargets)

	for _, target := range gateTargets {
		gate := c.Gates[target]
		if !gate.Enabled {
			continue
		}
		if gate.MinApprovals != nil && *gate.MinApprovals < 1 {
			return fmt.Errorf("config: gate %q has minApprovals %d, must be >= 1", target, *gate.MinApprovals)
		}
		if len(gate.Approvers) == 0 {
			return fmt.Errorf("config: gate %q is enabled but approvers is empty", target)
		}
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
