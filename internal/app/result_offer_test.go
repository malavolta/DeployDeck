package app

import (
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

var offerNow = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

func offerRec(f func(*runs.Record)) runs.Record {
	r := runs.Record{
		RunID: "run-1", Target: "UAT", Alias: "UAT_SBX", JobID: "0AfJOB", Status: "Succeeded",
		TestLevel: "RunLocalTests", CreatedAt: offerNow.Add(-time.Hour),
	}
	if f != nil {
		f(&r)
	}
	return r
}

func TestResultDeployOffer(t *testing.T) {
	gated := func(enabled bool) func(*config.Config) {
		return func(c *config.Config) { c.Gates = map[string]config.GateConfig{"UAT": {Enabled: enabled}} }
	}
	const disabledReason = "la ejecución está deshabilitada (quickDeploy.allowExecution)"
	cases := []struct {
		name   string
		cfg    func(*config.Config)
		rec    func(*runs.Record)
		want   deployOffer
		reason string
	}{
		{"all good", nil, nil, offerAvailable, ""},
		{"gated hidden", gated(true), nil, offerHidden, ""},
		{"gate disabled behaves ungated", gated(false), nil, offerAvailable, ""},
		{"production hidden", nil, func(r *runs.Record) { r.Target = "main" }, offerHidden, ""},
		{"unmapped branch hidden", nil, func(r *runs.Record) { r.Target = "feature-x" }, offerHidden, ""},
		{"empty target hidden", nil, func(r *runs.Record) { r.Target = "" }, offerHidden, ""},
		{"no test level", nil, func(r *runs.Record) { r.TestLevel = "" }, offerUnavailable, quickDeployReasonES("required tests not run")},
		{"expired", nil, func(r *runs.Record) { r.CreatedAt = offerNow.Add(-11 * 24 * time.Hour) }, offerUnavailable, quickDeployReasonES("older than 10 days")},
		{"already deployed", nil, func(r *runs.Record) { r.QuickDeployedAt = offerNow }, offerUnavailable, quickDeployReasonES("already quick-deployed")},
		{"execution disabled", func(c *config.Config) { c.QuickDeploy.AllowExecution = false }, nil, offerUnavailable, disabledReason},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := quickDeployConfig(true, false)
			if c.cfg != nil {
				c.cfg(&cfg)
			}
			got, reason := resultDeployOffer(cfg, offerRec(c.rec), offerNow)
			if got != c.want || reason != c.reason {
				t.Fatalf("got (%v, %q), want (%v, %q)", got, reason, c.want, c.reason)
			}
		})
	}
}

