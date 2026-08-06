# Verification Report: Validation-Comment Timing

**Change**: `validation-comment-timing`
**Branch**: `fix/validation-comment-timing` (uncommitted — current working tree verified)
**Mode**: Strict TDD (RED → GREEN) · OpenSpec artifact store
**Verdict**: **PASS**
**Date**: 2026-08-06

## Executive Summary

0 CRITICAL, 0 WARNING, 1 SUGGESTION (non-blocking). The single-line-block fix is
present exactly as designed, `onReportDone` is untouched, all 10 delta-spec
scenarios (6 deploy-gate + 4 validation-progress) map to passing tests, RED
authenticity for the load-bearing ordering test is credible from recorded
evidence, and every self-run gate is green. Ready for archive.

## Completeness

| Dimension | Result |
|---|---|
| Tasks complete | 14/14 `[x]` (tasks.md), matches apply-progress and code state |
| Spec artifacts | 2 delta specs present (deploy-gate, validation-progress) |
| Design artifact | Present; implementation matches D1–D5 |
| apply-progress | Present; TDD Cycle Evidence table populated |

## 1. Fix Fidelity (no scope creep)

CONFIRMED. `internal/app/update.go` `onPrCreated` success path (inside `msg.err == nil`),
after `MarkPRCreated(m.runID, msg.url)` (:1330), before the final `return m, nil`, inserts:

```go
if cmd := m.maybeTriggerValidationCommentCmd(m.report.Status); cmd != nil {
    return m, cmd
}
```

(lines 1332–1346, incl. explanatory comment). `onReportDone`'s own trigger (:1170–1172)
is byte-for-byte unchanged. `git diff --numstat`: `internal/app/update.go` **14 / 0** —
the only production change. No other tracked file modified. Matches design D1–D5 exactly.

## 2. Delta-Spec Scenario → Test Mapping (all PASS)

### deploy-gate (6 scenarios)

| # | Scenario | Covering test(s) | Kind | Result |
|---|---|---|---|---|
| 1 | Successful validation posts marker | `TestPostValidationCommentCmd_PostsWhenNoMarkerPresent` (deploy_gate_test.go:344) + new 1.6 | pre-existing + NEW | PASS |
| 2 | Re-validation no duplicate | `TestPostValidationCommentCmd_SkipsWhenMarkerAlreadyPresent` (:319) + new 1.4 | pre-existing + NEW | PASS |
| 3 | Condition passes only when marker present | `TestEvaluate_ValidationComment` present/absent/fail-closed (gate_test.go:243) + `TestHasValidationComment` (:516) | pre-existing | PASS |
| 4 | Ungated env posts no comment | `TestOnReportDone_TerminalSuccess_UngatedTarget_DoesNotTriggerPostComment` (update_test.go:379) + `_RequireCommentOff_` (:397) | pre-existing | PASS |
| 5 | Validation before PR → posts at PR creation | new 1.2 (zero at validation) + 1.3 (one at creation) | **NEW** | PASS |
| 6 | Exactly once across both trigger points | new 1.2 + 1.3 together | **NEW** | PASS |

### validation-progress (4 scenarios)

| # | Scenario | Covering test(s) | Kind | Result |
|---|---|---|---|---|
| 1 | Successful CheckOnly triggers comment | new 1.6 (reuse flow) + `TestOnReportDone_TerminalSuccess_...PostComment` (update_test.go:359) | NEW + pre-existing | PASS |
| 2 | Ungated env posts nothing | `TestOnReportDone_TerminalSuccess_UngatedTarget_DoesNotTriggerPostComment` (update_test.go:379) | pre-existing | PASS |
| 3 | Non-successful terminal state does not trigger | `TestOnReportDone_NonSuccessfulTerminal_GatedTarget_DoesNotTriggerPostComment` (:417) + new 1.5 (at onPrCreated point) | pre-existing + NEW | PASS |
| 4 | Validation before PR → PR creation (re)triggers | new 1.3 (load-bearing) | **NEW** | PASS |

**apply-progress claim audit** — "ungated-posts-nothing covered by pre-existing tests":
CONFIRMED. `TestOnReportDone_TerminalSuccess_UngatedTarget_DoesNotTriggerPostComment` and
`TestOnReportDone_TerminalSuccess_RequireCommentOff_DoesNotTrigger` both drive a terminal
`Succeeded` and assert `cmd != nil` FAILS (i.e. the trigger is genuinely suppressed) — not
vacuous. The condition-only-when-marker scenario is likewise really asserted in
`TestEvaluate_ValidationComment` (present passes / absent fails / CommentErr fail-closed).

## 3. RED Authenticity (load-bearing test)

