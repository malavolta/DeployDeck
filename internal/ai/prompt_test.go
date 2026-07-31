package ai_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/ai"
)

// TestBuild_IncludesTicketSubjectsAndSummary is task 1.3 (RED): Build is a
// pure function composing the system+user prompt from ticket, commit
// subjects, and the pre-rendered componentSummary (design ADR-1:
// componentSummary is rendered inside internal/app so internal/ai needs no
// internal/delta dependency).
func TestBuild_IncludesTicketSubjectsAndSummary(t *testing.T) {
	system, user := ai.Build("PROJ-1", []string{"feat: add X", "fix: bug Y"}, "Types: ApexClass(2), CustomField(1)")
	if system == "" {
		t.Fatal("expected a non-empty system prompt instructing the TITLE:/DESCRIPTION: response shape")
	}
	if !strings.Contains(strings.ToUpper(system), "TITLE:") {
		t.Errorf("system prompt should instruct the model to emit a TITLE: label, got %q", system)
	}
	for _, want := range []string{"PROJ-1", "feat: add X", "fix: bug Y", "ApexClass(2)"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing %q, got %q", want, user)
		}
	}
}

// TestBuild_CapsCommitSubjectCount is task 1.3 (RED): a table-driven
// truncation-cap proof — max commit subjects included.
func TestBuild_CapsCommitSubjectCount(t *testing.T) {
	var subjects []string
	for i := 0; i < 50; i++ {
		subjects = append(subjects, fmt.Sprintf("subject %d", i))
	}
	_, user := ai.Build("PROJ-1", subjects, "")

	count := strings.Count(user, "\n- ")
	if count > 20 {
		t.Fatalf("expected at most 20 commit-subject bullet lines, got %d", count)
	}
	if strings.Contains(user, "subject 49") {
		t.Error("expected subjects beyond the cap to be dropped, found the 50th subject")
	}
	if !strings.Contains(user, "subject 0") {
		t.Error("expected the earliest subjects to be kept")
	}
}

// TestBuild_CapsPerSubjectLength proves an individual overlong commit
// subject is truncated rather than included verbatim.
func TestBuild_CapsPerSubjectLength(t *testing.T) {
	long := strings.Repeat("x", 500)
	_, user := ai.Build("PROJ-1", []string{long}, "")
	if strings.Contains(user, long) {
		t.Fatal("expected an overlong subject to be truncated, not included verbatim")
	}
}

// TestBuild_CapsTotalUserPromptCharBudget proves the whole composed user
// prompt is bounded, so a huge selection plus a huge componentSummary can
// never produce an unbounded request body.
func TestBuild_CapsTotalUserPromptCharBudget(t *testing.T) {
	var subjects []string
	for i := 0; i < 20; i++ {
		subjects = append(subjects, strings.Repeat("y", 120))
	}
	_, user := ai.Build("PROJ-1", subjects, strings.Repeat("z", 10000))
	if len(user) > 4000 {
		t.Fatalf("expected the user prompt capped at 4000 chars, got %d", len(user))
	}
}

// TestBuild_EmptyInputsDegradeGracefully proves Build never panics on empty
// inputs (nil commitSubjects, empty componentSummary) — the degrade-to-no-op
// discipline this whole slice follows.
func TestBuild_EmptyInputsDegradeGracefully(t *testing.T) {
	system, user := ai.Build("", nil, "")
	if system == "" {
		t.Error("expected a non-empty system prompt even with empty inputs")
	}
	if user == "" {
		t.Error("expected a non-empty user prompt even with empty inputs")
	}
}
