package app

import (
	"errors"
	"testing"
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
