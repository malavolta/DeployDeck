package app

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// handleKey routes a key press to the current-state handler.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StatePrereqCheck:
		return m.keyPrereq(msg)
	case StateMainMenu:
		return m.keyMainMenu(msg)
	case StateDeltaSourceSelect:
		return m.keyDeltaSourceSelect(msg)
	case StatePackageSelect:
		return m.keyPackageSelect(msg)
	case StateSandboxSelect:
		return m.keySandboxSelect(msg)
	case StateTicketInput:
		return m.keyTicket(msg)
	case StateSourceConfirm:
		return m.keySourceConfirm(msg)
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
	case StateQuickDeploy:
		return m.keyQuickDeploy(msg)
	case StateSucceeded:
		return m.keySucceeded(msg)
	case StatePushPreparation:
		return m.keyPushPreparation(msg)
	case StateAborted:
		return m.keyAborted(msg)
	case StateBranchCleanup:
		return m.keyBranchCleanup(msg)
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
		// A run with no promotion branch (a standalone validate — HU-018) never
		// created a deliverable branch, so push is nonsensical: `git push -u
		// origin ""` is an invalid refspec (adversarial-review Finding 3). Only
		// offer push when a promotion branch actually exists.
		if m.plan.PromotionBranch == "" {
			return m, nil
		}
		m.pushErr = nil
		m.prErr = nil
		m.prURL = ""
		m.pushPhase = pushConfirm
		m.state = StatePushPreparation
		return m, nil
	case "d":
		// Same promotion-branch gate as `p` (Finding 3): a promotion-less
		// standalone validate has no branch to delete, so never fire the
		// inline delete gate on it.
		if m.plan.PromotionBranch == "" {
			return m, nil
		}
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

// keyBranchCleanup handles HU-017's StateBranchCleanup batch screen, routed
// by cleanupPhase. cleanupLoading is inert (the list-load result hasn't
// landed). cleanupBrowsing offers nav (↑/↓/k/j), `d` (per-row delete — fires
// unpushedCountCmd for the selected row, gating confirm/strongConfirm exactly
// like keySucceeded/keyAborted's inline delete: onUnpushedCount is REUSED
// unchanged, so the same count>0-requires-BORRAR rule applies here too), `p`
// (retention-prune, behind cleanupPruneConfirm), and `q`/`esc` (back to
// StateTicketInput). cleanupConfirm/cleanupStrongConfirm mirror
// keyDeleteConfirm's exact typed-BORRAR idiom (same deleteConfirmWord, same
// m.deleteConfirm buffer — safe to share since a Model is never on both a
// terminal screen and StateBranchCleanup at once) but confirm into
// confirmDeleteOrphan (the SELECTED ROW) rather than confirmDeleteCurrent
// (the run's own branch via quitCmd). cleanupPruneConfirm is a plain
// single-key y/n gate (deleting only policy-selected run.json directories,
// never a git branch — no BORRAR warranted).
func (m Model) keyBranchCleanup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.cleanupPhase {
	case cleanupLoading:
		return m, nil
	case cleanupCounting:
		// Review H-1: a per-row unpushed count is in flight for the CAPTURED
		// target (cleanupDeleteTarget). Ignore cursor-move and delete keys so
		// the confirm that lands can only ever bind to the captured row, never
		// one the cursor was moved to. esc cancels the pending delete.
		if msg.String() == "esc" {
			m.cleanupPhase = cleanupBrowsing
			m.cleanupDeleteTarget = ""
			m.cleanupDeleteTargetPushed = false
		}
		return m, nil
	case cleanupConfirm:
		switch msg.String() {
		case "y":
			return m.confirmDeleteOrphan()
		case "n", "esc":
			m.cleanupPhase = cleanupBrowsing
			m.cleanupDeleteTarget = ""
			m.cleanupDeleteTargetPushed = false
			return m, nil
		}
		return m, nil
	case cleanupStrongConfirm:
		switch msg.String() {
		case "enter":
			if m.deleteConfirm != deleteConfirmWord {
				m.cleanupNotice = "escribe BORRAR exactamente para confirmar el borrado"
				return m, nil
			}
			m.cleanupNotice = ""
			return m.confirmDeleteOrphan()
		case "esc":
			m.deleteConfirm = ""
			m.cleanupNotice = ""
			m.cleanupPhase = cleanupBrowsing
			m.cleanupDeleteTarget = ""
			m.cleanupDeleteTargetPushed = false
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
	case cleanupPruneConfirm:
		switch msg.String() {
		case "y":
			m.cleanupPhase = cleanupBrowsing
			return m, m.pruneRunsCmd()
		case "n", "esc":
			m.cleanupPhase = cleanupBrowsing
			return m, nil
		}
		return m, nil
	default: // cleanupBrowsing
		switch msg.String() {
		case "up", "k":
			if m.cleanupCursor > 0 {
				m.cleanupCursor--
			}
			return m, nil
		case "down", "j":
			if m.cleanupCursor < len(m.cleanupBranches)-1 {
				m.cleanupCursor++
			}
			return m, nil
		case "d":
			if m.cleanupCursor < 0 || m.cleanupCursor >= len(m.cleanupBranches) {
				return m, nil
			}
			// Review H-1: CAPTURE the target row the instant d is pressed, then
			// park in cleanupCounting so cursor-move keys can't shift the
			// selection out from under the in-flight count. confirmDeleteOrphan
			// deletes THIS captured branch, never the live cursor row.
			row := m.cleanupBranches[m.cleanupCursor]
			m.cleanupNotice = ""
			m.cleanupDeleteTarget = row.Name
			m.cleanupDeleteTargetPushed = row.Pushed
			m.cleanupPhase = cleanupCounting
			return m, m.unpushedCountCmd(row.Name)
		case "p":
			m.cleanupNotice = ""
			m.cleanupPhase = cleanupPruneConfirm
			return m, nil
		case "q", "esc":
			m.state = StateTicketInput
			m.cleanupPhase = cleanupIdle
			m.cleanupDeleteTarget = ""
			m.cleanupDeleteTargetPushed = false
			return m, nil
		}
	}
	return m, nil
}

