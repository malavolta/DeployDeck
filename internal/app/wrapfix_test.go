package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func realFixtureReport(t *testing.T) salesforce.DeployReport {
	t.Helper()
	raw, err := os.ReadFile("testdata/report_coverage_trailing.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := salesforce.ParseDeployReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// assertFits checks every line of the hint block (from the "El promedio
// global" sentence to the next blank line) fits width.
func assertFits(t *testing.T, screen string, width int) {
	t.Helper()
	in := false
	for _, l := range strings.Split(screen, "\n") {
		if strings.Contains(l, "El promedio global") {
			in = true
		}
		if strings.TrimSpace(l) == "" {
			in = false
		}
		if !in {
			continue
		}
		if w := ansi.StringWidth(l); w > width {
			t.Errorf("line wider than %d (%d): %q", width, w, ansi.Strip(l))
		}
	}
}

func TestResultScreen_HintWrapsAt80AndKeepsClassNames(t *testing.T) {
	m := resultModel(StateFailed, realFixtureReport(t))
	m.width, m.height = 80, 40
	plain := ansi.Strip(m.viewValidationResult())
	assertFits(t, plain, 80)
	for _, name := range []string{"NebulaDemoQueueable", "NebulaDemoService"} {
		if !strings.Contains(plain, name) {
			t.Errorf("class %s cut off:\n%s", name, plain)
		}
	}
	if !strings.Contains(plain, "\n  Clases al 0%: ") {
		t.Errorf("0%% list must start its own line:\n%s", plain)
	}
}

func TestResultScreen_UnsizedHintStaysOneLine(t *testing.T) {
	v := ansi.Strip(resultModel(StateFailed, realFixtureReport(t)).viewValidationResult())
	if !strings.Contains(v, "de este run. Clases al 0%") && !strings.Contains(v, "de este run.\n") {
		t.Errorf("unsized hint must not wrap mid-sentence:\n%s", v)
	}
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "El promedio global") && !strings.Contains(l, "de este run.") {
			t.Errorf("unsized hint wrapped: %q", l)
		}
	}
}

func TestRunDetail_HintWrapsAt80AndScrollCountsWrappedLines(t *testing.T) {
	m := loadSelected(t, reportHistoryModel(t))
	m.state = StateRunDetail
	unsized := len(m.detailLines())
	m.width, m.height = 80, 24
	if got := len(m.detailLines()); got <= unsized {
		t.Errorf("wrapped detail must have more lines than unsized (%d vs %d)", got, unsized)
	}
	var all []string
	for _, l := range m.detailLines() {
		all = append(all, ansi.Strip(l))
	}
	joined := strings.Join(all, "\n")
	assertFits(t, joined, 80)
	for _, name := range []string{"NebulaDemoQueueable", "NebulaDemoService"} {
		if !strings.Contains(joined, name) {
			t.Errorf("class %s cut off:\n%s", name, joined)
		}
	}
	if !strings.Contains(joined, "\n  Clases al 0%: ") {
		t.Errorf("0%% list must start its own line:\n%s", joined)
	}
}

func TestReportPath_RelativeUnderRootAbsoluteOtherwise(t *testing.T) {
	m := loadSelected(t, reportHistoryModel(t))
	m.state = StateRunDetail
	e := m.runReports["RUN-F"]
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(e.path)))) // base
	m.deps.ArtifactsRoot = root
	rel := filepath.Join(".deploydeck", "runs", "RUN-F", "report-002.json")
	joined := ansi.Strip(strings.Join(m.detailLines(), "\n"))
	if !strings.Contains(joined, "Reporte completo: "+rel) || strings.Contains(joined, root) {
		t.Errorf("want relative path %s:\n%s", rel, joined)
	}
	m.deps.ArtifactsRoot = "/elsewhere"
	m.deps.ProjectDir = "/elsewhere"
	joined = ansi.Strip(strings.Join(m.detailLines(), "\n"))
	if !strings.Contains(joined, "Reporte completo: "+e.path) {
		t.Errorf("outside root must stay absolute:\n%s", joined)
	}

	r := resultModel(StateFailed, realFixtureReport(t))
	r.deps.ArtifactsRoot = root
	r.reportPath = e.path
	if v := ansi.Strip(r.viewValidationResult()); !strings.Contains(v, "Reporte completo: "+rel) {
		t.Errorf("result screen must show relative path:\n%s", v)
	}
}

func TestResultScreen_CanceledShowsNameOnce(t *testing.T) {
	r := salesforce.DeployReport{Status: "Canceled", CanceledByName: "ana"}
	v := resultModel(StateFailed, r).viewValidationResult()
	if strings.Count(v, "ana") != 1 || strings.Contains(v, "Cancelado por:") {
		t.Errorf("canceled must show headline only:\n%s", v)
	}
	// Body alone (no headline on screen) keeps the section line, sanitized.
	r.CanceledByName = "an\x1b[31ma"
	body := resultModel(StateFailed, r).validationBody()
	if !strings.Contains(body, "Cancelado por: ") || strings.Contains(body, "\x1b") {
		t.Errorf("body must keep a sanitized section line: %q", body)
	}
}

func TestResultScreen_TimeoutWithNonTerminalReportHasNoHeadline(t *testing.T) {
	m := resultModel(StateFailed, salesforce.DeployReport{Status: "InProgress"})
	m.timedOut = true
	v := m.viewValidationResult()
	if !strings.Contains(v, "Timeout: la validación no alcanzó un estado terminal a tiempo.") {
		t.Errorf("timeout line missing:\n%s", v)
	}
	if strings.Contains(v, "Falló por") || strings.Contains(v, "En curso") {
		t.Errorf("no failure headline on timeout:\n%s", v)
	}
}
