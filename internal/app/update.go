package app

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// Update is the Bubble Tea reducer. It derives the next state from the
// incoming message and, where a service call is needed, returns a command
// that routes through the injected services (never an app-level exec).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			// Cancel any in-flight poll so a read-only report subprocess is torn
			// down on exit (no-op outside polling; never cancels the SF job).
			m.cancelPoll()
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case prereqDoneMsg:
		return m.onPrereqDone(msg)
	case resumeDetectMsg:
		return m.onResumeDetect(msg)
	case discoverDoneMsg:
		return m.onDiscoverDone(msg)
	case rePromoteSeededMsg:
		return m.onRePromoteSeeded(msg)
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
	case queueDoneMsg:
		return m.onQueueDone(msg)
	case validateDoneMsg:
		return m.onValidateDone(msg)
	case cancelDoneMsg:
		return m.onCancelDone(msg)
	case reportDoneMsg:
		return m.onReportDone(msg)
	case pollTickMsg:
		return m.onPollTick()
	case pushDoneMsg:
		return m.onPushDone(msg)
	case prepDoneMsg:
		return m.onPrepDone(msg)
	case prCreatedMsg:
		return m.onPrCreated(msg)
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
	// Default to the normal flow, but fire HU-013 resume-detection: once it
	// lands, onResumeDetect may redirect to the resume offer (StateRunHistory).
	// resumeDetectCmd is nil when Git/Runs are absent, so this stays a plain
	// advance to ticket input for callers without those deps (e.g. unit tests).
	m.state = StateTicketInput
	return m, m.resumeDetectCmd()
}

// onResumeDetect lands the HU-013 startup resume-detection. It first resyncs
// any stale cherry-pick record against the live repo (a record claiming a
// cherry-pick phase while the repo shows no in-progress pick was resolved or
// aborted externally — repo is the source of truth, AC docs/HISTORIAS.md:847),
// then offers resume via StateRunHistory PRE-SELECTED on the newest resumable
// run. When nothing is resumable it proceeds to the normal flow without a
// blocking prompt; a detection error also falls back to the normal flow.
func (m Model) onResumeDetect(msg resumeDetectMsg) (tea.Model, tea.Cmd) {
	if m.state != StateTicketInput {
		// Detection is dispatched async from onPrereqDone (RepoState shells out to
		// git); a slow launch can let this message land AFTER the user already
		// advanced (typed a ticket, reached commit selection, ...). Acting now
		// would yank them back to StateTicketInput or hijack them to
		// StateRunHistory, discarding in-progress work. The resume offer is a
		// startup-only courtesy, so off the initial screen it is a strict no-op.
		return m, nil
	}
	if msg.err != nil {
		// Best-effort: a detection failure must never block startup.
		return m, nil
	}
	records := m.reconcileStaleRuns(msg.records, msg.state)
	m.runs = records
	m.repoState = msg.state
	idx, ok := firstResumable(records, msg.state)
	if !ok {
		m.state = StateTicketInput
		return m, nil
	}
	m.runsCursor = idx
	m.state = StateRunHistory
	return m, nil
}

// reconcileStaleRuns resyncs cherry-pick-phase records against the live repo:
// when the repo shows NO in-progress cherry-pick, any record still claiming a
// cherry-pick/git-conflict phase is stale (resolved or aborted outside
// DeployDeck), so it is marked aborted and persisted (best-effort Save) before
// the resumable set is computed — so a stale conflict is never offered. When
// the repo IS mid-pick nothing is resynced (the matching run is live). The
// returned slice mirrors the on-disk correction so the in-memory history
// agrees with the reconciled records.
func (m Model) reconcileStaleRuns(records []runs.Record, state git.RepoState) []runs.Record {
	if state.InProgress {
		return records
	}
	out := make([]runs.Record, len(records))
	copy(out, records)
	for i := range out {
		if !isCherryPickPhase(out[i].Phase) {
			continue
		}
		out[i].Phase = "aborted"
		out[i].UpdatedAt = m.now()
		if m.deps.Runs != nil {
			_ = m.deps.Runs.Save(out[i])
		}
	}
	return out
}

