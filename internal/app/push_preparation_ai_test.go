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
// argument either via the explicit second 'a' accept on pushReady, OR via the
// never-silently-skip explicit-choice `y` at pushPRConfirm (which itself
// performs the accept before creating) — never implicitly. Requesting a
// suggestion once (first 'a') and then explicitly DECLINING it with `d` at
// the PR-confirm choice must still use the pre-existing github.SuggestedTitle
// formula.
func TestModel_PushPreparation_AITitle_ReachesGhOnlyAfterSecondAccept(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	formulaTitle := github.SuggestedTitle("PROJ-1", "UAT")
	aiTitle := "PROJ-1 - AI drafted title"

	t.Run("requested but explicitly declined (d) keeps the formula title on gh pr create", func(t *testing.T) {
		fr := ghRunner("authed")
		cannPRCreate(fr, "UAT", branch, formulaTitle, "", execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})

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

		// g reveals the explicit AI choice (never-silently-skip fix); d
		// explicitly declines the suggestion and creates with the formula
		// title.
		next3, _ := nm2.Update(keyPress("g"))
		next4, cmd4 := next3.(Model).Update(keyPress("d"))
		if cmd4 == nil {
			t.Fatal("confirming should fire createPRCmd")
		}
		cmd4()
		_ = next4

		if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", formulaTitle, "--body", "") {
			t.Fatalf("gh pr create should have used the formula title %q (explicitly declined), calls: %v", formulaTitle, fr.Calls)
		}
		if calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", aiTitle, "--body", "") {
			t.Fatal("gh pr create must NOT use the AI title after an explicit decline")
		}
	})

	t.Run("requested then accepted (second a) sends the AI title and description as discrete args", func(t *testing.T) {
		fr := ghRunner("authed")
		aiDescription := "AI drafted description"
		cannPRCreate(fr, "UAT", branch, aiTitle, aiDescription, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/2\n")})

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

		if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", aiTitle, "--body", aiDescription) {
			t.Fatalf("gh pr create should use the accepted AI title %q and description %q as discrete args, calls: %v", aiTitle, aiDescription, fr.Calls)
		}
		if n := prCreateCalls(fr); n != 1 {
			t.Fatalf("expected exactly 1 gh pr create call, got %d", n)
		}
	})
}

// TestModel_PushPreparation_AISuggestion_AutoFiresOnPushReady supersedes the
// former "never auto-fires after push" spec (proactive-suggestion revision):
// landing on pushReady right after a successful push now DOES proactively
// call Deps.GenerateSummary — the user no longer has to press `a` once just
// to see a suggestion. It still never auto-accepts: aiAccepted is set ONLY
// by an explicit second 'a' (keyPushPreparation), so effectiveTitle()/
// effectiveDescription() keep gating on it, and gh pr create is unaffected
// until that explicit accept.
func TestModel_PushPreparation_AISuggestion_AutoFiresOnPushReady(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	var calls int

	m := New(Deps{
		Dir:             "/repo",
		Config:          testConfig(),
		Git:             pushGit("/repo", branch, "git@github.com:org/repo.git", true),
		GH:              github.New(ghRunner("authed")),
		GenerateSummary: aiDep("PROJ-1 - AI drafted title", "AI drafted description", &calls),
	})
	m.state = StatePushPreparation
	m.pushPhase = pushConfirm
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(keyPress("p"))
	pushMsg := cmd()
	next2, cmd2 := next.(Model).Update(pushMsg)
	prepMsg := cmd2()
	next3, cmd3 := next2.(Model).Update(prepMsg)
	nm3 := next3.(Model)

	if nm3.pushPhase != pushReady {
		t.Fatalf("expected to land on pushReady, got %v", nm3.pushPhase)
	}
	if !nm3.aiPending {
		t.Fatal("landing on pushReady should auto-fire the AI suggestion request (aiPending)")
	}
	if cmd3 == nil {
		t.Fatal("onPrepDone should return the auto-fired aiSuggestCmd")
	}
	if !strings.Contains(nm3.View(), "Generando") {
		t.Errorf("pushReady should show the pending indicator while the auto-fired request is in flight\n%s", nm3.View())
	}

	// Land the auto-fired request's result.
	aiMsg := cmd3()
	next4, _ := nm3.Update(aiMsg)
	nm4 := next4.(Model)

	if calls != 1 {
		t.Fatalf("Deps.GenerateSummary should be called exactly once automatically, got %d call(s)", calls)
	}
	if nm4.aiTitle != "PROJ-1 - AI drafted title" || nm4.aiDescription != "AI drafted description" {
		t.Fatalf("the auto-fired suggestion should populate aiTitle/aiDescription, got title=%q description=%q", nm4.aiTitle, nm4.aiDescription)
	}
	if nm4.aiAccepted {
		t.Fatal("auto-firing must never auto-accept — only an explicit second 'a' accepts")
	}
	if !strings.Contains(nm4.View(), "PROJ-1 - AI drafted title") {
		t.Errorf("pushReady should show the auto-fetched suggestion for review\n%s", nm4.View())
	}
}

