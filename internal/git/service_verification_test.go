package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_VerifyPromotedContent_DetectsPartialPromotion pins AC10 (tasks
// 10.33/10.34): after all picks complete, a touched file whose final content
// differs from the source branch is reported as a per-file partial-promotion
// warning; a file matching the source is not.
func TestService_VerifyPromotedContent_DetectsPartialPromotion(t *testing.T) {
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

	// Resolve a.cls to content that does NOT match the selected commits — a
	// deliberate partial promotion. b.cls was applied cleanly (matches).
	writeFileHelper(t, dir, "a.cls", "l1\nPARTIAL-DIFFERENT\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	if _, err := svc.ContinueCherryPick(context.Background(), dir); err != nil {
		t.Fatalf("ContinueCherryPick error: %v", err)
	}

	verify, err := svc.VerifyPromotedContent(context.Background(), dir, "origin/UAT", feature2, []string{"a.cls", "b.cls"})
	if err != nil {
		t.Fatalf("VerifyPromotedContent error: %v", err)
	}
	if verify.OK() {
		t.Fatalf("expected a partial-promotion result, got OK")
	}
	if len(verify.PartialFiles) != 1 || verify.PartialFiles[0] != "a.cls" {
		t.Fatalf("expected only a.cls flagged partial, got %v", verify.PartialFiles)
	}
}

// TestService_VerifyPromotedContent_CleanPromotionMatchesSource confirms the
// happy path: a clean pick whose files equal the source yields OK.
func TestService_VerifyPromotedContent_CleanPromotionMatchesSource(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, feature2 := seedCleanFeature(t, runner, dir)
	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: feature1}},
		{Commit: git.Commit{SHA: feature2}},
	}
	if _, err := svc.CherryPick(context.Background(), dir, commits, true); err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}

	verify, err := svc.VerifyPromotedContent(context.Background(), dir, "origin/UAT", feature2, []string{"a.cls", "b.cls"})
	if err != nil {
		t.Fatalf("VerifyPromotedContent error: %v", err)
	}
	if !verify.OK() {
		t.Errorf("expected a clean promotion to equal the source, got partial files %v", verify.PartialFiles)
	}
}
