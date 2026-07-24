package git

import (
	"context"
	"fmt"
	"strings"
)

// FilesTouchedByCommit returns the file paths sha changes, via
// `git diff --name-only <sha>^ <sha>` — a single non-merge commit's own
// diff against its parent. Merge commits (parent count > 1) have no
// single well-defined parent for this comparison and are never selectable
// (HU-003), so callers skip them rather than calling this method for them.
func (s *Service) FilesTouchedByCommit(ctx context.Context, dir, sha string) ([]string, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	req := newRequest(root, "diff", "--name-only", sha+"^", sha)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: listing files touched by %s in %s: %w", sha, root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: listing files touched by %s in %s: %s", sha, root, strings.TrimSpace(string(result.Stderr)))
	}

	var files []string
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// DependencyWarning flags that a SELECTED commit's file was also touched
// by an EARLIER, UNSELECTED commit in the same ordered range: cherry-
// picking the selected commit alone may produce an incomplete or
// inconsistent result, since the file's current state assumes the earlier
// commit's change is already present (HU-003: "warning de dependencia
// (riesgo de promocion parcial)").
type DependencyWarning struct {
	File          string
	SelectedSHA   string
	UnselectedSHA string
}

// ComputeDependencyWarnings is the pure decision logic behind
// Service.DependencyWarnings: given ordered (topo order, earliest first),
// the current selection, and each commit's touched files (see
// FilesTouchedByCommit), it reports every (file, earlier unselected
// commit) pair that overlaps a later selected commit's files. A commit
// absent from filesBySHA (e.g. a merge commit, deliberately never queried)
// contributes no files and can never appear on either side of a warning.
func ComputeDependencyWarnings(ordered []DiscoveredCommit, selected map[string]bool, filesBySHA map[string][]string) []DependencyWarning {
	var warnings []DependencyWarning

	for i, c := range ordered {
		if !selected[c.SHA] {
			continue
		}
		selectedFiles := filesBySHA[c.SHA]
		if len(selectedFiles) == 0 {
			continue
		}

		for j := 0; j < i; j++ {
			earlier := ordered[j]
			if selected[earlier.SHA] {
				continue
			}
			earlierFiles := filesBySHA[earlier.SHA]
			for _, f := range selectedFiles {
				if containsFile(earlierFiles, f) {
					warnings = append(warnings, DependencyWarning{
						File:          f,
						SelectedSHA:   c.SHA,
						UnselectedSHA: earlier.SHA,
					})
				}
			}
		}
	}

	return warnings
}

func containsFile(files []string, target string) bool {
	for _, f := range files {
		if f == target {
			return true
		}
	}
	return false
}

// DependencyWarnings composes FilesTouchedByCommit (real git, skipping
// merge commits) with ComputeDependencyWarnings (pure) to produce HU-003's
// per-file intermediate-commit dependency warnings for ordered against the
// current selected set.
func (s *Service) DependencyWarnings(ctx context.Context, dir string, ordered []DiscoveredCommit, selected map[string]bool) ([]DependencyWarning, error) {
	filesBySHA := make(map[string][]string, len(ordered))
	for _, c := range ordered {
		if c.Merge {
			continue
		}
		files, err := s.FilesTouchedByCommit(ctx, dir, c.SHA)
		if err != nil {
			return nil, err
		}
		filesBySHA[c.SHA] = files
	}

	return ComputeDependencyWarnings(ordered, selected, filesBySHA), nil
}
