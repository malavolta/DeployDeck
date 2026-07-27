package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/delta"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// reportCallTimeout bounds a single `sf project deploy report` invocation
// (design.md: "Each ReportDeploy gets a per-call context.WithTimeout"). It is
// deliberately shorter than any sane poll timeout so one hung poll can never
// stall the whole ValidationPolling loop.
const reportCallTimeout = 60 * time.Second

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

// deltaDoneMsg carries the HU-007 delta result and HU-008 package summary
// (or an sgd/parse error, which keeps the user on DeltaGeneration).
type deltaDoneMsg struct {
	result  delta.Result
	summary delta.PackageSummary
	err     error
}

// validateDoneMsg carries the HU-010 validate outcome: the jobId-bearing
// result, the persisted run id/dir, or an error (CLI error or persistence
// failure — the flow stays alive either way).
type validateDoneMsg struct {
	result salesforce.ValidateResult
	runID  string
	runDir string
	err    error
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

// validateCmd runs HU-010 async validate through the Salesforce shim and, on a
// jobId, persists the run IMMEDIATELY (run-persistence spec: "before any
// further step"). A CLI error returns the message + raw for display without
// crashing; a persistence error still surfaces the jobId so the run is
// recoverable.
func (m Model) validateCmd() tea.Cmd {
	sf := m.deps.SF
	writer := m.deps.Runs
	now := m.now()
	dir := m.deps.Dir
	plan := m.plan
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
			return validateDoneMsg{result: result, err: err}
		}

		runID := plan.Ticket + "-to-" + plan.TargetBranch + "-" + now.Format("20060102150405")
		if writer == nil {
			return validateDoneMsg{result: result, runID: runID}
		}
		runDir, perr := writer.Create(runs.Record{
			RunID:     runID,
			Ticket:    plan.Ticket,
			Target:    plan.TargetBranch,
			Alias:     plan.SandboxAlias,
			JobID:     result.JobID,
			Status:    "Queued",
			CreatedAt: now,
			UpdatedAt: now,
		}, []byte(result.Raw))
		return validateDoneMsg{result: result, runID: runID, runDir: runDir, err: perr}
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
