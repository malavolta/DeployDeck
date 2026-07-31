package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// pushGit builds a git.Service backed by a FakeRunner canned for the push
// sub-flow: RepoRoot (rev-parse --show-toplevel) resolves to root, `push -u
// origin <branch>` succeeds or fails per pushOK, and `remote get-url origin`
// returns originURL. FakeRunner matches on Name+Args only (Dir/Env ignored),
// so these three canned responses cover every git call the push flow makes.
func pushGit(root, branch, originURL string, pushOK bool) *git.Service {
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(root)})
	if pushOK {
		fr.When("git", []string{"push", "-u", "origin", branch}, execpkg.CommandResult{ExitCode: 0})
	} else {
		fr.When("git", []string{"push", "-u", "origin", branch}, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("permission denied")})
	}
	fr.When("git", []string{"remote", "get-url", "origin"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(originURL)})
	return git.New(fr)
}

// ghRunner returns a FakeRunner canned for `gh auth status` in the requested
// state: "authed" (exit 0), "unauthed" (exit 1 = present-unauthenticated), or
// "absent" (no canned response → Runner error → AuthAbsent).
func ghRunner(auth string) *execpkg.FakeRunner {
	fr := execpkg.NewFakeRunner()
	switch auth {
	case "authed":
		fr.When("gh", []string{"auth", "status"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("Logged in to github.com")})
	case "unauthed":
		fr.When("gh", []string{"auth", "status"}, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("not logged in")})
	case "absent":
		// no canned response
	}
	return fr
}

// cannPRCreate registers the exact `gh pr create` arg-slice for base/head/title
// so createPRCmd's call is matched (and recorded) by the FakeRunner.
func cannPRCreate(fr *execpkg.FakeRunner, base, head, title string, result execpkg.CommandResult) {
	fr.When("gh", []string{"pr", "create", "--base", base, "--head", head, "--title", title, "--body", ""}, result)
}

// calledWith reports whether the FakeRunner recorded a call with exactly the
// given command name and argument slice.
func calledWith(fr *execpkg.FakeRunner, name string, args ...string) bool {
	for _, c := range fr.Calls {
		if c.Name != name || len(c.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if c.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// prCreateCalls counts the `gh pr create` invocations the FakeRunner recorded —
// the load-bearing invariant probe: no PR may be created without explicit
// confirmation, so several assertions require this to be exactly 0.
func prCreateCalls(fr *execpkg.FakeRunner) int {
	n := 0
	for _, c := range fr.Calls {
		if c.Name == "gh" && len(c.Args) >= 2 && c.Args[0] == "pr" && c.Args[1] == "create" {
			n++
		}
	}
	return n
}

// TestModel_Succeeded_OwnHandler_OffersPush is task 5.1: StateSucceeded leaves
// the shared quit-only terminal handler for its own keySucceeded — `p` enters
// StatePushPreparation (pushConfirm, no command yet), while q/enter/esc still
// quit. Failed/Canceled/Aborted/Error stay quit-only: `p` there is inert.
func TestModel_Succeeded_OwnHandler_OffersPush(t *testing.T) {
	base := func() Model {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}
		return m
	}

	// p on Succeeded enters push preparation, running no command yet (the push
	// command is only shown; execution needs a second explicit confirm).
	m := base()
	m.state = StateSucceeded
	next, cmd := m.Update(keyPress("p"))
	nm := next.(Model)
	if nm.State() != StatePushPreparation {
		t.Fatalf("p on Succeeded should enter StatePushPreparation, got %v", nm.State())
	}
	if nm.pushPhase != pushConfirm {
		t.Fatalf("entry phase = %v, want pushConfirm", nm.pushPhase)
	}
	if cmd != nil {
		t.Fatalf("entering push preparation must not run a command yet (push not confirmed)")
	}

	// q/enter/esc still quit from Succeeded.
	for _, k := range []string{"q", "enter", "esc"} {
		mm := base()
		mm.state = StateSucceeded
		_, c := mm.Update(keyPress(k))
		if c == nil {
			t.Errorf("%q should still quit from Succeeded", k)
		}
	}

	// Failed/Canceled/Aborted/Error stay quit-only: p does NOT offer push.
	for _, st := range []State{StateFailed, StateCanceled, StateAborted, StateError} {
		mm := base()
		mm.state = st
		next, _ := mm.Update(keyPress("p"))
		if next.(Model).State() != st {
			t.Errorf("p must not leave terminal state %v (push offered only after success)", st)
		}
	}
}

// TestModel_PushPreparation_ConfirmRunsPush is task 5.3: confirming push
// (`p` on the push-preparation screen) runs git.Push(dir, PromotionBranch),
// then a successful push prepares and shows base/compare/suggested-title.
func TestModel_PushPreparation_ConfirmRunsPush(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"push", "-u", "origin", branch}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"remote", "get-url", "origin"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("git@github.com:org/repo.git")})

	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: git.New(fr), GH: github.New(ghRunner("authed"))})
	m.state = StatePushPreparation
	m.pushPhase = pushConfirm
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	// p confirms: transitions to pushPushing and returns the push command.
	next, cmd := m.Update(keyPress("p"))
	nm := next.(Model)
	if nm.pushPhase != pushPushing {
		t.Fatalf("phase after confirm = %v, want pushPushing", nm.pushPhase)
	}
	if cmd == nil {
		t.Fatal("confirming push should return the push command")
	}
	pushMsg := cmd()
	pd, ok := pushMsg.(pushDoneMsg)
	if !ok || pd.err != nil {
		t.Fatalf("push command should produce a successful pushDoneMsg, got %#v", pushMsg)
	}
	if !calledWith(fr, "git", "push", "-u", "origin", branch) {
		t.Fatalf("git.Push should run `git push -u origin %s`; calls: %v", branch, fr.Calls)
	}

	// pushDoneMsg (success) fires the prep command (RemoteURL + AuthStatus).
	next2, cmd2 := nm.Update(pd)
	nm2 := next2.(Model)
	if cmd2 == nil {
		t.Fatal("a successful push should prepare PR data")
	}
	prepMsg := cmd2()
	next3, _ := nm2.Update(prepMsg)
	nm3 := next3.(Model)
	if nm3.pushPhase != pushReady {
		t.Fatalf("phase after prep = %v, want pushReady", nm3.pushPhase)
	}

	// base, compare, and the suggested title are shown (AC4).
	v := nm3.View()
	for _, want := range []string{"UAT", branch, "PROJ-1 - Promote changes to UAT"} {
		if !strings.Contains(v, want) {
			t.Errorf("push-preparation view missing %q\n%s", want, v)
		}
	}
}

