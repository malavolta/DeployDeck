package prereq_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
		GitRoot:       "/repo",
		ArtifactsRoot: "/repo",
		Git:           git.New(gitRunner),
		SF:            salesforce.New(sfRunner),
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

// deadlineRunner records the gh probe's args and deadline, and answers with
// a canned result/error.
type deadlineRunner struct {
	args     []string
	deadline time.Time
	hasDL    bool
	result   exec.CommandResult
	err      error
}

func (r *deadlineRunner) Run(ctx context.Context, req exec.CommandRequest) (exec.CommandResult, error) {
	r.args = req.Args
	r.deadline, r.hasDL = ctx.Deadline()
	return r.result, r.err
}

func originGit(origin string) *git.Service {
	g := exec.NewFakeRunner()
	g.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{Stdout: []byte("/repo\n")})
	g.When("git", []string{"remote", "get-url", "origin"}, exec.CommandResult{Stdout: []byte(origin + "\n")})
	return git.New(g)
}

func TestChecker_CheckGH_ProbeScopeAndTimeout(t *testing.T) {
	tests := []struct {
		name     string
		git      *git.Service
		wantArgs string
	}{
		{"origin host scoped", originGit("git@github.com:org/repo.git"), "auth status --hostname=github.com"},
		{"unsafe origin host falls back", originGit("https://--help/o/r"), "auth status"},
		{"no origin falls back", git.New(exec.NewFakeRunner()), "auth status"},
		{"nil git falls back", nil, "auth status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &deadlineRunner{result: exec.CommandResult{ExitCode: 0}}
			checker := &prereq.Checker{GH: github.New(r), Git: tt.git, GitRoot: "/repo", GHTimeout: 2 * time.Second}
			before := time.Now()
			check, _ := checker.CheckGH(context.Background())
			if got := strings.Join(r.args, " "); got != tt.wantArgs {
				t.Fatalf("args = %q, want %q", got, tt.wantArgs)
			}
			if check.Status != prereq.StatusOK {
				t.Fatalf("want OK, got %+v", check)
			}
			if !r.hasDL || r.deadline.After(before.Add(2*time.Second+time.Second)) {
				t.Fatalf("probe deadline %v not bounded by GHTimeout", r.deadline)
			}
		})
	}
}

func TestChecker_CheckGH_NonAuthStatesExplainConsequences(t *testing.T) {
	tests := []struct {
		name       string
		runner     *deadlineRunner
		want       string
		wantNotSay string
	}{
		{"absent", &deadlineRunner{err: errors.New("not found")}, "not installed", ""},
		{"unauthenticated", &deadlineRunner{result: exec.CommandResult{ExitCode: 1}}, "not authenticated", ""},
		{"timed out", &deadlineRunner{err: context.DeadlineExceeded}, "could not verify", "not authenticated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &prereq.Checker{GH: github.New(tt.runner)}
			check, _ := checker.CheckGH(context.Background())
			if check.Status != prereq.StatusWarning {
				t.Fatalf("want warning, got %+v", check)
			}
			for _, w := range []string{tt.want, "compare URL", "deploy gate", "validation comment", "push still works"} {
				if !strings.Contains(check.Detail, w) {
					t.Errorf("Detail %q missing %q", check.Detail, w)
				}
			}
			if tt.wantNotSay != "" && strings.Contains(check.Detail, tt.wantNotSay) {
				t.Errorf("Detail %q must not say %q", check.Detail, tt.wantNotSay)
			}
		})
	}
}
