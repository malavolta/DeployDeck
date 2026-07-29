package runs

import (
	"testing"
	"time"
)

// TestSelectPruneCandidates is task 1.9 (RED): the pure retention-selection
// rule (run-retention spec: "A Run Is Kept If Recent-By-Count OR
// Recent-By-Age") — a run is KEPT if it is within the most-recent keepLast
// runs (by rank, records assumed pre-sorted newest-first per List's
// contract) OR its age is <= keepDays; PRUNED only when outside BOTH.
func TestSelectPruneCandidates(t *testing.T) {
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	rec := func(id string, age time.Duration) Record {
		return Record{RunID: id, CreatedAt: now.Add(-age)}
	}

	tests := []struct {
		name     string
		records  []Record
		keepLast int
		keepDays int
		want     []string
	}{
		{
			name: "kept by recency-count despite being older than keepDays",
			records: []Record{
				rec("rank0", 1*day),
				rec("rank1-old-but-within-keepLast", 100*day),
			},
			keepLast: 2,
			keepDays: 5,
			want:     nil,
		},
		{
			name: "kept by age despite being outside the most-recent keepLast",
			records: []Record{
				rec("rank0", 1*day),
				rec("rank1", 2*day),
				rec("rank2-outside-keepLast-but-fresh", 3*day),
			},
			keepLast: 2,
			keepDays: 10,
			want:     nil,
		},
		{
			name: "pruned when outside both conditions",
			records: []Record{
				rec("rank0", 1*day),
				rec("rank1", 2*day),
				rec("prune-me", 100*day),
			},
			keepLast: 2,
			keepDays: 5,
			want:     []string{"prune-me"},
		},
		{
			name: "boundary age==keepDays is kept",
			records: []Record{
				rec("rank0", 1*day),
				rec("boundary", 5*day),
			},
			keepLast: 1,
			keepDays: 5,
			want:     nil,
		},
		{
			name: "keepLast=0 keeps nothing by count, only by age",
			records: []Record{
				rec("fresh", 1*day),
				rec("stale", 10*day),
			},
			keepLast: 0,
			keepDays: 5,
			want:     []string{"stale"},
		},
		{
			name:     "empty records prunes nothing",
			records:  nil,
			keepLast: 30,
			keepDays: 90,
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectPruneCandidates(tt.records, tt.keepLast, tt.keepDays, now)
			if !equalStringSlices(got, tt.want) {
				t.Fatalf("selectPruneCandidates() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsProtectedFromPrune is task 1.1 (RED): a run is protected from
// pruning, regardless of the count/age selection outcome, when it is
// non-terminal / still resumable — a jobId with a non-terminal Status (a
// validation in flight), or an unfinished cherry-pick Phase (run-retention
// spec: "Non-Terminal Or Resumable Runs Are Never Pruned").
func TestIsProtectedFromPrune(t *testing.T) {
	tests := []struct {
		name string
		rec  Record
		want bool
	}{
		{
			name: "jobId with non-terminal status is protected",
			rec:  Record{JobID: "0Af1", Status: "InProgress"},
			want: true,
		},
		{
			name: "unfinished cherry-pick phase is protected",
			rec:  Record{Phase: "cherry-pick"},
			want: true,
		},
		{
			name: "unresolved git-conflict phase is protected",
			rec:  Record{Phase: "git-conflict"},
			want: true,
		},
		{
			name: "jobId with terminal status is not protected",
			rec:  Record{JobID: "0Af1", Status: "Succeeded"},
			want: false,
		},
		{
			name: "no jobId and no unfinished phase is not protected",
			rec:  Record{Mode: "delta"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isProtectedFromPrune(tt.rec); got != tt.want {
				t.Errorf("isProtectedFromPrune(%+v) = %v, want %v", tt.rec, got, tt.want)
			}
		})
	}
}

// TestSelectPruneCandidates_ProtectsResumableRuns is task 1.2 (RED): extends
// (never replaces — TestSelectPruneCandidates above stays untouched) the
// count/age selection rule with the protected-run skip (run-retention
// spec, all 4 scenarios): an in-flight non-terminal run or an unfinished
// cherry-pick outside both keepLast/keepDays is KEPT; a terminal old run
// outside the window is STILL pruned; recent/count-kept runs stay kept
// regardless of status.
func TestSelectPruneCandidates_ProtectsResumableRuns(t *testing.T) {
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	rec := func(id string, age time.Duration, jobID, status, phase string) Record {
		return Record{RunID: id, CreatedAt: now.Add(-age), JobID: jobID, Status: status, Phase: phase}
	}

	tests := []struct {
		name     string
		records  []Record
		keepLast int
		keepDays int
		want     []string
	}{
		{
			name: "non-terminal in-flight run outside the window is kept",
			records: []Record{
				rec("rank0", 1*day, "", "", ""),
				rec("in-flight", 100*day, "0Af1", "InProgress", ""),
			},
			keepLast: 1,
			keepDays: 5,
			want:     nil,
		},
		{
			name: "unfinished cherry-pick outside the window is kept",
			records: []Record{
				rec("rank0", 1*day, "", "", ""),
				rec("mid-pick", 100*day, "", "", "cherry-pick"),
			},
			keepLast: 1,
			keepDays: 5,
			want:     nil,
		},
		{
			name: "terminal old run outside the window is still pruned",
			records: []Record{
				rec("rank0", 1*day, "", "", ""),
				rec("terminal-old", 100*day, "0Af1", "Succeeded", "done"),
			},
			keepLast: 1,
			keepDays: 5,
			want:     []string{"terminal-old"},
		},
		{
			name: "recent/count-kept runs remain kept regardless of status",
			records: []Record{
				rec("recent-in-flight", 1*day, "0Af1", "InProgress", ""),
			},
			keepLast: 1,
			keepDays: 5,
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectPruneCandidates(tt.records, tt.keepLast, tt.keepDays, now)
			if !equalStringSlices(got, tt.want) {
				t.Fatalf("selectPruneCandidates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
