package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"deploydeck/internal/exec"
)

// ErrContinueBlocked is returned by ContinueCherryPick when the continue-gate
// is not satisfied (unmerged paths remain, or a staged file still has
// conflict markers). No `--continue` is issued in that case.
var ErrContinueBlocked = errors.New("git: cannot continue cherry-pick: conflicts unresolved")

// PickOutcome bundles the reconciled repository state after a cherry-pick
// engine action with the transient signals derived from that action's own
// command output — whether the stopped pick is empty (offer `--skip`) and
// which paths git's rerere auto-resolved from a prior resolution (require
// explicit user confirmation). State is always re-read from the repo.
type PickOutcome struct {
	State          RepoState
	Empty          bool
	EmptyMessage   string
	RerereResolved []string
}

// gpgSignOff is the config override prepended (as `-c commit.gpgsign=false`)
// to every commit-CREATING cherry-pick invocation the tool issues. Promotion
// commits need no signature, and with commit.gpgsign=true (common in
// enterprise Salesforce repos) an unsigned-tty gpg call would hang the run —
// the same non-interactive failure class GIT_EDITOR=true prevents.
var gpgSignOff = []string{"-c", "commit.gpgsign=false"}

// cherryPickRevisions builds the revision argument(s) for a SINGLE
// sequencer-driven cherry-pick invocation. A contiguous ancestry selection
// uses the range form `<first>^..<last>` (git applies each commit in turn,
// populating .git/sequencer/todo); a non-contiguous selection uses the
// explicit ordered SHA list. Never a Go loop of single-sha picks.
func CherryPickRevisions(commits []DiscoveredCommit, contiguous bool) []string {
	if len(commits) == 0 {
		return nil
	}
	if contiguous {
		if len(commits) == 1 {
			return []string{commits[0].SHA}
		}
		return []string{commits[0].SHA + "^.." + commits[len(commits)-1].SHA}
	}
	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.SHA
	}
	return shas
}

// IsContiguousSelection reports whether selected is an unbroken run of
// fullOrdered (topological, earliest first): the selected commits occupy
// consecutive positions with no unselected commit between the first and last
// selected. The caller (internal/app) uses this to choose the range form.
func IsContiguousSelection(fullOrdered, selected []DiscoveredCommit) bool {
	if len(selected) == 0 {
		return false
	}
	sel := map[string]bool{}
	for _, c := range selected {
		sel[c.SHA] = true
	}
	first, last := -1, -1
	for i, c := range fullOrdered {
		if sel[c.SHA] {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	if first == -1 {
		return false
	}
	// Every position between first and last must be selected.
	for i := first; i <= last; i++ {
		if !sel[fullOrdered[i].SHA] {
			return false
		}
	}
	return (last - first + 1) == len(selected)
}

// cherryPickArgs is the full arg list for the initial cherry-pick:
// `-c commit.gpgsign=false cherry-pick <revs...>`.
func cherryPickArgs(revs []string) []string {
	args := append([]string(nil), gpgSignOff...)
	args = append(args, "cherry-pick")
	return append(args, revs...)
}

// continueArgs is the arg list for a non-interactive continue:
// `-c commit.gpgsign=false cherry-pick --continue`.
func continueArgs() []string {
	args := append([]string(nil), gpgSignOff...)
	return append(args, "cherry-pick", "--continue")
}

// skipArgs is the arg list for skipping an empty pick:
// `-c commit.gpgsign=false cherry-pick --skip` (a skip resumes the sequence,
// which may create later commits, so the gpg override still applies).
func skipArgs() []string {
	args := append([]string(nil), gpgSignOff...)
	return append(args, "cherry-pick", "--skip")
}

// CherryPick applies commits (topological order) in ONE sequencer-driven
// invocation (HU-006 AC1). contiguous selects the range form; otherwise the
// explicit ordered SHA list is used. A conflict or an empty pick stops the
// sequence and is returned as DATA in the outcome (not a Go error); the
// reconciled RepoState carries the real conflict list.
func (s *Service) CherryPick(ctx context.Context, dir string, commits []DiscoveredCommit, contiguous bool) (PickOutcome, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return PickOutcome{}, err
	}

	revs := CherryPickRevisions(commits, contiguous)
	if len(revs) == 0 {
		return PickOutcome{}, fmt.Errorf("git: cherry-pick requires at least one commit")
	}

	req := newRequest(root, cherryPickArgs(revs)...)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return PickOutcome{}, fmt.Errorf("git: cherry-pick %v in %s: %w", revs, root, err)
	}
	return s.pickOutcome(ctx, root, result)
}

// ContinueCherryPick runs `git cherry-pick --continue` non-interactively
// (HU-006 AC5), but ONLY after the continue-gate passes (AC3): it re-reads
// the repo state and staged conflict markers, and refuses with
// ErrContinueBlocked (plus the pending detail) while anything is unresolved,
// so `--continue` never opens an editor on a still-conflicted index or
// commits a file with leftover markers.
func (s *Service) ContinueCherryPick(ctx context.Context, dir string) (PickOutcome, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return PickOutcome{}, err
	}

	state, err := s.RepoState(ctx, root)
	if err != nil {
		return PickOutcome{}, err
	}
	markers, err := s.StagedConflictMarkers(ctx, root)
	if err != nil {
		return PickOutcome{}, err
	}

	gate := EvaluateContinueGate(state, markers)
	if !gate.Enabled {
		return PickOutcome{State: state}, fmt.Errorf("%w: %s", ErrContinueBlocked, strings.Join(gate.Pending, "; "))
	}

	return s.runSequencerStep(ctx, root, continueArgs())
}

// runSequencerStep runs a cherry-pick continuation/skip step (args already
// built) and reconciles the outcome.
func (s *Service) runSequencerStep(ctx context.Context, root string, args []string) (PickOutcome, error) {
	req := newRequest(root, args...)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return PickOutcome{}, fmt.Errorf("git: cherry-pick step %v in %s: %w", args, root, err)
	}
	return s.pickOutcome(ctx, root, result)
}

// pickOutcome interprets a cherry-pick command result and reconciles the
// repository state. Exit 0 (fully applied) and exit 1 (conflict or empty
// pick) are both DATA; any other exit is a genuine error.
func (s *Service) pickOutcome(ctx context.Context, root string, result exec.CommandResult) (PickOutcome, error) {
	if result.ExitCode != 0 && result.ExitCode != 1 {
		return PickOutcome{}, fmt.Errorf("git: cherry-pick in %s exited %d: %s", root, result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	state, err := s.RepoState(ctx, root)
	if err != nil {
		return PickOutcome{}, err
	}

	out := PickOutcome{State: state}
	combined := string(result.Stdout) + "\n" + string(result.Stderr)
	if isEmptyPickMessage(combined) {
		out.Empty = true
		out.EmptyMessage = emptyPickExplanation
	}
	out.RerereResolved = rerereResolvedPaths(combined)
	return out, nil
}

// SkipCherryPick runs `git cherry-pick --skip`, used to drop an empty pick
// (content already present) and resume the sequence — the squash safety-net
// (HU-006 AC8). It is non-interactive and keeps the gpgsign override since
// resuming may create later commits.
func (s *Service) SkipCherryPick(ctx context.Context, dir string) (PickOutcome, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return PickOutcome{}, err
	}
	return s.runSequencerStep(ctx, root, skipArgs())
}
