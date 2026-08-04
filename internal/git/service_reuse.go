package git

import (
	"context"
	"fmt"
	"strings"
)

// FFResult is FastForwardBranch's outcome, returned as DATA (never a Go
// error for a routine relationship) so a diverged remote never triggers a
// force-push — the caller (internal/app's reuseBranchCmd) branches on this
// value directly (incremental-promotion spec: "Diverged Remote Deploy
// Branch Blocks With A Clear Error").
type FFResult int

const (
	// FFUpToDate means the local branch and origin/<branch> already point
	// at the same commit — nothing to do.
	FFUpToDate FFResult = iota
	// FFFastForwarded means origin/<branch> was strictly ahead and the
	// local branch was fast-forwarded to match it.
	FFFastForwarded
	// FFLocalAhead means the local branch is strictly ahead of
	// origin/<branch> — a safe no-op, never pushed here.
	FFLocalAhead
	// FFRemoteAbsent means origin/<branch> does not resolve at all.
	FFRemoteAbsent
	// FFDiverged means the local branch and origin/<branch> have each
	// advanced independently (neither is an ancestor of the other) — the
	// highest-value safety case: this is DATA, and the caller must show a
	// clear error rather than ever force-pushing or rewriting either side.
	FFDiverged
)

// FastForwardBranch reconciles the local branch against the already-fetched
// origin/<branch> via an IsAncestor-layered classification (design.md
// "Diverged remote (Q1)" decision) BEFORE ever running `git merge --ff-only`:
// only when origin/<branch> is a strict descendant of the local branch does
// it actually run the merge (git itself then performs a genuine
// fast-forward, never a rewrite). Every other relationship — equal, local
// strictly ahead, or diverged — is resolved from the two IsAncestor checks
// alone, with NO merge/push call at all, so a diverged branch is reported
// purely as data and neither side is ever touched.
//
// Callers must have ALREADY checked out branch (see reuseBranchCmd's
// Checkout -> FastForwardBranch order): `git merge --ff-only` merges into
// whatever is currently checked out.
func (s *Service) FastForwardBranch(ctx context.Context, dir, branch string) (FFResult, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return FFUpToDate, err
	}

	remoteRef := "origin/" + branch
	if _, ok, err := s.revParseVerify(ctx, root, remoteRef); err != nil {
		return FFUpToDate, err
	} else if !ok {
		return FFRemoteAbsent, nil
	}

	localAncestorOfRemote, err := s.IsAncestor(ctx, root, branch, remoteRef)
	if err != nil {
		return FFUpToDate, err
	}
	remoteAncestorOfLocal, err := s.IsAncestor(ctx, root, remoteRef, branch)
	if err != nil {
		return FFUpToDate, err
	}

	switch {
	case localAncestorOfRemote && remoteAncestorOfLocal:
		// Both directions hold only when the two refs point at the exact
		// same commit.
		return FFUpToDate, nil
	case localAncestorOfRemote:
		// origin/<branch> is a strict descendant: a genuine fast-forward.
		req := newRequest(root, "merge", "--ff-only", remoteRef)
		result, err := s.runner.Run(ctx, req)
		if err != nil {
			return FFUpToDate, fmt.Errorf("git: merge --ff-only %s in %s: %w", remoteRef, root, err)
		}
		if result.ExitCode != 0 {
			return FFUpToDate, fmt.Errorf("git: merge --ff-only %s in %s: %s", remoteRef, root, strings.TrimSpace(string(result.Stderr)))
		}
		return FFFastForwarded, nil
	case remoteAncestorOfLocal:
		// The local branch is strictly ahead: a safe no-op, never pushed
		// here — Push (unchanged) is the caller's own later, explicit step.
		return FFLocalAhead, nil
	default:
		// Neither is an ancestor of the other: genuine divergence. NO merge
		// or push call runs — this is the load-bearing safety property.
		return FFDiverged, nil
	}
}

// FilterNotOnBranch classifies each of commits as already present on
// deployBranch or not, via the layered ordered check (incremental-promotion
// spec: "Layered Already-On-Branch Detection" — ancestry, then the -x
// provenance trailer (PRIMARY), then cherry-equivalence as a fallback),
// returning only the not-yet-present remainder in their original order.
// `CherryPick` itself is untouched: filtering happens here, before any pick
// invocation.
//
// The trailer is the primary re-application signal because it is exact and
// survives conflict-resolution edits that break patch-id equivalence (a
// commit resolved differently from its source no longer matches by patch-id,
// so cherry-equivalence alone would wrongly re-pick it and trigger a spurious
// re-conflict). Known, accepted limitation: the trailer records that a `-x`
// cherry-pick happened, not that its content still survives on the CURRENT
// tip — a promoted commit reverted/rewritten IN PLACE on the deploy branch
// stays classified as present and is not re-applied on reuse; the escape
// hatch is delete & recreate.
//
// The trailer log (`git log <deployBranch>`) and the cherry-equivalence
// classification (`git cherry <deployBranch> <sourceRef>`) are each fetched
// ONCE for the whole batch, not once per commit. sourceRef == "" degrades
// gracefully: the Cherry fallback is skipped entirely (no error) — the
// ancestor and trailer layers alone still run.
func (s *Service) FilterNotOnBranch(ctx context.Context, dir, deployBranch, sourceRef string, commits []DiscoveredCommit) ([]DiscoveredCommit, error) {
	if len(commits) == 0 {
		return nil, nil
	}

	trailers, err := s.cherryPickTrailersOnBranch(ctx, dir, deployBranch)
	if err != nil {
		return nil, err
	}

	var cherryMarkers map[string]byte
	if sourceRef != "" {
		cherryMarkers, err = s.Cherry(ctx, dir, deployBranch, sourceRef)
		if err != nil {
			return nil, err
		}
	}

	var remaining []DiscoveredCommit
	for _, c := range commits {
		isAncestor, err := s.IsAncestor(ctx, dir, c.SHA, deployBranch)
		if err != nil {
			return nil, err
		}
		if isAncestor {
			continue
		}
		if trailers[c.SHA] {
			continue
		}
		if cherryMarkers[c.SHA] == '-' {
			continue
		}
		remaining = append(remaining, c)
	}
	return remaining, nil
}

// cherryPickTrailersOnBranch runs `git log <branch>` once and parses every
// "(cherry picked from commit <sha>)" provenance trailer it carries via the
// pure ParseCherryPickTrailers — the sourceRef-unavailable FALLBACK signal
// FilterNotOnBranch's trailer layer classifies against (git-cherry is
// authoritative for content-presence whenever sourceRef != "").
func (s *Service) cherryPickTrailersOnBranch(ctx context.Context, dir, branch string) (map[string]bool, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	req := newRequest(root, "log", branch)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: log %s in %s: %w", branch, root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: log %s in %s: %s", branch, root, strings.TrimSpace(string(result.Stderr)))
	}

	return ParseCherryPickTrailers(result.Stdout), nil
}
