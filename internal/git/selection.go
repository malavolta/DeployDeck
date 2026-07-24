package git

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// CommitSelectionItem is one row of HU-003's commit-selection screen: a
// discovered commit (see DiscoveredCommit) annotated with the user's
// current selection state. Already-applied and merge commits start
// Disabled with an explanatory Reason and are never Selected by default
// (see NewCommitSelectionItems); every other commit starts Selected,
// mirroring DiscoveredCommit.SelectableByDefault. Embedding
// DiscoveredCommit (which itself embeds Commit) promotes SHA/ShortSHA/
// Author/Date/Subject/Merge/Equivalence directly onto the item, which is
// what a rendered row needs (HU-003 AC: "cada fila muestra datos
// suficientes para identificar el cambio").
type CommitSelectionItem struct {
	DiscoveredCommit

	Selected bool
	Disabled bool
	Reason   string

	// MultiTicketNotice is true when Subject mentions a ticket other than
	// the one the user searched for (HU-003: "Avisar cuando un commit
	// mencione otros tickets ademas del buscado").
	MultiTicketNotice bool
	// OtherTickets holds the other ticket-like tokens found in Subject,
	// excluding the searched ticket, in first-seen order.
	OtherTickets []string
}

// ReasonMergeCommit is the blocking reason shown for merge commits: merge
// cherry-pick (`-m`) is not supported in this MVP slice.
const ReasonMergeCommit = "merge commits are not supported for cherry-pick (-m) in MVP"

// alreadyAppliedReason renders a human Reason for an already-applied
// commit, naming which equivalence signal (SHA ancestry, git cherry, or
// patch-id) detected it, so the disabled row is actionable rather than
// generic.
func alreadyAppliedReason(status EquivalenceStatus) string {
	switch status {
	case AlreadyAppliedBySHA:
		return "already applied in target (identical SHA)"
	case EquivalentByCherry:
		return "already applied in target (equivalent content, git cherry)"
	case EquivalentByPatchID:
		return "already applied in target (equivalent content, patch-id)"
	default:
		return ""
	}
}

// ticketMentionPattern matches ticket-like tokens (e.g. "PROJ-1"): a
// letter-leading alphanumeric project key, a hyphen, and a numeric id.
// This mirrors the ticket shape used throughout HU-002/HU-003's fixtures
// and docs (docs/HISTORIAS.md), independent of any per-repo
// config.TicketPatterns regex (which recognizes tickets for SEARCH, a
// different concern from flagging EXTRA mentions in an already-discovered
// commit's own message).
var ticketMentionPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*-\d+`)

// OtherTicketMentions extracts ticket-like tokens from subject, excluding
// searchedTicket itself (case-insensitive), de-duplicated and in
// first-seen order.
func OtherTicketMentions(subject, searchedTicket string) []string {
	var others []string
	seen := make(map[string]bool)

	for _, match := range ticketMentionPattern.FindAllString(subject, -1) {
		if strings.EqualFold(match, searchedTicket) {
			continue
		}
		key := strings.ToUpper(match)
		if seen[key] {
			continue
		}
		seen[key] = true
		others = append(others, match)
	}

	return others
}

// NewCommitSelectionItems builds the HU-003 selection model from a
// discovery result's ordered commits (Service.Discover's
// DiscoverResult.OrderedCommits), preserving their exact order (the topo
// order HU-002 produces). Already-applied commits and merge commits start
// Disabled with an explanatory Reason and Selected=false; every other
// commit starts Selected=true. searchedTicket is the ticket the user
// searched for, used to detect additional-ticket mentions in each
// commit's Subject.
func NewCommitSelectionItems(ordered []DiscoveredCommit, searchedTicket string) []CommitSelectionItem {
	items := make([]CommitSelectionItem, 0, len(ordered))

	for _, c := range ordered {
		item := CommitSelectionItem{DiscoveredCommit: c}

		switch {
		case c.Merge:
			item.Disabled = true
			item.Reason = ReasonMergeCommit
		case c.AlreadyApplied():
			item.Disabled = true
			item.Reason = alreadyAppliedReason(c.Equivalence)
		default:
			item.Selected = true
		}

		item.OtherTickets = OtherTicketMentions(c.Subject, searchedTicket)
		item.MultiTicketNotice = len(item.OtherTickets) > 0

		items = append(items, item)
	}

	return items
}

// ToggleSelection flips Selected for the item at index, returning a new
// slice (items is never mutated in place). Attempting to toggle a Disabled
// item (already-applied or merge commit) is a documented no-op (HU-003 AC:
// "el usuario intenta seleccionarlo... la TUI impide la seleccion"), never
// a silent state change or an error — callers show Reason to explain why.
// An out-of-range index is also a no-op.
func ToggleSelection(items []CommitSelectionItem, index int) []CommitSelectionItem {
	if index < 0 || index >= len(items) {
		return items
	}
	if items[index].Disabled {
		return items
	}

	toggled := append([]CommitSelectionItem(nil), items...)
	toggled[index].Selected = !toggled[index].Selected
	return toggled
}

// ErrEmptySelection means the user confirmed with zero commits selected
// (HU-003 AC: "el usuario confirma sin seleccionar commits, entonces se
// bloquea el avance").
var ErrEmptySelection = errors.New("git: no commits selected")

// ValidateSelection blocks confirming with zero Selected items.
func ValidateSelection(items []CommitSelectionItem) error {
	for _, item := range items {
		if item.Selected {
			return nil
		}
	}
	return ErrEmptySelection
}

// ErrReorderRequiresAdvancedMode means the caller tried to reorder the
// selection outside advanced mode (HU-003: "Permitir reordenar commits
// solo en modo avanzado").
var ErrReorderRequiresAdvancedMode = errors.New("git: reordering commits requires advanced mode")

// ReorderConflictRiskWarning is returned alongside every successful
// reorder: altering topological order increases conflict risk, since each
// commit was written against the state its predecessor left (HU-003 AC).
const ReorderConflictRiskWarning = "reordering selected commits alters topological order and increases conflict risk"

// ReorderSelection moves the item at index from to index to within items,
// returning the reordered slice, ReorderConflictRiskWarning, and a nil
// error — but ONLY when advancedMode is true; otherwise it returns items
// unchanged, an empty warning, and ErrReorderRequiresAdvancedMode.
func ReorderSelection(items []CommitSelectionItem, from, to int, advancedMode bool) ([]CommitSelectionItem, string, error) {
	if !advancedMode {
		return items, "", ErrReorderRequiresAdvancedMode
	}
	if from < 0 || from >= len(items) || to < 0 || to >= len(items) {
		return items, "", fmt.Errorf("git: reorder index out of range: from=%d to=%d len=%d", from, to, len(items))
	}
	if from == to {
		return items, ReorderConflictRiskWarning, nil
	}

	reordered := append([]CommitSelectionItem(nil), items...)
	moved := reordered[from]
	reordered = append(reordered[:from], reordered[from+1:]...)

	rest := append([]CommitSelectionItem{moved}, reordered[to:]...)
	reordered = append(reordered[:to], rest...)

	return reordered, ReorderConflictRiskWarning, nil
}
