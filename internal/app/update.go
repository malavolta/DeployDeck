package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// cancelErrorCompanionFilename/quickErrorCompanionFilename are the
// deploy-error-detail (gap7) companion filenames SaveRawCompanion writes a
// GENERIC (non-already-terminal) cancel/quick-deploy failure's raw response
// under, mirroring the cancel.json/quick.json success-companion naming.
// Always compile-time consts, never derived from user input (design.md
// Threat Matrix "Companion file writes").
const (
	cancelErrorCompanionFilename = "cancel-error.json"
	quickErrorCompanionFilename  = "quick-error.json"
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
	case updateCheckDoneMsg:
		return m.onUpdateCheckDone(msg)
	case aiSuggestDoneMsg:
		return m.onAISuggestDone(msg)
	case resumeDetectMsg:
		return m.onResumeDetect(msg)
	case originalBranchMsg:
		return m.onOriginalBranch(msg)
	case unpushedMsg:
		return m.onUnpushedCount(msg)
	case deployBranchesMsg:
		return m.onDeployBranches(msg)
	case standaloneBranchesMsg:
		return m.onStandaloneBranches(msg)
	case deleteDoneMsg:
		return m.onDeleteDone(msg)
	case pruneDoneMsg:
		return m.onPruneDone(msg)
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
	case reuseReadyMsg:
		return m.onReuseReady(msg)
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
	case spinnerTickMsg:
		return m.onSpinnerTick()
	case deltaDoneMsg:
		return m.onDeltaDone(msg)
	case queueDoneMsg:
		return m.onQueueDone(msg)
	case validateDoneMsg:
		return m.onValidateDone(msg)
	case cancelDoneMsg:
		return m.onCancelDone(msg)
	case quickDeployDoneMsg:
		return m.onQuickDeployDone(msg)
	case gateCheckDoneMsg:
		return m.onGateCheckDone(msg)
	case reportDoneMsg:
		return m.onReportDone(msg)
	case postCommentDoneMsg:
		return m.onPostCommentDone(msg)
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
	// HU-018 (design ADR-1): the no-resume landing is the main menu, NOT ticket
	// input — the "menú principal" entry point from which the full promotion
	// flow and the two standalone modes are reached. Fire HU-013 resume-detection
	// AND HU-017's startup branch capture (design.md "onPrereqDone returns
	// tea.Batch(resumeDetectCmd(), originalBranchCmd())"): once resumeDetectMsg
	// lands, onResumeDetect may redirect to the resume offer (StateRunHistory) —
	// its guard now keys off StateMainMenu so a resumable run is still offered
	// after this landing (HU-013 preserved); once originalBranchMsg lands,
	// onOriginalBranch captures the branch quitCmd will later restore. Both
	// degrade to nil when their deps are absent, and tea.Batch drops nil
	// commands cleanly, so this stays a plain advance to the menu for callers
	// without those deps (e.g. unit tests).
	m.state = StateMainMenu
	return m, tea.Batch(m.resumeDetectCmd(), m.originalBranchCmd())
}

// onUpdateCheckDone lands the HU-019 update-availability result (design
// ADR-3: silent skip on any failure). A check error and "no newer version"
// are treated identically — a plain no-op — so startup is never gated or
// nagged; only a genuine newer release records the notice fields
// View()'s updateBanner() reads.
func (m Model) onUpdateCheckDone(msg updateCheckDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil || !msg.hasUpdate {
		return m, nil
	}
	m.updateAvailable = true
	m.updateLatest = msg.latest
	return m, nil
}

// onAISuggestDone lands the ai-pr-summary suggestion request's result
// (task 4.6/4.7). It ALWAYS clears aiPending first (the in-flight guard),
// then degrades an error or an empty title identically to a silent no-op —
// aiErr is recorded for internal bookkeeping only, no title/description
// change occurs, and the flow continues unaffected (spec: "Silent Graceful
// Degradation"). A non-empty title sets aiTitle/aiDescription as a
// PROPOSED suggestion; aiAccepted is left untouched — only the explicit
// second 'a' press (keyPushPreparation) ever sets it.
func (m Model) onAISuggestDone(msg aiSuggestDoneMsg) (tea.Model, tea.Cmd) {
	m.aiPending = false

	if msg.err != nil || msg.title == "" {
		m.aiErr = msg.err
		return m, nil
	}

	m.aiErr = nil
	m.aiTitle = msg.title
	m.aiDescription = msg.description
	return m, nil
}

