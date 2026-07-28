// service_cleanup.go implements HU-017's exec-free git.Service methods for
// branch & run cleanup: original-branch capture/restore, current/orphan
// deploy branch deletion, unpushed-commit counting, and the best-effort
// merged-into label (design.md's "Interfaces / Contracts" section). Every
// method follows the same shape as the rest of this package: resolve the
// repository root via RepoRoot, build the request via newRequest (carrying
// the non-interactive env by construction), and interpret the exit code —
// either as a hard success/failure or, where git itself uses exit codes as
// data (CurrentBranch's detached HEAD is not that case, but IsMergedInto
// is), per-command.
package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CurrentBranch reports the currently checked-out branch via
// `git rev-parse --abbrev-ref HEAD`. On a detached HEAD (checked out by SHA
// rather than a branch name), git itself reports the literal string "HEAD"
// — this is returned as-is, not an error, so callers (quitCmd's restore
// guard) can treat it as a no-op-restore signal.
func (s *Service) CurrentBranch(ctx context.Context, dir string) (string, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return "", err
	}

	req := newRequest(root, "rev-parse", "--abbrev-ref", "HEAD")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("git: resolving current branch in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("git: resolving current branch in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}

	return strings.TrimSpace(string(result.Stdout)), nil
}

// Checkout runs a PLAIN `git checkout <branch>` (never `-b`) — switching to
// an EXISTING branch. Creating a new branch is CreatePromotionBranch's job
// (HU-005); this method is the restore/navigation primitive quitCmd and the
// branch-cleanup screen use.
func (s *Service) Checkout(ctx context.Context, dir, branch string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}

	req := newRequest(root, "checkout", branch)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: checking out %q in %s: %w", branch, root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: checking out %q in %s: %s", branch, root, strings.TrimSpace(string(result.Stderr)))
	}

	return nil
}

// DeleteLocalBranch runs `git branch -D <branch>` — FORCE delete. This is
// deliberate (design.md's "-D force delete" decision): the app gates
// unpushed work and requires a strong (typed) confirmation BEFORE ever
// calling this, so `-D` here must succeed unconditionally rather than
// re-checking merge status itself — `-d`'s own safety check would double-
// gate and, worse, mis-refuse a squash/rebase-merged branch (new SHA means
// `--is-ancestor` false-negatives there too).
func (s *Service) DeleteLocalBranch(ctx context.Context, dir, branch string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}

	req := newRequest(root, "branch", "-D", branch)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: deleting local branch %q in %s: %w", branch, root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: deleting local branch %q in %s: %s", branch, root, strings.TrimSpace(string(result.Stderr)))
	}

	return nil
}

// DeleteRemoteBranch runs `git push origin --delete <branch>`, removing
// branch's ref on origin. Callers only call this when the branch is known
// pushed (Pushed on DeployBranch / currentPushed on Model) — deleting a ref
// that was never pushed is simply a git error, not a special case here.
func (s *Service) DeleteRemoteBranch(ctx context.Context, dir, branch string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}

	req := newRequest(root, "push", "origin", "--delete", branch)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: deleting origin/%s in %s: %w", branch, root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: deleting origin/%s in %s: %s", branch, root, strings.TrimSpace(string(result.Stderr)))
	}

	return nil
}

// UnpushedCommitCount counts commits on branch not yet reflected on origin,
// selecting between two rev-list forms depending on whether origin/<branch>
// resolves (the SAME revParseVerify primitive BranchExists uses):
//   - origin/<branch> resolves: `git rev-list origin/<b>..<b> --count` —
//     commits ahead of the branch's own remote-tracking ref.
//   - origin/<branch> does NOT resolve (never pushed): `git rev-list
//     --count <b> --not --remotes=origin` — every commit on the branch not
//     already reachable from ANY known origin ref, so a fresh unpushed
//     deploy branch based on an already-pushed target does not count its
//     inherited history as "unpushed".
func (s *Service) UnpushedCommitCount(ctx context.Context, dir, branch string) (int, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return 0, err
	}

	_, hasUpstream, err := s.revParseVerify(ctx, root, "origin/"+branch)
	if err != nil {
		return 0, err
	}

	args := []string{"rev-list", "--count", branch, "--not", "--remotes=origin"}
	if hasUpstream {
		args = []string{"rev-list", "origin/" + branch + ".." + branch, "--count"}
	}

	req := newRequest(root, args...)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("git: counting unpushed commits on %q in %s: %w", branch, root, err)
	}
	if result.ExitCode != 0 {
		return 0, fmt.Errorf("git: counting unpushed commits on %q in %s: %s", branch, root, strings.TrimSpace(string(result.Stderr)))
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(result.Stdout)))
	if err != nil {
		return 0, fmt.Errorf("git: parsing unpushed commit count for %q in %s: %w", branch, root, err)
	}

	return count, nil
}

