package ai

import (
	"fmt"
	"strings"
)

// maxCommitSubjects/maxSubjectLen/maxUserPromptChars bound Build's output
// (design ADR-4: "pure, table-tested" truncation caps), so a large commit
// selection or an oversized componentSummary can never produce an unbounded
// request body sent to the local model.
const (
	maxCommitSubjects  = 20
	maxSubjectLen      = 120
	maxUserPromptChars = 4000
)

// systemPrompt instructs the model to respond with the exact two-label,
// plain-text contract ParseSummary parses (design ADR-2) — NOT JSON, which
// small models routinely emit invalid.
const systemPrompt = `You are an assistant that drafts a conventional-commit-style pull request title and description from a list of commit subjects and a summary of changed metadata.
Respond with EXACTLY two labeled sections in plain text — no markdown code fences, no JSON:
TITLE: <a single-line, conventional-commit-style title, at most 120 characters>
DESCRIPTION: <a short description of the change, may span multiple lines>
Do not include anything else in your response.`

// Build composes the system and user prompt sent to the local model
// (design ADR-1/ADR-4). It is pure: componentSummary is a pre-rendered
// string (internal/app renders it from delta.PackageSummary before calling
// through the scalar Deps.GenerateSummary seam), so this package needs no
// internal/delta dependency. commitSubjects and the overall user prompt are
// truncated so an unbounded selection never produces an unbounded request.
func Build(ticket string, commitSubjects []string, componentSummary string) (system, user string) {
	subjects := truncateSubjects(commitSubjects)

	var b strings.Builder
	fmt.Fprintf(&b, "Ticket: %s\n\n", ticket)
	b.WriteString("Commit subjects:\n")
	if len(subjects) == 0 {
		b.WriteString("(none)\n")
	}
	for _, s := range subjects {
		b.WriteString("- " + s + "\n")
	}

	b.WriteString("\nChanged metadata summary:\n")
	if componentSummary == "" {
		b.WriteString("(none)\n")
	} else {
		b.WriteString(componentSummary + "\n")
	}

	return systemPrompt, capRunes(b.String(), maxUserPromptChars)
}

// truncateSubjects caps the number of commit subjects included and the
// length of each individual subject.
func truncateSubjects(subjects []string) []string {
	if len(subjects) > maxCommitSubjects {
		subjects = subjects[:maxCommitSubjects]
	}
	out := make([]string, len(subjects))
	for i, s := range subjects {
		out[i] = capRunes(s, maxSubjectLen)
	}
	return out
}