// offerModel is a StateSucceeded model whose run is persisted via a real writer.
func offerModel(t *testing.T, rec runs.Record) (Model, *runs.Writer) {
	t.Helper()
	w := runs.NewWriter(t.TempDir())
	if _, err := w.Create(rec, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	m := New(Deps{Dir: t.TempDir(), Config: quickDeployConfig(true, false), Runs: w, Now: func() time.Time { return offerNow }})
	m.state, m.runID = StateSucceeded, rec.RunID
	return m, w
}

func TestUpsertResultRun(t *testing.T) {
	t.Run("prepends and overlays live status, cursor untouched", func(t *testing.T) {
		m, _ := offerModel(t, offerRec(func(r *runs.Record) { r.Status = "Queued" }))
		m.runs = []runs.Record{{RunID: "old"}}
		m.runsCursor = 1
		m = m.upsertResultRun("Succeeded")
		if len(m.runs) != 2 || m.runs[0].RunID != "run-1" || m.runs[0].Status != "Succeeded" || m.runsCursor != 1 {
			t.Fatalf("runs=%+v cursor=%d", m.runs, m.runsCursor)
		}
		if got, _ := m.resultOffer(); got != offerAvailable {
			t.Fatalf("offer = %v, want available", got)
		}
	})
	t.Run("replaces the row with the same RunID in place", func(t *testing.T) {
		m, _ := offerModel(t, offerRec(nil))
		m.runs = []runs.Record{{RunID: "a"}, {RunID: "run-1", Status: "stale"}, {RunID: "b"}}
		m = m.upsertResultRun("Succeeded")
		if len(m.runs) != 3 || m.runs[1].Status != "Succeeded" || m.runs[1].JobID != "0AfJOB" {
			t.Fatalf("runs=%+v", m.runs)
		}
	})
	t.Run("nil writer or load error leaves the offer hidden", func(t *testing.T) {
		m, _ := offerModel(t, offerRec(nil))
		m.runID = "missing"
		m = m.upsertResultRun("Succeeded")
		m.deps.Runs = nil
		m = m.upsertResultRun("Succeeded")
		if got, _ := m.resultOffer(); got != offerHidden || len(m.runs) != 0 {
			t.Fatalf("offer=%v runs=%+v", got, m.runs)
		}
	})
}

func TestResultOffer_Guards(t *testing.T) {
	cases := []struct {
		name string
		rec  func(*runs.Record)
		set  func(*Model)
		want deployOffer
	}{
		{"available", nil, nil, offerAvailable},
		{"wrong state", nil, func(m *Model) { m.state = StateFailed }, offerHidden},
		{"other run id", nil, func(m *Model) { m.runID = "other" }, offerHidden},
		{"missing job id", func(r *runs.Record) { r.JobID = "" }, nil, offerHidden},
		{"missing alias", func(r *runs.Record) { r.Alias = "" }, nil, offerHidden},
		{"missing test level", func(r *runs.Record) { r.TestLevel = "" }, nil, offerUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, _ := offerModel(t, offerRec(c.rec))
			m = m.upsertResultRun("Succeeded")
			if c.set != nil {
				c.set(&m)
			}
			if got, _ := m.resultOffer(); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestOnReportDone_UpsertsOnlyOnSuccess(t *testing.T) {
	for _, c := range []struct {
		status string
		loaded bool
	}{{"Succeeded", true}, {"SucceededPartial", true}, {"Failed", false}, {"Canceled", false}} {
		m, _ := offerModel(t, offerRec(func(r *runs.Record) { r.Status = "Queued" }))
		m.state = StateValidationPolling
		next, _ := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: c.status, Raw: "{}"}})
		if got := len(next.(Model).runs) == 1; got != c.loaded {
			t.Errorf("%s: loaded=%v, want %v", c.status, got, c.loaded)
		}
	}
}

// A history reload replaces m.runs and resets the cursor, so the upserted
// result row never leaks into history.
func TestUpsertResultRun_HistoryReloadReplaces(t *testing.T) {
	m, _ := offerModel(t, offerRec(nil))
	m = m.upsertResultRun("Succeeded")
	m.state, m.runsCursor = StateRunHistory, 0
	next, _ := m.Update(runHistoryLoadedMsg{records: []runs.Record{{RunID: "disk"}}})
	nm := next.(Model)
	if len(nm.runs) != 1 || nm.runs[0].RunID != "disk" || nm.runsCursor != 0 {
		t.Fatalf("runs=%+v cursor=%d", nm.runs, nm.runsCursor)
	}
}

func TestViewValidationResult_OfferLine(t *testing.T) {
	cases := []struct {
		name string
		rec  func(*runs.Record)
		cfg  func(*config.Config)
		want string // "" means no offer line at all
	}{
		{"available", nil, nil, "Entorno sin gate: x despliega esta validación (quick deploy, requiere DESPLEGAR)"},
		{"ineligible", func(r *runs.Record) { r.TestLevel = "" }, nil, "Despliegue no disponible: " + quickDeployReasonES("required tests not run")},
		{"execution disabled", nil, func(c *config.Config) { c.QuickDeploy.AllowExecution = false }, "Despliegue no disponible: " + execDisabledReasonES},
		{"production hidden", func(r *runs.Record) { r.Target = "main" }, nil, ""},
		{"gated hidden", nil, func(c *config.Config) { c.Gates = map[string]config.GateConfig{"UAT": {Enabled: true}} }, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := offerReady(t, c.cfg, c.rec)
			v := m.View()
			if c.want == "" {
				if strings.Contains(v, "Entorno sin gate") || strings.Contains(v, "Despliegue no disponible") {
					t.Fatalf("no offer line expected:\n%s", v)
				}
				return
			}
			if !strings.Contains(v, c.want) {
				t.Fatalf("missing %q in:\n%s", c.want, v)
			}
		})
	}
}
