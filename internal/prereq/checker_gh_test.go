package prereq_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// TestChecker_CheckGH_NilGH_SkipsAndReportsOK is task 4.1 (RED)'s
// nil-guard: a Checker with GH left nil (every pre-HU-014 construction, and
// every existing checker test) must not panic and must report OK, never
// blocking.
func TestChecker_CheckGH_NilGH_SkipsAndReportsOK(t *testing.T) {
	checker := &prereq.Checker{}

	check, err := checker.CheckGH(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected a nil GH to report OK, got %+v", check)
	}
}

// TestChecker_CheckGH_Absent_NeverBlocks is task 4.1 (RED): gh absent is
// reported informationally and never blocks (prereq-check delta: "gh absent
// is reported informationally", "gh check never blocks the flow").
func TestChecker_CheckGH_Absent_NeverBlocks(t *testing.T) {
	runner := exec.NewFakeRunner() // no canned "gh auth status" response -> Runner error -> AuthAbsent
	checker := &prereq.Checker{GH: github.New(runner)}

	check, err := checker.CheckGH(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status == prereq.StatusBlocking {
		t.Fatalf("gh absent must never block, got %+v", check)
	}
	if check.FixCommand == "" {
		t.Fatal("expected a FixCommand suggesting gh installation")
	}
}

// TestChecker_CheckGH_Unauthenticated_NeverBlocks is task 4.1 (RED): gh
// present but unauthenticated is reported informationally and never blocks.
func TestChecker_CheckGH_Unauthenticated_NeverBlocks(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("gh", []string{"auth", "status"}, exec.CommandResult{ExitCode: 1})
	checker := &prereq.Checker{GH: github.New(runner)}

	check, err := checker.CheckGH(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status == prereq.StatusBlocking {
		t.Fatalf("gh unauthenticated must never block, got %+v", check)
	}
	if check.FixCommand == "" {
		t.Fatal("expected a FixCommand suggesting gh auth login")
	}
}

// TestChecker_CheckGH_Authenticated_OK is task 4.1 (RED): gh present and
// authenticated is reported OK.
func TestChecker_CheckGH_Authenticated_OK(t *testing.T) {
	runner := exec.NewFakeRunner()
	runner.When("gh", []string{"auth", "status"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("Logged in to github.com as octocat")})
	checker := &prereq.Checker{GH: github.New(runner)}

	check, err := checker.CheckGH(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected authenticated gh to be OK, got %+v", check)
	}
}

// TestChecker_Check_NilGH_IncludesSkippedGHCheck is task 4.1 (RED)'s
// regression proof: existing checker tests (which never set GH) must stay
// green — Check() itself must not panic or error with GH nil, and it must
// still include a "gh CLI" entry in the aggregated report.
func TestChecker_Check_NilGH_IncludesSkippedGHCheck(t *testing.T) {
	gitRunner := exec.NewFakeRunner() // every git call "fails to start"; only the gh CLI check is asserted

	sfRunner := exec.NewFakeRunner()
	sfRunner.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n")})
	sfRunner.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`[{"name":"sfdx-git-delta","version":"5.35.0","children":[]}]`)})
	sfRunner.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(`{"status":0,"result":{}}`)})

	checker := &prereq.Checker{
		Dir: "/repo",
		Git: git.New(gitRunner),
		SF:  salesforce.New(sfRunner),
	}

	checks, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() with nil GH must not error: %v", err)
	}

	var ghCheck *prereq.PrereqCheck
	for i := range checks {
		if checks[i].Name == "gh CLI" {
			ghCheck = &checks[i]
		}
	}
	if ghCheck == nil {
		t.Fatal("expected Check() to include the gh CLI check even with GH nil")
	}
	if ghCheck.Status != prereq.StatusOK {
		t.Fatalf("expected the skipped gh check to be OK, got %+v", ghCheck)
	}
}
