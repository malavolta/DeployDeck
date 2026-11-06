package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func stubAI(context.Context, string, []string, string) (string, string, error) {
	return "t", "d", nil
}

// subPhaseFixtures are the sub-phase screens whose footer parity is checked:
// delete confirmations on Succeeded/Aborted, cleanup confirm/prune, and
// push-ready with and without AI, on both the authed and compare branches.
func subPhaseFixtures() []struct {
	name string
	set  func(*Model)
} {
	del := func(s State, p cleanupPhase) func(*Model) {
		return func(m *Model) {
			m.state, m.cleanupPhase = s, p
			m.plan.PromotionBranch = "deploy/DD-1"
		}
	}
	ready := func(auth github.AuthState, ai bool) func(*Model) {
		return func(m *Model) {
			m.state, m.pushPhase, m.authState = StatePushPreparation, pushReady, auth
			m.plan.PromotionBranch = "deploy/DD-1"
			if ai {
				m.deps.GenerateSummary = stubAI
			}
		}
	}
	return []struct {
		name string
		set  func(*Model)
	}{
		{"succeeded confirm", del(StateSucceeded, cleanupConfirm)},
		{"succeeded strong", func(m *Model) { del(StateSucceeded, cleanupStrongConfirm)(m); m.deleteConfirm = deleteConfirmWord }},
		{"aborted confirm", del(StateAborted, cleanupConfirm)},
		{"aborted strong", func(m *Model) { del(StateAborted, cleanupStrongConfirm)(m); m.deleteConfirm = deleteConfirmWord }},
		{"succeeded idle", del(StateSucceeded, cleanupIdle)},
		{"cleanup confirm", func(m *Model) {
			*m = cleanupListModel()
			m.cleanupPhase, m.cleanupDeleteTarget = cleanupConfirm, "deploy/DD-7"
		}},
		{"cleanup prune", func(m *Model) { *m = cleanupListModel(); m.cleanupPhase = cleanupPruneConfirm }},
		{"cleanup list", func(m *Model) { *m = cleanupListModel() }},
		{"ready authed ai", ready(github.AuthAuthenticated, true)},
		{"ready authed ai after PR", func(m *Model) { ready(github.AuthAuthenticated, true)(m); m.prURL = "https://pr/1" }},
		{"cleanup empty prune", func(m *Model) {
			*m = cleanupListModel()
			m.cleanupBranches, m.cleanupPhase = nil, cleanupPruneConfirm
		}},
		{"ready authed no ai", ready(github.AuthAuthenticated, false)},
		{"ready compare ai", ready(github.AuthUnauthenticated, true)},
		{"ready compare no ai", ready(github.AuthUnauthenticated, false)},
	}
}

func acts(base Model, key string) bool {
	nm, cmd := press(base, key)
	nm.notice = ""
	return cmd != nil || nm.state != base.state || nm.pushPhase != base.pushPhase ||
		nm.cleanupPhase != base.cleanupPhase || nm.View() != base.View()
}

func TestSubPhase_DeletePromptOnSucceededAndAborted(t *testing.T) {
	for _, s := range []State{StateSucceeded, StateAborted} {
		m := New(Deps{Dir: "/repo"})
		m.state, m.cleanupPhase = s, cleanupConfirm
		m.plan.PromotionBranch = "deploy/DD-9"
		v := m.View()
		if !strings.Contains(v, "¿Borrar la rama deploy/DD-9?") {
			t.Errorf("%v confirm: no visible prompt:\n%s", s, v)
		}
		for _, banned := range []string{"p preparar push", "d borrar rama", "Enter/q/Esc salir"} {
			if strings.Contains(v, banned) {
				t.Errorf("%v confirm: footer still shows %q:\n%s", s, banned, v)
			}
		}
		m.cleanupPhase = cleanupStrongConfirm
		m.deleteConfirm = "BOR"
		v = m.View()
		for _, want := range []string{"deploy/DD-9", "BORRAR", "BOR_", "Enter confirmar", "Esc cancelar"} {
			if !strings.Contains(v, want) {
				t.Errorf("%v strong: missing %q:\n%s", s, want, v)
			}
		}
		if strings.Contains(v, "p preparar push") || strings.Contains(v, "Enter/q/Esc salir") {
			t.Errorf("%v strong: normal footer visible:\n%s", s, v)
		}
	}
}

