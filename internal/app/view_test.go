package app

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
)

// pushReadyModel builds a minimal pushReady Model for the AI-block view
// tests below.
func pushReadyModel(deps Deps) Model {
	m := New(deps)
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.authState = github.AuthUnauthenticated // compare-fallback path: no gh noise in assertions
	m.compareURL = "https://github.com/org/repo/compare/UAT...deploy/PROJ-1-to-UAT"
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}
	return m
}

// TestView_AI_NoConfig_NoAffordanceShown is task 4.10 (RED): with no
// Deps.GenerateSummary configured, the AI-suggestion affordance/block is
// entirely absent (spec: "No ai config leaves pushReady unchanged").
func TestView_AI_NoConfig_NoAffordanceShown(t *testing.T) {
	m := pushReadyModel(Deps{})
	v := m.View()
	if strings.Contains(v, "Sugerencia IA") {
		t.Errorf("no AI config should show no AI block at all, got:\n%s", v)
	}
}

// TestView_AI_Configured_NoSuggestionYet_ShowsRequestHint is task 4.10
// (RED): with AI configured but no suggestion requested yet, the screen
// offers the request affordance.
func TestView_AI_Configured_NoSuggestionYet_ShowsRequestHint(t *testing.T) {
	m := pushReadyModel(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "", "", nil
	}})
	v := m.View()
	if !strings.Contains(v, "Sugerencia IA") {
		t.Errorf("AI configured with no suggestion yet should show the AI block, got:\n%s", v)
	}
}

// TestView_AI_Pending_ShowsGenerating is task 4.10 (RED): while a request is
// in flight, the screen shows a pending indicator.
func TestView_AI_Pending_ShowsGenerating(t *testing.T) {
	m := pushReadyModel(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "", "", nil
	}})
	m.aiPending = true
	v := m.View()
	if !strings.Contains(v, "Generando") {
		t.Errorf("a pending AI request should show a generating indicator, got:\n%s", v)
	}
}

// TestView_AI_UnacceptedSuggestion_ShowsTitleAndAcceptHint is task 4.10
// (RED): a generated-but-unaccepted suggestion is shown for review, along
// with the accept affordance — but the formula title remains the effective
// one (spec: "Ignoring the suggestion keeps the formula title").
func TestView_AI_UnacceptedSuggestion_ShowsTitleAndAcceptHint(t *testing.T) {
	m := pushReadyModel(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "", "", nil
	}})
	m.aiTitle = "PROJ-1 - AI drafted title"
	m.aiDescription = "AI drafted description"

	v := m.View()
	if !strings.Contains(v, "PROJ-1 - AI drafted title") {
		t.Errorf("an unaccepted suggestion should still be shown for review, got:\n%s", v)
	}
	if !strings.Contains(v, "AI drafted description") {
		t.Errorf("the AI-generated description should be shown alongside the title for review, got:\n%s", v)
	}
	formulaTitle := github.SuggestedTitle("PROJ-1", "UAT")
	if !strings.Contains(v, "title:   "+formulaTitle) {
		t.Errorf("an unaccepted suggestion must NOT replace the effective title line, got:\n%s", v)
	}
}

// TestView_AI_UnacceptedSuggestion_NoDescription_OmitsDescriptionLine is a
// companion to TestView_AI_UnacceptedSuggestion_ShowsTitleAndAcceptHint: when
// the AI reply carries a title but no description, viewAIBlock renders no
// empty description line.
func TestView_AI_UnacceptedSuggestion_NoDescription_OmitsDescriptionLine(t *testing.T) {
	m := pushReadyModel(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "", "", nil
	}})
	m.aiTitle = "PROJ-1 - AI drafted title"

	v := m.View()
	wantBlock := "  PROJ-1 - AI drafted title\n  a aceptar esta sugerencia"
	if !strings.Contains(v, wantBlock) {
		t.Errorf("with no AI description, the title line should be followed directly by the accept hint (no blank description line), got:\n%s", v)
	}
}

// TestView_AI_Accepted_TitleReplacesFormulaEverywhere is task 4.10 (RED,
// design ADR-3): once accepted, the AI title replaces github.SuggestedTitle
// in every slot — the title line, the gh pr create preview, and the
// compare-URL manual-copy block's referenced title.
func TestView_AI_Accepted_TitleReplacesFormulaEverywhere(t *testing.T) {
	m := pushReadyModel(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "", "", nil
	}})
	m.aiTitle = "PROJ-1 - AI drafted title"
	m.aiAccepted = true

	v := m.View()
	formulaTitle := github.SuggestedTitle("PROJ-1", "UAT")
	if strings.Contains(v, formulaTitle) {
		t.Errorf("an accepted AI title must replace the formula title everywhere, formula still present:\n%s", v)
	}
	if !strings.Contains(v, "title:   PROJ-1 - AI drafted title") {
		t.Errorf("accepted AI title should appear on the title line, got:\n%s", v)
	}
}
