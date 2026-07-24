package git_test

import (
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
