package app

import (
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestStateSourceConfirm_Exists is task 3.1 (RED): StateSourceConfirm must
// exist as its own distinct State constant.
func TestStateSourceConfirm_Exists(t *testing.T) {
	states := map[State]bool{}
	for _, s := range []State{
		StatePrereqCheck, StateTicketInput, StateCommitDiscovery, StateCommitSelection,
		StateTargetSelection, StatePlanPreview, StateBranchCreation, StateCherryPicking,
		StateSourceConfirm,
	} {
		if states[s] {
			t.Fatalf("StateSourceConfirm collides with an existing State constant value (%v)", s)
		}
		states[s] = true
	}
}

// candidateFakeRunner cans the git calls discoverCmd's pass 1 issues for a
// message+branch-only Discover (Ticket only, no Target/Source).
func candidateFakeRunner(t *testing.T, root, ticket string, localBranches, remoteBranches string) *execpkg.FakeRunner {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(root + "\n")})
	fr.When("git", []string{"log", "--all", "--grep", ticket, "--format=%H%x09%h%x09%an%x09%aI%x09%P%x09%s"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"branch", "--format=%(refname:short)", "--list", "*" + ticket + "*"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(localBranches)})
	fr.When("git", []string{"branch", "--format=%(refname:short)", "-r", "--list", "*" + ticket + "*"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(remoteBranches)})
	return fr
}

// TestDiscoverCmd_NeedsConfirm_ReturnsConfirmMsgWithoutRangedDiscover is task
// 3.3 (RED): a resolveNeedsConfirm outcome returns confirm=true and the
// pending candidate WITHOUT the ranged discover ever running —
// OrderedCommits stays nil, and only the canned RepoRoot/log/branch calls
// were registered, so any extra call fails loudly via FakeRunner.
func TestDiscoverCmd_NeedsConfirm_ReturnsConfirmMsgWithoutRangedDiscover(t *testing.T) {
	fr := candidateFakeRunner(t, "/repo", "PROJ-1", "feature/PROJ-1\nhotfix/PROJ-1\n", "")

	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: config.Config{}})
	m.ticket = "PROJ-1"
	m.prelim = "UAT"
	m.originalBranch = "feature/PROJ-1"

	msg := run(t, m.discoverCmd())
	dmsg, ok := msg.(discoverDoneMsg)
	if !ok {
		t.Fatalf("expected discoverDoneMsg, got %T", msg)
	}
	if dmsg.err != nil {
		t.Fatalf("unexpected error: %v", dmsg.err)
	}
	if !dmsg.confirm {
		t.Fatal("expected confirm=true for a genuinely ambiguous current-branch match")
	}
	if dmsg.source.Name != "feature/PROJ-1" {
		t.Errorf("pending source = %+v, want Name=feature/PROJ-1", dmsg.source)
	}
	if dmsg.result.OrderedCommits != nil {
		t.Errorf("pass 1 must not run the ranged discover, OrderedCommits = %v", dmsg.result.OrderedCommits)
	}
}

// TestOnDiscoverDone_Confirm_RoutesToStateSourceConfirm is task 3.5 (RED):
// msg.confirm==true sets m.discovery/m.source, moves to StateSourceConfirm,
// and leaves m.items empty (no premature selection screen).
func TestOnDiscoverDone_Confirm_RoutesToStateSourceConfirm(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateCommitDiscovery
	base := git.DiscoverResult{
		Ticket:            "PROJ-1",
		CandidateBranches: []git.Branch{{Name: "feature/PROJ-1"}, {Name: "hotfix/PROJ-1"}},
	}
	pending := git.Branch{Name: "feature/PROJ-1"}

	next, cmd := m.Update(discoverDoneMsg{result: base, source: pending, confirm: true})
	nm := next.(Model)

	if nm.State() != StateSourceConfirm {
		t.Fatalf("confirm msg should route to StateSourceConfirm, got %v", nm.State())
	}
	if nm.discovery.Ticket != "PROJ-1" {
		t.Errorf("m.discovery should be set to the pass-1 base result, got %+v", nm.discovery)
	}
	if nm.source.Name != "feature/PROJ-1" {
		t.Errorf("m.source should carry the pending candidate, got %+v", nm.source)
	}
	if len(nm.items) != 0 {
		t.Errorf("confirm landing must not prematurely populate m.items, got %d", len(nm.items))
	}
	if cmd != nil {
		t.Error("confirm landing should fire no command (waits for the user's s/n/N/enter/esc)")
	}
}

