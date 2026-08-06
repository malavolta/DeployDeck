package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/delta"
	"github.com/malavolta/DeployDeck/internal/gate"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/provenance"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// reportCallTimeout bounds a single `sf project deploy report` invocation
// (design.md: "Each ReportDeploy gets a per-call context.WithTimeout"). It is
// deliberately shorter than any sane poll timeout so one hung poll can never
// stall the whole ValidationPolling loop.
const reportCallTimeout = 60 * time.Second

// queueCallTimeout bounds a single queueCmd invocation (the identity lookup
// plus the deploy-queue query combined, HU-009), so a hung Tooling API call
// can never stall the QueueReview screen indefinitely.
const queueCallTimeout = 30 * time.Second

// pollInterval is how often the cherry-pick screens re-read RepoState so an
// external `git cherry-pick --continue`/`--abort` reconciles live (HU-006
// AC4/AC6). The model is never the source of truth — the repo is.
const pollInterval = 750 * time.Millisecond

// spinnerInterval is the animated-progress-spinner frame cadence (design D5),
// a much faster in-process timer than pollInterval/pollTickCmd — it drives
// no service call, only a visual frame bump.
const spinnerInterval = 120 * time.Millisecond

// resumeDetectTimeout bounds the startup HU-013 resume-detection: RepoState
// spawns git subprocesses, so an unbounded context.Background() could let a
// slow/hung git stall detection indefinitely. A few seconds is ample for a
// local RepoState read; on timeout RepoState errors and onResumeDetect degrades
// to the normal flow (the offer is a best-effort courtesy, never a gate).
const resumeDetectTimeout = 5 * time.Second

// updateCheckTimeout bounds the HU-019 startup update-availability check
// (design ADR-3: "3s context.WithTimeout bounds it"). Bubble Tea's goroutine
// Cmd already makes the check non-blocking; this bounds how long the check
// itself may run before the courtesy notice is skipped.
const updateCheckTimeout = 3 * time.Second

// aiSuggestTimeout bounds one AI-suggestion request (ai-pr-summary design's
// Data Flow: "ctx=WithTimeout"), mirroring updateCheckTimeout's discipline —
// a slow/unreachable local model must never stall the pushReady screen.
const aiSuggestTimeout = 15 * time.Second

// gateCheckTimeout bounds gateCheckCmd's WHOLE sequence of gh reads
// (resolvePRURL's fallback lookup plus the 4 independent condition reads),
// mirroring queueCmd's "shared per-call timeout across multiple sequential
// calls" discipline (remediation-pass resilience WARNING). keyQuickDeploy
// clears the confirm buffer the instant this fires, so an unbounded
// context.Background() let a single hung gh invocation stall
// gateCheckDoneMsg — and the frozen-looking screen — forever.
const gateCheckTimeout = 30 * time.Second

// --- Messages ---

// prereqDoneMsg carries the HU-001 prereq report (or an error).
type prereqDoneMsg struct {
	checks []prereq.PrereqCheck
	err    error
}

// updateCheckDoneMsg carries the HU-019 update-availability check result (or
// an error). onUpdateCheckDone treats err!=nil and hasUpdate==false
// identically — a silent skip (design ADR-3).
type updateCheckDoneMsg struct {
	hasUpdate bool
	latest    string
	err       error
}

// aiSuggestDoneMsg carries the ai-pr-summary suggestion request's result (or
// an error). onAISuggestDone treats err!=nil and title=="" identically — a
// silent skip (spec: "Silent Graceful Degradation").
type aiSuggestDoneMsg struct {
	title       string
	description string
	err         error
}

