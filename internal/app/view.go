package app

import (
	"fmt"
	"strings"

	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
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
	if m.repoState.CurrentSHA != "" {
		b.WriteString(fmt.Sprintf("\n  Commit actual: %s\n", m.repoState.CurrentSHA))
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
