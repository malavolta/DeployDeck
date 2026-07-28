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
	"sort"
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

	// HU-013 additive fields — every one `omitempty`, zero-default safe. A
	// run.json written before these existed still Load()s cleanly with all
	// five zero-valued (run-persistence spec: "Additive Record Growth With
	// Backward Compatibility"). NO SchemaVersion bump accompanies this growth.
	//
	// Commits holds the selected commit SHAs (topological order), set once at
	// branch creation — it lets resume detection match the repo's live
	// CHERRY_PICK_HEAD against this run's original selection.
	Commits []string `json:"commits,omitempty"`
	// PickIndex/PickTotal are the cherry-pick sequencer's "pick N of M",
	// updated as the pick progresses (see internal/app's derivePickIndex).
	PickIndex int `json:"pickIndex,omitempty"`
	PickTotal int `json:"pickTotal,omitempty"`
	// CurrentCommit is the SHA currently being cherry-picked (mirrors
	// git.RepoState.CurrentSHA at the moment it was persisted).
	CurrentCommit string `json:"currentCommit,omitempty"`
	// Phase is the coarse flow stage: cherry-pick|git-conflict|validating|
	// done|aborted.
	Phase string `json:"phase,omitempty"`

	// PRUrl is HU-014's additive growth field: the created PR's URL, set
	// via MarkPRCreated after a successful `gh pr create`. `omitempty`,
	// like every other growth field this package has added — a run.json
	// written before PRUrl existed still Load()s cleanly with it
	// zero-valued. NO SchemaVersion bump accompanies this growth either.
	PRUrl string `json:"prUrl,omitempty"`

	// SourceRunID is HU-016's additive growth field: the prior run's ID
	// when this run is a re-promotion, set at NEW-run creation time (unlike
	// PRUrl, which is only known post-hoc). `omitempty`, like every other
	// growth field this package has added — a run.json written before
	// SourceRunID existed still Load()s cleanly with it zero-valued. NO
	// SchemaVersion bump accompanies this growth either.
	SourceRunID string `json:"sourceRunId,omitempty"`

	// TestLevel is HU-015's additive growth field: the deploy test level
	// used for this run's validation, set from the deployment plan's
	// TestLevel at run-creation time. `omitempty`, like every other growth
	// field this package has added — a run.json written before TestLevel
	// existed still Load()s cleanly with it zero-valued. NO SchemaVersion
	// bump accompanies this growth either. Feeds runs.QuickDeployEligible's
	// required-tests-ran predicate.
	TestLevel string `json:"testLevel,omitempty"`

	// QuickDeployedAt is HU-015's additive growth field: set by
	// MarkQuickDeployed once a quick deploy has been executed for this run,
	// used as the double-deploy guard in QuickDeployEligible (a zero value
	// means "never quick-deployed"). `omitempty`, like every other growth
	// field this package has added. NO SchemaVersion bump accompanies this
	// growth either.
	QuickDeployedAt time.Time `json:"quickDeployedAt,omitempty"`
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

// List scans .deploydeck/runs/*/run.json and returns every run, ordered
// newest-first by CreatedAt (run-persistence spec: "List Runs Newest First").
// A run directory whose run.json is missing or malformed is skipped rather
// than failing the whole listing — a single corrupted run must never make the
// rest of the history inaccessible. A repo with no runs dir yet (nothing ever
// created) returns an empty list, not an error.
func (w *Writer) List() ([]Record, error) {
	root := filepath.Join(w.baseDir, ".deploydeck", "runs")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("runs: reading runs dir %s: %w", root, err)
	}

	var records []Record
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rec, err := w.readRecord(filepath.Join(root, entry.Name()))
		if err != nil {
			continue
		}
		records = append(records, rec)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records, nil
}

// Load returns the persisted Record for runID (run-persistence spec: "Load A
// Single Run By ID"). An unknown runID (no run.json, e.g. never created) is
// an explicit error, never a silent zero Record.
func (w *Writer) Load(runID string) (Record, error) {
	return w.readRecord(w.runDir(runID))
}

// Save upserts rec's run.json (creating the run dir if needed), forcing
// SchemaVersion to SchemaVersion1 — this package is always the schema
// authority, same invariant as Create. Save is the general-purpose upsert
// HU-013's progress writes use (run creation at branch time, cherry-pick
// pick-index progress, phase transitions merged in place); Create remains the
// original HU-010 jobId-time constructor that also writes validate.json.
func (w *Writer) Save(rec Record) error {
	dir := w.runDir(rec.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("runs: creating run dir %s: %w", dir, err)
	}
	rec.SchemaVersion = SchemaVersion1
	return writeJSON(filepath.Join(dir, "run.json"), rec)
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

// MarkCanceled records a successful cancellation (HU-012): it writes the raw
// cancel response verbatim as a cancel.json companion — mirroring exactly how
// Create persists validate.json — and updates run.json's Status to "Canceled"
// with a fresh time.Now() UpdatedAt. Unlike AppendReport it deliberately does
// NOT use the report-<NNN>.json poll-numbering scheme: a cancel is a distinct
// one-off event, not a poll, so persisting it as a report would misrepresent it
// and perturb the poll sequence (run-persistence spec: "MarkCanceled SHALL NOT
// write the cancel result using the report-<NNN>.json poll-numbering scheme").
// An unknown runID (no run.json, e.g. a run never created via Create) is an
// explicit error, never a silent no-op.
func (w *Writer) MarkCanceled(runID string, cancelRaw []byte) error {
	dir := w.runDir(runID)

	rec, err := w.readRecord(dir)
	if err != nil {
		return err
	}

	rec.Status = "Canceled"
	rec.UpdatedAt = time.Now()
	if err := writeJSON(filepath.Join(dir, "run.json"), rec); err != nil {
		return err
	}

	cancelPath := filepath.Join(dir, "cancel.json")
	if err := os.WriteFile(cancelPath, cancelRaw, 0o644); err != nil {
		return fmt.Errorf("runs: writing %s: %w", cancelPath, err)
	}

	return nil
}

// MarkPRCreated records a successfully created PR's URL (HU-014): it loads
// the run's record, sets PRUrl, and persists it with a fresh time.Now()
// UpdatedAt — mirroring MarkCanceled's method shape (load -> mutate ->
// save). Unlike MarkCanceled it writes no raw-response companion file:
// gh pr create's raw output is shown to the user by the caller, never
// persisted here. An unknown runID (no run.json, e.g. a run never created
// via Create) is an explicit error, never a silent no-op.
func (w *Writer) MarkPRCreated(runID, prURL string) error {
	dir := w.runDir(runID)

	rec, err := w.readRecord(dir)
	if err != nil {
		return err
	}

	rec.PRUrl = prURL
	rec.UpdatedAt = time.Now()
	return writeJSON(filepath.Join(dir, "run.json"), rec)
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
