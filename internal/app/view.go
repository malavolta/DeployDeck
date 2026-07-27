package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"deploydeck/internal/config"
	"deploydeck/internal/git"
	"deploydeck/internal/github"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// View renders the current screen. Each branch mirrors a pane in
// docs/MOCKUPS_TUI.md; the scope edge is PickVerification (no
// delta/validation/push screens in this slice).
func (m Model) View() string {
	switch m.state {
	case StatePrereqCheck:
		return m.viewPrereq()
	case StateTicketInput:
		return m.viewTicket()
	case StateCommitDiscovery:
		return header("Buscando commits") + fmt.Sprintf("\n  Ticket: %s\n  Buscando...\n", m.ticket)
	case StateCommitSelection:
		return m.viewSelection()
	case StateTargetSelection:
		return m.viewTarget()
	case StatePlanPreview:
		return m.viewPlanPreview()
	case StateBranchCreation:
		return header("Creando rama de promocion") + fmt.Sprintf("\n  %s\n", m.branchName)
	case StateCherryPicking:
		return m.viewCherryPicking()
	case StateCherryPickConflict:
		return m.viewConflict()
	case StateAborted:
		return header("Cherry-pick abortado") + "\n  El run fue abortado. No se genera delta ni validacion.\n" + footer("q salir")
	case StatePickVerification:
		return m.viewVerification()
	case StateDeltaGeneration:
		return m.viewDeltaGeneration()
	case StatePackageReview:
		return m.viewPackageReview()
	case StateQueueReview:
		return m.viewQueueReview()
	case StateValidationStart:
		return m.viewValidationStart()
	case StateValidationPolling:
		return m.viewValidationPolling()
	case StateCancelConfirm:
		return m.viewCancelConfirm()
	case StateRunHistory:
		return m.viewRunHistory()
	case StatePushPreparation:
		return m.viewPushPreparation()
	case StateSucceeded, StateFailed, StateCanceled:
		return m.viewValidationResult()
	case StateError:
		return header("Error") + fmt.Sprintf("\n  %v\n", m.err) + footer("q salir")
	}
	return ""
}

func header(title string) string {
	return "DeployDeck > " + title + "\n"
}

func footer(keys string) string {
	return "\n[ " + keys + " ]\n"
}

func (m Model) viewPrereq() string {
	var b strings.Builder
	b.WriteString(header("Doctor"))
	b.WriteString(fmt.Sprintf("\n  Repo: %s\n\n  Prerequisitos\n\n", m.deps.Dir))
	for _, c := range m.checks {
		b.WriteString(fmt.Sprintf("  [%s] %s  %s\n", statusMark(c.Status), c.Name, c.Detail))
		if c.FixCommand != "" {
			b.WriteString("      fix: " + c.FixCommand + "\n")
		}
	}
	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	b.WriteString(footer("r reintentar   c continuar con warnings   q salir"))
	return b.String()
}

func statusMark(s prereq.Status) string {
	switch s {
	case prereq.StatusOK:
		return "OK"
	case prereq.StatusWarning:
		return "!!"
	case prereq.StatusBlocking:
		return "XX"
	default:
		return "--"
	}
}

func (m Model) viewTicket() string {
	var b strings.Builder
	b.WriteString(header("Promocionar Commits"))
	b.WriteString("\n  Ticket o incidencia\n\n")
	b.WriteString("  " + m.ticket + "_\n")
	if len(m.deps.Config.TicketPatterns) > 0 {
		b.WriteString("\n  Patrones configurados:\n")
		for _, p := range m.deps.Config.TicketPatterns {
			b.WriteString("  - " + p + "\n")
		}
	}
	b.WriteString("\n  Tambien se buscaran ramas que contengan el texto introducido.\n")
	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	b.WriteString(footer("Enter buscar   Esc volver"))
	return b.String()
}

