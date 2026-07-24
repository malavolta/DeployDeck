package git

import "testing"

// TestIsEmptyPickMessage pins the empty-pick signal parse: git's
// "is now empty" advice marks an empty cherry-pick; a normal conflict or
// success does not.
func TestIsEmptyPickMessage(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "empty pick advice", output: "The previous cherry-pick is now empty, possibly due to conflict resolution.", want: true},
		{name: "normal conflict", output: "CONFLICT (content): Merge conflict in a.cls", want: false},
		{name: "success", output: "[UAT 1234abc] PROJ-1: add A", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyPickMessage(tt.output); got != tt.want {
				t.Errorf("isEmptyPickMessage(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

// TestOfferPartialBranchCleanup pins the pure cleanup-offer predicate.
func TestOfferPartialBranchCleanup(t *testing.T) {
	if OfferPartialBranchCleanup(0) {
		t.Errorf("no picks applied -> no cleanup offer")
	}
	if !OfferPartialBranchCleanup(1) {
		t.Errorf("one applied pick -> cleanup offer expected")
	}
}
