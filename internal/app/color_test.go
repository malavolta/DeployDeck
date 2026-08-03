package app

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain forces the package's lipgloss renderer to the Ascii color profile
// for the WHOLE test binary (design D1: "Ascii = plain bytes under test").
// This is the package's sole TestMain — every pre-existing
// strings.Contains-on-plain-text assertion (115 of them) keeps working
// byte-identically, since mark() (style.go) renders plain "[tok]" text under
// Ascii and only emits ANSI escapes under a real TTY color profile.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}
