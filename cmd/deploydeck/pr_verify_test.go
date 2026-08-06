package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/provenance"
)

// prVerifyRunID/prVerifyURL/prVerifyBranch are the fixed identifiers reused
// across this file's table of scenarios.
const (
	prVerifyURL    = "https://github.com/org/repo/pull/42"
	prVerifyBranch = "deploy/PROJ-1-to-UAT"
	prVerifyRunID  = "PROJ-1-to-UAT-20260101000000"
)

// prViewRunner returns a FakeRunner canned for the EXACT `gh pr view <url>
// --json headRefName,body` invocation PRDetails makes, so runPRVerify's
// ParsePRURL->PRDetails->ParseMarkers->Verify pipeline can be driven without
// ever touching a real gh binary.
func prViewRunner(t *testing.T, url, headBranch, body string) *execpkg.FakeRunner {
	t.Helper()
	payload, err := json.Marshal(struct {
		HeadRefName string `json:"headRefName"`
		Body        string `json:"body"`
	}{headBranch, body})
	if err != nil {
		t.Fatalf("marshaling canned PR details: %v", err)
	}
	fr := execpkg.NewFakeRunner()
	fr.When("gh", []string{"pr", "view", url, "--json", "headRefName,body"}, execpkg.CommandResult{ExitCode: 0, Stdout: payload})
	return fr
}

// withBestResult overrides the package-level bestResult seam for the
// duration of the test, restoring it in cleanup. `go test` never links in
// the release ldflags secret, so the REAL provenance.Verify (which
// provenance.BestResult composes) can only ever return
// DevMarker/DevVerifier in this binary — Verified/Mismatch both require a
// real, non-empty injected secret, which is exhaustively exercised (with a
// real test secret) by internal/provenance's own TestBestResult_
// AnyOfClassification instead. This COARSE seam lets runPRVerify's
// exit-code MAPPING be tested deterministically for every classification,
// without runPRVerify (or this test file) ever having to fake per-marker
// signature discrimination itself (task 3.1 restructure — that any-of
// ranking proof now lives entirely in internal/provenance).
func withBestResult(t *testing.T, result provenance.Result, runID string) {
	t.Helper()
	restore := bestResult
	bestResult = func(ownerRepo, headBranch string, markers []provenance.Marker) (provenance.Result, string) {
		return result, runID
	}
	t.Cleanup(func() { bestResult = restore })
}

// --- verified -> 0 -----------------------------------------------------

// TestRunPRVerify_Verified is task 4.1 (RED): a genuine marker (via the
// injected bestResult seam) exits 0 and reports the run.
func TestRunPRVerify_Verified(t *testing.T) {
	body := "Some description.\n\n" + provenance.RenderFooter() + "\n" + provenance.RenderMarker(prVerifyRunID, "0123456789abcdef")
	withBestResult(t, provenance.Verified, prVerifyRunID)

	var buf bytes.Buffer
	code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, prVerifyBranch, body)), prVerifyURL)
	if code != 0 {
		t.Fatalf("runPRVerify() = %d, want 0 (verified)", code)
	}
	if !bytes.Contains(buf.Bytes(), []byte(prVerifyRunID)) {
		t.Errorf("verified output should mention the run, got %q", buf.String())
	}
}

// --- forged/mismatch -> 1 -----------------------------------------------

// TestRunPRVerify_Mismatch is task 4.1 (RED): a tampered/forged signature
// (via the injected bestResult seam) exits 1, never 0.
func TestRunPRVerify_Mismatch(t *testing.T) {
	body := "Some description.\n\n" + provenance.RenderFooter() + "\n" + provenance.RenderMarker(prVerifyRunID, "0123456789abcdef")
	withBestResult(t, provenance.Mismatch, prVerifyRunID)

	var buf bytes.Buffer
	code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, prVerifyBranch, body)), prVerifyURL)
	if code != 1 {
		t.Fatalf("runPRVerify() = %d, want 1 (mismatch)", code)
	}
}

