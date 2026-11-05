package app

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"

	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

func kb(ks ...string) key.Binding { return key.NewBinding(key.WithKeys(ks...)) }

// keyMap is the one key vocabulary: handlers match against it and footers are
// built from per-state copies of it (see hint). Some bindings serve several
// meanings by label (Retry is also refresh/reuse/re-promote, Delete is also
// detail/recreate, Push is also prune, Continue is also cancel-job).
type keyMap struct {
	Up, Down, Nav, Enter, Esc, Quit, Back, Exit, Backspace      key.Binding
	Yes, YesAny, No, NoAny, DeclineAny, Decline, ConfirmSrc     key.Binding
	Retry, Continue, Skip, Abort, Edit, EditAny, Delete, Push   key.Binding
	PR, AI, Update, Quick, Mark, Override, Interrupt, CancelEsc key.Binding
	Cleanup                                                     key.Binding
}

var keys = keyMap{
	Up: kb("up", "k"), Down: kb("down", "j"), Nav: kb("up", "down", "k", "j"),
	Enter: kb("enter"), Esc: kb("esc"), Quit: kb("q"), Back: kb("q", "esc"),
	Exit: kb("enter", "q", "esc"), Backspace: kb("backspace"),
	Yes: kb("y"), YesAny: kb("y", "Y"), No: kb("n", "esc", "q"), NoAny: kb("n", "N", "esc", "q"),
	DeclineAny: kb("d", "D"), Decline: kb("n", "N", "enter"), ConfirmSrc: kb("s", "S"),
	Retry: kb("r"), Continue: kb("c"), Skip: kb("s"), Abort: kb("a"), Edit: kb("e"), EditAny: kb("e", "E"),
	Delete: kb("d"), Push: kb("p"), PR: kb("g"), AI: kb("a"), Update: kb("u"), Quick: kb("x"),
	Mark: kb(" ", "space", "x"), Override: kb("o", "O"), Interrupt: kb("ctrl+c"), CancelEsc: kb("c", "esc"),
	Cleanup: kb("b"),
}

// hint returns a copy of b with footer help and enablement, leaving the shared
// vocabulary binding untouched.
func hint(b key.Binding, k, desc string, on bool) key.Binding {
	b.SetHelp(k, desc)
	b.SetEnabled(on)
	return b
}

func on(b key.Binding, k, desc string) key.Binding { return hint(b, k, desc, true) }

// helpView renders short help with zero styles so footers keep their bytes.
var helpView = help.Model{ShortSeparator: "   "}

// footerOf renders the enabled, help-bearing bindings as the `[ k desc   k desc ]`
// footer line, or "" when there is none.
func footerOf(bs ...key.Binding) string {
	var shown []key.Binding
	for _, b := range bs {
		if b.Help().Key != "" {
			shown = append(shown, b)
		}
	}
	if s := helpView.ShortHelpView(shown); s != "" {
		return footer(s)
	}
	return ""
}

func (m Model) footerLine() string { return footerOf(m.bindings()...) }

func (m Model) hasBranch() bool { return m.plan.PromotionBranch != "" }

func (m Model) aiAvailable() bool {
	return m.deps.GenerateSummary != nil && !m.aiPending && !m.aiAccepted
}

func (m Model) selectedRun() (runs.Record, bool) {
	if m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
		return runs.Record{}, false
	}
	return m.runs[m.runsCursor], true
}

func bEsc(d string) key.Binding  { return on(keys.Esc, "Esc", d) }
func bQuit() key.Binding         { return on(keys.Quit, "q", "salir") }
func bNav(d string) key.Binding  { return on(keys.Nav, "↑/↓", d) }
func bBack(d string) key.Binding { return on(keys.Back, "q/Esc", d) }
func bEnter(d string) key.Binding {
	return on(keys.Enter, "Enter", d)
}

