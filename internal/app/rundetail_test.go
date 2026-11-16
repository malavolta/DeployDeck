package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

const motivoCoverage = "Falló por cobertura: 66% global < 75%"

// reportHistoryModel parks a Model on history over a real runs.Writer whose
// run "RUN-F" holds the real (66% coverage) fixture as report-002.json.
func reportHistoryModel(t *testing.T, recs ...runs.Record) Model {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, ".deploydeck", "runs", "RUN-F")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/report_coverage_trailing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "report-002.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if len(recs) == 0 {
		recs = []runs.Record{{RunID: "RUN-F", Ticket: "T-1", Target: "INT", JobID: "0Af1", Status: "Failed"}}
	}
	m := New(Deps{Dir: "/repo", Config: validationConfig(), Runs: runs.NewWriter(base)})
	m.state = StateRunHistory
	m.runs = recs
	return m
}

// loadSelected runs the selected row's load cmd and feeds the result back.
func loadSelected(t *testing.T, m Model) Model {
	t.Helper()
	cmd := m.ensureReportCmd()
	if cmd == nil {
		t.Fatal("expected a load cmd")
	}
	return advance(t, m, cmd())
}

func TestEnsureReportCmd_NilCases(t *testing.T) {
	if cmd := historyModel([]runs.Record{{RunID: "RUN-F"}}).ensureReportCmd(); cmd != nil {
		t.Error("nil Deps.Runs must give a nil cmd")
	}
	m := reportHistoryModel(t)
	if m.ensureReportCmd() == nil {
		t.Fatal("uncached row must give a load cmd")
	}
	if m.ensureReportCmd() != nil {
		t.Error("a loading row must not start a second load")
	}
	loaded := loadSelected(t, reportHistoryModel(t))
	if loaded.ensureReportCmd() != nil {
		t.Error("a cached row must not reload")
	}
	empty := reportHistoryModel(t)
	empty.runs = nil
	if empty.ensureReportCmd() != nil {
		t.Error("no rows must give a nil cmd")
	}
}

func TestRunReport_StaleGenerationDropped(t *testing.T) {
	m := reportHistoryModel(t)
	cmd := m.ensureReportCmd()
	msg := cmd()
	m.reportGen++ // history re-entered meanwhile
	next := advance(t, m, msg)
	if e := next.runReports["RUN-F"]; e.loaded {
		t.Error("a stale-generation result must be dropped")
	}
}

func TestRunReport_LandsUnderItsOwnRunIDOnly(t *testing.T) {
	m := reportHistoryModel(t,
		runs.Record{RunID: "RUN-F", JobID: "0Af1", Status: "Failed"},
		runs.Record{RunID: "RUN-B", JobID: "0Af2", Status: "Failed"})
	msg := m.ensureReportCmd()() // load for RUN-F
	m.runsCursor = 1             // cursor moved before it arrived
	next := advance(t, m, msg)
	if !next.runReports["RUN-F"].loaded {
		t.Error("result must be cached under RUN-F")
	}
	if _, ok := next.runReports["RUN-B"]; ok {
		t.Error("result must not land on the other row")
	}
	if strings.Contains(next.View(), "Motivo:") {
		t.Errorf("the other row's tail must not show RUN-F's motivo:\n%s", next.View())
	}
}

func TestHistoryTail_MotivoOnlyAfterLoad(t *testing.T) {
	m := reportHistoryModel(t)
	before := m.View()
	if strings.Contains(before, "Motivo:") {
		t.Fatalf("no motivo before the load:\n%s", before)
	}
	loaded := loadSelected(t, m)
	v := loaded.View()
	want := "Motivo: " + mark("XX") + " " + motivoCoverage
	if !strings.Contains(v, want) {
		t.Errorf("tail missing %q:\n%s", want, v)
	}
}

func TestHistoryTail_ByteIdenticalWithoutRuns(t *testing.T) {
	rec := []runs.Record{{RunID: "RUN-F", Ticket: "T-1", Target: "INT", JobID: "0Af1", Status: "Failed"}}
	if v := historyModel(rec).View(); strings.Contains(v, "Motivo") {
		t.Errorf("nil Runs must leave the tail untouched:\n%s", v)
	}
}

func TestHistoryTail_NoMotivoForSucceededInProgressOrNoReport(t *testing.T) {
	m := reportHistoryModel(t)
	for _, status := range []string{"Succeeded", "SucceededPartial", "InProgress", "Queued"} {
		m.runReports = map[string]runReportEntry{"RUN-F": {loaded: true, report: salesforce.DeployReport{Status: status}}}
		if strings.Contains(m.View(), "Motivo:") {
			t.Errorf("%s must show no motivo", status)
		}
	}
	m.runReports = map[string]runReportEntry{"RUN-F": {loaded: true, missing: true}}
	if strings.Contains(m.View(), "Motivo:") {
		t.Error("a report-less run must show no motivo")
	}
}

