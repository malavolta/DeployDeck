package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/runs"
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

// richModel is a model whose lists all hold two rows, so navigation keys can
// act, with the editor hook wired and a resumable history.
func richModel() Model {
	m := New(Deps{Dir: "/repo", Config: testConfig(), Edit: func(string) tea.Cmd { return func() tea.Msg { return nil } }, NewChecker: func() (*prereq.Checker, error) { return nil, errStub }})
	m.branchList = []git.Branch{{Name: "a"}, {Name: "b"}}
	m.sandboxList = []string{"a", "b"}
	m.items = []git.CommitSelectionItem{{}, {}}
	m.destinations = []git.Destination{{Branch: "INT"}, {Branch: "UAT"}}
	m.repoState = git.RepoState{InProgress: true, Unmerged: conflictFiles("a.cls", "b.cls")}
	m.runs = []runs.Record{
		{RunID: "r1", Status: "InProgress", JobID: "J1", CreatedAt: time.Now()},
		{RunID: "r2", Status: "InProgress", JobID: "J2", CreatedAt: time.Now()},
	}
	m.plan.PromotionBranch = "deploy/DD-1"
	return m
}

// parityFixtures cover every State and sub-phase on top of richModel.
func parityFixtures() []struct {
	name string
	set  func(*Model)
} {
	push := func(p pushPhase, auth github.AuthState, ai bool, more func(*Model)) func(*Model) {
		return func(m *Model) {
			m.state, m.pushPhase, m.authState = StatePushPreparation, p, auth
			if ai {
				m.deps.GenerateSummary = stubAI
			}
			if more != nil {
				more(m)
			}
		}
	}
	set := func(s State, f func(*Model)) func(*Model) {
		return func(m *Model) { m.state = s; f(m) }
	}
	list := []struct {
		name string
		set  func(*Model)
	}{
		{"conflict continue on", set(StateCherryPickConflict, func(m *Model) { m.continueEnabled = true })},
		{"conflict continue off", set(StateCherryPickConflict, func(m *Model) { m.continuePending = []string{"x"} })},
		{"selection empty", set(StateCommitSelection, func(m *Model) { m.items = nil })},
		{"history empty", set(StateRunHistory, func(m *Model) { m.runs = nil })},
		{"history eligible", set(StateRunHistory, func(m *Model) {
			// Phase+Commits make Enter resumable while Status keeps x and r eligible.
			m.repoState.CurrentSHA = "a"
			m.runs = []runs.Record{{RunID: "r", Status: "Succeeded", Phase: "cherry-pick", TestLevel: "RunLocalTests", JobID: "J", Commits: []string{"a"}, CreatedAt: time.Now()}, m.runs[0]}
		})},
		{"quick deploy", set(StateQuickDeploy, quickReady)},
		{"quick deploy none", set(StateQuickDeploy, func(m *Model) { m.runs = nil })},
		{"aborted no branch", set(StateAborted, func(m *Model) { m.plan.PromotionBranch = "" })},
		{"succeeded no branch", set(StateSucceeded, func(m *Model) { m.plan.PromotionBranch = "" })},
		{"delta failed", set(StateDeltaGeneration, func(m *Model) { m.deltaErr = errStub })},
		{"validation start failed", set(StateValidationStart, func(m *Model) { m.validateErr = errStub })},
		{"package review empty", set(StatePackageReview, func(m *Model) { m.summary.Empty = true })},
		{"package review delta", set(StatePackageReview, func(m *Model) { m.standaloneMode = "delta" })},
		{"cleanup counting", func(m *Model) { *m = cleanupListModel(); m.cleanupPhase = cleanupCounting }},
		{"cleanup loading", func(m *Model) { *m = cleanupListModel(); m.cleanupPhase = cleanupLoading }},
		{"cleanup empty", func(m *Model) { *m = cleanupListModel(); m.cleanupBranches = nil }},
		{"push confirm", push(pushConfirm, github.AuthAuthenticated, false, nil)},
		{"pushing", push(pushPushing, github.AuthAuthenticated, false, nil)},
		{"pr confirm", push(pushPRConfirm, github.AuthAuthenticated, false, nil)},
		{"pr confirm ai ready", push(pushPRConfirm, github.AuthAuthenticated, true, func(m *Model) { m.aiTitle = "t" })},
		{"pr confirm ai pending", push(pushPRConfirm, github.AuthAuthenticated, true, func(m *Model) { m.aiPending = true })},
		{"help open", set(StateMainMenu, func(m *Model) { m.helpOpen = true })},
		{"queue failed", set(StateQueueReview, func(m *Model) { m.queueErr = errStub })},
		{"history terminal row", set(StateRunHistory, func(m *Model) {
			m.runs = []runs.Record{{RunID: "t", Status: "Succeeded", JobID: "J", CreatedAt: time.Now()}, m.runs[0]}
		})},
		{"ready authed accept", push(pushReady, github.AuthAuthenticated, true, func(m *Model) { m.aiTitle = "t" })},
	}
	for s := State(0); s < stateCount; s++ {
		list = append(list, struct {
			name string
			set  func(*Model)
		}{fmt.Sprintf("state %d", s), set(s, realistic(s))})
	}
	return append(list, subPhaseFixtures()...)
}