func TestSubPhase_PushReadyFooterFollowsAI(t *testing.T) {
	for _, auth := range []github.AuthState{github.AuthAuthenticated, github.AuthUnauthenticated} {
		for _, ai := range []bool{true, false} {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			m.state, m.pushPhase, m.authState = StatePushPreparation, pushReady, auth
			m.plan.PromotionBranch = "deploy/DD-1"
			if ai {
				m.deps.GenerateSummary = stubAI
			}
			keys, text, _ := footerKeys(m.View())
			if keys["a"] != ai {
				t.Errorf("auth=%v ai=%v: footer %q a=%v", auth, ai, text, keys["a"])
			}
			if ai {
				if nm, cmd := press(m, "a"); cmd == nil || !nm.aiPending {
					t.Errorf("a with AI must request a suggestion")
				}
			}
		}
	}
}

func TestSubPhase_CommitSelectionFooter(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateCommitSelection
	m.items = []git.CommitSelectionItem{{}}
	keys, text, _ := footerKeys(m.View())
	if !strings.Contains(text, "↑/↓") || !keys["space"] || !keys["x"] {
		t.Errorf("selection footer must list ↑/↓ and Space/x: %q", text)
	}
}

func TestSubPhase_CleanupColumnsFitHeaders(t *testing.T) {
	m := cleanupListModel()
	m.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: "deploy/DD-1", Pushed: true, LastCommit: time.Now()}, MergedLabel: "desconocido"},
		{DeployBranch: git.DeployBranch{Name: "deploy/DD-2", LastCommit: time.Now()}, MergedLabel: "abandonada"},
	}
	lines := strings.Split(m.View(), "\n")
	var hdr string
	var rows []string
	for _, l := range lines {
		switch {
		case strings.Contains(l, "Antigüedad"):
			hdr = l
		case strings.Contains(l, "deploy/DD-"):
			rows = append(rows, l)
		}
	}
	if hdr == "" || len(rows) != 2 {
		t.Fatalf("setup:\n%s", m.View())
	}
	col := func(l, tok string) int { return len([]rune(l[:strings.Index(l, tok)])) }
	if col(hdr, "Estado") != col(rows[0], "pusheada") || col(hdr, "Estado") != col(rows[1], "sin push") {
		t.Errorf("Estado misaligned:\n%s\n%s\n%s", hdr, rows[0], rows[1])
	}
	if col(hdr, "Merge") != col(rows[0], "desconocido") || col(hdr, "Merge") != col(rows[1], "abandonada") {
		t.Errorf("Merge misaligned:\n%s\n%s\n%s", hdr, rows[0], rows[1])
	}
	if col(hdr, "Estado") <= col(hdr, "Antigüedad")+len([]rune("Antigüedad")) {
		t.Errorf("Antigüedad must be followed by a gap:\n%s", hdr)
	}
}

func TestSubPhase_HistoryDestinoFitsHeader(t *testing.T) {
	m := historyModel([]runs.Record{{RunID: "r", Ticket: "DD-1", Target: "UAT", Status: "Failed", JobID: "0Af", CreatedAt: time.Now()}})
	var hdr, row string
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(l, "Destino") {
			hdr = l
		}
		if row == "" && strings.Contains(l, "DD-1") {
			row = l
		}
	}
	col := func(l, tok string) int { return len([]rune(l[:strings.Index(l, tok)])) }
	if !strings.Contains(hdr, "Destino ") || col(hdr, "Estado") != col(row, "Failed") {
		t.Errorf("Destino must fit its header and keep Estado aligned:\n%s\n%s", hdr, row)
	}
}

func TestSubPhase_RemainingEnglishCopy(t *testing.T) {
	if mergedLabel(false, false, nil) != "desconocida" || mergedLabel(true, true, nil) != "probablemente fusionada" || mergedLabel(true, false, nil) != "abandonada" {
		t.Errorf("mergedLabel must be Spanish: %q %q %q", mergedLabel(false, false, nil), mergedLabel(true, true, nil), mergedLabel(true, false, nil))
	}
	sel := New(Deps{Dir: "/repo", Config: testConfig()})
	sel.rePromoteMissing = []string{"abc"}
	if w := strings.Join(sel.selectionWarnings(), "\n"); strings.Contains(w, "commit(s) from the prior run") || !strings.Contains(w, "run anterior") {
		t.Errorf("re-promote warning must be Spanish: %q", w)
	}
	q := New(Deps{Dir: "/repo", Config: testConfig()})
	q.state = StateQueueReview
	q.queue = []salesforce.DeployQueueEntry{{JobID: "0Af1", CheckOnly: true}, {JobID: "0Af2"}}
	qv := q.View()
	for _, banned := range []string{"Deploy ", "Validate ", "Components "} {
		if strings.Contains(qv, banned) {
			t.Errorf("queue shows English %q:\n%s", banned, qv)
		}
	}
	for _, want := range []string{"Despliegue", "Validación", "Componentes"} {
		if !strings.Contains(qv, want) {
			t.Errorf("queue lacks %q:\n%s", want, qv)
		}
	}
	res := New(Deps{Dir: "/repo"})
	res.state, res.timedOut = StateFailed, true
	if strings.Contains(res.View(), "timed out") {
		t.Errorf("timeout line must be Spanish:\n%s", res.View())
	}
	ai := New(Deps{Dir: "/repo", GenerateSummary: stubAI})
	ai.aiAccepted = true
	if strings.Contains(ai.viewAIBlock(), "title de arriba") {
		t.Errorf("AI block must be Spanish: %s", ai.viewAIBlock())
	}
}

