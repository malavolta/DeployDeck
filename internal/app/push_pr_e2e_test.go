package app

import (
	"os"
	osexec "os/exec"
	"strings"
	"testing"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// gitOut runs a git command in dir for test assertions, returning trimmed
// stdout and failing the test on error. Test code may exec freely; only the
// internal/app PRODUCTION code is forbidden from doing so (boundary_test.go).
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := osexec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s in %s: %v", strings.Join(args, " "), dir, err)
	}
	return strings.TrimSpace(string(out))
}

// remoteHasBranch reports whether origin (the bare local remote) advertises
// refs/heads/<branch> — how the real `git push -u` round-trip is verified.
func remoteHasBranch(t *testing.T, local, branch string) bool {
	t.Helper()
	return strings.Contains(gitOut(t, local, "ls-remote", "--heads", "origin", branch), "refs/heads/"+branch)
}

// seedPromotionBranch creates a promotion branch off origin/UAT carrying one
// new commit, so the real push has a genuine ref to publish to the bare remote.
func seedPromotionBranch(t *testing.T, local, branch string) {
	t.Helper()
	gitRun(t, local, "checkout", "-b", branch, "origin/UAT")
	writeFile(t, local, "promoted.cls", "promoted content\n")
	gitRun(t, local, "add", ".")
	gitRun(t, local, "commit", "-m", "PROJ-1 promoted change")
}