// confirmDeleteOrphan lands an explicit per-row delete confirmation (normal
// or strong) on StateBranchCleanup: unlike confirmDeleteCurrent (which
// defers to quitCmd because it deletes the CHECKED-OUT branch), the selected
// row is never the current branch, so deleteOrphanCmd fires directly — no
// restore/checkout choreography needed.
//
// Review H-1: it deletes the branch CAPTURED when `d` was pressed
// (cleanupDeleteTarget / cleanupDeleteTargetPushed), NEVER the live
// m.cleanupBranches[m.cleanupCursor] — the cursor-move TOCTOU could otherwise
// point at a different (possibly unpushed) row than the one whose unpushed
// gate decided this confirm strength. An empty captured target (list
// reloaded/emptied out from under a stale confirm) is a safe no-op.
func (m Model) confirmDeleteOrphan() (tea.Model, tea.Cmd) {
	m.deleteConfirm = ""
	target, pushed := m.cleanupDeleteTarget, m.cleanupDeleteTargetPushed
	m.cleanupPhase = cleanupBrowsing
	m.cleanupDeleteTarget = ""
	m.cleanupDeleteTargetPushed = false
	if target == "" {
		return m, nil
	}
	return m, m.deleteOrphanCmd(target, pushed)
}

// keyPushPreparation handles HU-014's push + PR-preparation sub-flow (mockup
// docs/MOCKUPS_TUI.md "Push Y PR"), routing by pushPhase. The two external
// side effects are each gated behind an explicit confirmation: `p` on
// pushConfirm runs `git push -u origin <branch>` (spec: "show the push command
// before running it"); on the authed screen `g` reveals the PR confirm gate
// and an explicit key there fires `gh pr create` (spec invariant: "no PR is
// created without explicit confirmation"). On pushPRConfirm, an AI suggestion
// that is ready but not yet accepted turns the confirm into an explicit
// choice (never-silently-skip fix): `y` accepts-then-creates with the AI
// title/description, `d` creates with the default formula title. A
// still-generating suggestion blocks creation entirely until it lands or the
// user backs out. The compare-fallback path (gh absent/unauthenticated) never
// offers PR creation. `q` always quits.
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
		case "a":
			// Context-sensitive, two-press AI-suggestion affordance
			// (ai-pr-summary design ADR-7): pending or already-accepted is a
			// strict no-op (guards a second concurrent request and a
			// re-request after accept); no suggestion yet + a configured dep
			// requests one; an unaccepted, already-generated suggestion is
			// ACCEPTED by this second press — the only place aiAccepted is
			// ever set. A nil dep (AI not configured) leaves 'a' inert
			// entirely (spec: "No ai config leaves pushReady unchanged").
			switch {
			case m.aiPending || m.aiAccepted:
				return m, nil
			case m.aiTitle != "":
				m.aiAccepted = true
				return m, nil
			case m.deps.GenerateSummary != nil:
				m.aiPending = true
				return m, m.aiSuggestCmd()
			}
			return m, nil
		case "q", "esc":
			return m, m.quitCmd()
		}
	case pushPRConfirm:
		// Never-silently-skip fix: branch on the AI suggestion's state so PR
		// creation always makes an explicit choice when a suggestion could
		// still apply, instead of defaulting to whichever title
		// effectiveTitle() happens to resolve to.
		switch {
		case m.aiPending:
			// A suggestion is still generating: never create yet — this
			// closes the timing hole where a fast user creates before the
			// suggestion lands. Only back-out and quit work; any create key
			// is inert. If the request fails/times out, aiPending clears and
			// aiTitle stays "", so the next press falls into the
			// no-suggestion case below.
			switch msg.String() {
			case "n", "N", "esc":
				m.pushPhase = pushReady
				return m, nil
			case "q":
				return m, m.quitCmd()
			}
			return m, nil
		case m.aiTitle != "" && !m.aiAccepted:
			// A suggestion is ready but was never explicitly accepted via
			// pushReady's `a`: ask explicitly instead of silently creating
			// with the default formula title.
			switch msg.String() {
			case "y", "Y":
				// Accept the AI suggestion, THEN create — createPRCmd reads
				// effectiveTitle()/effectiveDescription() at call time, so
				// aiAccepted must be set before it's returned.
				m.aiAccepted = true
				m.pushPhase = pushPRCreating
				return m, m.createPRCmd()
			case "d", "D":
				// Explicitly decline the suggestion: create with the default
				// formula title and empty body (aiAccepted stays false).
				m.pushPhase = pushPRCreating
				return m, m.createPRCmd()
			case "n", "N", "esc":
				m.pushPhase = pushReady
				return m, nil
			case "q":
				return m, m.quitCmd()
			}
			return m, nil
		default:
			// No suggestion available, or already accepted via `a` on
			// pushReady: unchanged single explicit confirm.
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

// keyMainMenu handles HU-018's StateMainMenu (design ADR-1): ↑/↓ (and k/j)
// move the cursor over the FILTERED visibleMenuEntries slice (clamped so an
// unimplemented/off-list entry is never selectable), Enter routes the selected
// entry to its target State and stamps standaloneMode ("" for the unchanged
// full flow, "delta"/"validate" for the two standalone modes — the single
// behavioral discriminator, ADR-3). q/esc handling is added in task 2.6.
func (m Model) keyMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := visibleMenuEntries(menuEntries)
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
		return m, nil
	case "down", "j":
		if m.menuCursor < len(entries)-1 {
			m.menuCursor++
		}
		return m, nil
	case "enter":
		if m.menuCursor < 0 || m.menuCursor >= len(entries) {
			return m, nil
		}
		entry := entries[m.menuCursor]
		// standaloneMode is derived from the selected target: the full-flow
		// entry leaves it "" (ADR-3: "" preserves the full flow verbatim), the
		// two standalone entries stamp their mode so the shared review/command
		// path forks correctly.
		switch entry.target {
		case StateDeltaSourceSelect:
			// D1 guard (standalone-modes spec: "Standalone Entry Blocked
			// While A Git Operation Is In Progress"): block BEFORE
			// standaloneMode/state are set or any command fires. Fail-open
			// when m.repoState is still zero (not yet populated by
			// onResumeDetect, design's D1 decision) — InProgress==false
			// falls through unchanged.
			if m.repoState.InProgress {
				m.notice = "resolve the in-progress cherry-pick before starting a standalone delta"
				return m, nil
			}
			// Group 3: the delta base-branch picker needs a live branch list
			// to navigate, so dispatch fires standaloneBranchesCmd
			// immediately (mirrors StateBranchCleanup's load-on-entry).
			m.standaloneMode = "delta"
			m.state = entry.target
			return m, m.standaloneBranchesCmd()
		case StatePackageSelect:
			// D1 guard, same rationale as the delta case above.
			if m.repoState.InProgress {
				m.notice = "resolve the in-progress cherry-pick before starting a standalone validation"
				return m, nil
			}
			m.standaloneMode = "validate"
		default:
			m.standaloneMode = ""
			// Ticket-from-branch auto-suggest: only the full-flow entry
			// (target == StateTicketInput) lands here. Seed the ticket
			// buffer from the current branch (captured async at the
			// StateMainMenu landing via originalBranchCmd) ONLY when it is
			// still empty — a pre-existing ticket (e.g. from a prior visit)
			// is never overwritten. A non-matching or not-yet-captured
			// originalBranch ("" — the graceful timing path) makes
			// git.TicketFromBranch return "", so nothing is seeded.
			if m.ticket == "" {
				if seed := git.TicketFromBranch(m.originalBranch, m.deps.Config.TicketPatterns); seed != "" {
					m.ticket = seed
					m.ticketFromBranch = true
				}
			}
		}
		m.state = entry.target
		return m, nil
	case "q", "esc":
		// The menu is the top-level landing: there is no back target, so both
		// keys quit (reusing quitCmd's restore/delete choreography like every
		// other quit site).
		return m, m.quitCmd()
	}
	return m, nil
}

