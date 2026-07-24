package git

import (
	"context"
	"fmt"
)

// DeletedSourceBranchWarning reports whether search results suggest the
// originating source branch has been deleted: message-search found
// commits mentioning the ticket, but branch-name search found no
// candidate branch at all. HU-002: "Avisar cuando la rama origen ya no
// exista y la busqueda dependa solo del grep de mensajes (los commits sin
// ticket en el mensaje seran invisibles)." A ticket with zero matched
// commits is the separate no-results case (NoResultsAlternatives), not
// this one.
func DeletedSourceBranchWarning(matchedCommits []Commit, candidateBranches []Branch) bool {
	return len(matchedCommits) > 0 && len(candidateBranches) == 0
}

// DiscoveredCommit is a Commit annotated with HU-002's discovery flags:
// merge-commit and already-applied/equivalent classification. Neither a
// merge commit nor an already-applied one is selected by default.
type DiscoveredCommit struct {
	Commit
	Merge       bool
	Equivalence EquivalenceStatus
}

// AlreadyApplied reports whether d was classified as already present in
// the target, by SHA or content equivalence.
func (d DiscoveredCommit) AlreadyApplied() bool {
	return d.Equivalence.AlreadyApplied()
}

// SelectableByDefault reports whether d should be pre-selected in HU-003's
// commit-selection screen: neither a merge commit nor already applied.
func (d DiscoveredCommit) SelectableByDefault() bool {
	return !d.Merge && !d.AlreadyApplied()
}

// SquashMergeWarning is a best-effort courtesy heuristic, NOT a reliable
// detector: content equivalence for squash merges is not detectable
// statically (HU-002, DEC-001 in docs/ARQUITECTURA.md) — the real safety
// net is HU-006's empty-pick `--skip` handling. It fires when a ticket's
// classified commits are a MIX of already-applied and still-pending: a
// ticket's commits are normally all-pending or all-promoted together
// under this tool's cherry-pick flow, so a split result suggests some
// commits were absorbed into a squashed/rewritten history that
// individual-commit equivalence (SHA ancestry, git cherry, patch-id) can
// no longer see. A merge commit's own classification never counts toward
// either bucket (it is blocked from selection regardless, not a signal
// here).
func SquashMergeWarning(commits []DiscoveredCommit) bool {
	var sawApplied, sawPending bool
	for _, c := range commits {
		if c.Merge {
			continue
		}
		if c.AlreadyApplied() {
			sawApplied = true
		} else {
			sawPending = true
		}
		if sawApplied && sawPending {
			return true
		}
	}
	return false
}

// Search-diagnostics alternative identifiers offered when a ticket search
// returns no results at all (HU-002: "Manejar tickets sin resultados con
// pantalla accionable").
const (
	AlternativeManualSearch = "manual-search"
	AlternativeChangeTicket = "change-ticket"
	AlternativeSelectBranch = "select-branch"
)

// NoResultsAlternatives returns actionable alternatives for the TUI to
// offer when a ticket search finds neither matched commits nor candidate
// branches; nil when any result exists.
func NoResultsAlternatives(matchedCommits []Commit, candidateBranches []Branch) []string {
	if len(matchedCommits) > 0 || len(candidateBranches) > 0 {
		return nil
	}
	return []string{AlternativeManualSearch, AlternativeChangeTicket, AlternativeSelectBranch}
}

// DiscoverOptions parameterizes a full HU-002 discovery run. Target and
// Source are explicit inputs (never inferred): the exact point in the TUI
// flow at which both become known is an internal/app (Phase 11) concern,
// out of scope here — this package only requires them as plain
// parameters, matching the literal shape of the underlying git commands
// (`origin/<target>..origin/<source>`).
type DiscoverOptions struct {
	// Ticket is required: drives message and branch-name search.
	Ticket string
	// Target is the destination environment branch (e.g. "UAT"). Empty
	// skips range-based ordering/classification (message+branch search
	// only).
	Target string
	// Source is the resolved single source branch (e.g. "feature/PROJ-1"
	// or an env branch per RF-002). Empty skips range-based
	// ordering/classification, same as an empty Target.
	Source string
}