// resumeInto routes a resumable run directly into its live screen with no
// intermediate suspended state (run-resume spec: "Accepted Resume Routes
// Directly Into Conflict Or Polling"). A conflict-phase run matching the live
// in-progress cherry-pick rehydrates StateCherryPickConflict (ticket, pick N
// of M recomputed LIVE from RepoState via derivePickIndex — never the possibly
// stale persisted PickIndex — plus runID) and re-arms the reconciling poll; a
// non-terminal jobId run re-attaches StateValidationPolling exactly as
// onValidateDone does (jobID/runID/pollCtx re-armed, first reportCmd fired). A
// run that is neither (already terminal) is a no-op — the caller stays put.
func (m Model) resumeInto(rec runs.Record) (tea.Model, tea.Cmd) {
	switch {
	case m.repoState.InProgress && isCherryPickPhase(rec.Phase) && containsSHA(rec.Commits, m.repoState.CurrentSHA):
		m.runID = rec.RunID
		m.plan.Ticket = rec.Ticket
		m.plan.TargetBranch = rec.Target
		m.plan.SandboxAlias = rec.Alias
		// Reconstruct PromotionBranch the same way viewRunHistory displays it
		// (git.RenderBranchName over the config's BranchFormat). onBranchCreated
		// is the ONLY other writer of this field and is never revisited on
		// resume, so without this a resumed run that later reaches
		// StateSucceeded and pushes would run `git push -u origin ""` — an
		// invalid refspec (bug: HU-014 push after an HU-013 resume).
		m.plan.PromotionBranch = git.RenderBranchName(m.deps.Config.BranchFormat, rec.Ticket, rec.Target)
		// Rehydrate the selected set from the persisted SHAs so the resumed run
		// keeps its "pick N of M" (onPickDone recomputes from len) and still runs
		// post-pick verification over the selection (verifyCmd short-circuits to a
		// no-op on an empty set) — never zeroing progress or silently skipping the
		// HU-006 safety check on a continued/completed resume.
		m.plan.SelectedCommits = rehydrateSelectedCommits(rec.Commits)
		m.pickTotal = rec.PickTotal
		m.pickIndex = derivePickIndex(rec.PickTotal, m.repoState)
		m.state = StateCherryPickConflict
		return m, tea.Batch(m.repoStateCmd(), tickCmd())
	case rec.JobID != "" && !salesforce.IsTerminal(rec.Status):
		m.runID = rec.RunID
		m.jobID = rec.JobID
		m.plan.Ticket = rec.Ticket
		m.plan.TargetBranch = rec.Target
		m.plan.SandboxAlias = rec.Alias
		// Same reconstruction as the conflict-resume branch above — needed here
		// too so a jobId-reattached run that reaches StateSucceeded can push.
		m.plan.PromotionBranch = git.RenderBranchName(m.deps.Config.BranchFormat, rec.Ticket, rec.Target)
		m.validateErr = nil
		m.pollDeadline = m.now().Add(time.Duration(m.pollTimeoutSeconds()) * time.Second)
		m.state = StateValidationPolling
		// Arm the cancelable polling session exactly like onValidateDone: reportCmd
		// derives its per-call timeout from pollCtx, and any polling exit cancels
		// it. Fire only the FIRST report; onReportDone schedules the rest.
		m.pollCtx, m.pollCancel = context.WithCancel(context.Background())
		m.pollInFlight = true
		return m, m.reportCmd()
	default:
		return m, nil
	}
}

// isCherryPickPhase reports whether phase is one of the mid-Git phases a
// resume can re-enter or a resync must reconcile.
func isCherryPickPhase(phase string) bool {
	return phase == "cherry-pick" || phase == "git-conflict"
}

// containsSHA reports whether sha is one of the run's selected commit SHAs —
// how a live CHERRY_PICK_HEAD is matched back to the run that selected it.
func containsSHA(commits []string, sha string) bool {
	if sha == "" {
		return false
	}
	for _, c := range commits {
		if c == sha {
			return true
		}
	}
	return false
}