// keyDeltaSourceSelect handles HU-018's standalone-delta base-branch picker
// (StateDeltaSourceSelect, Group 3): ↑/↓ (and k/j) navigate m.branchList
// (loaded by standaloneBranchesCmd from the menu dispatch); the current ref
// is always HEAD (deltaCmd hardcodes "to" — display-only, never a picker).
// Enter normalizes the picked branch and confirms; q/esc return to
// StateMainMenu.
func (m Model) keyDeltaSourceSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.branchCursor > 0 {
			m.branchCursor--
		}
		return m, nil
	case "down", "j":
		if m.branchCursor < len(m.branchList)-1 {
			m.branchCursor++
		}
		return m, nil
	case "enter":
		return m.confirmDeltaSourceSelect()
	case "q", "esc":
		m.state = StateMainMenu
		return m, nil
	}
	return m, nil
}

// sanitizeBranch folds a base branch name into a string safe to embed as an
// output-dir/identity segment (design D2): "/" becomes "-" (e.g.
// "release/1.0" -> "release-1.0"). A branch with no "/" is a no-op.
func sanitizeBranch(base string) string {
	return strings.ReplaceAll(base, "/", "-")
}

// confirmDeltaSourceSelect seeds the MINIMAL DeploymentPlan deltaCmd reads
// (design D2: Ticket = "standalone-" + sanitizeBranch(base) — a
// self-describing, per-base identity, replacing the flat literal
// "standalone" that collided every base into the same output dir — and
// TargetBranch as the picked base with any "origin/" prefix stripped so
// deltaCmd's "origin/"+target resolves correctly), then enters
// StateDeltaGeneration and fires deltaCmd UNCHANGED — the exact same command
// the full flow uses, mirroring keyVerification's confirm.
func (m Model) confirmDeltaSourceSelect() (tea.Model, tea.Cmd) {
	if m.branchCursor < 0 || m.branchCursor >= len(m.branchList) {
		return m, nil
	}
	base := strings.TrimPrefix(m.branchList[m.branchCursor].Name, "origin/")
	// Build a FRESH minimal plan (adversarial-review Finding 1): no prior
	// full-flow SelectedCommits / PackageXMLPath / DestructiveChangesPath /
	// PromotionBranch may bleed into this standalone delta run. Replacing the
	// whole plan (never a field-by-field overwrite) guarantees no stale field
	// survives.
	m.plan = git.DeploymentPlan{
		Ticket:       "standalone-" + sanitizeBranch(base),
		TargetBranch: base,
	}
	m.notice = ""
	m.deltaErr = nil
	m.state = StateDeltaGeneration
	m, spin := m.startSpinner()
	return m, tea.Batch(m.deltaCmd(), spin)
}

