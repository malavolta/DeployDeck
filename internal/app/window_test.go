package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestWindow(t *testing.T) {
	for _, c := range []struct{ cursor, total, rows, start, end int }{
		{3, 5, 10, 0, 5}, {0, 50, 0, 0, 50}, {0, 50, 10, 0, 10}, {9, 50, 10, 0, 10}, {10, 50, 10, 1, 11},
		{25, 50, 10, 16, 26}, {49, 50, 10, 40, 50}, {99, 50, 10, 40, 50}, {-5, 50, 10, 0, 10},
	} {
		if s, e := window(c.cursor, c.total, c.rows); s != c.start || e != c.end {
			t.Errorf("window(%d,%d,%d) = %d,%d want %d,%d", c.cursor, c.total, c.rows, s, e, c.start, c.end)
		}
	}
}

func TestOverflow(t *testing.T) {
	if overflow(20, true) != "  ↑ 20 más\n" || overflow(3, false) != "  ↓ 3 más\n" || overflow(0, true) != "" || overflow(-1, false) != "" {
		t.Error("overflow copy or zero handling is wrong")
	}
}

func TestTruncate(t *testing.T) {
	long := strings.Repeat("áé", 50)
	if truncate(long, 0) != long {
		t.Error("width 0 must leave the string unchanged")
	}
	if got := truncate(long, 40); ansi.StringWidth(got) > 40 || !strings.HasSuffix(got, "…") {
		t.Errorf("truncate(40) = %q", got)
	}
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(strings.Repeat("x", 80))
	got := truncate(styled, 20)
	if ansi.StringWidth(got) > 20 || !strings.Contains(got, "\x1b[") || ansi.Strip(got) != strings.Repeat("x", 19)+"…" {
		t.Errorf("styled truncate broke ANSI: %q", got)
	}
}

func TestListRows(t *testing.T) {
	if listRows(0, "a\nb\n") != 0 || listRows(-3, "") != 0 {
		t.Error("unsized height must give 0 rows")
	}
	if got := listRows(24, "a\nb\nc\n"); got != 19 {
		t.Errorf("listRows(24, 3 lines) = %d, want 19", got)
	}
	if got := listRows(5, strings.Repeat("x\n", 10)); got != 3 {
		t.Errorf("listRows floors at 3, got %d", got)
	}
}
