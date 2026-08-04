# Apply Progress: Deploy Error Detail

**Mode**: Strict TDD
**Test runner**: `PATH=/usr/local/go/bin:$PATH go test ./... -race`

## Status
45/45 tasks complete. ALL PHASES DONE. Final gates (Phase 6) all green: `go build ./...`, `go vet ./...`, `gofmt -l internal cmd` (empty), `go test ./... -race -count=1` (full repo suite green).

## TDD Cycle Evidence — Phase 1

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `report_test.go` | Unit | ✅ full pkg green (baseline) | ✅ Written (compile fail: undefined fields) | ✅ Passed | ✅ Error+Warning+legacy-zero-value (3 entries) | ➖ None needed |
| 1.2 | `report_test.go` | Unit | ✅ (same run) | ✅ Written | ✅ Passed | ✅ present+absent StackTrace | ➖ None needed |
| 1.3 | `report_test.go` | Unit | ✅ (same run) | ✅ Written | ✅ Passed | ✅ 4 `Percent()` cases + envelope-parse case | ➖ None needed |
| 1.4 | `report_test.go` | Unit | ✅ (same run) | ✅ Written | ✅ Passed | ➖ Single scenario (spec has one) | ➖ None needed |
| 1.5 | `report.go` (GREEN impl for 1.1-1.4) | — | — | — | ✅ all 4 RED tests pass | — | ✅ gofmt clean |
| 1.6 | `queue_test.go` (mirrored SOQL const) | Unit | ✅ | ✅ Written (existing exact-match test now would mismatch) | ✅ Passed | ➖ covered by 1.7/existing tests | ➖ None needed |
| 1.7 | `queue_test.go` | Unit | ✅ | ✅ Written (compile fail: undefined fields) | ✅ Passed | ✅ errored-entry + absent-fields entry | ➖ None needed |
| 1.8 | `queue.go` (GREEN impl for 1.6-1.7) | — | — | — | ✅ all queue tests pass | — | ✅ gofmt clean |
| 1.9 | `cancel_test.go` | Unit | ✅ | ✅ Written (compile fail: undefined fields/sentinel) | ✅ Passed | ✅ Pre-name, raced-name, other-name, success-decode (4 cases) | ➖ None needed |
| 1.10 | `cancel.go` (GREEN impl) | — | — | — | ✅ all cancel tests pass | — | ✅ gofmt clean; fixed 1 pre-existing threat-matrix fixture (`{"status":0}` → `{"status":0,"result":{}}`) broken by the new success-decode path |
| 1.11 | `quick_test.go` | Unit | ✅ | ✅ Written (compile fail: undefined field) | ✅ Passed | ➖ Single scenario (spec has one) | ➖ None needed |
| 1.12 | `quick.go` (GREEN impl) | — | — | — | ✅ all quick tests pass | — | ✅ gofmt clean; fixed same pre-existing fixture issue in `argcomposition_test.go` |
| 1.13 | Verify | — | — | — | ✅ `go test ./internal/salesforce/... -race -count=1` GREEN | — | — |

### Test Summary (Phase 1)
- **Total tests written**: 13 new test functions (+ table-driven sub-cases: 4 `Percent()` cases, 2 queue-entry cases)
- **Total tests passing**: all (full `internal/salesforce` suite green, `-race`)
- **Layers used**: Unit (13)
- **Approval tests**: 2 pre-existing fixtures in `argcomposition_test.go` updated (`cancel`/`quick` adversarial-jobID tests) — not behavioral assertion changes, only adding a `"result":{}` envelope key the new decode path requires
- **Pure functions created**: 1 (`CodeCoverageResult.Percent()`)

### Deviations from Design (Phase 1)
- None — implementation matches design.md's Interfaces/Contracts section exactly (field names, JSON tags, `ErrCancelAlreadyTerminal` classification logic).
- Two PRE-EXISTING threat-matrix tests (`TestThreatMatrix_CancelDeploy_JobIDIsOneDiscreteArg`, `TestThreatMatrix_QuickDeploy_JobIDIsOneDiscreteArg` in `argcomposition_test.go`) broke as a direct, expected consequence of adding success-path envelope decoding (D6/gap7): their canned `{"status":0}` fixture had no `result` key, which `decodeEnvelope` requires. Fixed by adding `"result":{}` — no assertion logic changed, only the fixture's envelope shape to match reality (a real `sf` CLI success response always has a `result` object).

