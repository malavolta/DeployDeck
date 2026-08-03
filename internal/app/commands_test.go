package app

import (
	"context"
	"errors"
	"testing"

	"github.com/malavolta/DeployDeck/internal/delta"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestSpinnerCmd_SchedulesTick is task 5.1 (RED): spinnerCmd() mirrors the
// existing tickCmd/tickMsg idiom (commands.go tickCmd) — it returns a
// non-nil tea.Cmd whose invocation yields a spinnerTickMsg.
func TestSpinnerCmd_SchedulesTick(t *testing.T) {
	cmd := spinnerCmd()
	if cmd == nil {
		t.Fatal("spinnerCmd() should return a non-nil tea.Cmd")
	}
	msg := cmd()
	if _, ok := msg.(spinnerTickMsg); !ok {
		t.Fatalf("spinnerCmd()() = %T, want spinnerTickMsg", msg)
	}
}

// TestAISuggestCmd_NilDeps_ReturnsNil is task 4.4 (RED)'s nil-degrades
// guard, mirroring checkUpdateCmd's TestCheckUpdateCmd_NilDeps_ReturnsNil:
// with no Deps.GenerateSummary injected, aiSuggestCmd is a no-op.
func TestAISuggestCmd_NilDeps_ReturnsNil(t *testing.T) {
	m := New(Deps{})
	if cmd := m.aiSuggestCmd(); cmd != nil {
		t.Fatalf("aiSuggestCmd() with nil Deps.GenerateSummary should return nil, got non-nil cmd")
	}
}

// TestAISuggestCmd_BuildsTicketSubjectsAndComponentSummary is task 4.4
// (RED): aiSuggestCmd builds its ticket/commitSubjects/componentSummary
// arguments from data already on Model (design's Data Flow) and forwards
// the fake dep's result as aiSuggestDoneMsg.
func TestAISuggestCmd_BuildsTicketSubjectsAndComponentSummary(t *testing.T) {
	var gotTicket string
	var gotSubjects []string
	var gotSummary string

	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		gotTicket = ticket
		gotSubjects = commitSubjects
		gotSummary = componentSummary
		return "PROJ-1 - AI drafted title", "AI drafted description", nil
	}})
	m.plan = git.DeploymentPlan{
		Ticket: "PROJ-1",
		SelectedCommits: []git.DiscoveredCommit{
			{Commit: git.Commit{Subject: "feat: add discount validation"}},
			{Commit: git.Commit{Subject: "fix: null pointer on checkout"}},
		},
	}
	m.summary = delta.PackageSummary{Types: []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 2}}}

	cmd := m.aiSuggestCmd()
	if cmd == nil {
		t.Fatal("aiSuggestCmd() with a non-nil Deps.GenerateSummary should return a non-nil cmd")
	}

	msg, ok := cmd().(aiSuggestDoneMsg)
	if !ok {
		t.Fatalf("aiSuggestCmd()() = %T, want aiSuggestDoneMsg", msg)
	}

	if gotTicket != "PROJ-1" {
		t.Errorf("ticket = %q, want %q", gotTicket, "PROJ-1")
	}
	wantSubjects := []string{"feat: add discount validation", "fix: null pointer on checkout"}
	if len(gotSubjects) != len(wantSubjects) {
		t.Fatalf("commitSubjects = %v, want %v", gotSubjects, wantSubjects)
	}
	for i := range wantSubjects {
		if gotSubjects[i] != wantSubjects[i] {
			t.Errorf("commitSubjects[%d] = %q, want %q", i, gotSubjects[i], wantSubjects[i])
		}
	}
	if gotSummary == "" {
		t.Error("expected a non-empty rendered componentSummary")
	}

	if msg.title != "PROJ-1 - AI drafted title" || msg.description != "AI drafted description" {
		t.Errorf("aiSuggestDoneMsg = %+v, want title/description forwarded from the fake dep", msg)
	}
	if msg.err != nil {
		t.Errorf("aiSuggestDoneMsg.err = %v, want nil", msg.err)
	}
}

// TestAISuggestCmd_ForwardsError proves a fake dep's error is forwarded
// as-is on aiSuggestDoneMsg (onAISuggestDone, task 4.6, degrades it — this
// command itself just relays the result).
func TestAISuggestCmd_ForwardsError(t *testing.T) {
	wantErr := errors.New("ai: request failed")
	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "", "", wantErr
	}})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1"}

	msg := m.aiSuggestCmd()().(aiSuggestDoneMsg)
	if msg.err != wantErr {
		t.Fatalf("aiSuggestDoneMsg.err = %v, want %v", msg.err, wantErr)
	}
}
