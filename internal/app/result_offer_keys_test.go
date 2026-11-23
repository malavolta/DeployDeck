package app

import (
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// offerReady is a StateSucceeded model with the finished run loaded at index 1
// (behind a decoy row) so the cursor assertion is meaningful.
func offerReady(t *testing.T, cfg func(*config.Config), rec func(*runs.Record)) Model {
	t.Helper()
	m, _ := offerModel(t, offerRec(rec))
	if cfg != nil {
		cfg(&m.deps.Config)
	}
	m.runs = []runs.Record{{RunID: "decoy"}}
	return m.upsertResultRun("Succeeded")
}

func TestKeySucceeded_X(t *testing.T) {
	noExec := func(c *config.Config) { c.QuickDeploy.AllowExecution = false }
	gated := func(c *config.Config) { c.Gates = map[string]config.GateConfig{"UAT": {Enabled: true}} }
	cases := []struct {
		name string
		cfg  func(*config.Config)
		rec  func(*runs.Record)
		open bool
	}{
		{"available opens the confirmation", nil, nil, true},
		{"unavailable (ineligible) is a no-op", nil, func(r *runs.Record) { r.TestLevel = "" }, false},
		{"unavailable (execution disabled) is a no-op", noExec, nil, false},
		{"gated is a no-op", gated, nil, false},
		{"production is a no-op", nil, func(r *runs.Record) { r.Target = "main" }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := offerReady(t, c.cfg, c.rec)
			nm, cmd := press(m, "x")
			if cmd != nil || nm.quickDeployingRunID != "" || nm.gateCheckingRunID != "" {
				t.Fatalf("x must never execute: cmd=%v", cmd)
			}
			if !c.open {
				if nm.state != StateSucceeded || nm.notice != "" || nm.quickFromResult {
					t.Fatalf("x must be a strict no-op, state=%v notice=%q", nm.state, nm.notice)
				}
				return
			}
			if nm.state != StateQuickDeploy || !nm.quickFromResult || nm.runs[nm.runsCursor].RunID != "run-1" {
				t.Fatalf("state=%v fromResult=%v cursor=%d", nm.state, nm.quickFromResult, nm.runsCursor)
			}
		})
	}
}

func TestQuickDeploy_EscQReturnToOrigin(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		m, cmd := press(offerReady(t, nil, nil), "x")
		m, cmd = press(m, k)
		if m.state != StateSucceeded || m.quickFromResult || isQuit(cmd) {
			t.Errorf("%s from result: state=%v flag=%v quit=%v", k, m.state, m.quickFromResult, isQuit(cmd))
		}
		h := New(Deps{Dir: "/repo", Config: quickDeployConfig(true, false), Now: func() time.Time { return offerNow }})
		h.state, h.quickFromResult = StateRunHistory, true // stale flag must be reset by the history x
		h.runs = []runs.Record{offerRec(nil)}
		h, _ = press(h, "x")
		if h.state != StateQuickDeploy || h.quickFromResult {
			t.Fatalf("history x: state=%v flag=%v", h.state, h.quickFromResult)
		}
		if h, cmd = press(h, k); h.state != StateRunHistory || isQuit(cmd) {
			t.Errorf("%s from history: state=%v", k, h.state)
		}
	}
}

// TestResultDeploy_E2E drives the whole flow through a real writer and a fake
// sf runner: validation success -> x -> DESPLEGAR -> quick deploy -> offer
// -> Esc -> the offer turns into "already deployed". A target whose gate is
// explicitly disabled behaves like an ungated one: no gh, no gate check.
func TestResultDeploy_E2E(t *testing.T) {
	for _, tc := range []struct {
		name  string
		gates map[string]config.GateConfig
	}{
		{"no gate entry", nil},
		{"gate disabled", map[string]config.GateConfig{"UAT": {Enabled: false}}},
	} {
		t.Run(tc.name, func(t *testing.T) { resultDeployE2E(t, tc.gates) })
	}
}

func resultDeployE2E(t *testing.T, gates map[string]config.GateConfig) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	rec := offerRec(func(r *runs.Record) { r.Status = "Queued" })
	if _, err := writer.Create(rec, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	fr := execpkg.NewFakeRunner()
	fr.When("sf", quickDeployArgs(rec.JobID, rec.Alias), quickDeploySuccess(rec.JobID))
	cfg := quickDeployConfig(true, false)
	cfg.Gates = gates
	m := New(Deps{Dir: dir, Config: cfg, SF: salesforce.New(fr), Runs: writer, Now: func() time.Time { return offerNow }})
	m.state, m.runID = StateValidationPolling, rec.RunID

	m = advance(t, m, reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded", Raw: "{}"}})
	if got, _ := m.resultOffer(); m.state != StateSucceeded || got != offerAvailable {
		t.Fatalf("state=%v offer=%v", m.state, got)
	}
	m, _ = press(m, "x")
	m = typeString(m, "DESPLEGAR")
	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("confirmed deploy should dispatch")
	}
	if m.gateCheckingRunID != "" {
		t.Fatalf("no gate check expected, gateCheckingRunID=%q", m.gateCheckingRunID)
	}
	m = advance(t, m, run(t, cmd))
	if len(fr.Calls) != 1 || fr.Calls[0].Name != "sf" {
		t.Fatalf("expected exactly 1 sf call (no gh), got %+v", fr.Calls)
	}
	if saved, _ := writer.Load(rec.RunID); saved.QuickDeployedAt.IsZero() {
		t.Error("QuickDeployedAt should be recorded")
	}
	if m, _ = press(m, "esc"); m.state != StateSucceeded {
		t.Fatalf("esc should return to the result screen, got %v", m.state)
	}
	if got, reason := m.resultOffer(); got != offerUnavailable || reason != quickDeployReasonES("already quick-deployed") {
		t.Fatalf("offer=%v reason=%q", got, reason)
	}
}
