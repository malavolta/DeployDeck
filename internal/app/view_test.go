package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/gate"
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

// --- deploy-error-detail: validationBody rendering growth -------------------

// TestValidationBody_ComponentFailure_ShowsFileLineColumnAndProblemTypeLabel
// is the deploy-error-detail RED (task 4.1, validation-progress spec
// "Metadata Errors Shown With Component Detail" + "Warning-Typed Component
// Failure Is Labeled Distinctly"): componentFailures render
// fileName:lineNumber[:columnNumber], and problemType=Warning entries are
// labeled distinctly (mark "!!") from Error entries (mark "XX").
func TestValidationBody_ComponentFailure_ShowsFileLineColumnAndProblemTypeLabel(t *testing.T) {
	m := Model{}
	m.report = salesforce.DeployReport{
		Status: "Failed",
		ComponentFailures: []salesforce.ComponentFailure{
			{Component: "MyClass", Type: "ApexClass", Message: "Compile error", FileName: "classes/MyClass.cls", LineNumber: 12, ColumnNumber: 5, ProblemType: "Error"},
			{Component: "MyTrigger", Type: "ApexTrigger", Message: "Unused variable", FileName: "triggers/MyTrigger.trigger", LineNumber: 3, ProblemType: "Warning"},
		},
	}

	body := m.validationBody()
	if !strings.Contains(body, "classes/MyClass.cls:12:5") {
		t.Errorf("expected file:line:col for the Error entry, got:\n%s", body)
	}
	if !strings.Contains(body, "triggers/MyTrigger.trigger:3") {
		t.Errorf("expected file:line (no column) for the Warning entry, got:\n%s", body)
	}
	if !strings.Contains(body, mark("XX")) {
		t.Errorf("expected the Error entry labeled via mark(XX), got:\n%s", body)
	}
	if !strings.Contains(body, mark("!!")) {
		t.Errorf("expected the Warning entry labeled distinctly via mark(!!), got:\n%s", body)
	}
}

// TestValidationBody_TestFailure_ShowsFirstStackTraceFrameMuted is the
// deploy-error-detail RED (task 4.2, validation-progress spec "Failed Tests
// Shown With Class And Method Detail" + "Failed Test Shows A Compact
// Stack-Trace Excerpt"): a failed test with a multi-frame StackTrace renders
// ONLY its first frame, muted (styleDim, D2).
func TestValidationBody_TestFailure_ShowsFirstStackTraceFrameMuted(t *testing.T) {
	m := Model{}
	m.report = salesforce.DeployReport{
		Status: "Failed",
		TestFailures: []salesforce.TestFailure{
			{Class: "MyClassTest", Method: "testSomething", Message: "System.AssertException: Assertion Failed",
				StackTrace: "Class.MyClassTest.testSomething: line 10, column 1\nClass.Helper.doWork: line 3, column 1"},
		},
	}

	body := m.validationBody()
	if !strings.Contains(body, styleDim.Render("Class.MyClassTest.testSomething: line 10, column 1")) {
		t.Errorf("expected the first stack frame rendered muted, got:\n%s", body)
	}
	if strings.Contains(body, "Class.Helper.doWork") {
		t.Errorf("expected ONLY the first stack frame (bounded, D2), got second frame in:\n%s", body)
	}
}

// TestValidationBody_TestFailure_NoStackTrace_RendersNoFrameLine proves a
// test failure without a StackTrace renders no frame line at all (backward
// compatible with pre-existing reports).
func TestValidationBody_TestFailure_NoStackTrace_RendersNoFrameLine(t *testing.T) {
	m := Model{}
	m.report = salesforce.DeployReport{
		Status:       "Failed",
		TestFailures: []salesforce.TestFailure{{Class: "MyClassTest", Method: "testSomething", Message: "boom"}},
	}

	body := m.validationBody()
	if !strings.Contains(body, "MyClassTest.testSomething: boom") {
		t.Errorf("expected the class/method/message line, got:\n%s", body)
	}
}

