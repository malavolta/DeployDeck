package runs_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"deploydeck/internal/runs"
)

func TestWriter_Create_WritesRunJSONAndValidateJSON(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	rec := runs.Record{
		RunID:     "TICKET-123-to-UAT-20260727120000",
		Ticket:    "TICKET-123",
		Target:    "UAT",
		Alias:     "UAT_SANDBOX",
		JobID:     "0Af000000000001EAA",
		Status:    "Queued",
		CreatedAt: now,
		UpdatedAt: now,
	}
	validateRaw := []byte(`{"status":0,"result":{"id":"0Af000000000001EAA"}}`)

	dir, err := w.Create(rec, validateRaw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDir := filepath.Join(base, ".deploydeck", "runs", rec.RunID)
	if dir != wantDir {
		t.Fatalf("expected run dir %q, got %q", wantDir, dir)
	}

	runJSONPath := filepath.Join(dir, "run.json")
	data, err := os.ReadFile(runJSONPath)
	if err != nil {
		t.Fatalf("expected run.json to exist: %v", err)
	}
	var got runs.Record
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("run.json is not valid JSON: %v", err)
	}
	if got.SchemaVersion != runs.SchemaVersion1 {
		t.Fatalf("expected SchemaVersion %d, got %d", runs.SchemaVersion1, got.SchemaVersion)
	}
	if got.RunID != rec.RunID || got.Ticket != rec.Ticket || got.Target != rec.Target || got.Alias != rec.Alias {
		t.Fatalf("expected identifying fields to round-trip, got %+v", got)
	}
	if got.JobID != rec.JobID {
		t.Fatalf("expected JobID %q, got %q", rec.JobID, got.JobID)
	}
	if got.Status != rec.Status {
		t.Fatalf("expected Status %q, got %q", rec.Status, got.Status)
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now) {
		t.Fatalf("expected CreatedAt/UpdatedAt to round-trip as %v, got CreatedAt=%v UpdatedAt=%v", now, got.CreatedAt, got.UpdatedAt)
	}

	validateJSONPath := filepath.Join(dir, "validate.json")
	gotValidate, err := os.ReadFile(validateJSONPath)
	if err != nil {
		t.Fatalf("expected validate.json to exist: %v", err)
	}
	if string(gotValidate) != string(validateRaw) {
		t.Fatalf("expected validate.json to hold the raw validate response verbatim, got %q", gotValidate)
	}
}

func TestWriter_Create_AlwaysWritesCurrentSchemaVersion(t *testing.T) {
	// Triangulation: an explicitly wrong caller-supplied SchemaVersion must
	// still be overwritten with SchemaVersion1, proving Create is the
	// authority on the schema version rather than trusting the caller.
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{
		SchemaVersion: 99,
		RunID:         "TICKET-999-to-INT-20260101000000",
		Ticket:        "TICKET-999",
		Target:        "INT",
		Alias:         "INT_SANDBOX",
		JobID:         "0Af000000000002EAA",
		Status:        "Queued",
	}

	dir, err := w.Create(rec, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatalf("expected run.json to exist: %v", err)
	}
	var got runs.Record
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("run.json is not valid JSON: %v", err)
	}
	if got.SchemaVersion != runs.SchemaVersion1 {
		t.Fatalf("expected SchemaVersion forced to %d, got %d", runs.SchemaVersion1, got.SchemaVersion)
	}
}

func TestWriter_Create_CreatesRunDirectoryTree(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	dir, err := w.Create(runs.Record{RunID: "T-to-INT-20260101000000"}, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected %q to be a created directory, err=%v info=%v", dir, err, info)
	}
}