### Files Changed (Phase 1)
| File | Action | What Was Done |
|------|--------|----------------|
| `internal/salesforce/report.go` | Modified | Grew `ComponentFailure` (+FileName/LineNumber/ColumnNumber/ProblemType), `TestFailure` (+StackTrace); added `CodeCoverageResult`+`Percent()`, `FlowCoverageWarning`; grew `DeployReport` (+StateDetail/CodeCoverage/FlowCoverageWarnings); grew envelope + map loops |
| `internal/salesforce/report_test.go` | Modified | +5 new RED test functions (component location/problemType, stack trace, `Percent()` table test, codeCoverage parse, flowCoverageWarnings+stateDetail) |
| `internal/salesforce/queue.go` | Modified | `deployQueueSOQL` +3 columns (WHERE clause unchanged); grew `deployRequestRecord`, `DeployQueueEntry`, `toEntry` |
| `internal/salesforce/queue_test.go` | Modified | Updated mirrored SOQL const; +1 new RED test (StateDetail/ErrorMessage/ErrorStatusCode mapping + zero-value case) |
| `internal/salesforce/cancel.go` | Modified | Grew `CancelResult` (+Status/CanceledByName); added `ErrCancelAlreadyTerminal` sentinel + name-envelope classifier; success path now decodes via `decodeEnvelope` |
| `internal/salesforce/cancel_test.go` | Modified | +4 new RED test functions (status/canceledByName decode, Pre-name sentinel, raced-name sentinel, other-name-stays-generic) |
| `internal/salesforce/quick.go` | Modified | Grew `QuickDeployResult` (+Status); success path now decodes via `decodeEnvelope` |
| `internal/salesforce/quick_test.go` | Modified | +1 new RED test (status decode + Raw byte-identical) |
| `internal/salesforce/argcomposition_test.go` | Modified | Fixed 2 pre-existing fixtures (`cancel`/`quick` threat-matrix tests) to include a `result` envelope key — required by the new success-decode path; no assertion changed |

## TDD Cycle Evidence — Phase 2 (`internal/runs` writer)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 2.1 | `writer_test.go` | Unit | ✅ full pkg green | ✅ Written (compile fail: assignment mismatch) | ✅ Passed | ✅ 2 sequential calls (report-001/002) | ➖ None needed |
| 2.2 | `writer_test.go` | Unit | ✅ (same run) | ✅ Written (compile fail: `SaveRawCompanion` undefined) | ✅ Passed | ✅ write-success + unknown-runID-errors (2 cases) | ➖ None needed |
| 2.3 | `writer_test.go` | Unit | ✅ (same run) | ✅ Written (compile fail: `SaveRawCompanion` undefined) | ✅ Passed | ➖ Single scenario (old-shape + new companion) | ➖ None needed |
| 2.4 | `writer.go` (GREEN impl for 2.1-2.3) | — | — | — | ✅ all runs tests pass | — | ✅ gofmt clean; updated 6 pre-existing call sites in `writer_test.go` to the new 2-return signature (mechanical, no assertion change) |
| 2.5 | Verify | — | — | — | ✅ `go test ./internal/runs/... -race -count=1` GREEN; `go vet` clean | — | — |

### Test Summary (Phase 2)
- **Total tests written**: 4 new test functions
- **Total tests passing**: all (full `internal/runs` suite green, `-race`)
- **Layers used**: Unit (4)
- **Approval tests**: 6 pre-existing `AppendReport` call sites in `writer_test.go` updated to the 2-return signature (mechanical ripple, no behavioral change)
- **Pure functions created**: 0 (both new/changed methods have real disk I/O side effects by design)

### Files Changed (Phase 2)
| File | Action | What Was Done |
|------|--------|----------------|
| `internal/runs/writer.go` | Modified | `AppendReport` now returns `(string, error)` — the exact `report-NNN.json` path; new `SaveRawCompanion(runID, filename string, raw []byte) (string, error)` |
| `internal/runs/writer_test.go` | Modified | Updated 6 pre-existing `AppendReport` call sites to 2-return form; +4 new RED tests (exact-path return, companion write, unknown-runID error, old-shape-run + new-companion List/Load unaffected) |

