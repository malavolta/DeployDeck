package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/provenance"
)

// bestResult is the provenance.BestResult call runPRVerify makes, held
// behind a package-private var so tests can inject a coarse (Result, runID)
// outcome deterministically. `go test` never links in the release ldflags
// secret, so the REAL provenance.BestResult (which composes
// provenance.Verify) can only ever return DevMarker/DevVerifier in this
// binary — Verified/Mismatch both require a real, non-empty injected
// secret, which is exhaustively exercised (with a real test secret) by
// internal/provenance's own TestBestResult_AnyOfClassification instead.
// This seam lets pr_verify_test.go exercise runPRVerify's exit-code mapping
// regardless of build-time secret injection, WITHOUT pr.go owning any
// per-marker any-of ranking logic itself (design.md's "Provenance reuse"
// decision, D11: `pr verify` and the deploy-gate signature condition share
// ONE implementation, provenance.BestResult).
var bestResult = provenance.BestResult

// exitError carries an intentional non-zero process exit code from
// runPRVerify's RunE wrapper, letting main()'s post-Execute dispatch
// propagate the EXACT code (0-4) instead of the generic err!=nil -> exit 1
// path, which would collapse every runPRVerify outcome into an
// indistinguishable failure. Error() exists only to satisfy the error
// interface — the user-facing message was already written to
// cmd.OutOrStdout() by runPRVerify itself (newPRVerifyCmd sets
// SilenceErrors so cobra never prints a second, generic "Error: ..." line).
type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("pr verify exited %d", e.code)
}

// newPRCmd builds the `pr` command group for DeployDeck-created pull
// requests (pr-provenance).
func newPRCmd() *cobra.Command {
	prCmd := &cobra.Command{
		Use:          "pr",
		Short:        "Commands for DeployDeck-created pull requests",
		SilenceUsage: true,
	}
	prCmd.AddCommand(newPRVerifyCmd())
	return prCmd
}

// newPRVerifyCmd builds `deploydeck pr verify <url>`, wiring a real
// github.Client over exec.NewOSRunner() (main is the composition root; the
// testable core logic lives in runPRVerify). SilenceErrors keeps cobra from
// printing a second, generic "Error: ..." line — runPRVerify already wrote
// its own descriptive message to stdout.
func newPRVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "verify <url>",
		Short:         "Verifies a DeployDeck provenance marker on a pull request",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			gh := github.New(exec.NewOSRunner())
			code := runPRVerify(cmd.OutOrStdout(), gh, args[0])
			if code != 0 {
				return exitError{code: code}
			}
			return nil
		},
	}
}

// runPRVerify is the testable core of `pr verify`: it derives owner/repo
// from url, fetches the PR's headRefName/body, parses every marker in the
// body, and classifies the BEST outcome (bestResult, the
// provenance.BestResult seam) — writing a human-readable result to w and
// returning the matching exit code
// (design.md's Data Flow "Verify" and "pr verify exit codes" table):
//
//	0 verified · 1 mismatch (forged/copied) · 2 no marker ·
//	3 dev (dev-signed marker OR dev verifier) ·
//	4 degraded (gh/URL), OR an unrecognized/future Result value
func runPRVerify(w io.Writer, gh github.Client, url string) int {
	ownerRepo, ok := github.ParsePRURL(url)
	if !ok {
		fmt.Fprintf(w, "pr verify: could not parse owner/repo from PR URL %q\n", url)
		return 4
	}

	headBranch, body, err := gh.PRDetails(context.Background(), url)
	if err != nil {
		fmt.Fprintf(w, "pr verify: fetching PR details (is `gh` installed and authenticated?): %v\n", err)
		return 4
	}

	markers := provenance.ParseMarkers(body)
	if len(markers) == 0 {
		fmt.Fprintln(w, "pr verify: no DeployDeck provenance marker found in the PR body")
		return 2
	}

	result, runID := bestResult(ownerRepo, headBranch, markers)
	switch result {
	case provenance.Verified:
		fmt.Fprintf(w, "pr verify: verified — this PR was created by DeployDeck (run %s)\n", runID)
		return 0
	case provenance.DevMarker:
		fmt.Fprintf(w, "pr verify: dev-signed marker (run %s) — created by a dev/snapshot build, authenticity cannot be confirmed\n", runID)
		return 3
	case provenance.DevVerifier:
		fmt.Fprintln(w, "pr verify: this binary is a dev build and cannot verify release signatures")
		return 3
	case provenance.Mismatch:
		fmt.Fprintf(w, "pr verify: signature mismatch (run %s) — this marker was not signed for this repo/branch/run (forged or copied)\n", runID)
		return 1
	default:
		// Guards an UNKNOWN future provenance.Result value: never silently
		// treated as a forgery accusation (Mismatch, exit 1) — an
		// unrecognized classification is indeterminate, not evidence of
		// tampering.
		fmt.Fprintf(w, "pr verify: unrecognized verification result for run %s — treating as indeterminate, not a forgery\n", runID)
		return 4
	}
}