// isResumable reports whether rec can be resumed given the live repo state: a
// conflict-phase run whose selection holds the live CHERRY_PICK_HEAD, or a run
// with a non-terminal jobId (validation-progress spec: a terminal job stays
// browsable but is never offered for polling re-attach).
func isResumable(rec runs.Record, state git.RepoState) bool {
	if state.InProgress && isCherryPickPhase(rec.Phase) && containsSHA(rec.Commits, state.CurrentSHA) {
		return true
	}
	return rec.JobID != "" && !salesforce.IsTerminal(rec.Status)
}

// firstResumable returns the index of the newest resumable run (records are
// newest-first, List's contract) and whether one exists.
func firstResumable(records []runs.Record, state git.RepoState) (int, bool) {
	for i, rec := range records {
		if isResumable(rec, state) {
			return i, true
		}
	}
	return 0, false
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

// startRePromoteInto seeds a new promotion run from a prior successful run
// (HU-016 re-promotion). It structurally mirrors keyTicket's
// synchronous-seed-then-cmd shape — NOT resumeInto's single-shot resume:
// ticket and sourceRunID are seeded synchronously on the model, the
// next-environment default target is computed via git.NextEnvironmentBranch
// (commit-discovery spec: "Next-Environment Branch Suggested As Default
// Target"), and the interim StateCommitDiscovery (reused, no new state) is
// set while rePromoteRemapCmd runs the real patch-id remap asynchronously
// against origin/<next>..origin/<rec.Target> — its result lands via
// rePromoteSeededMsg, handled by onRePromoteSeeded.
//
// ok=false (currentTarget is already the last pipeline stage, e.g. prod, or
// an unconfigured/Release-* branch — design decision #4) has NO next
// environment to remap against: firing rePromoteRemapCmd anyway would build
// the ill-defined range "origin/..origin/<rec.Target>" and hard-fail with a
// real git error, landing the user on StateError. Instead this degrades to
// the NORMAL manual flow, mirroring keyTicket's Enter branch exactly — the
// ticket is pre-filled but sourceRunID is left unset (this degrade has no
// well-defined provenance link to the prior run) and the ordinary
// discoverCmd runs in place of the remap.
func (m Model) startRePromoteInto(rec runs.Record) (tea.Model, tea.Cmd) {
	next, ok := git.NextEnvironmentBranch(m.deps.Config, rec.Target)
	if !ok {
		m.ticket = rec.Ticket
		m.notice = "no next environment configured after " + rec.Target + "; continuing manually"
		m.prelim = preliminaryTarget(m.deps.Config)
		m.state = StateCommitDiscovery
		return m, m.discoverCmd()
	}

	m.ticket = rec.Ticket
	m.sourceRunID = rec.RunID
	m.prelim = next
	m.state = StateCommitDiscovery
	return m, m.rePromoteRemapCmd(rec.Commits, next, rec.Target)
}

// onRePromoteSeeded lands the HU-016 patch-id remap result. m.discovery is
// replaced with a FRESH git.DiscoverResult carrying only OrderedCommits =
// msg.Matched (review remediation, Finding 4b) — never mutated field-by-field
// on whatever a prior in-session discovery left behind, so stale sibling
// fields (e.g. Alternatives, CandidateBranches from an earlier no-results
// search) can never leak into viewSelection's rendering. OrderedCommits
// drives confirmSelection's git.IsContiguousSelection check (and the
// dependency-warning command below) against the real reused set
// (re-promotion spec: "Prior-Run Commits Pre-Loaded And Editable").
// NewCommitSelectionItems pre-checks every matched commit while keeping it
// fully toggleable, reusing the exact same selection machinery ordinary
// discovery uses. Unmatched is held as rePromoteMissing for the view's
// explicit warning (re-promotion spec: "Missing Commit Warned Explicitly" —
// never silently dropped). A hard remap error surfaces like every other
// command failure, landing on StateError.
func (m Model) onRePromoteSeeded(msg rePromoteSeededMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	m.discovery = git.DiscoverResult{OrderedCommits: msg.Matched}
	m.items = git.NewCommitSelectionItems(m.discovery.OrderedCommits, m.ticket)
	m.rePromoteMissing = msg.Unmatched
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

	// HU-013: create the run record IMMEDIATELY at branch creation — before
	// any pick runs — so a crash mid-cherry-pick still leaves enough
	// persisted context (Ticket/PickTotal/Commits) to offer resume (design.md:
	// "Resume needs Ticket + PickTotal persisted DURING cherry-pick, before
	// any jobId exists"). Best-effort: a nil Runs writer or a write hiccup
	// must never block the flow, mirroring every other Runs call site.
	m.runID = m.plan.Ticket + "-to-" + m.plan.TargetBranch + "-" + m.now().Format("20060102150405")
	if m.deps.Runs != nil {
		now := m.now()
		_ = m.deps.Runs.Save(runs.Record{
			RunID:       m.runID,
			Ticket:      m.plan.Ticket,
			Target:      m.plan.TargetBranch,
			Alias:       m.plan.SandboxAlias,
			Commits:     commitSHAs(m.plan.SelectedCommits),
			PickTotal:   len(m.plan.SelectedCommits),
			Phase:       "cherry-pick",
			CreatedAt:   now,
			UpdatedAt:   now,
			SourceRunID: m.sourceRunID,
		})
	}

	return m, tea.Batch(m.cherryPickCmd(), tickCmd())
}

// derivePickIndex computes the 1-based ordinal of the pick CURRENTLY applying
// (in progress or conflicted), from the total selected commits and the live
// RepoState. See design.md's locked formula: .git/sequencer/todo INCLUDES the
// commit CHERRY_PICK_HEAD already holds, so the completed count is
// pickTotal-SequencerRemaining and the current (conflicting) pick is that+1.
// Both edges are clamped: a completed sequence (!InProgress) returns
// pickTotal; a single/last pick whose sequencer file is entirely absent
// (SequencerRemaining==0) still clamps into [1,pickTotal], never overflowing.
func derivePickIndex(pickTotal int, st git.RepoState) int {
	if !st.InProgress {
		return pickTotal
	}
	idx := pickTotal - st.SequencerRemaining + 1
	if idx < 1 {
		idx = 1
	}
	if idx > pickTotal {
		idx = pickTotal
	}
	return idx
}

// saveRunProgress best-effort Loads the current run's persisted record and
// applies mutate to it before Saving, preserving every field mutate does not
// touch (CreatedAt, Ticket, PickTotal, Commits, ...) — the same
// Load-then-Save merge pattern validateCmd uses to reuse the run created at
// branch creation. A nil Runs writer, an empty runID, or a Load failure is a
// silent no-op: HU-013's progress persistence is best-effort, mirroring
// onReportDone's AppendReport (a write hiccup must never sink the live flow).
func (m Model) saveRunProgress(mutate func(rec *runs.Record)) {
	if m.deps.Runs == nil || m.runID == "" {
		return
	}
	rec, err := m.deps.Runs.Load(m.runID)
	if err != nil {
		return
	}
	mutate(&rec)
	_ = m.deps.Runs.Save(rec)
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
		// resolution/abort reconciles live. Persist the conflict's pick
		// progress (HU-013 "pick N of M") so a crash here still leaves
		// enough context to offer resume.
		idx := derivePickIndex(len(m.plan.SelectedCommits), msg.outcome.State)
		// Hold the live "pick N of M" on the model so the conflict screen shows
		// it for a fresh sequence, identically to a resumed one (cherry-pick
		// spec: "Resumed Entry Accepts Rehydrated Conflict Context").
		m.pickIndex = idx
		m.pickTotal = len(m.plan.SelectedCommits)
		m.saveRunProgress(func(rec *runs.Record) {
			rec.PickIndex = idx
			rec.PickTotal = len(m.plan.SelectedCommits)
			rec.CurrentCommit = msg.outcome.State.CurrentSHA
			rec.Phase = "git-conflict"
			rec.UpdatedAt = m.now()
		})
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
	m.saveRunProgress(func(rec *runs.Record) {
		rec.Phase = "aborted"
		rec.UpdatedAt = m.now()
	})
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
	m.saveRunProgress(func(rec *runs.Record) {
		rec.Phase = "done"
		rec.UpdatedAt = m.now()
	})
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

// onQueueDone lands the HU-009 deploy-queue outcome. A Tooling-API-permission
// failure (ErrQueuePermission) is a non-blocking degrade: it warns via
// m.notice and skips straight to ValidationStart, firing validateCmd — the
// queue view is never shown for this reason. A generic failure keeps the
// user on QueueReview with the error surfaced (non-aborting: `r` retries,
// `esc` backs out). Success stores the parsed entries and the resolved
// own-job identity for the view's highlight + approximate position.
func (m Model) onQueueDone(msg queueDoneMsg) (tea.Model, tea.Cmd) {
	m.identity = msg.identity
	if msg.err != nil {
		if errors.Is(msg.err, salesforce.ErrQueuePermission) {
			m.queue = nil
			m.queueErr = nil
			m.notice = "no Tooling API permission to view the deploy queue; continuing without it"
			m.validateErr = nil
			m.state = StateValidationStart
			return m, m.validateCmd()
		}
		m.queueErr = msg.err
		m.state = StateQueueReview
		return m, nil
	}
	m.queueErr = nil
	m.queue = msg.entries
	m.state = StateQueueReview
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
	// Arm the cancelable polling session context; reportCmd derives each poll's
	// per-call timeout from it, and any polling exit (terminal, timeout, quit)
	// cancels it to tear down an in-flight read-only report subprocess.
	m.pollCtx, m.pollCancel = context.WithCancel(context.Background())
	// Fire the FIRST report only. The next poll is scheduled by onReportDone
	// once this one returns (sequential polling, no free-running tick), so the
	// loop never runs two reports concurrently.
	m.pollInFlight = true
	return m, m.reportCmd()
}

// onCancelDone lands the HU-012 cancel outcome. It is GUARDED to act only while
// StateCancelConfirm (symmetric to onReportDone's polling guard): a late or
// duplicate cancelDoneMsg arriving after we already left the confirm screen
// (e.g. already terminal StateCanceled) is dropped, so it can never re-mark or
// clobber the terminal state. On success it tears down any in-flight read-only
// report subprocess (cancelPoll), persists the cancel via MarkCanceled
// (cancel.json + run.json Status=Canceled — best-effort, mirroring the polling
// persistence: a write hiccup must not undo an already-succeeded SF cancel), and
// moves to the terminal StateCanceled. On failure it surfaces the error and
// stays on StateCancelConfirm with the run NOT marked (validation-cancel spec:
// "a failed cancel leaves the run untouched").
func (m Model) onCancelDone(msg cancelDoneMsg) (tea.Model, tea.Cmd) {
	if m.state != StateCancelConfirm {
		// A late/duplicate cancel after we already left the confirm screen is
		// ignored — it must never re-mark the run or clobber the terminal state.
		return m, nil
	}
	if msg.err != nil {
		m.cancelErr = msg.err
		m.state = StateCancelConfirm
		return m, nil
	}
	m.cancelErr = nil
	// Leaving the polling session for the terminal cancel screen: cancel the
	// session context so no read-only report subprocess lingers.
	m.cancelPoll()
	if m.deps.Runs != nil && m.runID != "" {
		_ = m.deps.Runs.MarkCanceled(m.runID, []byte(msg.result.Raw))
	}
	m.state = StateCanceled
	return m, nil
}

// onReportDone processes one HU-011 poll. The outstanding report has returned,
// so the in-flight guard is cleared and — for a non-terminal result or a
// transient error, still within the deadline — the NEXT poll is scheduled here
// (never by a free-running tick), keeping polling strictly sequential. Every
// raw report is persisted (success OR errored poll: the spec saves each raw
// response), and a terminal status maps to its terminal screen and stops the
// loop.
func (m Model) onReportDone(msg reportDoneMsg) (tea.Model, tea.Cmd) {
	// The outstanding report has genuinely returned, so it is no longer in
	// flight regardless of the current screen: clear the guard BEFORE the
	// stale-message drop below. A report commonly lands while the user is on
	// StateCancelConfirm (the in-flight poll returns while they decide whether to
	// type CANCELAR). If the state guard returned first, pollInFlight would stay
	// stuck true, and a later esc back to StateValidationPolling could never
	// re-arm the loop — onPollTick and the manual `r` refresh both no-op while
	// pollInFlight — permanently freezing live progress. Clearing it here is
	// always correct (the report goroutine already returned, so it can never
	// cause a double report) and cannot double-poll: the successor tick is only
	// scheduled below, after the state guard, while still ValidationPolling.
	m.pollInFlight = false

	if m.state != StateValidationPolling {
		// A late/stale report after we already left polling is dropped, but the
		// in-flight guard was cleared above so esc-resume and manual `r` still work.
		return m, nil
	}

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
		// Leaving polling: cancel the session context so no read-only report
		// subprocess lingers past the terminal screen.
		m.cancelPoll()
		m.state = terminalState(msg.report.Status)
		return m, nil
	}
	return m.scheduleNextPoll()
}

// scheduleNextPoll enforces the hard deadline, then arms the NEXT poll tick.
// Past the deadline the run fails (timeout, StateFailed) and the session
// context is cancelled; otherwise a single pollTickCmd is scheduled. This is
// the ONLY place a poll tick is armed, so ticks can never accumulate
// independently of report completion.
func (m Model) scheduleNextPoll() (tea.Model, tea.Cmd) {
	if m.now().After(m.pollDeadline) {
		m.cancelPoll()
		m.timedOut = true
		m.state = StateFailed
		return m, nil
	}
	return m, pollTickCmd(m.pollIntervalSeconds())
}

// onPushDone lands the HU-014 push outcome. A push failure keeps the user on
// the push-confirm screen with the error shown (pushErr) — the flow never
// crashes into a terminal error state (push-failure UX: retry/quit). A
// successful push fires preparePRCmd (RemoteURL + gh AuthStatus); the phase
// stays pushPushing until that prep lands in onPrepDone.
func (m Model) onPushDone(msg pushDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.pushErr = msg.err
		m.pushPhase = pushConfirm
		return m, nil
	}
	m.pushErr = nil
	return m, m.preparePRCmd()
}

