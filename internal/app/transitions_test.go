package app

import (
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
)

// testConfig is a small, valid config: two environments (only UAT has a
// sandbox mapping, so INT exercises the unmapped-destination block) plus the
// default branch format.
func testConfig() config.Config {
	return config.Config{
		Branches: map[string]string{"integration": "INT", "uat": "UAT"},
		Sandboxes: map[string]config.SandboxConfig{
			"UAT": {Alias: "UAT_SBX", TestLevel: "RunLocalTests"},
		},
		TicketPatterns: []string{"PROJ-[0-9]+"},
		BranchFormat:   config.DefaultBranchFormat,
	}
}

func discovered(sha, short, subject string, merge bool) git.DiscoveredCommit {
	c := git.Commit{SHA: sha, ShortSHA: short, Author: "ana", Date: time.Now(), Subject: subject}
	if merge {
		c.Parents = []string{"p1", "p2"}
	}
	return git.DiscoveredCommit{Commit: c, Merge: merge}
}

// TestModel_TicketInput_To_Discovery types a ticket and confirms it, moving to
// discovery and fixing the preliminary target from config.
func TestModel_TicketInput_To_Discovery(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateTicketInput

	m = typeString(m, "PROJ-1")
	if m.ticket != "PROJ-1" {
		t.Fatalf("typed ticket = %q, want PROJ-1", m.ticket)
	}

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateCommitDiscovery {
		t.Errorf("enter should start discovery, got %v", nm.State())
	}
	if nm.prelim != "INT" { // ListDestinations sorts by env key: integration < uat
		t.Errorf("preliminary target = %q, want INT (first destination)", nm.prelim)
	}
	if cmd == nil {
		t.Errorf("discovery should return a command")
	}

	// Empty ticket is blocked.
	empty := New(Deps{Dir: "/repo", Config: testConfig()})
	empty.state = StateTicketInput
	next2, _ := empty.Update(keyPress("enter"))
	if next2.(Model).State() != StateTicketInput {
		t.Errorf("empty ticket must not start discovery")
	}
}

// TestModel_Discovery_To_Selection builds the selection model from a discovery
// result, disabling the merge commit.
func TestModel_Discovery_To_Selection(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateCommitDiscovery
	m.ticket = "PROJ-1"

	result := git.DiscoverResult{
		Ticket: "PROJ-1",
		OrderedCommits: []git.DiscoveredCommit{
			discovered("aaa111", "aaa111", "Fix validation", false),
			discovered("bbb222", "bbb222", "MERGE pr", true),
		},
	}
	next, _ := m.Update(discoverDoneMsg{result: result, source: git.Branch{Name: "feature/PROJ-1"}})
	nm := next.(Model)

	if nm.State() != StateCommitSelection {
		t.Fatalf("discovery done should reach StateCommitSelection, got %v", nm.State())
	}
	if len(nm.items) != 2 {
		t.Fatalf("expected 2 selection items, got %d", len(nm.items))
	}
	if nm.items[0].Selected != true {
		t.Errorf("normal commit should start selected")
	}
	if nm.items[1].Disabled != true || !nm.items[1].Merge {
		t.Errorf("merge commit should be disabled")
	}
}

// TestModel_Selection_Confirm_And_EmptyBlock triangulates the selection
// confirm gate: a non-empty selection advances to target selection and
// generates a plan; deselecting everything blocks with a notice.
func TestModel_Selection_Confirm_And_EmptyBlock(t *testing.T) {
	base := New(Deps{Dir: "/repo", Config: testConfig()})
	base.state = StateCommitSelection
	base.ticket = "PROJ-1"
	ordered := []git.DiscoveredCommit{
		discovered("aaa111", "aaa111", "Fix validation", false),
		discovered("bbb222", "bbb222", "Fields read only", false),
	}
	base.discovery = git.DiscoverResult{OrderedCommits: ordered}
	base.items = git.NewCommitSelectionItems(ordered, "PROJ-1")

	// Confirm with the default (both selected) -> target selection + plan.
	next, _ := base.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateTargetSelection {
		t.Fatalf("valid selection should reach StateTargetSelection, got %v", nm.State())
	}
	if len(nm.plan.SelectedCommits) != 2 {
		t.Errorf("plan should record 2 selected commits, got %d", len(nm.plan.SelectedCommits))
	}
	if nm.plan.Ticket != "PROJ-1" {
		t.Errorf("plan ticket = %q, want PROJ-1", nm.plan.Ticket)
	}
	if !nm.contiguous {
		t.Errorf("both consecutive commits selected should be a contiguous selection")
	}

	// Deselect both, then confirm -> blocked, stays in selection.
	blocked := base
	blocked.items = git.ToggleSelection(blocked.items, 0)
	blocked.items = git.ToggleSelection(blocked.items, 1)
	next2, _ := blocked.Update(keyPress("enter"))
	nm2 := next2.(Model)
	if nm2.State() != StateCommitSelection {
		t.Errorf("empty selection must stay on the selection screen")
	}
	if nm2.notice == "" {
		t.Errorf("empty selection should surface a notice")
	}
}

