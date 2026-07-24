package git

import "testing"

// TestDeltaAndValidationAllowed_TableDriven pins AC11 (tasks 10.35/10.36):
// delta generation and Salesforce validation are gated off whenever a
// cherry-pick is still in progress or the run was aborted.
func TestDeltaAndValidationAllowed_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		state   RepoState
		aborted bool
		want    bool
	}{
		{name: "in progress (conflict) -> blocked", state: RepoState{InProgress: true}, want: false},
		{name: "aborted -> blocked", state: RepoState{InProgress: false}, aborted: true, want: false},
		{name: "completed clean -> allowed", state: RepoState{InProgress: false, Clean: true}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeltaAndValidationAllowed(tt.state, tt.aborted); got != tt.want {
				t.Errorf("DeltaAndValidationAllowed(%+v, aborted=%v) = %v, want %v", tt.state, tt.aborted, got, tt.want)
			}
		})
	}
}

// TestPickVerification_Warnings pins the per-file warning rendering.
func TestPickVerification_Warnings(t *testing.T) {
	v := PickVerification{PartialFiles: []string{"a.cls", "b.cls"}}
	if v.OK() {
		t.Errorf("expected OK()=false when partial files exist")
	}
	if len(v.Warnings()) != 2 {
		t.Errorf("expected one warning per partial file, got %v", v.Warnings())
	}
	empty := PickVerification{}
	if !empty.OK() {
		t.Errorf("expected OK()=true with no partial files")
	}
}
