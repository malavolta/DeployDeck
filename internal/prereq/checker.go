package prereq

import (
	"github.com/malavolta/DeployDeck/internal/ai"
	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/salesforce"
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
	// GitRoot MUST be the resolved repository root (directory-resolution
	// design.md ADR-4): repository/working-tree/hooks/gpgsign checks run
	// against it, and the gitignore FALLBACK read consumes it directly
	// instead of re-resolving it via c.Git.RepoRoot.
	GitRoot string

	// ArtifactsRoot is where .deploydeck/ actually lives — the gitignore
	// check's PRIMARY probe and its AddGitignoreEntry fix target (ADR-2:
	// .deploydeck/ must never relocate for an existing install). No Checker
	// check runs an `sf project` command, so — unlike app.Deps — there is
	// deliberately no ProjectDir field here (ADR-4).
	ArtifactsRoot string

	Git    *git.Service
	SF     salesforce.Client
	Config config.Config

	// Lock is the single-instance lock to acquire as part of Check(). A nil
	// Lock skips lock acquisition entirely (useful for tests/dry runs of
	// the other checks).
	Lock *Lock

	// GH is the gh CLI client backing the informative, non-blocking gh
	// availability/auth doctor check (HU-014, CheckGH). A nil GH — every
	// Checker built before HU-014, and every existing checker test that
	// never sets it — skips CheckGH entirely, reporting OK.
	GH github.Client

	// AI is the local-model client backing the informative, non-blocking
	// AI endpoint/model availability doctor check (ai-pr-summary,
	// CheckAI). A nil AI — every Checker built before this slice, and
	// every existing checker test that never sets it — skips CheckAI
	// entirely, reporting OK.
	AI ai.Client
}