// discoverDoneMsg carries the assembled HU-002 discovery result and the
// resolved single source branch (or an error). confirm is true when pass-1
// discovery resolved to resolveNeedsConfirm — source is then only a PENDING
// current-branch suggestion, and result carries the base (message+branch
// only) discovery; onDiscoverDone then routes to StateSourceConfirm instead
// of StateCommitSelection. false (the zero value) preserves every
// pre-existing resolved/degrade discoverDoneMsg exactly.
type discoverDoneMsg struct {
	result  git.DiscoverResult
	source  git.Branch
	confirm bool
	err     error
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

// reuseReadyMsg carries incremental-promotion's StateBranchCollision "reuse &
// append" outcome (Checkout -> FastForwardBranch -> FilterNotOnBranch): the
// not-yet-present commit remainder plus the fast-forward classification, or
// err for a genuine command failure. ffResult == git.FFDiverged is DATA, not
// an error field — onReuseReady treats it identically to err != nil (a hard,
// clear error), but it is carried separately so a diverged remote is never
// confused with a real command failure in logs/telemetry.
type reuseReadyMsg struct {
	remaining []git.DiscoveredCommit
	ffResult  git.FFResult
	err       error
}

// rePromoteSeededMsg carries the HU-016 patch-id remap outcome computed by
// rePromoteRemapCmd: Matched holds the prior run's commits mapped to their
// new-range equivalents (re-promotion spec: "Patch-ID Remap Of Prior
// Commits"), Unmatched preserves the original prior SHAs with no equivalent
// in the new range (never dropped — "Missing Commit Warned Explicitly"). err
// is set only for a hard remap failure (the range itself could not be
// listed) — a per-SHA lookup failure never reaches here as an error, since
// git.Service.RemapCommitsByPatchID itself degrades those to Unmatched.
type rePromoteSeededMsg struct {
	Matched   []git.DiscoveredCommit
	Unmatched []string
	err       error
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

// deltaDoneMsg carries the HU-007 delta result and HU-008 package summary
// (or an sgd/parse error, which keeps the user on DeltaGeneration).
type deltaDoneMsg struct {
	result  delta.Result
	summary delta.PackageSummary
	err     error
}

// validateDoneMsg carries the HU-010 validate outcome: the jobId-bearing
// result, the persisted run id/dir, or an error (CLI error or persistence
// failure — the flow stays alive either way). rawPath (D7,
// deploy-error-detail) is the persisted validate.json companion's path when
// the launch itself failed (before any jobId is obtained), surfaced on
// viewValidationStart alongside the error message.
type validateDoneMsg struct {
	result  salesforce.ValidateResult
	runID   string
	runDir  string
	rawPath string
	err     error
}

// reportDoneMsg carries one HU-011 poll's DeployReport, or a transient error
// the poll loop retries within its deadline.
type reportDoneMsg struct {
	report salesforce.DeployReport
	err    error
}

// pollTickMsg drives the HU-011 ValidationPolling cadence. It is distinct from
// tickMsg (the 750ms cherry-pick re-poll): validation polls at the configured
// pollIntervalSeconds, a different, slower cadence firing a different command.
type pollTickMsg struct{}

// queueDoneMsg carries the HU-009 deploy-queue outcome: the parsed entries
// plus the resolved own-job identity (Orgs()->FindByAlias(alias).Username),
// or an error. onQueueDone distinguishes ErrQueuePermission (non-blocking
// skip to ValidationStart) from a generic query failure (stays on
// QueueReview, non-aborting).
type queueDoneMsg struct {
	entries  []salesforce.DeployQueueEntry
	identity string
	err      error
}

// cleanupRow is one StateBranchCleanup screen row: an orphan git.DeployBranch
// plus its best-effort "likely merged"/"abandoned"/"unknown" advisory label
// (mergedLabel) — never a delete gate, see branch-cleanup spec's "Merged-Vs-
// Abandoned Is A Best-Effort Label Only".
type cleanupRow struct {
	git.DeployBranch
	MergedLabel string
}

// deployBranchesMsg carries HU-017's listDeployBranchesCmd result: the
// orphan-filtered, merge-labeled row list, or a load error (surfaced as a
// cleanupNotice, never a crash — mirrors every other best-effort list load in
// this file).
type deployBranchesMsg struct {
	branches []cleanupRow
	err      error
}

// deleteDoneMsg carries HU-017's per-row deleteOrphanCmd outcome (local, +
// remote iff Pushed) for the StateBranchCleanup screen. Unlike the terminal
// screens' inline delete (which defers its delete to run INSIDE quitCmd,
// since it deletes the CURRENT branch), a batch-screen row is never the
// checked-out branch, so it can be deleted directly and reported here.
type deleteDoneMsg struct {
	err error
}

// pruneDoneMsg carries HU-017's retention-prune outcome (branch-cleanup spec:
// "Run Retention Applied From The Cleanup Surface") — the removed run IDs (as
// returned by the UNCHANGED runs.Writer.Prune), or an error.
type pruneDoneMsg struct {
	removed []string
	err     error
}

// originalBranchMsg carries the HU-017 startup branch capture (or an error,
// which onOriginalBranch treats as a best-effort no-op — quitCmd's
// shouldRestore guard already skips restore on an empty originalBranch).
type originalBranchMsg struct {
	branch string
	err    error
}

// unpushedMsg carries HU-017's UnpushedCommitCount result for the inline
// current-branch delete gate. err is treated conservatively by
// onUnpushedCount ("cannot confirm pushed") — the same posture as every
// other best-effort read in this file, but erring toward the STRONGER
// confirmation rather than a silent skip, since this gates a destructive
// action.
type unpushedMsg struct {
	// branch is the branch the count was requested FOR (review H-1). It is
	// carried so onUnpushedCount can drop a stale result whose target has since
	// changed (a moved cursor / a prior d+esc), rather than binding the confirm
	// strength of one branch to a count computed for another.
	branch string
	count  int
	err    error
}

// resumeDetectMsg is the HU-013 startup resume-detection result: the live
// RepoState (repo is the source of truth) plus the persisted run list, or an
// error. onResumeDetect reconciles stale conflict records against state, then
// offers resume (StateRunHistory) when anything is resumable, else proceeds to
// the normal flow. A matching cherry-pick run is the newest non-terminal run
// whose Commits contains state.CurrentSHA.
type resumeDetectMsg struct {
	state   git.RepoState
	records []runs.Record
	err     error
}

// cancelDoneMsg carries the HU-012 cancel outcome: the raw cancel response
// (persisted verbatim as cancel.json on success) or an error. onCancelDone
// marks the run Canceled + persists on success, or surfaces the error WITHOUT
// marking the run on failure (validation-cancel spec).
type cancelDoneMsg struct {
	result salesforce.CancelResult
	err    error
}

// quickDeployDoneMsg carries the HU-015 quick-deploy outcome: the raw
// response (persisted verbatim as quick.json on success) or an error.
// onQuickDeployDone best-effort marks the run via MarkQuickDeployed on
// success, or surfaces the error WITHOUT marking the run on failure
// (mirrors cancelDoneMsg's shape and onCancelDone's guard posture).
type quickDeployDoneMsg struct {
	result salesforce.QuickDeployResult
	err    error
}

// pushDoneMsg carries the HU-014 push outcome: nil on a successful
// `git push -u origin <branch>`, or the error (which keeps the user on the
// push-confirm screen, flow alive).
type pushDoneMsg struct {
	err error
}

// prepDoneMsg carries the HU-014 post-push PR-preparation outcome: the gh
// detection result (auth) plus the `origin` URL and the compare URL derived
// from it. remoteErr records a failed `git remote get-url`; compareErr records
// an unrecognized origin form (both degrade gracefully to the raw origin +
// manual data). The base/compare/suggested-title themselves are derived from
// the plan in the view, so they need no fields here.
type prepDoneMsg struct {
	auth       github.AuthState
	originURL  string
	compareURL string
	compareErr error
	remoteErr  error
	// prURL/prOpen (incremental-promotion) carry github.Client.PRForBranch's
	// result — consulted ONLY when m.reusing && auth==AuthAuthenticated
	// (design: "PR detection ... consulted only on the authenticated path").
	// prURL is set for BOTH an open and a closed/merged PR (gh still reports
	// its URL); prOpen distinguishes them so onPrepDone can skip create
	// (open) vs fall through to a normal create with a notice (closed/
	// merged). Both stay zero-valued on a non-reuse promotion or when no PR
	// exists at all.
	prURL  string
	prOpen bool
}

// prCreatedMsg carries the HU-014 `gh pr create` outcome: the created PR's URL
// (recorded on the run via MarkPRCreated) and Raw on success, or an error that
// surfaces alongside the manual base/compare/title data (flow continues).
type prCreatedMsg struct {
	url string
	raw string
	err error
}

// selectOrphans is the pure HU-017 orphan-correlation rule (branch-cleanup
// spec: "Orphan Deploy Branches Listed For Batch Cleanup" — "not tied to a
// live in-progress run"). A DeployBranch is EXCLUDED (not an orphan) when
// SOME record whose rendered name (via the SAME git.RenderBranchName the rest
// of the app uses to derive a promotion branch from Ticket+Target) matches the
// branch is still LIVE, in EITHER of two independent senses:
//
//   - Async-validating on the org: the record carries a JobID and a
//     NON-TERMINAL Status (i.e. NOT one of salesforce.IsTerminal's
//     {Succeeded, SucceededPartial, Failed, Canceled}). Such a run finished
//     cherry-picking but its deploy branch is still tied to a running deploy
//     job — so it is live regardless of RepoState.InProgress or cherry-pick
//     phase (review H-2: before this, a jobId-bearing InProgress/Queued run
//     whose RepoState.InProgress==false was mislabeled a deletable orphan).
//   - Mid cherry-pick: inProgress is true (RepoState is the single source of
//     truth for whether ANY pick is live right now) AND the record is in a
//     cherry-pick phase.
//
// Every other branch — one belonging to an already-terminal run, or one whose
// rendered name matches no record — is an orphan.
func selectOrphans(branches []git.DeployBranch, records []runs.Record, format string, inProgress bool) []git.DeployBranch {
	live := map[string]bool{}
	for _, rec := range records {
		name := git.RenderBranchName(format, rec.Ticket, rec.Target)
		// A run mid async-validation (jobId set, status not terminal) is live,
		// independent of repoState.InProgress and cherry-pick phase.
		if rec.JobID != "" && !salesforce.IsTerminal(rec.Status) {
			live[name] = true
		}
		// A live in-progress cherry-pick's own branch.
		if inProgress && isCherryPickPhase(rec.Phase) {
			live[name] = true
		}
	}

	var orphans []git.DeployBranch
	for _, b := range branches {
		if live[b.Name] {
			continue
		}
		orphans = append(orphans, b)
	}
	return orphans
}

// resolveMergeTarget finds the Target of the run.Record whose rendered branch
// name (git.RenderBranchName(format, rec.Ticket, rec.Target) — the SAME
// correlation selectOrphans uses) matches name. ok is false when no record
// correlates, e.g. an orphan whose originating run was pruned or never
// persisted (branch-cleanup spec: "Merged-Vs-Abandoned Is A Best-Effort Label
// Only" — "unknown when target can't be determined").
func resolveMergeTarget(records []runs.Record, format, name string) (string, bool) {
	for _, rec := range records {
		if git.RenderBranchName(format, rec.Ticket, rec.Target) == name {
			return rec.Target, true
		}
	}
	return "", false
}

// mergedLabel classifies an orphan branch's best-effort "likely merged" vs
// "abandoned" advisory label from IsMergedInto's result (branch-cleanup spec:
// "Merged-Vs-Abandoned Is A Best-Effort Label Only" — this label NEVER gates
// deletion, see keyBranchCleanup's unconditional unpushedCountCmd gate). An
// unresolved target (ok==false) or an ancestor-check error both degrade to
// "unknown" rather than guessing.
func mergedLabel(ok, merged bool, err error) string {
	if !ok || err != nil {
		return "unknown"
	}
	if merged {
		return "likely merged"
	}
	return "abandoned"
}

// shouldRestore is the pure HU-017 restore-on-quit guard (design.md "Quit-
// restore mechanism + guards"). Restore is skipped (returns false) when:
// inProgress (git REFUSES checkout with unmerged paths — this is the
// rationale, NOT a resume-detection need; CHERRY_PICK_HEAD lives in .git/
// and is worktree-scoped, so it survives regardless of which branch is
// checked out, keeping HU-013 resume-detection intact), original is
// empty/detached ("HEAD"), original already equals current, or original no
// longer resolves (exists is false). Restore is warranted otherwise.
func shouldRestore(inProgress bool, original, current string, exists bool) bool {
	if inProgress {
		return false
	}
	if original == "" || original == "HEAD" {
		return false
	}
	if original == current {
		return false
	}
	return exists
}

// --- Command constructors (every one routes through a service) ---

// checkUpdateCmd runs the HU-019 update-availability check via the injected
// scalar Deps.CheckUpdate. nil disables the check (same nil-degrades
// convention as runPrereqCmd's NewChecker). The 3s updateCheckTimeout bounds
// the call (design ADR-3); Bubble Tea's own goroutine Cmd already makes this
// non-blocking relative to startup.
func (m Model) checkUpdateCmd() tea.Cmd {
	checkUpdate := m.deps.CheckUpdate
	if checkUpdate == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		hasUpdate, latest, err := checkUpdate(ctx)
		return updateCheckDoneMsg{hasUpdate: hasUpdate, latest: latest, err: err}
	}
}

// aiSuggestCmd requests an ai-pr-summary suggestion via the injected scalar
// Deps.GenerateSummary. nil disables the affordance entirely (same
// nil-degrades convention as checkUpdateCmd's Deps.CheckUpdate) — keyPushPreparation's
// guard already checks this before firing the request, but the command
// itself stays defensive. ticket/commitSubjects/componentSummary are built
// ENTIRELY from data already on Model (design's Data Flow) — no new
// git/HTTP call originates here; the aiSuggestTimeout-bounded ctx is the
// only thing crossing into main's composed closure.
func (m Model) aiSuggestCmd() tea.Cmd {
	generateSummary := m.deps.GenerateSummary
	if generateSummary == nil {
		return nil
	}
	ticket := m.plan.Ticket
	subjects := commitSubjects(m.plan.SelectedCommits)
	componentSummary := renderComponentSummary(m.summary)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), aiSuggestTimeout)
		defer cancel()
		title, description, err := generateSummary(ctx, ticket, subjects, componentSummary)
		return aiSuggestDoneMsg{title: title, description: description, err: err}
	}
}

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

