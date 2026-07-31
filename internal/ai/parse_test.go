package ai_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/ai"
)

// TestParseSummary is task 1.1 (RED), extended by task 5.1's threat-matrix
// fixtures: ParseSummary must tolerate a small model's messy output while
// NEVER surfacing partial/garbled text as a usable title (design ADR-2).
// An untrusted title eventually reaches `gh pr create --title` (design.md
// Threat Matrix), so every fixture here doubles as that boundary's RED
// proof at the parser layer.
func TestParseSummary(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantTitle string
		wantOK    bool
	}{
		{
			name:      "labeled two-line response",
			raw:       "TITLE: PROJ-1 - Add discount validation\nDESCRIPTION: Adds server-side discount validation before checkout.",
			wantTitle: "PROJ-1 - Add discount validation",
			wantOK:    true,
		},
		{
			name:      "fenced response strips code-fence decoration",
			raw:       "```\nTITLE: PROJ-2 - Fix null pointer\nDESCRIPTION: Guards against a nil account lookup.\n```",
			wantTitle: "PROJ-2 - Fix null pointer",
			wantOK:    true,
		},
		{
			name:      "partial title-only response is still usable",
			raw:       "TITLE: PROJ-3 - Refactor queue handling",
			wantTitle: "PROJ-3 - Refactor queue handling",
			wantOK:    true,
		},
		{
			name:      "newline inside title keeps first line only",
			raw:       "TITLE: PROJ-4 - Fix payment\nflow edge case\nDESCRIPTION: details",
			wantTitle: "PROJ-4 - Fix payment",
			wantOK:    true,
		},
		{
			name:      "dash-dash-prefixed title kept as literal content",
			raw:       "TITLE: --evil-flag looks like a flag\nDESCRIPTION: still just text",
			wantTitle: "--evil-flag looks like a flag",
			wantOK:    true,
		},
		{
			name:      "oversized title is truncated, not rejected",
			raw:       "TITLE: " + strings.Repeat("x", 200),
			wantTitle: strings.Repeat("x", 120),
			wantOK:    true,
		},
		{
			name:   "garbled response with no TITLE label yields no suggestion",
			raw:    "Sure! Here's a summary for you:\nSomething about the change, no labels at all.",
			wantOK: false,
		},
		{
			name:   "empty title after label yields no suggestion",
			raw:    "TITLE:\nDESCRIPTION: some text",
			wantOK: false,
		},
		{
			name:   "empty raw response yields no suggestion",
			raw:    "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, _, ok := ai.ParseSummary(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ParseSummary(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			}
			if !ok {
				if title != "" {
					t.Errorf("ParseSummary ok=false must yield an empty title (never partial/garbled text), got %q", title)
				}
				return
			}
			if title != tt.wantTitle {
				t.Errorf("ParseSummary(%q) title = %q, want %q", tt.raw, title, tt.wantTitle)
			}
		})
	}
}

// TestParseSummary_TitleNeverContainsNewline is the threat-matrix RED proof
// (task 5.1) that a title handed to gh's --title argument can never smuggle
// a second line, regardless of how the raw model output is shaped.
func TestParseSummary_TitleNeverContainsNewline(t *testing.T) {
	raw := "TITLE: PROJ-6 - Fix\nsecond line of raw content\nDESCRIPTION: x"
	title, _, ok := ai.ParseSummary(raw)
	if !ok {
		t.Fatal("expected ok=true for a title-labeled response")
	}
	if strings.Contains(title, "\n") {
		t.Fatalf("title must never contain a newline, got %q", title)
	}
}

// TestParseSummary_DescriptionIsMultilineTrimmedAndCapped proves description
// text may span multiple lines (unlike title), is trimmed, and is capped —
// both by line count and character budget — so a runaway response cannot
// produce an unbounded description.
func TestParseSummary_DescriptionIsMultilineTrimmedAndCapped(t *testing.T) {
	raw := "TITLE: PROJ-5 - Add logging\nDESCRIPTION: Line one.\nLine two.\n\nLine three.\n"
	_, desc, ok := ai.ParseSummary(raw)
	if !ok {
		t.Fatal("expected ok=true")
	}
	for _, want := range []string{"Line one.", "Line two.", "Line three."} {
		if !strings.Contains(desc, want) {
			t.Errorf("description missing %q, got %q", want, desc)
		}
	}
	if strings.HasPrefix(desc, " ") || strings.HasSuffix(desc, "\n") {
		t.Errorf("description should be trimmed, got %q", desc)
	}

	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	huge := "TITLE: PROJ-5b - Add logging\nDESCRIPTION: " + strings.Join(lines, "\n")
	_, hugeDesc, ok := ai.ParseSummary(huge)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if strings.Count(hugeDesc, "\n")+1 > 60 {
		t.Fatalf("expected description line count to be capped, got %d lines", strings.Count(hugeDesc, "\n")+1)
	}
}

// TestParseSummary_LabelsAreCaseInsensitive proves ADR-2's "case-insensitive"
// label match.
func TestParseSummary_LabelsAreCaseInsensitive(t *testing.T) {
	raw := "title: PROJ-7 - lower-case label\ndescription: still parsed"
	title, desc, ok := ai.ParseSummary(raw)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if title != "PROJ-7 - lower-case label" {
		t.Errorf("title = %q, want %q", title, "PROJ-7 - lower-case label")
	}
	if desc != "still parsed" {
		t.Errorf("description = %q, want %q", desc, "still parsed")
	}
}
