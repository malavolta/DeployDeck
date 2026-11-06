package app

import (
	"os"
	"strings"
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
	// Pin a dark background so AdaptiveColor never issues an OSC 11 terminal
	// query under test and the bright palette stays deterministic.
	lipgloss.SetHasDarkBackground(true)
	code := m.Run()
	if pkgDir != "" {
		_ = os.RemoveAll(pkgDir)
	}
	os.Exit(code)
}

// withBackground forces the profile and background for one test and restores
// the package baseline (Ascii, dark) afterwards.
func withBackground(t *testing.T, profile termenv.Profile, dark bool) {
	t.Helper()
	lipgloss.SetColorProfile(profile)
	lipgloss.SetHasDarkBackground(dark)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(termenv.Ascii)
		lipgloss.SetHasDarkBackground(true)
	})
}

func TestMark_AdaptiveColor(t *testing.T) {
	tests := []struct {
		name string
		dark bool
		tok  string
		want string
	}{
		{"dark OK", true, "OK", "\x1b[92m"},
		{"dark warn", true, "!!", "\x1b[93m"},
		{"dark err", true, "XX", "\x1b[91m"},
		{"light OK", false, "OK", "\x1b[32m"},
		{"light warn", false, "!!", "\x1b[33m"},
		{"light err", false, "XX", "\x1b[31m"},
	}
	for _, tt := range tests {
		for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256} {
			t.Run(tt.name, func(t *testing.T) {
				withBackground(t, profile, tt.dark)
				if got := mark(tt.tok); !strings.Contains(got, tt.want) {
					t.Errorf("mark(%q) = %q, want %q", tt.tok, got, tt.want)
				}
			})
		}
	}
}

func TestMark_AsciiPlainOnBothBackgrounds(t *testing.T) {
	for _, dark := range []bool{true, false} {
		withBackground(t, termenv.Ascii, dark)
		for _, tok := range []string{"OK", "!!", "XX", "i"} {
			if got := mark(tok); got != "["+tok+"]" {
				t.Errorf("dark=%v mark(%q) = %q, want plain", dark, tok, got)
			}
		}
	}
}