// quickReady gives the quick-deploy screen an eligible run and the execution
// permission, so Enter (with DESPLEGAR typed) really dispatches.
func quickReady(m *Model) {
	m.deps.Config.QuickDeploy.AllowExecution = true
	m.runs[0] = runs.Record{RunID: "q", Status: "Succeeded", Target: "UAT", TestLevel: "RunLocalTests", JobID: "J", CreatedAt: time.Now()}
}

// realistic returns the minimal valid setup so the primary Enter key of the
// state acts instead of only raising a validation notice.
func realistic(s State) func(*Model) {
	switch s {
	case StateCommitSelection:
		return func(m *Model) { m.items[0].Selected = true }
	case StateTargetSelection:
		return func(m *Model) { m.targetCursor = 1 } // UAT has a sandbox, INT has none
	case StatePickVerification:
		return func(m *Model) { m.repoState.InProgress = false }
	case StateQuickDeploy:
		return quickReady
	}
	return func(*Model) {}
}

func fixtureModel(set func(*Model)) Model {
	m := richModel()
	m.ticket, m.packagePath, m.cancelInput, m.quickConfirm, m.deleteConfirm = "DD-1", validPackagePath(), "CANCELAR", "DESPLEGAR", "BORRAR"
	set(&m)
	if len(m.cleanupBranches) == 1 {
		m.cleanupBranches = append(m.cleanupBranches, m.cleanupBranches[0])
	}
	return m
}

var (
	pkgOnce sync.Once
	pkgPath string
	pkgDir  string // removed by TestMain
)

// validPackagePath writes (once) a minimal package.xml so Enter on the
// package-path fixture acts without relying on a notice.
func validPackagePath() string {
	pkgOnce.Do(func() {
		var err error
		if pkgDir, err = os.MkdirTemp("", "dd-pkg"); err != nil {
			panic(err)
		}
		pkgPath = filepath.Join(pkgDir, "package.xml")
		x := `<?xml version="1.0"?><Package xmlns="http://soap.sforce.com/2006/04/metadata"><types><members>A</members><name>ApexClass</name></types><version>60.0</version></Package>`
		if err := os.WriteFile(pkgPath, []byte(x), 0o600); err != nil {
			panic(err)
		}
	})
	return pkgPath
}

// blockedNotice lists the (state, key) pairs that are enabled by design yet
// only set a Spanish notice. Every entry must be used by some fixture.
var blockedNotice = map[State][]string{
	// An empty package needs the explicit `o` override before Enter validates.
	StatePackageReview: {"enter"},
}

func isBlockedNotice(s State, k string) bool {
	for _, b := range blockedNotice[s] {
		if b == k {
			return true
		}
	}
	return false
}