// discoverCmd runs HU-002 discovery's PASS 1: message+branch search, then
// resolveSource — threading m.originalBranch as the current-branch confirm
// candidate, NO new git/exec call. resolveReady runs the ranged discovery
// inline (PASS 2, unchanged) producing the classified OrderedCommits HU-003
// consumes; resolveNeedsConfirm returns the base result with the PENDING
// candidate and confirm=true WITHOUT running the ranged discover — the
// user's "s" on StateSourceConfirm fires confirmSourceCmd (PASS 2) instead.
// resolveDegrade (or no preliminary target) degrades to message-only
// results, unchanged.
func (m Model) discoverCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	cfg := m.deps.Config
	ticket := m.ticket
	prelim := m.prelim
	currentBranch := m.originalBranch
	ctx := m.ctx()
	return func() tea.Msg {
		base, err := g.Discover(ctx, dir, git.DiscoverOptions{Ticket: ticket, Cfg: cfg})
		if err != nil {
			return discoverDoneMsg{err: err}
		}

		source, outcome := resolveSource(base.CandidateBranches, cfg, prelim, currentBranch)
		switch outcome {
		case resolveNeedsConfirm:
			return discoverDoneMsg{result: base, source: source, confirm: true}
		case resolveReady:
			if prelim == "" {
				// Degrade to message-only results (no target).
				return discoverDoneMsg{result: base}
			}
			full, err := g.Discover(ctx, dir, git.DiscoverOptions{
				Ticket: ticket,
				Target: prelim,
				Source: sourceRefName(source),
				Cfg:    cfg,
			})
			if err != nil {
				return discoverDoneMsg{err: err}
			}
			return discoverDoneMsg{result: full, source: source}
		default: // resolveDegrade
			// Degrade to message-only results (no single source).
			return discoverDoneMsg{result: base}
		}
	}
}

// confirmSourceCmd runs HU-002 discovery's PASS 2 for the StateSourceConfirm
// accept path: once the user presses "s", it funnels the pending candidate
// through the SAME UNMODIFIED git.SelectSingleSource against the base
// candidates, then runs the ranged discover, returning the SAME
// discoverDoneMsg shape (confirm: false) so onDiscoverDone routes it
// straight to StateCommitSelection.
func (m Model) confirmSourceCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	cfg := m.deps.Config
	ticket := m.ticket
	prelim := m.prelim
	candidates := m.discovery.CandidateBranches
	pending := m.source.Name
	ctx := m.ctx()
	return func() tea.Msg {
		sel, err := git.SelectSingleSource(candidates, pending)
		if err != nil {
			return discoverDoneMsg{err: err}
		}
		full, err := g.Discover(ctx, dir, git.DiscoverOptions{
			Ticket: ticket,
			Target: prelim,
			Source: sourceRefName(sel),
			Cfg:    cfg,
		})
		if err != nil {
			return discoverDoneMsg{err: err}
		}
		return discoverDoneMsg{result: full, source: sel}
	}
}

// rePromoteRemapCmd runs the HU-016 patch-id remap through the git service:
// mapping priorSHAs (the prior run's Commits) to their equivalents in
// origin/<target>..origin/<source> — the SAME composed range ordinary
// discovery resolves — via git.Service.RemapCommitsByPatchID (re-promotion
// spec: "Re-Promotion Source Is The Prior Run's Environment Branch" +
// "Patch-ID Remap Of Prior Commits"). A hard remap error (the range itself
// could not be listed) surfaces on rePromoteSeededMsg.err and is handled by
// onRePromoteSeeded exactly like every other command's failure path
// (StateError).
func (m Model) rePromoteRemapCmd(priorSHAs []string, target, source string) tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		result, err := g.RemapCommitsByPatchID(ctx, dir, priorSHAs, target, source)
		if err != nil {
			return rePromoteSeededMsg{err: err}
		}
		return rePromoteSeededMsg{Matched: result.Matched, Unmatched: result.Unmatched}
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

// reuseBranchCmd runs incremental-promotion's StateBranchCollision "reuse &
// append" action, in order: Checkout the existing deploy branch, fast-
// forward-guard it against the already-fetched origin/<deploy> (never a
// force-push — FFDiverged is returned as DATA), then filter the CURRENT
// selection down to the commits not yet present on the reused branch (the
// git-layer layered classifier). sourceRef mirrors m.source's discovered
// name (possibly ""), degrading gracefully per FilterNotOnBranch's own
// contract when no source was ever resolved.
func (m Model) reuseBranchCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	branch := m.branchName
	sourceRef := m.source.Name
	commits := m.plan.SelectedCommits
	ctx := m.ctx()
	return func() tea.Msg {
		if err := g.Checkout(ctx, dir, branch); err != nil {
			return reuseReadyMsg{err: err}
		}

		ffResult, err := g.FastForwardBranch(ctx, dir, branch)
		if err != nil {
			return reuseReadyMsg{err: err}
		}
		if ffResult == git.FFDiverged {
			// Hard stop: never filter/plan against a diverged branch (design
			// "Diverged remote (Q1)" decision) — no push/overwrite attempted.
			return reuseReadyMsg{ffResult: ffResult}
		}

		remaining, err := g.FilterNotOnBranch(ctx, dir, branch, sourceRef, commits)
		if err != nil {
			return reuseReadyMsg{err: err}
		}
		return reuseReadyMsg{remaining: remaining, ffResult: ffResult}
	}
}