// TestValidationBody_CoverageBelowGate_ShowsPercentWorstFirstCappedWithOverflow
// is the deploy-error-detail RED (task 4.3, D3, validation-progress spec
// "Coverage Failures Show Per-Class Percentage Below Gate"): classes below
// the 75% gate render their percentage, worst-first, capped at 10 rows with
// a "+K más" overflow line.
func TestValidationBody_CoverageBelowGate_ShowsPercentWorstFirstCappedWithOverflow(t *testing.T) {
	m := Model{}
	var coverage []salesforce.CodeCoverageResult
	// 12 classes below gate (60%..71%) plus 1 class at/above gate (80%, must
	// be excluded entirely).
	for i := 0; i < 12; i++ {
		coverage = append(coverage, salesforce.CodeCoverageResult{
			Name: fmt.Sprintf("Class%02d", i), NumLocations: 100, NumLocationsNotCovered: 40 - i, // 60%..71%
		})
	}
	coverage = append(coverage, salesforce.CodeCoverageResult{Name: "WellCovered", NumLocations: 100, NumLocationsNotCovered: 20}) // 80%, excluded
	m.report = salesforce.DeployReport{Status: "Failed", CodeCoverage: coverage}

	body := m.validationBody()
	if !strings.Contains(body, "Class00: 60%") {
		t.Errorf("expected the worst class (60%%) shown FIRST, got:\n%s", body)
	}
	if strings.Contains(body, "WellCovered") {
		t.Errorf("a class at/above the 75%% gate must NOT be rendered, got:\n%s", body)
	}
	if !strings.Contains(body, "+2 más") {
		t.Errorf("expected a '+2 más' overflow line (12 below-gate, capped at 10), got:\n%s", body)
	}
	// Row 11 (index 10, 11th worst = 70%) must be capped OUT.
	if strings.Contains(body, "Class10: 70%") || strings.Contains(body, "Class11: 71%") {
		t.Errorf("expected only the 10 worst rows shown (capped), got:\n%s", body)
	}
	// Remediation RED (readability WARNING+SUGGESTION): a Failed report whose
	// ONLY detail is below-gate coverage must not ALSO render the
	// contradictory "sin detalle estructurado" fallback — the fallback guard
	// used to check only the legacy CodeCoverageWarnings field, missing
	// belowGate entirely.
	if strings.Contains(body, "sin detalle estructurado") {
		t.Errorf("below-gate coverage IS structured detail; must not also render the no-detail fallback, got:\n%s", body)
	}
}

// TestValidationBody_FlowCoverageWarnings_RenderInlineUnderCoverageHeading is
// the deploy-error-detail RED (task 4.3): flowCoverageWarnings render inline
// under the "Cobertura de código" heading, prefixed "flujo:".
func TestValidationBody_FlowCoverageWarnings_RenderInlineUnderCoverageHeading(t *testing.T) {
	m := Model{}
	m.report = salesforce.DeployReport{
		Status:               "Failed",
		FlowCoverageWarnings: []salesforce.FlowCoverageWarning{{FlowName: "My_Flow", Message: "Flow coverage below threshold"}},
	}

	body := m.validationBody()
	if !strings.Contains(body, "Cobertura de código") {
		t.Errorf("expected the coverage heading present even with only a flow warning, got:\n%s", body)
	}
	if !strings.Contains(body, "flujo: My_Flow: Flow coverage below threshold") {
		t.Errorf("expected the flow warning rendered inline prefixed 'flujo:', got:\n%s", body)
	}
	// Remediation RED (readability WARNING+SUGGESTION): a Failed report whose
	// ONLY detail is a Flow coverage warning must not ALSO render the
	// contradictory "sin detalle estructurado" fallback (same divergence bug
	// as the below-gate-only case above).
	if strings.Contains(body, "sin detalle estructurado") {
		t.Errorf("a flow coverage warning IS structured detail; must not also render the no-detail fallback, got:\n%s", body)
	}
}

