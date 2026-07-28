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
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/config"
	"deploydeck/internal/delta"
	"deploydeck/internal/git"
	"deploydeck/internal/github"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
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
	// StatePickVerification is the post-pick verification screen; confirming
	// here (when DeltaAllowed) enters delta generation.
	StatePickVerification
	// StateDeltaGeneration runs HU-007 `sf sgd source delta`. On success it
	// advances to PackageReview; an sgd failure keeps the user here with the
	// raw output shown and never launches validation.
	StateDeltaGeneration
	// StatePackageReview is HU-008's pre-validation package summary. An empty
	// package blocks confirm until an explicit override.
	StatePackageReview
	// StateQueueReview is HU-009's real stop between package confirm and
	// validation start: it queries the active DeployRequest queue and shows
	// user/status/progress per job, highlighting the current run's own job
	// (when present) with its approximate position. A Tooling-API-permission
	// failure auto-skips to StateValidationStart (non-blocking); a generic
	// failure keeps the user here with the error shown (non-aborting).
	StateQueueReview
	// StateValidationStart runs HU-010 `sf project deploy validate --async`
	// and persists the run on jobId receipt. A CLI error keeps the flow alive
	// here (message + raw shown) rather than crashing.
	StateValidationStart
	// StateValidationPolling runs HU-011: a tea.Tick-driven poll of
	// `sf project deploy report`, bounded by a hard deadline, retrying
	// transient errors and persisting every raw report.
	StateValidationPolling
	// StateCancelConfirm is HU-012's typed-confirmation cancel screen, reached
	// ONLY from StateValidationPolling via the distinct `c` key (never `q`,
	// whose exit-leaves-job-active invariant is unchanged). It gates the
	// destructive `sf project deploy cancel` behind the user typing the exact
	// literal CANCELAR; confirming fires cancelCmd for the CURRENT run's own
	// job only. On success the run moves to StateCanceled and is persisted; on
	// failure the user stays here with the error shown and the run unmarked.
	StateCancelConfirm
	// StateSucceeded is the terminal success screen ({Succeeded,
	// SucceededPartial} both fold here).
	StateSucceeded
	// StateFailed is the terminal failure screen (validation Failed or the
	// poll's hard timeout).
	StateFailed
	// StateCanceled is the terminal screen for a Canceled validation job.
	StateCanceled
	// StateError is a terminal error screen.
	StateError
	// StateRunHistory is HU-013's run-history browse + resume-offer screen
	// (mockup docs/MOCKUPS_TUI.md "Historial De Runs"). Startup resume-detection
	// routes here PRE-SELECTED on a resumable run; Enter resumes it (routing
	// directly into StateCherryPickConflict or StateValidationPolling), q/esc
	// declines to the normal ticket-input flow. There is deliberately NO
	// literal StateSuspended — resume routes straight into the live states.
	StateRunHistory
	// StatePushPreparation is HU-014's post-success push + PR-preparation
	// screen (mockup docs/MOCKUPS_TUI.md "Push Y PR"). It is reachable ONLY
	// from StateSucceeded (which folds Succeeded+SucceededPartial) via the
	// distinct `p` key — never from Failed/Canceled/Aborted/Error, so push is
	// offered only after a successful validation. Its own sub-flow (pushPhase)
	// gates git push and gh PR creation behind explicit confirmations; it
	// composes the injected git.Service + github.Client and, like every other
	// screen, never execs directly (boundary_test.go holds).
	StatePushPreparation
	// StateBranchCleanup is HU-017's batch cleanup screen (branch-cleanup
	// spec: "Orphan Deploy Branches Listed For Batch Cleanup"), reached from
	// StateTicketInput via the distinct `b` key. It lists every orphan
	// deploy/* branch (git.Service.ListDeployBranches, correlated against
	// runs.List() by selectOrphans to exclude a live in-progress run's own
	// branch) with age/push-status/merged-label, offering per-row delete
	// (reusing the SAME unpushed-gate + BORRAR strong-confirm machinery
	// Group 3's inline current-branch delete uses) and a retention-prune
	// action (deps.Runs.Prune, UNCHANGED). q/esc return to StateTicketInput.
	StateBranchCleanup
)

// cleanupPhase is HU-017 branch-cleanup's delete-confirmation sub-state,
// shared by the terminal screens' inline current-branch delete (Group 3:
// keySucceeded/keyAborted's `d`) and, later, the StateBranchCleanup batch
// screen's per-row delete (Group 4) — the two are mutually exclusive by
// construction (a Model is never on both StateSucceeded/StateAborted and
// StateBranchCleanup at once), so reusing one field across them is safe.
type cleanupPhase int

