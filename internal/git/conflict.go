package git

import (
	"bytes"
	"strings"
)

// ConflictKind classifies a cherry-pick conflict for the resolution UX
// (HU-006): text conflicts get marker resolution, binary conflicts get an
// ours/theirs choice, and modify/delete conflicts get a keep/delete choice.
type ConflictKind int

const (
	// ConflictText is a normal content conflict on a text file (porcelain
	// UU/AA on a non-binary blob): resolved by editing out the markers.
	ConflictText ConflictKind = iota
	// ConflictBinary is a content conflict where the blob is binary (static
	// resources): resolved via `git checkout --theirs/--ours`.
	ConflictBinary
	// ConflictModifyDelete is a modify/delete conflict (porcelain DU/UD):
	// one side deleted the file while the other modified it — resolved by an
	// explicit keep (`git add`) or delete (`git rm`) choice.
	ConflictModifyDelete
	// ConflictBothDeleted is a both-deleted conflict (porcelain DD): both
	// sides removed the file. Staging the deletion (`git rm`) resolves it.
	ConflictBothDeleted
)

// String renders a ConflictKind for user-facing conflict lists.
func (k ConflictKind) String() string {
	switch k {
	case ConflictText:
		return "text"
	case ConflictBinary:
		return "binary"
	case ConflictModifyDelete:
		return "modify/delete"
	case ConflictBothDeleted:
		return "both-deleted"
	default:
		return "unknown"
	}
}

// ConflictFile is a single unmerged path plus its classification, shown in
// the conflict screen so the user resolves each with the right primitive.
type ConflictFile struct {
	Path string
	Kind ConflictKind
}

// porcelainEntry is one `git status --porcelain -z` record: the two-character
// XY status code and the literal (unquoted, NUL-delimited) path.
type porcelainEntry struct {
	XY   string
	Path string
}

// parsePorcelainZ parses `git status --porcelain -z` output. The -z form is
// NUL-delimited and, crucially, does NOT quote paths — so paths containing
// spaces or unicode split safely, unlike the default space-delimited
// porcelain format (task 10.3). A staged rename/copy carries its source path
// as an extra NUL-separated record, which is consumed so later entries stay
// aligned.
func parsePorcelainZ(raw []byte) []porcelainEntry {
	records := bytes.Split(raw, []byte{0})

	var entries []porcelainEntry
	for i := 0; i < len(records); i++ {
		rec := records[i]
		// Minimum record is "XY p": 2 status chars, a space, >=1 path char.
		if len(rec) < 4 {
			continue
		}
		xy := string(rec[0:2])
		path := string(rec[3:])
		entries = append(entries, porcelainEntry{XY: xy, Path: path})

		// Rename/copy in the index emits a second record with the source
		// path; skip it so we never misread it as its own entry.
		if xy[0] == 'R' || xy[0] == 'C' {
			i++
		}
	}
	return entries
}

// isUnmergedCode reports whether an XY code is one of git's unmerged
// (conflict) states: DD, AU, UD, UA, DU, AA, UU.
func isUnmergedCode(xy string) bool {
	switch xy {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return false
	}
}

// classifyConflictKind maps an unmerged XY code (plus whether the blob is
// binary) to a ConflictKind. DU/UD are modify/delete, DD is both-deleted,
// and the both-present codes (UU/AA and the one-sided AU/UA) split on the
// binary marker.
func classifyConflictKind(xy string, binary bool) ConflictKind {
	switch xy {
	case "DD":
		return ConflictBothDeleted
	case "DU", "UD":
		return ConflictModifyDelete
	default:
		if binary {
			return ConflictBinary
		}
		return ConflictText
	}
}

// ClassifyConflicts turns raw `git status --porcelain -z` output plus a set
// of known-binary paths into the classified conflict list. Non-unmerged
// entries (staged/unstaged/untracked) are ignored. binaryPaths may be nil.
func ClassifyConflicts(porcelainZ []byte, binaryPaths map[string]bool) []ConflictFile {
	var conflicts []ConflictFile
	for _, e := range parsePorcelainZ(porcelainZ) {
		if !isUnmergedCode(e.XY) {
			continue
		}
		conflicts = append(conflicts, ConflictFile{
			Path: e.Path,
			Kind: classifyConflictKind(e.XY, binaryPaths[e.Path]),
		})
	}
	return conflicts
}

// numstatIsBinary reports whether a single-file `git diff --numstat` output
// marks the file binary. Git prints "-\t-\t<path>" for binary blobs and real
// added/deleted line counts for text, so a leading "-" field is the binary
// sentinel.
func numstatIsBinary(raw []byte) bool {
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return false
	}
	// First line only; fields are tab-delimited.
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	fields := strings.SplitN(line, "\t", 2)
	return len(fields) > 0 && fields[0] == "-"
}
