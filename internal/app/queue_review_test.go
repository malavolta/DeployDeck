package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// deployQueueSOQL mirrors internal/salesforce's private deployQueueSOQL
// constant (HU-009's exact query) so the FakeRunner canned response matches
// the real args ListDeployQueue composes.
const deployQueueSOQL = "SELECT Id,Status,CheckOnly,CreatedDate,StartDate,CompletedDate,CreatedBy.Name,CreatedBy.Username,NumberComponentsTotal,NumberComponentsDeployed,NumberComponentErrors,NumberTestsTotal,NumberTestsCompleted,NumberTestErrors FROM DeployRequest WHERE Status IN ('Pending','InProgress') ORDER BY CreatedDate ASC"

// queueReviewModel parks a Model on QueueReview with the plan's target org
// set, as confirmPackageReview leaves it before onQueueDone lands.
func queueReviewModel(t *testing.T, deps Deps) Model {
	t.Helper()
	m := New(deps)
	m.state = StateQueueReview
	m.plan = git.DeploymentPlan{
		Ticket: "PROJ-1", TargetBranch: "UAT", SandboxAlias: "UAT_SBX", TestLevel: "RunLocalTests",
	}
	m.plan = git.RegisterDeltaArtifacts(m.plan, "pkg/package.xml", "")
	return m
}

// queueDeps builds a Deps whose Salesforce client is canned for both the
// Orgs() identity lookup (design.md "Own-job identity source") and the
// ListDeployQueue query against alias.
func queueDeps(t *testing.T, alias, ownUsername string, queueStdout []byte) Deps {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{"org", "list", "--json"}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout: []byte(fmt.Sprintf(
			`{"status":0,"result":{"sandboxes":[{"alias":%q,"username":%q,"connectedStatus":"Connected"}]}}`,
			alias, ownUsername)),
	})
	fr.When("sf", []string{
		"data", "query",
		"--target-org", alias,
		"--use-tooling-api",
		"--json",
		"--query", deployQueueSOQL,
	}, execpkg.CommandResult{ExitCode: 0, Stdout: queueStdout})
	return Deps{
		Dir: "/repo", Config: validationConfig(), SF: salesforce.New(fr),
		Runs: runs.NewWriter(t.TempDir()), Now: (&fakeClock{t: time.Unix(0, 0)}).now,
	}
}

func multiUserQueueJSON() []byte {
	return []byte(`{"status":0,"result":{"totalSize":3,"done":true,"records":[
		{"Id":"0AfAAA","Status":"InProgress","CheckOnly":true,"CreatedDate":"2026-07-27T10:00:00.000+0000","StartDate":"2026-07-27T10:00:05.000+0000","CreatedBy":{"Name":"Maria Garcia","Username":"maria@example.com"},"NumberComponentsTotal":34,"NumberComponentsDeployed":12,"NumberComponentErrors":0,"NumberTestsTotal":142,"NumberTestsCompleted":80,"NumberTestErrors":0},
		{"Id":"0AfBBB","Status":"Pending","CheckOnly":false,"CreatedDate":"2026-07-27T10:10:00.000+0000","CreatedBy":{"Name":"Luis Perez","Username":"luis@example.com"},"NumberComponentsTotal":0,"NumberComponentsDeployed":0,"NumberComponentErrors":0,"NumberTestsTotal":0,"NumberTestsCompleted":0,"NumberTestErrors":0},
		{"Id":"0AfCCC","Status":"Pending","CheckOnly":true,"CreatedDate":"2026-07-27T10:15:00.000+0000","CreatedBy":{"Name":"Own User","Username":"own@example.com"},"NumberComponentsTotal":0,"NumberComponentsDeployed":0,"NumberComponentErrors":0,"NumberTestsTotal":0,"NumberTestsCompleted":0,"NumberTestErrors":0}
	]}}`)
}

// --- PackageReview confirm now enters a REAL QueueReview stop --------------

