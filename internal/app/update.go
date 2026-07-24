package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
)

// Update is the Bubble Tea reducer. It derives the next state from the
// incoming message and, where a service call is needed, returns a command
// that routes through the injected services (never an app-level exec).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case prereqDoneMsg:
		return m.onPrereqDone(msg)
	case discoverDoneMsg:
		return m.onDiscoverDone(msg)
	case depWarningsMsg:
		m.depWarnings = msg.warnings
		return m, nil
	case sandboxWarnMsg:
		m.sandboxWarn = msg.warn
		return m, nil
	case branchCreatedMsg:
		return m.onBranchCreated(msg)
	case pickDoneMsg:
		return m.onPickDone(msg)
	case repoStateMsg:
		return m.onRepoState(msg)
	case abortedMsg:
		return m.onAborted(msg)
	case verifyDoneMsg:
		return m.onVerifyDone(msg)
	case tickMsg:
		return m.onTick()
	}
	return m, nil
}

// prereqHasBlocking reports whether any check is blocking.
func prereqHasBlocking(checks []prereq.PrereqCheck) bool {
	for _, c := range checks {
		if c.Status == prereq.StatusBlocking {
			return true
		}
	}
	return false
}

func (m Model) onPrereqDone(msg prereqDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	m.checks = msg.checks
	if prereqHasBlocking(msg.checks) {
		// Blockers keep the user on the doctor screen (retry with `r`).
		m.state = StatePrereqCheck
		return m, nil
	}
	m.state = StateTicketInput
	return m, nil
}

func (m Model) onDiscoverDone(msg discoverDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	m.discovery = msg.result
	m.source = msg.source
	m.items = git.NewCommitSelectionItems(msg.result.OrderedCommits, m.ticket)
	m.cursor = 0
	m.state = StateCommitSelection
	return m, m.depWarningsCmd()
}

func (m Model) onBranchCreated(msg branchCreatedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	m.plan = git.RegisterPromotionBranch(m.plan, m.branchName)
	m.state = StateCherryPicking
	return m, tea.Batch(m.cherryPickCmd(), tickCmd())
}

func (m Model) onPickDone(msg pickDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	m.pickOutcome = msg.outcome
	m.repoState = msg.outcome.State

	if msg.outcome.State.InProgress {
		if msg.outcome.Empty {
			// Empty pick: the change is already in the target — skip and
			// resume the sequence automatically (HU-006 AC8).
			return m, m.skipCmd()
		}
		// Conflict: stop for resolution and start re-polling so external
		// resolution/abort reconciles live.
		m.state = StateCherryPickConflict
		return m, tea.Batch(m.repoStateCmd(), tickCmd())
	}

	// All picks applied — verify the promoted content.
	m.state = StateCherryPicking
	return m, m.verifyCmd()
}

func (m Model) onRepoState(msg repoStateMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Transient poll error: keep the current view, try again next tick.
		return m, nil
	}
	m.repoState = msg.state
	m.continueEnabled = msg.gate.Enabled
	m.continuePending = msg.gate.Pending

	// Reconcile external completion/abort: once the repo reports no
	// cherry-pick in progress, the sequence concluded outside this screen —
	// resynchronize by moving on to verification.
	if !msg.state.InProgress && (m.state == StateCherryPickConflict || m.state == StateCherryPicking) {
		m.state = StateCherryPicking
		return m, m.verifyCmd()
	}
	return m, nil
}

func (m Model) onAborted(msg abortedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	// A confirmed abort is NOT a clean completion: record it so DeltaAllowed
	// stays false even though the working tree is now clean.
	m.aborted = true
	m.repoState = git.RepoState{Clean: true}
	m.state = StateAborted
	return m, nil
}

func (m Model) onVerifyDone(msg verifyDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	m.verification = msg.verification
	m.deltaAllowed = git.DeltaAndValidationAllowed(m.repoState, m.aborted)
	m.state = StatePickVerification
	return m, nil
}

// onTick re-polls RepoState ONLY while a cherry-pick is active, so the screen
// reflects external `--continue`/`--abort` (HU-006 AC4/AC6). In any other
// state the tick stops (returns nil), so no background polling runs once the
// flow leaves the cherry-pick screens.
func (m Model) onTick() (tea.Model, tea.Cmd) {
	if m.state == StateCherryPicking || m.state == StateCherryPickConflict {
		return m, tea.Batch(m.repoStateCmd(), tickCmd())
	}
	return m, nil
}
