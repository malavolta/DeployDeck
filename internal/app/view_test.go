package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// TestViewDeltaGeneration_SpinnerFrameAppended is task 5.3 (RED): the
// delta-generation screen appends the animated spinner frame AFTER its
// existing static text (design: "spinnerView(f) ... appended after existing
// static text (never replaces asserted substrings, frame 0 deterministic)").
func TestViewDeltaGeneration_SpinnerFrameAppended(t *testing.T) {
	m := New(Deps{})
	m.state = StateDeltaGeneration
	m.plan.TargetBranch = "UAT"

	m.spinnerFrame = 0
	v0 := m.viewDeltaGeneration()
	if !strings.Contains(v0, "Generando package.xml...") {
		t.Fatalf("existing static text must stay intact, got:\n%s", v0)
	}
	if !strings.Contains(v0, "|") {
		t.Errorf("frame 0 should render the '|' spinner char, got:\n%s", v0)
	}

	m.spinnerFrame = 1
	v1 := m.viewDeltaGeneration()
	if !strings.Contains(v1, "Generando package.xml...") {
		t.Fatalf("existing static text must stay intact, got:\n%s", v1)
	}
	if !strings.Contains(v1, "/") {
		t.Errorf("frame 1 should render the '/' spinner char, got:\n%s", v1)
	}
}

// TestViewRunHistory_ColumnHeaders is task 3.3 (RED): the run-history table
// SHALL render a header row labeling each column, aligned to the existing
// `%-16s %-16s %-6s %-12s` row format (spec: "Labeled Table Column Headers").
func TestViewRunHistory_ColumnHeaders(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: validationConfig()})
	m.state = StateRunHistory
	m.runs = []runs.Record{
		{RunID: "n", Ticket: "PROJ-9", Target: "UAT", JobID: "0AfNEW", Status: "InProgress", CreatedAt: time.Now()},
	}

	v := m.viewRunHistory()
	headerIdx := strings.Index(v, "Fecha")
	dataIdx := strings.Index(v, "PROJ-9")
	if headerIdx < 0 {
		t.Fatalf("expected a header row with Fecha, got:\n%s", v)
	}
	for _, want := range []string{"Ticket", "Destino", "Estado", "Job Id"} {
		if !strings.Contains(v, want) {
			t.Errorf("expected header column %q, got:\n%s", want, v)
		}
	}
	if dataIdx >= 0 && headerIdx > dataIdx {
		t.Errorf("header row should precede the data row, got:\n%s", v)
	}
}

// TestViewQueueReview_ColumnHeaders is task 3.4 (RED): the queue table SHALL
// render a header row aligned to the existing `%-10s %-12s %-8s %-16s %4s`
// row format.
func TestViewQueueReview_ColumnHeaders(t *testing.T) {
	m := queueReviewModel(t, Deps{Dir: "/repo", Config: validationConfig()})
	m.queue = []salesforce.DeployQueueEntry{
		{JobID: "0AfQ1", Status: "InProgress", CreatedBy: "jdoe"},
	}

	v := m.viewQueueReview()
	headerIdx := strings.Index(v, "Job Id")
	dataIdx := strings.Index(v, "0AfQ1")
	for _, want := range []string{"#", "Job Id", "Estado", "Tipo", "Creado por", "Tiempo", "Componentes", "Tests"} {
		if !strings.Contains(v, want) {
			t.Errorf("expected header column %q, got:\n%s", want, v)
		}
	}
	if headerIdx < 0 || (dataIdx >= 0 && headerIdx > dataIdx) {
		t.Errorf("header row should precede the data row, got:\n%s", v)
	}
}

// TestViewBranchCleanup_ColumnHeaders is task 3.5 (RED): the branch-cleanup
// table SHALL render a header row aligned to the existing `%-30s %-6s %-7s`
// row format.
func TestViewBranchCleanup_ColumnHeaders(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: "deploy/PROJ-1-to-UAT"}, MergedLabel: "unknown"},
	}

	v := m.viewBranchCleanup()
	headerIdx := strings.Index(v, "Rama")
	dataIdx := strings.Index(v, "deploy/PROJ-1-to-UAT")
	for _, want := range []string{"Rama", "Antigüedad", "Estado", "Merge"} {
		if !strings.Contains(v, want) {
			t.Errorf("expected header column %q, got:\n%s", want, v)
		}
	}
	if headerIdx < 0 || (dataIdx >= 0 && headerIdx > dataIdx) {
		t.Errorf("header row should precede the data row, got:\n%s", v)
	}
}

// TestViewSelection_EmptyState_OmitsSpaceMarcar is task 3.1 (RED): a zero-item
// selection screen must not advertise the "Space marcar" hint — it only
// applies to populated rows (spec: "Actionable Empty States" — "the footer
// omits the 'Space marcar' hint"). Fails today: viewSelection's footer is
// unconditional.
func TestViewSelection_EmptyState_OmitsSpaceMarcar(t *testing.T) {
	m := New(Deps{})
	m.state = StateCommitSelection
	m.ticket = "PROJ-1"
	m.items = nil
	m.discovery.Alternatives = []string{"try a different ticket", "check the branch name"}

	v := m.viewSelection()
	if !strings.Contains(v, "Esc volver") {
		t.Errorf("empty selection should still offer Esc volver, got:\n%s", v)
	}
	if strings.Contains(v, "Space marcar") {
		t.Errorf("empty selection must NOT advertise Space marcar (no rows to mark), got:\n%s", v)
	}
}

// TestContextBar_RepoAndBranchNoOrg is task 2.1 (RED): before any target org
// is resolved (m.plan.SandboxAlias == ""), the context bar shows only
// "repo · branch" — no org segment, no dangling placeholder (design D2:
// "omit-until-resolved").
func TestContextBar_RepoAndBranchNoOrg(t *testing.T) {
	m := New(Deps{Dir: "/repo/my-org/DeployDeck"})
	m.originalBranch = "feature/X"

	got := m.contextBar()
	want := "  " + filepath.Base("/repo/my-org/DeployDeck") + " · feature/X\n"
	if got != want {
		t.Errorf("contextBar() = %q, want %q", got, want)
	}
}

// TestContextBar_AppendsOrgOnceResolved is task 2.2 (RED): once a target org
// is resolved (m.plan.SandboxAlias != ""), the context bar appends "· org"
// after the branch segment.
func TestContextBar_AppendsOrgOnceResolved(t *testing.T) {
	m := New(Deps{Dir: "/repo/my-org/DeployDeck"})
	m.originalBranch = "feature/X"
	m.plan.SandboxAlias = "myorg"

	got := m.contextBar()
	if !strings.Contains(got, "· feature/X · myorg") {
		t.Errorf("contextBar() = %q, want it to contain %q", got, "· feature/X · myorg")
	}
}

// TestScreenHeader_ComposesHeaderAndContextBar is task 2.1/2.2's companion
// (design's screenHeader contract): screenHeader(title) = header(title) +
// contextBar(), so every routed screen gets both the title line and the bar.
func TestScreenHeader_ComposesHeaderAndContextBar(t *testing.T) {
	m := New(Deps{Dir: "/repo/my-org/DeployDeck"})
	m.originalBranch = "feature/X"

	got := m.screenHeader("Menu Principal")
	want := header("Menu Principal") + m.contextBar()
	if got != want {
		t.Errorf("screenHeader(%q) = %q, want %q", "Menu Principal", got, want)
	}
	if !strings.Contains(got, "DeployDeck · feature/X") {
		t.Errorf("screenHeader(%q) = %q, want it to contain the context bar", "Menu Principal", got)
	}
}

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
