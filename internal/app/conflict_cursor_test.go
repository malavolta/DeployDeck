package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

func conflictFiles(paths ...string) []git.ConflictFile {
	var fs []git.ConflictFile
	for _, p := range paths {
		fs = append(fs, git.ConflictFile{Path: p, Kind: git.ConflictText})
	}
	return fs
}

// conflictModel enters the conflict screen through the pick entry site with a
// stale commit-selection cursor, as the real flow would have.
func pickConflictModel(stale int, paths ...string) Model {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.cursor = stale
	next, _ := m.onPickDone(pickDoneMsg{outcome: git.PickOutcome{
		State: git.RepoState{InProgress: true, Unmerged: conflictFiles(paths...)},
	}})
	return next.(Model)
}

// markedRows returns the file paths on lines rendered with the `>` marker.
func markedRows(view string) []string {
	var out []string
	for _, l := range strings.Split(view, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), ">") && strings.Contains(l, ".cls") {
			out = append(out, l)
		}
	}
	return out
}

func TestConflictCursor_StartsOnFirstFileDespiteStaleCursor(t *testing.T) {
	m := pickConflictModel(3, "a.cls", "b.cls", "c.cls")
	rows := markedRows(m.View())
	if len(rows) != 1 || !strings.Contains(rows[0], "a.cls") {
		t.Fatalf("want one marker on a.cls, got %v", rows)
	}
}

func TestConflictCursor_ResetsOnResumeAndSecondEntry(t *testing.T) {
	m := pickConflictModel(0, "a.cls", "b.cls")
	m.conflictIdx = 1
	next, _ := m.onPickDone(pickDoneMsg{outcome: git.PickOutcome{
		State: git.RepoState{InProgress: true, Unmerged: conflictFiles("a.cls", "b.cls")},
	}})
	if rows := markedRows(next.(Model).View()); len(rows) != 1 || !strings.Contains(rows[0], "a.cls") {
		t.Fatalf("second entry must reset the cursor, got %v", rows)
	}

	r := New(Deps{Dir: "/repo", Config: testConfig()})
	r.conflictIdx = 1
	r.repoState = git.RepoState{InProgress: true, CurrentSHA: "abc", Unmerged: conflictFiles("a.cls", "b.cls")}
	res, _ := r.resumeInto(resumableConflictRecord("abc"))
	if rows := markedRows(res.(Model).View()); len(rows) != 1 || !strings.Contains(rows[0], "a.cls") {
		t.Fatalf("resume entry must reset the cursor, got %v", rows)
	}
}

func TestConflictCursor_NavigationClamps(t *testing.T) {
	m := pickConflictModel(0, "a.cls", "b.cls")
	steps := []struct{ key, want string }{
		{"down", "b.cls"}, {"j", "b.cls"}, {"up", "a.cls"}, {"k", "a.cls"}, {"j", "b.cls"}, {"up", "a.cls"},
	}
	for _, s := range steps {
		next, _ := m.Update(keyPress(s.key))
		m = next.(Model)
		rows := markedRows(m.View())
		if len(rows) != 1 || !strings.Contains(rows[0], s.want) {
			t.Fatalf("after %q want marker on %s, got %v", s.key, s.want, rows)
		}
	}
}

func TestConflictCursor_ClampsWhenListShrinks(t *testing.T) {
	m := pickConflictModel(0, "a.cls", "b.cls", "c.cls")
	m.conflictIdx = 2
	next, _ := m.onRepoState(repoStateMsg{state: git.RepoState{InProgress: true, Unmerged: conflictFiles("a.cls")}})
	rows := markedRows(next.(Model).View())
	if len(rows) != 1 || !strings.Contains(rows[0], "a.cls") {
		t.Fatalf("shrunk list must mark file 1, got %v", rows)
	}
}

func TestConflictCursor_EditTargetsHighlightedFile(t *testing.T) {
	var got string
	m := pickConflictModel(2, "a.cls", "b.cls", "c.cls")
	m.deps.Edit = func(p string) tea.Cmd { got = p; return nil }
	next, _ := m.Update(keyPress("down"))
	next.(Model).Update(keyPress("e"))
	if got != "b.cls" {
		t.Fatalf("e opened %q, want b.cls", got)
	}
	got = ""
	m.Update(keyPress("e")) // stale m.cursor=2 must not leak in
	if got != "a.cls" {
		t.Fatalf("e opened %q, want a.cls", got)
	}
}

func TestConflictCursor_FooterNamesNavigationAndEdit(t *testing.T) {
	v := pickConflictModel(0, "a.cls", "b.cls").View()
	if !strings.Contains(v, "↑/↓") || !strings.Contains(v, "e editor") {
		t.Fatalf("conflict footer must list ↑/↓ and e editor:\n%s", v)
	}
}

func resumableConflictRecord(sha string) runs.Record {
	return runs.Record{RunID: "r1", Ticket: "T-1", Commits: []string{sha}, PickTotal: 1, Phase: "git-conflict"}
}