// sourceConfirmModel parks a Model on StateSourceConfirm with a pending
// candidate, as onDiscoverDone's confirm branch would have left it.
func sourceConfirmModel(t *testing.T) Model {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	m := New(Deps{Git: git.New(fr), Dir: "/repo", Config: testConfig()})
	m.state = StateSourceConfirm
	m.ticket = "PROJ-1"
	m.discovery = git.DiscoverResult{
		Ticket:            "PROJ-1",
		CandidateBranches: []git.Branch{{Name: "feature/PROJ-1"}, {Name: "hotfix/PROJ-1"}},
	}
	m.source = git.Branch{Name: "feature/PROJ-1"}
	return m
}

// TestKeySourceConfirm is task 3.7 (RED): `s` fires confirmSourceCmd;
// `n`/`N`/`enter` (default-No) clear m.source and degrade to
// StateCommitSelection with zero items; `esc` backs out to StateTicketInput.
func TestKeySourceConfirm(t *testing.T) {
	t.Run("s accepts and fires confirmSourceCmd", func(t *testing.T) {
		m := sourceConfirmModel(t)
		next, cmd := m.Update(keyPress("s"))
		nm := next.(Model)
		if nm.State() != StateCommitDiscovery {
			t.Fatalf("s should transition to StateCommitDiscovery, got %v", nm.State())
		}
		if cmd == nil {
			t.Fatal("s should fire a non-nil command (confirmSourceCmd)")
		}
	})

	// Bug fix: uppercase S (shifted/caps input) was silently dropped even
	// though the decline key already accepted both "n" and "N" — an
	// inconsistency within this same handler. S must produce the SAME result
	// as lowercase s.
	t.Run("S (uppercase) confirms exactly like lowercase s", func(t *testing.T) {
		m := sourceConfirmModel(t)
		next, cmd := m.Update(keyPress("S"))
		nm := next.(Model)
		if nm.State() != StateCommitDiscovery {
			t.Fatalf("S should transition to StateCommitDiscovery, got %v", nm.State())
		}
		if cmd == nil {
			t.Fatal("S should fire a non-nil command (confirmSourceCmd)")
		}
	})

	for _, key := range []string{"n", "N", "enter"} {
		t.Run("decline via "+key, func(t *testing.T) {
			m := sourceConfirmModel(t)
			next, _ := m.Update(keyPress(key))
			nm := next.(Model)
			if nm.State() != StateCommitSelection {
				t.Fatalf("%s should degrade to StateCommitSelection, got %v", key, nm.State())
			}
			if nm.source.Name != "" {
				t.Errorf("%s should clear the pending source, got %+v", key, nm.source)
			}
			if nm.discovery.OrderedCommits != nil {
				t.Errorf("%s must not have run the ranged discover, OrderedCommits = %v", key, nm.discovery.OrderedCommits)
			}
			if len(nm.items) != 0 {
				t.Errorf("%s degrade should leave zero selection items (no ordered commits), got %d", key, len(nm.items))
			}
		})
	}

	t.Run("esc backs out to StateTicketInput", func(t *testing.T) {
		m := sourceConfirmModel(t)
		next, _ := m.Update(keyPress("esc"))
		nm := next.(Model)
		if nm.State() != StateTicketInput {
			t.Fatalf("esc should return to StateTicketInput, got %v", nm.State())
		}
	})
}

// TestViewSourceConfirm is task 3.10 (RED): viewSourceConfirm renders the
// exact Spanish confirm literal, naming the pending m.source.Name.
func TestViewSourceConfirm(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateSourceConfirm
	m.source = git.Branch{Name: "feature/PROJ-1"}

	v := m.View()
	want := "¿Usar la rama actual 'feature/PROJ-1' como origen? [s/N]"
	if !strings.Contains(v, want) {
		t.Errorf("viewSourceConfirm missing %q\n%s", want, v)
	}
}