func TestWriter_AppendReport_PersistsEachPollAsNumberedReport(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{
		RunID:     "TICKET-1-to-UAT-20260727120000",
		Ticket:    "TICKET-1",
		Target:    "UAT",
		Alias:     "UAT_SANDBOX",
		JobID:     "0Af000000000003EAA",
		Status:    "Queued",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	dir, err := w.Create(rec, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error creating run: %v", err)
	}

	report1 := []byte(`{"status":"InProgress","numberComponentsDeployed":1}`)
	if err := w.AppendReport(rec.RunID, "InProgress", report1); err != nil {
		t.Fatalf("unexpected error on first AppendReport: %v", err)
	}
	report2 := []byte(`{"status":"Succeeded","numberComponentsDeployed":10}`)
	if err := w.AppendReport(rec.RunID, "Succeeded", report2); err != nil {
		t.Fatalf("unexpected error on second AppendReport: %v", err)
	}

	got1, err := os.ReadFile(filepath.Join(dir, "report-001.json"))
	if err != nil {
		t.Fatalf("expected report-001.json to exist: %v", err)
	}
	if string(got1) != string(report1) {
		t.Fatalf("expected report-001.json to hold the first raw report, got %q", got1)
	}

	got2, err := os.ReadFile(filepath.Join(dir, "report-002.json"))
	if err != nil {
		t.Fatalf("expected report-002.json to exist: %v", err)
	}
	if string(got2) != string(report2) {
		t.Fatalf("expected report-002.json to hold the second raw report, got %q", got2)
	}

	// report-001.json must never be overwritten by the second poll.
	got1Again, err := os.ReadFile(filepath.Join(dir, "report-001.json"))
	if err != nil {
		t.Fatalf("expected report-001.json to still exist after a second poll: %v", err)
	}
	if string(got1Again) != string(report1) {
		t.Fatalf("report-001.json was overwritten: got %q, want %q", got1Again, report1)
	}

	runData, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatalf("expected run.json to exist: %v", err)
	}
	var gotRec runs.Record
	if err := json.Unmarshal(runData, &gotRec); err != nil {
		t.Fatalf("run.json is not valid JSON: %v", err)
	}
	if gotRec.Status != "Succeeded" {
		t.Fatalf("expected run.json Status updated to %q, got %q", "Succeeded", gotRec.Status)
	}
	if !gotRec.UpdatedAt.After(rec.UpdatedAt) {
		t.Fatalf("expected UpdatedAt to advance past the original CreatedAt, got %v (original %v)", gotRec.UpdatedAt, rec.UpdatedAt)
	}
	// Identifying fields must be preserved across AppendReport calls.
	if gotRec.RunID != rec.RunID || gotRec.Ticket != rec.Ticket || gotRec.JobID != rec.JobID {
		t.Fatalf("expected identifying fields preserved, got %+v", gotRec)
	}
}

func TestWriter_AppendReport_UnknownRunIDErrors(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	err := w.AppendReport("does-not-exist", "InProgress", []byte(`{}`))
	if err == nil {
		t.Fatal("expected an error for an unknown runID, got nil")
	}
}

func TestWriter_MarkCanceled_WritesCancelJSONAndSetsCanceledStatus(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{
		RunID:     "TICKET-7-to-UAT-20260727120000",
		Ticket:    "TICKET-7",
		Target:    "UAT",
		Alias:     "UAT_SANDBOX",
		JobID:     "0Af000000000007EAA",
		Status:    "InProgress",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	dir, err := w.Create(rec, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error creating run: %v", err)
	}

	cancelRaw := []byte(`{"status":0,"result":{"id":"0Af000000000007EAA","status":"Canceled"}}`)
	if err := w.MarkCanceled(rec.RunID, cancelRaw); err != nil {
		t.Fatalf("unexpected error on MarkCanceled: %v", err)
	}

	// cancel.json companion holds the raw cancel response verbatim.
	gotCancel, err := os.ReadFile(filepath.Join(dir, "cancel.json"))
	if err != nil {
		t.Fatalf("expected cancel.json to exist: %v", err)
	}
	if string(gotCancel) != string(cancelRaw) {
		t.Fatalf("expected cancel.json to hold the raw cancel response verbatim, got %q", gotCancel)
	}

	// run.json Status transitions to Canceled, UpdatedAt advances, identity
	// fields preserved.
	runData, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatalf("expected run.json to exist: %v", err)
	}
	var gotRec runs.Record
	if err := json.Unmarshal(runData, &gotRec); err != nil {
		t.Fatalf("run.json is not valid JSON: %v", err)
	}
	if gotRec.Status != "Canceled" {
		t.Fatalf("expected run.json Status updated to %q, got %q", "Canceled", gotRec.Status)
	}
	if !gotRec.UpdatedAt.After(rec.UpdatedAt) {
		t.Fatalf("expected UpdatedAt to advance, got %v (original %v)", gotRec.UpdatedAt, rec.UpdatedAt)
	}
	if gotRec.RunID != rec.RunID || gotRec.JobID != rec.JobID || gotRec.Ticket != rec.Ticket {
		t.Fatalf("expected identifying fields preserved, got %+v", gotRec)
	}
}

