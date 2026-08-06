# Tasks: Validation-Comment Timing (post the deploy-gate comment once the PR exists)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~260-330 (1 prod line-insertion in `update.go` + 1 new test file with 5-6 test funcs) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk (default; risk is Low, no decision required) |
| Chain strategy | pending (no chaining needed) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | RED tests + GREEN trigger wiring (single insertion point) | PR 1 | `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race` | N/A — `FakeRunner`-backed unit tests only; no real `sf`/`gh` | Revert the single `onPrCreated` trigger line (update.go:1330-1332) and the new test file; `onReportDone` untouched |

## Phase 1: RED — Validation-Comment Trigger Ordering Tests

New file `internal/app/validation_comment_timing_test.go`, mirroring `deploy_gate_test.go`'s `postCommentModel`/`gateCheckModel` idiom and `flow_e2e_test.go`'s `run(t, cmd)` helper.

- [x] 1.1 Add `vcTimingModel(t, fr *execpkg.FakeRunner) Model`: `Deps{Dir:"/repo", Config: config.Config{BranchFormat: config.DefaultBranchFormat, Gates: map[string]config.GateConfig{"UAT": {Enabled: true, RequireValidationComment: ptrBool(true)}}}, GH: github.New(fr)}`; `m.plan.TargetBranch="UAT"`, `m.plan.Ticket="PROJ-1"`, `m.runID="run-1"`, `m.jobID="0Af1"`.
- [x] 1.2 RED `TestOnReportDone_TerminalSuccess_PRAbsent_PostsNothing`: `m.prURL=""`, fr canned `gh pr view <branch> --json url,state` → exit 1 (no PR); drive `reportDoneMsg{report: salesforce.DeployReport{Status:"Succeeded"}}` through `m.onReportDone`; `run(t, cmd)`; assert zero `pr comment` calls in `fr.Calls` (deploy-gate: "before the PR exists" half).
- [x] 1.3 RED `TestOnPrCreated_AfterTerminalSuccess_PostsCommentExactlyOnce` (load-bearing ordering test): continue from 1.2's model (`m.report.Status="Succeeded"`, `m.prURL` still `""`); can `gh pr view <PR-URL> --json comments` → no marker, `gh pr comment` → success; drive `prCreatedMsg{url: deployGatePRURL}` through `m.onPrCreated`; assert the returned `cmd` is non-nil (fails today: production still returns `(m, nil)`); `run(t, cmd)`; assert exactly one `pr comment` call (deploy-gate: "posts at PR creation" + "exactly once across both trigger points").
- [x] 1.4 RED `TestOnPrCreated_MarkerAlreadyPresent_NoDoublePost`: same setup as 1.3 but seed `PRComments` with the existing marker body (`gate.ValidationComment`, per `postCommentModel`'s `jsonQuote` pattern); drive `prCreatedMsg`; assert zero `pr comment` calls (deploy-gate: "no duplicate").
- [x] 1.5 RED `TestOnPrCreated_NoTerminalSuccessThisSession_PostsNothing`: table-driven over `m.report.Status` ∈ `{"", "Failed", "Canceled"}`; drive `prCreatedMsg{url: deployGatePRURL}`; assert `cmd == nil` (validation-progress: PR creation with no prior terminal success triggers nothing).
- [x] 1.6 RED `TestOnReportDone_ReuseFlow_PRAlreadyOpenAtValidation_PostsOnce`: `m.prURL` already set before `onReportDone` (PR open at validation); can `PRComments`/`PostComment`; drive `reportDoneMsg{Succeeded}`; assert exactly one `pr comment` call — pins the unchanged reuse path (validation-progress: successful CheckOnly triggers the comment).
- [x] 1.7 Verify: `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race` — confirm ONLY 1.3 fails; 1.2/1.4/1.5/1.6 already pass (existing guard/behavior is already correct; only the ordering fix is missing).

## Phase 2: GREEN — Wire The Second Trigger

- [x] 2.1 Modify `internal/app/update.go` `onPrCreated` (:1326-1332): after `_ = m.deps.Runs.MarkPRCreated(m.runID, msg.url)` (:1330), insert `if cmd := m.maybeTriggerValidationCommentCmd(m.report.Status); cmd != nil { return m, cmd }` before the existing `return m, nil`.
- [x] 2.2 Verify: `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race` — all Phase 1 tests GREEN, including 1.3.
- [x] 2.3 Verify no regression: `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race -run PostValidationCommentCmd` — existing skip/post pair (`deploy_gate_test.go` 6.17) unchanged GREEN.

## Phase 3: Final Gates

- [x] 3.1 `PATH=/usr/local/go/bin:$PATH go build ./...` — clean, exit 0.
- [x] 3.2 `PATH=/usr/local/go/bin:$PATH go vet ./...` — clean, exit 0.
- [x] 3.3 `gofmt -l internal cmd` — empty output.
- [x] 3.4 `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` — full suite GREEN; confirm all 6 `deploy-gate` and 4 `validation-progress` delta-spec scenarios are covered by Phase 1's tests.
