package app

import (
	"strings"
	"testing"
)

// footerKeys extracts the lower-cased key tokens advertised by the `[ ... ]`
// footer of a rendered view (segments are separated by three spaces, keys
// within a segment by "/"; the first word of a segment holds its keys).
func footerKeys(view string) (keys map[string]bool, text string, ok bool) {
	start, end := strings.LastIndex(view, "[ "), strings.LastIndex(view, " ]")
	if start < 0 || end < start {
		return nil, "", false
	}
	text = view[start+2 : end]
	keys = map[string]bool{}
	for _, seg := range strings.Split(text, "   ") {
		for _, k := range strings.Split(strings.Fields(seg)[0], "/") {
			keys[strings.ToLower(k)] = true
		}
	}
	return keys, text, true
}

// TestFooterParity_EscAndQMatchHandlers asserts, for every esc/q fixture, that
// the footer advertises esc (resp. q) exactly when the handler acts on it.
func TestFooterParity_EscAndQMatchHandlers(t *testing.T) {
	for _, f := range escQFixtures() {
		t.Run(f.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			f.set(&m)
			keys, text, ok := footerKeys(m.View())
			if !ok {
				t.Fatalf("no footer rendered:\n%s", m.View())
			}
			for key, want := range map[string]any{"esc": f.esc, "q": f.q} {
				if acts := want != "none"; keys[key] != acts {
					t.Errorf("footer %q: %s advertised=%v but handler acts=%v", text, key, keys[key], acts)
				}
			}
			if f.esc == "none" && f.q == "none" && !strings.Contains(text, "ctrl+c interrumpe") {
				t.Errorf("in-flight footer %q must state ctrl+c interrumpe", text)
			}
		})
	}
}

func TestFooterParity_PinnedFragments(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Model)
		want []string
	}{
		{"polling", st(StateValidationPolling), []string{"c cancelar", "r refrescar", "q salir (el job sigue activo)"}},
		{"conflict", st(StateCherryPickConflict), []string{"el run queda reanudable", "↑/↓"}},
		{"succeeded with branch", func(m *Model) { m.state, m.plan.PromotionBranch = StateSucceeded, "deploy/DD-1" }, []string{"d borrar rama"}},
		{"cleanup confirm", func(m *Model) {
			m.state, m.cleanupPhase = StateBranchCleanup, cleanupConfirm
			m.cleanupBranches = []cleanupRow{{}}
		}, []string{"Esc/q cancelar"}},
	}
	for _, c := range cases {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		c.set(&m)
		for _, w := range c.want {
			if !strings.Contains(m.View(), w) {
				t.Errorf("%s: footer lacks %q:\n%s", c.name, w, m.View())
			}
		}
	}
	m := New(Deps{Dir: "/repo"})
	m.state = StateSucceeded
	if strings.Contains(m.View(), "d borrar rama") {
		t.Error("succeeded without a branch must not advertise d")
	}
}
