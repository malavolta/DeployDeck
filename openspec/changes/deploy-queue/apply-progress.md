# Apply Progress: Deploy Queue Visibility and Self-Service Cancel (HU-009 + HU-012)

**Change**: `deploy-queue`
**Batches**: 1 (HU-009: Phases 1, 2, 7.1) — DONE, committed, green under `-race`;
2 (HU-012: Phases 3-6, 7.2 + Phase 8 final verification) — DONE;
3 (Remediation: MEDIUM liveness bug in the HU-012 cancel flow) — DONE (this batch).
**Mode**: Strict TDD (RED → GREEN per task)
**Branch**: `deploy-queue`
**Delivery**: single-pr, `size:exception` granted (whole change; no PRs/splits)

## Scope Recap

- Batch 1 delivered HU-009 (deploy queue visibility) end to end.
- Batch 2 (this batch) delivered HU-012 (self-service cancel): `internal/salesforce.CancelDeploy`,
  `internal/runs.Writer.MarkCanceled`, the `internal/app` typed cancel-confirm flow, the
  threat-matrix arg-slice proof, the best-effort real-org cancel e2e, and Phase 8 final
  verification spanning BOTH HUs.

## Completed Tasks (cumulative)

### Phase 1: HU-009 — `internal/salesforce` Queue (Foundation) — Batch 1
- [x] 1.1 RED `internal/salesforce/queue_test.go`: FakeRunner canned multi-user incl. own, permission-error, generic-error, empty, CheckOnly cases (HU-009)[U]
- [x] 1.2 GREEN `internal/salesforce/queue.go` + `client.go`: `ListDeployQueue` added to `Client` interface, SOQL builder (fields incl. `CreatedBy.Username`), envelope decode `{TotalSize,Done,Records[]}`, `ErrQueuePermission` heuristic (HU-009)[U]

### Phase 2: HU-009 — `internal/app` QueueReview Real Stop (Integration) — Batch 1
- [x] 2.1 RED `internal/app/queue_review_test.go` (HU-009)[T]
- [x] 2.2 GREEN `internal/app/app.go`: `queue`, `queueErr`, `identity` Model fields (HU-009)[U]
- [x] 2.3 GREEN `internal/app/commands.go`: `queueCmd`, `queueDoneMsg`, `queueCallTimeout` (HU-009)[U]
- [x] 2.4 GREEN `internal/app/update.go`: `onQueueDone` (HU-009)[U]
- [x] 2.5 GREEN `internal/app/keys.go`: `confirmPackageReview`→`StateQueueReview`+`queueCmd`; `keyQueueReview`; dispatcher case (HU-009)[U]
- [x] 2.6 GREEN `internal/app/view.go`: `viewQueueReview` (HU-009)[T]

### Phase 3: HU-012 — `internal/salesforce` CancelDeploy (Foundation) — Batch 2
- [x] 3.1 RED `internal/salesforce/cancel_test.go`: arg-slice, success/failure + Raw, unparseable-stderr, runner-error cases (HU-012)[U]
- [x] 3.2 GREEN `internal/salesforce/cancel.go` + `client.go`: `CancelDeploy` added to `Client` interface + impl on `*client`, `CancelResult{Raw}`, error handling mirrors `ValidateDeploy` (HU-012)[U]

### Phase 4: HU-012 — `internal/runs` MarkCanceled (Foundation) — Batch 2
- [x] 4.1 RED `internal/runs/writer_test.go`: `MarkCanceled` writes `cancel.json` + sets `run.json` Status=Canceled + advances UpdatedAt; does NOT consume `report-<NNN>.json` numbering; unknown-runID errors (HU-012)[U]
- [x] 4.2 GREEN `internal/runs/writer.go`: `Writer.MarkCanceled(runID, cancelRaw)` (HU-012)[U]

