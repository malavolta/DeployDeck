package git_test

// service_cleanup_e2e_test.go covers HU-017's exec-free git.Service cleanup
// methods (design.md's "Interfaces / Contracts" section) against a REAL
// temporary git repository (newTempRepo/newTempRepoWithRemote from
// helpers_test.go), never a fake runner — these are thin wrappers around
// exact git subprocess invocations, so their correctness lives in real git
// behavior. Skips under -short (inherited from the helpers).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestService_CurrentBranch_ReturnsCheckedOutBranch is task 1.1 (RED):
// CurrentBranch reports the branch checked out via `git rev-parse
// --abbrev-ref HEAD`.
func TestService_CurrentBranch_ReturnsCheckedOutBranch(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "feature/DD-1")

	got, err := svc.CurrentBranch(context.Background(), dir)
	if err != nil {
		t.Fatalf("CurrentBranch: unexpected error: %v", err)
	}
	if got != "feature/DD-1" {
		t.Errorf("CurrentBranch() = %q, want %q", got, "feature/DD-1")
	}
}

// TestService_CurrentBranch_DetachedReturnsHEAD is task 1.1 (RED)'s
// detached-HEAD companion: checking out a bare SHA (not a branch) must
// report the literal string "HEAD", matching `git rev-parse --abbrev-ref
// HEAD`'s own detached-HEAD output — this is what quitCmd's restore guard
// (design.md) treats as a no-op original branch.
func TestService_CurrentBranch_DetachedReturnsHEAD(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	sha := trimNewline(string(runGit(t, runner, dir, "rev-parse", "HEAD").Stdout))
	runGit(t, runner, dir, "checkout", sha)

	got, err := svc.CurrentBranch(context.Background(), dir)
	if err != nil {
		t.Fatalf("CurrentBranch: unexpected error: %v", err)
	}
	if got != "HEAD" {
		t.Errorf("CurrentBranch() = %q, want %q (detached HEAD)", got, "HEAD")
	}
}

// TestService_Checkout_SwitchesToExistingBranch is task 1.3 (RED): Checkout
// runs a PLAIN `git checkout <branch>` (never `-b`) against an existing
// branch, switching HEAD onto it.
func TestService_Checkout_SwitchesToExistingBranch(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	recorder := &callRecordingRunner{inner: runner}
	svc := git.New(recorder)

	runGit(t, runner, dir, "checkout", "-b", "feature/DD-2")
	runGit(t, runner, dir, "checkout", "main")

	if err := svc.Checkout(context.Background(), dir, "feature/DD-2"); err != nil {
		t.Fatalf("Checkout: unexpected error: %v", err)
	}

	got := trimNewline(string(runGit(t, runner, dir, "symbolic-ref", "--short", "HEAD").Stdout))
	if got != "feature/DD-2" {
		t.Errorf("HEAD after Checkout = %q, want %q", got, "feature/DD-2")
	}

	// L-2 (defense-in-depth): the branch operand is separated from options with a
	// trailing `--`, disambiguating it as a ref (never a pathspec) — proven here
	// to STILL switch branches, so the hardening does not change semantics.
	wantCall := "git checkout feature/DD-2 --"
	if !containsCall(recorder.calls, wantCall) {
		t.Errorf("expected the end-of-options checkout form %q, got calls: %v", wantCall, recorder.calls)
	}
}

// TestService_Checkout_UnknownBranchErrors is task 1.3 (RED)'s failure-path
// companion: checking out a branch that does not exist must error, not
// silently create one (that would be `-b`, deliberately not used here).
func TestService_Checkout_UnknownBranchErrors(t *testing.T) {
	dir := newTempRepo(t)
	svc := git.New(exec.NewOSRunner())

	if err := svc.Checkout(context.Background(), dir, "does-not-exist"); err == nil {
		t.Fatal("expected Checkout to error for a branch that does not exist")
	}
}