// onOriginalBranch lands the HU-017 startup branch capture. A capture error
// is a best-effort no-op — m.originalBranch stays "", which quitCmd's
// shouldRestore guard already treats as a skip-restore signal — so it must
// never block or crash the flow.
func (m Model) onOriginalBranch(msg originalBranchMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	m.originalBranch = msg.branch
	return m, nil
}

// onUnpushedCount lands HU-017's unpushedCountCmd result, fired by
// keySucceeded/keyAborted's `d` (the run's own branch) or keyBranchCleanup's
// `d` (a captured orphan row). Any real unpushed commit — or a resolution
// failure, treated conservatively as "cannot confirm pushed" — requires the
// strong typed-BORRAR confirmation; a fully pushed branch (count == 0) only
// needs the normal 'y' confirm. The deleteConfirm buffer and any stale notice
// are reset here so a PRIOR failed attempt on a different branch can never leak
// into a new one.
//
// Two staleness guards protect the destructive gate:
//   - Review M-2: the result is applied ONLY while the model is on a
//     delete-capable screen (StateSucceeded / StateAborted / StateBranchCleanup).
//     A count landing after the user left those screens (e.g. a d+esc on the
//     cleanup screen followed by a normal promotion) is dropped, so it can
//     never leave a terminal screen routing stray keys into keyDeleteConfirm.
//   - Review H-1: the count is authoritative only for the branch it was
//     requested FOR. The target is the batch screen's CAPTURED row
//     (cleanupDeleteTarget) or, on a terminal screen, the run's own
//     plan.PromotionBranch. A count for any other branch (moved cursor, prior
//     d+esc) is dropped rather than gating the confirm strength of a DIFFERENT
//     branch.
func (m Model) onUnpushedCount(msg unpushedMsg) (tea.Model, tea.Cmd) {
	var target string
	switch m.state {
	case StateBranchCleanup:
		target = m.cleanupDeleteTarget
	case StateSucceeded, StateAborted:
		target = m.plan.PromotionBranch
	default:
		return m, nil
	}
	if msg.branch != target {
		return m, nil
	}
	m.deleteConfirm = ""
	m.notice = ""
	m.cleanupNotice = ""
	if msg.err != nil || msg.count > 0 {
		m.cleanupPhase = cleanupStrongConfirm
	} else {
		m.cleanupPhase = cleanupConfirm
	}
	return m, nil
}

// resetCleanupState clears any in-flight inline/batch delete confirmation so a
// stale cleanupPhase can never leak onto a freshly-entered terminal screen
// (review M-2): a late unpushed-count landing after the user left the cleanup
// screen must not leave a terminal screen routing stray keys into
// keyDeleteConfirm (which could delete plan.PromotionBranch at a wrong-branch
// confirm strength). Called on every transition INTO a terminal state.
func (m Model) resetCleanupState() Model {
	m.cleanupPhase = cleanupIdle
	m.pendingDeleteCurrent = false
	m.deleteConfirm = ""
	m.cleanupDeleteTarget = ""
	m.cleanupDeleteTargetPushed = false
	return m
}

// onDeployBranches lands HU-017's listDeployBranchesCmd result: an error
// surfaces as a cleanupNotice (never a crash) with an empty list — indistinguishable
// in the view from a genuinely empty orphan set, both rendering the same
// explicit empty state; success populates cleanupBranches at cursor 0. Either
// way cleanupPhase settles on cleanupBrowsing, so the screen is never stuck
// on cleanupLoading.
func (m Model) onDeployBranches(msg deployBranchesMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.cleanupNotice = "no se pudieron listar las ramas: " + msg.err.Error()
		m.cleanupBranches = nil
		m.cleanupCursor = 0
		m.cleanupPhase = cleanupBrowsing
		return m, nil
	}
	m.cleanupNotice = ""
	m.cleanupBranches = msg.branches
	m.cleanupCursor = 0
	m.cleanupPhase = cleanupBrowsing
	return m, nil
}

// onStandaloneBranches lands HU-018's standaloneBranchesCmd result (Group 3,
// StateDeltaSourceSelect): an error surfaces as a notice (never a crash,
// mirrors onDeployBranches's degrade); success populates m.branchList at
// cursor 0, ready for keyDeltaSourceSelect's nav+confirm.
func (m Model) onStandaloneBranches(msg standaloneBranchesMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = "no se pudieron listar las ramas: " + msg.err.Error()
		m.branchList = nil
		m.branchCursor = 0
		return m, nil
	}
	m.notice = ""
	m.branchList = msg.branches
	m.branchCursor = 0
	return m, nil
}