// deleteExistingDeployBranch tolerantly removes name's LOCAL branch,
// skipping silently when it does not exist (`LocalBranchExists` exists-guard
// — DeleteLocalBranch's own `-D` force delete would otherwise error on an
// absent branch), and its REMOTE (origin) counterpart, skipping silently
// when it was never pushed (`RemoteBranchExists` exists-guard —
// DeleteRemoteBranch's own doc comment: "deleting a ref that was never
// pushed is simply a git error"). deleteAndRecreateBranchCmd uses this to
// fully clear BOTH sides of a name collision — a pushed branch or a purely
// origin-only collision — before CreatePromotionBranch's own BranchExists
// guard (which checks origin too) re-runs and recreates.
func deleteExistingDeployBranch(ctx context.Context, g *git.Service, dir, name string) error {
	localExists, err := g.LocalBranchExists(ctx, dir, name)
	if err != nil {
		return err
	}
	if localExists {
		if err := g.DeleteLocalBranch(ctx, dir, name); err != nil {
			return err
		}
	}

	remoteExists, err := g.RemoteBranchExists(ctx, dir, name)
	if err != nil {
		return err
	}
	if remoteExists {
		if err := g.DeleteRemoteBranch(ctx, dir, name); err != nil {
			return err
		}
	}

	return nil
}

// deleteAndRecreateBranchCmd runs StateBranchCollision's "delete & recreate"
// action (incremental-promotion spec: "Delete & recreate replaces the branch
// fresh"): deletes the existing promotion branch — LOCAL and REMOTE, each
// only if actually present (deleteExistingDeployBranch) — then re-runs the
// SAME fetch+create CreatePromotionBranch does: a fresh branch from
// origin/<target>, discarding the old one's history and PR linkage. Reuses
// branchCreatedMsg (the same message onBranchCreated already routes) so any
// still-colliding case re-enters StateBranchCollision instead of silently
// succeeding.
func (m Model) deleteAndRecreateBranchCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	target := m.plan.TargetBranch
	name := m.branchName
	ctx := m.ctx()
	return func() tea.Msg {
		if err := deleteExistingDeployBranch(ctx, g, dir, name); err != nil {
			return branchCreatedMsg{err: err}
		}
		return branchCreatedMsg{err: g.CreatePromotionBranch(ctx, dir, target, name)}
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

// spinnerTickMsg drives the progress-spinner frame cadence (design D5),
// mirroring tickMsg's own tea.Tick idiom but at spinnerInterval instead of
// pollInterval, and carrying no service-call payload.
type spinnerTickMsg struct{}

// spinnerCmd schedules the next spinner-frame tick (mirrors tickCmd).
func spinnerCmd() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

// startSpinner is the single-flight guard around spinnerCmd (reliability
// fix): every entry point into one of the 5 spinner states (design D5) must
// route its spinnerCmd() seed through here instead of calling it directly.
// While m.spinning is already true — an earlier entry's tea.Tick loop is
// still running, e.g. across the StateBranchCreation -> StateCherryPicking
// transition — it returns a nil cmd so a SECOND concurrent tick loop is
// never seeded (which used to double the animation's effective frame rate
// and leave a redundant timer running). Otherwise it arms m.spinning and
// returns the spinnerCmd seed. onSpinnerTick is the sole place that clears
// m.spinning back to false, the instant the flow leaves the spinner-state
// set, so the next fresh entry can reseed.
func (m Model) startSpinner() (Model, tea.Cmd) {
	if m.spinning {
		return m, nil
	}
	m.spinning = true
	return m, spinnerCmd()
}

// deltaCmd runs HU-007 delta generation through the delta service and builds
// the HU-008 summary: it composes ONE `sf sgd source delta` invocation
// (origin/<target>..HEAD over the configured sourceDirs), parses the resulting
// package.xml (+ destructiveChanges.xml when present), and folds in the
// git-diff changed-files list for the outside-sourceDirs check. Every external
// command routes through m.deps.Delta / m.deps.Git — internal/app never execs.
func (m Model) deltaCmd() tea.Cmd {
	svc := m.deps.Delta
	g := m.deps.Git
	dir := m.deps.Dir
	cfg := m.deps.Config
	ticket := m.plan.Ticket
	target := m.plan.TargetBranch
	ctx := m.ctx()
	return func() tea.Msg {
		from := "origin/" + target
		to := "HEAD"
		outputDir := filepath.Join(dir, deltaBaseDir(cfg), ticket+"-to-"+target)

		result, err := svc.Generate(ctx, delta.Request{
			Dir:                   dir,
			From:                  from,
			To:                    to,
			OutputDir:             outputDir,
			SourceDirs:            cfg.Delta.SourceDirs,
			IgnoreFile:            cfg.Delta.IgnoreFile,
			IgnoreDestructiveFile: cfg.Delta.IgnoreDestructiveFile,
		})
		if err != nil {
			// sgd failure: the error text carries the raw sgd output; no
			// artifacts, so downstream validation is never reached.
			return deltaDoneMsg{err: err}
		}

		pkg, err := parsePackageFile(result.PackageXMLPath, false)
		if err != nil {
			return deltaDoneMsg{err: err}
		}
		var destructive delta.Package
		if result.DestructiveChangesPath != "" {
			destructive, err = parsePackageFile(result.DestructiveChangesPath, true)
			if err != nil {
				return deltaDoneMsg{err: err}
			}
		}

		// OutsideSourceDirs is informational: a transient git-diff error must
		// not sink an otherwise successful delta, so ChangedFiles is
		// best-effort (mirrors depWarningsCmd's degrade-to-empty policy).
		changed, _ := g.ChangedFiles(ctx, dir, from, to)

		summary := delta.Summarize(pkg, destructive, changed, cfg.Delta.SourceDirs)
		return deltaDoneMsg{result: result, summary: summary}
	}
}

// parsePackageFile reads and parses one sgd-generated manifest. destructive
// selects ParseDestructive vs ParsePackage (identical schema, different role).
func parsePackageFile(path string, destructive bool) (delta.Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return delta.Package{}, err
	}
	if destructive {
		return delta.ParseDestructive(data)
	}
	return delta.ParsePackage(data)
}

