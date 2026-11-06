package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// window returns the [start,end) slice of a list of total rows that fits in
// rows lines with the cursor visible. It is stateless and bottom-anchored: the
// cursor is the last visible row once it passes the first screenful, so moving
// down scrolls one row at a time. rows <= 0 or a list that fits is unwindowed.
func window(cursor, total, rows int) (start, end int) {
	if rows <= 0 || total <= rows {
		return 0, total
	}
	start = min(max(cursor-rows+1, 0), total-rows)
	return start, start + rows
}

// overflow renders the "N más" indicator for n hidden rows above or below.
func overflow(n int, up bool) string {
	if n <= 0 {
		return ""
	}
	arrow := "↓"
	if up {
		arrow = "↑"
	}
	return fmt.Sprintf("  %s %d más\n", arrow, n)
}

// truncate cuts s to at most width display cells with a trailing ellipsis,
// keeping ANSI sequences intact. A width <= 0 leaves s unchanged.
func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

func lines(s string) int { return strings.Count(s, "\n") }

// listRows is how many data rows fit under height once chrome is drawn,
// reserving two lines for the overflow indicators and one for the trailing
// newline, which bubbletea counts as an extra line. Zero means unsized.
func listRows(height int, chrome string) int {
	if height <= 0 {
		return 0
	}
	return max(3, height-lines(chrome)-3)
}

// windowedRows renders data rows (each already newline-free) between top and
// bottom, windowed around cursor to the terminal height and truncated to its
// width. With height and width 0 it is the plain concatenation of the rows.
func (m Model) windowedRows(top, bottom string, rows []string, cursor int) string {
	start, end := window(cursor, len(rows), listRows(m.height, m.updateBanner()+top+bottom))
	var b strings.Builder
	b.WriteString(overflow(start, true))
	for _, r := range rows[start:end] {
		b.WriteString(truncate(r, m.width) + "\n")
	}
	b.WriteString(overflow(len(rows)-end, false))
	return b.String()
}

// clock formats a duration as mm:ss; minutes keep counting past 59.
func clock(d time.Duration) string {
	d = max(d, 0)
	return fmt.Sprintf("%02d:%02d", int(d/time.Minute), int(d%time.Minute/time.Second))
}
