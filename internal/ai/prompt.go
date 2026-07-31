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
// small models routinely emit invalid. The explicit conventional-commit type
// list, the anti-hallucination rule, and the one-shot example were tuned
// empirically against a local 3B model (qwen2.5-coder:3b): without them the
// model produced non-prefixed titles and invented changes (e.g. documentation)
// not present in the inputs.
const systemPrompt = `You draft a conventional-commit-style pull request title and description from a list of commit subjects and a summary of changed metadata.

Rules:
- The TITLE MUST begin with a conventional-commit type: feat, fix, chore, refactor, test, docs, perf, build, or ci, optionally with a scope in parentheses. Single line, imperative mood, at most 120 characters.
- The DESCRIPTION MUST summarize ONLY the changes stated in the commit subjects. Do NOT invent changes (such as documentation) that are not listed. Do NOT restate, recompute, or reinterpret the metadata counts.
- Respond with EXACTLY these two labeled plain-text sections, nothing else, no markdown fences, no JSON:
TITLE: <title>
DESCRIPTION: <2 to 4 concise sentences>

Example
Commit subjects:
- fix: handle empty cart on checkout
- test: cover empty cart path
Output:
TITLE: fix(checkout): guard against an empty cart
DESCRIPTION: Add a guard so checkout no longer fails on an empty cart. Add a test covering the empty-cart path.`

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