### Phase 5: HU-012 — `internal/app` Cancel Wiring (Integration) — Batch 2
- [x] 5.1 RED `internal/app/cancel_confirm_test.go`: `c`→CancelConfirm; typed `CANCELAR` gate (wrong/lowercase/backspace/esc); own-job-only target; success→Canceled+persist; failure→stay+unmarked; stale-message guards (both cancel + late-report); view (HU-012)[T]
- [x] 5.2 GREEN `internal/app/app.go`: `StateCancelConfirm` const + `cancelInput`/`cancelErr` fields (HU-012)[U]
- [x] 5.3 GREEN `internal/app/commands.go`: `cancelCmd` (targets exactly `m.jobID`, never `m.cancelInput`), `cancelDoneMsg` (HU-012)[U]
- [x] 5.4 GREEN `internal/app/update.go`: `onCancelDone` (success→`cancelPoll()`+`MarkCanceled`+`StateCanceled`; failure→stay+error; guard `state != StateCancelConfirm`); wired into `Update` (HU-012)[U]
- [x] 5.5 GREEN `internal/app/keys.go`: `c` case in `keyValidationPolling`; `keyCancelConfirm` (`keyTicket` idiom, exact literal `CANCELAR`); dispatcher case; `q` path untouched (HU-012)[U]
- [x] 5.6 GREEN `internal/app/view.go`: `viewCancelConfirm` + dispatcher case (HU-012)[T]

### Phase 6: Threat-Matrix Proof (Security) — Batch 2
- [x] 6.1 `internal/salesforce/argcomposition_test.go`: SOQL is one discrete `--query` arg for `ListDeployQueue`; jobID is one discrete `--job-id` arg for `CancelDeploy` — proven even for an adversarial metacharacter-laden jobId; never shell-joined/interpolated [U]

### Phase 7: Real E2E (Verification)
- [x] 7.1 `[E2E-ORG]` queue query vs `AM-DEV-EDITION` — `internal/salesforce/real_org_queue_e2e_test.go` (HU-009) — Batch 1
- [x] 7.2 `[E2E-ORG]` best-effort real cancel — `internal/salesforce/real_org_cancel_e2e_test.go`, double-gated (`DEPLOYDECK_E2E_ORG` + `DEPLOYDECK_E2E_CANCEL_JOBID`), asserts the CLI executes WITHOUT asserting the timing-hard terminal status; skips by default (HU-012) — Batch 2

### Phase 8: Final Verification (Cleanup) — Batch 2
- [x] 8.1 `go test -race ./...`, `go vet ./...`, `gofmt -l .` — all clean
- [x] 8.2 `internal/app/boundary_test.go` `TestApp_NeverImportsExecSeam` — PASS (app still reaches `sf` only through the injected `salesforce.Client`)
- [x] 8.3 `proposal.md` Success Criteria — all 5 checked off

## Files Changed (Batch 2)

