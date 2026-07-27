package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/salesforce"
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
	case deltaDoneMsg:
		return m.onDeltaDone(msg)
	case validateDoneMsg:
		return m.onValidateDone(msg)
	case reportDoneMsg:
		return m.onReportDone(msg)
	case pollTickMsg:
		return m.onPollTick()
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

// onDeltaDone lands the HU-007 delta result. An sgd/parse failure keeps the
// user on DeltaGeneration with the raw output shown and launches NO validation
// (delta-generation spec: "sgd failure surfaces output without running
// validation"). Success registers the artifact paths on the plan and advances
// to the HU-008 PackageReview.
func (m Model) onDeltaDone(msg deltaDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.deltaErr = msg.err
		m.state = StateDeltaGeneration
		return m, nil
	}
	m.deltaErr = nil
	m.deltaResult = msg.result
	m.summary = msg.summary
	m.plan = git.RegisterDeltaArtifacts(m.plan, msg.result.PackageXMLPath, msg.result.DestructiveChangesPath)
	m.state = StatePackageReview
	return m, nil
}

// onValidateDone lands the HU-010 validate outcome. A CLI error (or a
// persistence failure) keeps the flow alive on ValidationStart with the
// message + raw shown — never a crash or a terminal error state. Success holds
// the jobId, arms the hard poll deadline from the injected clock, and begins
// polling with an immediate first report plus the tick cadence.
func (m Model) onValidateDone(msg validateDoneMsg) (tea.Model, tea.Cmd) {
	m.jobID = msg.result.JobID
	m.runID = msg.runID
	m.runDir = msg.runDir
	if msg.err != nil {
		m.validateErr = msg.err
		m.state = StateValidationStart
		return m, nil
	}
	m.validateErr = nil
	m.pollDeadline = m.now().Add(time.Duration(m.pollTimeoutSeconds()) * time.Second)
	m.state = StateValidationPolling
	// Fire the FIRST report only. The next poll is scheduled by onReportDone
	// once this one returns (sequential polling, no free-running tick), so the
	// loop never runs two reports concurrently.
	m.pollInFlight = true
	return m, m.reportCmd()
}

// onReportDone processes one HU-011 poll. The outstanding report has returned,
// so the in-flight guard is cleared and — for a non-terminal result or a
// transient error, still within the deadline — the NEXT poll is scheduled here
// (never by a free-running tick), keeping polling strictly sequential. Every
// raw report is persisted (success OR errored poll: the spec saves each raw
// response), and a terminal status maps to its terminal screen and stops the
// loop.
func (m Model) onReportDone(msg reportDoneMsg) (tea.Model, tea.Cmd) {
	if m.state != StateValidationPolling {
		// A late report after we already left polling is ignored.
		return m, nil
	}

	// The outstanding report has returned: clear the guard so the loop can
	// schedule exactly one successor (and a manual refresh is allowed again).
	m.pollInFlight = false

	// Persist every raw report the poll produced — success OR a transient error
	// that still returned output (a non-parseable non-zero-exit report keeps
	// its Raw) — so the run retains a full trail (spec: "each raw report
	// saved"). Preserve the last known status so an errored, status-less poll
	// never clobbers run.json's status. Best-effort: a write hiccup must not
	// sink the live poll; the job keeps running and the next poll re-persists.
	if m.deps.Runs != nil && m.runID != "" && msg.report.Raw != "" {
		status := msg.report.Status
		if status == "" {
			status = m.report.Status
		}
		_ = m.deps.Runs.AppendReport(m.runID, status, []byte(msg.report.Raw))
	}

	if msg.err != nil {
		// Transient error: retry on the next poll, still within the deadline.
		m.reportErr = msg.err
		return m.scheduleNextPoll()
	}
	m.reportErr = nil
	m.report = msg.report

	if salesforce.IsTerminal(msg.report.Status) {
		m.state = terminalState(msg.report.Status)
		return m, nil
	}
	return m.scheduleNextPoll()
}

// scheduleNextPoll enforces the hard deadline, then arms the NEXT poll tick.
// Past the deadline the run fails (timeout, StateFailed); otherwise a single
// pollTickCmd is scheduled. This is the ONLY place a poll tick is armed, so
// ticks can never accumulate independently of report completion.
func (m Model) scheduleNextPoll() (tea.Model, tea.Cmd) {
	if m.now().After(m.pollDeadline) {
		m.timedOut = true
		m.state = StateFailed
		return m, nil
	}
	return m, pollTickCmd(m.pollIntervalSeconds())
}

// onPollTick fires the NEXT report ONLY while ValidationPolling, no report is
// already in flight, and the hard deadline has not passed. It never re-arms
// another tick: rescheduling happens in onReportDone after the report returns,
// so ticks cannot pile up. Any other state — or an already-in-flight report
// (e.g. a manual refresh just fired one) — stops here (nil), leaking no
// background polling past a terminal state or a user exit.
func (m Model) onPollTick() (tea.Model, tea.Cmd) {
	if m.state != StateValidationPolling {
		return m, nil
	}
	if m.pollInFlight {
		// A report is still outstanding; onReportDone will reschedule the loop.
		return m, nil
	}
	if m.now().After(m.pollDeadline) {
		m.timedOut = true
		m.state = StateFailed
		return m, nil
	}
	m.pollInFlight = true
	return m, m.reportCmd()
}
