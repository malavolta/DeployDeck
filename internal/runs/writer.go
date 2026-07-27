// Package runs is a MINIMAL local run-record writer, scoped to exactly what
// HU-010 (persist on jobId receipt) and HU-011 (persist every polled raw
// report) require. Full run history browsing/listing, retention/cleanup,
// and resume-by-jobId are explicitly OUT of scope here and belong to
// HU-013, which extends this writer by scanning the same run.json files
// rather than rewriting them (see design.md's "run persistence" decision).
package runs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// SchemaVersion1 is the current Record schema version. Create always writes
// this value, regardless of what the caller's Record.SchemaVersion held —
// this package is the single authority on its own on-disk schema, so a
// stale or mistaken caller value can never leak onto disk.
const SchemaVersion1 = 1

// Record is the minimal per-run record persisted to run.json. HU-013 grows
// this struct additively (new fields), never replaces it, keeping every
// run.json ever written forward-readable.
type Record struct {
	SchemaVersion int       `json:"schemaVersion"`
	RunID         string    `json:"runId"`
	Ticket        string    `json:"ticket"`
	Target        string    `json:"target"`
	Alias         string    `json:"alias"`
	JobID         string    `json:"jobId"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Writer persists run records under baseDir/.deploydeck/runs/. baseDir is
// always an explicit, already-resolved repository root passed by the
// caller (internal/app) — never assumed to be the process cwd.
type Writer struct {
	baseDir string
}

// NewWriter returns a Writer rooted at baseDir.
func NewWriter(baseDir string) *Writer {
	return &Writer{baseDir: baseDir}
}

// runDir returns the directory a given run's files live under.
func (w *Writer) runDir(runID string) string {
	return filepath.Join(w.baseDir, ".deploydeck", "runs", runID)
}

// Create writes .deploydeck/runs/<rec.RunID>/{run.json,validate.json},
// forcing SchemaVersion to SchemaVersion1 (HU-010 AC: "se persiste en
// .deploydeck/runs/" as soon as a jobId is obtained). validateRaw is the
// raw `sf project deploy validate --async --json` response, saved verbatim
// so its full content survives even though only JobID (already on rec) is
// parsed. Returns the created run directory.
func (w *Writer) Create(rec Record, validateRaw []byte) (string, error) {
	dir := w.runDir(rec.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("runs: creating run dir %s: %w", dir, err)
	}

	rec.SchemaVersion = SchemaVersion1
	if err := writeJSON(filepath.Join(dir, "run.json"), rec); err != nil {
		return "", err
	}

	validatePath := filepath.Join(dir, "validate.json")
	if err := os.WriteFile(validatePath, validateRaw, 0o644); err != nil {
		return "", fmt.Errorf("runs: writing %s: %w", validatePath, err)
	}

	return dir, nil
}

// reportFilePattern matches this package's own report-<NNN>.json naming
// (001, 002, ...), used by AppendReport to find the next unused number.
var reportFilePattern = regexp.MustCompile(`^report-(\d{3})\.json$`)

// AppendReport updates the run's Status/UpdatedAt on run.json and persists
// reportRaw as the run's NEXT report-<NNN>.json (001, 002, ... — never
// reused, never overwritten), so every raw `deploy report` poll relevant to
// the run survives on disk (HU-011 AC: "Guardar cada respuesta raw
// relevante"). UpdatedAt is stamped with time.Now() — AppendReport's
// signature carries no injected clock, unlike internal/app's polling
// loop, so the run.json timestamp always reflects the real time the poll
// was persisted.
func (w *Writer) AppendReport(runID, status string, reportRaw []byte) error {
	dir := w.runDir(runID)

	rec, err := w.readRecord(dir)
	if err != nil {
		return err
	}

	rec.Status = status
	rec.UpdatedAt = time.Now()
	if err := writeJSON(filepath.Join(dir, "run.json"), rec); err != nil {
		return err
	}

	n, err := nextReportNumber(dir)
	if err != nil {
		return err
	}
	reportPath := filepath.Join(dir, fmt.Sprintf("report-%03d.json", n))
	if err := os.WriteFile(reportPath, reportRaw, 0o644); err != nil {
		return fmt.Errorf("runs: writing %s: %w", reportPath, err)
	}

	return nil
}

// readRecord loads run.json from a run's directory. A missing/unreadable
// run.json (e.g. an unknown runID never created via Create) is an explicit
// error, never a silent zero Record.
func (w *Writer) readRecord(dir string) (Record, error) {
	data, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		return Record{}, fmt.Errorf("runs: reading run.json in %s: %w", dir, err)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, fmt.Errorf("runs: parsing run.json in %s: %w", dir, err)
	}
	return rec, nil
}

// nextReportNumber scans dir for existing report-<NNN>.json files and
// returns the next unused number (1 when none exist yet), so AppendReport
// never depends on in-memory Writer state across calls — a fresh Writer
// pointed at the same dir still numbers correctly.
func nextReportNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("runs: reading run dir %s: %w", dir, err)
	}

	max := 0
	for _, entry := range entries {
		m := reportFilePattern.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("runs: marshaling %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("runs: writing %s: %w", path, err)
	}
	return nil
}
