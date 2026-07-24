package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_DependencyWarnings_Integration_RealDiff seeds a real temp
// repo where an EARLIER unselected commit and a LATER selected commit both
// touch the same file, and proves Service.DependencyWarnings (composing
// real `git diff --name-only` per commit) flags the pair — HU-003:
// "Calcular, por fichero tocado por la seleccion, los commits intermedios
// no seleccionados... que tocan el mismo fichero, y mostrar warning de
// dependencia".
func TestService_DependencyWarnings_Integration_RealDiff(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	runGit(t, runner, dir, "push", "origin", "UAT")
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-1", "UAT")

	// earlierSHA: unselected, touches shared.txt.
	earlierSHA := writeAndCommit(t, runner, dir, "shared.txt", "v1\n", "PROJ-2: interleaved change to shared file")
	// laterSHA: selected, ALSO touches shared.txt.
	laterSHA := writeAndCommit(t, runner, dir, "shared.txt", "v2\n", "PROJ-1: update shared file")
	// untouchedSHA: selected, touches a different file entirely — must
	// never produce a warning.
	untouchedSHA := writeAndCommit(t, runner, dir, "other.txt", "other\n", "PROJ-1: unrelated change")

	runGit(t, runner, dir, "push", "origin", "feature/PROJ-1")

	ctx := context.Background()
	result, err := svc.Discover(ctx, dir, git.DiscoverOptions{Ticket: "PROJ-1", Target: "UAT", Source: "feature/PROJ-1"})
	if err != nil {
		t.Fatalf("Discover: unexpected error: %v", err)
	}
	if len(result.OrderedCommits) != 3 {
		t.Fatalf("expected 3 ordered commits, got %d: %+v", len(result.OrderedCommits), result.OrderedCommits)
	}

	selected := map[string]bool{
		laterSHA:     true,
		untouchedSHA: true,
		// earlierSHA intentionally left unselected.
	}

	warnings, err := svc.DependencyWarnings(ctx, dir, result.OrderedCommits, selected)
	if err != nil {
		t.Fatalf("DependencyWarnings: unexpected error: %v", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 dependency warning, got %d: %+v", len(warnings), warnings)
	}
	got := warnings[0]
	if got.File != "shared.txt" {
		t.Errorf("warning.File = %q, want %q", got.File, "shared.txt")
	}
	if got.SelectedSHA != laterSHA {
		t.Errorf("warning.SelectedSHA = %q, want %q", got.SelectedSHA, laterSHA)
	}
	if got.UnselectedSHA != earlierSHA {
		t.Errorf("warning.UnselectedSHA = %q, want %q", got.UnselectedSHA, earlierSHA)
	}
}

// TestComputeDependencyWarnings_TableDriven exercises the pure decision
// logic directly against precomputed per-commit touched-file data, no real
// git involved.
func TestComputeDependencyWarnings_TableDriven(t *testing.T) {
	ordered := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: "m1"}},              // earlier, touches shared.txt, unselected
		{Commit: git.Commit{SHA: "m2"}},              // later, touches shared.txt, selected
		{Commit: git.Commit{SHA: "m3"}},              // later, touches other.txt only, selected
		{Commit: git.Commit{SHA: "m4"}, Merge: true}, // merge commit, never contributes files
	}

	tests := []struct {
		name       string
		selected   map[string]bool
		filesBySHA map[string][]string
		want       []git.DependencyWarning
	}{
		{
			name:     "selected commit shares a file with an earlier unselected commit",
			selected: map[string]bool{"m2": true, "m3": true},
			filesBySHA: map[string][]string{
				"m1": {"shared.txt"},
				"m2": {"shared.txt"},
				"m3": {"other.txt"},
			},
			want: []git.DependencyWarning{
				{File: "shared.txt", SelectedSHA: "m2", UnselectedSHA: "m1"},
			},
		},
		{
			name:     "no overlap produces no warnings",
			selected: map[string]bool{"m2": true, "m3": true},
			filesBySHA: map[string][]string{
				"m1": {"only-m1.txt"},
				"m2": {"only-m2.txt"},
				"m3": {"other.txt"},
			},
			want: nil,
		},
		{
			name:     "an unselected LATER commit sharing a file is not flagged (only earlier dependencies matter)",
			selected: map[string]bool{"m1": true},
			filesBySHA: map[string][]string{
				"m1": {"shared.txt"},
				"m2": {"shared.txt"},
			},
			want: nil,
		},
		{
			name:     "when the overlapping commit is ALSO selected, no warning fires",
			selected: map[string]bool{"m1": true, "m2": true},
			filesBySHA: map[string][]string{
				"m1": {"shared.txt"},
				"m2": {"shared.txt"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.ComputeDependencyWarnings(ordered, tt.selected, tt.filesBySHA)
			if len(got) != len(tt.want) {
				t.Fatalf("ComputeDependencyWarnings() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("warning[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