func TestStateCount_Sentinel(t *testing.T) {
	for s := State(0); s < stateCount; s++ {
		probe := New(Deps{})
		probe.state = s
		if probe.viewBody() == "" {
			t.Errorf("state %d < stateCount has an empty view", s)
		}
	}
	for _, s := range []State{stateCount, stateCount + 1} {
		probe := New(Deps{})
		probe.state = s
		if probe.viewBody() != "" {
			t.Errorf("state %d >= stateCount has a view: stateCount is stale", s)
		}
	}
}

func TestParityFixtures_NamedSubPhases(t *testing.T) {
	have := map[string]bool{}
	for _, f := range parityFixtures() {
		have[f.name] = true
	}
	for _, n := range []string{"history terminal row", "ready authed ai after PR", "cleanup empty prune", "queue failed"} {
		if !have[n] {
			t.Errorf("missing parity fixture %q", n)
		}
	}
}

func isTyped(m Model) bool {
	switch m.state {
	case StateTicketInput, StatePackageSelect, StateCancelConfirm, StateQuickDeploy:
		return true
	}
	return m.cleanupPhase == cleanupStrongConfirm
}

// pressed reports whether k acts. A key on the explicit blocked-notice list
// may satisfy parity by producing its notice; nothing else may.
func pressed(base Model, k string, listed bool, used map[string]bool) bool {
	if listed {
		if nm, _ := press(base, k); nm.notice != "" {
			used[fmt.Sprint(base.state, k)] = true
			return true
		}
	}
	return acts(base, k)
}

// TestFooterParity_Structural: every state has bindings, the footer is exactly
// the enabled bindings, every enabled binding acts, and no key acts unless an
// enabled binding names it.
func TestFooterParity_Structural(t *testing.T) {
	fixtures := parityFixtures()
	usedBlocked := map[string]bool{}
	universe := map[string]bool{}
	covered := map[State]bool{}
	for _, f := range fixtures {
		m := fixtureModel(f.set)
		bs := m.bindings()
		if len(bs) == 0 {
			t.Errorf("%s: state %v has no bindings", f.name, m.state)
		}
		covered[m.state] = true
		for _, b := range bs {
			for _, k := range b.Keys() {
				universe[k] = true
			}
		}
	}
	for s := State(0); s < stateCount; s++ {
		if !covered[s] {
			t.Errorf("state %d has no fixture", s)
		}
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			m := fixtureModel(f.set)
			if want := footerOf(m.bindings()...); !strings.HasSuffix(m.View(), want) {
				t.Errorf("view does not end with the bindings footer %q:\n%s", want, m.View())
			}
			enabled := map[string]bool{}
			for _, b := range m.bindings() {
				if !b.Enabled() {
					continue
				}
				for _, k := range b.Keys() {
					enabled[k] = true
				}
				if b.Help().Key == "" {
					continue
				}
				ok := false
				for _, k := range b.Keys() {
					ok = ok || pressed(m, k, isBlockedNotice(m.state, k), usedBlocked)
				}
				if !ok {
					t.Errorf("enabled binding %q (%v) has no acting key", b.Help().Key, b.Keys())
				}
			}
			for k := range universe {
				if k == "ctrl+c" || (isTyped(m) && k != "enter" && k != "esc" && k != "backspace") {
					continue
				}
				if !enabled[k] && pressed(m, k, false, nil) {
					t.Errorf("key %q acts but no enabled binding names it", k)
				}
			}
		})
	}
	for st, ks := range blockedNotice {
		for _, k := range ks {
			if !usedBlocked[fmt.Sprint(st, k)] {
				t.Errorf("blockedNotice entry %v/%q is used by no fixture", st, k)
			}
		}
	}
}

// TestIsTyped_MatchesProduction keeps the test-side typed-state predicate from
// drifting away from the production one for every state and sub-phase.
func TestIsTyped_MatchesProduction(t *testing.T) {
	for _, f := range parityFixtures() {
		if m := fixtureModel(f.set); isTyped(m) != m.textInput() {
			t.Errorf("%s: isTyped=%v but textInput()=%v", f.name, isTyped(m), m.textInput())
		}
	}
}
