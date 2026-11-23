package runs_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/runs"
)

func seedReports(t *testing.T, files map[string]string) (*runs.Writer, string) {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, ".deploydeck", "runs", "RUN-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return runs.NewWriter(base), dir
}

func acceptGood(raw []byte) bool { return string(raw) != "corrupt" }

func TestLatestReport_NewestWins(t *testing.T) {
	w, dir := seedReports(t, map[string]string{"report-001.json": "one", "report-003.json": "three", "run.json": "{}"})
	got, rejected, err := w.LatestReport("RUN-1", acceptGood)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != filepath.Join(dir, "report-003.json") || string(got.Raw) != "three" || len(rejected) != 0 {
		t.Errorf("got %+v rejected %v", got, rejected)
	}
}

func TestLatestReport_CorruptNewestFallsBack(t *testing.T) {
	w, dir := seedReports(t, map[string]string{"report-001.json": "one", "report-002.json": "corrupt"})
	got, rejected, err := w.LatestReport("RUN-1", acceptGood)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != filepath.Join(dir, "report-001.json") {
		t.Errorf("want fallback to 001, got %s", got.Path)
	}
	if len(rejected) != 1 || rejected[0] != filepath.Join(dir, "report-002.json") {
		t.Errorf("rejected = %v", rejected)
	}
}

func TestLatestReport_AllRejected(t *testing.T) {
	w, dir := seedReports(t, map[string]string{"report-001.json": "corrupt", "report-002.json": "corrupt"})
	_, rejected, err := w.LatestReport("RUN-1", acceptGood)
	if !errors.Is(err, runs.ErrNoReport) {
		t.Fatalf("err = %v, want ErrNoReport", err)
	}
	if len(rejected) != 2 || rejected[0] != filepath.Join(dir, "report-002.json") {
		t.Errorf("rejected newest-first expected, got %v", rejected)
	}
}

func TestLatestReport_NoFilesAndNonReportsIgnored(t *testing.T) {
	w, _ := seedReports(t, map[string]string{"run.json": "{}", "report-1.json": "x", "cancel.json": "{}", "report-abc.json": "x"})
	if _, _, err := w.LatestReport("RUN-1", acceptGood); !errors.Is(err, runs.ErrNoReport) {
		t.Errorf("err = %v, want ErrNoReport", err)
	}
	if _, _, err := w.LatestReport("MISSING", acceptGood); !errors.Is(err, runs.ErrNoReport) {
		t.Errorf("missing run dir: err = %v, want ErrNoReport", err)
	}
	if _, _, err := w.LatestReport("../RUN-1", acceptGood); !errors.Is(err, runs.ErrNoReport) {
		t.Errorf("path-like run id must not escape the runs dir: err = %v", err)
	}
}
