package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"deploydeck/internal/git"
	"deploydeck/internal/github"
	"deploydeck/internal/runs"
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
	case StateDeltaGeneration:
		return m.keyDeltaGeneration(msg)
	case StatePackageReview:
		return m.keyPackageReview(msg)
	case StateQueueReview:
		return m.keyQueueReview(msg)
	case StateValidationStart:
		return m.keyValidationStart(msg)
	case StateValidationPolling:
		return m.keyValidationPolling(msg)
	case StateCancelConfirm:
		return m.keyCancelConfirm(msg)
	case StateRunHistory:
		return m.keyRunHistory(msg)
	case StateSucceeded:
		return m.keySucceeded(msg)
	case StatePushPreparation:
		return m.keyPushPreparation(msg)
	case StateAborted:
		return m.keyAborted(msg)
	case StateError, StateFailed, StateCanceled:
		if key := msg.String(); key == "q" || key == "enter" || key == "esc" {
			return m, m.quitCmd()
		}
	}
	return m, nil
}

// keySucceeded handles the terminal SUCCESS screen (HU-014/HU-017). Unlike
// the always-quit-only terminals (Failed/Canceled/Error), success offers a
// distinct `p` key that enters StatePushPreparation to push the validated
// deploy branch and prepare PR data (spec: "push offered only after a
// successful validation"), AND (like keyAborted) a `d` key that offers to
// delete the current run's own branch inline (branch-cleanup spec: "Current
// Run's Temp Branch Deleted With Confirmation"). q/enter/esc still quit.
func (m Model) keySucceeded(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.cleanupPhase != cleanupIdle {
		// Mid a delete confirmation: every key routes through the shared
		// confirm/strongConfirm handler until it resolves or is backed out.
		return m.keyDeleteConfirm(msg)
	}
	switch msg.String() {
	case "p":
		m.pushErr = nil
		m.prErr = nil
		m.prURL = ""
		m.pushPhase = pushConfirm
		m.state = StatePushPreparation
		return m, nil
	case "d":
		// Gate the confirmation strength on the CURRENT run's real
		// unpushed-commit count (design.md "Unpushed gate: on d, fire
		// unpushedCountCmd(name)") — never assumed from currentPushed alone,
		// since a pushed branch can still have local commits ahead of its
		// remote ref.
		m.notice = ""
		return m, m.unpushedCountCmd(m.plan.PromotionBranch)
	case "q", "enter", "esc":
		return m, m.quitCmd()
	}
	return m, nil
}

// keyAborted handles the terminal ABORTED screen (HU-017 branch-cleanup):
// like keySucceeded it gains the inline current-branch delete offer (`d`)
// alongside quit, but — unlike Succeeded — it never offers HU-014 push (an
// aborted run's branch was never validated/pushed as a deliverable).
func (m Model) keyAborted(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.cleanupPhase != cleanupIdle {
		return m.keyDeleteConfirm(msg)
	}
	switch msg.String() {
	case "d":
		m.notice = ""
		return m, m.unpushedCountCmd(m.plan.PromotionBranch)
	case "q", "enter", "esc":
		return m, m.quitCmd()
	}
	return m, nil
}

// deleteConfirmWord is HU-017's exact, case-sensitive typed literal for the
// strong branch-delete confirmation (design.md "Strong-confirm reuse"): its
// OWN word and field (m.deleteConfirm) — deliberately never HU-012's
// CANCELAR/cancelInput — so a stray "cancel" buffer can never delete a
// branch, and vice versa.
const deleteConfirmWord = "BORRAR"