// TestRunPRVerify_CrossRepoBranchCopiedMarker_Mismatch is task 4.1 (RED): a
// marker copied verbatim to a different repo/branch than it was signed for
// is reported as mismatch (exit 1), NOT verified — the same exit-code
// mapping as any other forged/tampered signature.
func TestRunPRVerify_CrossRepoBranchCopiedMarker_Mismatch(t *testing.T) {
	// The marker's runID/sig look plausible; the injected bestResult seam
	// stands in for provenance.BestResult correctly classifying the
	// copied-elsewhere signature as a Mismatch (proven with a real secret
	// in internal/provenance's own TestVerify "wrong repo"/"wrong branch"
	// cases, and its any-of composition in TestBestResult_AnyOfClassification).
	body := provenance.RenderFooter() + "\n" + provenance.RenderMarker(prVerifyRunID, "fedcba9876543210")
	withBestResult(t, provenance.Mismatch, prVerifyRunID)

	var buf bytes.Buffer
	code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, "deploy/OTHER-to-UAT", body)), prVerifyURL)
	if code != 1 {
		t.Fatalf("runPRVerify() = %d, want 1 (mismatch, not verified)", code)
	}
}

// --- no marker -> 2 ------------------------------------------------------

// TestRunPRVerify_NoMarker is task 4.1 (RED): a PR body with no marker at
// all exits 2 (real provenance.ParseMarkers, no seam needed).
func TestRunPRVerify_NoMarker(t *testing.T) {
	body := "Just a plain PR description with no marker."

	var buf bytes.Buffer
	code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, prVerifyBranch, body)), prVerifyURL)
	if code != 2 {
		t.Fatalf("runPRVerify() = %d, want 2 (no marker)", code)
	}
}

// --- dev-signed marker -> 3 -----------------------------------------------

// TestRunPRVerify_DevSignedMarker is task 4.1 (RED): a marker itself
// carrying "sig:dev" exits 3 — this needs NO seam, since real
// provenance.Verify classifies sig=="dev" as DevMarker unconditionally.
func TestRunPRVerify_DevSignedMarker(t *testing.T) {
	body := provenance.RenderFooter() + "\n" + provenance.RenderMarker(prVerifyRunID, "dev")

	var buf bytes.Buffer
	code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, prVerifyBranch, body)), prVerifyURL)
	if code != 3 {
		t.Fatalf("runPRVerify() = %d, want 3 (dev-signed marker)", code)
	}
}

// --- dev-BUILT verifier against a genuine release marker -> 3 -------------

// TestRunPRVerify_DevBuiltVerifier_GenuineMarker is task 4.1 (RED): a marker
// carrying a real-looking (non-"dev") hex signature, verified by a
// dev-built binary (this test binary: `go test` never links the release
// secret, so provenance.secret=="" here), exits 3 — and MUST NEVER be
// reported as invalid/forged (exit 1/2). This needs NO seam: the REAL
// provenance.Verify naturally returns DevVerifier whenever secret=="".
func TestRunPRVerify_DevBuiltVerifier_GenuineMarker(t *testing.T) {
	body := provenance.RenderFooter() + "\n" + provenance.RenderMarker(prVerifyRunID, "0123456789abcdef")

	var buf bytes.Buffer
	code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, prVerifyBranch, body)), prVerifyURL)
	if code != 3 {
		t.Fatalf("runPRVerify() = %d, want 3 (dev-built verifier — never invalid/forged)", code)
	}
}

// --- degraded (gh missing/unauth, PRDetails err, unparseable URL) -> 4 ---

