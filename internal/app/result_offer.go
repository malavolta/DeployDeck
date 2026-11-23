package app

import (
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// deployOffer classifies the result-screen quick-deploy offer (ungated-env-deploy).
type deployOffer int

const (
	offerHidden deployOffer = iota
	offerUnavailable
	offerAvailable
)

const execDisabledReasonES = "la ejecución está deshabilitada (quickDeploy.allowExecution)"

// resultDeployOffer is the pure offer predicate. Gated and production targets
// never see the offer (they deploy through run history only); otherwise the
// run's eligibility and the execution gate decide available vs unavailable.
func resultDeployOffer(cfg config.Config, rec runs.Record, now time.Time) (deployOffer, string) {
	if _, gated := cfg.GateFor(rec.Target); gated || git.IsProductionTarget(cfg, rec.Target) {
		return offerHidden, ""
	}
	if ok, reason := runs.QuickDeployEligible(rec, now); !ok {
		return offerUnavailable, quickDeployReasonES(reason)
	}
	if !quickDeployExecAllowed(cfg, false) {
		return offerUnavailable, execDisabledReasonES
	}
	return offerAvailable, ""
}

// upsertResultRun loads the just-finished run and puts it into m.runs (replace
// by RunID, otherwise prepend), overlaying the live Salesforce status in case
// the best-effort AppendReport left a stale one. m.runsCursor is untouched. A
// nil writer or a Load error leaves m.runs as is, so the offer stays hidden.
func (m Model) upsertResultRun(status string) Model {
	if m.deps.Runs == nil || m.runID == "" {
		return m
	}
	rec, err := m.deps.Runs.Load(m.runID)
	if err != nil {
		return m
	}
	if status != "" {
		rec.Status = status
	}
	rows := append([]runs.Record(nil), m.runs...)
	for i := range rows {
		if rows[i].RunID == rec.RunID {
			rows[i] = rec
			m.runs = rows
			return m
		}
	}
	m.runs = append([]runs.Record{rec}, rows...)
	return m
}

// resultRunIndex returns the m.runs row of the current run, or -1.
func (m Model) resultRunIndex() int {
	for i, r := range m.runs {
		if r.RunID == m.runID && m.runID != "" {
			return i
		}
	}
	return -1
}

// resultOffer is the offer for the current StateSucceeded screen. Hidden unless
// the finished run is loaded in m.runs with its JobID and alias persisted.
func (m Model) resultOffer() (deployOffer, string) {
	i := m.resultRunIndex()
	if m.state != StateSucceeded || i < 0 {
		return offerHidden, ""
	}
	rec := m.runs[i]
	if rec.JobID == "" || rec.Alias == "" {
		return offerHidden, ""
	}
	return resultDeployOffer(m.deps.Config, rec, m.now())
}
