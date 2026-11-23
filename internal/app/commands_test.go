package app

import (
	"context"
	"errors"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/delta"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
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

// timeoutGHRunner answers every gh call with context.DeadlineExceeded.
type timeoutGHRunner struct{ calls []execpkg.CommandRequest }

func (r *timeoutGHRunner) Run(_ context.Context, req execpkg.CommandRequest) (execpkg.CommandResult, error) {
	r.calls = append(r.calls, req)
	return execpkg.CommandResult{}, context.DeadlineExceeded
}

func TestPreparePRCmd_GHUnavailableStates(t *testing.T) {
	gitFR := execpkg.NewFakeRunner()
	gitFR.When("git", []string{"remote", "get-url", "origin"}, execpkg.CommandResult{Stdout: []byte("git@github.com:org/repo.git")})
	gitFR.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{Stdout: []byte("/repo")})

	tests := []struct {
		name string
		gh   github.Client
		want github.AuthState
	}{
		{"timed out", github.New(&timeoutGHRunner{}), github.AuthTimedOut},
		{"nil client is absent", nil, github.AuthAbsent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig(), Git: git.New(gitFR), GH: tt.gh})
			m.plan = git.DeploymentPlan{TargetBranch: "UAT", PromotionBranch: "deploy/x"}
			pm := run(t, m.preparePRCmd()).(prepDoneMsg)
			if pm.auth != tt.want {
				t.Fatalf("auth = %v, want %v", pm.auth, tt.want)
			}
			if pm.compareURL == "" {
				t.Fatalf("compare URL must still be derived, got %+v", pm)
			}
		})
	}
}

func TestGateCheckCmd_GHUnavailableBlocksNamingGH(t *testing.T) {
	gateCfg := config.GateConfig{Enabled: true, Approvers: []string{"alice"}}
	tests := []struct {
		name   string
		runner func() (execpkg.Runner, func() int)
		want   string
	}{
		{"absent", func() (execpkg.Runner, func() int) {
			fr := execpkg.NewFakeRunner()
			return fr, func() int { return len(fr.Calls) }
		}, "gh no disponible"},
		{"unauthenticated", func() (execpkg.Runner, func() int) {
			fr := ghRunner("unauthed")
			return fr, func() int { return len(fr.Calls) }
		}, "gh no autenticado"},
		{"timed out", func() (execpkg.Runner, func() int) {
			r := &timeoutGHRunner{}
			return r, func() int { return len(r.calls) }
		}, "no se pudo verificar gh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, calls := tt.runner()
			m, _ := gateCheckModel(t, gateCfg)
			m.deps.GH = github.New(r)
			done := run(t, m.gateCheckCmd()).(gateCheckDoneMsg)
			c := done.result.Conditions
			if done.result.Passed || len(c) != 1 || c[0].Name != "gh-availability" || c[0].Detail != tt.want {
				t.Fatalf("result = %+v, want one failed gh-availability %q", done.result, tt.want)
			}
			if calls() != 1 {
				t.Fatalf("only the auth probe may run, got %d gh calls", calls())
			}
		})
	}
}
