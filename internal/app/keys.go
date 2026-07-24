package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/git"
)

// handleKey routes a key press to the current-state handler.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StatePrereqCheck:
		return m.keyPrereq(msg)
	case StateTicketInput:
		return m.keyTicket(msg)
	case StateCommitSelection:
		return m.keySelection(msg)
	case StateTargetSelection:
		return m.keyTarget(msg)
	case StatePlanPreview:
		return m.keyPlanPreview(msg)
	case StateCherryPickConflict:
		return m.keyConflict(msg)
	case StatePickVerification:
		return m.keyVerification(msg)
	case StateAborted, StateError:
		if key := msg.String(); key == "q" || key == "enter" || key == "esc" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) keyPrereq(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		// Retry the prereq check.
		return m, m.runPrereqCmd()
	case "c":
		// Continue with warnings — only allowed when nothing is blocking.
		if prereqHasBlocking(m.checks) {
			m.notice = "cannot continue: resolve blocking prerequisites first"
			return m, nil
		}
		m.notice = ""
		m.state = StateTicketInput
		return m, nil
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) keyTicket(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.ticket == "" {
			m.notice = "enter a ticket to search"
			return m, nil
		}
		m.notice = ""
		m.prelim = preliminaryTarget(m.deps.Config)
		m.state = StateCommitDiscovery
		return m, m.discoverCmd()
	case "esc":
		m.state = StatePrereqCheck
		return m, nil
	case "backspace":
		if n := len(m.ticket); n > 0 {
			m.ticket = m.ticket[:n-1]
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.ticket += string(msg.Runes)
		}
		return m, nil
	}
}

func (m Model) keySelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil
	case " ", "space", "x":
		m.items = git.ToggleSelection(m.items, m.cursor)
		return m, m.depWarningsCmd()
	case "enter":
		return m.confirmSelection()
	case "esc":
		m.state = StateTicketInput
		return m, nil
	}
	return m, nil
}

func (m Model) confirmSelection() (tea.Model, tea.Cmd) {
	plan, err := git.GenerateDeploymentPlan(m.ticket, m.items)
	if err != nil {
		m.notice = "select at least one commit before continuing"
		return m, nil
	}
	m.notice = ""
	m.plan = plan
	m.contiguous = git.IsContiguousSelection(m.discovery.OrderedCommits, selectedCommits(m.items))
	m.destinations = git.ListDestinations(m.deps.Config)
	m.targetCursor = destinationIndex(m.destinations, m.prelim)
	m.state = StateTargetSelection
	return m, nil
}

// destinationIndex returns the index of the destination whose branch matches
// branch, or 0 when none match (a safe default cursor position).
func destinationIndex(dests []git.Destination, branch string) int {
	for i, d := range dests {
		if d.Branch == branch {
			return i
		}
	}
	return 0
}

func (m Model) keyTarget(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.targetCursor > 0 {
			m.targetCursor--
		}
		return m, nil
	case "down", "j":
		if m.targetCursor < len(m.destinations)-1 {
			m.targetCursor++
		}
		return m, nil
	case "enter":
		return m.confirmTarget()
	case "esc":
		m.state = StateCommitSelection
		return m, nil
	}
	return m, nil
}

func (m Model) confirmTarget() (tea.Model, tea.Cmd) {
	if len(m.destinations) == 0 {
		m.notice = "no destinations configured"
		return m, nil
	}
	dest := m.destinations[m.targetCursor]
	sandbox, blockMsg, err := git.ResolveSandbox(m.deps.Config, dest.Branch)
	if err != nil {
		m.notice = blockMsg
		return m, nil
	}
	m.notice = ""
	m.sandboxWarn = false
	m.plan = git.ConfirmTargetSelection(m.plan, dest.Branch, sandbox.Alias, sandbox.TestLevel)
	m.branchName = git.RenderBranchName(m.deps.Config.BranchFormat, m.plan.Ticket, dest.Branch)
	m.state = StatePlanPreview
	return m, m.sandboxWarnCmd(sandbox.Alias)
}

func (m Model) keyPlanPreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.state = StateBranchCreation
		return m, m.branchCreateCmd()
	case "esc":
		m.state = StateTargetSelection
		return m, nil
	}
	return m, nil
}

func (m Model) keyConflict(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		if !m.continueEnabled {
			m.notice = "continue is disabled: resolve every conflict first"
			return m, nil
		}
		m.notice = ""
		return m, m.continueCmd()
	case "s":
		return m, m.skipCmd()
	case "a":
		return m, m.abortCmd()
	case "e":
		// Interactive editor handoff (injected by main via tea.ExecProcess).
		if m.deps.Edit != nil && len(m.repoState.Unmerged) > 0 {
			return m, m.deps.Edit(m.repoState.Unmerged[m.conflictCursor()].Path)
		}
		return m, nil
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// conflictCursor clamps the selection cursor into the current unmerged set.
func (m Model) conflictCursor() int {
	if m.cursor < 0 || m.cursor >= len(m.repoState.Unmerged) {
		return 0
	}
	return m.cursor
}

func (m Model) keyVerification(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "q":
		return m, tea.Quit
	case "e":
		// Edit selection: PickVerification -> CommitSelection (partial
		// promotion, per the state diagram).
		m.state = StateCommitSelection
		return m, nil
	}
	return m, nil
}
