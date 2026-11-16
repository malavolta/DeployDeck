package runs

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// ErrNoReport is returned by LatestReport when a run has no persisted
// report-<NNN>.json that the caller accepts (none exist, or all rejected).
var ErrNoReport = errors.New("runs: no persisted report")

// ReportFile is one persisted report-<NNN>.json read back from disk.
type ReportFile struct {
	Path string
	Raw  []byte
}

// LatestReport scans the run's report-<NNN>.json files newest-first and
// returns the first one accept takes, plus the paths it rejected on the way
// (newest-first). Files are read lazily, so a run with hundreds of polls
// stops at the first good file. The accept callback keeps this package free
// of any salesforce dependency. It returns ErrNoReport (with the rejected
// paths) when nothing is accepted. Read-only: nothing is executed.
func (w *Writer) LatestReport(runID string, accept func([]byte) bool) (ReportFile, []string, error) {
	// RunIDs come from List, but never let a crafted one escape runs/.
	if runID == "" || filepath.Base(runID) != runID {
		return ReportFile{}, nil, ErrNoReport
	}
	dir := w.runDir(runID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ReportFile{}, nil, ErrNoReport
	}
	type numbered struct {
		n    int
		name string
	}
	var files []numbered
	for _, e := range entries {
		m := reportFilePattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		files = append(files, numbered{n, e.Name()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].n > files[j].n })

	var rejected []string
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		raw, err := os.ReadFile(path)
		if err != nil || !accept(raw) {
			rejected = append(rejected, path)
			continue
		}
		return ReportFile{Path: path, Raw: raw}, rejected, nil
	}
	return ReportFile{}, rejected, ErrNoReport
}
