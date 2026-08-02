package app

import (
	"context"
	"errors"
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
)

// TestOnAISuggestDone_Success_SetsTitleAndDescription_NotAccepted is task
// 4.6 (RED): a successful generation sets aiTitle/aiDescription and clears
// aiPending, but does NOT set aiAccepted — a generated suggestion is
// PROPOSED, never auto-accepted (spec: "Explicit Accept Overrides Only The
// PR-Creation Title Source").
func TestOnAISuggestDone_Success_SetsTitleAndDescription_NotAccepted(t *testing.T) {
	m := New(Deps{})
	m.aiPending = true

	next, cmd := m.Update(aiSuggestDoneMsg{title: "PROJ-1 - AI drafted title", description: "AI drafted description"})
	if cmd != nil {
		t.Errorf("onAISuggestDone should return a nil cmd, got non-nil")
	}
	nm := next.(Model)

	if nm.aiPending {
		t.Error("aiPending should be false after the request lands")
	}
	if nm.aiTitle != "PROJ-1 - AI drafted title" {
		t.Errorf("aiTitle = %q, want %q", nm.aiTitle, "PROJ-1 - AI drafted title")
	}
	if nm.aiDescription != "AI drafted description" {
		t.Errorf("aiDescription = %q, want %q", nm.aiDescription, "AI drafted description")
	}
	if nm.aiAccepted {
		t.Error("aiAccepted must stay false — a generated suggestion is proposed, not accepted")
	}
}

// TestOnAISuggestDone_ErrOrEmptyTitle_NoSuggestion is task 4.6 (RED): an
// error and an empty-title result are treated identically — aiPending
// clears, aiErr is recorded, but NO title/description change occurs (spec:
// "Silent Graceful Degradation").
func TestOnAISuggestDone_ErrOrEmptyTitle_NoSuggestion(t *testing.T) {
	wantErr := errors.New("ai: request failed")
	tests := []struct {
		name string
		msg  aiSuggestDoneMsg
	}{
		{name: "request errored", msg: aiSuggestDoneMsg{err: wantErr}},
		{name: "empty title, no error", msg: aiSuggestDoneMsg{title: "", description: "some description"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Deps{})
			m.aiPending = true

			next, cmd := m.Update(tt.msg)
			if cmd != nil {
				t.Errorf("onAISuggestDone should return a nil cmd, got non-nil")
			}
			nm := next.(Model)

			if nm.aiPending {
				t.Error("aiPending should be false after the request lands, even on failure")
			}
			if nm.aiTitle != "" {
				t.Errorf("aiTitle = %q, want empty (no suggestion surfaced on failure/empty title)", nm.aiTitle)
			}
			if nm.aiAccepted {
				t.Error("aiAccepted must stay false")
			}
		})
	}
}

// TestOnPrepDone_AutoFiresAISuggestion_WhenConfigured is Feature B's RED:
// once the screen settles on pushReady, onPrepDone proactively requests an
// ai-pr-summary suggestion — the user no longer has to press `a` first — but
// never auto-accepts (aiAccepted stays false; only the explicit second 'a'
// press, keyPushPreparation, ever sets it).
func TestOnPrepDone_AutoFiresAISuggestion_WhenConfigured(t *testing.T) {
	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "PROJ-1 - AI drafted title", "AI drafted description", nil
	}})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	next, cmd := m.Update(prepDoneMsg{})
	nm := next.(Model)

	if nm.pushPhase != pushReady {
		t.Fatalf("onPrepDone should settle pushPhase on pushReady, got %v", nm.pushPhase)
	}
	if !nm.aiPending {
		t.Error("onPrepDone should set aiPending when Deps.GenerateSummary is configured")
	}
	if cmd == nil {
		t.Fatal("onPrepDone should return the auto-fired aiSuggestCmd")
	}
	if nm.aiAccepted {
		t.Error("auto-firing must never set aiAccepted")
	}
}

// TestOnPrepDone_NoAutoFire_WhenGenerateSummaryNil is Feature B's
// nil-degrades companion: with no Deps.GenerateSummary configured, onPrepDone
// never sets aiPending and returns a nil cmd (same nil-degrades convention as
// every other optional Dep in this file).
func TestOnPrepDone_NoAutoFire_WhenGenerateSummaryNil(t *testing.T) {
	m := New(Deps{})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	next, cmd := m.Update(prepDoneMsg{})
	nm := next.(Model)

	if nm.pushPhase != pushReady {
		t.Fatalf("onPrepDone should settle pushPhase on pushReady, got %v", nm.pushPhase)
	}
	if nm.aiPending {
		t.Error("onPrepDone should not set aiPending without Deps.GenerateSummary")
	}
	if cmd != nil {
		t.Error("onPrepDone should return a nil cmd without Deps.GenerateSummary")
	}
}

// TestOnPrepDone_NoAutoFire_WhenAlreadyPendingOrHeld proves the auto-fire
// guard (m.aiTitle == "" && !m.aiPending) prevents a duplicate request: a
// late/duplicate prepDoneMsg landing while a request is already in flight,
// or once a suggestion is already held, must not fire a second one.
func TestOnPrepDone_NoAutoFire_WhenAlreadyPendingOrHeld(t *testing.T) {
	dep := func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "PROJ-1 - AI drafted title", "AI drafted description", nil
	}

	t.Run("already pending", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiPending = true

		_, cmd := m.Update(prepDoneMsg{})
		if cmd != nil {
			t.Error("onPrepDone must not fire a second request while one is already pending")
		}
	})

	t.Run("suggestion already held", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiTitle = "PROJ-1 - AI drafted title"

		_, cmd := m.Update(prepDoneMsg{})
		if cmd != nil {
			t.Error("onPrepDone must not re-request once a suggestion is already held")
		}
	})
}
