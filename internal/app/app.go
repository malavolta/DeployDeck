// Package app is the DeployDeck Bubble Tea TUI: the state machine that
// composes the prereq/discovery/selection/target/promotion/cherry-pick
// services into the interactive promotion flow
// (PrereqCheck -> ... -> PickVerification).
//
// Architecture invariant (enforced by boundary_test.go): app NEVER execs
// git/sf itself. Every external command is reached through the injected
// services — git.Service, salesforce.Client — which are the sole holders of
// the internal/exec seam. The Bubble Tea model is NEVER the source of truth
// for git state: it reconciles RepoState from the repository on every poll
// (design "repo is source of truth"). tea.ExecProcess interactive handoff
// ($EDITOR / mergetool) is injected via Deps.Edit by main so app itself need
// not import os/exec.
package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/config"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/salesforce"
)

// State is the current screen/phase of the promotion flow, mirroring the
// design's cherry-pick-subset state machine.
type State int

const (
	// StatePrereqCheck runs HU-001 local prerequisites; blockers keep the
	// user here (retry) and OK advances to ticket input.
	StatePrereqCheck State = iota
	// StateTicketInput collects the ticket/incidence to promote.
	StateTicketInput
	// StateCommitDiscovery runs HU-002 discovery for the entered ticket.
	StateCommitDiscovery
	// StateCommitSelection is HU-003: choose which discovered commits to
	// promote.
	StateCommitSelection
	// StateTargetSelection is HU-004: choose the destination branch/sandbox.
	StateTargetSelection
	// StatePlanPreview shows the assembled DeploymentPlan and the git
	// commands about to run.
	StatePlanPreview
	// StateBranchCreation runs HU-005 fetch + promotion-branch creation.
	StateBranchCreation
	// StateCherryPicking runs HU-006 sequencer-driven cherry-pick.
	StateCherryPicking
	// StateCherryPickConflict is the conflict resolution screen; it
	// re-polls RepoState so external --continue/--abort reconcile live.
	StateCherryPickConflict
	// StateAborted is the terminal reached by a confirmed abort — NOT a
	// clean completion (delta/validation stay disabled).
	StateAborted
	// StatePickVerification is the post-pick verification screen and the
	// scope edge of this slice (no DeltaGeneration/validation/push).
	StatePickVerification
	// StateError is a terminal error screen.
	StateError
)

// Deps carries the services and configuration the TUI composes. main() wires
// real NewOSRunner-backed services; tests inject fakes / real-git-on-temp-repo.
type Deps struct {
	// Git is the git service (real OSRunner in main, real-on-temp-repo in
	// integration tests). Required.
	Git *git.Service
	// SF is the read-only Salesforce shim, used for the target-selection
	// sandbox-auth warning. May be nil (warning is then skipped).
	SF salesforce.Client
	// Config is the loaded, validated deploydeck.yaml.
	Config config.Config
	// Dir is the working directory the flow runs in (the git service
	// resolves the repo root from it).
	Dir string
	// NewChecker builds the HU-001 prereq.Checker for the prereq screen.
	// When nil, the prereq step is a no-op the caller drives with a
	// prereqDoneMsg directly (used by tests that start past prereqs).
	NewChecker func(dir string) (*prereq.Checker, error)
	// Edit optionally returns a tea.Cmd that hands off to an interactive
	// editor/mergetool over path (built by main via tea.ExecProcess, which
	// may import os/exec — app itself must not). nil disables the handoff.
	Edit func(path string) tea.Cmd
}

// Model is the Bubble Tea model. It holds the current State, the cumulative
// DeploymentPlan, and the service deps — never the authoritative git state,
// which is re-read from the repo on every poll.
type Model struct {
	deps  Deps
	state State
	err   error
	// notice is a transient, non-terminal message (e.g. an empty-selection
	// or unmapped-sandbox block) shown on the current screen.
	notice string

	// PrereqCheck
	checks []prereq.PrereqCheck

	// TicketInput
	ticket string

	// Discovery / Selection
	discovery   git.DiscoverResult
	source      git.Branch
	prelim      string // preliminary target used for discovery/classification
	items       []git.CommitSelectionItem
	cursor      int
	depWarnings []git.DependencyWarning

	// TargetSelection
	destinations []git.Destination
	targetCursor int
	sandboxWarn  bool

	// Plan / Branch
	plan       git.DeploymentPlan
	branchName string

	// CherryPicking / Conflict
	contiguous      bool
	repoState       git.RepoState
	pickOutcome     git.PickOutcome
	continueEnabled bool
	continuePending []string
	aborted         bool

	// PickVerification
	verification git.PickVerification
	deltaAllowed bool
}

// New builds the initial Model in StatePrereqCheck.
func New(deps Deps) Model {
	return Model{deps: deps, state: StatePrereqCheck}
}

// State exposes the current state (for tests and callers).
func (m Model) State() State { return m.state }

// Err exposes any terminal error (for tests and callers).
func (m Model) Err() error { return m.err }

// Plan exposes the cumulative DeploymentPlan (for tests and callers).
func (m Model) Plan() git.DeploymentPlan { return m.plan }

// Verification exposes the post-pick verification result.
func (m Model) Verification() git.PickVerification { return m.verification }

// DeltaAllowed reports whether the flow may proceed to the (out-of-slice)
// delta/validation step: it threads the run's real aborted flag into
// git.DeltaAndValidationAllowed, so a successful abort (clean tree, but
// aborted=true) is never mistaken for a clean completion.
func (m Model) DeltaAllowed() bool {
	return git.DeltaAndValidationAllowed(m.repoState, m.aborted)
}

// Init kicks off the prereq check.
func (m Model) Init() tea.Cmd {
	return m.runPrereqCmd()
}

// ctx returns the context used for service calls. A short-lived TUI uses the
// background context; cancellation is handled by the process lifecycle.
func (m Model) ctx() context.Context { return context.Background() }