// keyPackageSelect handles HU-018's standalone-validation package-path input
// (StatePackageSelect, Group 4): typed-text entry into m.packagePath,
// reusing the keyTicket idiom (backspace/KeyRunes buffer build). `q` is
// guarded exactly like keyTicket's `b` shortcut — only treated as "back" on
// an EMPTY buffer — so a path that happens to contain the letter q (e.g.
// "qa/package.xml") types correctly instead of being swallowed as a quit;
// `esc` always backs out unconditionally to StateMainMenu. `enter` runs the
// pre-check (confirmPackageSelect) before ever considering a launch.
func (m Model) keyPackageSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.packagePath == "" {
			m.notice = "ingresa la ruta al package.xml"
			return m, nil
		}
		return m.confirmPackageSelect()
	case "q":
		if m.packagePath != "" {
			m.packagePath += "q"
			return m, nil
		}
		m.notice = ""
		m.state = StateMainMenu
		return m, nil
	case "esc":
		m.notice = ""
		m.state = StateMainMenu
		return m, nil
	case "backspace":
		if n := len(m.packagePath); n > 0 {
			m.packagePath = m.packagePath[:n-1]
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.packagePath += string(msg.Runes)
		}
		return m, nil
	}
}

// confirmPackageSelect is HU-018's validate path pre-check (design
// "Validate path pre-check: parsePackageFile(path,false); on error set
// actionable m.notice, STAY on StatePackageSelect, do NOT fire
// validateCmd"): parsePackageFile is the SAME reader deltaCmd's summary step
// uses, reused UNCHANGED, so a nonexistent or malformed package.xml is
// caught HERE — before validateCmd/ValidateDeploy is ever reachable. On
// success it advances to StateSandboxSelect with the picker built
// SYNCHRONOUSLY from cfg.Sandboxes (design ADR-2: "no exec" — unlike the
// delta base-branch picker, this needs no async command/message).
func (m Model) confirmPackageSelect() (tea.Model, tea.Cmd) {
	if _, err := parsePackageFile(m.packagePath, false); err != nil {
		m.notice = "no se pudo leer el package.xml: " + err.Error()
		return m, nil
	}
	m.notice = ""
	m.sandboxList = standaloneSandboxAliases(m.deps.Config)
	m.sandboxCursor = 0
	m.state = StateSandboxSelect
	return m, nil
}

