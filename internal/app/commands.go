package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
)

// pollInterval is how often the cherry-pick screens re-read RepoState so an
// external `git cherry-pick --continue`/`--abort` reconciles live (HU-006
// AC4/AC6). The model is never the source of truth — the repo is.
const pollInterval = 750 * time.Millisecond

// --- Messages ---

// prereqDoneMsg carries the HU-001 prereq report (or an error).
type prereqDoneMsg struct {
	checks []prereq.PrereqCheck
	err    error
}

// discoverDoneMsg carries the assembled HU-002 discovery result and the
// resolved single source branch (or an error).
type discoverDoneMsg struct {
	result git.DiscoverResult
	source git.Branch
	err    error
}

// depWarningsMsg carries HU-003 per-file dependency warnings (best-effort).
type depWarningsMsg struct {
	warnings []git.DependencyWarning
}

// sandboxWarnMsg carries the HU-004 unauthenticated-sandbox warning flag
// (best-effort; errors are swallowed to a false warning).
type sandboxWarnMsg struct {
	warn bool
}

// branchCreatedMsg is the result of HU-005 promotion-branch creation.
type branchCreatedMsg struct {
	err error
}

// pickDoneMsg is the reconciled outcome of a cherry-pick engine action
// (initial pick, --continue, or --skip).
type pickDoneMsg struct {
	outcome git.PickOutcome
	err     error
}

// repoStateMsg is a re-poll result: the freshly reconciled RepoState plus the
// computed continue-gate. It is how external resolution/abort becomes visible.
type repoStateMsg struct {
	state git.RepoState
	gate  git.ContinueGate
	err   error
}

// abortedMsg is the result of a confirmed abort.
type abortedMsg struct {
	err error
}

// verifyDoneMsg is the HU-006 post-pick verification result.
type verifyDoneMsg struct {
	verification git.PickVerification
	err          error
}

// tickMsg drives the cherry-pick re-poll cadence.
type tickMsg struct{}

// --- Command constructors (every one routes through a service) ---

// runPrereqCmd runs the HU-001 checker. When no checker is injected (tests
// starting past prereqs) it is a no-op and the caller drives prereqDoneMsg.
func (m Model) runPrereqCmd() tea.Cmd {
	if m.deps.NewChecker == nil {
		return nil
	}
	dir := m.deps.Dir
	newChecker := m.deps.NewChecker
	ctx := m.ctx()
	return func() tea.Msg {
		checker, err := newChecker(dir)
		if err != nil {
			return prereqDoneMsg{err: err}
		}
		checks, err := checker.Check(ctx)
		return prereqDoneMsg{checks: checks, err: err}
	}
}

// discoverCmd runs HU-002 discovery in two composed passes through the git
// service: first message+branch search (to resolve the single source), then a
// ranged discovery with (preliminary target, resolved source) to produce the
// classified OrderedCommits HU-003 consumes.
func (m Model) discoverCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	cfg := m.deps.Config
	ticket := m.ticket
	prelim := m.prelim
	ctx := m.ctx()
	return func() tea.Msg {
		base, err := g.Discover(ctx, dir, git.DiscoverOptions{Ticket: ticket})
		if err != nil {
			return discoverDoneMsg{err: err}
		}

		source, ok := resolveSource(base.CandidateBranches, cfg, prelim)
		if !ok || prelim == "" {
			// Degrade to message-only results (no single source / no target).
			return discoverDoneMsg{result: base}
		}

		full, err := g.Discover(ctx, dir, git.DiscoverOptions{
			Ticket: ticket,
			Target: prelim,
			Source: sourceRefName(source),
		})
		if err != nil {
			return discoverDoneMsg{err: err}
		}
		return discoverDoneMsg{result: full, source: source}
	}
}