// keyDeleteConfirm handles HU-017's inline current-branch delete
// confirmation, reached from keySucceeded/keyAborted once unpushedCountCmd's
// result has gated m.cleanupPhase: cleanupConfirm needs only a single 'y'
// (mirroring keyPushPreparation's pushPRConfirm); cleanupStrongConfirm
// reuses keyCancelConfirm's exact typed-input idiom (backspace/KeyRunes
// buffer build + case-sensitive == gate) on the dedicated deleteConfirm
// field. n/esc back out to cleanupIdle without deleting anything.
func (m Model) keyDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.cleanupPhase {
	case cleanupConfirm:
		switch msg.String() {
		case "y":
			return m.confirmDeleteCurrent()
		case "n", "esc":
			m.cleanupPhase = cleanupIdle
			return m, nil
		case "q":
			return m, m.quitCmd()
		}
	case cleanupStrongConfirm:
		switch msg.String() {
		case "enter":
			if m.deleteConfirm != deleteConfirmWord {
				m.notice = "escribe BORRAR exactamente para confirmar el borrado"
				return m, nil
			}
			m.notice = ""
			return m.confirmDeleteCurrent()
		case "esc":
			m.deleteConfirm = ""
			m.notice = ""
			m.cleanupPhase = cleanupIdle
			return m, nil
		case "backspace":
			if n := len(m.deleteConfirm); n > 0 {
				m.deleteConfirm = m.deleteConfirm[:n-1]
			}
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.deleteConfirm += string(msg.Runes)
			}
			return m, nil
		}
	}
	return m, nil
}

// confirmDeleteCurrent lands an explicit delete confirmation (normal or
// strong): it marks the current run's branch for deletion and reuses
// quitCmd, which performs the delete SYNCHRONOUSLY right after its restore
// checkout (design.md: "delete happens INSIDE quitCmd after the
// restore-checkout, since you cannot delete the branch you are on").
func (m Model) confirmDeleteCurrent() (tea.Model, tea.Cmd) {
	m.pendingDeleteCurrent = true
	m.cleanupPhase = cleanupIdle
	m.deleteConfirm = ""
	return m, m.quitCmd()
}

// keyPushPreparation handles HU-014's push + PR-preparation sub-flow (mockup
// docs/MOCKUPS_TUI.md "Push Y PR"), routing by pushPhase. The two external
// side effects are each gated behind an explicit confirmation: `p` on
// pushConfirm runs `git push -u origin <branch>` (spec: "show the push command
// before running it"); on the authed screen `g` reveals the PR confirm gate
// and only an explicit `y` there fires `gh pr create` (spec invariant: "no PR
// is created without explicit confirmation"). The compare-fallback path
// (gh absent/unauthenticated) never offers PR creation. `q` always quits.
func (m Model) keyPushPreparation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.pushPhase {
	case pushConfirm:
		switch msg.String() {
		case "p":
			m.pushErr = nil
			m.pushPhase = pushPushing
			return m, m.pushCmd()
		case "q", "esc":
			return m, m.quitCmd()
		}
	case pushReady:
		switch msg.String() {
		case "g":
			// Reveal the explicit PR confirm gate — ONLY when gh is
			// authenticated and no PR was already created. On the
			// compare-fallback path (absent/unauthenticated) `g` is inert, so
			// gh pr create is unreachable there.
			if m.prURL == "" && m.authState == github.AuthAuthenticated {
				m.prErr = nil
				m.pushPhase = pushPRConfirm
			}
			return m, nil
		case "q", "esc":
			return m, m.quitCmd()
		}
	case pushPRConfirm:
		switch msg.String() {
		case "y":
			// The single, explicit confirmation that fires gh pr create.
			m.pushPhase = pushPRCreating
			return m, m.createPRCmd()
		case "n", "esc":
			m.pushPhase = pushReady
			return m, nil
		case "q":
			return m, m.quitCmd()
		}
	case pushPushing, pushPRCreating:
		// A side effect is in flight: ignore everything but quit so a second
		// push / PR can never be triggered while one is outstanding.
		if msg.String() == "q" {
			return m, m.quitCmd()
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
		return m, m.quitCmd()
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
		// Reset any re-promote state left over from an abandoned r flow
		// (HU-016 review remediation, Findings 2/3): this is the single
		// choke point every non-re-promote run passes through, so a stale
		// sourceRunID/rePromoteMissing must never bleed onto an unrelated
		// normal run.
		m.sourceRunID = ""
		m.rePromoteMissing = nil
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
		return m, m.quitCmd()
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
	case "enter":
		// Confirm gated by the reused cherry-pick gate (delta-generation spec:
		// "entry gated by post-pick verification"): a failed/aborted run never
		// enters delta generation.
		if !m.DeltaAllowed() {
			m.notice = "delta and validation blocked: the cherry-pick did not complete cleanly"
			return m, nil
		}
		m.notice = ""
		m.deltaErr = nil
		m.state = StateDeltaGeneration
		return m, m.deltaCmd()
	case "q":
		return m, m.quitCmd()
	case "e":
		// Edit selection: PickVerification -> CommitSelection (partial
		// promotion, per the state diagram).
		m.state = StateCommitSelection
		return m, nil
	}
	return m, nil
}