// standaloneSandboxAliases returns the deduplicated, sorted set of configured
// sandbox aliases (HU-018 Group 4's StateSandboxSelect picker list), built
// directly from cfg.Sandboxes — the config is already synchronously
// available (Deps.Config), so unlike the delta base-branch picker this needs
// no async command/message (design ADR-2 "no exec"). Sorting keeps the
// picker's order deterministic across the map's inherently unstable
// iteration.
func standaloneSandboxAliases(cfg config.Config) []string {
	seen := map[string]bool{}
	var aliases []string
	for _, sb := range cfg.Sandboxes {
		if sb.Alias == "" || seen[sb.Alias] {
			continue
		}
		seen[sb.Alias] = true
		aliases = append(aliases, sb.Alias)
	}
	sort.Strings(aliases)
	return aliases
}

// standaloneSandboxTestLevel resolves the configured TestLevel for alias
// (design ADR-2's minimal-plan table: "TestLevel=<SandboxConfig.TestLevel>")
// by scanning cfg.Sandboxes for the first entry whose Alias matches. Empty
// when no entry matches — a safe zero-value default (validateCmd passes it
// straight through to the sf CLI's own --test-level flag).
func standaloneSandboxTestLevel(cfg config.Config, alias string) string {
	for _, sb := range cfg.Sandboxes {
		if sb.Alias == alias {
			return sb.TestLevel
		}
	}
	return ""
}

// keySandboxSelect handles HU-018's standalone-validation sandbox picker
// (StateSandboxSelect, Group 4): ↑/↓ (and k/j) navigate m.sandboxList
// (loaded synchronously by confirmPackageSelect); Enter confirms the
// selected alias; q/esc return to StatePackageSelect (the previous step —
// unlike StateDeltaSourceSelect/StateMainMenu, which back out to the top-level
// menu, this screen backs out only one step).
func (m Model) keySandboxSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.sandboxCursor > 0 {
			m.sandboxCursor--
		}
		return m, nil
	case "down", "j":
		if m.sandboxCursor < len(m.sandboxList)-1 {
			m.sandboxCursor++
		}
		return m, nil
	case "enter":
		return m.confirmSandboxSelect()
	case "q", "esc":
		m.notice = ""
		m.state = StatePackageSelect
		return m, nil
	}
	return m, nil
}