func (m Model) viewSelection() string {
	var b strings.Builder
	b.WriteString(header("Commits Encontrados"))
	b.WriteString(fmt.Sprintf("\n  Ticket: %s\n", m.ticket))
	if m.source.Name != "" {
		b.WriteString(fmt.Sprintf("  Rama sugerida: %s\n", m.source.Name))
	}
	if m.prelim != "" {
		b.WriteString(fmt.Sprintf("  Destino preliminar: %s\n", m.prelim))
	}
	b.WriteString("\n")

	if len(m.items) == 0 {
		b.WriteString("  No se encontraron commits en el rango.\n")
		for _, alt := range m.discovery.Alternatives {
			b.WriteString("  - " + alt + "\n")
		}
	}

	for i, it := range m.items {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		mark := selectionMark(it)
		line := fmt.Sprintf("%s [%s] %s  %s  %s  %s", cursor, mark, it.ShortSHA, it.Date.Format("2006-01-02"), it.Author, it.Subject)
		if it.Disabled {
			line += "  [" + it.Reason + "]"
		}
		b.WriteString(line + "\n")
	}

	b.WriteString(fmt.Sprintf("\n  Seleccionados: %d\n", countSelected(m.items)))

	warnings := m.selectionWarnings()
	if len(warnings) > 0 {
		b.WriteString("\n  Warnings:\n")
		for _, w := range warnings {
			b.WriteString("  - " + w + "\n")
		}
	}
	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	b.WriteString(footer("Space marcar   Enter continuar   Esc volver"))
	return b.String()
}

func selectionMark(it git.CommitSelectionItem) string {
	switch {
	case it.Disabled && it.Merge:
		return "-"
	case it.Disabled:
		return "!"
	case it.Selected:
		return "x"
	default:
		return " "
	}
}

func countSelected(items []git.CommitSelectionItem) int {
	n := 0
	for _, it := range items {
		if it.Selected {
			n++
		}
	}
	return n
}

// selectionWarnings assembles the per-file dependency warnings and
// multi-ticket notices shown on the selection screen.
func (m Model) selectionWarnings() []string {
	var out []string
	for _, w := range m.depWarnings {
		out = append(out, fmt.Sprintf("%s tiene 1 commit intermedio no seleccionado (%s)", w.File, w.UnselectedSHA))
	}
	for _, it := range m.items {
		if it.MultiTicketNotice {
			out = append(out, fmt.Sprintf("%s menciona tambien %s", it.ShortSHA, strings.Join(it.OtherTickets, ", ")))
		}
	}
	return out
}

func (m Model) viewTarget() string {
	var b strings.Builder
	b.WriteString(header("Seleccionar Destino"))
	b.WriteString(fmt.Sprintf("\n  Ticket: %s\n  Commits seleccionados: %d\n\n  Rama destino\n", m.plan.Ticket, len(m.plan.SelectedCommits)))
	for i, d := range m.destinations {
		cursor := " "
		if i == m.targetCursor {
			cursor = ">"
		}
		line := fmt.Sprintf("%s %-14s Sandbox: %s", cursor, d.Branch, d.Sandbox.Alias)
		if !d.SandboxResolved {
			line += "  [sin sandbox]"
		}
		if git.IsProductionBranch(d.Branch) {
			line += "  [protegida]"
		}
		b.WriteString(line + "\n")
	}
	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	b.WriteString(footer("Enter seleccionar   ↑/↓ navegar   Esc volver"))
	return b.String()
}

func (m Model) viewPlanPreview() string {
	var b strings.Builder
	b.WriteString(header("Preview Del Plan"))
	b.WriteString(fmt.Sprintf("\n  Ticket:          %s\n", m.plan.Ticket))
	b.WriteString(fmt.Sprintf("  Target branch:   %s\n", m.plan.TargetBranch))
	b.WriteString(fmt.Sprintf("  Target org:      %s\n", m.plan.SandboxAlias))
	b.WriteString(fmt.Sprintf("  Deploy branch:   %s\n", m.branchName))
	if m.sandboxWarn {
		b.WriteString(fmt.Sprintf("\n  Warning: la sandbox %s no esta autenticada.\n", m.plan.SandboxAlias))
	}
	b.WriteString("\n  Commits a aplicar:\n")
	for i, c := range m.plan.SelectedCommits {
		b.WriteString(fmt.Sprintf("  %d. %s  %s\n", i+1, c.ShortSHA, c.Subject))
	}
	b.WriteString("\n  Comandos previstos:\n")
	b.WriteString("  git fetch origin\n")
	b.WriteString(fmt.Sprintf("  git checkout -b %s origin/%s\n", m.branchName, m.plan.TargetBranch))
	for _, c := range m.plan.SelectedCommits {
		b.WriteString("  git cherry-pick " + c.ShortSHA + "\n")
	}
	b.WriteString(footer("Enter ejecutar   Esc volver"))
	return b.String()
}