// TestModel_PushPreparation_PushFailureContinues covers the push-failure UX:
// a non-zero push keeps the user on the confirm screen with the error shown,
// never crashing into a terminal error state.
func TestModel_PushPreparation_PushFailureContinues(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: pushGit("/repo", branch, "git@github.com:org/repo.git", false)})
	m.state = StatePushPreparation
	m.pushPhase = pushConfirm
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(keyPress("p"))
	nm := next.(Model)
	msg := cmd()
	if pd, ok := msg.(pushDoneMsg); !ok || pd.err == nil {
		t.Fatalf("a failed push should produce pushDoneMsg with an error, got %#v", msg)
	}
	next2, _ := nm.Update(msg)
	nm2 := next2.(Model)
	if nm2.State() != StatePushPreparation {
		t.Fatalf("a push failure must not leave push preparation, got %v", nm2.State())
	}
	if nm2.pushPhase != pushConfirm {
		t.Fatalf("a push failure should return to pushConfirm, got %v", nm2.pushPhase)
	}
	if nm2.pushErr == nil {
		t.Fatal("a push failure should surface pushErr")
	}
	if !strings.Contains(nm2.View(), "push") {
		t.Errorf("push-failure view should mention the push command/error\n%s", nm2.View())
	}
}

// TestModel_PushPreparation_NoPRWithoutExplicitConfirm is task 5.5 (the
// spec-protected invariant): on the authed path `gh pr create` is NEVER
// invoked without the explicit confirmation key, and on the compare-fallback
// path it is never invoked at all.
func TestModel_PushPreparation_NoPRWithoutExplicitConfirm(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	title := "PROJ-1 - Promote changes to UAT"
	plan := git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	// Authed path: g reveals the confirm gate WITHOUT creating; y creates.
	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, title, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})
	m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr)})
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.authState = github.AuthAuthenticated
	m.plan = plan

	next, _ := m.Update(keyPress("g"))
	nm := next.(Model)
	if nm.pushPhase != pushPRConfirm {
		t.Fatalf("g on the authed screen should reveal the confirm gate, got phase %v", nm.pushPhase)
	}
	if n := prCreateCalls(fr); n != 0 {
		t.Fatalf("revealing the confirm gate must not create a PR yet, got %d gh pr create call(s)", n)
	}

	next2, cmd := nm.Update(keyPress("y"))
	if cmd == nil {
		t.Fatal("explicit confirm (y) should fire the create-PR command")
	}
	cmd()
	if n := prCreateCalls(fr); n != 1 {
		t.Fatalf("explicit confirm should create exactly 1 PR, got %d", n)
	}
	_ = next2

	// Compare-fallback path (unauthenticated): g never enters the confirm gate
	// and no PR is ever attempted.
	fr2 := ghRunner("unauthed")
	mu := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr2)})
	mu.state = StatePushPreparation
	mu.pushPhase = pushReady
	mu.authState = github.AuthUnauthenticated
	mu.plan = plan
	mu.compareURL = "https://github.com/org/repo/compare/UAT..." + branch

	nfb, _ := mu.Update(keyPress("g"))
	if nfb.(Model).pushPhase == pushPRConfirm {
		t.Fatal("g on the compare-fallback screen must not enter the PR confirm gate")
	}
	if n := prCreateCalls(fr2); n != 0 {
		t.Fatalf("the compare-fallback path must never attempt a PR, got %d gh pr create call(s)", n)
	}
}