// onDeleteDone lands HU-017's per-row deleteOrphanCmd outcome. A failure
// surfaces as a cleanupNotice with the list left untouched (the caller can
// retry); success reloads the list (listDeployBranchesCmd) so the deleted
// branch disappears and any externally-changed state reconciles, mirroring
// resumeDetectCmd's repo-is-source-of-truth posture.
func (m Model) onDeleteDone(msg deleteDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.cleanupNotice = "no se pudo borrar la rama: " + msg.err.Error()
		return m, nil
	}
	m.cleanupNotice = ""
	m.cleanupPhase = cleanupLoading
	return m, m.listDeployBranchesCmd()
}

// onPruneDone lands HU-017's retention-prune outcome (branch-cleanup spec:
// "Run Retention Applied From The Cleanup Surface"), reporting the pruned
// count via cleanupNotice. A failure surfaces its own error the same way.
func (m Model) onPruneDone(msg pruneDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.cleanupNotice = "no se pudo aplicar la retención: " + msg.err.Error()
		return m, nil
	}
	m.cleanupNotice = fmt.Sprintf("%d run(s) eliminados por retención", len(msg.removed))
	return m, nil
}

// onResumeDetect lands the HU-013 startup resume-detection. It first resyncs
// any stale cherry-pick record against the live repo (a record claiming a
// cherry-pick phase while the repo shows no in-progress pick was resolved or
// aborted externally — repo is the source of truth, AC docs/HISTORIAS.md:847),
// then offers resume via StateRunHistory PRE-SELECTED on the newest resumable
// run. When nothing is resumable it proceeds to the normal flow without a
// blocking prompt; a detection error also falls back to the normal flow.
func (m Model) onResumeDetect(msg resumeDetectMsg) (tea.Model, tea.Cmd) {
	if m.state != StateMainMenu {
		// Detection is dispatched async from onPrereqDone (RepoState shells out to
		// git); a slow launch can let this message land AFTER the user already
		// advanced off the landing (opened a mode, typed a ticket, ...). Acting
		// now would yank them back or hijack them to StateRunHistory, discarding
		// in-progress work. The resume offer is a startup-only courtesy, so off
		// the initial menu landing it is a strict no-op. HU-018 (design ADR-1):
		// this guard MUST key off StateMainMenu — the new no-resume landing — or
		// a resumable run detected right after startup would be silently dropped
		// (the landing would look like "user already advanced"), killing HU-013.
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
		// Nothing resumable: stay on the HU-018 menu landing (was
		// StateTicketInput pre-HU-018), symmetric with onPrereqDone.
		m.state = StateMainMenu
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
		// EXCEPT a standalone-validate run (HU-018): it has no promotion branch
		// by construction (Ticket/Target are both empty), so rendering one here
		// would fabricate a bogus non-empty branch like "deploy/-to-" — defeating
		// R3's `PromotionBranch != ""` gate at the terminal StateSucceeded screen
		// (adversarial-review W1). Leave PromotionBranch empty in that case; the
		// run still resumes to re-poll its validation jobId exactly as before.
		if rec.Mode != "validate" {
			m.plan.PromotionBranch = git.RenderBranchName(m.deps.Config.BranchFormat, rec.Ticket, rec.Target)
		}
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
	if msg.confirm {
		// Pass 1 needs-confirm: park on StateSourceConfirm, no items yet.
		m.state = StateSourceConfirm
		return m, nil
	}
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
		if errors.Is(msg.err, git.ErrPromotionBranchExists) {
			// incremental-promotion: never dead-end on a name collision —
			// offer reuse/recreate/cancel instead of the generic terminal
			// error (spec: "SHALL NOT dead-end into an unrecoverable error
			// state on this collision").
			m.notice = ""
			m.state = StateBranchCollision
			return m, nil
		}
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
			TestLevel:   m.plan.TestLevel,
		})
	}

	m, spin := m.startSpinner()
	return m, tea.Batch(m.cherryPickCmd(), tickCmd(), spin)
}