func (m Model) viewCherryPicking() string {
	var b strings.Builder
	b.WriteString(header("Aplicando Cherry-Picks"))
	b.WriteString(fmt.Sprintf("\n  Rama: %s\n  Base: origin/%s\n\n", m.branchName, m.plan.TargetBranch))
	for _, c := range m.plan.SelectedCommits {
		b.WriteString(fmt.Sprintf("  [..] %s %s\n", c.ShortSHA, c.Subject))
	}
	b.WriteString(footer("q salir cuando termine"))
	return b.String()
}

func (m Model) viewConflict() string {
	var b strings.Builder
	b.WriteString(header("Conflicto De Cherry-Pick"))
	b.WriteString("\n")
	// Ticket + "pick N of M" render identically for a fresh sequence and a
	// resumed one (cherry-pick spec: rehydrated conflict context, HU-013 AC
	// docs/HISTORIAS.md:846). pickTotal is 0 outside a cherry-pick, so the
	// counter is omitted then.
	if m.plan.Ticket != "" {
		b.WriteString(fmt.Sprintf("  Ticket: %s\n", m.plan.Ticket))
	}
	if m.pickTotal > 0 {
		b.WriteString(fmt.Sprintf("  Pick %d de %d\n", m.pickIndex, m.pickTotal))
	}
	if m.repoState.CurrentSHA != "" {
		b.WriteString(fmt.Sprintf("  Commit actual: %s\n", m.repoState.CurrentSHA))
	}
	b.WriteString("\n  Archivos en conflicto (se actualiza solo al detectar cambios en el repo):\n\n")
	for _, f := range m.repoState.Unmerged {
		b.WriteString(fmt.Sprintf("  [%s] %s\n", conflictMark(f.Kind), f.Path))
	}
	if m.continueEnabled {
		b.WriteString("\n  Continuar: habilitado\n")
	} else {
		b.WriteString("\n  Continuar: deshabilitado")
		if len(m.continuePending) > 0 {
			b.WriteString(" (" + strings.Join(m.continuePending, "; ") + ")")
		}
		b.WriteString("\n")
	}
	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	b.WriteString(footer("e editor   c continuar   s saltar   a abortar   q salir y retomar"))
	return b.String()
}

func conflictMark(k git.ConflictKind) string {
	switch k {
	case git.ConflictModifyDelete:
		return "D"
	case git.ConflictBinary:
		return "B"
	default:
		return "U"
	}
}

func (m Model) viewDeltaGeneration() string {
	var b strings.Builder
	b.WriteString(header("Generando Delta"))
	if m.deltaErr != nil {
		b.WriteString("\n  [XX] sfdx-git-delta fallo. No se ejecuta validacion.\n\n")
		b.WriteString("  Salida:\n")
		b.WriteString("  " + m.deltaErr.Error() + "\n")
		b.WriteString(footer("r reintentar   e editar seleccion   q salir"))
		return b.String()
	}
	b.WriteString(fmt.Sprintf("\n  Comparando origin/%s..HEAD\n  Generando package.xml...\n", m.plan.TargetBranch))
	b.WriteString(footer("q salir"))
	return b.String()
}

