package app

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
)

// TestKeyPushPreparation_A_NoSuggestion_RequestsWhenDepPresent is task 4.8
// (RED): the FIRST 'a' press at pushReady, with no suggestion yet and a
// non-nil GenerateSummary dep, sets aiPending and fires aiSuggestCmd (spec:
// "Explicit keypress triggers generation").
func TestKeyPushPreparation_A_NoSuggestion_RequestsWhenDepPresent(t *testing.T) {
	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "PROJ-1 - AI drafted title", "AI drafted description", nil
	}})
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}

	next, cmd := m.Update(keyPress("a"))
	nm := next.(Model)

	if !nm.aiPending {
		t.Fatal("first 'a' with a suggestion dep present should set aiPending")
	}
	if nm.aiAccepted {
		t.Fatal("first 'a' must not accept anything yet")
	}
	if cmd == nil {
		t.Fatal("first 'a' should return the aiSuggestCmd command")
	}
	msg, ok := cmd().(aiSuggestDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want aiSuggestDoneMsg", msg)
	}
}

// TestKeyPushPreparation_A_UnacceptedSuggestion_Accepts is task 4.8 (RED):
// the SECOND 'a' press, once a suggestion has been generated but not yet
// accepted, sets aiAccepted — the distinct accept action (spec: "Accepting
// overrides the PR-creation title source").
func TestKeyPushPreparation_A_UnacceptedSuggestion_Accepts(t *testing.T) {
	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "should not be called again", "", nil
	}})
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}
	m.aiTitle = "PROJ-1 - AI drafted title"
	m.aiDescription = "AI drafted description"

	next, cmd := m.Update(keyPress("a"))
	nm := next.(Model)

	if !nm.aiAccepted {
		t.Fatal("second 'a' on an unaccepted suggestion should accept it")
	}
	if cmd != nil {
		t.Fatal("accepting should not fire another aiSuggestCmd")
	}
}

// TestKeyPushPreparation_A_PendingOrAccepted_Inert is task 4.8 (RED): once
// aiPending or aiAccepted, further 'a' presses are a strict no-op — a
// suggestion can never be requested twice concurrently, nor re-requested
// after acceptance.
func TestKeyPushPreparation_A_PendingOrAccepted_Inert(t *testing.T) {
	dep := func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "unexpected call", "", nil
	}

	t.Run("pending is inert", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.state = StatePushPreparation
		m.pushPhase = pushReady
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiPending = true

		next, cmd := m.Update(keyPress("a"))
		if cmd != nil {
			t.Fatal("'a' while aiPending should be inert (no new command)")
		}
		if next.(Model).aiAccepted {
			t.Fatal("'a' while aiPending must not accept anything")
		}
	})

	t.Run("accepted is inert", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.state = StatePushPreparation
		m.pushPhase = pushReady
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiTitle = "PROJ-1 - AI drafted title"
		m.aiAccepted = true

		next, cmd := m.Update(keyPress("a"))
		if cmd != nil {
			t.Fatal("'a' once already accepted should be inert (no new command)")
		}
		if next.(Model).aiTitle != "PROJ-1 - AI drafted title" {
			t.Fatal("'a' once already accepted must not change the accepted title")
		}
	})
}

// TestKeyPushPreparation_A_NilDep_Inert is task 4.8 (RED): with no
// GenerateSummary dep configured, 'a' is inert (spec: "No ai config leaves
// pushReady unchanged").
func TestKeyPushPreparation_A_NilDep_Inert(t *testing.T) {
	m := New(Deps{})
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	next, cmd := m.Update(keyPress("a"))
	if cmd != nil {
		t.Fatal("'a' with no GenerateSummary dep should be inert")
	}
	if next.(Model).aiPending {
		t.Fatal("'a' with no GenerateSummary dep must not set aiPending")
	}
}