// confirmSandboxSelect seeds the MINIMAL DeploymentPlan validateCmd's REUSE
// branch reads (design ADR-2: PackageXMLPath/SandboxAlias/TestLevel — never
// Ticket/TargetBranch, which only the FALLBACK branch consumes), then
// pre-creates a local run tagged Mode="validate" BEFORE firing validateCmd
// (design ADR-4: "on sandbox confirm, BEFORE firing validateCmd") — mirroring
// onBranchCreated's best-effort, nil-Runs-safe save — and sets m.runID so
// validateCmd's EXISTING runID!="" reuse branch (commands.go, UNCHANGED)
// Loads this very record and merges the jobId onto it instead of creating a
// second run. Finally it enters StateValidationStart and fires validateCmd
// UNCHANGED — the exact same command the full flow uses, mirroring
// confirmDeltaSourceSelect's confirm-then-fire shape.
func (m Model) confirmSandboxSelect() (tea.Model, tea.Cmd) {
	if m.sandboxCursor < 0 || m.sandboxCursor >= len(m.sandboxList) {
		return m, nil
	}
	alias := m.sandboxList[m.sandboxCursor]
	// Build a FRESH minimal plan (adversarial-review Finding 1): a prior
	// full-flow promotion's DestructiveChangesPath (and every other field) must
	// NOT bleed into this standalone validation — validateCmd reads
	// plan.DestructiveChangesPath as PostDestructivePath, so a leftover value
	// would silently include an earlier promotion's destructive deletions.
	// Replacing the whole plan (never a field-by-field overwrite) guarantees no
	// stale field survives.
	m.plan = git.DeploymentPlan{
		PackageXMLPath: m.packagePath,
		SandboxAlias:   alias,
		TestLevel:      standaloneSandboxTestLevel(m.deps.Config, alias),
	}
	m.notice = ""
	m.validateErr = nil

	if m.deps.Runs != nil {
		now := m.now()
		m.runID = "validate-" + alias + "-" + now.Format("20060102150405")
		_ = m.deps.Runs.Save(runs.Record{
			RunID:        m.runID,
			Mode:         "validate",
			ManifestPath: m.packagePath,
			Alias:        alias,
			TestLevel:    m.plan.TestLevel,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	m.state = StateValidationStart
	m, spin := m.startSpinner()
	return m, tea.Batch(m.validateCmd(), spin)
}

func (m Model) keyPrereq(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		// Retry the prereq check.
		return m, m.runPrereqCmd()
	case "c":
		// Continue with warnings — only allowed when nothing is blocking. HU-018
		// (design ADR-1): this `c`-continue is the 4th post-prereq landing site
		// and must land on the main menu, symmetric with onPrereqDone.
		if prereqHasBlocking(m.checks) {
			m.notice = "cannot continue: resolve blocking prerequisites first"
			return m, nil
		}
		m.notice = ""
		m.state = StateMainMenu
		// Symmetric with onPrereqDone (adversarial-review Finding 4): this
		// `c`-continue is the 4th post-prereq landing site, so it must fire the
		// SAME startup batch — HU-013 resume-detection and HU-017's original-
		// branch capture — that onPrereqDone's clean landing does. Both cmds
		// degrade to nil without their deps and tea.Batch drops nils cleanly, so
		// this stays a plain advance for callers without those deps.
		return m, tea.Batch(m.resumeDetectCmd(), m.originalBranchCmd())
	case "q":
		return m, m.quitCmd()
	}
	return m, nil
}

func (m Model) keyTicket(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b":
		// HU-017: enter the branch-cleanup batch screen (branch-cleanup spec:
		// "Orphan Deploy Branches Listed For Batch Cleanup") — but ONLY on an
		// EMPTY ticket buffer. tea.KeyMsg.String() for a typed rune 'b'
		// equals "b" too, so without this guard the shortcut would swallow
		// every literal 'b' typed into a ticket ID (e.g. "WEB-1"),
		// clobbering normal ticket entry. Once ANY text has been typed, 'b'
		// falls through to the default branch below like every other rune.
		if m.ticket != "" {
			m.ticket += "b"
			return m, nil
		}
		m.cleanupPhase = cleanupLoading
		m.cleanupNotice = ""
		m.cleanupBranches = nil
		m.cleanupCursor = 0
		m.state = StateBranchCleanup
		return m, m.listDeployBranchesCmd()
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
		m, spin := m.startSpinner()
		return m, tea.Batch(m.discoverCmd(), spin)
	case "q":
		// Guarded-q (design D3, mirrors keyPackageSelect:638): a non-empty
		// buffer keeps "q" typeable (a ticket like "Q-123" must never be
		// swallowed as a shortcut); an EMPTY buffer backs out exactly like
		// "esc" instead of silently appending "q" into nothing (bug fix).
		if m.ticket != "" {
			m.ticket += "q"
			m.ticketFromBranch = false
			return m, nil
		}
		m.notice = ""
		m.state = StatePrereqCheck
		return m, nil
	case "esc":
		m.notice = ""
		m.state = StatePrereqCheck
		return m, nil
	case "backspace":
		if n := len(m.ticket); n > 0 {
			m.ticket = m.ticket[:n-1]
		}
		// Editing an auto-seeded buffer makes it no longer a pristine
		// suggestion: clear the flag so viewTicket's hint stops rendering.
		m.ticketFromBranch = false
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.ticket += string(msg.Runes)
		}
		// Same rationale as backspace above.
		m.ticketFromBranch = false
		return m, nil
	}
}

// keySourceConfirm handles StateSourceConfirm: `s`/`S` fires confirmSourceCmd
// (bug fix: the confirm key must be case-insensitive, matching the decline
// key's existing n/N handling); `n`/`N`/`enter` (default-No) reuse
// onDiscoverDone with the unchanged base result to degrade straight to
// StateCommitSelection; `esc` backs out.
func (m Model) keySourceConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s", "S":
		m.state = StateCommitDiscovery
		return m, m.confirmSourceCmd()
	case "n", "N", "enter":
		return m.onDiscoverDone(discoverDoneMsg{result: m.discovery})
	case "esc":
		m.notice = ""
		m.state = StateTicketInput
		return m, nil
	}
	return m, nil
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
		m.notice = ""
		m.state = StateTicketInput
		return m, nil
	}
	return m, nil
}

