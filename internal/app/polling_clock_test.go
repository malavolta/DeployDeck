package app

import (
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func TestPollingClock_Format(t *testing.T) {
	for d, want := range map[time.Duration]string{65 * time.Second: "01:05", 61*time.Minute + 5*time.Second: "61:05", 0: "00:00", -3 * time.Second: "00:00"} {
		if got := clock(d); got != want {
			t.Errorf("clock(%v) = %q, want %q", d, got, want)
		}
	}
}

func pollingAt(t *testing.T) (Model, *fakeClock) {
	clk := &fakeClock{t: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)}
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Now: clk.now})
	m.state = StateValidationStart
	nm, _ := m.Update(validateDoneMsg{result: salesforce.ValidateResult{JobID: "J1"}})
	return nm.(Model), clk
}

func TestPollingClock_ElapsedAndLastPoll(t *testing.T) {
	m, clk := pollingAt(t)
	if v := m.View(); !strings.Contains(v, "Transcurrido: 00:00") || !strings.Contains(v, "Último sondeo: pendiente") || strings.Contains(v, "hace") {
		t.Errorf("a fresh poll shows 00:00 and pendiente:\n%s", v)
	}
	clk.advance(65 * time.Second)
	if v := m.View(); !strings.Contains(v, "Transcurrido: 01:05") {
		t.Errorf("65s elapsed must read 01:05:\n%s", v)
	}
	nm, _ := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: "InProgress"}})
	m = nm.(Model)
	if v := m.View(); !strings.Contains(v, "hace 0s") {
		t.Errorf("a new report reads hace 0s:\n%s", v)
	}
	clk.advance(7 * time.Second)
	if v := m.View(); !strings.Contains(v, "hace 7s") || !strings.Contains(v, "Transcurrido: 01:12") {
		t.Errorf("7s after the report:\n%s", v)
	}
	nm, _ = m.Update(reportDoneMsg{err: errStub})
	if v := nm.(Model).View(); !strings.Contains(v, "hace 7s") {
		t.Errorf("an errored report must leave the age unchanged:\n%s", v)
	}
	clk.advance(60*time.Minute - 7*time.Second)
	if v := m.View(); !strings.Contains(v, "Transcurrido: 61:05") {
		t.Errorf("long jobs keep counting:\n%s", v)
	}
}

func TestPollingClock_TerminalDropsLiveLinesAndResumeRestarts(t *testing.T) {
	m, clk := pollingAt(t)
	nm, _ := m.Update(reportDoneMsg{report: salesforce.DeployReport{Status: "Succeeded"}})
	if v := nm.(Model).View(); strings.Contains(v, "Transcurrido") || strings.Contains(v, "hace") {
		t.Errorf("a terminal screen has no live lines:\n%s", v)
	}
	clk.advance(10 * time.Minute)
	h := historyModel([]runs.Record{{RunID: "r", Status: "InProgress", JobID: "J", CreatedAt: clk.now()}})
	h.deps.Now = clk.now
	r, _ := press(h, "enter")
	if !strings.Contains(r.View(), "Transcurrido: 00:00") {
		t.Errorf("resume restarts elapsed from the re-attach time:\n%s", r.View())
	}
}
