package git

import (
	"reflect"
	"testing"
)

// TestEvaluateContinueGate_TableDriven pins AC3 (tasks 10.17/10.18): the pure
// continue-gate predicate. Continue is enabled only when zero paths are
// unmerged AND no staged file still contains conflict markers; otherwise it
// is disabled with the pending detail.
func TestEvaluateContinueGate_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		state       RepoState
		markerFiles []string
		wantEnabled bool
		wantMarkers []string
		pendingHas  string
	}{
		{
			name:        "clean and no markers -> enabled",
			state:       RepoState{InProgress: true},
			wantEnabled: true,
		},
		{
			name: "unmerged path -> disabled with pending detail",
			state: RepoState{InProgress: true, Unmerged: []ConflictFile{
				{Path: "a.cls", Kind: ConflictText},
			}},
			wantEnabled: false,
			pendingHas:  "a.cls",
		},
		{
			name:        "staged conflict marker -> disabled naming the file",
			state:       RepoState{InProgress: true},
			markerFiles: []string{"b.cls"},
			wantEnabled: false,
			wantMarkers: []string{"b.cls"},
			pendingHas:  "b.cls",
		},
		{
			name: "both unmerged and markers -> disabled",
			state: RepoState{InProgress: true, Unmerged: []ConflictFile{
				{Path: "a.cls", Kind: ConflictModifyDelete},
			}},
			markerFiles: []string{"b.cls"},
			wantEnabled: false,
			wantMarkers: []string{"b.cls"},
			pendingHas:  "a.cls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate := EvaluateContinueGate(tt.state, tt.markerFiles)
			if gate.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v (gate=%+v)", gate.Enabled, tt.wantEnabled, gate)
			}
			if tt.wantMarkers != nil && !reflect.DeepEqual(gate.MarkerFiles, tt.wantMarkers) {
				t.Errorf("MarkerFiles = %v, want %v", gate.MarkerFiles, tt.wantMarkers)
			}
			if tt.pendingHas != "" {
				found := false
				for _, p := range gate.Pending {
					if containsStr(p, tt.pendingHas) {
						found = true
					}
				}
				if !found {
					t.Errorf("expected a pending detail mentioning %q, got %v", tt.pendingHas, gate.Pending)
				}
			}
			if !tt.wantEnabled && len(gate.Pending) == 0 {
				t.Errorf("a disabled gate must report at least one pending detail")
			}
		})
	}
}

// TestParseCheckMarkers pins the `git diff --cached --check` parser: it
// extracts the unique file paths flagged with leftover conflict markers,
// including paths containing spaces.
func TestParseCheckMarkers(t *testing.T) {
	raw := []byte("a.cls:2: leftover conflict marker\n" +
		"a.cls:4: leftover conflict marker\n" +
		"dir sp/b.cls:1: leftover conflict marker\n" +
		"c.cls:3: trailing whitespace.\n")

	got := parseCheckMarkers(raw)
	want := []string{"a.cls", "dir sp/b.cls"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseCheckMarkers() = %v, want %v", got, want)
	}
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