func (m Model) viewPackageReview() string {
	var b strings.Builder
	b.WriteString(header("Resumen De Package"))
	b.WriteString("\n  Cambios aditivos (package.xml):\n")
	if len(m.summary.Types) == 0 {
		b.WriteString("  (sin tipos)\n")
	}
	for _, t := range m.summary.Types {
		b.WriteString(fmt.Sprintf("  - %-16s %d\n", t.Name, t.Count))
	}

	if m.summary.HasDestructive {
		b.WriteString("\n  [!!] Destructive changes (destructiveChanges.xml):\n")
		for _, t := range m.summary.DestructiveTypes {
			b.WriteString(fmt.Sprintf("  - %-16s %d\n", t.Name, t.Count))
		}
	}

	if len(m.summary.SensitiveTypes) > 0 {
		b.WriteString("\n  [!!] Metadata sensible: " + strings.Join(m.summary.SensitiveTypes, ", ") + "\n")
	}

	if len(m.summary.OutsideSourceDirs) > 0 {
		b.WriteString("\n  Ficheros fuera de sourceDirs configurados:\n")
		for _, f := range m.summary.OutsideSourceDirs {
			b.WriteString("  - " + f + "\n")
		}
	}

	if m.summary.Empty {
		if m.emptyConfirmed {
			b.WriteString("\n  [!!] Package vacio: override confirmado.\n")
		} else {
			b.WriteString("\n  [!!] Package vacio. Validacion bloqueada hasta confirmar override.\n")
		}
	}

	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	if m.summary.Empty {
		b.WriteString(footer("o override   Enter validar   e editar seleccion   q salir"))
	} else {
		b.WriteString(footer("Enter validar   e editar seleccion   q salir"))
	}
	return b.String()
}

// viewQueueReview renders HU-009's deploy-queue screen (mockup
// docs/MOCKUPS_TUI.md "Cola De Deploys"): each active job's user, status,
// elapsed time, and component/test progress, highlighting the current run's
// own job (when present) with its approximate 1-based position in the
// CreatedDate-ordered list.
func (m Model) viewQueueReview() string {
	var b strings.Builder
	b.WriteString(header("Cola De Deploys: " + m.plan.SandboxAlias))

	if m.queueErr != nil {
		b.WriteString("\n  [XX] No se pudo consultar la cola de despliegues (el flujo sigue vivo):\n\n")
		b.WriteString("  " + m.queueErr.Error() + "\n")
		b.WriteString(footer("r reintentar   Enter continuar validacion   Esc volver"))
		return b.String()
	}

	b.WriteString("\n  Jobs activos\n\n")
	if len(m.queue) == 0 {
		b.WriteString("  (sin jobs en cola)\n")
	}
	ownPosition := 0
	for i, e := range m.queue {
		mark := ""
		if m.identity != "" && e.Username == m.identity {
			mark = "  [propio]"
			ownPosition = i + 1
		}
		kind := "Deploy"
		if e.CheckOnly {
			kind = "Validate"
		}
		b.WriteString(fmt.Sprintf("  %d. %-10s %-12s %-8s %-16s %4s  Components %d/%d  Tests %d/%d%s\n",
			i+1, e.JobID, e.Status, kind, e.CreatedBy, m.queueElapsed(e),
			e.Components.Deployed, e.Components.Total,
			e.Tests.Completed, e.Tests.Total, mark))
	}
	if ownPosition > 0 {
		b.WriteString(fmt.Sprintf("\n  Posicion aproximada de tu job: %d\n", ownPosition))
	}

	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	b.WriteString(footer("r refrescar   Enter continuar validacion   Esc volver"))
	return b.String()
}

// queueElapsed renders the elapsed time since a queue entry started
// (StartDate, falling back to CreatedDate for a job not yet started),
// relative to the model's injected clock. A zero timestamp (unparseable or
// missing) renders as "-" rather than a nonsensical duration.
func (m Model) queueElapsed(e salesforce.DeployQueueEntry) string {
	start := e.StartDate
	if start.IsZero() {
		start = e.CreatedDate
	}
	if start.IsZero() {
		return "-"
	}
	elapsed := m.now().Sub(start)
	if elapsed < 0 {
		elapsed = 0
	}
	return fmt.Sprintf("%dm", int(elapsed.Minutes()))
}

func (m Model) viewValidationStart() string {
	var b strings.Builder
	b.WriteString(header("Lanzando Validacion"))
	if m.validateErr != nil {
		b.WriteString("\n  [XX] Salesforce CLI devolvio error (el flujo sigue vivo):\n\n")
		b.WriteString("  " + m.validateErr.Error() + "\n")
		b.WriteString(footer("r reintentar   q salir"))
		return b.String()
	}
	b.WriteString(fmt.Sprintf("\n  Target org: %s\n  Test level: %s\n  Validando (async)...\n", m.plan.SandboxAlias, m.plan.TestLevel))
	b.WriteString(footer("q salir"))
	return b.String()
}

