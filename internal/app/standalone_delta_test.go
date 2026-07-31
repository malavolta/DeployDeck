package app

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/delta"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// fakeBranchListRunner cans the exact sequence of git commands ListBranches
// issues (RepoRoot's rev-parse, then the local and remote branchNames
// listings) so standaloneBranchesCmd can be tested without a real repo.
func fakeBranchListRunner(t *testing.T, root, localOut, remoteOut string) *execpkg.FakeRunner {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(root + "\n")})
	fr.When("git", []string{"branch", "--format=%(refname:short)"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(localOut)})
	fr.When("git", []string{"branch", "--format=%(refname:short)", "-r"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(remoteOut)})
	return fr
}

// TestStandaloneBranchesCmd_ComposesListBranches is task 3.1 (RED):
// standaloneBranchesCmd composes git.Service.ListBranches (the same
// primitive HU-002's candidate-branch search and HU-017's cleanup screen
// use) and reports the result as standaloneBranchesMsg, ready for
// StateDeltaSourceSelect's picker.
func TestStandaloneBranchesCmd_ComposesListBranches(t *testing.T) {
	fr := fakeBranchListRunner(t, "/repo", "main\n", "origin/main\norigin/UAT\n")
	m := New(Deps{Git: git.New(fr), Dir: "/repo"})

	msg := run(t, m.standaloneBranchesCmd())
	bmsg, ok := msg.(standaloneBranchesMsg)
	if !ok {
		t.Fatalf("expected standaloneBranchesMsg, got %T", msg)
	}
	if bmsg.err != nil {
		t.Fatalf("unexpected error: %v", bmsg.err)
	}
	want := []git.Branch{
		{Name: "main", Remote: false},
		{Name: "origin/main", Remote: true},
		{Name: "origin/UAT", Remote: true},
	}
	if !reflect.DeepEqual(bmsg.branches, want) {
		t.Fatalf("branches = %+v, want %+v", bmsg.branches, want)
	}
}