// validateCmd runs HU-010 async validate through the Salesforce shim and, on
// a jobId, persists the run IMMEDIATELY (run-persistence spec: "before any
// further step"). A CLI error returns the message + raw for display without
// crashing; a persistence error still surfaces the jobId so the run is
// recoverable.
//
// HU-013: when m.runID is ALREADY set (a branch-creation run precedes
// validate, the normal in-flow path), the jobId + Phase="validating" are
// MERGED into that already-persisted record via Load+Save — preserving
// Ticket/PickTotal/Commits/CreatedAt — rather than creating a second run for
// the same promotion. When m.runID is empty (the standalone HU-010 path:
// validate reached with no prior branch-creation run, e.g. a re-attach flow
// or a pre-HU-013 caller), the ORIGINAL derivation + Create fallback runs
// unchanged, keeping HU-010/011's existing behavior intact.
func (m Model) validateCmd() tea.Cmd {
	sf := m.deps.SF
	writer := m.deps.Runs
	now := m.now()
	dir := m.deps.Dir
	plan := m.plan
	runID := m.runID
	ctx := m.ctx()
	return func() tea.Msg {
		result, err := sf.ValidateDeploy(ctx, salesforce.ValidateRequest{
			Dir:                 dir,
			ManifestPath:        plan.PackageXMLPath,
			PostDestructivePath: plan.DestructiveChangesPath,
			TargetOrg:           plan.SandboxAlias,
			TestLevel:           plan.TestLevel,
		})
		if err != nil {
			// D7: persist the raw launch-failure envelope so its path can be
			// surfaced alongside the message (deploy-validation spec "Launch
			// Error Shows An Actionable Message And The Persisted-Raw Path").
			// A pre-created run (runID already known, e.g. standalone-validate's
			// Mode="validate" record) reuses it via SaveRawCompanion, mirroring
			// the established writer-companion pattern; with no prior run this
			// falls back to Create(rec{Status:"Failed"}, raw) — the SAME
			// fallback-id derivation the success no-runID branch below uses —
			// since Create already writes validate.json itself. Best-effort:
			// a persistence hiccup never replaces the original launch error.
			if writer == nil {
				return validateDoneMsg{result: result, err: err}
			}
			if runID != "" {
				if path, serr := writer.SaveRawCompanion(runID, "validate.json", []byte(result.Raw)); serr == nil {
					return validateDoneMsg{result: result, err: err, rawPath: path}
				}
				return validateDoneMsg{result: result, err: err}
			}
			fallbackID := plan.Ticket + "-to-" + plan.TargetBranch + "-" + now.Format("20060102150405")
			failedRunDir, cerr := writer.Create(runs.Record{
				RunID:     fallbackID,
				Ticket:    plan.Ticket,
				Target:    plan.TargetBranch,
				Alias:     plan.SandboxAlias,
				Status:    "Failed",
				CreatedAt: now,
				UpdatedAt: now,
			}, []byte(result.Raw))
			if cerr != nil {
				return validateDoneMsg{result: result, err: err}
			}
			return validateDoneMsg{result: result, err: err, runID: fallbackID, rawPath: filepath.Join(failedRunDir, "validate.json")}
		}

		if runID != "" {
			if writer == nil {
				return validateDoneMsg{result: result, runID: runID}
			}
			rec, lerr := writer.Load(runID)
			if lerr != nil {
				return validateDoneMsg{result: result, runID: runID, err: lerr}
			}
			rec.JobID = result.JobID
			rec.Status = "Queued"
			rec.Phase = "validating"
			rec.UpdatedAt = now
			if serr := writer.Save(rec); serr != nil {
				return validateDoneMsg{result: result, runID: runID, err: serr}
			}
			return validateDoneMsg{result: result, runID: runID}
		}

		// Fallback: derive the runID exactly as HU-010 originally did, and
		// create a fresh record (no prior branch-creation run to reuse).
		fallbackID := plan.Ticket + "-to-" + plan.TargetBranch + "-" + now.Format("20060102150405")
		if writer == nil {
			return validateDoneMsg{result: result, runID: fallbackID}
		}
		runDir, perr := writer.Create(runs.Record{
			RunID:     fallbackID,
			Ticket:    plan.Ticket,
			Target:    plan.TargetBranch,
			Alias:     plan.SandboxAlias,
			JobID:     result.JobID,
			Status:    "Queued",
			CreatedAt: now,
			UpdatedAt: now,
		}, []byte(result.Raw))
		return validateDoneMsg{result: result, runID: fallbackID, runDir: runDir, err: perr}
	}
}

// resumeDetectCmd composes the HU-013 startup resume-detection: it reads the
// live RepoState (Git.RepoState — the same authority the cherry-pick screens
// poll, no new exec seam) and the persisted run list (Runs.List) so
// onResumeDetect can offer to continue an in-progress cherry-pick or a
// non-terminal validation job. Best-effort: a nil Git or Runs writer disables
// detection (returns nil), and internal/app still NEVER execs directly — every
// call routes through the injected services (boundary_test.go holds).
func (m Model) resumeDetectCmd() tea.Cmd {
	g := m.deps.Git
	writer := m.deps.Runs
	if g == nil || writer == nil {
		return nil
	}
	dir := m.deps.Dir
	parent := m.ctx()
	return func() tea.Msg {
		// Bound the git-backed RepoState read so a slow/hung git never stalls
		// startup detection; on timeout it errors and onResumeDetect falls back
		// to the normal flow.
		ctx, cancel := context.WithTimeout(parent, resumeDetectTimeout)
		defer cancel()
		state, err := g.RepoState(ctx, dir)
		if err != nil {
			return resumeDetectMsg{err: err}
		}
		records, err := writer.List()
		return resumeDetectMsg{state: state, records: records, err: err}
	}
}

// originalBranchCmd captures the branch checked out at flow startup —
// BEFORE any promotion branch is created — through git.Service.CurrentBranch,
// so quitCmd can restore it later (branch-cleanup spec: "Original Branch
// Restored On Finish Or Abort"). It is fired batched alongside
// resumeDetectCmd from onPrereqDone: the reducer itself must stay pure, so
// capture is a cmd+msg round trip, never a synchronous field write (design.md
// "Refines exploration"). A nil Git degrades to no command, mirroring
// resumeDetectCmd's best-effort nil — tea.Batch drops it cleanly.
func (m Model) originalBranchCmd() tea.Cmd {
	g := m.deps.Git
	if g == nil {
		return nil
	}
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		branch, err := g.CurrentBranch(ctx, dir)
		return originalBranchMsg{branch: branch, err: err}
	}
}

// unpushedCountCmd runs git.Service.UnpushedCommitCount for branch through
// the injected service (never an app-level exec), gating whether the
// inline current-branch delete confirmation is normal or strong
// (design.md "Unpushed gate: on d, fire unpushedCountCmd(name); >0 -> strong
// (BORRAR), else normal (y) confirm — one call per attempt, no N+1").
func (m Model) unpushedCountCmd(branch string) tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		count, err := g.UnpushedCommitCount(ctx, dir, branch)
		return unpushedMsg{branch: branch, count: count, err: err}
	}
}

// quitCmd builds the terminal-quit command every quit key site fires
// (design.md "Quit-restore mechanism + guards"). When warranted, it restores
// the original branch SYNCHRONOUSLY (git.Checkout) before returning
// tea.Quit()'s QuitMsg — Bubble Tea runs a Cmd to completion before
// delivering its message, so the checkout finishes before Program.Run()
// returns and main.go needs no change.
//
// Guard, pure/model-state (checked BEFORE building the closure, so a
// disqualified quit fires no git call at all): deps.Git == nil (unit tests /
// best-effort degrade), m.repoState.InProgress (git REFUSES checkout with
// unmerged paths — this is the rationale, NOT a resume-detection need;
// CHERRY_PICK_HEAD lives in .git/ and is worktree-scoped, so it survives
// regardless of which branch is checked out, keeping HU-013 resume-detection
// intact), or m.originalBranch being empty/detached ("HEAD" — never
// captured, or a detached-HEAD startup).
//
// Git-dependent guards, evaluated live INSIDE the closure via shouldRestore:
// the current branch is RE-QUERIED (never the possibly-stale model field) —
// the flow checks out the deploy branch mid-run (CreatePromotionBranch), so
// the branch at quit time is never assumed equal to what it was at
// startup — and BranchExists(original) confirms it still resolves. Either
// call erroring is treated as "cannot confirm restore is safe" and skips it,
// same as an ineligible guard.
//
// HU-017 inline delete: when m.pendingDeleteCurrent is set (an explicit
// normal/strong delete confirmation already landed), quitCmd ALSO deletes
// the current run's own branch. The branch is ALWAYS m.plan.PromotionBranch —
// never m.branchName, which stays empty on a resumed run (resumeInto never
// sets it, only reconstructs plan.PromotionBranch) — and the remote ref is
// also deleted iff m.currentPushed.
//
// The delete is DECOUPLED from the restore checkout (review M-1): it fires
// whenever it is confirmed and we are not currently standing on the target
// branch, EVEN WHEN the restore was skipped (a resumed run where
// current==original, a detached/gone original, ...). It is only the
// current==target residual — still standing on the branch because restore
// couldn't move HEAD off it — that stays a SAFE no-op (`git branch -D` of the
// checked-out branch is refused anyway, and we never force a checkout to an
// invalid branch). Mid-conflict (RepoState.InProgress) still skips BOTH
// restore and delete: the branch holding CHERRY_PICK_HEAD is never safe to
// delete, and HU-013 resume-detection must stay intact.
func (m Model) quitCmd() tea.Cmd {
	if m.deps.Git == nil || m.repoState.InProgress {
		return tea.Quit
	}
	del, name, pushed := m.pendingDeleteCurrent, m.plan.PromotionBranch, m.currentPushed
	// A restore is only warranted with a concrete, non-detached original branch
	// to return to. The delete no longer depends on this being true.
	canRestore := m.originalBranch != "" && m.originalBranch != "HEAD"
	if !canRestore && !del {
		return tea.Quit
	}
	g, dir, ctx, original := m.deps.Git, m.deps.Dir, m.ctx(), m.originalBranch
	return func() tea.Msg {
		current, err := g.CurrentBranch(ctx, dir)
		if err != nil {
			// The live branch can't be confirmed — do nothing destructive
			// (neither restore nor delete) rather than act on an unknown branch.
			return tea.Quit()
		}

		// Restore the original branch when warranted (unchanged guard). Only
		// attempt it when we are actually off the original branch.
		if canRestore && current != original {
			exists := false
			if e, berr := g.BranchExists(ctx, dir, original); berr == nil {
				exists = e
			}
			if shouldRestore(false, original, current, exists) {
				if g.Checkout(ctx, dir, original) == nil {
					current = original // HEAD is now on the original branch
				}
			}
		}

		// Delete the current run's own branch when confirmed — INDEPENDENT of
		// whether the restore above happened. Refused as a safe no-op only when
		// we are still standing on that very branch, or there is no branch name.
		if del && name != "" && current != name {
			_ = g.DeleteLocalBranch(ctx, dir, name)
			if pushed {
				_ = g.DeleteRemoteBranch(ctx, dir, name)
			}
		}
		return tea.Quit()
	}
}