// keyDeltaGeneration handles the DeltaGeneration screen's keys. On success the
// state auto-advances (no key needed); these keys serve the sgd-failure screen:
// retry, edit the selection, or quit.
func (m Model) keyDeltaGeneration(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		if m.deltaErr != nil {
			m.deltaErr = nil
			return m, m.deltaCmd()
		}
		return m, nil
	case "e":
		m.state = StateCommitSelection
		return m, nil
	case "q":
		return m, m.quitCmd()
	}
	return m, nil
}

// keyPackageReview handles the HU-008 review screen: confirm proceeds to the
// HU-009 QueueReview stop, an empty package is blocked until an explicit `o`
// override, `e` edits the selection, `q` quits.
func (m Model) keyPackageReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.confirmPackageReview()
	case "o":
		// Explicit override for an empty package (delta-generation spec:
		// "validation only proceeds after the user explicitly confirms an
		// override").
		if m.summary.Empty {
			m.emptyConfirmed = true
			m.notice = ""
		}
		return m, nil
	case "e":
		m.state = StateCommitSelection
		return m, nil
	case "q":
		return m, m.quitCmd()
	}
	return m, nil
}

// confirmPackageReview gates on the empty-package override, then enters the
// HU-009 QueueReview stop and fires queueCmd (own-job identity + active
// DeployRequest queue).
func (m Model) confirmPackageReview() (tea.Model, tea.Cmd) {
	if m.summary.Empty && !m.emptyConfirmed {
		m.notice = "empty package blocks validation: press o to override, or e to edit the selection"
		return m, nil
	}
	m.notice = ""
	m.queueErr = nil
	m.state = StateQueueReview
	return m, m.queueCmd()
}

// keyQueueReview handles the HU-009 QueueReview screen (mockup
// docs/MOCKUPS_TUI.md "Cola De Deploys"): `enter` continues into validation,
// `r` re-fires the queue query, `esc` returns to PackageReview.
func (m Model) keyQueueReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.notice = ""
		m.validateErr = nil
		m.state = StateValidationStart
		return m, m.validateCmd()
	case "r":
		m.queueErr = nil
		return m, m.queueCmd()
	case "esc":
		m.state = StatePackageReview
		return m, nil
	}
	return m, nil
}

// keyValidationStart handles the ValidationStart screen. On success the state
// auto-advances to polling; these keys serve the CLI-error screen (retry the
// validate call) and quit.
func (m Model) keyValidationStart(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		if m.validateErr != nil {
			m.validateErr = nil
			return m, m.validateCmd()
		}
		return m, nil
	case "q":
		return m, m.quitCmd()
	}
	return m, nil
}

// keyValidationPolling handles the live-progress screen. `r` refreshes
// manually (an immediate extra poll) — but ONLY when no report is already in
// flight, so a manual refresh can never stack a second concurrent poll on top
// of the running loop. `q` exits WITHOUT issuing any cancel/abort — the
// Salesforce job stays active and the persisted run stays resumable
// (validation-progress spec: "user exit leaves the job active and resumable").
func (m Model) keyValidationPolling(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		if m.pollInFlight {
			// A poll is already outstanding — don't stack a second report.
			return m, nil
		}
		m.pollInFlight = true
		return m, m.reportCmd()
	case "c":
		// Enter the HU-012 typed-confirmation cancel flow. This is a SEPARATE,
		// explicit action from `q` (exit): `q` leaves the job active/resumable
		// and cancels nothing; `c` opens the CANCELAR confirmation before any
		// destructive `sf project deploy cancel` is issued (validation-progress
		// spec: distinct cancel entry key; the `q` invariant is unchanged). The
		// poll loop naturally quiesces while off ValidationPolling — onPollTick
		// and onReportDone both no-op for any non-polling state — so a late
		// report can't clobber the confirm/cancel path.
		m.cancelInput = ""
		m.cancelErr = nil
		m.notice = ""
		m.state = StateCancelConfirm
		return m, nil
	case "q":
		// Exit WITHOUT cancelling the Salesforce job: only tear down the local
		// read-only report subprocess via the session context.
		m.cancelPoll()
		return m, m.quitCmd()
	}
	return m, nil
}