// TestModel_PackageReview_Confirm_EntersQueueReviewAndFiresQueueCmd is task
// 2.5: confirm no longer passes through QueueReview inertly — it stops there
// and fires queueCmd, which resolves identity via Orgs()/FindByAlias and
// queries the queue.
func TestModel_PackageReview_Confirm_EntersQueueReviewAndFiresQueueCmd(t *testing.T) {
	deps := queueDeps(t, "UAT_SBX", "own@example.com", multiUserQueueJSON())
	m := reviewedModel(t, deps, false)

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateQueueReview {
		t.Fatalf("confirm should enter the REAL QueueReview stop, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("entering QueueReview should fire queueCmd")
	}

	msg := run(t, cmd)
	qmsg, ok := msg.(queueDoneMsg)
	if !ok {
		t.Fatalf("expected a queueDoneMsg, got %T", msg)
	}
	if qmsg.err != nil {
		t.Fatalf("queueCmd errored: %v", qmsg.err)
	}
	if qmsg.identity != "own@example.com" {
		t.Fatalf("identity should resolve via Orgs()/FindByAlias(alias).Username, got %q", qmsg.identity)
	}
	if len(qmsg.entries) != 3 {
		t.Fatalf("expected 3 parsed entries, got %d", len(qmsg.entries))
	}
}

// TestModel_QueueCmd_IdentityResolutionIsBestEffort proves an alias that
// Orgs() doesn't know about degrades to an empty identity WITHOUT blocking
// the queue query itself (the query still succeeds and returns entries).
func TestModel_QueueCmd_IdentityResolutionIsBestEffort(t *testing.T) {
	fr := execpkg.NewFakeRunner()
	// Orgs() knows a DIFFERENT alias than the plan targets, so FindByAlias
	// will miss for "UAT_SBX".
	fr.When("sf", []string{"org", "list", "--json"}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"sandboxes":[{"alias":"OTHER_ALIAS","username":"someone@example.com","connectedStatus":"Connected"}]}}`),
	})
	fr.When("sf", []string{
		"data", "query",
		"--target-org", "UAT_SBX",
		"--use-tooling-api",
		"--json",
		"--query", deployQueueSOQL,
	}, execpkg.CommandResult{ExitCode: 0, Stdout: multiUserQueueJSON()})

	deps := Deps{Dir: "/repo", Config: validationConfig(), SF: salesforce.New(fr), Runs: runs.NewWriter(t.TempDir()), Now: (&fakeClock{t: time.Unix(0, 0)}).now}
	m := queueReviewModel(t, deps) // plan.SandboxAlias = "UAT_SBX"

	msg := run(t, m.queueCmd())
	qmsg, ok := msg.(queueDoneMsg)
	if !ok {
		t.Fatalf("expected a queueDoneMsg, got %T", msg)
	}
	if qmsg.err != nil {
		t.Fatalf("a missing alias in Orgs() must not fail the queue query itself: %v", qmsg.err)
	}
	if qmsg.identity != "" {
		t.Fatalf("expected an empty identity when the alias is not found in Orgs(), got %q", qmsg.identity)
	}
	if len(qmsg.entries) != 3 {
		t.Fatalf("the queue query itself should still succeed, got %d entries", len(qmsg.entries))
	}
}

// --- onQueueDone: success shows the list with own-highlight + position -----

func TestModel_OnQueueDone_Success_HighlightsOwnJobWithPositionAndDetails(t *testing.T) {
	m := queueReviewModel(t, Deps{Dir: "/repo", Config: validationConfig()})

	entries := []salesforce.DeployQueueEntry{
		{JobID: "0AfAAA", Status: "InProgress", CreatedBy: "Maria Garcia", Username: "maria@example.com",
			Components: salesforce.QueueComponentProgress{Total: 34, Deployed: 12},
			Tests:      salesforce.QueueTestProgress{Total: 142, Completed: 80}},
		{JobID: "0AfBBB", Status: "Pending", CreatedBy: "Luis Perez", Username: "luis@example.com"},
		{JobID: "0AfCCC", Status: "Pending", CreatedBy: "Own User", Username: "own@example.com", CheckOnly: true},
	}

	next, cmd := m.Update(queueDoneMsg{entries: entries, identity: "own@example.com"})
	nm := next.(Model)
	if nm.State() != StateQueueReview {
		t.Fatalf("a successful queue fetch should land on QueueReview, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("landing on QueueReview with results should not fire another command")
	}
	if len(nm.queue) != 3 {
		t.Fatalf("expected the 3 entries stored on the model, got %d", len(nm.queue))
	}
	if nm.identity != "own@example.com" {
		t.Fatalf("identity should be recorded, got %q", nm.identity)
	}

	view := nm.View()
	if !strings.Contains(view, "propio") {
		t.Errorf("own job should be highlighted, view:\n%s", view)
	}
	if !strings.Contains(view, "tu job: 3") {
		t.Errorf("own job's approximate position (3rd) should be shown, view:\n%s", view)
	}
	if !strings.Contains(view, "Luis Perez") {
		t.Errorf("other jobs should show their user, view:\n%s", view)
	}
}

// TestModel_OnQueueDone_Success_OwnAbsent_ListsRestWithoutHighlight is the
// spec's "own job absent" scenario.
func TestModel_OnQueueDone_Success_OwnAbsent_ListsRestWithoutHighlight(t *testing.T) {
	m := queueReviewModel(t, Deps{Dir: "/repo", Config: validationConfig()})
	entries := []salesforce.DeployQueueEntry{
		{JobID: "0AfBBB", Status: "Pending", CreatedBy: "Luis Perez", Username: "luis@example.com"},
	}
	next, _ := m.Update(queueDoneMsg{entries: entries, identity: "own@example.com"})
	nm := next.(Model)
	view := nm.View()
	if strings.Contains(view, "propio") {
		t.Errorf("own job absent should not show a highlight, view:\n%s", view)
	}
	if !strings.Contains(view, "Luis Perez") {
		t.Errorf("the rest of the queue should still be listed, view:\n%s", view)
	}
}

// TestModel_OnQueueDone_Success_EmptyQueueShowsClearEmptyState is the spec's
// "empty queue" scenario.
func TestModel_OnQueueDone_Success_EmptyQueueShowsClearEmptyState(t *testing.T) {
	m := queueReviewModel(t, Deps{Dir: "/repo", Config: validationConfig()})
	next, _ := m.Update(queueDoneMsg{entries: nil, identity: "own@example.com"})
	nm := next.(Model)
	if len(nm.queue) != 0 {
		t.Fatalf("expected an empty queue, got %d", len(nm.queue))
	}
	view := nm.View()
	if view == "" {
		t.Fatal("empty queue should still render a screen")
	}
}

// --- onQueueDone: permission error auto-skips to ValidationStart -----------

// TestModel_OnQueueDone_PermissionError_AutoSkipsToValidationStartWithNotice
// is the spec's "permission failure skips the queue non-blockingly"
// scenario.
func TestModel_OnQueueDone_PermissionError_AutoSkipsToValidationStartWithNotice(t *testing.T) {
	deps := Deps{Dir: "/repo", Config: validationConfig(), SF: exec_sf(t), Runs: runs.NewWriter(t.TempDir()), Now: (&fakeClock{t: time.Unix(0, 0)}).now}
	m := queueReviewModel(t, deps)

	permErr := fmt.Errorf("wrapped: %w", salesforce.ErrQueuePermission)
	next, cmd := m.Update(queueDoneMsg{err: permErr})
	nm := next.(Model)
	if nm.State() != StateValidationStart {
		t.Fatalf("a permission error should auto-skip to ValidationStart, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("auto-skipping to ValidationStart should fire the validate command")
	}
	if nm.notice == "" {
		t.Error("a permission-error skip should surface a notice explaining why the queue view is skipped")
	}
	if nm.queueErr != nil {
		t.Error("a permission error should not be shown as a queue error (non-blocking skip, not a shown failure)")
	}
}

// --- onQueueDone: generic error stays on QueueReview, non-aborting ---------

// TestModel_OnQueueDone_GenericError_StaysOnQueueReviewNonAborting is the
// spec's "generic query failure shows an actionable error without aborting"
// scenario.
func TestModel_OnQueueDone_GenericError_StaysOnQueueReviewNonAborting(t *testing.T) {
	m := queueReviewModel(t, Deps{Dir: "/repo", Config: validationConfig()})

	next, cmd := m.Update(queueDoneMsg{err: errStub})
	nm := next.(Model)
	if nm.State() != StateQueueReview {
		t.Fatalf("a generic query failure must not abort the flow, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("a generic error should not auto-fire another command")
	}
	if nm.queueErr == nil {
		t.Error("a generic error should be recorded for display")
	}
	if !strings.Contains(nm.View(), "stub error") {
		t.Errorf("QueueReview error view should surface the error, got:\n%s", nm.View())
	}
}

// --- keyQueueReview: enter / r / esc ----------------------------------------

// TestModel_KeyQueueReview_EnterAdvancesToValidationStartAndFiresValidate
// covers `enter`.
func TestModel_KeyQueueReview_EnterAdvancesToValidationStartAndFiresValidate(t *testing.T) {
	deps := Deps{Dir: "/repo", Config: validationConfig(), SF: exec_sf(t), Runs: runs.NewWriter(t.TempDir()), Now: (&fakeClock{t: time.Unix(0, 0)}).now}
	m := queueReviewModel(t, deps)

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateValidationStart {
		t.Fatalf("enter should advance to ValidationStart, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("advancing to ValidationStart should fire the validate command")
	}
}

// TestModel_KeyQueueReview_RRefiresQueueCmd covers `r`.
func TestModel_KeyQueueReview_RRefiresQueueCmd(t *testing.T) {
	deps := queueDeps(t, "UAT_SBX", "own@example.com", multiUserQueueJSON())
	m := queueReviewModel(t, deps)
	m.queueErr = errStub

	next, cmd := m.Update(keyPress("r"))
	nm := next.(Model)
	if nm.State() != StateQueueReview {
		t.Fatalf("r should stay on QueueReview while re-fetching, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("r should re-fire queueCmd")
	}
	if nm.queueErr != nil {
		t.Error("r should clear the previous error before re-fetching")
	}
	msg := run(t, cmd)
	if _, ok := msg.(queueDoneMsg); !ok {
		t.Fatalf("expected a queueDoneMsg from the re-fired command, got %T", msg)
	}
}

// TestModel_KeyQueueReview_EscReturnsToPackageReview covers `esc`.
func TestModel_KeyQueueReview_EscReturnsToPackageReview(t *testing.T) {
	m := queueReviewModel(t, Deps{Dir: "/repo", Config: validationConfig()})

	next, cmd := m.Update(keyPress("esc"))
	nm := next.(Model)
	if nm.State() != StatePackageReview {
		t.Fatalf("esc should return to PackageReview, got %v", nm.State())
	}
	if cmd != nil {
		t.Error("esc should not fire a command")
	}
}