// TestRunPRVerify_Degraded is task 4.1 (RED): every gh/URL degrade path
// exits 4, with a clear message, never a panic.
func TestRunPRVerify_Degraded(t *testing.T) {
	t.Run("gh binary missing", func(t *testing.T) {
		fr := execpkg.NewFakeRunner() // no canned response: Runner error
		var buf bytes.Buffer
		code := runPRVerify(&buf, github.New(fr), prVerifyURL)
		if code != 4 {
			t.Fatalf("runPRVerify() = %d, want 4 (gh missing)", code)
		}
	})

	t.Run("gh pr view non-zero exit (e.g. unauthenticated)", func(t *testing.T) {
		fr := execpkg.NewFakeRunner()
		fr.When("gh", []string{"pr", "view", prVerifyURL, "--json", "headRefName,body"}, execpkg.CommandResult{ExitCode: 1, Stderr: []byte("gh: not logged in")})
		var buf bytes.Buffer
		code := runPRVerify(&buf, github.New(fr), prVerifyURL)
		if code != 4 {
			t.Fatalf("runPRVerify() = %d, want 4 (gh pr view failed)", code)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		fr := execpkg.NewFakeRunner()
		fr.When("gh", []string{"pr", "view", prVerifyURL, "--json", "headRefName,body"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("not json")})
		var buf bytes.Buffer
		code := runPRVerify(&buf, github.New(fr), prVerifyURL)
		if code != 4 {
			t.Fatalf("runPRVerify() = %d, want 4 (malformed JSON)", code)
		}
	})

	t.Run("unparseable URL never calls gh at all", func(t *testing.T) {
		fr := execpkg.NewFakeRunner() // no canned response: any gh call would be a Runner error
		var buf bytes.Buffer
		code := runPRVerify(&buf, github.New(fr), "not-a-pr-url")
		if code != 4 {
			t.Fatalf("runPRVerify() = %d, want 4 (unparseable URL)", code)
		}
		if len(fr.Calls) != 0 {
			t.Fatalf("an unparseable URL must never call gh at all, calls: %v", fr.Calls)
		}
	})
}

// --- any-of classification across multiple markers (restructured, task 3.1) -

// TestRunPRVerify_AnyOfClassification is task 3.1 (RED, RESTRUCTURED — not a
// rename): a PR body can carry more than one marker — a stale one an
// AI-generated description echoed verbatim, or a bad-faith one prepended by
// anyone with edit access to the PR to try to discredit the genuine marker.
// The sig-DISCRIMINATING any-of ranking proof (which marker wins per
// classification) now lives entirely in
// internal/provenance's own TestBestResult_AnyOfClassification, exercised
// with a REAL secret — provenance.BestResult owns that logic now, not
// pr.go. This restructured test proves ONLY pr.go's remaining
// responsibility: runPRVerify parses the FULL body and passes EVERY parsed
// marker through to the bestResult seam (never pre-filtering to "the first
// marker"), and maps whatever Result the seam returns to the documented
// exit code — a coarse plumbing proof, per the design's "pr_verify_test
// keeps only exit-code mapping via the coarse bestResult seam".
func TestRunPRVerify_AnyOfClassification(t *testing.T) {
	genuineMarker := provenance.RenderMarker(prVerifyRunID, "cafebabecafebabe")
	bogusMarker := provenance.RenderMarker("bogus-run", "deadbeefdeadbeef")
	body := bogusMarker + "\n\n" + genuineMarker

	var gotMarkers []provenance.Marker
	restore := bestResult
	bestResult = func(ownerRepo, headBranch string, markers []provenance.Marker) (provenance.Result, string) {
		gotMarkers = markers
		return provenance.Verified, prVerifyRunID
	}
	t.Cleanup(func() { bestResult = restore })

	var buf bytes.Buffer
	code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, prVerifyBranch, body)), prVerifyURL)
	if code != 0 {
		t.Fatalf("runPRVerify() = %d, want 0 (bestResult's Verified must map to exit 0)", code)
	}
	if len(gotMarkers) != 2 {
		t.Fatalf("runPRVerify() must pass ALL parsed markers to bestResult, got %d: %+v", len(gotMarkers), gotMarkers)
	}
	if gotMarkers[0].RunID != "bogus-run" || gotMarkers[1].RunID != prVerifyRunID {
		t.Fatalf("runPRVerify() must preserve document order when passing markers to bestResult, got %+v", gotMarkers)
	}
}