// TestService_DeleteLocalBranch_RemovesBranch is task 1.5 (RED):
// DeleteLocalBranch runs `git branch -D <branch>` — FORCE delete, a
// deliberate design choice (design.md's "-D force delete" decision): the
// app gates unpushed work + strong-confirm BEFORE calling this, so `-D`
// here must succeed even for a branch with commits unmerged anywhere
// (`-d` would refuse and double-gate). Deletion is verified via
// `rev-parse --verify`, mirroring revParseVerify's own exit-code contract.
func TestService_DeleteLocalBranch_RemovesBranch(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	recorder := &callRecordingRunner{inner: runner}
	svc := git.New(recorder)

	runGit(t, runner, dir, "checkout", "-b", "feature/DD-3")
	writeAndCommit(t, runner, dir, "unmerged.txt", "wip\n", "chore: unmerged work")
	runGit(t, runner, dir, "checkout", "main")

	if err := svc.DeleteLocalBranch(context.Background(), dir, "feature/DD-3"); err != nil {
		t.Fatalf("DeleteLocalBranch: unexpected error: %v", err)
	}

	result, err := runner.Run(context.Background(), exec.CommandRequest{
		Name: "git",
		Args: []string{"rev-parse", "--verify", "--quiet", "refs/heads/feature/DD-3"},
		Dir:  dir,
	})
	if err != nil {
		t.Fatalf("verifying branch deletion: unexpected error: %v", err)
	}
	if result.ExitCode == 0 {
		t.Errorf("expected feature/DD-3 to no longer resolve locally, but it still does")
	}

	// L-2 (defense-in-depth): the branch operand follows the `--` end-of-options
	// separator, so a branch name that begins with `-` can never be misread as a
	// flag — proven here to STILL delete the branch.
	wantCall := "git branch -D -- feature/DD-3"
	if !containsCall(recorder.calls, wantCall) {
		t.Errorf("expected the end-of-options delete form %q, got calls: %v", wantCall, recorder.calls)
	}
}

// TestService_DeleteRemoteBranch_RemovesOriginRef is task 1.7 (RED):
// DeleteRemoteBranch runs `git push origin --delete <branch>` against a
// REAL bare origin remote, and the deleted ref is verified gone via
// `git ls-remote`.
func TestService_DeleteRemoteBranch_RemovesOriginRef(t *testing.T) {
	localDir, remoteDir, _ := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	recorder := &callRecordingRunner{inner: runner}
	svc := git.New(recorder)

	runGit(t, runner, localDir, "checkout", "-b", "deploy/DD-4")
	writeAndCommit(t, runner, localDir, "feature.txt", "feature\n", "chore: add feature")
	runGit(t, runner, localDir, "push", "-u", "origin", "deploy/DD-4")

	if err := svc.DeleteRemoteBranch(context.Background(), localDir, "deploy/DD-4"); err != nil {
		t.Fatalf("DeleteRemoteBranch: unexpected error: %v", err)
	}

	result := runGit(t, runner, localDir, "ls-remote", "--heads", remoteDir, "deploy/DD-4")
	if got := strings.TrimSpace(string(result.Stdout)); got != "" {
		t.Errorf("expected origin/deploy/DD-4 to be deleted, but ls-remote still reports it: %q", got)
	}

	// L-2 (defense-in-depth): the refspec operand follows the `--` end-of-options
	// separator on the destructive remote delete too — proven here to STILL
	// delete the origin ref.
	wantCall := "git push origin --delete -- deploy/DD-4"
	if !containsCall(recorder.calls, wantCall) {
		t.Errorf("expected the end-of-options remote-delete form %q, got calls: %v", wantCall, recorder.calls)
	}
}

