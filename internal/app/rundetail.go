package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// runReportEntry is one run's cached read-back state. loading is set while a
// load is in flight (blocks duplicate loads); loaded once a result landed.
// missing means no persisted report was found; rejected lists unreadable
// report files skipped on the way to report (newest-first).
type runReportEntry struct {
	loading  bool
	loaded   bool
	missing  bool
	report   salesforce.DeployReport
	path     string
	rejected []string
}

// runReportMsg is the result of one async LatestReport load. gen is the
// history session that fired it; a mismatch drops the result.
type runReportMsg struct {
	runID    string
	gen      int
	found    bool
	report   salesforce.DeployReport
	path     string
	rejected []string
}

// enterHistoryReports starts a new history session: the cache is replaced
// (never mutated, other Model copies may share the old map) and the
// generation bumped so loads from earlier sessions are dropped.
func (m *Model) enterHistoryReports() {
	m.runReports = map[string]runReportEntry{}
	m.reportGen++
}

// ensureReportCmd returns a cmd loading the SELECTED run's latest report, or
// nil when Deps.Runs is unset, there is no selected row, or the entry is
// already loading/loaded. All file I/O happens inside the cmd.
//
// Contract: m.runReports is a map SHARED by every Model copy of the current
// history session. This value-receiver method writes the "loading" mark into
// it on purpose so a copy cannot start a duplicate load; entries are only ever
// replaced whole, and the map itself is replaced (never cleared) by
// enterHistoryReports, so results of an older session cannot leak in.
func (m Model) ensureReportCmd() tea.Cmd {
	writer := m.deps.Runs
	if writer == nil || m.runReports == nil || m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
		return nil
	}
	runID := m.runs[m.runsCursor].RunID
	if e, ok := m.runReports[runID]; ok && (e.loading || e.loaded) {
		return nil
	}
	m.runReports[runID] = runReportEntry{loading: true}
	gen := m.reportGen
	return func() tea.Msg {
		var parsed salesforce.DeployReport
		file, rejected, err := writer.LatestReport(runID, func(raw []byte) bool {
			r, perr := salesforce.ParseDeployReport(raw)
			if perr != nil {
				return false
			}
			parsed = r
			return true
		})
		msg := runReportMsg{runID: runID, gen: gen, rejected: rejected}
		if err == nil {
			msg.found, msg.report, msg.path = true, parsed, file.Path
		}
		return msg
	}
}

// onRunReport lands a load result under ITS RunID (a late result for another
// run is cached for it and never alters the selected run's view), dropping
// results from an earlier history session.
func (m Model) onRunReport(msg runReportMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.reportGen || m.runReports == nil {
		return m, nil
	}
	m.runReports[msg.runID] = runReportEntry{
		loaded:   true,
		missing:  !msg.found,
		report:   msg.report,
		path:     msg.path,
		rejected: msg.rejected,
	}
	return m, nil
}

// motivoLine is the history tail's failure line: shown only once the selected
// run's report has loaded AND classifies as a failure; otherwise empty so the
// tail stays byte-identical.
func (m Model) motivoLine(runID string) string {
	e, ok := m.runReports[runID]
	if !ok || !e.loaded || e.missing {
		return ""
	}
	v := classify(e.report)
	if !v.isFailure() {
		return ""
	}
	return "  Motivo: " + v.headline + "\n"
}

// selectedRecord is the run under the history cursor (zero when out of range).
func (m Model) selectedRecord() (runs.Record, bool) {
	if m.runsCursor < 0 || m.runsCursor >= len(m.runs) {
		return runs.Record{}, false
	}
	return m.runs[m.runsCursor], true
}

// detailLines is the detail's scrollable content, one string per line: the
// failure headline/hint, the run fields, the shared report sections and the
// report path. It is rebuilt from the cache on every call (pure).
func (m Model) detailLines() []string {
	rec, ok := m.selectedRecord()
	if !ok {
		return nil
	}
	e, cached := m.runReports[rec.RunID]
	var b strings.Builder
	var v verdict
	switch {
	case cached && e.loaded && !e.missing:
		v = classify(e.report)
		if v.isFailure() {
			b.WriteString(failureBlock(v, m.width))
		} else {
			b.WriteString("\n  " + v.headline + "\n")
		}
	case cached && e.loading:
		b.WriteString("\n  Cargando reporte...\n")
	default:
		b.WriteString("\n  Sin reporte persistido\n")
	}
	if cached && len(e.rejected) > 0 {
		b.WriteString("  " + styleDim.Render("Reporte ilegible: "+sanitizeTermLine(e.rejected[0])) + "\n")
	}
	b.WriteString("\n")
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
	if cached && e.loaded && !e.missing {
		b.WriteString(reportSections(e.report, v.kind == causeCanceled))
		if e.path != "" {
			b.WriteString("\n  " + styleDim.Render("Reporte completo: "+m.displayPath(e.path)) + "\n")
		}
	}
	return strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
}

// detailChrome is everything the detail draws around its scrollable lines.
// The footer is always a single line, so it is counted as one newline rather
// than rendered: its bindings depend on detailScrollable, which depends on
// the chrome height.
func (m Model) detailChrome() string {
	return m.updateBanner() + m.screenHeader("Detalle Del Run") + "\n"
}

// detailRows is how many content lines fit (0 = unsized, unwindowed).
func (m Model) detailRows() int { return listRows(m.height, m.detailChrome()) }

// detailScrollable reports whether the content overflows the window.
func (m Model) detailScrollable() bool {
	rows := m.detailRows()
	return rows > 0 && len(m.detailLines()) > rows
}

func (m Model) viewRunDetail() string {
	var b strings.Builder
	b.WriteString(m.screenHeader("Detalle Del Run"))
	content := m.detailLines()
	start, end := scrollWindow(m.detailScroll, len(content), m.detailRows())
	b.WriteString(overflow(start, true))
	for _, l := range content[start:end] {
		b.WriteString(truncate(l, m.width) + "\n")
	}
	b.WriteString(overflow(len(content)-end, false))
	b.WriteString(m.footerLine())
	return b.String()
}

// keyRunDetail: up/down scroll (clamped), Esc returns to history with the
// cursor untouched, q quits. No other key acts.
func (m Model) keyRunDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up), key.Matches(msg, keys.Down):
		cur, _ := scrollWindow(m.detailScroll, len(m.detailLines()), m.detailRows())
		if key.Matches(msg, keys.Down) {
			cur++
		} else {
			cur--
		}
		m.detailScroll, _ = scrollWindow(cur, len(m.detailLines()), m.detailRows())
		return m, nil
	case key.Matches(msg, keys.Esc):
		m.state = StateRunHistory
		return m, nil
	case key.Matches(msg, keys.Quit):
		return m, m.quitCmd()
	}
	return m, nil
}

// displayPath shows p relative to the artifacts root (or project dir) when it
// lives under it, so it fits narrow terminals; otherwise p unchanged.
func (m Model) displayPath(p string) string {
	for _, root := range []string{m.deps.ArtifactsRoot, m.deps.ProjectDir} {
		if root == "" {
			continue
		}
		if rel, err := filepath.Rel(root, p); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return rel
		}
	}
	return p
}
