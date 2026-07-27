package app

import (
	"testing"

	"deploydeck/internal/git"
	"deploydeck/internal/runs"
)

// TestOnResumeDetect_IgnoredOffTicketInput is HOLE A RED: startup
// resume-detection fires asynchronously from onPrereqDone, and RepoState shells
// out to git. If that launch is slow, the user can advance past the initial
// ticket-input screen before resumeDetectMsg lands. onResumeDetect must then be
// a NO-OP — it must never yank the user back to StateTicketInput / hijack them
// to StateRunHistory or discard an in-progress selection. It only acts while
// the user is still on the initial screen.
func TestOnResumeDetect_IgnoredOffTicketInput(t *testing.T) {
	t.Run("a resumable message while on commit selection is a no-op", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
		// The user already advanced to commit selection and picked commits.
		m.state = StateCommitSelection
		m.items = []git.CommitSelectionItem{
			{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "sha-A"}}, Selected: true},
			{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "sha-B"}}},
		}
		m.cursor = 1

		state := git.RepoState{InProgress: true, CurrentSHA: "sha-A", SequencerRemaining: 2}
		records := []runs.Record{
			{RunID: "live", Ticket: "PROJ-1", Commits: []string{"sha-A", "sha-B"}, PickTotal: 2, Phase: "git-conflict"},
		}

		next, cmd := m.Update(resumeDetectMsg{state: state, records: records})
		nm := next.(Model)

		if nm.State() != StateCommitSelection {
			t.Fatalf("a late resumeDetectMsg must not yank the user off StateCommitSelection, got %v", nm.State())
		}
		if len(nm.items) != 2 || !nm.items[0].Selected {
			t.Errorf("the in-progress selection must be preserved untouched, got %+v", nm.items)
		}
		if nm.cursor != 1 {
			t.Errorf("the selection cursor must be preserved, got %d", nm.cursor)
		}
		if len(nm.runs) != 0 {
			t.Errorf("detection must not populate the history off the initial screen, got %d runs", len(nm.runs))
		}
		if cmd != nil {
			t.Error("a no-op detection must fire no command")
		}
	})

	t.Run("a nothing-resumable message while on commit selection is a no-op", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
		m.state = StateCommitSelection
		m.items = []git.CommitSelectionItem{
			{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "sha-A"}}, Selected: true},
		}
		records := []runs.Record{
			{RunID: "done", Ticket: "PROJ-1", JobID: "JOB1", Status: "Succeeded", Phase: "done"},
		}

		next, cmd := m.Update(resumeDetectMsg{state: git.RepoState{Clean: true}, records: records})
		nm := next.(Model)

		// Without the guard, "nothing resumable" would reset state to
		// StateTicketInput — discarding the user's screen and selection.
		if nm.State() != StateCommitSelection {
			t.Fatalf("a late nothing-resumable message must not reset the user to StateTicketInput, got %v", nm.State())
		}
		if len(nm.items) != 1 || !nm.items[0].Selected {
			t.Errorf("the in-progress selection must be preserved, got %+v", nm.items)
		}
		if cmd != nil {
			t.Error("a no-op detection must fire no command")
		}
	})
}
