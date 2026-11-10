package app

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/runs"
)

func TestNotices_NoBranchPAndD(t *testing.T) {
	for _, s := range []State{StateSucceeded, StateAborted} {
		for _, k := range []string{"p", "d"} {
			if s == StateAborted && k == "p" {
				continue
			}
			m := New(Deps{Dir: "/repo"})
			m.state = s
			nm, cmd := press(m, k)
			if nm.state != s || cmd != nil || nm.cleanupPhase != cleanupIdle || nm.pushPhase != m.pushPhase {
				t.Fatalf("%v %s without a branch must not act", s, k)
			}
			if !strings.Contains(nm.View(), "rama") {
				t.Errorf("%v %s: view lacks the Spanish no-branch notice:\n%s", s, k, nm.View())
			}
		}
	}
}

func TestNotices_StandaloneDeltaEnterAndEdit(t *testing.T) {
	m := New(Deps{Dir: "/repo"})
	m.state, m.standaloneMode = StatePackageReview, "delta"
	nm, cmd := press(m, "enter")
	if cmd != nil || nm.state != StatePackageReview || !strings.Contains(nm.View(), "modo delta") {
		t.Fatalf("standalone delta Enter must explain itself, view:\n%s", nm.View())
	}
	g := New(Deps{Dir: "/repo"})
	g.state = StateDeltaGeneration
	if nm, cmd := press(g, "e"); nm.state != StateDeltaGeneration || cmd != nil {
		t.Fatal("delta e must be inert while no error is shown")
	}
}

func TestNotices_ClearOnNavigation(t *testing.T) {
	m := New(Deps{Dir: "/repo"})
	m.state = StateSucceeded
	nm, _ := press(m, "p")
	if nm.notice == "" {
		t.Fatal("setup: expected a notice")
	}
	nm, _ = press(nm, "x") // any further key, then the screen changes
	nm.state = StateMainMenu
	if strings.Contains(nm.View(), "rama de promoción") {
		t.Fatal("notice must not follow the user to another screen")
	}
}

func TestNotices_RunHistoryReasonsAndRePromote(t *testing.T) {
	now := time.Now()
	failed := runs.Record{RunID: "f", Status: "Failed", JobID: "0Af", CreatedAt: now}
	m := historyModel([]runs.Record{failed})
	for key, want := range map[string]string{"x": "quick deploy no disponible", "r": "exitosos"} {
		nm, cmd := press(m, key)
		v := nm.View()
		if nm.state != StateRunHistory || cmd != nil || !strings.Contains(v, want) {
			t.Fatalf("%s on a Failed row: state=%v want notice %q in:\n%s", key, nm.state, want, v)
		}
		if strings.Contains(v, "not a successful") {
			t.Errorf("%s: English reason leaked into the notice", key)
		}
		if nm, _ := press(nm, "down"); strings.Contains(nm.View(), want) {
			t.Errorf("%s: notice must clear on movement", key)
		}
	}
}

func TestNotices_QuickDeployReasonMapIsSpanishAndDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range []string{"not a successful validation", "required tests not run", "older than 10 days", "already quick-deployed"} {
		es := quickDeployReasonES(r)
		if es == r || seen[es] {
			t.Errorf("reason %q -> %q must be a distinct Spanish text", r, es)
		}
		seen[es] = true
	}
	if g := quickDeployReasonES("brand new reason"); g == "brand new reason" || g == "" {
		t.Errorf("unknown reason needs a generic Spanish fallback, got %q", g)
	}
}

// TestNotices_NoEnglishNoticeLiterals scans the key/update handlers: every
// user-facing notice literal must be Spanish (none starts with an English word).
func TestNotices_NoEnglishNoticeLiterals(t *testing.T) {
	english := regexp.MustCompile(`^(resolve|cannot|enter a|no destinations|continue|delta and|empty package|no next|no Tooling)`)
	lit := regexp.MustCompile(`notice = "([^"]+)"`)
	for _, f := range []string{"keys.go", "update.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, mm := range lit.FindAllStringSubmatch(string(src), -1) {
			if english.MatchString(mm[1]) {
				t.Errorf("%s: English notice %q", f, mm[1])
			}
		}
	}
}
