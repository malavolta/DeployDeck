package git_test

import (
	"testing"

	"deploydeck/internal/git"
)

func TestClassifyEquivalence_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		isAncestor     bool
		cherryMarker   byte
		patchID        string
		targetPatchIDs map[string]bool
		want           git.EquivalenceStatus
	}{
		{
			name:       "identical-SHA ancestor of target is already applied",
			isAncestor: true,
			want:       git.AlreadyAppliedBySHA,
		},
		{
			name:         "git cherry '-' marker means equivalent content already applied",
			isAncestor:   false,
			cherryMarker: '-',
			want:         git.EquivalentByCherry,
		},
		{
			name:         "git cherry '+' marker alone means not applied",
			isAncestor:   false,
			cherryMarker: '+',
			want:         git.NotApplied,
		},
		{
			name:           "matching patch-id against a target commit means equivalent",
			isAncestor:     false,
			cherryMarker:   '+',
			patchID:        "deadbeef",
			targetPatchIDs: map[string]bool{"deadbeef": true},
			want:           git.EquivalentByPatchID,
		},
		{
			name:           "non-matching patch-id means not applied",
			isAncestor:     false,
			cherryMarker:   '+',
			patchID:        "cafebabe",
			targetPatchIDs: map[string]bool{"deadbeef": true},
			want:           git.NotApplied,
		},
		{
			name: "no signals at all means not applied",
			want: git.NotApplied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.ClassifyEquivalence(tt.isAncestor, tt.cherryMarker, tt.patchID, tt.targetPatchIDs)
			if got != tt.want {
				t.Fatalf("ClassifyEquivalence() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEquivalenceStatus_AlreadyApplied(t *testing.T) {
	tests := []struct {
		status git.EquivalenceStatus
		want   bool
	}{
		{git.NotApplied, false},
		{git.AlreadyAppliedBySHA, true},
		{git.EquivalentByCherry, true},
		{git.EquivalentByPatchID, true},
	}
	for _, tt := range tests {
		if got := tt.status.AlreadyApplied(); got != tt.want {
			t.Errorf("%v.AlreadyApplied() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestParseCherryOutput_TableDriven(t *testing.T) {
	raw := []byte(
		"+ aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n" +
			"- bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n" +
			"\n",
	)

	markers := git.ParseCherryOutput(raw)

	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %d: %+v", len(markers), markers)
	}
	if markers["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] != '+' {
		t.Errorf("expected '+' marker for aaaa..., got %q", markers["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])
	}
	if markers["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"] != '-' {
		t.Errorf("expected '-' marker for bbbb..., got %q", markers["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"])
	}
}

func TestParsePatchID_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		wantID string
		wantOK bool
	}{
		{
			name:   "well-formed patch-id line",
			raw:    "75ac47e587280444a32ccb562799285cc3e8d699 ec0940e353f0d4a2fdad9b4fa81bbd850e667b4b\n",
			wantID: "75ac47e587280444a32ccb562799285cc3e8d699",
			wantOK: true,
		},
		{
			name:   "empty output (e.g. an empty diff) has no patch-id",
			raw:    "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := git.ParsePatchID([]byte(tt.raw))
			if ok != tt.wantOK {
				t.Fatalf("ParsePatchID() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && id != tt.wantID {
				t.Fatalf("ParsePatchID() = %q, want %q", id, tt.wantID)
			}
		})
	}
}