// bindings is the footer/keymap of the current screen; it mirrors viewBody.
func (m Model) bindings() []key.Binding {
	picker := []key.Binding{bEnter("seleccionar"), bNav("navegar"), bEsc("volver"), bQuit()}
	exit := on(keys.Exit, "Enter/q/Esc", "salir")
	switch m.state {
	case StatePrereqCheck:
		return []key.Binding{on(keys.Retry, "r", "reintentar"), on(keys.Continue, "c", "continuar con warnings"), bBack("salir")}
	case StateMainMenu:
		return []key.Binding{bEnter("seleccionar"), bNav("navegar"),
			hint(keys.Update, "u", "actualizar", m.updateAvailable && m.deps.CanUpgrade), bBack("salir")}
	case StateDeltaSourceSelect, StateSandboxSelect, StateTargetSelection:
		return picker
	case StatePackageSelect:
		return []key.Binding{bEnter("continuar"), bBack("volver")}
	case StateTicketInput:
		return []key.Binding{bEnter("buscar"), bEsc("volver")}
	case StateSourceConfirm:
		return []key.Binding{on(keys.ConfirmSrc, "s", "confirmar"), on(keys.Decline, "n/N/Enter", "declinar"), bEsc("volver"), bQuit()}
	case StateCommitDiscovery, StateBranchCreation, StateCherryPicking:
		return []key.Binding{on(keys.Interrupt, "ctrl+c", "interrumpe")}
	case StateCommitSelection:
		rows := len(m.items) > 0
		return []key.Binding{hint(keys.Nav, "↑/↓", "navegar", rows), hint(keys.Mark, "Space/x", "marcar", rows),
			hint(keys.Enter, "Enter", "continuar", rows), bEsc("volver"), bQuit()}
	case StatePlanPreview:
		return []key.Binding{bEnter("ejecutar"), bEsc("volver"), bQuit()}
	case StateCherryPickConflict:
		return []key.Binding{bNav("elegir"), on(keys.Edit, "e", "editor"), hint(keys.Continue, "c", "continuar", m.continueEnabled),
			on(keys.Skip, "s", "saltar"), on(keys.Abort, "a", "abortar"), on(keys.Quit, "q", "salir (el run queda reanudable)")}
	case StateAborted, StateSucceeded, StateFailed, StateCanceled:
		if bs, ok := m.deleteBindings(); ok {
			return bs
		}
		ok := m.hasBranch()
		return []key.Binding{hint(keys.Push, "p", "preparar push", ok && m.state == StateSucceeded),
			hint(keys.Delete, "d", "borrar rama", ok && (m.state == StateSucceeded || m.state == StateAborted)), exit}
	case StatePickVerification:
		return []key.Binding{bEnter("continuar igualmente"), on(keys.Edit, "e", "editar selección"), bQuit()}
	case StateDeltaGeneration:
		failed := m.deltaErr != nil
		return []key.Binding{hint(keys.Retry, "r", "reintentar", failed), hint(keys.Edit, "e", "editar selección", failed), bQuit()}
	case StatePackageReview:
		delta := m.standaloneMode == "delta"
		return []key.Binding{hint(keys.Override, "o", "override", !delta && m.summary.Empty),
			hint(keys.Enter, "Enter", "validar", !delta), hint(keys.EditAny, "e", "editar selección", !delta), bQuit()}
	case StateQueueReview:
		return []key.Binding{on(keys.Retry, "r", "refrescar"), bEnter("continuar validación"), bEsc("volver"), bQuit()}
	case StateValidationStart:
		return []key.Binding{hint(keys.Retry, "r", "reintentar", m.validateErr != nil), bQuit()}
	case StateValidationPolling:
		return []key.Binding{on(keys.Retry, "r", "refrescar"), on(keys.Continue, "c", "cancelar mi job"), on(keys.Quit, "q", "salir (el job sigue activo)")}
	case StateCancelConfirm:
		return []key.Binding{bEnter("confirmar"), bEsc("volver")}
	case StateRunHistory:
		return m.historyBindings()
	case StateQuickDeploy:
		_, sel := m.selectedRun()
		return []key.Binding{hint(keys.Enter, "Enter", "confirmar", sel), bBack("volver")}
	case StatePushPreparation:
		return m.pushBindings()
	case StateBranchCleanup:
		return m.cleanupBindings()
	case StateBranchCollision:
		return []key.Binding{on(keys.Retry, "r", "reusar"), on(keys.Delete, "d", "borrar y recrear"), on(keys.CancelEsc, "c/Esc", "cancelar"), bQuit()}
	case StateDeployGateBlocked:
		return []key.Binding{bEsc("volver al historial"), bQuit()}
	case StateError:
		return []key.Binding{exit}
	}
	return nil
}