// TestSanitizeBranch is task 3.1 (RED): sanitizeBranch folds "/" to "-" so a
// picked base branch name is safe to embed in the standalone-delta run
// identity (design D2: Ticket = "standalone-" + sanitizeBranch(base)); a
// branch with no "/" is a no-op.
func TestSanitizeBranch(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "slash-separated branch folds to a hyphen", base: "release/1.0", want: "release-1.0"},
		{name: "a branch with no slash is a no-op", base: "main", want: "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeBranch(tt.base); got != tt.want {
				t.Errorf("sanitizeBranch(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

// deltaSourceSelectModel parks a Model on StateDeltaSourceSelect with a
// preloaded branch list, as onStandaloneBranches would have left it.
func deltaSourceSelectModel(branches []git.Branch, cursor int) Model {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateDeltaSourceSelect
	m.standaloneMode = "delta"
	m.branchList = branches
	m.branchCursor = cursor
	return m
}

// TestModel_DeltaSourceSelect_CursorPickAndNormalize is task 3.3 (RED):
// Enter on a remote-tracking branch strips the "origin/" prefix so
// deltaCmd's "origin/"+target resolves correctly, seeds the MINIMAL plan
// deltaCmd reads (design ADR-2: Ticket as a fixed standalone marker,
// TargetBranch as the normalized base), and fires deltaCmd via
// StateDeltaGeneration — mirroring keyVerification's confirm exactly. AC:
// "Standalone Delta Generates A Package Without Cherry-Picks".
func TestModel_DeltaSourceSelect_CursorPickAndNormalize(t *testing.T) {
	branches := []git.Branch{
		{Name: "main", Remote: false},
		{Name: "origin/main", Remote: true},
	}
	m := deltaSourceSelectModel(branches, 1)

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)

	if nm.Plan().TargetBranch != "main" {
		t.Fatalf("origin/ prefix should be stripped, got TargetBranch=%q", nm.Plan().TargetBranch)
	}
	if nm.Plan().Ticket != "standalone-main" {
		t.Fatalf("standalone delta should seed a base-sanitized Ticket, got %q", nm.Plan().Ticket)
	}
	if nm.State() != StateDeltaGeneration {
		t.Fatalf("confirm should enter StateDeltaGeneration (deltaCmd unchanged), got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("confirm should fire deltaCmd")
	}
}

// TestModel_DeltaSourceSelect_Confirm_ResetsStalePlan is the adversarial-review
// remediation for Finding 1 (HIGH), delta side: confirmDeltaSourceSelect must
// build a FRESH minimal {Ticket, TargetBranch} plan so a prior full-flow
// promotion's SelectedCommits / PackageXMLPath / DestructiveChangesPath /
// PromotionBranch cannot bleed into the standalone delta run. AC: "Standalone
// Delta Generates A Package Without Cherry-Picks" (no stale-state leak).
func TestModel_DeltaSourceSelect_Confirm_ResetsStalePlan(t *testing.T) {
	branches := []git.Branch{
		{Name: "main", Remote: false},
		{Name: "origin/main", Remote: true},
	}
	m := deltaSourceSelectModel(branches, 1)
	// Pre-seed a DIRTY plan from a prior full-flow promotion.
	m.plan = git.DeploymentPlan{
		Ticket:                 "PROJ-9",
		SelectedCommits:        []git.DiscoveredCommit{{}},
		TargetBranch:           "UAT",
		SandboxAlias:           "OLD_SBX",
		TestLevel:              "NoTestRun",
		PromotionBranch:        "PROJ-9-to-UAT",
		PackageXMLPath:         "/stale/package.xml",
		DestructiveChangesPath: "/stale/destructiveChanges.xml",
	}

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)

	// Fresh minimal plan: only Ticket + the normalized TargetBranch.
	if nm.Plan().Ticket != "standalone-main" {
		t.Errorf("standalone delta Ticket = %q, want %q", nm.Plan().Ticket, "standalone-main")
	}
	if nm.Plan().TargetBranch != "main" {
		t.Errorf("standalone delta TargetBranch = %q, want %q", nm.Plan().TargetBranch, "main")
	}
	// No stale field survives the reset.
	if got := nm.Plan().PackageXMLPath; got != "" {
		t.Errorf("stale PackageXMLPath must not survive standalone delta, got %q", got)
	}
	if got := nm.Plan().DestructiveChangesPath; got != "" {
		t.Errorf("stale DestructiveChangesPath must not survive standalone delta, got %q", got)
	}
	if len(nm.Plan().SelectedCommits) != 0 {
		t.Errorf("stale SelectedCommits must not survive standalone delta, got %v", nm.Plan().SelectedCommits)
	}
	if got := nm.Plan().PromotionBranch; got != "" {
		t.Errorf("stale PromotionBranch must not survive standalone delta, got %q", got)
	}
	if got := nm.Plan().SandboxAlias; got != "" {
		t.Errorf("stale SandboxAlias must not survive standalone delta, got %q", got)
	}

	if nm.State() != StateDeltaGeneration {
		t.Fatalf("confirm should enter StateDeltaGeneration, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("confirm should fire deltaCmd")
	}
}

// TestModel_DeltaSourceSelect_TicketIncludesSanitizedBase is task 3.2 (RED):
// design D2's identity fix — confirmDeltaSourceSelect folds the picked
// (normalized) base branch into the run's Ticket (Ticket = "standalone-" +
// sanitizeBranch(base)) instead of the flat literal "standalone", so
// standalone-delta runs from different bases produce distinct identities
// (standalone-modes spec: "Standalone Runs Are Distinguishable And
// Collision-Free").
func TestModel_DeltaSourceSelect_TicketIncludesSanitizedBase(t *testing.T) {
	branches := []git.Branch{
		{Name: "main", Remote: false},
		{Name: "origin/release/1.0", Remote: true},
	}
	m := deltaSourceSelectModel(branches, 1)

	next, _ := m.Update(keyPress("enter"))
	nm := next.(Model)

	if nm.Plan().Ticket != "standalone-release-1.0" {
		t.Fatalf("standalone delta Ticket = %q, want %q", nm.Plan().Ticket, "standalone-release-1.0")
	}
	if nm.Plan().TargetBranch != "release/1.0" {
		t.Fatalf("TargetBranch should keep the normalized base unchanged, got %q", nm.Plan().TargetBranch)
	}
}

// TestModel_DeltaSourceSelect_CursorNav is task 3.3's cursor-nav companion
// (triangulation): up/down/k/j move branchCursor, clamped to [0, len-1].
func TestModel_DeltaSourceSelect_CursorNav(t *testing.T) {
	branches := []git.Branch{{Name: "main"}, {Name: "origin/main", Remote: true}, {Name: "origin/UAT", Remote: true}}
	m := deltaSourceSelectModel(branches, 0)

	m = advance(t, m, keyPress("down"))
	if m.branchCursor != 1 {
		t.Fatalf("down should move cursor to 1, got %d", m.branchCursor)
	}
	m = advance(t, m, keyPress("j"))
	if m.branchCursor != 2 {
		t.Fatalf("j should move cursor to 2, got %d", m.branchCursor)
	}
	if advance(t, m, keyPress("down")).branchCursor != 2 {
		t.Error("down at the last branch should clamp at 2")
	}
	m = advance(t, m, keyPress("up"))
	if m.branchCursor != 1 {
		t.Fatalf("up should move cursor to 1, got %d", m.branchCursor)
	}
	m = advance(t, m, keyPress("k"))
	if m.branchCursor != 0 {
		t.Fatalf("k should move cursor to 0, got %d", m.branchCursor)
	}
	if advance(t, m, keyPress("up")).branchCursor != 0 {
		t.Error("up at the first branch should clamp at 0")
	}
}

// TestModel_PackageReview_DeltaModeStopsAfterSummary is task 3.5 (RED):
// design ADR-3's mode-aware stop. When standaloneMode=="delta", the shared
// PackageReview screen's `enter` is a terminal no-op (no cherry-picks were
// ever made, so there is nothing to queue/validate — `confirmPackageReview`
// / StateQueueReview must never fire) and `e` (edit selection) is neutralized
// too, since standalone delta never ran commit selection. The footer must
// omit "Enter validar".
func TestModel_PackageReview_DeltaModeStopsAfterSummary(t *testing.T) {
	m := reviewedModel(t, Deps{Dir: "/repo", Config: validationConfig()}, false)
	m.standaloneMode = "delta"

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StatePackageReview {
		t.Fatalf("standalone delta review enter must stay terminal (no queue/validate), got %v", nm.State())
	}
	if cmd != nil {
		t.Error("standalone delta review enter must not fire any command")
	}

	next2, cmd2 := nm.Update(keyPress("e"))
	nm2 := next2.(Model)
	if nm2.State() != StatePackageReview {
		t.Fatalf("standalone delta review 'e' must be neutralized (no commit selection to edit), got %v", nm2.State())
	}
	if cmd2 != nil {
		t.Error("neutralized 'e' must not fire any command")
	}

	if strings.Contains(nm.View(), "Enter validar") {
		t.Errorf("standalone delta footer must omit 'Enter validar':\n%s", nm.View())
	}
}

// TestModel_PackageReview_FullFlow_StillReachesQueueReview is task 3.5's
// triangulation companion: with standaloneMode=="" (the unchanged full
// flow), `enter` on PackageReview MUST still reach StateQueueReview and fire
// queueCmd — proving the ADR-3 mode fork never touches the full-flow path.
// (Pre-existing coverage already exercises this via
// TestModel_PackageReview_EmptyBlocksUntilOverride/queue_review_test.go; this
// test pins the exact non-empty happy path explicitly for HU-018.)
func TestModel_PackageReview_FullFlow_StillReachesQueueReview(t *testing.T) {
	deps := Deps{Dir: "/repo", Config: validationConfig(), SF: exec_sf(t), Runs: runs.NewWriter(t.TempDir()), Now: (&fakeClock{t: time.Unix(0, 0)}).now}
	m := reviewedModel(t, deps, false)

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateQueueReview {
		t.Fatalf("full-flow review enter must still reach StateQueueReview, got %v", nm.State())
	}
	if cmd == nil {
		t.Fatal("entering QueueReview should fire queueCmd")
	}
}

// TestOnDeltaDone_StandaloneDelta_SavesRunModeDelta is task 3.7 (RED): design
// ADR-4's delta-mode run creation. When standaloneMode=="delta", onDeltaDone
// persists a local run — mirroring onBranchCreated's best-effort, nil-Runs-
// safe save — tagged Mode="delta" with ManifestPath set to the generated
// package.xml and NO jobId (standalone delta never validates). AC:
// "Standalone Modes Create A Local Run" (delta scenario).
func TestOnDeltaDone_StandaloneDelta_SavesRunModeDelta(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	m := New(Deps{Dir: dir, Config: validationConfig(), Runs: writer, Now: (&fakeClock{t: time.Unix(0, 0)}).now})
	m.state = StateDeltaGeneration
	m.standaloneMode = "delta"
	m.plan.Ticket = "standalone"
	m.plan.TargetBranch = "main"

	result := delta.Result{PackageXMLPath: filepath.Join(dir, "package.xml")}
	summary := delta.PackageSummary{Types: []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 1}}}

	next, _ := m.Update(deltaDoneMsg{result: result, summary: summary})
	nm := next.(Model)
	if nm.State() != StatePackageReview {
		t.Fatalf("standalone delta should still reach PackageReview, got %v", nm.State())
	}

	records, err := writer.List()
	if err != nil {
		t.Fatalf("writer.List(): %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly one persisted run, got %d", len(records))
	}
	rec := records[0]
	if rec.Mode != "delta" {
		t.Errorf("expected Mode=%q, got %q", "delta", rec.Mode)
	}
	if rec.ManifestPath != result.PackageXMLPath {
		t.Errorf("expected ManifestPath=%q, got %q", result.PackageXMLPath, rec.ManifestPath)
	}
	if rec.JobID != "" {
		t.Errorf("a standalone delta run must not carry a jobId, got %q", rec.JobID)
	}
}

// TestModel_StandaloneDelta_EmptyDelta_ReusesWarning is task 3.9 (pure-reuse
// pin, already-GREEN): standalone delta renders the EXACT SAME HU-007/008
// empty-delta warning as the full flow — the summary.Empty block in
// viewPackageReview is unconditional on standaloneMode, so nothing new is
// built for this case. AC: "Empty delta reuses the existing warning".
func TestModel_StandaloneDelta_EmptyDelta_ReusesWarning(t *testing.T) {
	m := reviewedModel(t, Deps{Dir: "/repo", Config: validationConfig()}, true) // empty=true
	m.standaloneMode = "delta"

	v := m.View()
	if !strings.Contains(v, "Package vacio") {
		t.Errorf("standalone delta should reuse the existing empty-delta warning verbatim, got:\n%s", v)
	}
}
