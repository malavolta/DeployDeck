package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepoState reconciles the full cherry-pick state directly from the
// repository — CHERRY_PICK_HEAD, .git/sequencer/todo, and
// `git status --porcelain -z` — on EVERY call, never trusting a cached or
// in-memory model (design "repo is source of truth"). Because it always
// re-reads, an external `git cherry-pick --continue`/`--skip`/`--abort` run
// outside DeployDeck is detected here automatically (HU-006 external-action
// reconciliation, AC6).
func (s *Service) RepoState(ctx context.Context, dir string) (RepoState, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return RepoState{}, err
	}

	gitDir, err := s.gitDir(ctx, root)
	if err != nil {
		return RepoState{}, err
	}

	state := RepoState{}

	// CHERRY_PICK_HEAD: existence => a pick is in progress; its content is
	// the SHA of the commit currently being applied.
	if sha, ok := readTrimmedFile(filepath.Join(gitDir, "CHERRY_PICK_HEAD")); ok {
		state.InProgress = true
		state.CurrentSHA = sha
	}

	// .git/sequencer/todo: remaining pick instructions.
	if todo, ok := readTrimmedFile(filepath.Join(gitDir, "sequencer", "todo")); ok {
		state.SequencerRemaining = countSequencerTodo(todo)
	}

	// `git status --porcelain -z`: clean flag + unmerged conflict files.
	porcelain, err := s.statusPorcelainZ(ctx, root)
	if err != nil {
		return RepoState{}, err
	}
	state.Clean = strings.TrimSpace(string(porcelain)) == ""

	entries := parsePorcelainZ(porcelain)
	binaryPaths := s.binaryConflictPaths(ctx, root, entries)
	state.Unmerged = ClassifyConflicts(porcelain, binaryPaths)

	return state, nil
}

// gitDir returns the absolute path of the repository's git directory (the
// per-worktree one), where CHERRY_PICK_HEAD and sequencer/ live.
func (s *Service) gitDir(ctx context.Context, root string) (string, error) {
	req := newRequest(root, "rev-parse", "--absolute-git-dir")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("git: resolving git dir in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("git: resolving git dir in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// statusPorcelainZ returns raw `git status --porcelain -z` output (NUL
// delimited so paths with spaces/unicode parse safely).
func (s *Service) statusPorcelainZ(ctx context.Context, root string) ([]byte, error) {
	req := newRequest(root, "status", "--porcelain", "-z")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: status in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: status in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}
	return result.Stdout, nil
}

// binaryConflictPaths determines, for each both-sides-present unmerged entry
// (UU/AA), whether its blob is binary by diffing the two conflict stages
// (`git diff --numstat :2:<path> :3:<path>` => "-\t-" for binary). Modify/
// delete and both-deleted conflicts are classified by their XY code alone
// and never probed here.
func (s *Service) binaryConflictPaths(ctx context.Context, root string, entries []porcelainEntry) map[string]bool {
	binary := map[string]bool{}
	for _, e := range entries {
		if e.XY != "UU" && e.XY != "AA" {
			continue
		}
		req := newRequest(root, "diff", "--numstat", ":2:"+e.Path, ":3:"+e.Path)
		result, err := s.runner.Run(ctx, req)
		if err != nil || result.ExitCode != 0 {
			// Best-effort: if the stages cannot be diffed, leave it as text.
			continue
		}
		if numstatIsBinary(result.Stdout) {
			binary[e.Path] = true
		}
	}
	return binary
}

// readTrimmedFile reads path and returns its trimmed content plus whether it
// exists. A missing file is not an error (idle repo); any other read error
// also reports absence conservatively.
func readTrimmedFile(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

// countSequencerTodo counts actionable instruction lines (pick/revert/edit/
// reword/squash/fixup) in a .git/sequencer/todo body, ignoring comments and
// blank lines.
func countSequencerTodo(todo string) int {
	count := 0
	for _, line := range strings.Split(todo, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch strings.Fields(line)[0] {
		case "pick", "p", "revert", "edit", "e", "reword", "r", "squash", "s", "fixup", "f":
			count++
		}
	}
	return count
}