func (m Model) viewValidationPolling() string {
	var b strings.Builder
	b.WriteString(header("Validacion En Vivo"))
	b.WriteString(m.validationBody())
	if m.reportErr != nil {
		b.WriteString("\n  (error transitorio, reintentando: " + m.reportErr.Error() + ")\n")
	}
	b.WriteString(footer("r refrescar   q salir (deja el job activo)"))
	return b.String()
}

// viewCancelConfirm renders HU-012's typed-confirmation cancel screen (mockup
// docs/MOCKUPS_TUI.md "Confirmacion De Cancelacion"): it shows the current
// run's OWN job id/org/status and echoes the CANCELAR prompt with what the user
// has typed so far. Confirm only fires when the typed text exactly equals
// CANCELAR (keyCancelConfirm). A cancel failure is surfaced here (non-terminal,
// the run stays unmarked) so the user can retry or back out.
func (m Model) viewCancelConfirm() string {
	var b strings.Builder
	b.WriteString(header("Cancelar Validacion"))
	b.WriteString("\n  Vas a cancelar tu job:\n\n")
	b.WriteString(fmt.Sprintf("  Job Id: %s\n", m.jobID))
	b.WriteString(fmt.Sprintf("  Org:    %s\n", m.plan.SandboxAlias))
	b.WriteString(fmt.Sprintf("  Estado: %s\n", m.report.Status))
	b.WriteString("\n  Esta accion no afecta jobs de otros usuarios.\n")
	b.WriteString("\n  Escribe CANCELAR para confirmar:\n")
	b.WriteString("  " + m.cancelInput + "_\n")
	if m.cancelErr != nil {
		b.WriteString("\n  [XX] La cancelacion fallo (el run NO se marca como cancelado):\n")
		b.WriteString("  " + m.cancelErr.Error() + "\n")
	}
	if m.notice != "" {
		b.WriteString("\n  " + m.notice + "\n")
	}
	b.WriteString(footer("Enter confirmar   Esc volver"))
	return b.String()
}

func (m Model) viewValidationResult() string {
	var b strings.Builder
	b.WriteString(header("Resultado De Validacion"))
	b.WriteString(m.validationBody())
	if m.timedOut {
		b.WriteString("\n  [XX] Timeout: la validacion no alcanzo un estado terminal a tiempo (timed out).\n")
	}
	// Only a successful validation offers push (HU-014 AC1/AC2): Failed and
	// Canceled stay quit-only.
	if m.state == StateSucceeded {
		b.WriteString(footer("p preparar push   Enter/q salir"))
	} else {
		b.WriteString(footer("Enter/q salir"))
	}
	return b.String()
}

// viewPushPreparation renders HU-014's push + PR-preparation screen (mockup
// docs/MOCKUPS_TUI.md "Push Y PR"): the local/target branches and job id, the
// push command, and — once the push completes — the PR data and gh branch. It
// renders by pushPhase so the two side-effect confirmations (push, PR) are
// always shown before they run.
func (m Model) viewPushPreparation() string {
	var b strings.Builder
	b.WriteString(header("Push Y PR"))
	b.WriteString("\n  Validacion exitosa\n\n")
	b.WriteString(fmt.Sprintf("  Branch local:  %s\n", m.plan.PromotionBranch))
	b.WriteString(fmt.Sprintf("  Target branch: %s\n", m.plan.TargetBranch))
	if m.jobID != "" {
		b.WriteString(fmt.Sprintf("  Job Id:        %s\n", m.jobID))
	}
	b.WriteString("\n  Comando:\n")
	b.WriteString("  git push -u origin " + m.plan.PromotionBranch + "\n")

	switch m.pushPhase {
	case pushConfirm:
		if m.pushErr != nil {
			b.WriteString("\n  [XX] El push fallo (el flujo sigue vivo):\n")
			b.WriteString("  " + m.pushErr.Error() + "\n")
		}
		b.WriteString(footer("p ejecutar push   q salir"))
	case pushPushing:
		b.WriteString("\n  Ejecutando push...\n")
		b.WriteString(footer("q salir"))
	case pushReady, pushPRConfirm, pushPRCreating:
		b.WriteString(m.viewPRData())
	default:
		b.WriteString(footer("q salir"))
	}
	return b.String()
}

