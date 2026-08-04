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

// verifyFn is the provenance.Verify call runPRVerify makes, held behind a
// package-private var so tests can inject each Result classification
// deterministically. `go test` never links in the release ldflags secret,
// so the REAL provenance.Verify can only ever return DevMarker/DevVerifier
// in this binary — Verified/Mismatch both require a real, non-empty
// injected secret, which is exhaustively exercised (with a real test
// secret) by internal/provenance's own tests instead. This seam lets
// pr_verify_test.go exercise runPRVerify's full exit-code mapping
// regardless of build-time secret injection.
var verifyFn = provenance.Verify

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
// body, and classifies the BEST outcome (bestVerifyResult) — writing a
// human-readable result to w and returning the matching exit code
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

	result, runID := bestVerifyResult(ownerRepo, headBranch, markers)
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

// bestVerifyResult classifies every parsed marker via verifyFn and returns
// the highest-ranked outcome (verifyResultRank): Verified beats
// Dev(Marker/Verifier) beats Mismatch. A PR body is untrusted, editable
// text — an AI-generated description can echo a prior marker verbatim, and
// anyone with edit access to the PR can prepend a forged one — so a single
// genuine signature anywhere in the body must always win over noise
// elsewhere in it (any-of classification), never whichever marker happens
// to come first.
func bestVerifyResult(ownerRepo, headBranch string, markers []provenance.Marker) (result provenance.Result, runID string) {
	for i, m := range markers {
		r := verifyFn(ownerRepo, headBranch, m.RunID, m.Sig)
		if i == 0 || verifyResultRank(r) > verifyResultRank(result) {
			result, runID = r, m.RunID
		}
		if r == provenance.Verified {
			break // nothing outranks Verified
		}
	}
	return result, runID
}

// verifyResultRank orders provenance.Result values for bestVerifyResult's
// any-of classification: Verified > Dev(Marker/Verifier) > Mismatch. An
// unrecognized/future Result ranks lowest of all, so it can never silently
// outrank a real classification when multiple markers are present.
func verifyResultRank(r provenance.Result) int {
	switch r {
	case provenance.Verified:
		return 3
	case provenance.DevMarker, provenance.DevVerifier:
		return 2
	case provenance.Mismatch:
		return 1
	default:
		return 0
	}
}
