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
