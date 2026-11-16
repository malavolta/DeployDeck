package app

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/github"
)

func TestAIActionGuard(t *testing.T) {
	ready := func(ai bool, pr string) Model {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state, m.pushPhase, m.authState, m.prURL = StatePushPreparation, pushReady, github.AuthAuthenticated, pr
		if ai {
			m.deps.GenerateSummary = stubAI
		}
		return m
	}
	m := ready(true, "")
	if keys, _, _ := footerKeys(m.View()); !keys["a"] {
		t.Error("a must be offered before the PR exists")
	}
	if nm, cmd := press(m, "a"); cmd == nil || !nm.aiPending {
		t.Error("a must start generation before the PR exists")
	}
	for name, m := range map[string]Model{"after PR": ready(true, "https://pr/1"), "no AI": ready(false, "")} {
		if keys, _, _ := footerKeys(m.View()); keys["a"] {
			t.Errorf("%s: footer must not offer a", name)
		}
		if nm, cmd := press(m, "a"); cmd != nil || nm.aiPending || nm.aiTitle != "" || nm.View() != m.View() {
			t.Errorf("%s: a must be inert", name)
		}
	}
	// Accepting an existing suggestion after the PR is also inert.
	m = ready(true, "https://pr/1")
	m.aiTitle = "t"
	if nm, _ := press(m, "a"); nm.aiAccepted {
		t.Error("a must not accept a suggestion after the PR exists")
	}
}
