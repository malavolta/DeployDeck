package app

import (
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/runs"
)

// A terminal row neither advertises nor acts on Enter; the footer follows the cursor.
func TestHistoryResumeGuard_TerminalRowHasNoEnter(t *testing.T) {
	m := historyModel([]runs.Record{
		{RunID: "live", Status: "InProgress", JobID: "J1", CreatedAt: time.Now()},
		{RunID: "done", Status: "Succeeded", JobID: "J2", CreatedAt: time.Now()},
	})
	if !strings.Contains(m.View(), "Enter reanudar") {
		t.Fatalf("resumable row must advertise Enter:\n%s", m.View())
	}
	m, _ = press(m, "down")
	if strings.Contains(m.View(), "Enter reanudar") {
		t.Fatalf("terminal row must not advertise Enter:\n%s", m.View())
	}
	nm, cmd := press(m, "enter")
	if cmd != nil || nm.state != StateRunHistory || nm.runsCursor != m.runsCursor || nm.View() != m.View() {
		t.Fatalf("Enter on a terminal row must be inert: cmd=%v state=%v", cmd, nm.state)
	}
	m, _ = press(m, "up")
	if !strings.Contains(m.View(), "Enter reanudar") {
		t.Fatal("Enter reanudar must return when the cursor moves back")
	}
}
