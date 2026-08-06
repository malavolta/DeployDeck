# Archive Report — validation-comment-timing

**Status:** archived · **Delivery:** single-PR · **Mode:** strict TDD · **Milestone:** v1.3.1 (patch)

## Summary

Bug fix found by dogfooding v1.3.0 (agenforce-demo PR #7): with the deploy gate enabled, the
DeployDeck validation comment was never posted in the normal flow. Root cause — a trigger-ordering
defect: the comment fired only at validation success (`onReportDone`), which in the standard flow
happens BEFORE the PR exists (the PR is created afterward via the `p` key → `onPrCreated`). At
validation time `resolvePRURL` found no PR, the comment was silently skipped (best-effort), and it
was never retried once the PR existed. So the PR carried the provenance SIGNATURE (body) but never
the validation COMMENT.

Fix: add a second, idempotent trigger in `onPrCreated`'s success path (after `MarkPRCreated`),
reusing the existing `maybeTriggerValidationCommentCmd(m.report.Status)` guard (gate-enabled +
`requireValidationComment` on + status ∈ {Succeeded, SucceededPartial}). The existing
`onReportDone` trigger stays for the reuse/incremental flow (where the PR already exists at
validation). The existing hidden-marker skip-if-present makes the two trigger points post
exactly-once. Production diff: 14 lines in `internal/app/update.go`, no new state, gh method, or
schema change.

## SDD trail

exploration (orchestrator-authored root-cause diagnosis) → propose → spec + design (parallel) →
fresh-context design validation (PASS, 0 CRITICAL) → tasks (14) → apply (14/14, strict TDD) →
verify (PASS) + reliability review (CLEAN) → archive. Artifact store: OpenSpec.

## Why the original reviews missed it

The deploy-gate tests exercised `postValidationCommentCmd` with the PR already resolvable
(`rec.PRUrl` set) and asserted post-when-absent / skip-when-present — but never drove the real
ordering where validation completes before the PR exists. `onPrCreated`/`onReportDone` had no
covering tests at all. A classic assume-the-precondition gap that only surfaced running it for
real. The new `internal/app/validation_comment_timing_test.go` closes it with the load-bearing
validate-then-create-PR ordering test (fails pre-fix: `onPrCreated` returned `(m, nil)`
unconditionally).

## Review

- **sdd-verify — PASS** (0 CRITICAL, 0 WARNING, 1 informational SUGGESTION): fix present exactly
  as designed (14/0 in update.go, `onReportDone` byte-unchanged), all 10 delta-spec scenarios map
  to passing tests, RED authentic, gates green.
- **Reliability — CLEAN** (0 findings): the two triggers are mutually exclusive per PR/branch
  (normal flow: onReportDone finds no PR → no post; onPrCreated posts once. reuse flow: onPrCreated
  not reached). `m.report` is not stale (one run per session, written only at `onReportDone`). A
  comment-post failure cannot corrupt PR creation (`MarkPRCreated` committed synchronously before
  the async cmd; `onPostCommentDone` swallows the error). No regression to `onReportDone` or the
  pre-existing `postValidationCommentCmd` tests.

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l internal cmd` clean.
- `go test ./... -race -count=1` green across all 15 packages (re-run by the orchestrator).
- Exec boundary intact (all gh via the injected fake; `internal/app` never execs).

## Spec merge note

Two MODIFIED requirements merged ADDITIVELY into the living specs (replace the requirement block
with its superset; delta-only `(Previously: …)` note dropped): `deploy-gate`'s "Validation-Comment
Condition And Posting" (+2 scenarios: validate-before-PR posts at creation; exactly-once across
both trigger points) and `validation-progress`'s "Terminal Successful CheckOnly Triggers The
Deploy-Gate Validation Comment" (+1 scenario: PR-creation re-trigger). Diffstat +29/−6; no
pre-existing scenario lost; no delta markers leaked.

## Follow-up (pre-existing, not introduced here)

- `HasValidationComment` matches any marker prefix (job-agnostic), so a stale marker from a prior
  job on the SAME PR would suppress a new job's comment. Unchanged by this fix (the new call site
  posts to freshly-created PRs with no stale marker). A future change could key the dedup on the
  job/run if same-PR re-validation for a different job becomes a real workflow.