CREDIBLE from recorded evidence. apply-progress records that in Phase 1, 4/5 tests passed
immediately and only **1.3** `TestOnPrCreated_AfterTerminalSuccess_PostsCommentExactlyOnce`
FAILED pre-fix with `onPrCreated() cmd = nil, want the validation-comment cmd...`. This is
internally consistent with the code: the test asserts `cmd != nil` after `onPrCreated`
(test lines 141–143); pre-fix, the success path returned `(m, nil)` unconditionally
(design D5, update.go:1332), so `cmd` would be nil → 1.3 fails. Tests 1.2/1.4/1.5/1.6
legitimately passed pre-fix because they exercise unchanged guard/dedup/reuse paths. Genuine
failing-for-the-right-reason RED tied to real behavior, not a manufactured assertion.

## 4. Gates (run in this verify phase)

| Gate | Command | Result |
|---|---|---|
| Build | `go build ./...` | exit 0 |
| Vet | `go vet ./...` | exit 0 |
| Format | `gofmt -l internal cmd` | empty output |
| Targeted race | `go test ./internal/app/... -race -count=1` | `ok ... 34.720s` |

(Full-repo suite intentionally left to the orchestrator, not repeated here.)

## 5. Boundary / Regression

- **No direct exec in internal/app**: `TestApp_NeverImportsExecSeam` (boundary_test.go)
  enforces that internal/app's own source imports none of `os/exec`, `internal/exec`,
  `net/http`, `version`, `update`, `ai`. All `gh` interaction in the new tests goes through
  `execpkg.FakeRunner` via `github.New(fr)`. Guard passed in the race run.
- **No pre-existing test weakened**: `git diff --numstat -- '*_test.go'` = empty (0 tracked
  test additions/deletions). The only test change is the new untracked file
  `internal/app/validation_comment_timing_test.go`.
- **Pre-existing pair still green**: `TestPostValidationCommentCmd_SkipsWhenMarkerAlreadyPresent`
  and `TestPostValidationCommentCmd_PostsWhenNoMarkerPresent` both PASS unchanged.

## 6. Idempotency / Exactly-Once

PROVEN. `TestOnPrCreated_MarkerAlreadyPresent_NoDoublePost` (1.4) seeds the PR with this
job's marker body, drives a second trigger through `onPrCreated`, and asserts **zero**
`gh pr comment` calls — the `gate.HasValidationComment` skip-if-present in
`postValidationCommentCmd` (commands.go:1746) is the single dedup authority across both fire
sites. Combined with 1.2 (zero post at validation, PR absent) + 1.3 (exactly one post at
PR creation), exactly-once delivery across both trigger points is behaviorally pinned.

## TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD Evidence reported | ✅ | Cycle table present in apply-progress |
| All tasks have tests | ✅ | 5 new test funcs (incl. 2 sub-tables) + reused pre-existing coverage |
| RED confirmed | ✅ | 1.3 failed pre-fix (recorded), consistent with code |
| GREEN confirmed | ✅ | All 5 new + pre-existing pass on this run |
| Triangulation adequate | ✅ | 1.2↔1.3 (zero vs one), 1.4↔1.3 (marker vs none), 1.5 table over 3 statuses |
| Safety net for modified file | ✅ | update.go modified; whole internal/app package (incl. boundary guard) green |

## Assertion Quality

✅ All assertions verify real behavior. Every test calls production code
(`m.onReportDone` / `m.onPrCreated`, then `run(t, cmd)`) and asserts concrete outcomes:
`cmd` nilness and exact `gh pr comment` call counts (0 or 1) via `countPrComments`. No
tautologies, no ghost loops (the count loop is a counter; assertions on `n == 0/1` sit
outside it), no smoke-only tests. Empty-count assertions (1.2, 1.4, 1.5) each have a
non-empty companion (1.3, 1.6). Helpers `vcCommentBody`/`primeVcPostComment`/`countPrComments`
avoid hand-computing gh's arg shape (probe-then-reprime).

## Quality Metrics

- **Vet**: ✅ no errors. **gofmt**: ✅ clean. Coverage tool: not run (informational only).

## Issues

### CRITICAL
None.

### WARNING
None.

### SUGGESTION
- **S1 (informational)**: CodeGraph static analysis flags
  `maybeTriggerValidationCommentCmd` as "no covering tests found" because no test invokes it
  by name. This is a static-analysis artifact, not a real gap — it is exercised through BOTH
  seams (`onReportDone` and `onPrCreated`) by the new and pre-existing tests. No action
  required; a direct unit test could be added later for locality but adds no coverage.

## Design Coherence

No deviations. Implementation matches design D1 (dual trigger, onReportDone kept verbatim),
D2 (reuse `maybeTriggerValidationCommentCmd`), D3 (marker-on-PR idempotency),
D4 (`m.report.Status` guard), D5 (`if cmd := …; cmd != nil { return m, cmd }` mirror).

## Verdict

**PASS** — 0 CRITICAL, 0 WARNING, 1 SUGGESTION (non-blocking). Recommend `sdd-archive`.
