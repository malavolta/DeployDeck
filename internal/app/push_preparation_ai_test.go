package app

import (
	"context"
	"strings"
	"testing"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
)

// aiDep returns a fake Deps.GenerateSummary that always drafts title/desc,
// recording each invocation's ticket for TestModel_..._NeverAutoFiresAfterPush.
func aiDep(title, description string, calls *int) func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
	return func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		if calls != nil {
			*calls++
		}
		return title, description, nil
	}
}

// TestModel_PushPreparation_AITitle_ReachesGhOnlyAfterSecondAccept is task
// 4.12 (RED+GREEN, mirrors push_preparation_test.go) and doubles as task
// 5.2's threat-matrix proof: the AI title reaches createPRCmd's --title
// argument ONLY after the explicit second 'a' accept. Requesting a
// suggestion once (first 'a') and confirming PR creation WITHOUT accepting
// must still use the pre-existing github.SuggestedTitle formula.
func TestModel_PushPreparation_AITitle_ReachesGhOnlyAfterSecondAccept(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	formulaTitle := github.SuggestedTitle("PROJ-1", "UAT")
	aiTitle := "PROJ-1 - AI drafted title"

	t.Run("requested but not accepted keeps the formula title on gh pr create", func(t *testing.T) {
		fr := ghRunner("authed")
		cannPRCreate(fr, "UAT", branch, formulaTitle, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})

		m := New(Deps{
			Dir:             "/repo",
			Config:          testConfig(),
			GH:              github.New(fr),
			GenerateSummary: aiDep(aiTitle, "AI drafted description", nil),
		})
		m.state = StatePushPreparation
		m.pushPhase = pushReady
		m.authState = github.AuthAuthenticated
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

		// First 'a': request only.
		next, cmd := m.Update(keyPress("a"))
		nm := next.(Model)
		if !nm.aiPending {
			t.Fatal("first 'a' should set aiPending")
		}
		msg := cmd()
		next2, _ := nm.Update(msg)
		nm2 := next2.(Model)
		if nm2.aiTitle != aiTitle || nm2.aiAccepted {
			t.Fatalf("expected a generated, unaccepted suggestion, got aiTitle=%q aiAccepted=%v", nm2.aiTitle, nm2.aiAccepted)
		}

		// g -> y confirms PR creation WITHOUT ever pressing 'a' a second time.
		next3, _ := nm2.Update(keyPress("g"))
		next4, cmd4 := next3.(Model).Update(keyPress("y"))
		if cmd4 == nil {
			t.Fatal("confirming should fire createPRCmd")
		}
		cmd4()
		_ = next4

		if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", formulaTitle, "--body", "") {
			t.Fatalf("gh pr create should have used the formula title %q (never accepted), calls: %v", formulaTitle, fr.Calls)
		}
		if calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", aiTitle, "--body", "") {
			t.Fatal("gh pr create must NOT use the AI title before an explicit accept")
		}
	})

	t.Run("requested then accepted (second a) sends the AI title as one discrete arg", func(t *testing.T) {
		fr := ghRunner("authed")
		cannPRCreate(fr, "UAT", branch, aiTitle, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/2\n")})

		m := New(Deps{
			Dir:             "/repo",
			Config:          testConfig(),
			GH:              github.New(fr),
			GenerateSummary: aiDep(aiTitle, "AI drafted description", nil),
		})
		m.state = StatePushPreparation
		m.pushPhase = pushReady
		m.authState = github.AuthAuthenticated
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

		// First 'a': request.
		next, cmd := m.Update(keyPress("a"))
		msg := cmd()
		next2, _ := next.(Model).Update(msg)
		nm2 := next2.(Model)

		// Second 'a': accept — the ONLY place aiAccepted is ever set.
		next3, cmd3 := nm2.Update(keyPress("a"))
		nm3 := next3.(Model)
		if !nm3.aiAccepted {
			t.Fatal("second 'a' should accept the suggestion")
		}
		if cmd3 != nil {
			t.Fatal("accepting must not itself fire a new command")
		}

		// Still requires the pre-existing explicit PR-creation confirmation
		// gate (g then y) — acceptance alone must never create a PR.
		if n := prCreateCalls(fr); n != 0 {
			t.Fatalf("accepting a title must not itself create a PR, got %d gh pr create call(s)", n)
		}

		next4, _ := nm3.Update(keyPress("g"))
		next5, cmd5 := next4.(Model).Update(keyPress("y"))
		if cmd5 == nil {
			t.Fatal("confirming should fire createPRCmd")
		}
		prMsg := cmd5()
		_, _ = next5.(Model).Update(prMsg)

		if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", aiTitle, "--body", "") {
			t.Fatalf("gh pr create should use the accepted AI title %q as ONE discrete arg, calls: %v", aiTitle, fr.Calls)
		}
		if n := prCreateCalls(fr); n != 1 {
			t.Fatalf("expected exactly 1 gh pr create call, got %d", n)
		}
	})
}

// TestModel_PushPreparation_AISuggestion_NeverAutoFiresAfterPush is task
// 5.2/spec's "Suggestion never auto-fires after push" proof: landing on
// pushReady right after a successful push must never call
// Deps.GenerateSummary on its own.
func TestModel_PushPreparation_AISuggestion_NeverAutoFiresAfterPush(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	var calls int

	m := New(Deps{
		Dir:             "/repo",
		Config:          testConfig(),
		Git:             pushGit("/repo", branch, "git@github.com:org/repo.git", true),
		GH:              github.New(ghRunner("authed")),
		GenerateSummary: aiDep("PROJ-1 - AI drafted title", "", &calls),
	})
	m.state = StatePushPreparation
	m.pushPhase = pushConfirm
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(keyPress("p"))
	pushMsg := cmd()
	next2, cmd2 := next.(Model).Update(pushMsg)
	prepMsg := cmd2()
	next3, _ := next2.(Model).Update(prepMsg)
	nm3 := next3.(Model)

	if nm3.pushPhase != pushReady {
		t.Fatalf("expected to land on pushReady, got %v", nm3.pushPhase)
	}
	if calls != 0 {
		t.Fatalf("Deps.GenerateSummary must never be called automatically after push, got %d call(s)", calls)
	}
	if nm3.aiTitle != "" || nm3.aiPending {
		t.Fatalf("no AI request should be in flight or completed without an explicit 'a', got aiTitle=%q aiPending=%v", nm3.aiTitle, nm3.aiPending)
	}
	if !strings.Contains(nm3.View(), "a solicitar") {
		t.Errorf("pushReady should offer the request affordance, not an auto-fired suggestion\n%s", nm3.View())
	}
}
