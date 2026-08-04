package runs_test

import (
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// TestFindRunForBranch is task 1.9 (RED): FindRunForBranch re-renders each
// record's branch name (via the SAME git.RenderBranchName every other
// correlation in this codebase uses) to locate the prior run targeting
// branchName (incremental-promotion spec: "Reuse Targets The Prior Run For
// The Same Branch").
func TestFindRunForBranch(t *testing.T) {
	format := config.DefaultBranchFormat

	t.Run("match", func(t *testing.T) {
		records := []runs.Record{
			{RunID: "r1", Ticket: "PROJ-1", Target: "UAT", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
			{RunID: "r2", Ticket: "PROJ-2", Target: "UAT", CreatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
		}
		got, ok := runs.FindRunForBranch(records, format, "deploy/PROJ-1-to-UAT")
		if !ok {
			t.Fatal("expected a match")
		}
		if got.RunID != "r1" {
			t.Fatalf("FindRunForBranch() RunID = %q, want %q", got.RunID, "r1")
		}
	})

	t.Run("no match", func(t *testing.T) {
		records := []runs.Record{
			{RunID: "r1", Ticket: "PROJ-2", Target: "UAT", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		}
		_, ok := runs.FindRunForBranch(records, format, "deploy/PROJ-1-to-UAT")
		if ok {
			t.Fatal("expected no match")
		}
	})

	t.Run("newest of many wins, records not pre-sorted", func(t *testing.T) {
		records := []runs.Record{
			{RunID: "oldest", Ticket: "PROJ-1", Target: "UAT", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
			{RunID: "newest", Ticket: "PROJ-1", Target: "UAT", CreatedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
			{RunID: "middle", Ticket: "PROJ-1", Target: "UAT", CreatedAt: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
		}
		got, ok := runs.FindRunForBranch(records, format, "deploy/PROJ-1-to-UAT")
		if !ok {
			t.Fatal("expected a match")
		}
		if got.RunID != "newest" {
			t.Fatalf("FindRunForBranch() RunID = %q, want the newest record %q", got.RunID, "newest")
		}
	})
}