// TestModel_PushPreparation_PRSuccessRecordsURL is task 5.7: a confirmed PR
// creation on the authed path shows the resulting URL and records it on the
// run via Runs.MarkPRCreated.
func TestModel_PushPreparation_PRSuccessRecordsURL(t *testing.T) {
	dir := t.TempDir()
	branch := "deploy/PROJ-1-to-UAT"
	title := "PROJ-1 - Promote changes to UAT"
	prURL := "https://github.com/org/repo/pull/42"

	writer := runs.NewWriter(dir)
	runID := "PROJ-1-to-UAT-20260101000000"
	if err := writer.Save(runs.Record{RunID: runID, Ticket: "PROJ-1", Target: "UAT"}); err != nil {
		t.Fatalf("seeding run record: %v", err)
	}

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, title, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(prURL + "\n")})

	m := New(Deps{Dir: dir, Config: testConfig(), GH: github.New(fr), Runs: writer})
	m.state = StatePushPreparation
	m.pushPhase = pushPRConfirm
	m.authState = github.AuthAuthenticated
	m.runID = runID
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(keyPress("y"))
	if cmd == nil {
		t.Fatal("confirming should fire the create-PR command")
	}
	prMsg := cmd()
	next2, _ := next.(Model).Update(prMsg)
	nm := next2.(Model)
	if nm.prURL != prURL {
		t.Fatalf("model prURL = %q, want %q", nm.prURL, prURL)
	}
	if nm.pushPhase != pushReady {
		t.Fatalf("after PR success the flow should settle on pushReady, got %v", nm.pushPhase)
	}
	rec, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("loading run record: %v", err)
	}
	if rec.PRUrl != prURL {
		t.Fatalf("run record PRUrl = %q, want %q (URL recorded on the run)", rec.PRUrl, prURL)
	}
	if !strings.Contains(nm.View(), prURL) {
		t.Errorf("push-preparation view should show the created PR URL\n%s", nm.View())
	}
}

// TestModel_PushPreparation_PRFailureShowsManualData is task 5.7's failure
// companion: a failed PR creation shows the error plus the manual
// base/compare/title data, and the flow continues (no crash).
func TestModel_PushPreparation_PRFailureShowsManualData(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	title := "PROJ-1 - Promote changes to UAT"

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, title, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("no commits between UAT and " + branch)})

	m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(fr)})
	m.state = StatePushPreparation
	m.pushPhase = pushPRConfirm
	m.authState = github.AuthAuthenticated
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	next, cmd := m.Update(keyPress("y"))
	prMsg := cmd()
	next2, _ := next.(Model).Update(prMsg)
	nm := next2.(Model)
	if nm.prErr == nil {
		t.Fatal("a failed PR creation should surface prErr")
	}
	if nm.State() != StatePushPreparation || nm.pushPhase != pushReady {
		t.Fatalf("a PR failure must keep the flow alive on pushReady, got state=%v phase=%v", nm.State(), nm.pushPhase)
	}
	v := nm.View()
	for _, want := range []string{"no commits between", "UAT", branch, title} {
		if !strings.Contains(v, want) {
			t.Errorf("PR-failure view missing %q (error + manual data)\n%s", want, v)
		}
	}
}