// TestModel_Target_Resolve triangulates the destination resolution: choosing a
// mapped destination advances to the plan preview and renders the branch name;
// choosing an unmapped destination blocks with an actionable notice.
func TestModel_Target_Resolve(t *testing.T) {
	build := func() Model {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StateTargetSelection
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", SelectedCommits: []git.DiscoveredCommit{discovered("aaa111", "aaa111", "x", false)}}
		m.destinations = git.ListDestinations(m.deps.Config) // [INT(unmapped), UAT(mapped)]
		return m
	}

	// Cursor on UAT (index 1) -> mapped -> plan preview.
	mapped := build()
	mapped.targetCursor = 1
	next, _ := mapped.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StatePlanPreview {
		t.Fatalf("mapped destination should reach StatePlanPreview, got %v (notice=%q)", nm.State(), nm.notice)
	}
	if nm.plan.TargetBranch != "UAT" || nm.plan.SandboxAlias != "UAT_SBX" || nm.plan.TestLevel != "RunLocalTests" {
		t.Errorf("plan target not persisted: %+v", nm.plan)
	}
	if nm.branchName != "deploy/PROJ-1-to-UAT" {
		t.Errorf("rendered branch name = %q, want deploy/PROJ-1-to-UAT", nm.branchName)
	}

	// Cursor on INT (index 0) -> unmapped -> blocked with notice.
	unmapped := build()
	unmapped.targetCursor = 0
	next2, _ := unmapped.Update(keyPress("enter"))
	nm2 := next2.(Model)
	if nm2.State() != StateTargetSelection {
		t.Errorf("unmapped destination must block on the target screen")
	}
	if !strings.Contains(nm2.notice, "no sandbox") {
		t.Errorf("unmapped destination should surface an actionable notice, got %q", nm2.notice)
	}
}

// TestModel_PlanPreview_To_BranchCreation confirms the plan preview starts
// branch creation.
func TestModel_PlanPreview_To_BranchCreation(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StatePlanPreview
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
	m.branchName = "deploy/PROJ-1-to-UAT"

	next, cmd := m.Update(keyPress("enter"))
	if next.(Model).State() != StateBranchCreation {
		t.Errorf("enter should start branch creation, got %v", next.(Model).State())
	}
	if cmd == nil {
		t.Errorf("branch creation should return a command")
	}
}

// TestModel_View_RendersScreens asserts each rendered screen carries its
// load-bearing content (behavioral, not styling).
func TestModel_View_RendersScreens(t *testing.T) {
	ordered := []git.DiscoveredCommit{
		discovered("aaa111", "aaa111", "Fix validation", false),
		discovered("bbb222", "bbb222", "MERGE pr", true),
	}

	// Selection screen.
	sel := New(Deps{Dir: "/repo", Config: testConfig()})
	sel.state = StateCommitSelection
	sel.ticket = "PROJ-1"
	sel.source = git.Branch{Name: "origin/feature/PROJ-1"}
	sel.prelim = "UAT"
	sel.items = git.NewCommitSelectionItems(ordered, "PROJ-1")
	selView := sel.View()
	for _, want := range []string{"PROJ-1", "Fix validation", "Seleccionados: 1", "origin/feature/PROJ-1", git.ReasonMergeCommit} {
		if !strings.Contains(selView, want) {
			t.Errorf("selection view missing %q\n%s", want, selView)
		}
	}

	// Plan preview screen.
	pv := New(Deps{Dir: "/repo", Config: testConfig()})
	pv.state = StatePlanPreview
	pv.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", SandboxAlias: "UAT_SBX", SelectedCommits: ordered[:1]}
	pv.branchName = "deploy/PROJ-1-to-UAT"
	pvView := pv.View()
	// "-x" is asserted here (not just "git cherry-pick aaa111") so the
	// preview stays in lockstep with the executed command: CherryPick
	// (service_cherrypick.go's cherryPickArgs) runs `git cherry-pick -x
	// <sha>` for its provenance trailer (design D4); the preview must show
	// exactly what will run, never a stale command shape.
	for _, want := range []string{"deploy/PROJ-1-to-UAT", "git cherry-pick -x aaa111", "git checkout -b deploy/PROJ-1-to-UAT origin/UAT"} {
		if !strings.Contains(pvView, want) {
			t.Errorf("plan preview view missing %q\n%s", want, pvView)
		}
	}

	// Conflict screen with continue disabled.
	cf := conflictModel()
	cf.continuePending = []string{"classes/AccountService.cls (text) is unmerged/unstaged"}
	cfView := cf.View()
	for _, want := range []string{"classes/AccountService.cls", "deshabilitado"} {
		if !strings.Contains(cfView, want) {
			t.Errorf("conflict view missing %q\n%s", want, cfView)
		}
	}

	// Verification screen with a partial-promotion warning.
	vf := New(Deps{Dir: "/repo", Config: testConfig()})
	vf.state = StatePickVerification
	vf.plan = git.DeploymentPlan{TargetBranch: "UAT"}
	vf.verification = git.PickVerification{PartialFiles: []string{"objects/Account/fields/Status__c.field-meta.xml"}}
	vfView := vf.View()
	if !strings.Contains(vfView, "Status__c.field-meta.xml") || !strings.Contains(vfView, "partial") {
		t.Errorf("verification view missing the partial-promotion warning\n%s", vfView)
	}
}
