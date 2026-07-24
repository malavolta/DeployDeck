package git

import (
	"reflect"
	"testing"
)

// TestClassifyConflicts_TableDriven pins HU-006's conflict classification
// (tasks 10.1/10.2): porcelain unmerged XY codes map to a ConflictKind, with
// DU/UD -> ModifyDelete, DD -> BothDeleted, and the both-modified codes
// (UU/AA) splitting on the numstat binary marker.
func TestClassifyConflicts_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		porcelainZ  []byte
		binaryPaths map[string]bool
		want        []ConflictFile
	}{
		{
			name:       "text content conflict UU",
			porcelainZ: []byte("UU a.txt\x00"),
			want:       []ConflictFile{{Path: "a.txt", Kind: ConflictText}},
		},
		{
			name:        "binary content conflict UU with numstat marker",
			porcelainZ:  []byte("UU img.png\x00"),
			binaryPaths: map[string]bool{"img.png": true},
			want:        []ConflictFile{{Path: "img.png", Kind: ConflictBinary}},
		},
		{
			name:       "deleted by us / modified by them -> modify-delete",
			porcelainZ: []byte("DU f.cls\x00"),
			want:       []ConflictFile{{Path: "f.cls", Kind: ConflictModifyDelete}},
		},
		{
			name:       "modified by us / deleted by them -> modify-delete",
			porcelainZ: []byte("UD g.cls\x00"),
			want:       []ConflictFile{{Path: "g.cls", Kind: ConflictModifyDelete}},
		},
		{
			name:       "both deleted -> both-deleted",
			porcelainZ: []byte("DD h.cls\x00"),
			want:       []ConflictFile{{Path: "h.cls", Kind: ConflictBothDeleted}},
		},
		{
			name:       "both added -> text when not binary",
			porcelainZ: []byte("AA new.cls\x00"),
			want:       []ConflictFile{{Path: "new.cls", Kind: ConflictText}},
		},
		{
			name:       "non-conflict entries are ignored",
			porcelainZ: []byte("UU a.txt\x00 M staged.txt\x00?? untracked.txt\x00"),
			want:       []ConflictFile{{Path: "a.txt", Kind: ConflictText}},
		},
		{
			name:       "no output -> no conflicts",
			porcelainZ: []byte(""),
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyConflicts(tt.porcelainZ, tt.binaryPaths)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClassifyConflicts() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestParsePorcelainZ_HandlesPathsWithSpacesAndUnicode pins task 10.3/10.4:
// the -z NUL-delimited porcelain format must split paths containing spaces
// and unicode safely, unlike the default space-delimited format which would
// break on such paths.
func TestParsePorcelainZ_HandlesPathsWithSpacesAndUnicode(t *testing.T) {
	// Two unmerged paths: one with an embedded space, one with unicode.
	raw := []byte("UU force-app/main default/classes/Añez Controller.cls\x00DU dir with space/b.cls\x00")

	got := ClassifyConflicts(raw, nil)
	want := []ConflictFile{
		{Path: "force-app/main default/classes/Añez Controller.cls", Kind: ConflictText},
		{Path: "dir with space/b.cls", Kind: ConflictModifyDelete},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ClassifyConflicts() with spaced/unicode paths = %+v, want %+v", got, want)
	}
}

// TestParsePorcelainZ_SkipsRenameSourcePath ensures a staged rename's second
// NUL-separated source path does not desynchronize parsing of later entries.
func TestParsePorcelainZ_SkipsRenameSourcePath(t *testing.T) {
	// `R  new.cls\0old.cls\0UU conflict.cls\0`
	raw := []byte("R  new.cls\x00old.cls\x00UU conflict.cls\x00")

	got := ClassifyConflicts(raw, nil)
	want := []ConflictFile{{Path: "conflict.cls", Kind: ConflictText}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ClassifyConflicts() after a rename entry = %+v, want %+v", got, want)
	}
}

// TestNumstatIsBinary pins the numstat binary marker parse: `-\t-\t<path>`
// (git's binary sentinel) is binary, real add/delete counts are not.
func TestNumstatIsBinary(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want bool
	}{
		{name: "binary marker", raw: []byte("-\t-\timg.png\n"), want: true},
		{name: "text counts", raw: []byte("1\t1\tt.txt\n"), want: false},
		{name: "empty output", raw: []byte(""), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := numstatIsBinary(tt.raw); got != tt.want {
				t.Errorf("numstatIsBinary(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