const (
	// cleanupIdle is the ZERO VALUE: no delete confirmation is in progress.
	// A terminal screen starts here; pressing `d` leaves it only once
	// unpushedCountCmd's result lands (onUnpushedCount picks confirm vs
	// strongConfirm) — this is deliberate so a stray key press before `d`
	// (e.g. a leftover `y`) can never be misread as a delete confirmation.
	cleanupIdle cleanupPhase = iota
	// cleanupConfirm is a normal, single-key ('y') confirmation — the
	// branch has zero unpushed commits (UnpushedCommitCount == 0).
	cleanupConfirm
	// cleanupStrongConfirm requires the exact typed literal BORRAR
	// (case-sensitive, via the dedicated m.deleteConfirm buffer) — the
	// branch has unpushed commits (design.md "Strong-confirm reuse").
	cleanupStrongConfirm
	// cleanupLoading is StateBranchCleanup's initial sub-state: the `b` key
	// (or a post-delete reload) fired listDeployBranchesCmd and its result
	// hasn't landed yet. Appended here rather than interleaved with the
	// Group 3 consts above, so their existing values are NEVER renumbered.
	cleanupLoading
	// cleanupBrowsing is StateBranchCleanup once the list has loaded: nav
	// (↑/↓/k/j), `d` (per-row delete, gated exactly like the terminal
	// screens' inline delete), `p` (retention-prune, gated behind
	// cleanupPruneConfirm), and `q`/`esc` (back to StateTicketInput) are all
	// live here.
	cleanupBrowsing
	// cleanupPruneConfirm is StateBranchCleanup's retention-prune
	// confirmation (branch-cleanup spec: "Run Retention Applied From The
	// Cleanup Surface"), reached via `p`. A single 'y'/'n' gate — pruning
	// deletes only already-decided-by-policy run.json directories, never a
	// git branch, so it does not warrant the typed BORRAR strong confirm.
	cleanupPruneConfirm
	// cleanupCounting is StateBranchCleanup's in-flight per-row unpushed-count
	// window (review H-1): pressing `d` CAPTURES the target row
	// (cleanupDeleteTarget) and parks here while unpushedCountCmd runs. Cursor-
	// move keys are inert here — and stay inert through the confirm phases — so
	// the confirm that lands can never bind to a DIFFERENT row than the one `d`
	// captured (the cursor-move TOCTOU). Appended last so the existing values
	// are never renumbered.
	cleanupCounting
)

// pushPhase is StatePushPreparation's sub-state machine (HU-014). It gates the
// two external side effects — `git push -u` and `gh pr create` — behind
// explicit confirmations so neither ever runs implicitly.
type pushPhase int

const (
	// pushConfirm shows the push command and waits for the explicit `p`
	// confirm before running git.Push (spec: "show the push command before
	// running it").
	pushConfirm pushPhase = iota
	// pushPushing is the in-flight window between the confirmed push and its
	// result (and the subsequent RemoteURL+AuthStatus prep).
	pushPushing
	// pushReady is the post-push screen: base/compare/suggested-title plus the
	// gh branch (authed → offer PR; absent/unauthenticated → compare URL).
	pushReady
	// pushPRConfirm is the authed-only explicit-confirm gate revealed by `g`;
	// only an explicit confirm key here fires gh pr create (spec invariant:
	// "no PR is created without explicit confirmation").
	pushPRConfirm
	// pushPRCreating is the in-flight window while gh pr create runs.
	pushPRCreating
)