## TDD Cycle Evidence — Phase 3 (`internal/app` wiring)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 3.1 | `delta_validation_test.go` | Unit | ✅ (blocked pre-existing compile break from Phase 2 ripple, expected) | ✅ Written (compile fail: `reportPath` undefined) | ✅ Passed | ➖ Single scenario | ➖ None needed |
| 3.2 | `cancel_confirm_test.go` | Unit | ✅ | ✅ Written + **re-verified via temporary revert**: reverted the `errors.Is` branch, reran, saw genuine assertion failures (cancelErr non-nil, notice empty), then restored | ✅ Passed | ➖ Single scenario | ➖ None needed |
| 3.3 | `cancel_confirm_test.go` (extends existing test) | Unit | ✅ | ✅ Written + **re-verified via temporary revert**: reverted `SaveRawCompanion` call, reran, saw genuine file-not-found failure, then restored | ✅ Passed | ➖ Extends existing single scenario | ➖ None needed |
| 3.4 | `update.go` (GREEN impl for 3.2/3.3) | — | — | — | ✅ | — | ✅ gofmt clean |
| 3.5 | `quick_deploy_test.go` (extends existing test) | Unit | ✅ | ✅ Written + **re-verified via temporary revert**: reverted `SaveRawCompanion` call, reran, saw genuine file-not-found failure, then restored | ✅ Passed | ➖ Extends existing single scenario | ➖ None needed |
| 3.6 | `update.go` (GREEN impl for 3.5) | — | — | — | ✅ | — | ✅ gofmt clean |
| 3.7 | `standalone_validate_test.go` | Unit | ✅ | ✅ Written (compile fail: `rawPath` undefined) | ✅ Passed | ✅ pre-created-runID + no-runID-fallback (2 cases) | ➖ None needed |
| 3.8 | `commands.go` (GREEN impl for 3.7) | — | — | — | ✅ | — | ✅ gofmt clean |
| 3.9 | Verify | — | — | — | ✅ `go test ./internal/app/... -race -run "Cancel\|Quick\|Validate\|Report"` GREEN | — | — |

### Test Summary (Phase 3)
- **Total tests written**: 5 new test functions + 2 extended pre-existing tests (3.3, 3.5)
- **Total tests passing**: all within the Phase 3 scoped run (`Cancel|Quick|Validate|Report`); full-package run shows exactly the 2 EXPECTED Phase-4-owned queue-SOQL failures (see Known pending above), nothing else
- **Layers used**: Unit (7)
- **Approval tests**: 3 tasks (3.2/3.3/3.5) used the temporary-revert-then-restore technique to obtain genuine RED evidence where the new assertion didn't fail to COMPILE (the referenced symbols already existed from Phase 1/self) — production code was reverted, the failure was observed and recorded, then restored
- **Pure functions created**: 0

### Deviations from Design (Phase 3)
- None — `onCancelDone`/`onQuickDeployDone`/`validateCmd` implement D6/D7/gap7 exactly as specified. Companion filenames (`cancel-error.json`, `quick-error.json`, `validate.json`) are compile-time consts, never derived from input, per the Threat Matrix.
- Process note (self-correction): tasks 3.2/3.3/3.5 were initially implemented GREEN without an isolated RED run (the RED test text was written, but the assertion-level failure was only proven by compile errors that happened to be batched with other Phase-3 changes). Caught during evidence review — I temporarily reverted each production branch, reran the exact test, observed a genuine failure, then restored the implementation. All three now have verified RED→GREEN evidence.

