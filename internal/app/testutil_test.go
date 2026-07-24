package app

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
)

// errStub is a sentinel error for error-path transition tests.
var errStub = errors.New("stub error")

// keyPress builds the tea.KeyMsg for a key spec, translating the special-key
// names the handlers match via KeyMsg.String() and treating anything else as a
// literal rune sequence.
func keyPress(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// typeString feeds each rune of s as a key press, returning the resulting
// model.
func typeString(m Model, s string) Model {
	for _, r := range s {
		next, _ := m.Update(keyPress(string(r)))
		m = next.(Model)
	}
	return m
}
