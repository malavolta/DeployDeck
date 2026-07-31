package prereq_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/malavolta/DeployDeck/internal/ai"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// TestChecker_CheckAI_NilAI_SkipsAndReportsOK is task 3.1 (RED)'s nil-guard
// (mirrors checker_gh_test.go's TestChecker_CheckGH_NilGH_SkipsAndReportsOK):
// a Checker with AI left nil (every pre-ai-pr-summary construction, and
// every existing checker test that never sets AI) must not panic and must
// report OK, never blocking (prereq-check delta: "Absent config or nil
// client skips the check").
func TestChecker_CheckAI_NilAI_SkipsAndReportsOK(t *testing.T) {
	checker := &prereq.Checker{}

	check, err := checker.CheckAI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected a nil AI to report OK, got %+v", check)
	}
}

// TestChecker_CheckAI_Ready_OK proves a reachable endpoint listing the
// configured model reports OK.
func TestChecker_CheckAI_Ready_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen2.5-coder:3b"}]}`))
	}))
	defer srv.Close()

	checker := &prereq.Checker{AI: ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())}

	check, err := checker.CheckAI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected a ready AI endpoint to report OK, got %+v", check)
	}
}

// TestChecker_CheckAI_ModelMissing_Warning proves a reachable endpoint that
// does not list the configured model reports a non-blocking warning
// (prereq-check delta: "Reachable endpoint with configured model missing").
func TestChecker_CheckAI_ModelMissing_Warning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"llama3"}]}`))
	}))
	defer srv.Close()

	checker := &prereq.Checker{AI: ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())}

	check, err := checker.CheckAI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusWarning {
		t.Fatalf("expected a missing-model AI endpoint to report warning, got %+v", check)
	}
}

// TestChecker_CheckAI_Unreachable_Warning_NeverBlocks proves an unreachable
// endpoint is reported informationally, never blocking (prereq-check delta:
// "Unreachable endpoint is reported informationally", "AI check never
// blocks the flow").
func TestChecker_CheckAI_Unreachable_Warning_NeverBlocks(t *testing.T) {
	checker := &prereq.Checker{AI: ai.New("http://127.0.0.1:1", "qwen2.5-coder:3b", &http.Client{})}

	check, err := checker.CheckAI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status == prereq.StatusBlocking {
		t.Fatalf("AI unreachable must never block, got %+v", check)
	}
	if check.Status != prereq.StatusWarning {
		t.Fatalf("expected unreachable AI to report warning, got %+v", check)
	}
}

// TestChecker_Check_NilAI_IncludesSkippedAICheck is task 3.4 (RED)'s
// regression proof (mirrors TestChecker_Check_NilGH_IncludesSkippedGHCheck):
// existing checker tests (which never set AI) must stay green — Check()
// itself must not panic or error with AI nil, and it must still include an
// "AI model" entry in the aggregated report.
func TestChecker_Check_NilAI_IncludesSkippedAICheck(t *testing.T) {
	gitRunner := exec.NewFakeRunner() // every git call "fails to start"; only the AI check is asserted

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
		t.Fatalf("Check() with nil AI must not error: %v", err)
	}

	var aiCheck *prereq.PrereqCheck
	for i := range checks {
		if checks[i].Name == "AI model" {
			aiCheck = &checks[i]
		}
	}
	if aiCheck == nil {
		t.Fatal("expected Check() to include the AI model check even with AI nil")
	}
	if aiCheck.Status != prereq.StatusOK {
		t.Fatalf("expected the skipped AI check to be OK, got %+v", aiCheck)
	}
}