// TestValidationBody_OrgWideCoverageWarning_StaysDistinctFromPerClassLines is
// the deploy-error-detail RED (task 4.3): when both a per-class % line and
// an existing org-wide (empty-Name) CodeCoverageWarning are present, the
// org-wide entry keeps its own "cobertura global" label, distinct from the
// new per-class lines.
func TestValidationBody_OrgWideCoverageWarning_StaysDistinctFromPerClassLines(t *testing.T) {
	m := Model{}
	m.report = salesforce.DeployReport{
		Status:               "Failed",
		CodeCoverageWarnings: []salesforce.CodeCoverageWarning{{Name: "", Message: "Average test coverage across all Apex Classes and Triggers is 40%"}},
		CodeCoverage:         []salesforce.CodeCoverageResult{{Name: "MyClass", NumLocations: 100, NumLocationsNotCovered: 40}},
	}

	body := m.validationBody()
	if !strings.Contains(body, "[cobertura global] Average test coverage") {
		t.Errorf("expected the org-wide warning to keep its distinct 'cobertura global' label, got:\n%s", body)
	}
	if !strings.Contains(body, "MyClass: 60%") {
		t.Errorf("expected the per-class line shown too, got:\n%s", body)
	}
}

// TestValidationBody_ReportPath_ShownOnTerminalScreensOnly is the
// deploy-error-detail RED (task 4.4, D5, validation-progress spec "Latest
// Persisted Report Path Shown On Failure Screens"): m.reportPath renders
// muted on a terminal screen (state != StateValidationPolling), replacing
// the "revisa el JSON crudo" dead-end, but is OMITTED while still live
// polling.
func TestValidationBody_ReportPath_ShownOnTerminalScreensOnly(t *testing.T) {
	m := Model{}
	m.reportPath = "/repo/.deploydeck/runs/PROJ-1-to-UAT/report-003.json"
	m.report = salesforce.DeployReport{Status: "Failed"}

	m.state = StateFailed
	terminalBody := m.validationBody()
	if !strings.Contains(terminalBody, styleDim.Render("Reporte completo: "+m.reportPath)) {
		t.Errorf("expected the muted report path line on a terminal screen, got:\n%s", terminalBody)
	}
	if strings.Contains(terminalBody, "revisa el JSON crudo") {
		t.Errorf("the dead-end 'revisa el JSON crudo' hint must be gone, got:\n%s", terminalBody)
	}

	m.state = StateValidationPolling
	livePollBody := m.validationBody()
	if strings.Contains(livePollBody, m.reportPath) {
		t.Errorf("the report path must NOT render while still live-polling, got:\n%s", livePollBody)
	}
}

// TestViewValidationResult_SucceededPartial_ShowsDistinctHeaderAndCallout is
// the deploy-error-detail RED (task 4.5, D4, validation-progress spec
// "SucceededPartial Renders An Explicit Partial-Success Callout"):
// StateSucceeded with report.Status=="SucceededPartial" renders a distinct
// header + amber callout, never the plain-success header text.
func TestViewValidationResult_SucceededPartial_ShowsDistinctHeaderAndCallout(t *testing.T) {
	m := Model{}
	m.state = StateSucceeded
	m.report = salesforce.DeployReport{Status: "SucceededPartial"}

	view := m.View()
	if strings.Contains(view, "Resultado De Validación") {
		t.Errorf("SucceededPartial must NOT render the plain-success header, got:\n%s", view)
	}
	if !strings.Contains(view, mark("!!")) {
		t.Errorf("expected an amber (mark !!) partial-success callout, got:\n%s", view)
	}
}

// TestViewValidationStart_LaunchFailure_ShowsMessageAndPersistedPath is the
// deploy-error-detail RED (task 4.9, D7, deploy-validation spec "Launch
// Error Shows An Actionable Message And The Persisted-Raw Path"):
// viewValidationStart on a launch failure shows the error message together
// with the persisted validate.json path (Phase 3.7/3.8's rawPath).
func TestViewValidationStart_LaunchFailure_ShowsMessageAndPersistedPath(t *testing.T) {
	m := Model{}
	m.state = StateValidationStart
	m.validateErr = errStub
	m.validateRawPath = "/repo/.deploydeck/runs/PROJ-1-to-UAT/validate.json"

	view := m.View()
	if !strings.Contains(view, "stub error") {
		t.Errorf("expected the launch error message shown, got:\n%s", view)
	}
	if !strings.Contains(view, m.validateRawPath) {
		t.Errorf("expected the persisted validate.json path shown, got:\n%s", view)
	}
}

