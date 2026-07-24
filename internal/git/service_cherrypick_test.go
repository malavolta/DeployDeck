package git_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_CherryPick_AppliesSelectedCommitsInOrder_SingleInvocation pins
// AC1 (tasks 10.7/10.8): a contiguous selection applies in topological order
// via ONE sequencer-driven invocation (NOT a Go loop of single-sha picks),
// and the promoted content matches the source branch.
func TestService_CherryPick_AppliesSelectedCommitsInOrder_SingleInvocation(t *testing.T) {
	dir := newTempRepo(t)
	realRunner := exec.NewOSRunner()
	feature1, feature2 := seedCleanFeature(t, realRunner, dir)

	// Spy on the real runner to prove exactly ONE cherry-pick process runs.
	spy := &callRecordingRunner{inner: realRunner}
	svc := git.New(spy)

	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: feature1}},
		{Commit: git.Commit{SHA: feature2}},
	}

	outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
	if err != nil {
		t.Fatalf("CherryPick returned error: %v", err)
	}
	if outcome.State.InProgress {
		t.Fatalf("expected a clean multi-commit pick to complete, got InProgress=true: %+v", outcome.State)
	}

	// Both files landed on the target branch, in order.
	log := string(runGit(t, realRunner, dir, "log", "--format=%s", "-2").Stdout)
	if !strings.Contains(log, "PROJ-1: add B") || !strings.Contains(log, "PROJ-1: add A") {
		t.Errorf("expected both feature commits applied, got log:\n%s", log)
	}

	// Content equals the source branch for the touched files.
	diff := runGitAllow(t, realRunner, dir, "diff", "HEAD", "feature", "--", "a.cls", "b.cls")
	if strings.TrimSpace(string(diff.Stdout)) != "" {
		t.Errorf("expected promoted content == source for touched files, got diff:\n%s", diff.Stdout)
	}

	// Exactly one cherry-pick invocation (sequencer-driven, not a loop).
	pickCalls := 0
	var pickArgs string
	for _, c := range spy.calls {
		if strings.Contains(c, "cherry-pick") {
			pickCalls++
			pickArgs = c
		}
	}
	if pickCalls != 1 {
		t.Fatalf("expected exactly ONE cherry-pick invocation (single sequencer-driven pick), got %d: %v", pickCalls, spy.calls)
	}
	// Contiguous selection uses the range form <first>^..<last>.
	if !strings.Contains(pickArgs, feature1+"^.."+feature2) {
		t.Errorf("expected contiguous range form %s^..%s, got %q", feature1, feature2, pickArgs)
	}
}

// TestCherryPickRevisions_TableDriven pins the range-vs-explicit-list form
// selection (task 10.8): contiguous -> range, non-contiguous -> ordered SHA
// list, single -> bare SHA.
func TestCherryPickRevisions_TableDriven(t *testing.T) {
	commits := func(shas ...string) []git.DiscoveredCommit {
		out := make([]git.DiscoveredCommit, len(shas))
		for i, s := range shas {
			out[i] = git.DiscoveredCommit{Commit: git.Commit{SHA: s}}
		}
		return out
	}
	tests := []struct {
		name       string
		commits    []git.DiscoveredCommit
		contiguous bool
		want       []string
	}{
		{name: "contiguous multi -> range", commits: commits("aaa", "bbb", "ccc"), contiguous: true, want: []string{"aaa^..ccc"}},
		{name: "single contiguous -> bare sha", commits: commits("aaa"), contiguous: true, want: []string{"aaa"}},
		{name: "non-contiguous -> explicit ordered list", commits: commits("aaa", "ccc"), contiguous: false, want: []string{"aaa", "ccc"}},
		{name: "empty -> nil", commits: nil, contiguous: true, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.CherryPickRevisions(tt.commits, tt.contiguous)
			if len(got) != len(tt.want) {
				t.Fatalf("CherryPickRevisions() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("CherryPickRevisions()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestService_CherryPick_And_Continue_CarryGpgSignFalse pins M2 (tasks
// 10.9/10.10): the tool's own cherry-pick and --continue invocations carry
// `-c commit.gpgsign=false` so a no-tty gpg invocation can never hang the run.
func TestService_CherryPick_And_Continue_CarryGpgSignFalse(t *testing.T) {
	cap := &capturingRunner{result: exec.CommandResult{ExitCode: 0}}
	svc := git.New(cap)

	commits := []git.DiscoveredCommit{{Commit: git.Commit{SHA: "aaa"}}, {Commit: git.Commit{SHA: "bbb"}}}
	if _, err := svc.CherryPick(context.Background(), "/repo", commits, true); err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}

	pick, ok := cap.callWithArg("cherry-pick")
	if !ok {
		t.Fatalf("no cherry-pick request was issued; calls: %+v", cap.calls)
	}
	if !argsContain(pick.Args, "-c") || !argsContain(pick.Args, "commit.gpgsign=false") {
		t.Errorf("cherry-pick Args missing `-c commit.gpgsign=false`: %v", pick.Args)
	}
	// The -c flag must precede the cherry-pick subcommand.
	if idxOf(pick.Args, "commit.gpgsign=false") > idxOf(pick.Args, "cherry-pick") {
		t.Errorf("`-c commit.gpgsign=false` must come BEFORE the cherry-pick subcommand: %v", pick.Args)
	}

	cap.calls = nil
	if _, err := svc.ContinueCherryPick(context.Background(), "/repo"); err != nil {
		t.Fatalf("ContinueCherryPick error: %v", err)
	}
	cont, ok := cap.callWithArg("--continue")
	if !ok {
		t.Fatalf("no --continue request was issued; calls: %+v", cap.calls)
	}
	if !argsContain(cont.Args, "commit.gpgsign=false") {
		t.Errorf("cherry-pick --continue Args missing `-c commit.gpgsign=false`: %v", cont.Args)
	}
}

// TestIsContiguousSelection_TableDriven pins the contiguity predicate the
// caller uses to choose the range vs explicit-list form (task 10.8).
func TestIsContiguousSelection_TableDriven(t *testing.T) {
	full := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: "c1"}},
		{Commit: git.Commit{SHA: "c2"}},
		{Commit: git.Commit{SHA: "c3"}},
		{Commit: git.Commit{SHA: "c4"}},
	}
	pick := func(shas ...string) []git.DiscoveredCommit {
		out := make([]git.DiscoveredCommit, len(shas))
		for i, s := range shas {
			out[i] = git.DiscoveredCommit{Commit: git.Commit{SHA: s}}
		}
		return out
	}
	tests := []struct {
		name     string
		selected []git.DiscoveredCommit
		want     bool
	}{
		{name: "consecutive run is contiguous", selected: pick("c2", "c3"), want: true},
		{name: "whole set is contiguous", selected: pick("c1", "c2", "c3", "c4"), want: true},
		{name: "single is contiguous", selected: pick("c3"), want: true},
		{name: "gap breaks contiguity", selected: pick("c1", "c3"), want: false},
		{name: "empty is not contiguous", selected: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.IsContiguousSelection(full, tt.selected); got != tt.want {
				t.Errorf("IsContiguousSelection(%v) = %v, want %v", tt.selected, got, tt.want)
			}
		})
	}
}

func idxOf(args []string, token string) int {
	for i, a := range args {
		if a == token {
			return i
		}
	}
	return -1
}