// TestWriter_MarkCanceled_DoesNotConsumeReportNumbering proves a cancel is
// persisted as cancel.json, NOT as a new report-<NNN>.json — it must not
// misrepresent a cancel as a poll nor perturb the poll numbering (HU-012
// run-persistence spec).
func TestWriter_MarkCanceled_DoesNotConsumeReportNumbering(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{RunID: "TICKET-8-to-UAT-20260727120000", Status: "InProgress"}
	dir, err := w.Create(rec, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error creating run: %v", err)
	}

	// Two prior polls exist as report-001/002.json.
	if err := w.AppendReport(rec.RunID, "InProgress", []byte(`{"n":1}`)); err != nil {
		t.Fatalf("seeding report 1: %v", err)
	}
	if err := w.AppendReport(rec.RunID, "InProgress", []byte(`{"n":2}`)); err != nil {
		t.Fatalf("seeding report 2: %v", err)
	}

	if err := w.MarkCanceled(rec.RunID, []byte(`{"canceled":true}`)); err != nil {
		t.Fatalf("unexpected error on MarkCanceled: %v", err)
	}

	// The cancel must NOT have been written as report-003.json.
	if _, err := os.Stat(filepath.Join(dir, "report-003.json")); err == nil {
		t.Fatal("MarkCanceled must NOT consume report numbering (report-003.json should not exist)")
	}
	if _, err := os.Stat(filepath.Join(dir, "cancel.json")); err != nil {
		t.Fatalf("the cancel must be written as cancel.json: %v", err)
	}

	// A subsequent poll still numbers as report-003.json (cancel did not
	// perturb the sequence).
	if err := w.AppendReport(rec.RunID, "InProgress", []byte(`{"n":3}`)); err != nil {
		t.Fatalf("appending report 3: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "report-003.json")); err != nil {
		t.Fatalf("the next poll should still number as report-003.json: %v", err)
	}
}

func TestWriter_MarkCanceled_UnknownRunIDErrors(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	err := w.MarkCanceled("does-not-exist", []byte(`{}`))
	if err == nil {
		t.Fatal("expected an error for an unknown runID, got nil")
	}
}

// --- HU-014: additive PRUrl + MarkPRCreated ---------------------------------

// TestRecord_PRUrl_RoundTripsThroughWriteReload is task 3.1 (RED): a record
// with PRUrl set round-trips unchanged through Save then Load
// (run-persistence spec: "PRUrl round-trips through a full write/reload").
func TestRecord_PRUrl_RoundTripsThroughWriteReload(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{RunID: "run-prurl", PRUrl: "https://github.com/org/repo/pull/1"}
	if err := w.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := w.Load(rec.RunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.PRUrl != rec.PRUrl {
		t.Fatalf("expected PRUrl to round-trip as %q, got %q", rec.PRUrl, got.PRUrl)
	}
}

// TestRecord_BackwardCompat_PriorRunJSONLoadsWithZeroPRUrl is task 3.1
// (RED): a run.json written before PRUrl existed still Loads cleanly with
// PRUrl zero-valued, and with NO SchemaVersion bump (run-persistence spec:
// "Prior-slice run.json still loads without PRUrl").
func TestRecord_BackwardCompat_PriorRunJSONLoadsWithZeroPRUrl(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	runID := "TICKET-9-to-UAT-20260101000000"
	dir := filepath.Join(base, ".deploydeck", "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seeding run dir: %v", err)
	}
	oldShape := `{"schemaVersion":1,"runId":"TICKET-9-to-UAT-20260101000000","ticket":"TICKET-9","target":"UAT","status":"Succeeded"}`
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(oldShape), 0o644); err != nil {
		t.Fatalf("seeding old-shape run.json: %v", err)
	}

	rec, err := w.Load(runID)
	if err != nil {
		t.Fatalf("an old-shape run.json should still Load cleanly: %v", err)
	}
	if rec.SchemaVersion != runs.SchemaVersion1 {
		t.Fatalf("expected SchemaVersion unchanged at %d, got %d", runs.SchemaVersion1, rec.SchemaVersion)
	}
	if rec.PRUrl != "" {
		t.Fatalf("expected PRUrl zero-valued on an old-shape record, got %q", rec.PRUrl)
	}
}

