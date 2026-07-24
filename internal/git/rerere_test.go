package git

import (
	"reflect"
	"testing"
)

// TestRerereResolvedPaths pins the rerere auto-resolution parse: git's
// "Resolved '<path>' using previous resolution." lines yield the unique
// resolved paths; other output yields none.
func TestRerereResolvedPaths(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "single auto-resolution",
			output: "Auto-merging a.cls\nCONFLICT (content): Merge conflict in a.cls\nResolved 'a.cls' using previous resolution.\n",
			want:   []string{"a.cls"},
		},
		{
			name:   "two auto-resolutions deduped",
			output: "Resolved 'a.cls' using previous resolution.\nResolved 'dir sp/b.cls' using previous resolution.\nResolved 'a.cls' using previous resolution.\n",
			want:   []string{"a.cls", "dir sp/b.cls"},
		},
		{
			name:   "no rerere output",
			output: "CONFLICT (content): Merge conflict in a.cls\n",
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rerereResolvedPaths(tt.output); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rerereResolvedPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSuggestEnableRerere pins the suggestion predicate.
func TestSuggestEnableRerere(t *testing.T) {
	if !SuggestEnableRerere(false) {
		t.Errorf("rerere disabled -> suggest enabling")
	}
	if SuggestEnableRerere(true) {
		t.Errorf("rerere enabled -> no suggestion")
	}
}
