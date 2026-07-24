package git_test

import (
	"errors"
	"testing"
	"time"

	"deploydeck/internal/git"
)

// TestNewCommitSelectionItems_TableDriven drives HU-003's selection model:
// each discovered commit becomes a row exposing identifying fields (short
// SHA, message/subject, author, date), plus Disabled/Reason flags for
// already-applied and merge commits, and a multi-ticket notice when the
// commit message mentions tickets beyond the one searched.
func TestNewCommitSelectionItems_TableDriven(t *testing.T) {
	commitDate := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		ordered        []git.DiscoveredCommit
		searchedTicket string
		want           []git.CommitSelectionItem
	}{
		{
			name: "plain selectable commit shows identifying fields and defaults selected",
			ordered: []git.DiscoveredCommit{
				{
					Commit: git.Commit{
						SHA:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						ShortSHA: "aaaaaaa",
						Author:   "Jane Doe",
						Date:     commitDate,
						Subject:  "PROJ-1: add feature",
					},
				},
			},
			searchedTicket: "PROJ-1",
			want: []git.CommitSelectionItem{
				{
					DiscoveredCommit: git.DiscoveredCommit{
						Commit: git.Commit{
							SHA:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
							ShortSHA: "aaaaaaa",
							Author:   "Jane Doe",
							Date:     commitDate,
							Subject:  "PROJ-1: add feature",
						},
					},
					Selected: true,
				},
			},
		},
		{
			name: "already-applied commit is disabled with a reason and not selected",
			ordered: []git.DiscoveredCommit{
				{
					Commit:      git.Commit{SHA: "bbb", ShortSHA: "bbb", Subject: "PROJ-1: dup fix"},
					Equivalence: git.EquivalentByCherry,
				},
			},
			searchedTicket: "PROJ-1",
			want: []git.CommitSelectionItem{
				{
					DiscoveredCommit: git.DiscoveredCommit{
						Commit:      git.Commit{SHA: "bbb", ShortSHA: "bbb", Subject: "PROJ-1: dup fix"},
						Equivalence: git.EquivalentByCherry,
					},
					Selected: false,
					Disabled: true,
					Reason:   "already applied in target (equivalent content, git cherry)",
				},
			},
		},
		{
			name: "merge commit is disabled with a reason and not selected",
			ordered: []git.DiscoveredCommit{
				{
					Commit: git.Commit{SHA: "ccc", ShortSHA: "ccc", Subject: "Merge feature branch"},
					Merge:  true,
				},
			},
			searchedTicket: "PROJ-1",
			want: []git.CommitSelectionItem{
				{
					DiscoveredCommit: git.DiscoveredCommit{
						Commit: git.Commit{SHA: "ccc", ShortSHA: "ccc", Subject: "Merge feature branch"},
						Merge:  true,
					},
					Selected: false,
					Disabled: true,
					Reason:   "merge commits are not supported for cherry-pick (-m) in MVP",
				},
			},
		},
		{
			name: "commit mentioning another ticket sets a multi-ticket notice",
			ordered: []git.DiscoveredCommit{
				{
					Commit: git.Commit{SHA: "ddd", ShortSHA: "ddd", Subject: "PROJ-1 PROJ-2: shared fix"},
				},
			},
			searchedTicket: "PROJ-1",
			want: []git.CommitSelectionItem{
				{
					DiscoveredCommit: git.DiscoveredCommit{
						Commit: git.Commit{SHA: "ddd", ShortSHA: "ddd", Subject: "PROJ-1 PROJ-2: shared fix"},
					},
					Selected:          true,
					MultiTicketNotice: true,
					OtherTickets:      []string{"PROJ-2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.NewCommitSelectionItems(tt.ordered, tt.searchedTicket)

			if len(got) != len(tt.want) {
				t.Fatalf("NewCommitSelectionItems() returned %d items, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i].SHA != tt.want[i].SHA {
					t.Errorf("item[%d].SHA = %q, want %q", i, got[i].SHA, tt.want[i].SHA)
				}
				if got[i].ShortSHA != tt.want[i].ShortSHA {
					t.Errorf("item[%d].ShortSHA = %q, want %q", i, got[i].ShortSHA, tt.want[i].ShortSHA)
				}
				if got[i].Author != tt.want[i].Author {
					t.Errorf("item[%d].Author = %q, want %q", i, got[i].Author, tt.want[i].Author)
				}
				if !got[i].Date.Equal(tt.want[i].Date) {
					t.Errorf("item[%d].Date = %v, want %v", i, got[i].Date, tt.want[i].Date)
				}
				if got[i].Subject != tt.want[i].Subject {
					t.Errorf("item[%d].Subject = %q, want %q", i, got[i].Subject, tt.want[i].Subject)
				}
				if got[i].Selected != tt.want[i].Selected {
					t.Errorf("item[%d].Selected = %v, want %v", i, got[i].Selected, tt.want[i].Selected)
				}
				if got[i].Disabled != tt.want[i].Disabled {
					t.Errorf("item[%d].Disabled = %v, want %v", i, got[i].Disabled, tt.want[i].Disabled)
				}
				if got[i].Reason != tt.want[i].Reason {
					t.Errorf("item[%d].Reason = %q, want %q", i, got[i].Reason, tt.want[i].Reason)
				}
				if got[i].MultiTicketNotice != tt.want[i].MultiTicketNotice {
					t.Errorf("item[%d].MultiTicketNotice = %v, want %v", i, got[i].MultiTicketNotice, tt.want[i].MultiTicketNotice)
				}
				if len(got[i].OtherTickets) != len(tt.want[i].OtherTickets) {
					t.Fatalf("item[%d].OtherTickets = %v, want %v", i, got[i].OtherTickets, tt.want[i].OtherTickets)
				}
				for j := range got[i].OtherTickets {
					if got[i].OtherTickets[j] != tt.want[i].OtherTickets[j] {
						t.Errorf("item[%d].OtherTickets[%d] = %q, want %q", i, j, got[i].OtherTickets[j], tt.want[i].OtherTickets[j])
					}
				}
			}
		})
	}
}

// TestToggleSelection_TableDriven proves selecting a Disabled item (already
// applied or merge commit) is a documented no-op, never a silent state
// flip, while a normal item toggles freely.
func TestToggleSelection_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		items        []git.CommitSelectionItem
		index        int
		wantSelected []bool
	}{
		{
			name: "toggling a selectable item flips Selected",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "a"}}, Selected: true},
			},
			index:        0,
			wantSelected: []bool{false},
		},
		{
			name: "toggling a disabled item is a no-op",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "a"}}, Selected: false, Disabled: true, Reason: "already applied in target (identical SHA)"},
			},
			index:        0,
			wantSelected: []bool{false},
		},
		{
			name: "toggling one item leaves siblings untouched",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "a"}}, Selected: true},
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "b"}}, Selected: true},
			},
			index:        1,
			wantSelected: []bool{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.ToggleSelection(tt.items, tt.index)
			for i, want := range tt.wantSelected {
				if got[i].Selected != want {
					t.Errorf("item[%d].Selected = %v, want %v", i, got[i].Selected, want)
				}
			}
		})
	}
}