// queueCmd resolves the current run's own-job identity through
// Client.Orgs()->FindByAlias(alias).Username (zero new subprocess type: reuses
// the existing org-list call) and queries the active DeployRequest queue
// (HU-009), under a shared per-call timeout. Identity resolution is
// best-effort: an Orgs() failure or a missing alias degrades to an empty
// identity (no own-job highlight) WITHOUT blocking the queue query itself.
func (m Model) queueCmd() tea.Cmd {
	sf := m.deps.SF
	alias := m.plan.SandboxAlias
	parent := m.ctx()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, queueCallTimeout)
		defer cancel()

		var identity string
		if orgs, err := sf.Orgs(ctx); err == nil {
			if org, ok := orgs.FindByAlias(alias); ok {
				identity = org.Username
			}
		}

		entries, err := sf.ListDeployQueue(ctx, alias)
		return queueDoneMsg{entries: entries, identity: identity, err: err}
	}
}

// cancelCmd runs HU-012's `sf project deploy cancel` through the Salesforce
// shim for the CURRENT run's own job ONLY. It targets m.jobID — NEVER
// m.cancelInput (which is just the typed CANCELAR confirmation literal) and
// never another user's job — so a cancel can only ever hit the run's own
// validation. The alias is the plan's configured SandboxAlias. Persistence
// (MarkCanceled) and the terminal transition happen in onCancelDone.
func (m Model) cancelCmd() tea.Cmd {
	sf := m.deps.SF
	jobID := m.jobID
	alias := m.plan.SandboxAlias
	ctx := m.ctx()
	return func() tea.Msg {
		result, err := sf.CancelDeploy(ctx, jobID, alias)
		return cancelDoneMsg{result: result, err: err}
	}
}

// quickDeployCmd runs HU-015's `sf project deploy quick` through the
// Salesforce shim, for the SELECTED HISTORY ROW's own job ONLY: jobID and
// alias are captured HERE, synchronously, from m.runs[m.runsCursor] — never
// m.jobID/m.plan.SandboxAlias, which belong to whatever run is currently
// in-flight, not the historical row being quick-deployed. keyQuickDeploy only
// calls this once every gate (quickDeployExecAllowed + the typed DESPLEGAR
// confirmation) has already passed. Persistence (MarkQuickDeployed) and
// staying on StateQuickDeploy happen in onQuickDeployDone.
func (m Model) quickDeployCmd() tea.Cmd {
	sf := m.deps.SF
	rec := m.runs[m.runsCursor]
	jobID := rec.JobID
	alias := rec.Alias
	ctx := m.ctx()
	return func() tea.Msg {
		result, err := sf.QuickDeploy(ctx, jobID, alias)
		return quickDeployDoneMsg{result: result, err: err}
	}
}

// reportCmd polls HU-011's `sf project deploy report` ONCE through the shim,
// under a per-call timeout so a single hung poll cannot stall the loop. The
// per-call context is derived from the cancelable polling SESSION context
// (m.pollContext), so a user exit cancels an in-flight report subprocess.
// Callers (onValidateDone / onPollTick) schedule the repetition.
func (m Model) reportCmd() tea.Cmd {
	sf := m.deps.SF
	jobID := m.jobID
	alias := m.plan.SandboxAlias
	dir := m.deps.Dir
	parent := m.pollContext()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, reportCallTimeout)
		defer cancel()
		report, err := sf.ReportDeploy(ctx, jobID, alias, dir)
		return reportDoneMsg{report: report, err: err}
	}
}

// pollTickCmd schedules the next ValidationPolling tick at the configured
// interval (a different, slower cadence than the cherry-pick tickCmd).
func pollTickCmd(seconds int) tea.Cmd {
	return tea.Tick(time.Duration(seconds)*time.Second, func(time.Time) tea.Msg { return pollTickMsg{} })
}

// pushCmd runs HU-014's `git push -u origin <PromotionBranch>` through the git
// service (never an app-level exec). It is fired only after the explicit `p`
// confirm on the push-preparation screen (spec: "show the push command before
// running it").
func (m Model) pushCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	branch := m.plan.PromotionBranch
	ctx := m.ctx()
	return func() tea.Msg {
		return pushDoneMsg{err: g.Push(ctx, dir, branch)}
	}
}

// preparePRCmd composes the HU-014 post-push PR-preparation data: ONE
// `gh auth status` (through the github.Client — AuthAbsent when no client is
// wired) plus `git remote get-url origin` (through the git service), from
// which it derives the compare URL. A failed RemoteURL degrades to remoteErr;
// an unrecognized origin form degrades to compareErr — both surface the raw
// origin + manual data rather than a malformed link. internal/app never execs:
// gh and git are reached only through the injected clients.
func (m Model) preparePRCmd() tea.Cmd {
	g := m.deps.Git
	gh := m.deps.GH
	dir := m.deps.Dir
	base := m.plan.TargetBranch
	head := m.plan.PromotionBranch
	reusing := m.reusing
	ctx := m.ctx()
	return func() tea.Msg {
		auth := github.AuthAbsent
		if gh != nil {
			auth = gh.AuthStatus(ctx)
		}
		msg := prepDoneMsg{auth: auth}

		// incremental-promotion: only a REUSED branch could already have an
		// open PR (a brand-new branch never does) — gated on the
		// authenticated path exactly like every other gh call in this file.
		// A PRForBranch error degrades silently (msg.prURL/prOpen stay zero)
		// so a transient gh hiccup never blocks the rest of this prep.
		if reusing && auth == github.AuthAuthenticated {
			if url, open, perr := gh.PRForBranch(ctx, head); perr == nil {
				msg.prURL = url
				msg.prOpen = open
			}
		}

		originURL, rerr := g.RemoteURL(ctx, dir, "origin")
		if rerr != nil {
			msg.remoteErr = rerr
			return msg
		}
		msg.originURL = originURL

		if compareURL, cerr := github.CompareURL(originURL, base, head); cerr != nil {
			msg.compareErr = cerr
		} else {
			msg.compareURL = compareURL
		}
		return msg
	}
}