// deleteBindings are the inline current-branch delete confirmation's keys.
func (m Model) deleteBindings() ([]key.Binding, bool) {
	switch m.cleanupPhase {
	case cleanupConfirm:
		return yesNo(), true
	case cleanupStrongConfirm:
		return typedConfirm(), true
	}
	return nil, false
}

func yesNo() []key.Binding {
	return []key.Binding{on(keys.Yes, "y", "confirmar"), on(keys.No, "n/Esc/q", "cancelar")}
}

func typedConfirm() []key.Binding { return []key.Binding{bEnter("confirmar"), bEsc("cancelar")} }

func (m Model) historyBindings() []key.Binding {
	rec, sel := m.selectedRun()
	quick, _ := runs.QuickDeployEligible(rec, m.now())
	return []key.Binding{hint(keys.Enter, "Enter", "reanudar", len(m.runs) > 0), hint(keys.Delete, "d", "detalle", len(m.runs) > 0),
		hint(keys.Quick, "x", "quick deploy", sel && quick), hint(keys.Retry, "r", "re-promocionar", sel && isRePromoteEligible(rec)),
		hint(keys.Nav, "↑/↓", "navegar", len(m.runs) > 0), bEsc("volver"), bQuit()}
}

func (m Model) cleanupBindings() []key.Binding {
	prune := on(keys.Push, "p", "retención de runs")
	switch {
	case m.cleanupPhase == cleanupLoading:
		return []key.Binding{on(keys.Interrupt, "ctrl+c", "interrumpe")}
	case len(m.cleanupBranches) == 0:
		return []key.Binding{prune, bEsc("volver"), bQuit()}
	}
	switch m.cleanupPhase {
	case cleanupCounting:
		return []key.Binding{bEsc("cancelar")}
	case cleanupConfirm, cleanupPruneConfirm:
		return yesNo()
	case cleanupStrongConfirm:
		return typedConfirm()
	}
	return []key.Binding{on(keys.Delete, "d", "borrar"), prune, bNav("navegar"), bEsc("volver"), bQuit()}
}

func (m Model) pushBindings() []key.Binding {
	q := bQuit()
	ai := "sugerir título"
	if m.aiTitle != "" {
		ai = "aceptar sugerencia"
	}
	aiKey := hint(keys.AI, "a", ai, m.aiAvailable())
	switch m.pushPhase {
	case pushConfirm:
		return []key.Binding{on(keys.Push, "p", "ejecutar push"), bEsc("volver"), q}
	case pushReady, pushPRConfirm, pushPRCreating:
	default:
		return []key.Binding{q}
	}
	if m.authState != github.AuthAuthenticated {
		return []key.Binding{hint(aiKey, "a", ai, m.pushPhase == pushReady && m.aiAvailable()), q}
	}
	switch {
	case m.prURL != "", m.pushPhase == pushPRCreating:
		return []key.Binding{q}
	case m.pushPhase == pushPRConfirm && m.aiPending:
		return []key.Binding{on(keys.NoAny, "n/Esc/q", "volver")}
	case m.pushPhase == pushPRConfirm && m.aiTitle != "" && !m.aiAccepted:
		return []key.Binding{on(keys.YesAny, "y", "con IA"), on(keys.DeclineAny, "d", "título default"), on(keys.NoAny, "n/Esc/q", "cancelar")}
	case m.pushPhase == pushPRConfirm:
		return []key.Binding{on(keys.Yes, "y", "confirmar crear PR"), on(keys.No, "n/Esc/q", "cancelar")}
	}
	return []key.Binding{on(keys.PR, "g", "crear PR con gh"), aiKey, q}
}