// DiscoverResult is the assembled outcome of a ticket search: message
// matches, candidate branches, and — once Source and Target are both
// resolved — topo-ordered commits classified for merge/equivalence
// (HU-003 consumes OrderedCommits directly), plus diagnostic warnings and
// no-results alternatives.
type DiscoverResult struct {
	Ticket            string
	MatchedCommits    []Commit
	CandidateBranches []Branch
	// OrderedCommits is populated only when Source and Target are both
	// given: topo-ordered commits in origin/<target>..origin/<source>,
	// each classified for merge and equivalence status.
	OrderedCommits []DiscoveredCommit
	// Warnings are best-effort diagnostic notices (deleted source branch,
	// squash-merge courtesy signal).
	Warnings []string
	// Alternatives is non-empty only when the search found nothing at
	// all.
	Alternatives []string
}

// Discover runs a full HU-002 discovery: message search
// (SearchCommits), branch-name search (CandidateBranches), and — when
// opts.Source and opts.Target are both set — topo-ordered commit listing
// (CommitsInRange) with merge flagging (Commit.IsMerge) and
// content-equivalence classification (IsAncestor/Cherry/PatchID +
// ClassifyEquivalence) against origin/<target>. This is the single
// discovery entry point later phases (HU-003 selection, internal/app)
// compose against.
func (s *Service) Discover(ctx context.Context, dir string, opts DiscoverOptions) (DiscoverResult, error) {
	matched, err := s.SearchCommits(ctx, dir, opts.Ticket)
	if err != nil {
		return DiscoverResult{}, err
	}

	candidates, err := s.CandidateBranches(ctx, dir, opts.Ticket)
	if err != nil {
		return DiscoverResult{}, err
	}

	result := DiscoverResult{
		Ticket:            opts.Ticket,
		MatchedCommits:    matched,
		CandidateBranches: candidates,
	}

	if opts.Source != "" && opts.Target != "" {
		ordered, err := s.orderedAndClassified(ctx, dir, opts.Target, opts.Source)
		if err != nil {
			return DiscoverResult{}, err
		}
		result.OrderedCommits = ordered

		if SquashMergeWarning(ordered) {
			result.Warnings = append(result.Warnings, "squash-merge-history")
		}
	}

	if DeletedSourceBranchWarning(matched, candidates) {
		result.Warnings = append(result.Warnings, "deleted-source-branch")
	}
	result.Alternatives = NoResultsAlternatives(matched, candidates)

	return result, nil
}

// orderedAndClassified lists commits in origin/<target>..origin/<source>
// (topo order) and classifies each for merge status and target
// equivalence, via SHA ancestry (IsAncestor) and content equivalence
// (Cherry). `git cherry` already performs a patch-id-based comparison
// against the FULL target range internally, so it is the primary
// content-equivalence signal here (matching the spec scenario: "un commit
// cuyo contenido ya fue cherry-pickeado a destino con otro SHA... se
// marca como equivalente ya aplicado (git cherry)"). Service.PatchID and
// ClassifyEquivalence's patch-id fallback path remain available,
// separately tested (service_equivalence_test.go, equivalence_test.go)
// building blocks for callers that need a targeted single-commit
// comparison; wiring an unbounded target-history patch-id index into this
// default composition is out of proportion for this MVP slice (HU-006's
// empty-pick `--skip` is the documented safety net for whatever cherry
// alone doesn't catch, e.g. genuine squash merges — see
// SquashMergeWarning).
func (s *Service) orderedAndClassified(ctx context.Context, dir, target, source string) ([]DiscoveredCommit, error) {
	commits, err := s.CommitsInRange(ctx, dir, target, source)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, nil
	}

	targetRef := "origin/" + target
	sourceRef := "origin/" + source

	cherryMarkers, err := s.Cherry(ctx, dir, targetRef, sourceRef)
	if err != nil {
		return nil, err
	}

	discovered := make([]DiscoveredCommit, 0, len(commits))
	for _, c := range commits {
		isAncestor, err := s.IsAncestor(ctx, dir, c.SHA, targetRef)
		if err != nil {
			return nil, fmt.Errorf("git: classifying %s against %s: %w", c.SHA, targetRef, err)
		}

		status := ClassifyEquivalence(isAncestor, cherryMarkers[c.SHA], "", nil)

		discovered = append(discovered, DiscoveredCommit{
			Commit:      c,
			Merge:       c.IsMerge(),
			Equivalence: status,
		})
	}

	return discovered, nil
}
