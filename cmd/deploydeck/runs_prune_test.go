package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/runs"
)

// seedPruneRun writes a minimal run.json under dir/.deploydeck/runs/<id>/ with
// the given CreatedAt, so the retention window can be exercised end-to-end.
func seedPruneRun(t *testing.T, dir, id string, createdAt time.Time) {
	t.Helper()
	if err := runs.NewWriter(dir).Save(runs.Record{
		RunID:     id,
		Ticket:    id,
		Target:    "UAT",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}); err != nil {
		t.Fatalf("seeding run %s: %v", id, err)
	}
}

// writePruneConfig writes a deploydeck.yaml carrying the retention bounds.
func writePruneConfig(t *testing.T, dir string, keepLast, keepDays int) {
	t.Helper()
	body := "runs:\n" +
		"  keepLast: " + strconv.Itoa(keepLast) + "\n" +
		"  keepDays: " + strconv.Itoa(keepDays) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "deploydeck.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

// runDirExists reports whether the on-disk run directory is still present.
func runDirExists(dir, id string) bool {
	_, err := os.Stat(filepath.Join(dir, ".deploydeck", "runs", id))
	return err == nil
}

// --- 6.1/6.2: prune removes only outside-window runs ------------------------

// TestRunsPrune_RemovesOnlyOutsideWindow is task 6.1 (RED): with a fixture of
// more than keepLast runs, some older than keepDays, prune removes ONLY the
// runs outside BOTH retention conditions and leaves the rest untouched.
func TestRunsPrune_RemovesOnlyOutsideWindow(t *testing.T) {
	dir := t.TempDir()
	writePruneConfig(t, dir, 2, 30)

	now := time.Now()
	day := 24 * time.Hour
	seedPruneRun(t, dir, "recent-0", now.Add(-1*day))    // rank 0: within keepLast
	seedPruneRun(t, dir, "recent-1", now.Add(-10*day))   // rank 1: within keepLast
	seedPruneRun(t, dir, "within-age", now.Add(-20*day)) // rank 2: outside keepLast but age<=keepDays
	seedPruneRun(t, dir, "stale", now.Add(-100*day))     // rank 3: outside BOTH -> pruned

	var buf bytes.Buffer
	if err := runPrune(&buf, dir); err != nil {
		t.Fatalf("runPrune: %v", err)
	}

	if runDirExists(dir, "stale") {
		t.Error("the outside-both run should have been pruned")
	}
	for _, kept := range []string{"recent-0", "recent-1", "within-age"} {
		if !runDirExists(dir, kept) {
			t.Errorf("run %q was inside the retention window and must be kept", kept)
		}
	}
	if !strings.Contains(buf.String(), "stale") {
		t.Errorf("prune output should report what was removed:\n%s", buf.String())
	}
}

// --- 6.3: end-to-end through the registered cobra command -------------------

// TestRunsPruneCmd_EndToEnd is task 6.3 (RED): invoking `deploydeck runs prune`
// end-to-end (real cwd + config + runs dir) removes exactly the outside-window
// dirs and keeps the rest.
func TestRunsPruneCmd_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	writePruneConfig(t, dir, 1, 30)

	now := time.Now()
	day := 24 * time.Hour
	seedPruneRun(t, dir, "keep", now)               // rank 0: within keepLast
	seedPruneRun(t, dir, "drop", now.Add(-100*day)) // outside both -> pruned
	seedPruneRun(t, dir, "recent", now.Add(-2*day)) // rank 1 (outside keepLast=1) but age<=30 -> kept

	t.Chdir(dir) // RunE resolves the runs dir via os.Getwd

	var buf bytes.Buffer
	cmd := newRootCmd(Deps{})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"runs", "prune"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("runs prune should succeed, got: %v", err)
	}

	if runDirExists(dir, "drop") {
		t.Error("the outside-window run should be pruned end-to-end")
	}
	if !runDirExists(dir, "keep") || !runDirExists(dir, "recent") {
		t.Error("in-window runs must be kept end-to-end")
	}
}

// TestRunsCmd_Registered proves the runs command (with its prune subcommand) is
// registered on the root.
func TestRunsCmd_Registered(t *testing.T) {
	root := newRootCmd(Deps{})
	pruneCmd, _, err := root.Find([]string{"runs", "prune"})
	if err != nil {
		t.Fatalf("expected root to register `runs prune`, got: %v", err)
	}
	if pruneCmd.Name() != "prune" {
		t.Fatalf("expected the prune subcommand, got %q", pruneCmd.Name())
	}
}

// --- 6.4: non-zero exit on failure -----------------------------------------

// TestRunsPrune_ConfigError_NonZeroExit is task 6.4 (RED): a missing config
// surfaces an error (non-zero exit), mirroring newDoctorCmd's failure path
// rather than silently succeeding.
func TestRunsPrune_ConfigError_NonZeroExit(t *testing.T) {
	dir := t.TempDir() // no deploydeck.yaml

	var buf bytes.Buffer
	if err := runPrune(&buf, dir); err == nil {
		t.Fatal("runPrune with no config should return an error (non-zero exit)")
	}

	t.Chdir(dir)
	cmd := newRootCmd(Deps{})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"runs", "prune"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("`deploydeck runs prune` with no config should exit non-zero")
	}
}