func TestEnterHistory_ClearsCacheAndBumpsGeneration(t *testing.T) {
	m := reportHistoryModel(t)
	m.state = StateMainMenu
	m.runReports = map[string]runReportEntry{"RUN-F": {loaded: true}}
	gen := m.reportGen
	m.enterHistoryReports()
	if len(m.runReports) != 0 || m.reportGen != gen+1 {
		t.Errorf("reports=%v gen %d->%d", m.runReports, gen, m.reportGen)
	}
}

func TestHistoryUpDown_ReturnsLoadCmdWhenRunsPresent(t *testing.T) {
	m := reportHistoryModel(t,
		runs.Record{RunID: "RUN-F", JobID: "0Af1", Status: "Failed"},
		runs.Record{RunID: "RUN-B", JobID: "0Af2", Status: "Failed"})
	_, cmd := m.Update(keyPress("down"))
	if cmd == nil {
		t.Fatal("moving onto an uncached row must load its report")
	}
	if _, ok := cmd().(runReportMsg); !ok {
		t.Errorf("cmd must produce a runReportMsg")
	}
	var _ tea.Cmd = cmd
}

// --- slice 2: StateRunDetail -------------------------------------------------

func detailModel(t *testing.T) Model {
	t.Helper()
	m := loadSelected(t, reportHistoryModel(t))
	m.runs[0].Phase, m.runs[0].PickIndex, m.runs[0].PickTotal = "validating", 2, 5
	return advance(t, m, keyPress("d"))
}

func tallDetail(t *testing.T) Model {
	t.Helper()
	m := reportHistoryModel(t)
	var fails []salesforce.ComponentFailure
	for i := 0; i < 40; i++ {
		fails = append(fails, salesforce.ComponentFailure{Component: "Comp" + string(rune('A'+i%26)), Type: "ApexClass", Message: "bad", ProblemType: "Error"})
	}
	m.runReports["RUN-F"] = runReportEntry{loaded: true, report: salesforce.DeployReport{Status: "Failed", ComponentFailures: fails}}
	m.height, m.width = 14, 80
	return advance(t, m, keyPress("d"))
}

func TestKeyRunHistory_DOpensDetailAndLoads(t *testing.T) {
	m := reportHistoryModel(t)
	next, cmd := m.Update(keyPress("d"))
	if next.(Model).state != StateRunDetail {
		t.Fatalf("d must open StateRunDetail, got %v", next.(Model).state)
	}
	if cmd == nil {
		t.Error("d on an uncached row must trigger the load")
	}
	empty := historyModel(nil)
	if advance(t, empty, keyPress("d")).state != StateRunHistory {
		t.Error("d with no runs must stay on history")
	}
}

func TestKeyRunHistory_EnterNeverOpensDetail(t *testing.T) {
	m := reportHistoryModel(t) // Failed row: not resumable
	if advance(t, m, keyPress("enter")).state == StateRunDetail {
		t.Error("Enter must not open the detail")
	}
}

func TestRunDetail_EscKeepsCursorAndQQuits(t *testing.T) {
	m := reportHistoryModel(t,
		runs.Record{RunID: "RUN-A", JobID: "J", Status: "Failed"},
		runs.Record{RunID: "RUN-F", JobID: "J", Status: "Failed"})
	m.runsCursor = 1
	d := advance(t, m, keyPress("d"))
	back := advance(t, d, keyPress("esc"))
	if back.state != StateRunHistory || back.runsCursor != 1 {
		t.Errorf("esc: state=%v cursor=%d", back.state, back.runsCursor)
	}
	if _, cmd := d.Update(keyPress("q")); !isQuit(cmd) {
		t.Error("q must quit")
	}
}

func TestRunDetail_ShowsHeadlineHintFieldsSectionsAndPath(t *testing.T) {
	v := detailModel(t).View()
	for _, want := range []string{
		motivoCoverage, "incluye todas las clases Apex de la org", "NebulaDemoQueueable",
		"Estado: Failed", "Job Id: 0Af1", "Fase: validating", "Pick: 2 de 5",
		"Componentes:", "Cobertura de código:", "Reporte completo: ", "report-002.json",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q:\n%s", want, v)
		}
	}
	if strings.Index(v, motivoCoverage) > strings.Index(v, "Estado:") {
		t.Error("headline must precede Estado")
	}
}