// IsMergedInto reports whether branch is an ancestor of origin/target via
// `git merge-base --is-ancestor`, mirroring revParseVerify's exit-code-as-
// data contract: exit 0 = ancestor (true), exit 1 = not an ancestor
// (false) — both DATA, never a Runner failure; any other exit is a genuine
// error. This is a BEST-EFFORT "likely merged" label only (branch-cleanup
// spec's "Merged-Vs-Abandoned Is A Best-Effort Label Only" requirement): a
// squash/rebase merge produces a new SHA that this check cannot see, so a
// false negative here MUST NEVER be treated as a delete gate — callers only
// ever use it for display, with the unpushed/strong-confirm gate applying
// unconditionally regardless of this result.
func (s *Service) IsMergedInto(ctx context.Context, dir, branch, target string) (bool, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return false, err
	}

	req := newRequest(root, "merge-base", "--is-ancestor", branch, "origin/"+target)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return false, fmt.Errorf("git: checking whether %q is merged into origin/%s in %s: %w", branch, target, root, err)
	}

	switch result.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git: checking whether %q is merged into origin/%s in %s: %s", branch, target, root, strings.TrimSpace(string(result.Stderr)))
	}
}

// DeployBranch is one deploy/* branch surfaced by ListDeployBranches for
// HU-017's batch cleanup screen: its short name (the "origin/" prefix is
// always stripped — Pushed already carries that signal), the tip's
// committer date, and whether an origin/<name> remote-tracking ref exists.
type DeployBranch struct {
	Name       string
	LastCommit time.Time
	Pushed     bool
}

// deployBranchEntry accumulates the local and/or remote for-each-ref lines
// seen for one short branch name before parseDeployBranches folds them into
// a single DeployBranch.
type deployBranchEntry struct {
	hasLocal   bool
	localDate  time.Time
	hasRemote  bool
	remoteDate time.Time
}

// parseDeployBranches is the pure parse half of ListDeployBranches: it
// turns raw `git for-each-ref
// --format='%(refname:short)|%(committerdate:iso-strict)'` output —
// covering BOTH refs/heads/deploy/* and refs/remotes/origin/deploy/* in one
// pass, per design.md's single-subprocess decision — into deduplicated
// DeployBranches. A branch's "origin/" prefix is always stripped; Pushed is
// set when a remote-tracking entry was seen for that short name (the same
// ref-existence semantics BranchExists/revParseVerify use). LastCommit
// prefers the local head's committer date when a local ref exists (the more
// common/authoritative case — the working branch itself) and falls back to
// the remote-tracking ref's date for a remote-only (already deleted
// locally) branch. Malformed lines (wrong field count, unparseable date)
// are skipped rather than erroring, since this feeds a best-effort cleanup
// listing, not a correctness-critical path — and no git process runs here
// at all, so there is nothing to retry.
func parseDeployBranches(raw []byte) []DeployBranch {
	entries := map[string]*deployBranchEntry{}
	var order []string

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "|", 2)
		if len(fields) != 2 {
			continue
		}

		name := fields[0]
		isRemote := strings.HasPrefix(name, "origin/")
		if isRemote {
			name = strings.TrimPrefix(name, "origin/")
		}

		date, err := time.Parse(time.RFC3339, fields[1])
		if err != nil {
			continue
		}

		e, ok := entries[name]
		if !ok {
			e = &deployBranchEntry{}
			entries[name] = e
			order = append(order, name)
		}
		if isRemote {
			e.hasRemote = true
			e.remoteDate = date
		} else {
			e.hasLocal = true
			e.localDate = date
		}
	}

	var branches []DeployBranch
	for _, name := range order {
		e := entries[name]
		lastCommit := e.remoteDate
		if e.hasLocal {
			lastCommit = e.localDate
		}
		branches = append(branches, DeployBranch{
			Name:       name,
			LastCommit: lastCommit,
			Pushed:     e.hasRemote,
		})
	}

	return branches
}

// ListDeployBranches lists every deploy/* branch (local and origin
// remote-tracking) in ONE `git for-each-ref` subprocess covering both
// refspecs at once (design.md's single-pass decision — avoids an N+1
// per-branch origin resolve), then folds the raw lines into DeployBranches
// via the pure parseDeployBranches.
func (s *Service) ListDeployBranches(ctx context.Context, dir string) ([]DeployBranch, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	req := newRequest(root,
		"for-each-ref",
		"--format=%(refname:short)|%(committerdate:iso-strict)",
		"refs/heads/deploy/*",
		"refs/remotes/origin/deploy/*",
	)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: listing deploy branches in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: listing deploy branches in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}

	return parseDeployBranches(result.Stdout), nil
}