// --- exhaustive Result -> exit-code mapping, incl. an unknown Result -------

// TestRunPRVerify_ExhaustiveResultMapping is task C (RED, remediation):
// every provenance.Result value maps to its documented exit code, including
// an OUT-OF-RANGE/future Result value, which must degrade to exit 4
// (indeterminate) rather than being silently routed through the catch-all
// default as a forgery accusation (exit 1). Against the unmodified switch
// (which routes Mismatch through `default:` and therefore treats ANY
// unrecognized Result the same as Mismatch), the "out-of-range" subtest
// fails: it gets exit 1 instead of 4.
func TestRunPRVerify_ExhaustiveResultMapping(t *testing.T) {
	body := provenance.RenderFooter() + "\n" + provenance.RenderMarker(prVerifyRunID, "0123456789abcdef")

	tests := []struct {
		name   string
		result provenance.Result
		want   int
	}{
		{"Verified", provenance.Verified, 0},
		{"Mismatch", provenance.Mismatch, 1},
		{"DevMarker", provenance.DevMarker, 3},
		{"DevVerifier", provenance.DevVerifier, 3},
		{"out-of-range/unknown Result", provenance.Result(99), 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withBestResult(t, tt.result, prVerifyRunID)
			var buf bytes.Buffer
			code := runPRVerify(&buf, github.New(prViewRunner(t, prVerifyURL, prVerifyBranch, body)), prVerifyURL)
			if code != tt.want {
				t.Fatalf("runPRVerify() with Result(%v) = %d, want %d", tt.result, code, tt.want)
			}
		})
	}
}

// --- cobra wiring ----------------------------------------------------------

// TestNewPRCmd_RegistersVerifySubcommand is task 4.1 (RED): `pr verify` is
// registered on the root command tree.
func TestNewPRCmd_RegistersVerifySubcommand(t *testing.T) {
	root := newRootCmd(Deps{})
	cmd, _, err := root.Find([]string{"pr", "verify"})
	if err != nil {
		t.Fatalf("expected root to register `pr verify`, got: %v", err)
	}
	if cmd.Name() != "verify" {
		t.Fatalf("expected the verify subcommand, got %q", cmd.Name())
	}
}

// TestNewPRVerifyCmd_RequiresExactlyOneArg is task 4.1 (RED): `pr verify`
// rejects zero or multiple URL arguments.
func TestNewPRVerifyCmd_RequiresExactlyOneArg(t *testing.T) {
	for _, args := range [][]string{
		{"pr", "verify"},
		{"pr", "verify", "url-one", "url-two"},
	} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			cmd := newRootCmd(Deps{})
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected an arg-count validation error")
			}
		})
	}
}

// TestNewPRVerifyCmd_Execute_PropagatesExitCodeViaExitError is task 4.1
// (RED): cobra's Execute() surfaces runPRVerify's non-zero code as an
// exitError all the way up through RunE. An unparseable URL is used so the
// REAL github.New(exec.NewOSRunner()) composition inside newPRVerifyCmd is
// instantiated but NEVER actually invoked (ParsePRURL fails first) — this
// exercises the true end-to-end cobra wiring without ever touching a real
// gh binary.
func TestNewPRVerifyCmd_Execute_PropagatesExitCodeViaExitError(t *testing.T) {
	cmd := newRootCmd(Deps{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"pr", "verify", "not-a-pr-url"})

	err := cmd.Execute()
	var ee exitError
	if !errors.As(err, &ee) {
		t.Fatalf("Execute() error = %v (%T), want an exitError", err, err)
	}
	if ee.code != 4 {
		t.Fatalf("exitError.code = %d, want 4 (degraded: unparseable URL)", ee.code)
	}
}