func TestRunDetail_StatesWithoutFailure(t *testing.T) {
	cases := []struct {
		name  string
		entry runReportEntry
		want  string
		not   string
	}{
		{"no report", runReportEntry{loaded: true, missing: true}, "Sin reporte persistido", "Falló"},
		{"loading", runReportEntry{loading: true}, "Cargando reporte", "Falló"},
		{"in progress", runReportEntry{loaded: true, report: salesforce.DeployReport{Status: "InProgress"}}, "En curso", "Falló"},
		{"succeeded", runReportEntry{loaded: true, report: salesforce.DeployReport{Status: "Succeeded"}}, "Validación exitosa", "Falló"},
		{"partial", runReportEntry{loaded: true, report: salesforce.DeployReport{Status: "SucceededPartial"}}, mark("!!") + " Resultado parcial", "Falló"},
		{"unreadable fallback", runReportEntry{loaded: true, report: coverageReport(), path: "/r/report-001.json", rejected: []string{"/r/report-002.json"}}, "Reporte ilegible: /r/report-002.json", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := reportHistoryModel(t)
			m.runReports["RUN-F"] = tc.entry
			m = advance(t, m, keyPress("d"))
			v := m.View()
			if !strings.Contains(v, tc.want) || (tc.not != "" && strings.Contains(v, tc.not)) {
				t.Errorf("want %q (not %q):\n%s", tc.want, tc.not, v)
			}
		})
	}
	// no runs writer at all: nothing to load, no report.
	m := historyModel([]runs.Record{{RunID: "RUN-F", JobID: "J"}})
	if v := advance(t, m, keyPress("d")).View(); !strings.Contains(v, "Sin reporte persistido") {
		t.Errorf("nil Runs detail:\n%s", v)
	}
}

func TestRunDetail_ScrollClampsAtBothEnds(t *testing.T) {
	m := tallDetail(t)
	if m.detailScroll != 0 {
		t.Fatalf("opens at the top, got %d", m.detailScroll)
	}
	if advance(t, m, keyPress("up")).detailScroll != 0 {
		t.Error("up at the top must stay at 0")
	}
	for i := 0; i < 200; i++ {
		m = advance(t, m, keyPress("down"))
	}
	max := len(m.detailLines()) - listRows(m.height, m.detailChrome())
	if m.detailScroll != max {
		t.Errorf("offset must clamp at last page %d, got %d", max, m.detailScroll)
	}
	if advance(t, m, keyPress("down")).detailScroll != max {
		t.Error("down past the end must not move")
	}
	if got := advance(t, m, keyPress("up")).detailScroll; got != max-1 {
		t.Errorf("up from the end: %d", got)
	}
	if v := m.View(); !strings.Contains(v, "↑") || strings.Count(v, "\n") > m.height {
		t.Errorf("windowed detail must show the overflow marker and fit the terminal:\n%s", v)
	}
}

func TestRunDetail_UnsizedIsNotScrollable(t *testing.T) {
	m := tallDetail(t)
	m.height = 0
	if advance(t, m, keyPress("down")).detailScroll != 0 {
		t.Error("unsized detail must not scroll")
	}
}

func TestEscQ_RunDetailRow(t *testing.T) {
	found := false
	for _, f := range escQFixtures() {
		found = found || f.name == "run detail"
	}
	if !found {
		t.Error("esc/q matrix lacks a run detail row")
	}
}

func TestHelpOverlay_RunDetailListsBindings(t *testing.T) {
	m := tallDetail(t)
	m = advance(t, m, keyPress("?"))
	v := strings.Join(strings.Fields(m.View()), " ") // the overlay aligns columns
	for _, want := range []string{"↑/↓ desplazar", "Esc volver", "q salir"} {
		if !strings.Contains(v, want) {
			t.Errorf("help overlay lacks %q:\n%s", want, v)
		}
	}
}

func TestScrollWindow(t *testing.T) {
	cases := []struct{ offset, total, rows, s, e int }{
		{0, 5, 10, 0, 5}, {0, 20, 5, 0, 5}, {-3, 20, 5, 0, 5}, {7, 20, 5, 7, 12},
		{99, 20, 5, 15, 20}, {3, 20, 0, 0, 20},
	}
	for _, c := range cases {
		if s, e := scrollWindow(c.offset, c.total, c.rows); s != c.s || e != c.e {
			t.Errorf("scrollWindow(%d,%d,%d) = %d,%d want %d,%d", c.offset, c.total, c.rows, s, e, c.s, c.e)
		}
	}
}

func TestHistory_InlineDetalleToggleIsGone(t *testing.T) {
	m := historyModel([]runs.Record{{RunID: "r", Ticket: "T-1", Target: "UAT", JobID: "J1", Status: "Failed", Phase: "validating"}})
	d := advance(t, m, keyPress("d"))
	if d.state != StateRunDetail {
		t.Fatalf("d must open the detail, got %v", d.state)
	}
	back := advance(t, d, keyPress("esc"))
	v := back.View()
	for _, banned := range []string{"Detalle:", "Fase:", "Job Id:", "Pick:"} {
		if strings.Contains(v, banned) {
			t.Errorf("history must not render the inline block (%q):\n%s", banned, v)
		}
	}
}
