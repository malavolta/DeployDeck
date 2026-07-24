package git_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestHU006_CherryPick_E2E is the consolidated HU-006 Test E2E from
// docs/HISTORIAS.md: it drives the cherry-pick engine end-to-end on a REAL
// temp git repo (no org) through pick -> conflict detection/classification ->
// resolve -> --continue -> post-pick verification, plus every named variant
// (text/binary/modify-delete conflicts, empty-pick --skip, mid-sequence abort
// with partial-branch cleanup, and external --continue/--abort
// reconciliation). Real git throughout — no FakeRunner for the pick itself.
func TestHU006_CherryPick_E2E(t *testing.T) {
	// Primary: 2-commit clean promotion onto a real temp branch. The branch
	// contains both changes, content equals the source, order is topological,
	// and downstream steps are unblocked.
	t.Run("clean promotion: both commits, content==source, topological order", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature1, feature2 := seedCleanFeature(t, runner, dir)

		// Real temp promotion branch off origin/UAT (HU-005 primitive).
		if err := svc.CreatePromotionBranch(context.Background(), dir, "UAT", "deploy/PROJ-1-to-UAT"); err != nil {
			t.Fatalf("CreatePromotionBranch error: %v", err)
		}

		commits := []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: feature1}},
			{Commit: git.Commit{SHA: feature2}},
		}
		outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
		if err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		if outcome.State.InProgress {
			t.Fatalf("expected a clean promotion to complete, got %+v", outcome.State)
		}

		// Topological order: A applied before B.
		log := string(runGit(t, runner, dir, "log", "--format=%s", "-2").Stdout)
		if strings.Index(log, "add A") < strings.Index(log, "add B") {
			t.Errorf("expected topological order (A before B) in log:\n%s", log)
		}

		// Content equals the selected commits for all touched files, and no
		// unselected file was dragged onto the branch.
		verify, err := svc.VerifyPromotedContent(context.Background(), dir, "origin/UAT", feature2, []string{"a.cls", "b.cls"})
		if err != nil {
			t.Fatalf("VerifyPromotedContent error: %v", err)
		}
		if !verify.OK() {
			t.Errorf("expected promoted content == source, partial files: %v, spurious files: %v", verify.PartialFiles, verify.SpuriousFiles)
		}

		if !git.DeltaAndValidationAllowed(outcome.State, false) {
			t.Errorf("a clean completed promotion must allow delta + validation")
		}
	})

	// Text conflict full lifecycle: stop -> classify -> external resolve is
	// reflected by re-reading the repo (no TUI action) -> gated
	// non-interactive --continue -> completion.
	t.Run("text conflict: stop, classify, external resolve reconciled, gated continue", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature1, feature2 := seedConflictingFeature(t, runner, dir)
		commits := []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: feature1}},
			{Commit: git.Commit{SHA: feature2}},
		}
		outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
		if err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}

		// Flow stopped with a classified text conflict.
		if !outcome.State.InProgress || len(outcome.State.Unmerged) != 1 || outcome.State.Unmerged[0].Kind != git.ConflictText {
			t.Fatalf("expected a stopped flow with one text conflict, got %+v", outcome.State)
		}

		// While unmerged, continue is disabled and downstream is blocked.
		if git.EvaluateContinueGate(outcome.State, nil).Enabled {
			t.Errorf("continue must be disabled while a conflict is unresolved")
		}
		if git.DeltaAndValidationAllowed(outcome.State, false) {
			t.Errorf("a failed/in-progress cherry-pick must block delta + validation")
		}

		// External resolution (no TUI action): the next repo re-read reflects
		// it automatically.
		writeFileHelper(t, dir, "a.cls", "l1\nRESOLVED\nl3\n")
		runGit(t, runner, dir, "add", "a.cls")
		reread, _ := svc.RepoState(context.Background(), dir)
		if len(reread.Unmerged) != 0 {
			t.Fatalf("expected the external resolution reflected on re-read, still unmerged: %+v", reread.Unmerged)
		}
		if !git.EvaluateContinueGate(reread, nil).Enabled {
			t.Errorf("continue should be enabled once the conflict is resolved and staged")
		}

		// Non-interactive gated continue completes the sequence.
		done, err := svc.ContinueCherryPick(context.Background(), dir)
		if err != nil {
			t.Fatalf("ContinueCherryPick error: %v", err)
		}
		if done.State.InProgress {
			t.Errorf("expected the sequence to complete after continue, got %+v", done.State)
		}
	})

	// Binary conflict variant: classified binary, resolved via --theirs.
	t.Run("binary conflict classified and resolved via theirs", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature, _ := seedBinaryConflict(t, runner, dir)
		outcome, err := svc.CherryPick(context.Background(), dir, []git.DiscoveredCommit{{Commit: git.Commit{SHA: feature}}}, true)
		if err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		if len(outcome.State.Unmerged) != 1 || outcome.State.Unmerged[0].Kind != git.ConflictBinary {
			t.Fatalf("expected a binary conflict, got %+v", outcome.State.Unmerged)
		}
		if err := svc.ResolveBinaryConflict(context.Background(), dir, "img.png", git.SideTheirs); err != nil {
			t.Fatalf("ResolveBinaryConflict error: %v", err)
		}
		if state, _ := svc.RepoState(context.Background(), dir); len(state.Unmerged) != 0 {
			t.Errorf("expected the binary conflict resolved, got %+v", state.Unmerged)
		}
	})

	// Modify/delete variant: classified modify/delete, resolved via keep.
	t.Run("modify-delete conflict classified and kept", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature := seedModifyDeleteConflict(t, runner, dir)
		outcome, err := svc.CherryPick(context.Background(), dir, []git.DiscoveredCommit{{Commit: git.Commit{SHA: feature}}}, true)
		if err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		if len(outcome.State.Unmerged) != 1 || outcome.State.Unmerged[0].Kind != git.ConflictModifyDelete {
			t.Fatalf("expected a modify/delete conflict, got %+v", outcome.State.Unmerged)
		}
		if err := svc.KeepConflictFile(context.Background(), dir, "f.cls"); err != nil {
			t.Fatalf("KeepConflictFile error: %v", err)
		}
		if state, _ := svc.RepoState(context.Background(), dir); len(state.Unmerged) != 0 {
			t.Errorf("expected the modify/delete conflict resolved, got %+v", state.Unmerged)
		}
	})

	// Empty-pick variant: content already present -> detected -> --skip.
	t.Run("empty pick detected and skipped", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature1, feature2 := seedEmptyThenClean(t, runner, dir)
		commits := []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: feature1}},
			{Commit: git.Commit{SHA: feature2}},
		}
		outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
		if err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		if !outcome.Empty {
			t.Fatalf("expected an empty pick, got %+v", outcome)
		}
		done, err := svc.SkipCherryPick(context.Background(), dir)
		if err != nil {
			t.Fatalf("SkipCherryPick error: %v", err)
		}
		if done.State.InProgress {
			t.Errorf("expected the sequence to complete after skipping, got %+v", done.State)
		}
	})

	// Abort mid-sequence: partial picks applied -> cleanup offered -> abort.
	t.Run("mid-sequence abort offers partial branch cleanup", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature1, feature2 := seedPartialThenConflict(t, runner, dir)
		commits := []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: feature1}},
			{Commit: git.Commit{SHA: feature2}},
		}
		if _, err := svc.CherryPick(context.Background(), dir, commits, true); err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		applied, err := svc.AppliedPickCount(context.Background(), dir, "origin/UAT")
		if err != nil {
			t.Fatalf("AppliedPickCount error: %v", err)
		}
		if !git.OfferPartialBranchCleanup(applied) {
			t.Fatalf("expected a partial-cleanup offer, applied=%d", applied)
		}
		if err := svc.AbortCherryPick(context.Background(), dir); err != nil {
			t.Fatalf("AbortCherryPick error: %v", err)
		}
		if state, _ := svc.RepoState(context.Background(), dir); state.InProgress {
			t.Errorf("expected not-in-progress after abort, got %+v", state)
		}
	})

	// External abort reconciliation: a --abort run outside DeployDeck is
	// detected on the next re-read.
	t.Run("external abort reconciled on re-read", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature1, feature2 := seedConflictingFeature(t, runner, dir)
		commits := []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: feature1}},
			{Commit: git.Commit{SHA: feature2}},
		}
		if _, err := svc.CherryPick(context.Background(), dir, commits, true); err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		// External, not through the Service.
		runGit(t, runner, dir, "cherry-pick", "--abort")
		state, _ := svc.RepoState(context.Background(), dir)
		if state.InProgress || state.SequencerRemaining != 0 {
			t.Errorf("expected the external abort reconciled on re-read, got %+v", state)
		}
	})

	// Post-pick verification detects a partial promotion when a file was
	// resolved differently than the source branch.
	t.Run("post-pick verification detects partial promotion", func(t *testing.T) {
		dir := newTempRepo(t)
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		feature1, feature2 := seedConflictingFeature(t, runner, dir)
		commits := []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: feature1}},
			{Commit: git.Commit{SHA: feature2}},
		}
		if _, err := svc.CherryPick(context.Background(), dir, commits, true); err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}
		writeFileHelper(t, dir, "a.cls", "l1\nDIVERGENT\nl3\n")
		runGit(t, runner, dir, "add", "a.cls")
		if _, err := svc.ContinueCherryPick(context.Background(), dir); err != nil {
			t.Fatalf("ContinueCherryPick error: %v", err)
		}
		verify, err := svc.VerifyPromotedContent(context.Background(), dir, "origin/UAT", feature2, []string{"a.cls", "b.cls"})
		if err != nil {
			t.Fatalf("VerifyPromotedContent error: %v", err)
		}
		if verify.OK() || len(verify.Warnings()) == 0 {
			t.Errorf("expected a per-file partial-promotion warning, got %+v", verify)
		}
	})
}
