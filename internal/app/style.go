package app

import "github.com/charmbracelet/lipgloss"

// styleOK/styleWarn/styleErr/styleDim render the semantic color for a status
// token (design D1). styleBold marks a screen's title/header. Under the
// package's Ascii TestMain baseline every style renders byte-identical plain
// text (lipgloss's Ascii profile strips all escape codes); under a real TTY
// color profile each renders its semantic color.
var (
	styleOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // amber/yellow
	styleErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
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