// Deps carries the services and configuration the TUI composes. main() wires
// real NewOSRunner-backed services; tests inject fakes / real-git-on-temp-repo.
type Deps struct {
	// Git is the git service (real OSRunner in main, real-on-temp-repo in
	// integration tests). Required.
	Git *git.Service
	// SF is the read-only Salesforce shim, used for the target-selection
	// sandbox-auth warning and HU-010/011 validate + report calls. May be nil
	// (the warning is then skipped; delta/validation deps are wired by main).
	SF salesforce.Client
	// GH is the gh CLI client backing HU-014's StatePushPreparation: gh
	// auth-status detection and `gh pr create`. main() wires
	// github.New(runner); a nil GH degrades the push-preparation screen to the
	// compare-URL fallback (no PR offered), and — like every other dep —
	// internal/app reaches gh ONLY through this interface, never execing.
	GH github.Client
	// Delta generates HU-007 delta packages via `sf sgd source delta`. main()
	// wires delta.New(runner); nil disables delta generation.
	Delta *delta.Service
	// Runs persists validation runs under .deploydeck/runs/ (HU-010/011).
	// main() wires runs.NewWriter(dir); nil disables persistence.
	Runs *runs.Writer
	// Now returns the current time for HU-011's poll deadline. It is
	// injectable so ValidationPolling's timeout is testable without real
	// time; nil falls back to time.Now.
	Now func() time.Time
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

	// originalBranch (HU-017) is the branch checked out at flow startup,
	// captured via originalBranchCmd batched from onPrereqDone (BEFORE any
	// promotion branch is created). quitCmd restores it on a terminal/
	// abandoned quit, guarded by shouldRestore. Empty when never captured
	// (deps.Git nil, or the capture itself errored) — shouldRestore already
	// treats "" as a no-op-restore signal.
	originalBranch string
	// currentPushed (HU-017) records whether THIS run's own promotion
	// branch (m.plan.PromotionBranch) was successfully pushed at least once
	// (set true by onPushDone's success branch). quitCmd's delete step uses
	// it to decide whether DeleteRemoteBranch also runs alongside
	// DeleteLocalBranch.
	currentPushed bool
	// pendingDeleteCurrent (HU-017) is set by an explicit inline delete
	// confirmation (normal or strong) on a terminal screen. The actual
	// delete is deferred to run INSIDE quitCmd, right after its restore
	// checkout — a branch can't be deleted while it is checked out.
	pendingDeleteCurrent bool
	// cleanupPhase (HU-017) gates the terminal screens' inline delete
	// confirmation UI; see the cleanupPhase type doc above.
	cleanupPhase cleanupPhase
	// deleteConfirm (HU-017) is the dedicated typed-BORRAR buffer for
	// cleanupStrongConfirm — a DEDICATED field, deliberately never HU-012's
	// cancelInput, so a stray CANCELAR/cancel buffer can never delete a
	// branch and vice versa (design.md "Strong-confirm reuse"). Group 4's
	// StateBranchCleanup per-row strong-confirm reuses this SAME field/word
	// (never both active at once — see cleanupPhase's doc comment).
	deleteConfirm string

	// StateBranchCleanup (HU-017 Group 4): cleanupBranches is the loaded,
	// orphan-only (selectOrphans-filtered) row list, each carrying its
	// best-effort merged/abandoned label; cleanupCursor is the selected row;
	// cleanupNotice is a DEDICATED transient message for this screen —
	// separate from the generic m.notice — so a stale load/delete/prune
	// error can never bleed onto (or be silently cleared by) an unrelated
	// screen's own notice.
	cleanupBranches []cleanupRow
	cleanupCursor   int
	cleanupNotice   string
	// cleanupDeleteTarget (HU-017, review H-1) is the branch NAME captured the
	// instant `d` is pressed on the batch screen, with cleanupDeleteTargetPushed
	// its push status. confirmDeleteOrphan deletes THIS captured branch — never
	// the live m.cleanupBranches[m.cleanupCursor], which the cursor-move TOCTOU
	// could have shifted to a different (possibly unpushed) row. onUnpushedCount
	// also drops any count whose branch no longer matches this target (a stale
	// result from a prior d/esc), so the confirm strength is always the one
	// computed for the captured branch.
	cleanupDeleteTarget       string
	cleanupDeleteTargetPushed bool

	// CherryPicking / Conflict
	contiguous      bool
	repoState       git.RepoState
	pickOutcome     git.PickOutcome
	continueEnabled bool
	continuePending []string
	aborted         bool
	// pickIndex/pickTotal are the live "pick N of M" shown on the conflict
	// screen — for a fresh sequence (set in onPickDone's conflict branch) and a
	// resumed one (recomputed live in resumeInto via derivePickIndex). pickTotal
	// is 0 outside a cherry-pick, so the view omits the counter then.
	pickIndex int
	pickTotal int

	// RunHistory (HU-013): the browsable past-run list that doubles as the
	// resume-offer surface. runs is the full List() snapshot (newest-first),
	// runsCursor the selected row, runDetail the expanded-detail toggle.
	runs       []runs.Record
	runsCursor int
	runDetail  bool

	// RePromote (HU-016): sourceRunID links a completed re-promotion run back
	// to the prior run it was re-promoted from — seeded synchronously by
	// startRePromoteInto and threaded into the new run.Record at creation
	// (onBranchCreated). rePromoteMissing holds prior-run commit SHAs with no
	// patch-id equivalent in the new discovery range (onRePromoteSeeded),
	// surfaced as an explicit warning on the selection screen rather than
	// silently dropped (re-promotion spec: "Missing Commit Warned
	// Explicitly").
	sourceRunID      string
	rePromoteMissing []string

	// PushPreparation (HU-014): the sub-flow driving push + PR preparation
	// from the success screen. pushPhase is the sub-state machine; pushErr
	// holds a failed push's error (surfaced on pushConfirm, flow survives).
	// authState is the gh detection result; originURL is `origin`'s raw URL
	// (from git.RemoteURL); compareURL is the derived compare link (empty when
	// authed or when the origin form is unrecognized); compareErr/remoteErr
	// record the graceful-degradation causes. prURL is the created PR's URL
	// (also recorded on the run via MarkPRCreated); prErr is a failed
	// `gh pr create`'s error, shown with the manual base/compare/title data.
	pushPhase  pushPhase
	pushErr    error
	authState  github.AuthState
	originURL  string
	compareURL string
	compareErr error
	remoteErr  error
	prURL      string
	prErr      error

	// PickVerification
	verification git.PickVerification
	deltaAllowed bool

	// DeltaGeneration / PackageReview (HU-007/008)
	deltaResult    delta.Result
	summary        delta.PackageSummary
	deltaErr       error // sgd failure, surfaced on DeltaGeneration
	emptyConfirmed bool  // explicit override to validate an empty package

	// QueueReview (HU-009)
	queue    []salesforce.DeployQueueEntry // parsed active DeployRequest queue
	queueErr error                         // generic (non-permission) query failure, surfaced on QueueReview
	identity string                        // own-job identity: Orgs()->FindByAlias(alias).Username

	// CancelConfirm (HU-012)
	cancelInput string // typed confirmation buffer; cancel fires only when == "CANCELAR"
	cancelErr   error  // failed-cancel error, surfaced on StateCancelConfirm (run left unmarked)

	// Validation (HU-010/011)
	jobID        string
	runID        string
	runDir       string
	report       salesforce.DeployReport
	validateErr  error     // CLI validate error, surfaced on ValidationStart
	reportErr    error     // last transient report error, surfaced while polling
	pollDeadline time.Time // hard poll timeout, from injected Now + PollTimeout
	timedOut     bool      // true when StateFailed was reached via the deadline
	// pollInFlight is true while a reportCmd is outstanding. It serializes the
	// poll loop so at most ONE `sf project deploy report` runs at a time: a
	// tick (or manual refresh) that arrives before the current report returns
	// is a no-op, and the next poll is scheduled only once onReportDone lands —
	// so a slow report can never let ticks pile up into concurrent subprocesses.
	pollInFlight bool
	// pollCtx / pollCancel bound the whole ValidationPolling session. Entering
	// polling arms a cancelable context that every reportCmd derives its
	// per-call timeout from; leaving polling (terminal, timeout, or user exit)
	// cancels it, tearing down any in-flight read-only `sf project deploy
	// report` subprocess. This NEVER touches the Salesforce job — report is
	// read-only, so cancelling the local query leaves the async validation
	// running and the persisted run resumable.
	pollCtx    context.Context
	pollCancel context.CancelFunc
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

// pollContext returns the ValidationPolling session context reportCmd derives
// its per-call timeout from. It falls back to Background before polling is
// armed so the poll command is always safe to build.
func (m Model) pollContext() context.Context {
	if m.pollCtx != nil {
		return m.pollCtx
	}
	return context.Background()
}

// cancelPoll cancels the polling session context (if armed), tearing down any
// in-flight `sf project deploy report` subprocess. It is idempotent and a
// no-op before polling starts. It NEVER issues a `deploy cancel`: report is
// read-only, so the Salesforce job stays active and the run stays resumable.
func (m Model) cancelPoll() {
	if m.pollCancel != nil {
		m.pollCancel()
	}
}

// now returns the current time through the injected Deps.Now (nil → time.Now),
// so HU-011's poll deadline is deterministic under test.
func (m Model) now() time.Time {
	if m.deps.Now != nil {
		return m.deps.Now()
	}
	return time.Now()
}

// pollIntervalSeconds is the configured HU-011 report poll cadence, falling
// back to the package default when unset.
func (m Model) pollIntervalSeconds() int {
	if m.deps.Config.PollIntervalSeconds > 0 {
		return m.deps.Config.PollIntervalSeconds
	}
	return config.DefaultPollIntervalSeconds
}

// pollTimeoutSeconds is the configured HU-011 hard poll timeout, falling back
// to the package default when unset.
func (m Model) pollTimeoutSeconds() int {
	if m.deps.Config.PollTimeoutSeconds > 0 {
		return m.deps.Config.PollTimeoutSeconds
	}
	return config.DefaultPollTimeoutSeconds
}

// terminalState maps a terminal deploy-report status to its screen: both
// Succeeded and SucceededPartial fold into StateSucceeded (design's
// "SucceededPartial folds into StateSucceeded"), Failed and Canceled to their
// own terminal states.
func terminalState(status string) State {
	switch status {
	case "Failed":
		return StateFailed
	case "Canceled":
		return StateCanceled
	default: // Succeeded, SucceededPartial
		return StateSucceeded
	}
}

// deltaBaseDir is the configured base directory for delta artifacts, defaulting
// to config.DefaultDeltaOutputDir. The per-run directory
// (<base>/<ticket>-to-<target>) is composed by deltaCmd.
func deltaBaseDir(cfg config.Config) string {
	if cfg.Delta.OutputDir != "" {
		return cfg.Delta.OutputDir
	}
	return config.DefaultDeltaOutputDir
}