// listDeployBranchesCmd runs HU-017's StateBranchCleanup list load: ONE
// git.Service.ListDeployBranches call plus a best-effort runs.List() read,
// correlated via selectOrphans (excluding a live in-progress run's own
// branch) into the orphan set, then labeled via IsMergedInto — one ancestor
// check per orphan, run ONLY when resolveMergeTarget finds a correlating
// record's Target (branch-cleanup spec: "or 'unknown' when target can't be
// determined"). A nil Git degrades to an explicit error (never a panic); a
// nil Runs writer or a runs.List() failure degrades to no records — every
// branch is then treated as an orphan and every label as "unknown", mirroring
// this file's other best-effort degrades.
func (m Model) listDeployBranchesCmd() tea.Cmd {
	g := m.deps.Git
	writer := m.deps.Runs
	dir := m.deps.Dir
	format := m.deps.Config.BranchFormat
	inProgress := m.repoState.InProgress
	ctx := m.ctx()
	return func() tea.Msg {
		if g == nil {
			return deployBranchesMsg{err: errors.New("app: no git service configured")}
		}
		branches, err := g.ListDeployBranches(ctx, dir)
		if err != nil {
			return deployBranchesMsg{err: err}
		}

		var records []runs.Record
		if writer != nil {
			records, _ = writer.List()
		}

		orphans := selectOrphans(branches, records, format, inProgress)
		rows := make([]cleanupRow, len(orphans))
		for i, b := range orphans {
			target, ok := resolveMergeTarget(records, format, b.Name)
			var merged bool
			var merr error
			if ok {
				merged, merr = g.IsMergedInto(ctx, dir, b.Name, target)
			}
			rows[i] = cleanupRow{DeployBranch: b, MergedLabel: mergedLabel(ok, merged, merr)}
		}
		return deployBranchesMsg{branches: rows}
	}
}

// standaloneBranchesMsg carries HU-018's standalone-delta base-branch list
// (Group 3, StateDeltaSourceSelect): git.ListBranches's full local+
// remote-tracking result, or a load error surfaced on the picker screen
// (never a crash — mirrors deployBranchesMsg's degrade).
type standaloneBranchesMsg struct {
	branches []git.Branch
	err      error
}

// standaloneBranchesCmd lists local+remote-tracking branches for HU-018's
// standalone-delta base-branch picker, mirroring onDeployBranches/
// listDeployBranchesCmd's async pattern. A nil Git service (never expected
// outside tests) degrades to an explicit error rather than a panic.
func (m Model) standaloneBranchesCmd() tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		if g == nil {
			return standaloneBranchesMsg{err: errors.New("app: no git service configured")}
		}
		branches, err := g.ListBranches(ctx, dir)
		if err != nil {
			return standaloneBranchesMsg{err: err}
		}
		return standaloneBranchesMsg{branches: branches}
	}
}

// deleteOrphanCmd deletes ONE StateBranchCleanup row's branch — local always,
// remote iff pushed — through the SAME git.Service primitives quitCmd's
// inline current-branch delete uses (DeleteLocalBranch/DeleteRemoteBranch).
// Unlike quitCmd's delete, this branch is never the one checked out, so no
// restore/checkout choreography is needed: it is deleted directly.
func (m Model) deleteOrphanCmd(name string, pushed bool) tea.Cmd {
	g := m.deps.Git
	dir := m.deps.Dir
	ctx := m.ctx()
	return func() tea.Msg {
		if err := g.DeleteLocalBranch(ctx, dir, name); err != nil {
			return deleteDoneMsg{err: err}
		}
		if pushed {
			if err := g.DeleteRemoteBranch(ctx, dir, name); err != nil {
				return deleteDoneMsg{err: err}
			}
		}
		return deleteDoneMsg{}
	}
}

// pruneRunsCmd invokes the EXISTING, UNCHANGED run-retention mechanism
// (branch-cleanup spec: "Run Retention Applied From The Cleanup Surface" —
// "MUST NOT redefine, reimplement, or change it") from the StateBranchCleanup
// screen, using the same injected clock (m.now()) every other timed command
// in this file uses.
func (m Model) pruneRunsCmd() tea.Cmd {
	writer := m.deps.Runs
	keepLast := m.deps.Config.Runs.KeepLast
	keepDays := m.deps.Config.Runs.KeepDays
	now := m.now()
	return func() tea.Msg {
		if writer == nil {
			return pruneDoneMsg{err: errors.New("app: no runs writer configured")}
		}
		removed, err := writer.Prune(keepLast, keepDays, now)
		return pruneDoneMsg{removed: removed, err: err}
	}
}

// createPRCmd runs HU-014's `gh pr create` through the github.Client. It is
// fired ONLY after the explicit `y` confirmation on the authed path
// (keyPushPreparation's pushPRConfirm gate) — never implicitly, and never on
// the compare-fallback path. Raw is preserved on both success and failure so a
// failed creation can still show gh's output. A nil GH client returns an error
// rather than panicking (the flow degrades to manual data). The title comes
// from effectiveTitle() and the body from effectiveDescription() (ai-pr-summary
// design ADR-3): the accepted AI title/description ONLY after the explicit
// second 'a' accept, else the pre-existing github.SuggestedTitle formula /
// "" body — the exact same sources viewPRData displays, so the executed
// command can never drift from the shown one.
//
// pr-provenance additive growth: the submitted body always gains the visible
// footer plus the invisible signed marker via provenance.Compose, on BOTH
// the AI-accepted and non-AI paths (push-pr-preparation spec: "PR Creation
// Requires Explicit Confirmation And Records The URL"). ownerRepo is
// derived from m.originURL (already captured by preparePRCmd before this
// screen is reachable) via github.OwnerRepo; when it cannot be parsed, the
// signature payload has no repo to bind to, so Compose degrades to a
// footer-only body with NO marker — creation itself proceeds normally
// either way.
func (m Model) createPRCmd() tea.Cmd {
	gh := m.deps.GH
	base := m.plan.TargetBranch
	head := m.plan.PromotionBranch
	title := m.effectiveTitle()
	ownerRepo, _ := github.OwnerRepo(m.originURL)
	body := provenance.Compose(m.effectiveDescription(), ownerRepo, head, m.runID)
	ctx := m.ctx()
	return func() tea.Msg {
		if gh == nil {
			return prCreatedMsg{err: errors.New("app: no gh client configured")}
		}
		url, raw, err := gh.CreatePR(ctx, base, head, title, body)
		return prCreatedMsg{url: url, raw: raw, err: err}
	}
}

// --- deploy-gate --------------------------------------------------------

// errNoGitHubClient is returned by deploy-gate's gh reads when Deps.GH is
// nil — never a panic. A gate must fail closed exactly like a real gh
// failure would (design.md's "any gh read error ... captured into the
// matching Facts.*Err" contract), so a missing client degrades to the same
// per-condition failure a genuine gh error would produce.
var errNoGitHubClient = errors.New("app: no gh client configured")

// resolvePRURL implements deploy-gate's fail-closed PR resolution (deploy-
// gate spec: "Fail-Closed PR Resolution"): rec.PRUrl is used when already
// recorded; an empty PRUrl falls back to locating the PR by the run's own
// promotion-branch name (PRForBranch(RenderBranchName(branchFormat,
// rec.Ticket, rec.Target))). Neither path resolving (including a nil gh
// client) returns ok=false — the caller BLOCKS rather than skips
// evaluation, never silently treating "no PR" as passing.
func resolvePRURL(ctx context.Context, gh github.Client, rec runs.Record, branchFormat string) (string, bool) {
	if rec.PRUrl != "" {
		return rec.PRUrl, true
	}
	if gh == nil {
		return "", false
	}
	branch := git.RenderBranchName(branchFormat, rec.Ticket, rec.Target)
	url, _, err := gh.PRForBranch(ctx, branch)
	if err != nil || url == "" {
		return "", false
	}
	return url, true
}

// gateCheckDoneMsg carries a completed deploy-gate evaluation (design.md's
// "gateCheckCmd" data flow): always a fully-formed gate.Result, whatever the
// individual gh reads did — gateCheckCmd never panics and never returns a
// bare error, only a Result whose conditions reflect any read failure
// fail-closed.
type gateCheckDoneMsg struct {
	result gate.Result
}

