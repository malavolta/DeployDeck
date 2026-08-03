package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMark_AsciiProfileIsPlain is task 1.3 (RED): under the package's Ascii
// TestMain baseline, mark() renders plain "[tok]" text with no ANSI escapes
// — the invariant every pre-existing strings.Contains assertion depends on.
func TestMark_AsciiProfileIsPlain(t *testing.T) {
	cases := map[string]string{
		"OK": "[OK]",
		"!!": "[!!]",
		"XX": "[XX]",
		"i":  "[i]",
	}
	for tok, want := range cases {
		got := mark(tok)
		if got != want {
			t.Errorf("mark(%q) = %q, want %q", tok, got, want)
		}
		if strings.Contains(got, "\x1b") {
			t.Errorf("mark(%q) = %q, want no ANSI escape under Ascii profile", tok, got)
		}
	}
}

// TestMark_NonAsciiProfileAppliesEscape is task 1.4 (RED): under a real color
// profile (TrueColor), mark() applies a styled ANSI escape around the
// bracketed token — the runtime behavior TestMain's Ascii override otherwise
// hides for every other test in the package.
func TestMark_NonAsciiProfileAppliesEscape(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	got := mark("XX")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("mark(\"XX\") under TrueColor = %q, want an ANSI escape sequence", got)
	}
}

// TestMark_NonAsciiUsesBrightColors is task 2.1 (RED, design D2): under a
// real color profile, mark()'s OK/warning/error tokens must use the BRIGHT
// ANSI palette (SGR 92/93/91 for Color("10")/("11")/("9")) rather than the
// base palette (SGR 32/33/31 for Color("2")/("3")/("1")) — bright colors are
// legible on dark-theme terminals, base colors render muted. The Ascii
// baseline (TestMark_AsciiProfileIsPlain, unmodified) stays plain regardless
// of this change, since lipgloss's Ascii profile strips all escape codes.
func TestMark_NonAsciiUsesBrightColors(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	tests := []struct {
		tok        string
		brightCode string
		baseCode   string
	}{
		{tok: "OK", brightCode: "\x1b[92m", baseCode: "\x1b[32m"},
		{tok: "!!", brightCode: "\x1b[93m", baseCode: "\x1b[33m"},
		{tok: "XX", brightCode: "\x1b[91m", baseCode: "\x1b[31m"},
	}

	for _, tt := range tests {
		got := mark(tt.tok)
		if !strings.Contains(got, tt.brightCode) {
			t.Errorf("mark(%q) = %q, want bright escape %q", tt.tok, got, tt.brightCode)
		}
		if strings.Contains(got, tt.baseCode) {
			t.Errorf("mark(%q) = %q, want base escape %q ABSENT (must use bright, not base)", tt.tok, got, tt.baseCode)
		}
	}
}

// TestHeader_NonAsciiProfileBoldsTitle is the RED test for wiring the
// previously-dead styleBold into header() (adversarial-review remediation:
// "bold headers" visual hierarchy). Under a real color profile, header(title)
// must wrap ONLY the title portion in styleBold's ANSI escape — the
// "DeployDeck > " prefix stays plain — mirroring mark()'s own
// Ascii-vs-real-profile split above. Under the package's Ascii TestMain
// baseline, lipgloss strips the escape entirely, so header() stays
// byte-identical plain text and every pre-existing exact-string assertion
// (e.g. TestScreenHeader_ComposesHeaderAndContextBar) keeps passing
// unchanged.
func TestHeader_NonAsciiProfileBoldsTitle(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	got := header("Menu Principal")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("header(%q) under TrueColor = %q, want an ANSI escape sequence (bold title)", "Menu Principal", got)
	}
	if !strings.HasPrefix(got, "DeployDeck > ") {
		t.Errorf("header(%q) = %q, want it to still start with the plain \"DeployDeck > \" prefix", "Menu Principal", got)
	}
	if !strings.Contains(got, "Menu Principal") {
		t.Errorf("header(%q) = %q, want the title text still present", "Menu Principal", got)
	}
}