| File | Action | What Was Done |
|------|--------|----------------|
| `internal/salesforce/client.go` | Modified | Added `CancelDeploy(ctx, jobID, targetOrg) (CancelResult, error)` to the `Client` interface. |
| `internal/salesforce/cancel.go` | Created | `CancelDeploy` impl + `CancelResult{Raw}`; `sf project deploy cancel --job-id <id> --target-org <alias> --json` (discrete args); Raw preserved on success/failure; error handling mirrors `ValidateDeploy`. |
| `internal/salesforce/cancel_test.go` | Created | Arg-slice composition, success-preserves-Raw, CLI-error decoded-message+Raw, unparseable-stderr fallback, runner-error-no-panic. |
| `internal/salesforce/argcomposition_test.go` | Created | Threat-matrix "PR / argument composition" proof for BOTH `ListDeployQueue` (SOQL) and `CancelDeploy` (jobID, incl. adversarial value). |
| `internal/salesforce/real_org_cancel_e2e_test.go` | Created | Opt-in, double-gated, best-effort real-org cancel e2e (skips by default). |
| `internal/runs/writer.go` | Modified | Added `Writer.MarkCanceled(runID, cancelRaw)` → `cancel.json` companion + `run.json` Status=Canceled + UpdatedAt (NOT `report-<NNN>.json`). |
| `internal/runs/writer_test.go` | Modified | `MarkCanceled` writes `cancel.json`+Canceled status; does-not-consume report numbering; unknown-runID errors. |
| `internal/app/app.go` | Modified | `StateCancelConfirm` const (doc'd) + `cancelInput`/`cancelErr` Model fields. |
| `internal/app/commands.go` | Modified | `cancelDoneMsg` + `cancelCmd()` (targets `m.jobID` only). |
| `internal/app/update.go` | Modified | `onCancelDone` (+ `state != StateCancelConfirm` stale guard); wired into `Update`. |
| `internal/app/keys.go` | Modified | `c` case in `keyValidationPolling` → `StateCancelConfirm`; `cancelConfirmWord` const; `keyCancelConfirm`; dispatcher case; `q` untouched. |
| `internal/app/view.go` | Modified | `viewCancelConfirm` + dispatcher case. |
| `internal/app/cancel_confirm_test.go` | Created | Full `Model.Update`-driven cancel-flow coverage (12 tests). |
| `openspec/changes/deploy-queue/{proposal,tasks,apply-progress}.md` | Modified | Success Criteria checked; Phase 3-8 tasks marked `[x]`; this merged progress. |

## TDD Cycle Evidence (Batch 2)

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 3.1/3.2 `CancelDeploy` | `cancel_test.go` first; `go vet ./internal/salesforce/` → `client.CancelDeploy undefined` (compile-time RED) | `client.go` interface + `cancel.go` impl; `go test ./internal/salesforce/ -run Cancel` → 5/5 PASS | None — impl mirrored `ValidateDeploy` verbatim; `gofmt` clean |
| 4.1/4.2 `MarkCanceled` | `writer_test.go` new cases first; `go vet ./internal/runs/` → `w.MarkCanceled undefined` (compile-time RED) | `writer.go` `MarkCanceled`; `go test ./internal/runs/ -run MarkCanceled` → 3/3 PASS | None |
| 5.1-5.6 cancel wiring | `cancel_confirm_test.go` first; `go vet ./internal/app/` → `undefined: StateCancelConfirm` (compile-time RED) | `app.go`/`commands.go`/`update.go`/`keys.go`/`view.go` implemented together (one cohesive state-machine change — const, fields, cmd, msg, handler, keys, view all needed to compile); `go test ./internal/app/ -run 'Cancel\|CancelConfirm'` → 12/12 PASS, full app suite PASS | None; boundary test re-verified PASS |
| 6.1 threat-matrix | `argcomposition_test.go` asserts existing GREEN; first run FAILED on a wrong hardcoded arg-count (7 vs actual 8) — a genuine RED in the assertion | Corrected count to 8; `go test -run ThreatMatrix` → 2/2 PASS | None |
| 7.2 cancel e2e | N/A — opt-in e2e, not a red/green unit | Skips cleanly when env unset (`-run E2ERealOrg_Cancel` → SKIP) | N/A |

## Work Unit Evidence

### Unit 2 — HU-012 self-service cancel (this batch, complete)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/salesforce/... ./internal/runs/... ./internal/app/... -run 'Cancel\|MarkCanceled\|ThreatMatrix'` → all PASS (salesforce Cancel 5 + ThreatMatrix 2, runs MarkCanceled 3, app cancel-confirm 12) |
| Runtime harness command/scenario and exact result | FakeRunner is primary/required (design: cancel real-org e2e is timing-hard). Best-effort real e2e: `DEPLOYDECK_E2E_ORG=<a> DEPLOYDECK_E2E_CANCEL_JOBID=<j> go test ./internal/salesforce/ -run E2ERealOrg_Cancel` (opt-in, skips by default; asserts the CLI executes, not terminal status). Not run in this batch (no live cancelable job); SKIP verified. |
| Rollback boundary | Revert `internal/salesforce/cancel.go`, `cancel_test.go`, `argcomposition_test.go`, `real_org_cancel_e2e_test.go`, the `CancelDeploy` interface add in `client.go`; `internal/runs/writer.go` `MarkCanceled` + its tests; in `internal/app`, revert the `StateCancelConfirm` const, `cancelInput`/`cancelErr` fields, `cancelCmd`/`cancelDoneMsg`, `onCancelDone` + its `Update` case, the `c` case + `keyCancelConfirm` + `cancelConfirmWord` + dispatcher case in `keys.go`, and `viewCancelConfirm` + dispatcher case in `view.go`, plus `cancel_confirm_test.go`. This restores `keyValidationPolling` to `r`/`q` only and leaves the HU-009 queue path and the `q` exit invariant untouched. |

## Full Suite Verification (Batch 2 — verbatim, as requested)

```
export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./...
```

Result: build OK; `go vet ./...` clean; `gofmt -l .` produced no output (clean);
`go test -race ./...` → all packages `ok` (`cmd/deploydeck`, `internal/app`,
`internal/config`, `internal/delta`, `internal/exec`, `internal/git`,
`internal/prereq`, `internal/runs`, `internal/salesforce`).
`internal/app/boundary_test.go` `TestApp_NeverImportsExecSeam` re-verified: PASS.

## Deviations From Design

None in the implementation shape — `CancelDeploy`/`CancelResult`,
`MarkCanceled` (+ `cancel.json`), `StateCancelConfirm`, `cancelCmd`/
`cancelDoneMsg`/`onCancelDone`/`keyCancelConfirm`/`viewCancelConfirm`, the
symmetric `state != StateCancelConfirm` stale guard, and the reuse of
`cancelPoll()` + `StateCanceled` all match design.md's Interfaces/Contracts,
Data Flow, and Architecture Decisions.

Minor, intentional additions beyond the literal task text (all design-consistent):
- Added a `cancelErr` Model field (alongside `cancelInput`) so the failed-cancel
  error renders on the confirm screen without abusing `notice`.
- `keyCancelConfirm`'s `esc` re-arms the poll loop (`pollTickCmd`) so live
  progress resumes after backing out of the modal, rather than leaving polling
  silently paused. The `pollInFlight`/deadline guards keep this safe (no stacked
  reports).
- Task 7.2's real cancel e2e is gated behind a SECOND env var
  (`DEPLOYDECK_E2E_CANCEL_JOBID`) so it can never accidentally cancel an
  unintended job — stricter than the task's single-gate suggestion, matching the
  "best-effort, do not let it block" guidance.

## Issues Found

None. The only RED-beyond-compile was a self-inflicted wrong arg-count literal
(7 vs 8) in the threat-matrix test, corrected immediately.

## Workload / PR Boundary

- Mode: single PR, `size:exception` granted for the whole `deploy-queue` change.
- Current work unit: Unit 2 — HU-012 self-service cancel (complete).
- Boundary: this batch starts from HU-009 complete (batch 1) and ends with HU-012
  fully implemented, tested (unit + threat-matrix + opt-in e2e), and Phase 8 final
  verification green across BOTH HUs. 5 focused conventional commits.
- Estimated review budget impact: HU-012 adds ~700 changed lines (impl + tests)
  within the granted whole-change exception.

## Commits (Batch 2)

1. `a8c3f1f` — `feat(salesforce): cancel deploy via sf project deploy cancel`
2. `6d6ccef` — `feat(runs): mark run canceled with cancel.json companion`
3. `652d5e3` — `feat(app): typed cancel-confirm flow`
4. `3ed1de7` — `test(salesforce): threat-matrix arg-slice proof + best-effort cancel e2e`
5. (docs) — Success Criteria + tasks `[x]` + merged apply-progress (this commit)

## Batch 3 — Remediation: MEDIUM liveness bug in the HU-012 cancel flow

**Found by**: adversarial review (change otherwise archive-ready). **Severity**: MEDIUM
(broken liveness + broken esc/`r` promise; no data loss — the SF job stays resumable).

### The bug

`onReportDone` (`internal/app/update.go`) hit its stale-message state guard
`if m.state != StateValidationPolling { return m, nil }` and returned BEFORE the
line that clears `m.pollInFlight = false`. In the normal case an in-flight report
lands while the user is on `StateCancelConfirm` (deciding whether to type
CANCELAR); the guard dropped that report but left `pollInFlight` stuck `true`.
Pressing `esc` back to `StateValidationPolling` then fired `pollTickCmd`, but
`onPollTick` bails on `pollInFlight == true` → no report fired, no tick
rescheduled → the live-progress loop was PERMANENTLY FROZEN. Manual refresh `r`
was also dead (it too bails on `pollInFlight`). Only `q`/`c` still responded.

### The fix

Moved `m.pollInFlight = false` to BEFORE the state guard in `onReportDone`. By the
time `reportDoneMsg` arrives the report goroutine has genuinely returned, so
clearing the flag regardless of state is correct and cannot cause a double report.
The state guard still drops the stale message, but `pollInFlight` is now cleared so
esc-resume and `r` work again. The successor tick (`scheduleNextPoll`) stays AFTER
the guard, fired only while still `StateValidationPolling` — so no double-poll is
introduced (verified by the still-green `TestModel_ValidationPolling_PollsSequentially`).

### TDD Cycle Evidence (Batch 3)

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| Remediation: cancel-confirm report drop must not freeze polling | Added `TestModel_ReportDoneMsg_DuringCancelConfirm_DoesNotFreezePolling` + `TestModel_CancelConfirm_EscAfterInFlightReport_ManualRefreshWorks` FIRST; `go test ./internal/app/ -run '...DoesNotFreezePolling|...ManualRefreshWorks'` → both FAIL (`pollInFlight` stuck true; manual refresh a dead key) — the freeze reproduced | Moved `m.pollInFlight = false` before the state guard in `onReportDone`; same command → both PASS | None — one-line move; sound-parts regression set (esc-return, sequential-poll/no-double-poll, late-report drop, `q`-invariant, typed-CANCELAR gate, own-job-only, onCancelDone guards) all still PASS; `gofmt` clean |

### Files Changed (Batch 3)

| File | Action | What Was Done |
|------|--------|----------------|
| `internal/app/update.go` | Modified | `onReportDone`: moved `m.pollInFlight = false` ahead of the `state != StateValidationPolling` stale guard so a report dropped while on `StateCancelConfirm` still clears the in-flight guard; the successor tick stays after the guard (no double-poll). Doc comment updated to explain the ordering. |
| `internal/app/cancel_confirm_test.go` | Modified | Added the two RED-first liveness tests above (kept the existing `TestModel_CancelConfirm_EscReturnsToPolling` intact — added the in-flight-report variant rather than replacing it). |
| `openspec/changes/deploy-queue/apply-progress.md` | Modified | This Batch 3 remediation section. |

### Work Unit Evidence (Batch 3)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/app/ -run 'DoesNotFreezePolling|ManualRefreshWorks|EscReturnsToPolling|PollsSequentially|LateReportDoneMsg_AfterCancel|QLeavesJobActive|CorrectTextCancelsOwnJobOnly|OnCancelDone' -v` → all PASS (2 new liveness tests + 7 sound-parts regression tests, incl. the sequential/no-double-poll and `q`-invariant guards) |
| Runtime harness command/scenario and exact result | N/A for a new runtime boundary — this is a pure `Model.Update` state-machine liveness fix (no new external command/subprocess). Full-suite runtime coverage exercised via `go test -race ./...` → all packages `ok`. |
| Rollback boundary | Revert the single `m.pollInFlight = false` move in `internal/app/update.go` `onReportDone` (back after the state guard) and delete the two added tests in `cancel_confirm_test.go`. Nothing else in HU-009/HU-012 is touched; the typed-CANCELAR gate, own-job-only target, two-writer stale guards, `cancelPoll` teardown, and `q`-exit invariant are all unchanged. |

### Full Suite Verification (Batch 3 — verbatim, as requested)

```
export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./...
```

Result: build OK; `go vet ./...` clean; `gofmt -l .` produced no output (clean);
`go test -race ./...` → all packages `ok` (`cmd/deploydeck`, `internal/app`,
`internal/config`, `internal/delta`, `internal/exec`, `internal/git`,
`internal/prereq`, `internal/runs`, `internal/salesforce`).

## Status

24/24 total change tasks complete (Phase 1: 2/2, Phase 2: 6/6, Phase 3: 2/2,
Phase 4: 2/2, Phase 5: 6/6, Phase 6: 1/1, Phase 7: 2/2, Phase 8: 3/3). HU-009 and
HU-012 are both fully implemented and verified end to end. Batch 3 remediates the
MEDIUM cancel-flow liveness bug found by adversarial review (RED→GREEN, no
regressions). Ready for `sdd-verify`.