// TestRecord_SourceRunID_RoundTripsThroughWriteReload is task 1.1 (RED): a
// record with SourceRunID set round-trips unchanged through Save then Load
// (run-persistence spec: "SourceRunID round-trips through a full
// write/reload").
func TestRecord_SourceRunID_RoundTripsThroughWriteReload(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{RunID: "run-x", SourceRunID: "prior-run-id"}
	if err := w.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := w.Load(rec.RunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SourceRunID != rec.SourceRunID {
		t.Fatalf("expected SourceRunID to round-trip as %q, got %q", rec.SourceRunID, got.SourceRunID)
	}
}

// TestRecord_BackwardCompat_PriorRunJSONLoadsWithZeroSourceRunID is task 1.2
// (RED): a run.json written before SourceRunID existed still Loads cleanly
// with SourceRunID zero-valued, and with NO SchemaVersion bump
// (run-persistence spec: "Prior-slice run.json still loads without
// SourceRunID").
func TestRecord_BackwardCompat_PriorRunJSONLoadsWithZeroSourceRunID(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	runID := "TICKET-11-to-UAT-20260101000000"
	dir := filepath.Join(base, ".deploydeck", "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seeding run dir: %v", err)
	}
	oldShape := `{"schemaVersion":1,"runId":"TICKET-11-to-UAT-20260101000000","ticket":"TICKET-11","target":"UAT","status":"Succeeded"}`
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(oldShape), 0o644); err != nil {
		t.Fatalf("seeding old-shape run.json: %v", err)
	}

	rec, err := w.Load(runID)
	if err != nil {
		t.Fatalf("an old-shape run.json should still Load cleanly: %v", err)
	}
	if rec.SchemaVersion != runs.SchemaVersion1 {
		t.Fatalf("expected SchemaVersion unchanged at %d, got %d", runs.SchemaVersion1, rec.SchemaVersion)
	}
	if rec.SourceRunID != "" {
		t.Fatalf("expected SourceRunID zero-valued on an old-shape record, got %q", rec.SourceRunID)
	}
}

// TestWriter_MarkPRCreated_PersistsPRUrl is task 3.3 (RED): MarkPRCreated
// persists the given PR URL to run.json, mirroring MarkCanceled's method
// shape (load -> mutate -> save) (run-persistence spec: "MarkPRCreated
// persists the PR URL").
func TestWriter_MarkPRCreated_PersistsPRUrl(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{
		RunID:     "TICKET-10-to-UAT-20260727120000",
		Ticket:    "TICKET-10",
		Target:    "UAT",
		Status:    "Succeeded",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	dir, err := w.Create(rec, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error creating run: %v", err)
	}

	prURL := "https://github.com/org/repo/pull/99"
	if err := w.MarkPRCreated(rec.RunID, prURL); err != nil {
		t.Fatalf("unexpected error on MarkPRCreated: %v", err)
	}

	runData, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatalf("expected run.json to exist: %v", err)
	}
	var gotRec runs.Record
	if err := json.Unmarshal(runData, &gotRec); err != nil {
		t.Fatalf("run.json is not valid JSON: %v", err)
	}
	if gotRec.PRUrl != prURL {
		t.Fatalf("expected run.json PRUrl updated to %q, got %q", prURL, gotRec.PRUrl)
	}
	if !gotRec.UpdatedAt.After(rec.UpdatedAt) {
		t.Fatalf("expected UpdatedAt to advance, got %v (original %v)", gotRec.UpdatedAt, rec.UpdatedAt)
	}
	if gotRec.RunID != rec.RunID || gotRec.Ticket != rec.Ticket {
		t.Fatalf("expected identifying fields preserved, got %+v", gotRec)
	}
}

// TestWriter_MarkPRCreated_UnknownRunIDErrors is task 3.3 (RED):
// MarkPRCreated errors on an unknown runID rather than a silent no-op,
// mirroring MarkCanceled's own unknown-runID behavior.
func TestWriter_MarkPRCreated_UnknownRunIDErrors(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	err := w.MarkPRCreated("does-not-exist", "https://github.com/org/repo/pull/1")
	if err == nil {
		t.Fatal("expected an error for an unknown runID, got nil")
	}
}

// --- HU-013: additive Record growth + List/Load/Save/Prune -----------------

// mustSave is a small seeding helper for the tests below: it calls Save and
// fails the test on error.
func mustSave(t *testing.T, w *runs.Writer, rec runs.Record) {
	t.Helper()
	if err := w.Save(rec); err != nil {
		t.Fatalf("seeding run %s: %v", rec.RunID, err)
	}
}

