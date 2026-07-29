package runs

import (
	"os"
	"time"
)

// terminalStatuses mirrors salesforce.terminalStatuses (source of truth:
// internal/salesforce/report.go) — duped here, never imported, so this
// package stays below the Salesforce adapter (design's L-1 layering
// decision: internal/runs must not import internal/salesforce or
// internal/app).
var terminalStatuses = map[string]bool{
	"Succeeded":        true,
	"SucceededPartial": true,
	"Failed":           true,
	"Canceled":         true,
}

// isProtectedFromPrune reports whether rec is non-terminal / still
// resumable and therefore must never be pruned, regardless of the
// count/age selection outcome (run-retention spec: "Non-Terminal Or
// Resumable Runs Are Never Pruned"): a jobId with a non-terminal Status (a
// validation in flight), or an unfinished cherry-pick Phase — "cherry-pick"
// and "git-conflict" mirror app.isCherryPickPhase (internal/app/update.go),
// duped here for the same layering reason as terminalStatuses above.
func isProtectedFromPrune(rec Record) bool {
	if rec.JobID != "" && !terminalStatuses[rec.Status] {
		return true
	}
	return rec.Phase == "cherry-pick" || rec.Phase == "git-conflict"
}

// selectPruneCandidates is the pure run-retention selection rule
// (run-retention spec: "A Run Is Kept If Recent-By-Count OR Recent-By-Age"):
// a run is KEPT if it is within the most-recent keepLast runs (by rank —
// records are assumed already sorted newest-first, List's contract) OR its
// age is <= keepDays. It is PRUNED only when it is outside BOTH conditions.
// keepLast=0 keeps nothing by count; keepDays=0 keeps nothing by age (both
// fall out of the same two comparisons with no special-casing needed). A
// protected run (isProtectedFromPrune) is skipped FIRST, before the
// count/age check, so it is always kept even outside the window — this does
// not shift the count rank `i` for any other record, so every other
// record's count/age math stays byte-identical.
func selectPruneCandidates(records []Record, keepLast, keepDays int, now time.Time) []string {
	var pruned []string
	for i, rec := range records {
		if isProtectedFromPrune(rec) {
			continue
		}
		keptByCount := i < keepLast
		age := now.Sub(rec.CreatedAt)
		keptByAge := age <= time.Duration(keepDays)*24*time.Hour
		if keptByCount || keptByAge {
			continue
		}
		pruned = append(pruned, rec.RunID)
	}
	return pruned
}

// Prune deletes the on-disk directory for every run selectPruneCandidates
// selects and returns the removed run IDs, leaving every other run untouched
// (run-persistence spec: "Prune Removes Runs Outside The Retention Window").
// It touches ONLY <baseDir>/.deploydeck/runs/<runID> directories enumerated
// by List — never arbitrary or user-supplied paths (design's threat matrix).
func (w *Writer) Prune(keepLast, keepDays int, now time.Time) ([]string, error) {
	records, err := w.List()
	if err != nil {
		return nil, err
	}

	removed := selectPruneCandidates(records, keepLast, keepDays, now)
	for _, runID := range removed {
		if err := os.RemoveAll(w.runDir(runID)); err != nil {
			return removed, err
		}
	}
	return removed, nil
}