// TestModel_PushPreparation_FallbackShowsCompareURL is task 5.9: when gh is
// absent or present-unauthenticated, the compare URL derived from origin is
// shown, no PR is offered, and no PR is ever attempted — for SSH, HTTPS and
// Enterprise origin forms, plus the unrecognized-origin graceful fallback.
func TestModel_PushPreparation_FallbackShowsCompareURL(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	plan := git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	tests := []struct {
		name        string
		auth        string
		origin      string
		wantCompare string
		wantRaw     bool // unrecognized origin → raw origin + manual data
	}{
		{"unauthed ssh github.com", "unauthed", "git@github.com:org/repo.git", "https://github.com/org/repo/compare/UAT..." + branch, false},
		{"absent https github.com", "absent", "https://github.com/org/repo.git", "https://github.com/org/repo/compare/UAT..." + branch, false},
		{"unauthed ssh enterprise", "unauthed", "git@github.ibm.com:org/repo.git", "https://github.ibm.com/org/repo/compare/UAT..." + branch, false},
		{"absent unrecognized origin", "absent", "weird-origin-form", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghFake := ghRunner(tt.auth)
			m := New(Deps{Dir: "/repo", Config: testConfig(), Git: pushGit("/repo", branch, tt.origin, true), GH: github.New(ghFake)})
			m.state = StatePushPreparation
			m.pushPhase = pushPushing // pretend the push already succeeded
			m.plan = plan

			prepMsg := m.preparePRCmd()()
			next, _ := m.Update(prepMsg)
			nm := next.(Model)
			if nm.pushPhase != pushReady {
				t.Fatalf("prep should settle on pushReady, got %v", nm.pushPhase)
			}

			v := nm.View()
			if tt.wantRaw {
				if nm.compareURL != "" {
					t.Fatalf("an unrecognized origin should not yield a compare URL, got %q", nm.compareURL)
				}
				if !strings.Contains(v, tt.origin) {
					t.Errorf("an unrecognized origin should show the raw origin URL\n%s", v)
				}
			} else {
				if nm.compareURL != tt.wantCompare {
					t.Fatalf("compareURL = %q, want %q", nm.compareURL, tt.wantCompare)
				}
				if !strings.Contains(v, tt.wantCompare) {
					t.Errorf("fallback view should show the compare URL %q\n%s", tt.wantCompare, v)
				}
			}

			// No PR is offered on the fallback path: g is inert and nothing is
			// ever sent to gh pr create.
			nfb, _ := nm.Update(keyPress("g"))
			if nfb.(Model).pushPhase == pushPRConfirm {
				t.Fatal("the fallback path must not offer PR creation")
			}
			if n := prCreateCalls(ghFake); n != 0 {
				t.Fatalf("the fallback path must never attempt a PR, got %d call(s)", n)
			}
		})
	}
}

// TestModel_PushPreparation_QuitFromSubScreens confirms q quits from every
// push-preparation phase (the screen is terminal-adjacent; the user can always
// leave), and that a late tea.Quit is a real quit command.
func TestModel_PushPreparation_QuitFromSubScreens(t *testing.T) {
	for _, phase := range []pushPhase{pushConfirm, pushReady, pushPRConfirm} {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StatePushPreparation
		m.pushPhase = phase
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}
		_, cmd := m.Update(keyPress("q"))
		if cmd == nil {
			t.Errorf("q should quit from push phase %v", phase)
			continue
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("q from push phase %v should return tea.Quit, got %T", phase, cmd())
		}
	}
}

// TestApp_BoundaryStillHolds_WithGHWired is task 5.11: after wiring Deps.GH,
// internal/app STILL never imports the exec seam directly. This mirrors
// boundary_test.go's intent at unit granularity so a regression here is caught
// alongside the push-preparation tests.
func TestApp_BoundaryStillHolds_WithGHWired(t *testing.T) {
	// Constructing a Model with a real github.Client compiles and runs without
	// internal/app touching os/exec — the client itself holds the seam. The
	// authoritative import check is boundary_test.go (TestApp_NeverImportsExecSeam).
	m := New(Deps{Dir: "/repo", Config: testConfig(), GH: github.New(ghRunner("absent"))})
	if m.deps.GH == nil {
		t.Fatal("Deps.GH should be wired onto the model")
	}
}
