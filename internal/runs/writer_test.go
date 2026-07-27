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