// TestModel_PushPreparation_AISuggestion_NoAutoFireWithoutConfig mirrors
// every other nil-degrades Deps convention in this package: with no
// Deps.GenerateSummary configured, landing on pushReady never sets aiPending
// and never returns a command (spec: "No ai config leaves pushReady
// unchanged").
func TestModel_PushPreparation_AISuggestion_NoAutoFireWithoutConfig(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"

	m := New(Deps{
		Dir:    "/repo",
		Config: testConfig(),
		Git:    pushGit("/repo", branch, "git@github.com:org/repo.git", true),
		GH:     github.New(ghRunner("authed")),
	})
	m.state = StatePushPreparation
	m.pushPhase = pushConfirm
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(keyPress("p"))
	pushMsg := cmd()
	next2, cmd2 := next.(Model).Update(pushMsg)
	prepMsg := cmd2()
	next3, cmd3 := next2.(Model).Update(prepMsg)
	nm3 := next3.(Model)

	if nm3.pushPhase != pushReady {
		t.Fatalf("expected to land on pushReady, got %v", nm3.pushPhase)
	}
	if nm3.aiPending {
		t.Fatal("no Deps.GenerateSummary should never set aiPending")
	}
	if cmd3 != nil {
		t.Fatal("no Deps.GenerateSummary should return a nil cmd from onPrepDone")
	}
	if strings.Contains(nm3.View(), "Sugerencia IA") {
		t.Errorf("no AI config should show no AI block at all\n%s", nm3.View())
	}
}

// TestModel_PushPreparation_PRConfirm_ReadyUnaccepted_YUsesAI is the
// never-silently-skip fix: at pushPRConfirm, when an AI suggestion is ready
// but NOT yet accepted (aiTitle != "" && !aiAccepted), pressing `y` accepts
// it (sets aiAccepted) and creates the PR WITH the AI title and description,
// as discrete gh pr create args.
func TestModel_PushPreparation_PRConfirm_ReadyUnaccepted_YUsesAI(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	aiTitle := "PROJ-1 - AI drafted title"
	aiDescription := "AI drafted description"

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, aiTitle, aiDescription, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})

	m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr)})
	m.state = StatePushPreparation
	m.pushPhase = pushPRConfirm
	m.authState = github.AuthAuthenticated
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
	m.aiTitle = aiTitle
	m.aiDescription = aiDescription

	next, cmd := m.Update(keyPress("y"))
	nm := next.(Model)
	if !nm.aiAccepted {
		t.Fatal("y on ready+unaccepted should accept the AI suggestion")
	}
	if nm.pushPhase != pushPRCreating {
		t.Fatalf("phase after y = %v, want pushPRCreating", nm.pushPhase)
	}
	if cmd == nil {
		t.Fatal("y should fire createPRCmd")
	}
	cmd()

	if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", aiTitle, "--body", aiDescription) {
		t.Fatalf("gh pr create should use the AI title/body, calls: %v", fr.Calls)
	}
}