### Files Changed (Phase 3)
| File | Action | What Was Done |
|------|--------|----------------|
| `internal/app/app.go` | Modified | Added `Model.reportPath` (D5) and `Model.validateRawPath` (D7) fields |
| `internal/app/update.go` | Modified | `onReportDone` captures `AppendReport`'s returned path into `m.reportPath`; `onCancelDone` branches `errors.Is(ErrCancelAlreadyTerminal)` (friendly, no companion) before the generic-failure path (persists `cancel-error.json`); `onQuickDeployDone` persists `quick-error.json` on failure; `onValidateDone` captures `msg.rawPath` into `m.validateRawPath`; added `cancelErrorCompanionFilename`/`quickErrorCompanionFilename` consts |
| `internal/app/commands.go` | Modified | `validateDoneMsg` grew `rawPath`; `validateCmd`'s launch-error branch persists via `SaveRawCompanion` (pre-created runID) or `Create(rec{Status:"Failed"}, raw)` fallback (no runID), per D7 |
| `internal/app/delta_validation_test.go` | Modified | +1 new RED test (`reportPath` capture) |
| `internal/app/cancel_confirm_test.go` | Modified | +1 new RED test (already-terminal friendly outcome); extended 1 existing test (generic-failure companion) |
| `internal/app/quick_deploy_test.go` | Modified | Extended 1 existing test (failure companion) |
| `internal/app/standalone_validate_test.go` | Modified | +2 new RED tests (pre-created-runID persistence, no-runID fallback) |

## TDD Cycle Evidence — Phase 4 (`internal/app` view rendering)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 4.1 | `view_test.go` | Unit | ✅ | ✅ Written, confirmed FAIL before impl | ✅ Passed | ✅ Error + Warning cases in one test | ➖ None needed |
| 4.2 | `view_test.go` | Unit | ✅ | ✅ Written, confirmed FAIL before impl (+ companion no-stack-trace case, legitimately already-passing) | ✅ Passed | ✅ multi-frame-truncation + no-trace (2 cases) | ➖ None needed |
| 4.3 | `view_test.go` | Unit | ✅ | ✅ Written (3 tests), confirmed FAIL before impl | ✅ Passed | ✅ worst-first+cap+overflow, flow-inline, org-wide-distinct (3 scenarios) | ➖ None needed |
| 4.4 | `view_test.go` | Unit | ✅ | ✅ Written, confirmed FAIL before impl | ✅ Passed | ✅ terminal-shows vs live-polling-omits (2 branches in 1 test) | ➖ None needed |
| 4.5 | `view_test.go` | Unit | ✅ | ✅ Written (2 tests: partial + plain-Succeeded regression guard), confirmed FAIL before impl | ✅ Passed | ✅ partial vs plain (2 cases) | ➖ None needed |
| 4.6 | `view.go` (GREEN impl for 4.1-4.5) | — | — | — | ✅ all pass | — | ✅ gofmt clean; extracted 3 pure helpers (`belowGateCoverage`, `componentFailureLocation`, `firstStackFrame`); renamed a shadowed local var (`mark`→`ownMark`) forced by the new `mark()` call in the same scope |
| 4.7 | `queue_review_test.go` | Unit | ✅ | ✅ Written (2 tests + mirrored-const update), confirmed FAIL before impl | ✅ Passed | ✅ errored+StateDetail-context vs non-errored-no-line (2 cases) | ➖ None needed |
| 4.8 | `view.go` (GREEN impl for 4.7) | — | — | — | ✅ all pass | — | ✅ gofmt clean |
| 4.9 | `view_test.go` | Unit | ✅ | ✅ Written, confirmed FAIL before impl | ✅ Passed | ➖ Single scenario | ➖ None needed |
| 4.10 | `view.go` (GREEN impl for 4.9) | — | — | — | ✅ | — | ✅ gofmt clean |
| 4.11 | Verify | — | — | — | ✅ `go test ./internal/app/... -race -run "View\|Queue\|Transitions"` GREEN under Ascii `TestMain` | — | — |

### Test Summary (Phase 4)
- **Total tests written**: 12 new test functions
- **Total tests passing**: all; full-package run (`go test ./internal/app/... -race -count=1`) also fully green — the 2 queue-SOQL failures noted after Phase 3 are now resolved
- **Layers used**: Unit (12) — all pure-render/model-field assertions via `strings.Contains`, matching the codebase's established convention
- **Approval tests**: 0
- **Pure functions created**: 3 (`belowGateCoverage`, `componentFailureLocation`, `firstStackFrame`)