func TestSubPhase_RNoticeOnCanceledAndAborted(t *testing.T) {
	for _, st := range []string{"Canceled", "Aborted"} {
		m := historyModel([]runs.Record{{RunID: "r", Status: st, JobID: "0Af", Commits: []string{"a"}, CreatedAt: time.Now()}})
		nm, cmd := press(m, "r")
		if nm.state != StateRunHistory || cmd != nil || !strings.Contains(nm.View(), "exitosos") {
			t.Errorf("r on %s: state=%v cmd=%v\n%s", st, nm.state, cmd, nm.View())
		}
	}
}

func TestSubPhase_EscOnCleanupResultView(t *testing.T) {
	m := cleanupListModel()
	m.cleanupBranches = nil
	m.cleanupNotice = "Se borró deploy/DD-7"
	nm, cmd := press(m, "esc")
	if nm.state != StateMainMenu || isQuit(cmd) {
		t.Errorf("esc on result view: state=%v quit=%v", nm.state, isQuit(cmd))
	}
}

// TestSubPhase_DeclinedResumeIsOfferedAgain is task 3.8: esc declines the
// resume offer, q quits, the run record is untouched, and a new Model on the
// same store offers the run again.
func TestSubPhase_DeclinedResumeIsOfferedAgain(t *testing.T) {
	store := runs.NewWriter(t.TempDir())
	if _, err := store.Create(runs.Record{RunID: "live", Ticket: "DD-1", JobID: "JOB1", Status: "InProgress", Phase: "validating", CreatedAt: time.Now()}, nil); err != nil {
		t.Fatal(err)
	}
	launch := func() Model {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: store})
		m.state = StateMainMenu
		records, err := store.List()
		if err != nil {
			t.Fatal(err)
		}
		return advance(t, m, resumeDetectMsg{state: git.RepoState{Clean: true}, records: records})
	}
	m := launch()
	if m.state != StateRunHistory {
		t.Fatalf("resume must be offered via history, got %v", m.state)
	}
	m, cmd := press(m, "esc")
	if m.state != StateMainMenu || isQuit(cmd) {
		t.Fatalf("esc declines to the menu, state=%v quit=%v", m.state, isQuit(cmd))
	}
	if _, cmd := press(m, "q"); !isQuit(cmd) {
		t.Fatal("q must quit")
	}
	rec, err := store.Load("live")
	if err != nil || rec.Status != "InProgress" || rec.Phase != "validating" {
		t.Fatalf("record must be untouched, got %+v err=%v", rec, err)
	}
	if again := launch(); again.state != StateRunHistory {
		t.Fatalf("next launch must offer the run again, got %v", again.state)
	}
}

func TestSubPhase_PersistedPhaseShownInSpanish(t *testing.T) {
	for phase, want := range map[string]string{"git-conflict": "conflicto", "aborted": "abortado", "done": "completado", "validating": "validando", "cherry-pick": "cherry-pick", "other": "other"} {
		if got := runProgressLabel(runs.Record{Phase: phase}); got != want {
			t.Errorf("phase %q shown as %q, want %q", phase, got, want)
		}
	}
}

// TestSubPhase_QFromMainFlowKeepsRunResumable is task 3.6: q from a main-flow
// screen quits and leaves the persisted run record untouched.
func TestSubPhase_QFromMainFlowKeepsRunResumable(t *testing.T) {
	store := runs.NewWriter(t.TempDir())
	if _, err := store.Create(runs.Record{RunID: "live", Ticket: "DD-1", Phase: "cherry-pick", CreatedAt: time.Now()}, nil); err != nil {
		t.Fatal(err)
	}
	m := New(Deps{Dir: t.TempDir(), Config: testConfig(), Runs: store})
	m.state = StatePlanPreview
	if _, cmd := press(m, "q"); !isQuit(cmd) {
		t.Fatal("q must quit")
	}
	recs, err := store.List()
	if err != nil || len(recs) != 1 || recs[0].Phase != "cherry-pick" {
		t.Fatalf("run must stay resumable: %+v err=%v", recs, err)
	}
}