// TestService_UnpushedCommitCount_WithUpstream is task 1.9 (RED):
// when origin/<branch> resolves, UnpushedCommitCount counts via
// `git rev-list origin/<b>..<b> --count` — commits reachable from the
// local branch but not from its own remote-tracking ref. The exact
// git invocation is asserted via callRecordingRunner (design.md pins the
// precise rev-list form per case), not just the resulting count.
func TestService_UnpushedCommitCount_WithUpstream(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	recorder := &callRecordingRunner{inner: runner}
	svc := git.New(recorder)

	runGit(t, runner, dir, "checkout", "-b", "deploy/DD-5")
	writeAndCommit(t, runner, dir, "a.txt", "a\n", "chore: a")
	runGit(t, runner, dir, "push", "-u", "origin", "deploy/DD-5")
	writeAndCommit(t, runner, dir, "b.txt", "b\n", "chore: b")
	writeAndCommit(t, runner, dir, "c.txt", "c\n", "chore: c")

	got, err := svc.UnpushedCommitCount(context.Background(), dir, "deploy/DD-5")
	if err != nil {
		t.Fatalf("UnpushedCommitCount: unexpected error: %v", err)
	}
	if got != 2 {
		t.Errorf("UnpushedCommitCount() = %d, want 2", got)
	}

	wantCall := "git rev-list origin/deploy/DD-5..deploy/DD-5 --count"
	if !containsCall(recorder.calls, wantCall) {
		t.Errorf("expected the with-upstream rev-list form %q to run, got calls: %v", wantCall, recorder.calls)
	}
}

// TestService_UnpushedCommitCount_NoUpstream is task 1.9 (RED)'s companion:
// when origin/<branch> does NOT resolve (never pushed), UnpushedCommitCount
// falls back to `git rev-list --count <b> --not --remotes=origin` — every
// commit on the branch not already reachable from ANY known origin ref
// (design.md's exact pinned form).
func TestService_UnpushedCommitCount_NoUpstream(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	recorder := &callRecordingRunner{inner: runner}
	svc := git.New(recorder)

	runGit(t, runner, dir, "checkout", "-b", "deploy/DD-6")
	writeAndCommit(t, runner, dir, "x.txt", "x\n", "chore: x")
	writeAndCommit(t, runner, dir, "y.txt", "y\n", "chore: y")

	got, err := svc.UnpushedCommitCount(context.Background(), dir, "deploy/DD-6")
	if err != nil {
		t.Fatalf("UnpushedCommitCount: unexpected error: %v", err)
	}
	if got != 2 {
		t.Errorf("UnpushedCommitCount() = %d, want 2", got)
	}

	wantCall := "git rev-list --count deploy/DD-6 --not --remotes=origin"
	if !containsCall(recorder.calls, wantCall) {
		t.Errorf("expected the no-upstream rev-list form %q to run, got calls: %v", wantCall, recorder.calls)
	}
}

// containsCall reports whether calls (callRecordingRunner's recorded
// "name args..." strings) contains want exactly.
func containsCall(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

// TestService_IsMergedInto_TrueForAncestor is task 1.11 (RED): a branch
// merged into origin/<target> (via `git merge-base --is-ancestor`) reports
// true. This label is best-effort/advisory ONLY per branch-cleanup's spec
// ("Merged-Vs-Abandoned Is A Best-Effort Label Only") — Group 4 wires it for
// DISPLAY, never as a delete gate; this test only proves the primitive.
func TestService_IsMergedInto_TrueForAncestor(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "deploy/DD-7")
	writeAndCommit(t, runner, dir, "merged.txt", "merged\n", "chore: merged work")
	runGit(t, runner, dir, "checkout", "main")
	runGit(t, runner, dir, "merge", "--no-ff", "deploy/DD-7", "-m", "merge deploy/DD-7")
	runGit(t, runner, dir, "push", "origin", "main")

	got, err := svc.IsMergedInto(context.Background(), dir, "deploy/DD-7", "main")
	if err != nil {
		t.Fatalf("IsMergedInto: unexpected error: %v", err)
	}
	if !got {
		t.Errorf("IsMergedInto() = false, want true (deploy/DD-7 was merged into origin/main)")
	}
}

