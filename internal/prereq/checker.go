package prereq

import (
	"deploydeck/internal/config"
	"deploydeck/internal/git"
	"deploydeck/internal/salesforce"
)

// Status is the outcome of one PrereqCheck.
type Status string

const (
	StatusOK       Status = "OK"
	StatusWarning  Status = "warning"
	StatusBlocking Status = "blocking"
)

// PrereqCheck reports the outcome of one local-prerequisite check: HU-001
// AC ("Status = OK/warning/blocking, Detail, FixCommand").
type PrereqCheck struct {
	Name       string
	Status     Status
	Detail     string
	FixCommand string
}

// Checker runs HU-001 local-prerequisite checks composing git.Service,
// salesforce.Client and config.Config. It is reused by both the TUI and the
// `deploydeck doctor` CLI subcommand so their results never drift.
type Checker struct {
	// Dir is the directory to run repo/worktree checks from; Git resolves
	// the actual repo root itself.
	Dir string

	Git    *git.Service
	SF     salesforce.Client
	Config config.Config

	// Lock is the single-instance lock to acquire as part of Check(). A nil
	// Lock skips lock acquisition entirely (useful for tests/dry runs of
	// the other checks).
	Lock *Lock
}
