package app

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

func sized(m Model, w, h int) Model {
	nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return nm.(Model)
}

func manyRuns(n int) Model {
	rs := make([]runs.Record, n)
	for i := range rs {
		rs[i] = runs.Record{RunID: fmt.Sprintf("r%02d", i), Ticket: fmt.Sprintf("DD-%02d", i), Status: "InProgress", JobID: fmt.Sprintf("J%02d", i), CreatedAt: time.Now()}
	}
	return historyModel(rs)
}

func manyCommits(n int) Model {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateCommitSelection
	for i := 0; i < n; i++ {
		m.items = append(m.items, git.CommitSelectionItem{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{ShortSHA: fmt.Sprintf("c%03d", i), Subject: fmt.Sprintf("subject %d", i)}}})
	}
	return m
}

// maxWidth is the widest line containing marker (data rows; chrome is never cut).
func maxWidth(v, marker string) int {
	w := 0
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, marker) {
			w = max(w, ansi.StringWidth(l))
		}
	}
	return w
}

var downIndicator = regexp.MustCompile(`↓ \d+ más`)

func TestWindowSize_StoredWithNoCmd(t *testing.T) {
	for _, s := range []State{StateMainMenu, StateRunHistory, StateCommitSelection, StateValidationPolling} {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = s
		nm, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		got := nm.(Model)
		if cmd != nil || got.width != 80 || got.height != 24 || got.state != s {
			t.Errorf("state %v: cmd=%v size=%dx%d state=%v", s, cmd, got.width, got.height, got.state)
		}
	}
}

func TestWindowSize_ZeroKeepsOutputUnchanged(t *testing.T) {
	for _, m := range []Model{manyRuns(50), manyCommits(60)} {
		v := m.View()
		if strings.Contains(v, "más") || sized(m, 0, 0).View() != v {
			t.Error("a zero size must not window the list")
		}
		if n := strings.Count(v, "\n"); n < 50 {
			t.Errorf("unsized list must render every row, got %d lines", n)
		}
	}
}

func TestHistoryWindow_FitsAndFollowsCursor(t *testing.T) {
	m := sized(manyRuns(50), 80, 24)
	m.runsCursor = 49
	v := m.View()
	if n := strings.Count(v, "\n"); n > 24 {
		t.Errorf("view has %d lines, want <= 24:\n%s", n, v)
	}
	first := strings.Index(v, "J49")
	if first < 0 || !strings.Contains(v, "> ") || !strings.Contains(v, "↑ ") {
		t.Fatalf("last row must be visible with an up indicator:\n%s", v)
	}
	var hidden int
	for i := 0; i < 50; i++ {
		if !strings.Contains(v, fmt.Sprintf("J%02d", i)) {
			hidden++
		}
	}
	if !strings.Contains(v, fmt.Sprintf("↑ %d más", hidden)) || downIndicator.MatchString(v) {
		t.Errorf("up indicator must equal %d hidden rows, no down indicator:\n%s", hidden, v)
	}
	m, _ = press(manyRuns(50), "down")
	m = sized(m, 80, 24)
	for i := 0; i < 30; i++ {
		prev := m
		m, _ = press(m, "down")
		if !strings.Contains(m.View(), fmt.Sprintf("J%02d", m.runsCursor)) {
			t.Fatalf("cursor row %d scrolled out of view:\n%s", m.runsCursor, m.View())
		}
		if i > 20 && strings.Count(m.View(), "J") != strings.Count(prev.View(), "J") {
			t.Error("window must keep its size while scrolling")
		}
	}
}

func TestHistoryWindow_ActionsApplyOutsideFirstWindowAndRowsTruncate(t *testing.T) {
	m := sized(manyRuns(50), 60, 24)
	m.runsCursor = 40
	if nm, cmd := press(m, "enter"); cmd == nil || nm.state == StateRunHistory {
		t.Error("Enter must resume the selected row outside the first window")
	}
	m.runs[40].Ticket = strings.Repeat("T", 90)
	v := m.View()
	if maxWidth(v, "InProgress") > 60 || !strings.Contains(v, "…") {
		t.Errorf("rows must be cut to 60 cells ending with an ellipsis (max %d):\n%s", maxWidth(v, "InProgress"), v)
	}
}

func TestSelectionWindow_FitsAndMarksPersist(t *testing.T) {
	m := sized(manyCommits(60), 100, 24)
	m.cursor = 59
	v := m.View()
	if n := strings.Count(v, "\n"); n > 24 || !strings.Contains(v, "c059") {
		t.Fatalf("view has %d lines or hides the cursor row:\n%s", n, v)
	}
	if !strings.Contains(v, "↑ ") || downIndicator.MatchString(v) {
		t.Errorf("want only an up indicator:\n%s", v)
	}
	up := m
	for i := 0; i < 5; i++ {
		up, _ = press(up, "up")
		if !strings.Contains(up.View(), fmt.Sprintf("c%03d", up.cursor)) {
			t.Fatalf("moving up lost the cursor row %d", up.cursor)
		}
	}
	if !downIndicator.MatchString(up.View()) {
		t.Errorf("moving up must show hidden rows below:\n%s", up.View())
	}
	// A mark scrolled out of the window still counts.
	m = sized(manyCommits(60), 100, 24)
	m, _ = press(m, " ")
	m.cursor = 59
	if strings.Contains(m.View(), "c000") || countSelected(m.items) != 1 || !m.items[0].Selected || !strings.Contains(m.View(), "Seleccionados: 1") {
		t.Errorf("a scrolled-out mark must persist:\n%s", m.View())
	}
}

func TestSelectionWindow_TruncatesWithStatusMark(t *testing.T) {
	m := sized(manyCommits(3), 60, 24)
	m.items[1].Subject = strings.Repeat("s", 120)
	m.items[1].Disabled, m.items[1].Reason = true, "merge commit"
	v := m.View()
	if maxWidth(v, "c00") > 60 || !strings.Contains(v, "…") {
		t.Errorf("selection rows must be cut to 60 cells (max %d):\n%s", maxWidth(v, "c00"), v)
	}
}