// gateCheckCmd runs deploy-gate's async evaluation for the CURRENTLY
// SELECTED run (m.runs[m.runsCursor], mirroring quickDeployCmd's own
// selection): resolvePRURL, then PRReviews/UnresolvedThreadCount/
// PRComments/verifyProvenance (D11) — mapping ANY gh read error
// independently into the MATCHING Facts.*Err (never a blanket failure, and
// NEVER a crash) — before calling gate.Evaluate. When resolvePRURL cannot
// resolve a PR at all, the 4 reads are skipped entirely and
// Facts.PRResolved stays false (gate.Evaluate then reports a single failed
// pr-resolution condition).
func (m Model) gateCheckCmd() tea.Cmd {
	gh := m.deps.GH
	rec := m.runs[m.runsCursor]
	gateCfg, _ := m.deps.Config.GateFor(rec.Target)
	branchFormat := m.deps.Config.BranchFormat
	parent := m.ctx()
	return func() tea.Msg {
		// gateCheckTimeout bounds the WHOLE sequence below (resolvePRURL's
		// fallback lookup plus the 4 independent reads) under ONE shared
		// deadline (mirrors queueCmd's discipline) — a hung gh now fails
		// closed via the normal Facts.*Err -> failed-condition path instead
		// of stalling gateCheckDoneMsg indefinitely.
		ctx, cancel := context.WithTimeout(parent, gateCheckTimeout)
		defer cancel()

		facts := gate.Facts{Config: gateCfg}

		prURL, ok := resolvePRURL(ctx, gh, rec, branchFormat)
		facts.PRResolved = ok
		if !ok {
			return gateCheckDoneMsg{result: gate.Evaluate(facts)}
		}

		if gh == nil {
			// Defensive only: resolvePRURL(ctx, nil, ...) can return ok=true
			// solely via rec.PRUrl already being set, without ever needing gh —
			// still map every remaining read as failed rather than crash on a
			// nil Client.
			facts.ReviewsErr = errNoGitHubClient
			facts.ThreadErr = errNoGitHubClient
			facts.CommentErr = errNoGitHubClient
			facts.ProvenanceErr = errNoGitHubClient
			return gateCheckDoneMsg{result: gate.Evaluate(facts)}
		}

		if reviews, err := gh.PRReviews(ctx, prURL); err != nil {
			facts.ReviewsErr = err
		} else {
			facts.Reviews = mapReviews(reviews)
		}

		if unresolved, err := gh.UnresolvedThreadCount(ctx, prURL); err != nil {
			facts.ThreadErr = err
		} else {
			facts.UnresolvedCount = unresolved
		}

		if comments, err := gh.PRComments(ctx, prURL); err != nil {
			facts.CommentErr = err
		} else {
			facts.CommentPresent = gate.HasValidationComment(commentBodies(comments))
		}

		if result, err := verifyProvenance(ctx, gh, prURL); err != nil {
			facts.ProvenanceErr = err
		} else {
			facts.Provenance = result
		}

		return gateCheckDoneMsg{result: gate.Evaluate(facts)}
	}
}

// verifyProvenance composes deploy-gate's signature condition source
// (design.md D11): PRDetails -> provenance.ParseMarkers -> provenance.
// BestResult — the SAME ranking implementation `pr verify` uses (cmd/
// deploydeck's bestResult seam), so both share one classification. A gh/
// parse failure fetching the PR body returns an error (the caller maps it
// to Facts.ProvenanceErr, fail-closed); a PR body with NO marker at all is
// legitimate data, not a failure — it classifies as provenance.Mismatch
// (never the zero-valued provenance.Verified, which would wrongly pass an
// absent signature).
func verifyProvenance(ctx context.Context, gh github.Client, prURL string) (provenance.Result, error) {
	ownerRepo, ok := github.ParsePRURL(prURL)
	if !ok {
		return 0, fmt.Errorf("app: could not parse owner/repo from PR URL %q", prURL)
	}
	headBranch, body, err := gh.PRDetails(ctx, prURL)
	if err != nil {
		return 0, fmt.Errorf("app: fetching PR details: %w", err)
	}
	markers := provenance.ParseMarkers(body)
	if len(markers) == 0 {
		return provenance.Mismatch, nil
	}
	result, _ := provenance.BestResult(ownerRepo, headBranch, markers)
	return result, nil
}

// mapReviews maps github.Review DTOs into gate.ApproverReview value types at
// the app boundary (design.md's "Gate package" decision: gate stays pure,
// app owns the gh-DTO -> gate-value mapping).
func mapReviews(reviews []github.Review) []gate.ApproverReview {
	mapped := make([]gate.ApproverReview, len(reviews))
	for i, r := range reviews {
		mapped[i] = gate.ApproverReview{Login: r.Login, State: r.State}
	}
	return mapped
}

// commentBodies extracts just the Body field from github.Comment DTOs, the
// shape gate.HasValidationComment consumes.
func commentBodies(comments []github.Comment) []string {
	bodies := make([]string, len(comments))
	for i, c := range comments {
		bodies[i] = c.Body
	}
	return bodies
}

// postCommentDoneMsg carries the deploy-gate validation-comment post/upsert
// outcome (best-effort — see onPostCommentDone).
type postCommentDoneMsg struct {
	err error
}

// requireValidationCommentOn resolves GateConfig.RequireValidationComment's
// default-on toggle (nil/omitted -> on; explicit false -> off) — the SAME
// semantics internal/gate.Evaluate applies to its own condition toggles.
// This is purely a dispatch-gating decision, not part of the gate evaluation
// itself, so it stays here rather than inline in gate.Evaluate — but it
// reuses gate.ToggleOn (remediation-pass readability fix) instead of
// duplicating the nil-check, so the default-on convention lives in ONE place.
func requireValidationCommentOn(cfg config.GateConfig) bool {
	return gate.ToggleOn(cfg.RequireValidationComment)
}

// postValidationCommentCmd composes and posts (or skips, if already
// present) deploy-gate's validation-comment (deploy-gate spec:
// "Validation-Comment Condition And Posting"; validation-progress spec:
// "Terminal Successful CheckOnly Triggers The Deploy-Gate Validation
// Comment"). It is fired ONLY from onReportDone's already-terminal-SUCCESS
// branch — best-effort and async: resolvePRURL failing to find a PR, or any
// gh read/write error, degrades to a silent skip/no-op (postCommentDoneMsg
// carries the error for onPostCommentDone's best-effort landing, but never
// blocks or alters the terminal-success screen itself). Re-validation of
// the same job skips posting when a marker comment already exists on the
// PR — never a duplicate.
func (m Model) postValidationCommentCmd() tea.Cmd {
	gh := m.deps.GH
	format := m.deps.Config.BranchFormat
	rec := runs.Record{PRUrl: m.prURL, Ticket: m.plan.Ticket, Target: m.plan.TargetBranch}
	jobID := m.jobID
	runID := m.runID
	report := m.report
	ctx := m.ctx()
	return func() tea.Msg {
		if gh == nil {
			return postCommentDoneMsg{}
		}
		prURL, ok := resolvePRURL(ctx, gh, rec, format)
		if !ok {
			return postCommentDoneMsg{}
		}

		comments, err := gh.PRComments(ctx, prURL)
		if err != nil {
			return postCommentDoneMsg{err: err}
		}
		if gate.HasValidationComment(commentBodies(comments)) {
			return postCommentDoneMsg{} // already present: skip, never a duplicate
		}

		pct, known := gate.AggregateCoverage(coverageTotals(report.CodeCoverage), coverageNotCovered(report.CodeCoverage))
		_, body := gate.ValidationComment(jobID, runID, report.NumberComponentErrors, report.NumberTestErrors, pct, known)
		_, err = gh.PostComment(ctx, prURL, body)
		return postCommentDoneMsg{err: err}
	}
}

// coverageTotals/coverageNotCovered project salesforce.DeployReport's
// per-class CodeCoverage into the two parallel []int slices
// gate.AggregateCoverage consumes.
func coverageTotals(cov []salesforce.CodeCoverageResult) []int {
	totals := make([]int, len(cov))
	for i, c := range cov {
		totals[i] = c.NumLocations
	}
	return totals
}

func coverageNotCovered(cov []salesforce.CodeCoverageResult) []int {
	notCovered := make([]int, len(cov))
	for i, c := range cov {
		notCovered[i] = c.NumLocationsNotCovered
	}
	return notCovered
}