// viewPRData renders the post-push PR block: the suggested base/compare/title
// (always shown after a successful push, HU-014 AC4) and the gh branch —
// authed offers `gh pr create` behind an explicit confirm; absent/
// unauthenticated shows the compare URL derived from origin (or the raw origin
// + manual data when the origin form is unrecognized).
func (m Model) viewPRData() string {
	var b strings.Builder
	title := github.SuggestedTitle(m.plan.Ticket, m.plan.TargetBranch)

	b.WriteString("\n  PR sugerido:\n")
	b.WriteString("  base:    " + m.plan.TargetBranch + "\n")
	b.WriteString("  compare: " + m.plan.PromotionBranch + "\n")
	b.WriteString("  title:   " + title + "\n")

	ghCmd := fmt.Sprintf("gh pr create --base %s --head %s --title %q", m.plan.TargetBranch, m.plan.PromotionBranch, title)

	if m.authState == github.AuthAuthenticated {
		b.WriteString("\n  gh detectado y autenticado. Comando de PR:\n")
		b.WriteString("  " + ghCmd + "\n")
		switch {
		case m.prURL != "":
			b.WriteString("\n  [OK] PR creado:\n")
			b.WriteString("  " + m.prURL + "\n")
			b.WriteString(footer("q salir"))
		case m.pushPhase == pushPRConfirm:
			b.WriteString("\n  Confirmar creacion del PR con gh?\n")
			b.WriteString(footer("y confirmar crear PR   n cancelar   q salir"))
		case m.pushPhase == pushPRCreating:
			b.WriteString("\n  Creando PR...\n")
			b.WriteString(footer("q salir"))
		default: // pushReady
			if m.prErr != nil {
				b.WriteString("\n  [XX] La creacion del PR fallo (crea el PR manualmente con los datos de arriba):\n")
				b.WriteString("  " + m.prErr.Error() + "\n")
			}
			b.WriteString(footer("g crear PR con gh   q salir"))
		}
		return b.String()
	}

	// Compare-URL fallback: gh absent or present-unauthenticated.
	if m.authState == github.AuthUnauthenticated {
		b.WriteString("\n  gh detectado pero sin autenticar. Abre el PR en el navegador:\n")
	} else {
		b.WriteString("\n  gh no disponible. Abre el PR en el navegador:\n")
	}
	if m.compareURL != "" {
		b.WriteString("  " + m.compareURL + "\n")
	} else {
		if m.originURL != "" {
			b.WriteString("  origin: " + m.originURL + "\n")
		}
		if m.remoteErr != nil {
			b.WriteString("  (no se pudo leer origin: " + m.remoteErr.Error() + ")\n")
		}
		b.WriteString("  Crea el PR manualmente con base/compare/title de arriba.\n")
	}
	b.WriteString(footer("q salir"))
	return b.String()
}

// validationBody renders the shared live/terminal progress body: job id,
// status, component/test counters, metadata errors and failed tests (HU-011).
func (m Model) validationBody() string {
	var b strings.Builder
	r := m.report
	if m.jobID != "" {
		b.WriteString(fmt.Sprintf("\n  Job: %s\n", m.jobID))
	}
	b.WriteString(fmt.Sprintf("  Estado: %s\n", r.Status))
	b.WriteString(fmt.Sprintf("  Componentes: %d/%d (errores: %d)\n", r.NumberComponentsDeployed, r.NumberComponentsTotal, r.NumberComponentErrors))
	b.WriteString(fmt.Sprintf("  Tests: %d/%d (errores: %d)\n", r.NumberTestsCompleted, r.NumberTestsTotal, r.NumberTestErrors))

	if len(r.ComponentFailures) > 0 {
		b.WriteString("\n  Errores de metadata:\n")
		for _, f := range r.ComponentFailures {
			b.WriteString(fmt.Sprintf("  - %s (%s): %s\n", f.Component, f.Type, f.Message))
		}
	}
	if len(r.TestFailures) > 0 {
		b.WriteString("\n  Tests fallidos:\n")
		for _, f := range r.TestFailures {
			b.WriteString(fmt.Sprintf("  - %s.%s: %s\n", f.Class, f.Method, f.Message))
		}
	}
	return b.String()
}