// onPrepDone lands the HU-014 PR-preparation data (gh detection + compare URL)
// and settles the screen on pushReady, where the view shows base/compare/title
// and branches on authState (authed → offer PR; absent/unauthenticated → show
// the compare URL, or the raw origin + manual data when it could not be
// derived).
func (m Model) onPrepDone(msg prepDoneMsg) (tea.Model, tea.Cmd) {
	m.authState = msg.auth
	m.originURL = msg.originURL
	m.compareURL = msg.compareURL
	m.compareErr = msg.compareErr
	m.remoteErr = msg.remoteErr
	m.pushPhase = pushReady
	return m, nil
}

// onPrCreated lands the HU-014 `gh pr create` outcome. A failure surfaces the
// error (prErr) beside the manual base/compare/title data and the flow
// continues on pushReady (spec: "PR creation failure does not abort the
// flow"). On success it records the URL on the run via MarkPRCreated
// (best-effort, mirroring every other Runs call site — a write hiccup must not
// sink the just-created PR) and shows the URL.
func (m Model) onPrCreated(msg prCreatedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.prErr = msg.err
		m.prURL = ""
		m.pushPhase = pushReady
		return m, nil
	}
	m.prErr = nil
	m.prURL = msg.url
	m.pushPhase = pushReady
	if m.deps.Runs != nil && m.runID != "" {
		_ = m.deps.Runs.MarkPRCreated(m.runID, msg.url)
	}
	return m, nil
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
		m.cancelPoll()
		m.timedOut = true
		m.state = StateFailed
		return m, nil
	}
	m.pollInFlight = true
	return m, m.reportCmd()
}