### Deviations from Design (Phase 4)
- None — `validationBody`/`viewValidationResult`/`viewQueueReview`/`viewValidationStart` implement D2-D5/D8 exactly as specified (first-frame-only stack trace, gate=75%/cap=10 coverage, org-wide-vs-per-class label distinction, terminal-only report-path line, `SucceededPartial` amber callout, errored-only queue-row detail).
- Minor unplanned fix: `viewQueueReview`'s pre-existing loop-local variable named `mark` shadowed the package-level `mark()` semantic-color function once D8's rendering needed to call it inside the same loop scope. Renamed the local to `ownMark` — a mechanical, behavior-preserving rename (verified by the full pre-existing queue-view test suite staying green).

### Files Changed (Phase 4)
| File | Action | What Was Done |
|------|--------|----------------|
| `internal/app/view.go` | Modified | `validationBody`: component file:line:col + Warning label, first-stack-frame (muted), below-gate coverage % (worst-first, capped, overflow line), flow warnings inline, terminal-only report-path line, dead-end hint text simplified; `viewValidationResult`: `SucceededPartial` distinct header + amber callout; `viewValidationStart`: persisted validate.json path on launch failure; `viewQueueReview`: errored-entry detail (D8); added pure helpers `belowGateCoverage`/`componentFailureLocation`/`firstStackFrame` + consts `coverageGatePercent`/`coverageMaxRows` |
| `internal/app/view_test.go` | Modified | +12 new RED tests covering 4.1-4.5 and 4.9 |
| `internal/app/queue_review_test.go` | Modified | Updated mirrored `deployQueueSOQL` const (+3 columns); +2 new RED tests for D8 |

## Phase 5: Exact-String Assertion Churn

Inventory (5.1) found NO additional stale assertions beyond what Phases 1-4 already updated in-flight: a due-diligence grep across `delta_validation_test.go`, `cancel_confirm_test.go`, `quick_deploy_test.go`, `quick_deploy_e2e_test.go`, `queue_review_test.go`, `standalone_validate_test.go` for `"revisa el JSON crudo"` and for exact full-string equality assertions (`== ` comparisons on rendered view/body strings) found none — the codebase's established convention is `strings.Contains`, which degrades gracefully to added/changed content. The full package/repo test suite is green (5.2/5.3 — see Phase 6 below), confirming no assertion silently broke.

One extra pre-existing call site was found and fixed during this pass (outside the named Phase 5 file list, but the same category of Phase-2-signature ripple): `internal/salesforce/real_org_e2e_test.go:236` still used `AppendReport`'s old single-return form, which failed `go build ./...`/`go test ./...`. Fixed mechanically (`if _, err := writer.AppendReport(...)`), no assertion changed.

## Phase 6: Final Gates — ALL GREEN

| Gate | Command | Result |
|------|---------|--------|
| 6.1 | `PATH=/usr/local/go/bin:$PATH go build ./...` | exit 0, clean |
| 6.2 | `PATH=/usr/local/go/bin:$PATH go vet ./...` | exit 0, clean |
| 6.3 | `gofmt -l internal cmd` | empty output |
| 6.4 | `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` | full repo suite GREEN (14 packages, all `ok`) |

All ~29 delta-spec scenarios across the six capability deltas (`deploy-queue` 9, `deploy-validation` 2, `quick-deploy` 2, `run-persistence` 4, `validation-cancel` 4, `validation-progress` 8) are covered by RED tests written in Phases 1-4.

## Workload / PR Boundary
- Mode: single PR, `size:exception` pre-approved (tasks.md Review Workload Forecast)
- Current work unit: ALL 4 units done (`internal/salesforce` parse growth, `internal/runs` writer, `internal/app` wiring, `internal/app` view rendering) + Phases 5-6
- Boundary: the full change is complete and independently verified; rollback boundary per tasks.md Migration/Rollout — revert the `view`/`app` additions + the `SaveRawCompanion`/`AppendReport`-return call sites to restore prior screens; additive struct/field growth is backward-compatible (no `SchemaVersion` bump, no migration)
- Estimated review budget impact: within the pre-approved `size:exception` envelope; diff is additive-only except mechanical call-site/fixture updates forced by the 2 signature changes (`AppendReport`, `CancelResult`/`QuickDeployResult` decode)
