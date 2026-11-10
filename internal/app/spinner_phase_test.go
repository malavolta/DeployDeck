package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func pushModel(p pushPhase) Model {
	m := richModel()
	m.state, m.pushPhase, m.authState = StatePushPreparation, p, github.AuthAuthenticated
	return m
}

func gateModel(t *testing.T) Model {
	cfg := gatedQuickDeployConfig(true, config.GateConfig{Enabled: true, Approvers: []string{"alice"}})
	m, _, _, _ := quickDeployModel(t, cfg, "UAT", "UAT_SBX", "0AfGATE1")
	return typeString(m, "DESPLEGAR")
}

func TestSpinnerPhase_EntrySeedsOneTick(t *testing.T) {
	entries := []struct {
		name string
		m    Model
		key  string
	}{
		{"push", pushModel(pushConfirm), "p"},
		{"pr create", pushModel(pushPRConfirm), "y"},
		{"cleanup", func() Model { m := richModel(); m.state = StateTicketInput; m.ticket = ""; return m }(), "b"},
		{"queue load", func() Model { m := richModel(); m.state = StatePackageReview; return m }(), "enter"},
		{"queue refresh", func() Model { m := richModel(); m.state = StateQueueReview; return m }(), "r"},
		{"gate check", gateModel(t), "enter"},
		{"resume polling", historyModel([]runs.Record{{RunID: "r", Status: "InProgress", JobID: "J", CreatedAt: time.Now()}}), "enter"},
	}
	for _, e := range entries {
		t.Run(e.name, func(t *testing.T) {
			if e.m.spinnerActive() {
				t.Fatal("setup: must start outside a spinner phase")
			}
			nm, cmd := press(e.m, e.key)
			if !nm.spinnerActive() || !nm.spinning || !hasTick(cmd) {
				t.Errorf("entry must animate: active=%v spinning=%v tick=%v", nm.spinnerActive(), nm.spinning, hasTick(cmd))
			}
			if again, cmd := press(nm, "?"); hasTick(cmd) || again.spinning != nm.spinning {
				t.Error("a key on an already spinning model must not seed a tick")
			}
		})
	}
}

func TestSpinnerPhase_OneLoopAndNoSeedWhileSpinning(t *testing.T) {
	m := pushModel(pushConfirm)
	m.spinning = true
	if _, cmd := press(m, "p"); hasTick(cmd) {
		t.Error("a second spinner phase entry must not start a second tick loop")
	}
}

func TestSpinnerPhase_TickAnimatesAndStopsWithThePhase(t *testing.T) {
	m := pushModel(pushPushing)
	m.spinning = true
	first := m.View()
	nm, cmd := m.Update(spinnerTickMsg{})
	m2 := nm.(Model)
	if !isTick(cmd) || m2.View() == first || !strings.Contains(first, "\n  |\n") || !strings.Contains(m2.View(), "\n  /\n") {
		t.Errorf("tick must reschedule and advance the frame:\n%s\n%s", first, m2.View())
	}
	m2.pushPhase = pushReady
	nm, cmd = m2.Update(spinnerTickMsg{})
	if cmd != nil || nm.(Model).spinning {
		t.Error("a tick after the phase ended must return no command and clear spinning")
	}
}

func TestSpinnerPhase_ViewsShowGlyphOnlyWhileInPhase(t *testing.T) {
	glyph := "\n  |\n"
	cleanup := cleanupListModel()
	cleanup.cleanupPhase = cleanupLoading
	queue := richModel()
	queue.state, queue.queueLoading = StateQueueReview, true
	quick := gateModel(t)
	quick.gateCheckingRunID = "r1"
	for name, m := range map[string]Model{"pushing": pushModel(pushPushing), "pr creating": pushModel(pushPRCreating), "cleanup": cleanup, "queue": queue, "quick": quick} {
		if !strings.Contains(m.View(), glyph) {
			t.Errorf("%s: no spinner glyph line:\n%s", name, m.View())
		}
	}
	cleanup.cleanupPhase, queue.queueLoading, quick.gateCheckingRunID = cleanupBrowsing, false, ""
	for name, m := range map[string]Model{"pushReady": pushModel(pushReady), "cleanup": cleanup, "queue": queue, "quick": quick} {
		if strings.Contains(m.View(), glyph) {
			t.Errorf("%s: glyph must be gone after the phase:\n%s", name, m.View())
		}
	}
}

func TestSpinnerPhase_PollingKeepsTickButRendersNoGlyph(t *testing.T) {
	m := richModel()
	m.state, m.spinning = StateValidationPolling, true
	nm, cmd := m.Update(spinnerTickMsg{})
	if !isTick(cmd) || strings.Contains(nm.(Model).View(), "\n  |\n") || strings.Contains(nm.(Model).View(), "\n  /\n") {
		t.Error("polling must keep the tick alive without a glyph")
	}
}

func TestSpinnerPhase_QueueLoadingClearedOnEveryDonePath(t *testing.T) {
	for name, msg := range map[string]queueDoneMsg{"ok": {}, "error": {err: errors.New("boom")}, "permission": {err: salesforce.ErrQueuePermission}} {
		m := richModel()
		m.state, m.queueLoading = StateQueueReview, true
		nm, _ := m.Update(msg)
		if nm.(Model).queueLoading {
			t.Errorf("%s: queueLoading must be cleared", name)
		}
	}
	m := richModel()
	m.state = StatePackageReview
	if nm, _ := press(m, "enter"); !nm.queueLoading {
		t.Error("entering the queue must set queueLoading")
	}
}
