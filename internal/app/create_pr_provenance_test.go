package app

import (
	"strings"
	"testing"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/provenance"
)

// --- 3.1: AI-accepted / non-AI paths gain footer+marker; exactly 1 marker;
// runID wiring --------------------------------------------------------------

// TestCreatePRCmd_Provenance_AIAcceptedPath is task 3.1 (RED): the
// AI-accepted body (effectiveDescription()) is followed by the visible
// footer and the invisible signed marker — push-pr-preparation's
// "AI-accepted body gains footer and marker" scenario. Fails against the
// unmodified createPRCmd, which still submits the bare aiDescription with no
// footer/marker (the FakeRunner has no canned response for the composed
// args, so calledWith reports false).
func TestCreatePRCmd_Provenance_AIAcceptedPath(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	title := "PROJ-1 - Promote changes to UAT"
	originURL := "git@github.com:org/repo.git"
	runID := "PROJ-1-to-UAT-20260101000000"

	ownerRepo, ok := github.OwnerRepo(originURL)
	if !ok {
		t.Fatalf("test setup: OwnerRepo(%q) should parse", originURL)
	}
	// wantBody is computed via the SAME provenance.Compose the production
	// code must call — an exact-match proof that createPRCmd threads
	// effectiveDescription()/ownerRepo/head/runID through unchanged.
	wantBody := provenance.Compose("AI drafted description", ownerRepo, branch, runID)

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, title, wantBody, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/1\n")})

	m := New(Deps{GH: github.New(fr)})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
	m.aiTitle = title
	m.aiDescription = "AI drafted description"
	m.aiAccepted = true
	m.originURL = originURL
	m.runID = runID

	m.createPRCmd()()

	if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", title, "--body", wantBody) {
		t.Fatalf("createPRCmd should compose body via provenance.Compose(effectiveDescription(), ownerRepo, head, runID), calls: %v", fr.Calls)
	}
	if !strings.Contains(wantBody, "AI drafted description") {
		t.Fatalf("composed body should still carry the accepted AI description, got %q", wantBody)
	}
	if !strings.Contains(wantBody, "Created with DeployDeck") {
		t.Fatalf("composed body should carry the visible footer, got %q", wantBody)
	}
	if strings.Count(wantBody, "<!-- deploydeck:") != 1 {
		t.Fatalf("composed body should carry exactly 1 marker, got %q", wantBody)
	}
}

// TestCreatePRCmd_Provenance_NonAIPath is task 3.1 (RED): the non-AI path no
// longer submits an empty body — it submits footer+marker only (push-pr-
// preparation's "Non-AI path submits a footer-and-marker-only body"
// scenario, replacing the historical bare `--body ""` default).
func TestCreatePRCmd_Provenance_NonAIPath(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	title := "PROJ-1 - Promote changes to UAT"
	originURL := "https://github.com/org/repo"
	runID := "PROJ-1-to-UAT-20260101000000"

	ownerRepo, ok := github.OwnerRepo(originURL)
	if !ok {
		t.Fatalf("test setup: OwnerRepo(%q) should parse", originURL)
	}
	wantBody := provenance.Compose("", ownerRepo, branch, runID)

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, title, wantBody, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/2\n")})

	m := New(Deps{GH: github.New(fr)})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
	m.aiAccepted = false
	m.originURL = originURL
	m.runID = runID

	m.createPRCmd()()

	if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", title, "--body", wantBody) {
		t.Fatalf("the non-AI path should submit footer+marker only (never empty), calls: %v", fr.Calls)
	}
	if wantBody == "" {
		t.Fatal("test setup sanity: composed body must not be empty")
	}
	if strings.Count(wantBody, "<!-- deploydeck:") != 1 {
		t.Fatalf("composed body should carry exactly 1 marker, got %q", wantBody)
	}
}

// TestCreatePRCmd_Provenance_RunIDWiring is task 3.1 (RED): createPRCmd
// threads the model's OWN m.runID (not a hardcoded/blank value) into the
// marker — proven by varying runID across two otherwise-identical models and
// asserting each exact literal appears in the composed body sent to
// `gh pr create`.
func TestCreatePRCmd_Provenance_RunIDWiring(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	title := "PROJ-1 - Promote changes to UAT"
	originURL := "git@github.com:org/repo.git"

	for _, runID := range []string{"PROJ-1-to-UAT-20260101000000", "PROJ-1-to-UAT-20260202113000"} {
		t.Run(runID, func(t *testing.T) {
			ownerRepo, _ := github.OwnerRepo(originURL)
			wantBody := provenance.Compose("", ownerRepo, branch, runID)

			fr := ghRunner("authed")
			cannPRCreate(fr, "UAT", branch, title, wantBody, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/3\n")})

			m := New(Deps{GH: github.New(fr)})
			m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
			m.originURL = originURL
			m.runID = runID

			m.createPRCmd()()

			markers := provenance.ParseMarkers(wantBody)
			if len(markers) != 1 || markers[0].RunID != runID {
				t.Fatalf("test setup sanity: expected exactly 1 marker with runID %q, got %+v", runID, markers)
			}
			if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", title, "--body", wantBody) {
				t.Fatalf("createPRCmd should thread m.runID=%q into the marker, calls: %v", runID, fr.Calls)
			}
		})
	}
}

// --- 3.2: unparseable origin degrades to footer-only, no marker ------------

// TestCreatePRCmd_Provenance_UnparseableOrigin_FooterOnly is task 3.2 (RED):
// when m.originURL cannot be parsed into owner/repo, the submitted body ends
// with the visible footer only — NO marker — and PR creation still proceeds
// normally (push-pr-preparation's "Unparseable origin degrades to
// footer-only, never an unbindable marker" scenario).
func TestCreatePRCmd_Provenance_UnparseableOrigin_FooterOnly(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	title := "PROJ-1 - Promote changes to UAT"

	wantBody := provenance.RenderFooter()

	fr := ghRunner("authed")
	cannPRCreate(fr, "UAT", branch, title, wantBody, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("https://github.com/org/repo/pull/4\n")})

	m := New(Deps{GH: github.New(fr)})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}
	m.originURL = "not-a-valid-origin-url" // github.OwnerRepo rejects this form
	m.runID = "PROJ-1-to-UAT-20260101000000"

	m.createPRCmd()()

	if !calledWith(fr, "gh", "pr", "create", "--base", "UAT", "--head", branch, "--title", title, "--body", wantBody) {
		t.Fatalf("an unparseable origin should degrade to a footer-only body and still create the PR normally, calls: %v", fr.Calls)
	}
	if strings.Contains(wantBody, "<!-- deploydeck:") {
		t.Fatalf("an unparseable origin must never write a marker, got body %q", wantBody)
	}
	if prCreateCalls(fr) != 1 {
		t.Fatalf("PR creation should proceed normally despite the unparseable origin, got %d call(s)", prCreateCalls(fr))
	}
}