// onReuseReady lands incremental-promotion's reuseBranchCmd outcome
// (StateBranchCollision's "reuse & append"), following design.md's Data
// Flow exactly:
//
//   - FFDiverged or a genuine command error: a clear, terminal StateError —
//     NEVER a push/overwrite attempt (spec: "Diverged Remote Deploy Branch
//     Blocks With A Clear Error").
//   - an empty remainder (every selected commit already present): an
//     explicit notice, then StatePushPreparation — never a raw cherry-pick
//     error (spec: "Empty-After-Filter Shows An Explicit Notice").
//   - a non-empty remainder: m.reusing is set true, the selection is
//     narrowed to JUST the remainder (so cherryPickCmd never re-picks an
//     already-present commit), m.contiguous is forced false (the filtered
//     remainder is not guaranteed contiguous in the original ordered range;
//     CherryPick's own range-verification would fall back safely regardless,
//     but this skips that extra check), and the target run record is
//     resolved via runs.FindRunForBranch: a match is mutated IN PLACE
//     (Commits extended, PickTotal/UpdatedAt bumped, CreatedAt/RunID/
//     SourceRunID left untouched — spec: "Incremental Append Mutates The
//     Same Run Record In Place"); no match seeds a fresh run exactly like
//     onBranchCreated's own first-time path (spec: "No prior run found is
//     treated as a fresh increment target").
func (m Model) onReuseReady(msg reuseReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.state = StateError
		return m, nil
	}
	if msg.ffResult == git.FFDiverged {
		m.err = fmt.Errorf("la rama de despliegue remota diverge de la copia local; resuélvelo manualmente antes de reusarla")
		m.state = StateError
		return m, nil
	}

	m.reusing = true
	m.plan = git.RegisterPromotionBranch(m.plan, m.branchName)
	m.plan.SelectedCommits = msg.remaining
	m.contiguous = false

	if len(msg.remaining) == 0 {
		m.notice = "nada nuevo para aplicar: todos los commits seleccionados ya están en la rama"
		m.pushErr = nil
		m.prErr = nil
		m.prURL = ""
		m.prExisting = false
		m.pushPhase = pushConfirm
		m.state = StatePushPreparation
		return m, nil
	}

	now := m.now()
	if rec, ok := runs.FindRunForBranch(m.runs, m.deps.Config.BranchFormat, m.branchName); ok {
		rec.Commits = append(rec.Commits, commitSHAs(msg.remaining)...)
		rec.PickTotal = len(rec.Commits)
		rec.Phase = "cherry-pick"
		rec.UpdatedAt = now
		m.runID = rec.RunID
		if m.deps.Runs != nil {
			_ = m.deps.Runs.Save(rec)
		}
	} else {
		m.runID = m.plan.Ticket + "-to-" + m.plan.TargetBranch + "-" + now.Format("20060102150405")
		if m.deps.Runs != nil {
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
				TestLevel:   m.plan.TestLevel,
			})
		}
	}

	m.state = StateCherryPicking
	m, spin := m.startSpinner()
	return m, tea.Batch(m.cherryPickCmd(), tickCmd(), spin)
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
	// Review M-2: reset any leaked delete-confirmation state on entry into the
	// terminal StateAborted screen (symmetric with onReportDone's terminal path).
	m = m.resetCleanupState()
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

// isSpinnerState reports whether state is one of the 5 long-running states
// the progress spinner animates during (design D5's Data Flow / Testing
// Strategy): CommitDiscovery, BranchCreation, CherryPicking, DeltaGeneration,
// ValidationStart.
func isSpinnerState(state State) bool {
	switch state {
	case StateCommitDiscovery, StateBranchCreation, StateCherryPicking, StateDeltaGeneration, StateValidationStart:
		return true
	default:
		return false
	}
}

// onSpinnerTick bumps m.spinnerFrame and reschedules the next tick ONLY
// while still in one of the 5 spinner states (mirrors onTick's own
// state-scoped guard exactly) — so the animation stops the instant the flow
// leaves those screens, never leaking a background tick past them.
func (m Model) onSpinnerTick() (tea.Model, tea.Cmd) {
	if !isSpinnerState(m.state) {
		// The single continuing loop dies here: clear the single-flight guard
		// so the NEXT spinner-state entry (startSpinner) is free to reseed a
		// fresh tick loop instead of being wrongly blocked by a stale true.
		m.spinning = false
		return m, nil
	}
	m.spinnerFrame++
	return m, spinnerCmd()
}