// TestRecord_BackwardCompat_OldShapeRunJSONStillLoads is task 1.1 (RED):
// a run.json written before PickIndex/PickTotal/CurrentCommit/Phase/Commits
// existed must still Load cleanly, with the new fields zero-valued
// (run-persistence spec: "Additive Record Growth With Backward Compatibility").
func TestRecord_BackwardCompat_OldShapeRunJSONStillLoads(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	runID := "TICKET-1-to-UAT-20260101000000"
	dir := filepath.Join(base, ".deploydeck", "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seeding run dir: %v", err)
	}
	// Old-shape run.json: exactly the pre-HU-013 field set, nothing more.
	oldShape := `{"schemaVersion":1,"runId":"TICKET-1-to-UAT-20260101000000","ticket":"TICKET-1","target":"UAT","alias":"UAT_SANDBOX","jobId":"0Af1","status":"Queued","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(oldShape), 0o644); err != nil {
		t.Fatalf("seeding old-shape run.json: %v", err)
	}

	rec, err := w.Load(runID)
	if err != nil {
		t.Fatalf("an old-shape run.json should still Load cleanly: %v", err)
	}
	if rec.RunID != runID || rec.Ticket != "TICKET-1" || rec.Target != "UAT" || rec.JobID != "0Af1" {
		t.Fatalf("expected the existing fields to round-trip, got %+v", rec)
	}
	if rec.PickIndex != 0 || rec.PickTotal != 0 || rec.CurrentCommit != "" || rec.Phase != "" || rec.Commits != nil {
		t.Fatalf("expected every new field zero-valued on an old-shape record, got %+v", rec)
	}
}

// TestRecord_NewFields_RoundTripUnchanged is task 1.1 (RED): a record with
// PickIndex/PickTotal/CurrentCommit/Phase/Commits set round-trips unchanged
// through Save then Load.
func TestRecord_NewFields_RoundTripUnchanged(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	rec := runs.Record{
		RunID:         "TICKET-2-to-UAT-20260727120000",
		Ticket:        "TICKET-2",
		Target:        "UAT",
		Commits:       []string{"aaa111", "bbb222"},
		PickIndex:     1,
		PickTotal:     2,
		CurrentCommit: "aaa111",
		Phase:         "git-conflict",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := w.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := w.Load(rec.RunID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Commits) != 2 || got.Commits[0] != "aaa111" || got.Commits[1] != "bbb222" {
		t.Errorf("Commits did not round-trip, got %+v", got.Commits)
	}
	if got.PickIndex != 1 || got.PickTotal != 2 || got.CurrentCommit != "aaa111" || got.Phase != "git-conflict" {
		t.Errorf("new fields did not round-trip, got %+v", got)
	}
}

// TestWriter_List_ReturnsNewestFirstAndSkipsMalformed is task 1.3 (RED):
// List scans .deploydeck/runs/*/run.json and returns records newest-first by
// CreatedAt; a malformed run dir is skipped rather than failing the listing
// (run-persistence spec: "List Runs Newest First").
func TestWriter_List_ReturnsNewestFirstAndSkipsMalformed(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	middle := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	mustSave(t, w, runs.Record{RunID: "run-old", CreatedAt: older})
	mustSave(t, w, runs.Record{RunID: "run-mid", CreatedAt: middle})
	mustSave(t, w, runs.Record{RunID: "run-new", CreatedAt: newest})

	// A malformed run dir (unparseable run.json) must be skipped, not fail
	// the whole listing.
	badDir := filepath.Join(base, ".deploydeck", "runs", "run-bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("seeding malformed run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "run.json"), []byte("not json"), 0o644); err != nil {
		t.Fatalf("seeding malformed run.json: %v", err)
	}

	got, err := w.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 valid runs (malformed skipped), got %d: %+v", len(got), got)
	}
	wantOrder := []string{"run-new", "run-mid", "run-old"}
	for i, id := range wantOrder {
		if got[i].RunID != id {
			t.Errorf("position %d = %q, want %q (newest-first)", i, got[i].RunID, id)
		}
	}
}

// TestWriter_List_NoRunsDirReturnsEmpty proves List degrades to an empty
// (nil, nil) result rather than erroring when .deploydeck/runs/ does not
// exist yet (a fresh repo with no runs ever created).
func TestWriter_List_NoRunsDirReturnsEmpty(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	got, err := w.List()
	if err != nil {
		t.Fatalf("List on a repo with no runs dir should not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected an empty list, got %+v", got)
	}
}

// TestWriter_Load_ReturnsPersistedRecord is task 1.5 (RED): Load(runID)
// returns the persisted Record for an existing run (run-persistence spec:
// "Load A Single Run By ID").
func TestWriter_Load_ReturnsPersistedRecord(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)
	mustSave(t, w, runs.Record{RunID: "run-1", Ticket: "T-1"})

	got, err := w.Load("run-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RunID != "run-1" || got.Ticket != "T-1" {
		t.Errorf("unexpected record: %+v", got)
	}
}

// TestWriter_Load_MissingRunIDErrors is task 1.5 (RED): Load errors on an
// unknown runID rather than returning a silent zero Record.
func TestWriter_Load_MissingRunIDErrors(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	if _, err := w.Load("does-not-exist"); err == nil {
		t.Fatal("expected an error for a missing runID, got nil")
	}
}

// TestWriter_Save_UpsertsRunJSONAndForcesSchemaVersion is task 1.7 (RED):
// Save upserts run.json (creating the run dir if needed) and forces
// SchemaVersion to SchemaVersion1, exactly like Create.
func TestWriter_Save_UpsertsRunJSONAndForcesSchemaVersion(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)

	rec := runs.Record{SchemaVersion: 99, RunID: "run-save", Ticket: "T-1", Status: "Queued"}
	if err := w.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := w.Load("run-save")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SchemaVersion != runs.SchemaVersion1 {
		t.Errorf("expected SchemaVersion forced to %d, got %d", runs.SchemaVersion1, got.SchemaVersion)
	}
	if got.Status != "Queued" {
		t.Errorf("expected Status %q, got %q", "Queued", got.Status)
	}

	// A second Save upserts (overwrites) rather than duplicating.
	got.Status = "InProgress"
	if err := w.Save(got); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	got2, err := w.Load("run-save")
	if err != nil {
		t.Fatalf("Load after upsert: %v", err)
	}
	if got2.Status != "InProgress" {
		t.Errorf("expected upserted Status %q, got %q", "InProgress", got2.Status)
	}
	entries, err := os.ReadDir(filepath.Join(base, ".deploydeck", "runs"))
	if err != nil {
		t.Fatalf("reading runs dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 run dir after upsert (no duplicate), got %d", len(entries))
	}
}

// TestWriter_Prune_RemovesOnlyRunsOutsideRetentionWindow is task 1.11 (RED):
// a mixed fixture removes only the runs outside BOTH keepLast and keepDays,
// returns their IDs, and leaves every other run untouched (run-persistence
// spec: "Prune Removes Runs Outside The Retention Window").
func TestWriter_Prune_RemovesOnlyRunsOutsideRetentionWindow(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	mustSave(t, w, runs.Record{RunID: "keep-recent", CreatedAt: now.Add(-1 * day)})
	mustSave(t, w, runs.Record{RunID: "keep-by-age", CreatedAt: now.Add(-3 * day)})
	mustSave(t, w, runs.Record{RunID: "prune-me", CreatedAt: now.Add(-100 * day)})

	removed, err := w.Prune(2, 5, now)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != "prune-me" {
		t.Fatalf("expected only prune-me removed, got %v", removed)
	}

	if _, err := os.Stat(filepath.Join(base, ".deploydeck", "runs", "prune-me")); !os.IsNotExist(err) {
		t.Error("prune-me directory should have been removed")
	}
	for _, keep := range []string{"keep-recent", "keep-by-age"} {
		if _, err := os.Stat(filepath.Join(base, ".deploydeck", "runs", keep)); err != nil {
			t.Errorf("%s should still exist untouched: %v", keep, err)
		}
	}
}

// TestWriter_Prune_NeverTouchesArbitraryPaths is the threat-matrix guard
// (design.md: "Prune removes ONLY <baseDir>/.deploydeck/runs/<runID>
// directories enumerated by List — never arbitrary or user-supplied paths"):
// a sibling directory OUTSIDE .deploydeck/runs/ must survive Prune even when
// every real run is pruned.
func TestWriter_Prune_NeverTouchesArbitraryPaths(t *testing.T) {
	base := t.TempDir()
	w := runs.NewWriter(base)
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	mustSave(t, w, runs.Record{RunID: "prune-me", CreatedAt: now.Add(-100 * 24 * time.Hour)})

	sibling := filepath.Join(base, "some-other-dir")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatalf("seeding sibling dir: %v", err)
	}

	if _, err := w.Prune(0, 0, now); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if _, err := os.Stat(sibling); err != nil {
		t.Errorf("Prune must never touch paths outside .deploydeck/runs/, sibling dir gone: %v", err)
	}
}
