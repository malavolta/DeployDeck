package ai

import (
	"strings"
	"unicode"
)

// maxTitleRunes/maxDescriptionLines/maxDescriptionChars bound ParseSummary's
// output so a runaway or hostile model response can never produce an
// unbounded title/description (design ADR-2, threat matrix: the title
// eventually reaches `gh pr create --title`).
const (
	maxTitleRunes       = 120
	maxDescriptionLines = 40
	maxDescriptionChars = 4000
)

const (
	titleLabel       = "title:"
	descriptionLabel = "description:"
)

// ParseSummary extracts a title/description pair from raw, tolerant model
// output (design ADR-2). It strips markdown fence decoration and normalizes
// newlines first, then looks for a line whose trimmed, case-insensitive
// form starts with "TITLE:" — the remainder of THAT line only (never a
// later line) becomes the title, control-char-stripped and length-capped.
// ok is false whenever no usable, non-empty title can be extracted; callers
// then show no suggestion at all — this function NEVER returns a
// partial/garbled title. DESCRIPTION: is optional (a title-only response is
// still usable) and, unlike title, may span multiple lines; it is trimmed
// and capped both by line count and character budget.
func ParseSummary(raw string) (title, description string, ok bool) {
	lines := prepareLines(raw)

	titleIdx := findLabelLine(lines, titleLabel)
	if titleIdx == -1 {
		return "", "", false
	}

	title = sanitizeTitle(labelValue(lines[titleIdx], titleLabel))
	if title == "" {
		return "", "", false
	}

	if descIdx := findLabelLine(lines, descriptionLabel); descIdx != -1 {
		description = extractDescription(lines, descIdx)
	}

	return title, description, true
}

// prepareLines normalizes newlines and strips markdown code-fence delimiter
// lines (``` or ```lang), so a fenced response parses identically to an
// unfenced one.
func prepareLines(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		if isFenceLine(l) {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

func isFenceLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

// findLabelLine returns the index of the first line whose trimmed form
// starts with label (case-insensitive), or -1 when none match.
func findLabelLine(lines []string, label string) int {
	for i, line := range lines {
		if hasLabel(line, label) {
			return i
		}
	}
	return -1
}

func hasLabel(line, label string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < len(label) {
		return false
	}
	return strings.EqualFold(trimmed[:len(label)], label)
}

// labelValue returns the trimmed remainder of line after its label prefix.
func labelValue(line, label string) string {
	trimmed := strings.TrimSpace(line)
	return strings.TrimSpace(trimmed[len(label):])
}

// sanitizeTitle strips every control character (title is always a single
// line — no embedded newline is ever preserved) and caps the result at
// maxTitleRunes.
func sanitizeTitle(s string) string {
	stripped := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return capRunes(strings.TrimSpace(stripped), maxTitleRunes)
}

// extractDescription gathers the DESCRIPTION: line's remainder plus every
// following line up to maxDescriptionLines, skipping any stray repeated
// TITLE: line, then trims and caps the joined result at maxDescriptionChars.
// Unlike title, embedded newlines are preserved (description is allowed to
// span multiple lines); other control characters are stripped.
func extractDescription(lines []string, descIdx int) string {
	var b strings.Builder
	b.WriteString(labelValue(lines[descIdx], descriptionLabel))
	lineCount := 1

	for i := descIdx + 1; i < len(lines) && lineCount < maxDescriptionLines; i++ {
		if hasLabel(lines[i], titleLabel) {
			continue
		}
		b.WriteString("\n")
		b.WriteString(lines[i])
		lineCount++
	}

	stripped := strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, b.String())

	return capRunes(strings.TrimSpace(stripped), maxDescriptionChars)
}
