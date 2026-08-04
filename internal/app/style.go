package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sanitizeTerm strips C0/C1 control characters — including ESC (0x1b), which
// kills any ANSI/OSC escape sequence — from org-sourced text before it
// reaches the operator's terminal (deploy-error-detail remediation, risk
// WARNING: no sanitizer existed on the render path even though the queue/
// validation screens render another org member's free-text fields
// verbatim, e.g. a deploy ErrorMessage). Only printable runes and the
// literal space survive; anything else, including a bare newline or tab, is
// DROPPED. Use sanitizeTermLine instead wherever the sanitized text is
// embedded inline in a single output line, so an embedded newline/tab
// collapses to a readable space instead of vanishing and running two words
// together.
func sanitizeTerm(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !isControlRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sanitizeTermLine is sanitizeTerm's single-line variant: a newline or tab
// collapses to a single space BEFORE the remaining control characters are
// stripped, so an org-controlled field rendered inline (queue detail
// fields, a stack-trace excerpt, component/test/coverage messages) cannot
// use an embedded newline to inject a fake terminal row — the text stays
// readable on ONE line instead of silently losing its word boundaries.
func sanitizeTermLine(s string) string {
	return sanitizeTerm(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		return r
	}, s))
}

// isControlRune reports whether r is a C0 control character (0x00-0x1F,
// including ESC 0x1b and BEL 0x07), DEL itself (0x7f), or a C1 control
// character (0x80-0x9f) — the range ANSI/OSC escape sequences are built
// from.
func isControlRune(r rune) bool {
	return (r >= 0x00 && r <= 0x1f) || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

// styleOK/styleWarn/styleErr/styleDim render the semantic color for a status
// token (design D1/D2). styleBold marks a screen's title/header. Under the
// package's Ascii TestMain baseline every style renders byte-identical plain
// text (lipgloss's Ascii profile strips all escape codes); under a real TTY
// color profile each renders its semantic color. OK/warning/error use the
// BRIGHT ANSI palette (design D2) rather than the base palette — muted base
// colors are hard to read on dark-theme terminals.
var (
	styleOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // bright green
	styleWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // bright yellow
	styleErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // bright red
	styleBold = lipgloss.NewStyle().Bold(true)
	styleDim  = lipgloss.NewStyle().Faint(true)
)

// mark colorizes a bracketed status token (design's "Semantic Color And
// Visual Hierarchy" requirement): [OK] green, [!!] amber, [XX] red, [i]/[--]
// dim, anything else plain. Ascii → plain "[tok]" (no escapes); a real color
// profile → the same text wrapped in the token's semantic ANSI style.
func mark(tok string) string {
	plain := "[" + tok + "]"
	switch tok {
	case "OK":
		return styleOK.Render(plain)
	case "!!":
		return styleWarn.Render(plain)
	case "XX":
		return styleErr.Render(plain)
	case "i", "--":
		return styleDim.Render(plain)
	default:
		return plain
	}
}