// keyRunHistory handles HU-013's run-history browse + resume-offer screen
// (mockup docs/MOCKUPS_TUI.md "Historial De Runs", footer line 386): `↑/↓`
// (and k/j) move the selection, `d` toggles the expanded detail, `Enter`
// resumes the selected run when it is resumable (routing into
// StateCherryPickConflict or StateValidationPolling) and is a no-op on a
// terminal run (the detail stays shown), and `q`/`esc` declines the resume
// offer, proceeding to the normal ticket-input flow (run-resume spec: "User
// declines the resume offer").
func (m Model) keyRunHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.runsCursor > 0 {
			m.runsCursor--
		}
		return m, nil
	case "down", "j":
		if m.runsCursor < len(m.runs)-1 {
			m.runsCursor++
		}
		return m, nil
	case "d":
		m.runDetail = !m.runDetail
		return m, nil
	case "enter":
		if m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
			return m, nil
		}
		// resumeInto is a no-op for a terminal run, leaving the user on history.
		return m.resumeInto(m.runs[m.runsCursor])
	case "r":
		// HU-016: re-promote the selected row's ticket into the next
		// environment. Distinct from Enter's resume — never alters it (an
		// out-of-range cursor or an ineligible row's status is a strict
		// no-op, mirroring the Enter guard above).
		if m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
			return m, nil
		}
		rec := m.runs[m.runsCursor]
		if !isRePromoteEligible(rec) {
			return m, nil
		}
		return m.startRePromoteInto(rec)
	case "q", "esc":
		// Decline the offer: proceed to the normal flow rather than resuming.
		m.state = StateTicketInput
		return m, nil
	}
	return m, nil
}

// isRePromoteEligible reports whether rec's prior run status allows starting
// a re-promotion (re-promotion spec: "Re-Promote Eligibility Gate") — only a
// terminal-success run (Succeeded or SucceededPartial) offers reuse; every
// other status (Failed, Canceled, or any non-success terminal status) offers
// no re-promotion. A terminal-success run with ZERO Commits has nothing to
// remap (review remediation, Finding 4a), so it is treated as ineligible
// too — pressing r on such a row is a strict no-op, exactly like an
// ineligible-status row.
func isRePromoteEligible(rec runs.Record) bool {
	return (rec.Status == "Succeeded" || rec.Status == "SucceededPartial") && len(rec.Commits) > 0
}

// cancelConfirmWord is the exact, case-sensitive literal the user must type to
// confirm a destructive cancel (design.md "Cancel gate": no normalization —
// deliberate friction for a shared-queue action). Mockup docs/MOCKUPS_TUI.md
// "Confirmacion De Cancelacion".
const cancelConfirmWord = "CANCELAR"

// keyCancelConfirm handles HU-012's typed-confirmation cancel screen, reusing
// the keyTicket typed-input idiom to build m.cancelInput. `enter` fires the
// cancel ONLY when the typed text exactly equals CANCELAR (case-sensitive);
// any other text stays with a notice and never cancels. `backspace` edits the
// buffer; `esc` backs out to ValidationPolling (clearing the buffer) and
// re-arms the poll loop so live progress resumes.
func (m Model) keyCancelConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.cancelInput != cancelConfirmWord {
			m.notice = "escribe CANCELAR exactamente para confirmar la cancelacion"
			return m, nil
		}
		m.notice = ""
		return m, m.cancelCmd()
	case "esc":
		m.cancelInput = ""
		m.notice = ""
		m.state = StateValidationPolling
		// Resume the (idempotent, guarded) poll loop; onPollTick fires the next
		// report only when no poll is in flight and the deadline holds.
		return m, pollTickCmd(m.pollIntervalSeconds())
	case "backspace":
		if n := len(m.cancelInput); n > 0 {
			m.cancelInput = m.cancelInput[:n-1]
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.cancelInput += string(msg.Runes)
		}
		return m, nil
	}
}