// TestValidateSelection_TableDriven proves confirming with zero selected
// commits is blocked (HU-003 AC: "el usuario confirma sin seleccionar
// commits, entonces se bloquea el avance"), while any non-empty selection
// (including one where every OTHER item is disabled) is allowed through.
func TestValidateSelection_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		items   []git.CommitSelectionItem
		wantErr error
	}{
		{
			name: "zero selected items is blocked",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "a"}}, Selected: false},
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "b"}}, Selected: false, Disabled: true, Reason: "merge commits are not supported for cherry-pick (-m) in MVP"},
			},
			wantErr: git.ErrEmptySelection,
		},
		{
			name: "at least one selected item passes",
			items: []git.CommitSelectionItem{
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "a"}}, Selected: true},
				{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "b"}}, Selected: false, Disabled: true},
			},
			wantErr: nil,
		},
		{
			name:    "an empty item list is blocked",
			items:   nil,
			wantErr: git.ErrEmptySelection,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := git.ValidateSelection(tt.items)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateSelection() unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateSelection() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestReorderSelection_TableDriven proves reordering is available ONLY in
// advanced mode and always carries the conflict-risk warning (HU-003 AC:
// "Permitir reordenar commits solo en modo avanzado, mostrando
// advertencia... aumenta el riesgo de conflictos").
func TestReorderSelection_TableDriven(t *testing.T) {
	base := []git.CommitSelectionItem{
		{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "a"}}, Selected: true},
		{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "b"}}, Selected: true},
		{DiscoveredCommit: git.DiscoveredCommit{Commit: git.Commit{SHA: "c"}}, Selected: true},
	}

	t.Run("non-advanced mode rejects reordering", func(t *testing.T) {
		got, warning, err := git.ReorderSelection(base, 0, 2, false)
		if !errors.Is(err, git.ErrReorderRequiresAdvancedMode) {
			t.Fatalf("ReorderSelection() error = %v, want %v", err, git.ErrReorderRequiresAdvancedMode)
		}
		if warning != "" {
			t.Errorf("expected no warning on rejection, got %q", warning)
		}
		if len(got) != len(base) || got[0].SHA != "a" {
			t.Errorf("expected items unchanged on rejection, got %+v", got)
		}
	})

	t.Run("advanced mode reorders and returns the conflict-risk warning", func(t *testing.T) {
		got, warning, err := git.ReorderSelection(base, 0, 2, true)
		if err != nil {
			t.Fatalf("ReorderSelection() unexpected error: %v", err)
		}
		if warning != git.ReorderConflictRiskWarning {
			t.Errorf("warning = %q, want %q", warning, git.ReorderConflictRiskWarning)
		}
		wantOrder := []string{"b", "c", "a"}
		if len(got) != len(wantOrder) {
			t.Fatalf("expected %d items, got %d: %+v", len(wantOrder), len(got), got)
		}
		for i, want := range wantOrder {
			if got[i].SHA != want {
				t.Errorf("got[%d].SHA = %q, want %q", i, got[i].SHA, want)
			}
		}
	})
}