// TestViewValidationResult_PlainSucceeded_KeepsExistingHeader proves a plain
// Succeeded result is UNCHANGED by the SucceededPartial callout addition.
func TestViewValidationResult_PlainSucceeded_KeepsExistingHeader(t *testing.T) {
	m := Model{}
	m.state = StateSucceeded
	m.report = salesforce.DeployReport{Status: "Succeeded"}

	view := m.View()
	if !strings.Contains(view, "Resultado De Validación") {
		t.Errorf("a plain Succeeded result should keep the existing header, got:\n%s", view)
	}
}

// --- deploy-error-detail remediation: org-sourced string sanitization -----

// TestValidationBody_ComponentFailure_SanitizesControlCharsInMessage is the
// remediation RED (review finding risk WARNING: "Sanitize org-sourced
// strings at render"): a component failure's org-sourced Message reaches
// the operator's terminal verbatim today — a co-org user's crafted deploy
// error must not be able to inject control characters/ANSI escapes there.
func TestValidationBody_ComponentFailure_SanitizesControlCharsInMessage(t *testing.T) {
	m := Model{}
	m.report = salesforce.DeployReport{
		Status: "Failed",
		ComponentFailures: []salesforce.ComponentFailure{
			{Component: "MyClass", Type: "ApexClass", Message: "Compile error\x07\x1b[31m injected", ProblemType: "Error"},
		},
	}

	body := m.validationBody()
	if strings.ContainsRune(body, 0x1b) || strings.ContainsRune(body, 0x07) {
		t.Errorf("expected control chars stripped from the component failure message, got:\n%q", body)
	}
}

// TestValidationBody_TestFailure_StackFrame_SanitizesEscapes is the
// remediation RED (same finding): the first-stack-trace-frame excerpt is
// org-sourced text too, and must have escape sequences stripped before
// rendering (styling is applied AFTER sanitization).
func TestValidationBody_TestFailure_StackFrame_SanitizesEscapes(t *testing.T) {
	m := Model{}
	m.report = salesforce.DeployReport{
		Status: "Failed",
		TestFailures: []salesforce.TestFailure{
			{Class: "MyClassTest", Method: "testSomething", Message: "boom",
				StackTrace: "Class.MyClassTest.testSomething: line 10\x1b[31m, column 1"},
		},
	}

	body := m.validationBody()
	if strings.ContainsRune(body, 0x1b) {
		t.Errorf("expected the ESC byte stripped from the stack-frame excerpt, got:\n%q", body)
	}
}

// TestBelowGateCoverage_SkipsZeroLocationClasses is the remediation RED
// (review finding reliability WARNING: "NumLocations==0 classes are N/A,
// not 0% culprits"): a class with no executable locations is vacuously
// covered, not a 0% culprit — Percent()'s divide-by-zero guard would
// otherwise sort it FIRST (worst) and displace a genuine low-coverage class
// out of the capped rows into "+K más".
func TestBelowGateCoverage_SkipsZeroLocationClasses(t *testing.T) {
	coverage := []salesforce.CodeCoverageResult{
		{Name: "NoLocations", NumLocations: 0, NumLocationsNotCovered: 0},
		{Name: "LowCoverage", NumLocations: 100, NumLocationsNotCovered: 50}, // 50%, genuine culprit
	}

	below := belowGateCoverage(coverage)
	for _, c := range below {
		if c.Name == "NoLocations" {
			t.Fatalf("expected the NumLocations==0 class excluded entirely (vacuously covered), got:\n%+v", below)
		}
	}
	if len(below) != 1 || below[0].Name != "LowCoverage" {
		t.Errorf("expected only the genuine low-coverage class, got:\n%+v", below)
	}
}

