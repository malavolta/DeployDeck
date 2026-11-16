package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func assertBodyFits(t *testing.T, plain string, width int) {
	t.Helper()
	for _, l := range strings.Split(plain, "\n") {
		if strings.Contains(l, "Reporte completo:") {
			continue // path line is truncated by the renderer, never wrapped
		}
		if w := ansi.StringWidth(l); w > width {
			t.Errorf("line wider than %d (%d): %q", width, w, l)
		}
	}
	flat := strings.Join(strings.Fields(plain), " ")
	if !strings.Contains(flat, "at least 75% test coverage is required.") {
		t.Errorf("coverage sentence lost:\n%s", plain)
	}
}

func TestResultScreen_CoverageWarningWrapsAt80(t *testing.T) {
	m := resultModel(StateFailed, realFixtureReport(t))
	m.width, m.height = 80, 40
	plain := ansi.Strip(m.viewValidationResult())
	assertBodyFits(t, plain, 80)
	if !strings.Contains(plain, "\n    ") {
		t.Errorf("no hanging indent:\n%s", plain)
	}
}

func TestRunDetail_CoverageWarningWrapsAt80(t *testing.T) {
	m := loadSelected(t, reportHistoryModel(t))
	m.state = StateRunDetail
	m.width, m.height = 80, 24
	assertBodyFits(t, ansi.Strip(strings.Join(m.detailLines(), "\n")), 80)
}

func TestWrapLine_UnsizedUntouched(t *testing.T) {
	l := "  - [x] " + strings.Repeat("word ", 40)
	if got := wrapLine(l, 0); got != l {
		t.Errorf("width 0 changed line")
	}
}
