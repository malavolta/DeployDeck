package app

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
)

// TestEffectiveTitle_UnacceptedSuggestion_UsesFormulaTitle is task 4.3
// (RED): with no AI suggestion accepted, effectiveTitle() falls back to the
// pre-existing github.SuggestedTitle formula (design ADR-3, spec:
// "Ignoring the suggestion keeps the formula title").
func TestEffectiveTitle_UnacceptedSuggestion_UsesFormulaTitle(t *testing.T) {
	m := New(Deps{})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	want := github.SuggestedTitle("PROJ-1", "UAT")
	if got := m.effectiveTitle(); got != want {
		t.Fatalf("effectiveTitle() = %q, want %q (formula title, no AI accepted)", got, want)
	}

	// Generating (but not accepting) a suggestion must not change the
	// effective title either.
	m.aiTitle = "PROJ-1 - AI drafted title"
	if got := m.effectiveTitle(); got != want {
		t.Fatalf("effectiveTitle() with an unaccepted aiTitle = %q, want %q (formula title)", got, want)
	}
}

// TestEffectiveTitle_Accepted_UsesAITitle is task 4.3 (RED): once aiAccepted
// is set, effectiveTitle() returns aiTitle instead of the formula (design
// ADR-3: "the AI title replaces github.SuggestedTitle in ALL slots").
func TestEffectiveTitle_Accepted_UsesAITitle(t *testing.T) {
	m := New(Deps{})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
	m.aiTitle = "PROJ-1 - AI drafted title"
	m.aiAccepted = true

	if got := m.effectiveTitle(); got != "PROJ-1 - AI drafted title" {
		t.Fatalf("effectiveTitle() = %q, want the accepted aiTitle", got)
	}
}