// --- 6.14: viewDeployGateBlocked -------------------------------------------

// TestViewDeployGateBlocked_ListsEveryUnmetCondition is task 6.14 (RED,
// Ascii TestMain plain-text assertions), updated by the remediation-pass
// readability fix (Fix 7): the gate-block screen lists EVERY unmet condition
// + its reason, not only the first (deploy-gate spec: "All unmet conditions
// are listed together") — rendered with a Spanish LABEL (conditionLabelES),
// never c.Name's raw English machine key ("approvals", "signature", …)
// inlined into otherwise-Spanish copy.
func TestViewDeployGateBlocked_ListsEveryUnmetCondition(t *testing.T) {
	m := New(Deps{})
	m.state = StateDeployGateBlocked
	m.gateConditions = []gate.Condition{
		{Name: "approvals", Passed: false, Detail: "0/1 aprobaciones requeridas"},
		{Name: "threads", Passed: true, Detail: "todos los threads resueltos"},
		{Name: "validation-comment", Passed: false, Detail: "no existe un comentario de validación en el PR"},
		{Name: "signature", Passed: false, Detail: "firma de procedencia ausente o no verificable"},
	}

	v := m.View()
	for _, want := range []string{
		"aprobaciones", "0/1 aprobaciones requeridas",
		"comentario de validación", "no existe un comentario",
		"firma", "firma de procedencia ausente",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("expected the gate-block screen to mention %q, got:\n%s", want, v)
		}
	}
	// The raw English machine keys must never leak into the rendered screen.
	for _, badKey := range []string{"approvals", "signature", "validation-comment"} {
		if strings.Contains(v, badKey) {
			t.Errorf("expected the raw English condition key %q to be replaced by a Spanish label, got:\n%s", badKey, v)
		}
	}
}

// TestConditionLabelES_MapsEveryKnownConditionName is the remediation-pass
// readability fix (Fix 7): every gate.Condition.Name the app ever produces
// (pr-resolution, approvals, threads, validation-comment, signature) has a
// dedicated Spanish label — never the raw English key rendered verbatim.
func TestConditionLabelES_MapsEveryKnownConditionName(t *testing.T) {
	tests := map[string]string{
		"approvals":          "aprobaciones",
		"threads":            "comentarios sin resolver",
		"validation-comment": "comentario de validación",
		"signature":          "firma",
		"pr-resolution":      "localización de la PR",
	}
	for name, want := range tests {
		if got := conditionLabelES(name); got != want {
			t.Errorf("conditionLabelES(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestViewDeployGateBlocked_NoOverrideKeyOffered is task 6.14 (RED): the
// footer offers ONLY q/Esc — no other key is ever advertised as a way to
// proceed anyway (deploy-gate spec: "No override exists"). The screen's own
// body text MAY explain that no override exists (that is the correct,
// honest copy) — this test targets the actionable footer, not prose.
func TestViewDeployGateBlocked_NoOverrideKeyOffered(t *testing.T) {
	m := New(Deps{})
	m.state = StateDeployGateBlocked
	m.gateConditions = []gate.Condition{{Name: "approvals", Passed: false, Detail: "0/1 aprobaciones requeridas"}}

	v := m.View()
	footerStart := strings.LastIndex(v, "[ ")
	if footerStart < 0 {
		t.Fatalf("expected a footer block, got:\n%s", v)
	}
	footerLine := v[footerStart:]
	if !strings.Contains(footerLine, "q") || !(strings.Contains(footerLine, "Esc") || strings.Contains(footerLine, "esc")) {
		t.Fatalf("expected the footer to offer q/Esc, got %q", footerLine)
	}
	for _, bad := range []string{"override", "bypass", "forzar igual", "continuar de todos modos"} {
		if strings.Contains(strings.ToLower(footerLine), bad) {
			t.Errorf("the footer must never advertise a bypass key, found %q in %q", bad, footerLine)
		}
	}
}