func (m Model) confirmSelection() (tea.Model, tea.Cmd) {
	plan, err := git.GenerateDeploymentPlan(m.ticket, m.items)
	if err != nil {
		m.notice = "selecciona al menos un commit para continuar"
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
		m.notice = ""
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
		m, spin := m.startSpinner()
		return m, tea.Batch(m.branchCreateCmd(), spin)
	case "esc":
		m.notice = ""
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
		m, spin := m.startSpinner()
		return m, tea.Batch(m.deltaCmd(), spin)
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
// HU-009 QueueReview stop, an empty package is blocked until an explicit
// `o`/`O` override, `e`/`E` edits the selection, `q` quits. Bug fix: the
// override and edit action keys accept both cases (same case-insensitivity
// fix as keySourceConfirm's s/S), while typed-buffer and navigation keys
// elsewhere are left untouched. HU-018's standalone delta (design ADR-3)
// forks `enter`/`e` into terminal no-ops: standaloneMode=="" — the unchanged
// full flow — is UNTOUCHED by either branch below.
func (m Model) keyPackageReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.standaloneMode == "delta" {
			// Standalone delta never ran cherry-picks: there is nothing to
			// queue or validate, so the summary IS the terminal screen.
			return m, nil
		}
		return m.confirmPackageReview()
	case "o", "O":
		// Explicit override for an empty package (delta-generation spec:
		// "validation only proceeds after the user explicitly confirms an
		// override").
		if m.summary.Empty {
			m.emptyConfirmed = true
			m.notice = ""
		}
		return m, nil
	case "e", "E":
		if m.standaloneMode == "delta" {
			// Neutralized: standalone delta never ran commit selection, so
			// there is no selection to edit.
			return m, nil
		}
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
		m, spin := m.startSpinner()
		return m, tea.Batch(m.validateCmd(), spin)
	case "r":
		m.queueErr = nil
		return m, m.queueCmd()
	case "esc":
		m.notice = ""
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
	case "x":
		// HU-015: open the quick-deploy view for the selected row, gated on
		// runs.QuickDeployEligible — an out-of-range cursor or an ineligible
		// row (any predicate failing) is a strict no-op, mirroring the r gate
		// above. Never alters Enter/d/r (run-history spec: "Enter, d, and r
		// remain unaffected").
		if m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
			return m, nil
		}
		rec := m.runs[m.runsCursor]
		eligible, _ := runs.QuickDeployEligible(rec, m.now())
		if !eligible {
			return m, nil
		}
		m.state = StateQuickDeploy
		m.quickConfirm = ""
		m.quickErr = nil
		return m, nil
	case "q", "esc":
		// Decline the offer: return to the HU-018 main menu (design ADR-1
		// refinement) rather than StateTicketInput, so the hub stays reachable
		// after a declined resume.
		m.state = StateMainMenu
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

// quickDeployExecAllowed is HU-015's pure execution gate (quick-deploy spec:
// "Production Target Blocked Without Explicit Configuration" + "Suggest-Only
// By Default"): execution is permitted only when AllowExecution is true AND
// the target is EITHER non-production OR AllowProduction is explicitly true.
// isProd is computed by the caller via git.IsProductionTarget — this
// function holds no target/branch knowledge of its own, keeping it a
// trivially testable pure predicate.
func quickDeployExecAllowed(cfg config.Config, isProd bool) bool {
	return cfg.QuickDeploy.AllowExecution && (!isProd || cfg.QuickDeploy.AllowProduction)
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
			m.notice = "escribe CANCELAR exactamente para confirmar la cancelación"
			return m, nil
		}
		m.notice = ""
		return m, m.cancelCmd()
	case "q":
		// Guarded-q (design D3): a non-empty buffer keeps "q" typeable (the
		// CANCELAR literal contains no "q", but the guard is uniform across
		// every free-text screen); an EMPTY buffer mirrors "esc" exactly
		// instead of silently appending "q" into nothing (bug fix).
		if m.cancelInput != "" {
			m.cancelInput += "q"
			return m, nil
		}
		m.cancelInput = ""
		m.notice = ""
		m.state = StateValidationPolling
		return m, pollTickCmd(m.pollIntervalSeconds())
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

// quickDeployConfirmWord is HU-015's exact, case-sensitive literal the user
// must type to authorize a quick deploy — its OWN word on its OWN dedicated
// m.quickConfirm field (ADR-2, deliberately never HU-012's
// cancelInput/CANCELAR), so a stray cancel buffer can never authorize a
// quick deploy, and vice versa.
const quickDeployConfirmWord = "DESPLEGAR"

// keyQuickDeploy handles HU-015's StateQuickDeploy screen, reusing the
// keyCancelConfirm typed-input idiom (backspace/KeyRunes buffer build) for
// m.quickConfirm. `enter` fires quickDeployCmd ONLY when ALL FOUR gates pass:
// no deploy already in flight, the selected run is still
// runs.QuickDeployEligible (adversarial-review follow-up: refuses a
// deliberate re-type on an already-quick-deployed/no-longer-eligible run),
// quickDeployExecAllowed(cfg, isProd) (AllowExecution AND non-prod-or-
// AllowProduction), AND the typed text exactly equals quickDeployConfirmWord
// — any gate failing surfaces a notice and stays put, never executing
// (quick-deploy spec: "Production target blocked", "Missing confirmation
// blocks execution", "Suggest-only by default"). `q`/`esc` back out to
// StateRunHistory, clearing the confirm buffer and any surfaced error.
func (m Model) keyQuickDeploy(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.quickDeployingRunID != "" {
			// A quick deploy is already in flight (Finding H-1): ignore Enter so
			// the same destructive `sf project deploy quick` can never fire twice,
			// even if the confirm buffer is re-typed. Mirrors pushPushing/
			// pushPRCreating's "side effect outstanding" guard.
			return m, nil
		}
		if m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
			return m, nil
		}
		rec := m.runs[m.runsCursor]
		// Adversarial-review follow-up hardening: re-check the SELECTED run's
		// eligibility here too, not only on entry via the x key. Without this,
		// a deliberate re-type of DESPLEGAR + Enter on the same
		// StateQuickDeploy screen could still fire a SECOND real `sf project
		// deploy quick` on a run the H-2 fix already marked
		// QuickDeployedAt in-memory this session (or one that otherwise
		// stopped being eligible while the user sat on this screen). This
		// closes the deliberate-re-type vector as defense in depth alongside
		// the in-flight guard above and keyRunHistory's own x-entry gate.
		if eligible, _ := runs.QuickDeployEligible(rec, m.now()); !eligible {
			m.notice = "este run ya no es apto para quick deploy: vuelve al historial para revisar su estado"
			return m, nil
		}
		isProd := git.IsProductionTarget(m.deps.Config, rec.Target)
		if !quickDeployExecAllowed(m.deps.Config, isProd) {
			m.notice = "quick deploy no autorizado: revisa la configuración allowExecution/allowProduction"
			return m, nil
		}
		if m.quickConfirm != quickDeployConfirmWord {
			m.notice = "escribe DESPLEGAR exactamente para confirmar el quick deploy"
			return m, nil
		}
		m.notice = ""
		// Finding H-1: clear the confirm buffer on firing so a subsequent stray
		// Enter fails the DESPLEGAR gate (closing the after-success/duplicate
		// re-fire), and capture the firing run's RunID so onQuickDeployDone can
		// register it regardless of the current screen (Finding M-1) and mark it
		// in-memory ineligible (Finding H-2). Bubble Tea serializes key msgs, so
		// this closes both the concurrent and after-success re-fire windows.
		m.quickConfirm = ""
		m.quickDeployingRunID = rec.RunID
		return m, m.quickDeployCmd()
	case "q":
		// Guarded-q (design D3): a non-empty buffer keeps "q" typeable — the
		// DESPLEGAR literal contains no "q", but a stray "q" mid-typing must
		// never silently discard the whole confirmation (bug fix: this case
		// was previously folded into the unconditional "q", "esc" branch
		// below, which backed out even mid-DESPLEGAR). An EMPTY buffer still
		// backs out to StateRunHistory exactly like "esc".
		if m.quickConfirm != "" {
			m.quickConfirm += "q"
			return m, nil
		}
		m.quickConfirm = ""
		m.quickErr = nil
		m.notice = ""
		m.state = StateRunHistory
		return m, nil
	case "esc":
		m.quickConfirm = ""
		m.quickErr = nil
		m.notice = ""
		m.state = StateRunHistory
		return m, nil
	case "backspace":
		if n := len(m.quickConfirm); n > 0 {
			m.quickConfirm = m.quickConfirm[:n-1]
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.quickConfirm += string(msg.Runes)
		}
		return m, nil
	}
}
