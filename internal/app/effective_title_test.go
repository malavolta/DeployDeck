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

// TestEffectiveDescription_UnacceptedSuggestion_IsEmpty mirrors
// TestEffectiveTitle_UnacceptedSuggestion_UsesFormulaTitle for the
// description slot: with no AI suggestion accepted, effectiveDescription()
// is "" (the historical gh pr create --body "" default) even when
// aiDescription is set.
func TestEffectiveDescription_UnacceptedSuggestion_IsEmpty(t *testing.T) {
	m := New(Deps{})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	if got := m.effectiveDescription(); got != "" {
		t.Fatalf("effectiveDescription() = %q, want empty (no AI accepted)", got)
	}

	// Generating (but not accepting) a suggestion must not change the
	// effective description either.
	m.aiDescription = "PROJ-1 - AI drafted description"
	if got := m.effectiveDescription(); got != "" {
		t.Fatalf("effectiveDescription() with an unaccepted aiDescription = %q, want empty", got)
	}
}

// TestEffectiveDescription_Accepted_UsesAIDescription mirrors
// TestEffectiveTitle_Accepted_UsesAITitle: once aiAccepted is set,
// effectiveDescription() returns aiDescription instead of "".
func TestEffectiveDescription_Accepted_UsesAIDescription(t *testing.T) {
	m := New(Deps{})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
	m.aiDescription = "PROJ-1 - AI drafted description"
	m.aiAccepted = true

	if got := m.effectiveDescription(); got != "PROJ-1 - AI drafted description" {
		t.Fatalf("effectiveDescription() = %q, want the accepted aiDescription", got)
	}
}