// TestModel_PushPreparation_PRConfirm_ReadyUnaccepted_DUsesDefault is the
// explicit-decline counterpart: `d` on the same ready+unaccepted state
// creates the PR with the DEFAULT formula title and an empty body, and
// aiAccepted stays false (the suggestion was explicitly declined, not
// silently skipped).
func TestModel_PushPreparation_PRConfirm_ReadyUnaccepted_DUsesDefault(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	formulaTitle := github.SuggestedTitle("PROJ-1", "UAT")
	aiTitle := "PROJ-1 - AI drafted title"

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, formulaTitle, "", execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})

	m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr)})
	m.state = StatePushPreparation
	m.pushPhase = pushPRConfirm
	m.authState = github.AuthAuthenticated
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
	m.aiTitle = aiTitle
	m.aiDescription = "AI drafted description"

	next, cmd := m.Update(keyPress("d"))
	nm := next.(Model)
	if nm.aiAccepted {
		t.Fatal("d on ready+unaccepted must NOT accept the AI suggestion")
	}
	if nm.pushPhase != pushPRCreating {
		t.Fatalf("phase after d = %v, want pushPRCreating", nm.pushPhase)
	}
	if cmd == nil {
		t.Fatal("d should fire createPRCmd")
	}
	cmd()

	if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", formulaTitle, "--body", "") {
		t.Fatalf("gh pr create should use the default formula title and empty body, calls: %v", fr.Calls)
	}
}

// TestModel_PushPreparation_PRConfirm_Pending_BlocksCreate closes the timing
// hole: while a suggestion is still generating (aiPending), y and d at
// pushPRConfirm must NOT create a PR (no gh pr create call, phase stays
// pushPRConfirm) — only n/N/esc (back to pushReady) and q (quit) work.
func TestModel_PushPreparation_PRConfirm_Pending_BlocksCreate(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := ghRunner("authed")

	base := func() Model {
		m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr)})
		m.state = StatePushPreparation
		m.pushPhase = pushPRConfirm
		m.authState = github.AuthAuthenticated
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
		m.aiPending = true
		return m
	}

	for _, k := range []string{"y", "d"} {
		m := base()
		next, cmd := m.Update(keyPress(k))
		nm := next.(Model)
		if nm.pushPhase != pushPRConfirm {
			t.Errorf("%q while pending should stay on pushPRConfirm, got %v", k, nm.pushPhase)
		}
		if cmd != nil {
			t.Errorf("%q while pending must not return a command", k)
		}
	}
	if n := prCreateCalls(fr); n != 0 {
		t.Fatalf("no gh pr create call should have happened while pending, got %d", n)
	}

	m := base()
	next, _ := m.Update(keyPress("n"))
	nm := next.(Model)
	if nm.pushPhase != pushReady {
		t.Fatalf("n while pending should return to pushReady, got %v", nm.pushPhase)
	}
}

// TestModel_PushPreparation_PRConfirm_AlreadyAccepted_YUsesAI is case 3
// (unchanged behavior): when aiAccepted is already true (set by an earlier
// `a` press on pushReady), `y` still creates with the AI title/body exactly
// as before.
func TestModel_PushPreparation_PRConfirm_AlreadyAccepted_YUsesAI(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	aiTitle := "PROJ-1 - AI drafted title"
	aiDescription := "AI drafted description"

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, aiTitle, aiDescription, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})

	m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr)})
	m.state = StatePushPreparation
	m.pushPhase = pushPRConfirm
	m.authState = github.AuthAuthenticated
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
	m.aiTitle = aiTitle
	m.aiDescription = aiDescription
	m.aiAccepted = true // pre-accepted via an earlier 'a' press on pushReady

	next, cmd := m.Update(keyPress("y"))
	nm := next.(Model)
	if nm.pushPhase != pushPRCreating {
		t.Fatalf("phase after y = %v, want pushPRCreating", nm.pushPhase)
	}
	if cmd == nil {
		t.Fatal("y should fire createPRCmd")
	}
	cmd()

	if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", aiTitle, "--body", aiDescription) {
		t.Fatalf("gh pr create should use the already-accepted AI title/body, calls: %v", fr.Calls)
	}
}