// TestHU014_PushPR_E2E_Consolidated is HU-014's consolidated end-to-end test
// (tasks 7.1-7.3): it drives a SUCCESSFUL validation terminal → p → real
// `git push -u` (round-tripping to a bare local remote) → base/compare/title,
// then exercises the three gh states through a FakeRunner (never a real gh):
//
//	(a) authenticated + explicit confirm → gh pr create runs, URL recorded on
//	    the run (MarkPRCreated); no PR without confirmation;
//	(b) unauthenticated and (c) absent → the compare URL is shown, derived from
//	    origin in BOTH SSH and HTTPS forms (Enterprise-host safe), flow continues;
//	    a PR-creation failure shows the error + manual data.
//
// REAL `gh pr create` is NEVER run — the github.Client is FakeRunner-backed, so
// no PR is ever created on a real host.
func TestHU014_PushPR_E2E_Consolidated(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}

	local := setupFlowRepo(t)
	branch := "deploy/PROJ-1-to-UAT"
	seedPromotionBranch(t, local, branch)

	realGit := git.New(execpkg.NewOSRunner())
	plan := git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
	title := "PROJ-1 - Promote changes to UAT"

	// ---- (a) authed: real push round-trip + confirmed PR + URL recorded ----
	writer := runs.NewWriter(local)
	runID := "PROJ-1-to-UAT-e2e"
	if err := writer.Save(runs.Record{RunID: runID, Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX"}); err != nil {
		t.Fatalf("seeding run record: %v", err)
	}
	prURL := "https://github.com/org/repo/pull/7"
	ghFake := ghRunner("authed")
	cannPRCreate(ghFake, "UAT", branch, title, "", execpkg.CommandResult{ExitCode: 0, Stdout: []byte(prURL + "\n")})

	m := New(Deps{Dir: local, Config: flowConfig(), Git: realGit, GH: github.New(ghFake), Runs: writer})
	m.state = StateSucceeded
	m.plan = plan
	m.runID = runID
	m.jobID = "0AfE2ETEST"

	// The success screen offers push (AC1).
	if !strings.Contains(m.View(), "preparar push") {
		t.Fatalf("success screen should offer push\n%s", m.View())
	}

	// p enters push preparation without running anything (command shown first).
	m = advance(t, m, keyPress("p"))
	if m.State() != StatePushPreparation || m.pushPhase != pushConfirm {
		t.Fatalf("p should enter push preparation (pushConfirm), got state=%v phase=%v", m.State(), m.pushPhase)
	}

	// p confirms → REAL git push -u origin <branch> against the bare remote.
	next, cmd := m.Update(keyPress("p"))
	m = next.(Model)
	pushMsg := run(t, cmd)
	if e := pushMsg.(pushDoneMsg).err; e != nil {
		t.Fatalf("real `git push -u` failed: %v", e)
	}
	if !remoteHasBranch(t, local, branch) {
		t.Fatalf("push should round-trip: origin missing refs/heads/%s", branch)
	}
	if remote := gitOut(t, local, "config", "--get", "branch."+branch+".remote"); remote != "origin" {
		t.Fatalf("push -u should set upstream tracking to origin, got %q", remote)
	}

	// push done → prep (RemoteURL + gh auth status) → pushReady, authed.
	next, cmd = m.Update(pushMsg)
	m = next.(Model)
	prepMsg := run(t, cmd)
	next, _ = m.Update(prepMsg)
	m = next.(Model)
	if m.pushPhase != pushReady {
		t.Fatalf("prep should settle on pushReady, got %v", m.pushPhase)
	}
	if m.authState != github.AuthAuthenticated {
		t.Fatalf("gh should be detected authenticated, got %v", m.authState)
	}
	v := m.View()
	for _, want := range []string{"UAT", branch, title} {
		if !strings.Contains(v, want) {
			t.Errorf("push-ready view missing %q (base/compare/title)\n%s", want, v)
		}
	}

	// g reveals the confirm gate WITHOUT creating a PR (invariant).
	next, _ = m.Update(keyPress("g"))
	m = next.(Model)
	if m.pushPhase != pushPRConfirm {
		t.Fatalf("g should reveal the PR confirm gate, got %v", m.pushPhase)
	}
	if n := prCreateCalls(ghFake); n != 0 {
		t.Fatalf("no PR may be created before explicit confirmation, got %d call(s)", n)
	}

	// y is the explicit confirmation → gh pr create runs, URL shown + recorded.
	next, cmd = m.Update(keyPress("y"))
	m = next.(Model)
	prMsg := run(t, cmd)
	next, _ = m.Update(prMsg)
	m = next.(Model)
	if n := prCreateCalls(ghFake); n != 1 {
		t.Fatalf("explicit confirm should run gh pr create exactly once, got %d", n)
	}
	if m.prURL != prURL {
		t.Fatalf("model prURL = %q, want %q", m.prURL, prURL)
	}
	if !strings.Contains(m.View(), prURL) {
		t.Errorf("view should show the created PR URL\n%s", m.View())
	}
	rec, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("loading run record: %v", err)
	}
	if rec.PRUrl != prURL {
		t.Fatalf("run record PRUrl = %q, want %q (URL recorded on the run)", rec.PRUrl, prURL)
	}

	// ---- (b)/(c) fallback: compare URL from origin in SSH & HTTPS forms ----
	fallbacks := []struct {
		name        string
		auth        string
		origin      string
		wantCompare string
	}{
		{"unauthenticated SSH github.com", "unauthed", "git@github.com:org/repo.git", "https://github.com/org/repo/compare/UAT..." + branch},
		{"absent HTTPS github.com", "absent", "https://github.com/org/repo.git", "https://github.com/org/repo/compare/UAT..." + branch},
		{"unauthenticated SSH Enterprise", "unauthed", "git@github.ibm.com:org/repo.git", "https://github.ibm.com/org/repo/compare/UAT..." + branch},
	}
	for _, fb := range fallbacks {
		t.Run(fb.name, func(t *testing.T) {
			// Point origin at a GitHub-style URL (Part (a) already pushed to the
			// bare remote; the fallback derives the compare link locally via
			// `git remote get-url`, no network).
			gitRun(t, local, "remote", "set-url", "origin", fb.origin)

			ghFB := ghRunner(fb.auth)
			mm := New(Deps{Dir: local, Config: flowConfig(), Git: realGit, GH: github.New(ghFB)})
			mm.state = StatePushPreparation
			mm.pushPhase = pushPushing // push already succeeded
			mm.plan = plan

			prepMsg := run(t, mm.preparePRCmd())
			next, _ := mm.Update(prepMsg)
			mm = next.(Model)
			if mm.pushPhase != pushReady {
				t.Fatalf("prep should settle on pushReady, got %v", mm.pushPhase)
			}
			if mm.compareURL != fb.wantCompare {
				t.Fatalf("compareURL = %q, want %q", mm.compareURL, fb.wantCompare)
			}
			if !strings.Contains(mm.View(), fb.wantCompare) {
				t.Errorf("fallback view should show the compare URL %q\n%s", fb.wantCompare, mm.View())
			}
			// No PR offered and none attempted on the fallback path.
			nfb, _ := mm.Update(keyPress("g"))
			if nfb.(Model).pushPhase == pushPRConfirm {
				t.Fatal("the compare-fallback path must not offer PR creation")
			}
			if n := prCreateCalls(ghFB); n != 0 {
				t.Fatalf("the fallback path must never attempt a PR, got %d call(s)", n)
			}
		})
	}

	// ---- PR-creation failure → error + manual data, flow continues ----
	t.Run("authenticated PR creation failure shows manual data", func(t *testing.T) {
		ghFail := ghRunner("authed")
		cannPRCreate(ghFail, "UAT", branch, title, "", execpkg.CommandResult{ExitCode: 1, Stderr: []byte("pull request create failed: no commits between UAT and " + branch)})

		mc := New(Deps{Dir: local, Config: flowConfig(), Git: realGit, GH: github.New(ghFail)})
		mc.state = StatePushPreparation
		mc.pushPhase = pushReady
		mc.authState = github.AuthAuthenticated
		mc.plan = plan

		next, _ := mc.Update(keyPress("g"))
		mc = next.(Model)
		next, cmd := mc.Update(keyPress("y"))
		mc = next.(Model)
		prMsg := run(t, cmd)
		next, _ = mc.Update(prMsg)
		mc = next.(Model)

		if mc.prErr == nil {
			t.Fatal("a failed PR creation should surface prErr")
		}
		if mc.State() != StatePushPreparation || mc.pushPhase != pushReady {
			t.Fatalf("a PR failure must keep the flow alive on pushReady, got state=%v phase=%v", mc.State(), mc.pushPhase)
		}
		v := mc.View()
		for _, want := range []string{"no commits between", "UAT", branch, title} {
			if !strings.Contains(v, want) {
				t.Errorf("PR-failure view missing %q (error + manual data)\n%s", want, v)
			}
		}
	})
}

// TestHU014_PushPR_E2E_FailedValidationNeverOffersPush is the AC2 variant: a
// FAILED (or Canceled) validation terminal never offers push as a primary
// action — p is inert and the compare/push affordances are absent.
func TestHU014_PushPR_E2E_FailedValidationNeverOffersPush(t *testing.T) {
	for _, st := range []State{StateFailed, StateCanceled} {
		m := New(Deps{Dir: "/repo", Config: flowConfig()})
		m.state = st
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}

		if strings.Contains(m.View(), "preparar push") {
			t.Errorf("state %v must not offer push as a primary action\n%s", st, m.View())
		}
		next, _ := m.Update(keyPress("p"))
		if next.(Model).State() != st {
			t.Errorf("p must be inert on terminal state %v", st)
		}
	}
}