// TestService_IsMergedInto_FalseForDivergent is task 1.11 (RED)'s negative
// companion: a branch never merged into target's remote reports false, via
// exit-code-as-data mirroring revParseVerify (exit 1 = not-an-ancestor,
// which is DATA, not a Runner error).
func TestService_IsMergedInto_FalseForDivergent(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "deploy/DD-8")
	writeAndCommit(t, runner, dir, "unmerged.txt", "wip\n", "chore: unmerged work")

	got, err := svc.IsMergedInto(context.Background(), dir, "deploy/DD-8", "main")
	if err != nil {
		t.Fatalf("IsMergedInto: unexpected error: %v", err)
	}
	if got {
		t.Errorf("IsMergedInto() = true, want false (deploy/DD-8 was never merged into origin/main)")
	}
}

// TestService_ListDeployBranches_SinglePassWithAgeAndPushStatus is task
// 1.15 (RED): ListDeployBranches lists every deploy/* branch — local-only
// and pushed alike — with age (committer date) and push status, in ONE
// `git for-each-ref` covering both refspecs (design.md's single-pass
// decision). Ages are pinned via explicit GIT_COMMITTER_DATE/GIT_AUTHOR_DATE
// env (an older seeded branch vs. a newer one) rather than real-clock
// sleeps, so the ordering assertion is never flaky.
func TestService_ListDeployBranches_SinglePassWithAgeAndPushStatus(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// A pushed deploy branch, committed at an OLD, fixed date.
	runGit(t, runner, dir, "checkout", "-b", "deploy/DD-9", "main")
	commitAt(t, runner, dir, "old.txt", "old\n", "chore: old work", 1577836800) // 2020-01-01T00:00:00Z
	runGit(t, runner, dir, "push", "-u", "origin", "deploy/DD-9")
	runGit(t, runner, dir, "checkout", "main")

	// A local-only (never pushed) deploy branch, committed at a NEWER date.
	runGit(t, runner, dir, "checkout", "-b", "deploy/DD-10", "main")
	commitAt(t, runner, dir, "new.txt", "new\n", "chore: new work", 1717200000) // 2024-06-01T00:00:00Z
	runGit(t, runner, dir, "checkout", "main")

	got, err := svc.ListDeployBranches(context.Background(), dir)
	if err != nil {
		t.Fatalf("ListDeployBranches: unexpected error: %v", err)
	}

	byName := map[string]git.DeployBranch{}
	for _, b := range got {
		byName[b.Name] = b
	}

	pushed, ok := byName["deploy/DD-9"]
	if !ok {
		t.Fatalf("expected deploy/DD-9 in the result, got: %+v", got)
	}
	if !pushed.Pushed {
		t.Errorf("expected deploy/DD-9.Pushed = true, got false")
	}

	localOnly, ok := byName["deploy/DD-10"]
	if !ok {
		t.Fatalf("expected deploy/DD-10 in the result, got: %+v", got)
	}
	if localOnly.Pushed {
		t.Errorf("expected deploy/DD-10.Pushed = false, got true")
	}

	if !localOnly.LastCommit.After(pushed.LastCommit) {
		t.Errorf("expected deploy/DD-10 (seeded later) LastCommit %v to be after deploy/DD-9's %v", localOnly.LastCommit, pushed.LastCommit)
	}
}

// commitAt writes name/content in dir and commits it with a FIXED
// committer/author date (unixSeconds, UTC — via git's accepted
// "@<unix> +0000" date form), so age-ordering assertions never depend on
// real wall-clock timing between two commits made moments apart.
func commitAt(t *testing.T, runner exec.Runner, dir, name, content, message string, unixSeconds int64) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	runGit(t, runner, dir, "add", name)

	fixedDate := fmt.Sprintf("@%d +0000", unixSeconds)
	env := append([]string{}, nonInteractiveGitEnv...)
	env = append(env, "GIT_COMMITTER_DATE="+fixedDate, "GIT_AUTHOR_DATE="+fixedDate)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "git",
		Args: []string{"commit", "-m", message},
		Dir:  dir,
		Env:  env,
	})
	if err != nil {
		t.Fatalf("git commit failed to run: %v (stderr: %s)", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("git commit exited %d: %s", result.ExitCode, result.Stderr)
	}
}