// viewRunHistory renders HU-013's run-history browse + resume-offer screen
// (mockup docs/MOCKUPS_TUI.md "Historial De Runs"): the past runs newest-first
// (date, ticket, target, progress, jobId) with the selected row marked, plus a
// selected-run detail panel (branch, commit count, delta package path). An
// empty history renders no rows and no error. `d` toggles an expanded detail
// block (status/jobId/phase/pick).
func (m Model) viewRunHistory() string {
	var b strings.Builder
	b.WriteString(header("Historial"))
	b.WriteString("\n\n")

	if len(m.runs) == 0 {
		b.WriteString("  (sin runs registrados)\n")
		b.WriteString(footer("q salir"))
		return b.String()
	}

	for i, rec := range m.runs {
		cursor := " "
		if i == m.runsCursor {
			cursor = ">"
		}
		date := "-"
		if !rec.CreatedAt.IsZero() {
			date = rec.CreatedAt.Format("2006-01-02 15:04")
		}
		b.WriteString(fmt.Sprintf("  %s %-16s %-16s %-6s %-12s %s\n",
			cursor, date, rec.Ticket, rec.Target, runProgressLabel(rec), rec.JobID))
	}

	if m.runsCursor >= 0 && m.runsCursor < len(m.runs) {
		rec := m.runs[m.runsCursor]
		b.WriteString("\n  Run seleccionado:\n")
		b.WriteString(fmt.Sprintf("  Branch: %s\n", git.RenderBranchName(m.deps.Config.BranchFormat, rec.Ticket, rec.Target)))
		b.WriteString(fmt.Sprintf("  Commits: %d\n", len(rec.Commits)))
		b.WriteString(fmt.Sprintf("  Package: %s\n", runPackagePath(m.deps.Config, rec)))
		if m.runDetail {
			b.WriteString("\n  Detalle:\n")
			b.WriteString(fmt.Sprintf("  Estado: %s\n", runProgressLabel(rec)))
			if rec.JobID != "" {
				b.WriteString(fmt.Sprintf("  Job Id: %s\n", rec.JobID))
			}
			if rec.Phase != "" {
				b.WriteString(fmt.Sprintf("  Fase: %s\n", rec.Phase))
			}
			if rec.PickTotal > 0 {
				b.WriteString(fmt.Sprintf("  Pick: %d de %d\n", rec.PickIndex, rec.PickTotal))
			}
		}
	}

	b.WriteString(footer("Enter reanudar   d detalle   ↑/↓ navegar   q salir"))
	return b.String()
}

// runProgressLabel is a history row's progress cell: a run WITH a jobId shows
// its validation status (the last known job state); a run WITHOUT a job shows
// the last flow step reached, i.e. its Phase (run-history spec: "Row Shows
// Progress Reached", HU-013 AC docs/HISTORIAS.md:845).
func runProgressLabel(rec runs.Record) string {
	if rec.JobID != "" {
		if rec.Status != "" {
			return rec.Status
		}
		return "validating"
	}
	if rec.Phase != "" {
		return rec.Phase
	}
	if rec.Status != "" {
		return rec.Status
	}
	return "-"
}

// runPackagePath derives the run's delta package.xml path for the detail panel
// (mockup docs/MOCKUPS_TUI.md:383), composed the same way deltaCmd builds the
// sgd output dir: <deltaBaseDir>/<ticket>-to-<target>/package/package.xml.
func runPackagePath(cfg config.Config, rec runs.Record) string {
	return filepath.Join(deltaBaseDir(cfg), rec.Ticket+"-to-"+rec.Target, "package", "package.xml")
}

func (m Model) viewVerification() string {
	var b strings.Builder
	b.WriteString(header("Verificacion De Promocion"))
	b.WriteString(fmt.Sprintf("\n  Comparando estado final contra origin/%s\n\n", m.plan.TargetBranch))
	if m.verification.OK() {
		b.WriteString("  [OK] Todos los ficheros coinciden con la seleccion.\n")
	} else {
		for _, w := range m.verification.Warnings() {
			b.WriteString("  [!!] " + w + "\n")
		}
		b.WriteString("\n  El estado resultante de los ficheros marcados no existe en ninguna\n  rama y no ha sido probado. Revisa antes de generar el delta.\n")
	}
	b.WriteString(footer("Enter continuar igualmente   e editar seleccion   Esc volver"))
	return b.String()
}