// onDeltaDone lands the HU-007 delta result. An sgd/parse failure keeps the
// user on DeltaGeneration with the raw output shown and launches NO validation
// (delta-generation spec: "sgd failure surfaces output without running
// validation"). Success registers the artifact paths on the plan and advances
// to the HU-008 PackageReview. HU-018 standalone delta (design ADR-4) also
// persists a local run the instant the package is generated — mirroring
// onBranchCreated's best-effort, nil-Runs-safe save — tagged Mode="delta"
// with no jobId, since standalone delta never validates.
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

	if m.standaloneMode == "delta" && m.deps.Runs != nil {
		now := m.now()
		m.runID = "delta-" + m.plan.TargetBranch + "-" + now.Format("20060102150405")
		_ = m.deps.Runs.Save(runs.Record{
			RunID:        m.runID,
			Ticket:       m.plan.Ticket,
			Target:       m.plan.TargetBranch,
			Mode:         "delta",
			ManifestPath: msg.result.PackageXMLPath,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

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
			m, spin := m.startSpinner()
			return m, tea.Batch(m.validateCmd(), spin)
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
	// Preserve a pre-created runID across a validate error (adversarial-review
	// Finding 2): an error carries runID=="", and clearing the id would orphan
	// the already-persisted Mode="validate" run and send a retry down
	// validateCmd's malformed fallback branch. Only overwrite when the message
	// actually carries a run id.
	if msg.runID != "" {
		m.runID = msg.runID
	}
	m.runDir = msg.runDir
	if msg.err != nil {
		m.validateErr = msg.err
		// D7: surface the persisted validate.json companion's path, when the
		// launch failure was persisted (best-effort; empty when persistence
		// itself failed or Deps.Runs is nil).
		m.validateRawPath = msg.rawPath
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
// moves to the terminal StateCanceled. On failure, two outcomes are
// distinguished (D6, deploy-error-detail): an already-terminal outcome
// (errors.Is ErrCancelAlreadyTerminal — the job reached a terminal state on
// Salesforce before the cancel landed) is a FRIENDLY informational message,
// never a raw CLI error, and writes NO companion — the run's persisted state
// stays untouched, exactly as a success-path no-op would. A GENERIC failure
// surfaces the error, stays on StateCancelConfirm with the run NOT marked
// (validation-cancel spec: "a failed cancel leaves the run untouched"), and
// best-effort persists the raw failure response as cancel-error.json via
// SaveRawCompanion (gap7).
func (m Model) onCancelDone(msg cancelDoneMsg) (tea.Model, tea.Cmd) {
	if m.state != StateCancelConfirm {
		// A late/duplicate cancel after we already left the confirm screen is
		// ignored — it must never re-mark the run or clobber the terminal state.
		return m, nil
	}
	if msg.err != nil {
		if errors.Is(msg.err, salesforce.ErrCancelAlreadyTerminal) {
			// D6: a friendly, non-fatal outcome — never a raw CLI error, and the
			// run's persisted state is left completely untouched (no companion).
			m.cancelErr = nil
			m.cancelInput = ""
			m.notice = "el job ya finalizó en Salesforce; no hay nada que cancelar"
			return m, nil
		}
		m.cancelErr = msg.err
		if m.deps.Runs != nil && m.runID != "" {
			_, _ = m.deps.Runs.SaveRawCompanion(m.runID, cancelErrorCompanionFilename, []byte(msg.result.Raw))
		}
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

// onQuickDeployDone lands the HU-015 quick-deploy outcome. Registration keys
// off the RunID CAPTURED when the deploy was fired (m.quickDeployingRunID),
// NOT the current cursor/state, because the `sf project deploy quick`
// subprocess runs under a background ctx and completes on the org even if the
// user has esc'd back to StateRunHistory in the meantime (adversarial-review
// Finding M-1: a navigated-away success was previously dropped, silently
// leaving the run unrecorded and still eligible). The captured id and the
// confirm buffer are cleared on BOTH outcomes so the in-flight guard reopens
// and no stale buffer can re-fire. On failure the run is left UNMARKED (retry
// allowed), mirroring onCancelDone's failure branch. On success it best-effort
// marks the run via MarkQuickDeployed (quick.json + QuickDeployedAt, ADR-4:
// Status is deliberately never touched) AND mirrors that write into the
// in-memory m.runs row (Finding H-2), so this session's own eligibility gate
// excludes the row without a restart. There is no new terminal state (design
// "single StateQuickDeploy to minimize footprint"); the success notice is only
// surfaced when the user is still on the screen.
func (m Model) onQuickDeployDone(msg quickDeployDoneMsg) (tea.Model, tea.Cmd) {
	runID := m.quickDeployingRunID
	onScreen := m.state == StateQuickDeploy

	// Clear the in-flight capture + confirm buffer regardless of outcome or the
	// current screen (Findings H-1/M-1): the guard reopens and no stale
	// DESPLEGAR buffer can re-authorize a deploy.
	m.quickDeployingRunID = ""
	m.quickConfirm = ""

	if msg.err != nil {
		m.quickErr = msg.err
		// gap7: a generic quick-deploy failure best-effort persists its raw
		// response as quick-error.json (mirrors onCancelDone's cancel-error.json).
		if m.deps.Runs != nil && runID != "" {
			_, _ = m.deps.Runs.SaveRawCompanion(runID, quickErrorCompanionFilename, []byte(msg.result.Raw))
		}
		return m, nil
	}
	m.quickErr = nil
	if m.deps.Runs != nil && runID != "" {
		_ = m.deps.Runs.MarkQuickDeployed(runID, []byte(msg.result.Raw))
		// Finding H-2: mirror the disk write into the in-memory history (which is
		// only ever written at startup) so the in-session x-eligibility gate
		// treats the row as already quick-deployed immediately.
		for i := range m.runs {
			if m.runs[i].RunID == runID {
				m.runs[i].QuickDeployedAt = m.now()
				break
			}
		}
	}
	if onScreen {
		m.notice = "quick deploy ejecutado y registrado"
	}
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
		if path, aerr := m.deps.Runs.AppendReport(m.runID, status, []byte(msg.report.Raw)); aerr == nil {
			// D5: capture the exact persisted path so failure/result screens
			// can surface it in place of the "revisa el JSON crudo" dead-end.
			m.reportPath = path
		}
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
		// Review M-2: reset any leaked delete-confirmation state on entry into
		// the terminal screen so a stray key can't funnel into keyDeleteConfirm.
		m = m.resetCleanupState()
		m.state = terminalState(msg.report.Status)

		// deploy-gate / validation-progress: a terminal SUCCESS on a gate-
		// enabled target with requireValidationComment on triggers the
		// validation-comment post/upsert, best-effort and async (never blocks
		// or alters this already-terminal-success screen). An ungated target,
		// a toggled-off condition, or a non-successful terminal state
		// (Failed/Canceled) never triggers it.
		if cmd := m.maybeTriggerValidationCommentCmd(msg.report.Status); cmd != nil {
			return m, cmd
		}
		return m, nil
	}
	return m.scheduleNextPoll()
}

// maybeTriggerValidationCommentCmd is onReportDone's terminal-success gate
// (validation-progress spec: "Terminal Successful CheckOnly Triggers The
// Deploy-Gate Validation Comment"): status must be a SUCCESS terminal
// (Succeeded/SucceededPartial — never Failed/Canceled), the target's gate
// must be Enabled (config.GateFor), and requireValidationComment must be on
// (nil/omitted defaults on; explicit false skips it). Only then does it
// return postValidationCommentCmd; otherwise nil (no trigger).
func (m Model) maybeTriggerValidationCommentCmd(status string) tea.Cmd {
	if status != "Succeeded" && status != "SucceededPartial" {
		return nil
	}
	gateCfg, gated := m.deps.Config.GateFor(m.plan.TargetBranch)
	if !gated || !requireValidationCommentOn(gateCfg) {
		return nil
	}
	return m.postValidationCommentCmd()
}

// onGateCheckDone lands deploy-gate's async evaluation (design.md's
// "onGateCheckDone" data flow): a passed Result dispatches quickDeployCmd —
// the point-of-no-return real `sf project deploy quick` — mirroring
// keyQuickDeploy's own pre-gate capture (m.quickDeployingRunID = the
// checked run's own RunID). A failed Result records every unmet condition
// (m.gateConditions) and transitions to StateDeployGateBlocked WITHOUT ever
// dispatching quickDeployCmd — no destructive action is taken.
func (m Model) onGateCheckDone(msg gateCheckDoneMsg) (tea.Model, tea.Cmd) {
	runID := m.gateCheckingRunID
	m.gateCheckingRunID = ""

	if !msg.result.Passed {
		m.gateConditions = msg.result.Conditions
		m.state = StateDeployGateBlocked
		return m, nil
	}

	m.gateConditions = nil
	if m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
		return m, nil
	}
	m.quickDeployingRunID = runID
	return m, m.quickDeployCmd()
}

// onPostCommentDone lands the deploy-gate validation-comment post/upsert
// outcome. It is deliberately best-effort (task 6.19): a failure (msg.err)
// is silently dropped — it NEVER alters the already-terminal-success run
// state, never surfaces on the success screen, and fires no further command.
func (m Model) onPostCommentDone(msg postCommentDoneMsg) (tea.Model, tea.Cmd) {
	return m, nil
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
// successful push records HU-017's currentPushed (so a later inline delete
// also removes the origin ref) and fires preparePRCmd (RemoteURL + gh
// AuthStatus); the phase stays pushPushing until that prep lands in
// onPrepDone.
func (m Model) onPushDone(msg pushDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.pushErr = msg.err
		m.pushPhase = pushConfirm
		return m, nil
	}
	m.pushErr = nil
	// HU-017: record that THIS run's promotion branch was successfully
	// pushed, so quitCmd's later inline-delete step also deletes the
	// origin ref (design.md "Corrections Baked In" #4).
	m.currentPushed = true
	return m, m.preparePRCmd()
}

// onPrepDone lands the HU-014 PR-preparation data (gh detection + compare URL)
// and settles the screen on pushReady, where the view shows base/compare/title
// and branches on authState (authed → offer PR; absent/unauthenticated → show
// the compare URL, or the raw origin + manual data when it could not be
// derived).
//
// ai-pr-summary (Feature B, proactive suggestion): the instant the screen
// settles here, a suggestion is requested automatically when Deps.GenerateSummary
// is configured and none is already in flight or held (m.aiTitle == "" &&
// !m.aiPending) — the user no longer has to press `a` once just to see it.
// This mirrors the first 'a' press's own request branch (keyPushPreparation)
// but fires it proactively; it NEVER auto-accepts — aiAccepted is set ONLY by
// an explicit second 'a', so effectiveTitle()/effectiveDescription() still
// gate on it (no silent override).
func (m Model) onPrepDone(msg prepDoneMsg) (tea.Model, tea.Cmd) {
	m.authState = msg.auth
	m.originURL = msg.originURL
	m.compareURL = msg.compareURL
	m.compareErr = msg.compareErr
	m.remoteErr = msg.remoteErr
	m.pushPhase = pushReady

	if m.reusing && msg.prOpen {
		// incremental-promotion: an OPEN PR already exists for the reused
		// branch — skip gh pr create entirely and show/report the existing
		// URL directly; the confirmed push has already updated it (spec:
		// "Open PR already exists for the branch"). No AI suggestion is
		// requested either: there is nothing left to create.
		m.prURL = msg.prURL
		m.prErr = nil
		m.prExisting = true
		return m, nil
	}
	if m.reusing && msg.prURL != "" {
		// CLOSED/MERGED: fall through to the normal create path with a
		// notice, rather than silently reusing a stale, unreusable PR
		// (design: "closed/merged falls through to normal create").
		m.notice = "el PR anterior de esta rama ya está cerrado/mergeado; se creará uno nuevo"
	}

	var cmd tea.Cmd
	if m.deps.GenerateSummary != nil && m.aiTitle == "" && !m.aiPending {
		m.aiPending = true
		cmd = m.aiSuggestCmd()
	}
	return m, tea.Batch(cmd)
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

	// validation-comment-timing: the normal flow validates BEFORE the PR
	// exists, so onReportDone's own trigger (below) fires-and-skips with no
	// PR to resolve and never retries. Re-attempt here, at the exact point
	// the PR first exists — reusing the same guard (gate-enabled +
	// requireValidationComment-on + a terminal-success status this session)
	// and the same skip-if-present marker for exactly-once delivery
	// (deploy-gate: "posts at PR creation" / "exactly once across both
	// trigger points"; validation-progress: "PR creation (re)triggers the
	// comment"). A non-success/empty m.report.Status, an ungated target, or
	// an already-posted marker all no-op inside the returned cmd.
	if cmd := m.maybeTriggerValidationCommentCmd(m.report.Status); cmd != nil {
		return m, cmd
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