// TestModel_PushPreparation_PRConfirm_NoAI_YUsesFormula is case 3's other
// unchanged path: with no AI configured at all (aiTitle stays ""), `y`
// creates normally with the formula title exactly as before.
func TestModel_PushPreparation_PRConfirm_NoAI_YUsesFormula(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	formulaTitle := github.SuggestedTitle("PROJ-1", "UAT")

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, formulaTitle, "", execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})

	// Deps.GenerateSummary nil: no AI configured at all.
	m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr)})
	m.state = StatePushPreparation
	m.pushPhase = pushPRConfirm
	m.authState = github.AuthAuthenticated
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(keyPress("y"))
	nm := next.(Model)
	if nm.pushPhase != pushPRCreating {
		t.Fatalf("phase after y = %v, want pushPRCreating", nm.pushPhase)
	}
	if cmd == nil {
		t.Fatal("y should fire createPRCmd")
	}
	cmd()

	if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", formulaTitle, "--body", "") {
		t.Fatalf("gh pr create should use the formula title, calls: %v", fr.Calls)
	}
}

// TestModel_PushPreparation_PRConfirm_ViewMatchesAIState is the view
// counterpart: the "Crear la PR con la sugerencia IA?" prompt + y/d/n footer
// render ONLY in ready+unaccepted; the plain confirm renders otherwise
// (no-AI or already-accepted); the pending indicator renders when aiPending.
func TestModel_PushPreparation_PRConfirm_ViewMatchesAIState(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	base := func() Model {
		m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(ghRunner("authed"))})
		m.state = StatePushPreparation
		m.pushPhase = pushPRConfirm
		m.authState = github.AuthAuthenticated
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
		return m
	}

	t.Run("pending shows waiting indicator", func(t *testing.T) {
		m := base()
		m.aiPending = true
		v := m.View()
		if !strings.Contains(v, "Esperando sugerencia IA") {
			t.Errorf("pending pushPRConfirm should show the waiting indicator\n%s", v)
		}
		if !strings.Contains(v, "n volver") || !strings.Contains(v, "q salir") {
			t.Errorf("pending pushPRConfirm footer should offer n/q only\n%s", v)
		}
		if strings.Contains(v, "Confirmar creación del PR con gh?") {
			t.Errorf("pending pushPRConfirm must not show the plain confirm prompt\n%s", v)
		}
	})

	t.Run("ready+unaccepted shows the explicit AI choice", func(t *testing.T) {
		m := base()
		m.aiTitle = "PROJ-1 - AI drafted title"
		m.aiDescription = "AI drafted description"
		v := m.View()
		if !strings.Contains(v, "Crear la PR con la sugerencia IA?") {
			t.Errorf("ready+unaccepted pushPRConfirm should ask the explicit AI question\n%s", v)
		}
		if !strings.Contains(v, "y con IA") || !strings.Contains(v, "d título default") {
			t.Errorf("ready+unaccepted pushPRConfirm footer should offer y/d\n%s", v)
		}
		if strings.Contains(v, "Confirmar creación del PR con gh?") {
			t.Errorf("ready+unaccepted pushPRConfirm must not show the plain confirm prompt\n%s", v)
		}
	})

	t.Run("no suggestion renders the plain confirm", func(t *testing.T) {
		m := base()
		v := m.View()
		if !strings.Contains(v, "Confirmar creación del PR con gh?") {
			t.Errorf("no-AI pushPRConfirm should show the plain confirm prompt\n%s", v)
		}
		if strings.Contains(v, "Crear la PR con la sugerencia IA?") {
			t.Errorf("no-AI pushPRConfirm must not show the AI-choice prompt\n%s", v)
		}
	})

	t.Run("already accepted renders the plain confirm", func(t *testing.T) {
		m := base()
		m.aiTitle = "PROJ-1 - AI drafted title"
		m.aiDescription = "AI drafted description"
		m.aiAccepted = true
		v := m.View()
		if !strings.Contains(v, "Confirmar creación del PR con gh?") {
			t.Errorf("already-accepted pushPRConfirm should show the plain confirm prompt\n%s", v)
		}
		if strings.Contains(v, "Crear la PR con la sugerencia IA?") {
			t.Errorf("already-accepted pushPRConfirm must not show the AI-choice prompt\n%s", v)
		}
	})
}