// depWarningsCmd computes HU-003 dependency warnings for the current selection
// through the git service (real diff). Best-effort: an error yields no
// warnings rather than blocking the selection screen.
func (m Model) depWarningsCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ordered := m.discovery.OrderedCommits
	selected := selectedSHASet(m.items)
	ctx := m.ctx()
	return func() tea.Msg {
		warnings, err := g.DependencyWarnings(ctx, dir, ordered, selected)
		if err != nil {
			return depWarningsMsg{}
		}
		return depWarningsMsg{warnings: warnings}
	}
}

// sandboxWarnCmd runs the HU-004 unauthenticated-sandbox warning through the
// salesforce shim. Best-effort and skipped entirely when no SF client or
// alias is available.
func (m Model) sandboxWarnCmd(alias string) tea.Cmd {
	sf := m.deps.SF
	if sf == nil || alias == "" {
		return nil
	}
	ctx := m.ctx()
	return func() tea.Msg {
		warn, err := git.SandboxAuthWarning(ctx, sf, alias)
		if err != nil {
			return sandboxWarnMsg{warn: false}
		}
		return sandboxWarnMsg{warn: warn}
	}
}

// branchCreateCmd runs HU-005 fetch + promotion-branch creation.
func (m Model) branchCreateCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	target := m.plan.TargetBranch
	name := m.branchName
	ctx := m.ctx()
	return func() tea.Msg {
		err := g.CreatePromotionBranch(ctx, dir, target, name)
		return branchCreatedMsg{err: err}
	}
}

// cherryPickCmd runs the HU-006 sequencer-driven cherry-pick of the selected
// commits in ONE invocation (never a Go loop).
func (m Model) cherryPickCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	commits := m.plan.SelectedCommits
	contiguous := m.contiguous
	ctx := m.ctx()
	return func() tea.Msg {
		outcome, err := g.CherryPick(ctx, dir, commits, contiguous)
		return pickDoneMsg{outcome: outcome, err: err}
	}
}

// continueCmd runs `git cherry-pick --continue` through the gate.
func (m Model) continueCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		outcome, err := g.ContinueCherryPick(ctx, dir)
		return pickDoneMsg{outcome: outcome, err: err}
	}
}

// skipCmd runs `git cherry-pick --skip` for an empty pick.
func (m Model) skipCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		outcome, err := g.SkipCherryPick(ctx, dir)
		return pickDoneMsg{outcome: outcome, err: err}
	}
}

// abortCmd runs `git cherry-pick --abort`.
func (m Model) abortCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		return abortedMsg{err: g.AbortCherryPick(ctx, dir)}
	}
}

// repoStateCmd re-reads RepoState and the staged-marker set, computing the
// continue-gate. This is the poll that reconciles external actions.
func (m Model) repoStateCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		state, err := g.RepoState(ctx, dir)
		if err != nil {
			return repoStateMsg{err: err}
		}
		markers, err := g.StagedConflictMarkers(ctx, dir)
		if err != nil {
			return repoStateMsg{err: err}
		}
		return repoStateMsg{state: state, gate: git.EvaluateContinueGate(state, markers)}
	}
}

// verifyCmd runs HU-006 post-pick verification, wiring the H3-hardened
// VerifyPromotedContent: base = origin/<target>, selectedTip = the last
// SELECTED commit's SHA, selectedFiles = the union of files the selected set
// touched (each looked up through the git service, never an app-level exec).
func (m Model) verifyCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	selected := m.plan.SelectedCommits
	base := "origin/" + m.plan.TargetBranch
	ctx := m.ctx()
	return func() tea.Msg {
		if len(selected) == 0 {
			return verifyDoneMsg{}
		}
		selectedTip := selected[len(selected)-1].SHA

		seen := map[string]bool{}
		var files []string
		for _, c := range selected {
			if c.Merge {
				continue
			}
			touched, err := g.FilesTouchedByCommit(ctx, dir, c.SHA)
			if err != nil {
				return verifyDoneMsg{err: err}
			}
			for _, f := range touched {
				if !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}

		v, err := g.VerifyPromotedContent(ctx, dir, base, selectedTip, files)
		return verifyDoneMsg{verification: v, err: err}
	}
}

// tickCmd schedules the next re-poll tick.
func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return tickMsg{} })
}
