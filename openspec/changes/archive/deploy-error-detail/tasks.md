# Tasks: Deploy Error Detail (surface the failure detail the CLI already emits)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~850-1050 (5 salesforce files + 2 runs files + 4 app files + RED tests + churn across ~6 existing test files) |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR, 6 sequential internal work units (parse → persistence → wiring → render → churn → gates) |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

Rationale: `size:exception` pre-approved (proposal Delivery Note, >800 lines). Phase 2
(`internal/runs`) hard-depends on Phase 1's parse types compiling; Phase 3 (`internal/app`
wiring) hard-depends on Phase 2's `AppendReport`/`SaveRawCompanion` signatures; Phase 4
(rendering) depends on Phase 1's new fields being populated. Splitting would add review-order
friction without shrinking the reviewer's real surface (one additive parse/persist/render arc).

### Suggested Work Units (sequential commits inside the single PR)

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|------|------|----------------------|-----------------|-------------------|
| 1 | `internal/salesforce` parse growth (report/queue/cancel/quick) | `go test ./internal/salesforce/... -race` | N/A — `FakeRunner`-backed unit tests | Revert the 4 file diffs; structs zero-valued, no caller yet |
| 2 | `internal/runs` writer signature + companion | `go test ./internal/runs/... -race` | N/A — tmpdir-backed writer tests | Revert `AppendReport` return + `SaveRawCompanion`; Unit 1 unaffected |
| 3 | `internal/app` wiring (update.go/commands.go/app.go) | `go test ./internal/app/... -race -run "Cancel\|Quick\|Validate\|Report"` | N/A — `FakeRunner`/tmpdir harness, no real `sf` | Revert wiring call sites; Units 1-2 unaffected |
| 4 | `internal/app` view rendering (view.go) | `go test ./internal/app/... -race -run "View\|Queue\|Transitions"` | N/A — Ascii `TestMain` plain-text harness | Revert render additions; data stays parsed but unshown |

## Phase 1: `internal/salesforce` parse growth

- [x] 1.1 RED `report_test.go`: componentFailures `fileName`/`lineNumber`/`columnNumber`/`problemType` (Error+Warning), single+multi entries, empty `details` stays zero-valued.
- [x] 1.2 RED `report_test.go`: `TestFailure.StackTrace` captured when present, empty when absent.
- [x] 1.3 RED `report_test.go`: `codeCoverage[]` → `CodeCoverageResult{Name,Namespace,NumLocations,NumLocationsNotCovered}`; `Percent()` (num==0→0, normal division).
- [x] 1.4 RED `report_test.go`: `flowCoverageWarnings[]` → `FlowCoverageWarning{FlowName,Message}`; top-level `stateDetail` → `DeployReport.StateDetail`.
- [x] 1.5 GREEN `internal/salesforce/report.go`: grow `ComponentFailure`/`TestFailure`; add `CodeCoverageResult`+`Percent()`, `FlowCoverageWarning`; `DeployReport{+StateDetail,CodeCoverage,FlowCoverageWarnings}`; envelope tags + map loops (`report.go:216-236`).
- [x] 1.6 RED `queue_test.go`: `deployQueueSOQL` string contains `StateDetail,ErrorMessage,ErrorStatusCode`; still ONE `--query` slice element.
- [x] 1.7 RED `queue_test.go`: `toEntry` maps `StateDetail`/`ErrorMessage`/`ErrorStatusCode`; absent → zero value.
- [x] 1.8 GREEN `internal/salesforce/queue.go`: extend `deployQueueSOQL` (`queue.go:18`), `deployRequestRecord`, `DeployQueueEntry`, `toEntry` (`queue.go:109`).
- [x] 1.9 RED `cancel_test.go`: decode `Status`/`CanceledByName`; `name`=`CannotCancelDeployPre`→`errors.Is(ErrCancelAlreadyTerminal)`; `name`=`CannotCancelDeploy`→same; other name→generic error; success `Raw` byte-identical.
- [x] 1.10 GREEN `internal/salesforce/cancel.go`: grow `CancelResult{+Status,CanceledByName}`; add `ErrCancelAlreadyTerminal` + name-envelope classifier on the error branch (`cancel.go:48-53`).
- [x] 1.11 RED `quick_test.go`: decode `Status`; `Raw` preserved byte-identical.
- [x] 1.12 GREEN `internal/salesforce/quick.go`: grow `QuickDeployResult{+Status}`.
- [x] 1.13 Verify: `go test ./internal/salesforce/... -race` GREEN (validate.go untouched — already returns `Raw` on launch failure, `validate.go:90`).

## Phase 2: `internal/runs` writer

- [x] 2.1 RED `writer_test.go`: `AppendReport` returns `(path, error)` where `path` is the exact `report-NNN.json` written (`writer.go:237`).
- [x] 2.2 RED `writer_test.go`: new `SaveRawCompanion(runID, filename, raw) (path, error)` writes verbatim bytes to the given const filename under the run dir; unknown `runID` errors.
- [x] 2.3 RED `writer_test.go`: a run directory persisted BEFORE `SaveRawCompanion`'s companions existed still `List`/`Load`s unchanged (no `SchemaVersion` bump; unaffected by the new companion types).
- [x] 2.4 GREEN `internal/runs/writer.go`: change `AppendReport` signature (update its 5 existing callers' call sites are Phase 3's concern, not this file); add `SaveRawCompanion`.
- [x] 2.5 Verify: `go test ./internal/runs/... -race` GREEN.

## Phase 3: `internal/app` wiring (persistence + branching, no rendering)

- [x] 3.1 RED `update_test.go` or `run_persistence_test.go`: `onReportDone` captures `m.reportPath` from `AppendReport`'s returned path (`update.go:1098`).
- [x] 3.2 RED `cancel_confirm_test.go`: `onCancelDone` on `errors.Is(ErrCancelAlreadyTerminal)` shows a friendly message, run stays UNMARKED, and writes NO `cancel-error.json` companion.
- [x] 3.3 RED `cancel_confirm_test.go`: `onCancelDone` on a generic (non-sentinel) failure persists `cancel-error.json` via `SaveRawCompanion`; run stays unmarked (extends the existing "must NOT write cancel.json" test at `cancel_confirm_test.go:308`).
- [x] 3.4 GREEN `internal/app/update.go`: `onCancelDone` (`update.go:989`) branches `errors.Is(ErrCancelAlreadyTerminal)` before the generic-error path.
- [x] 3.5 RED `quick_deploy_test.go`: `onQuickDeployDone` failure persists `quick-error.json` via `SaveRawCompanion`; run stays unmarked.
- [x] 3.6 GREEN `internal/app/update.go`: `onQuickDeployDone` (`update.go:1027`) calls `SaveRawCompanion` on `msg.err != nil`.
- [x] 3.7 RED `delta_validation_test.go` or `standalone_validate_test.go`: `validateCmd` launch failure with a pre-created `runID` persists `validate.json` via `SaveRawCompanion` and returns `runDir`; no-`runID` fallback uses `Create(rec{Status:"Failed"}, raw)`.
- [x] 3.8 GREEN `internal/app/commands.go`: `validateCmd` (`commands.go:972`) error branch (`commands.go:988`) persists raw + sets `runDir` per D7.
- [x] 3.9 Verify: `go test ./internal/app/... -race -run "Cancel|Quick|Validate|Report"` GREEN.

## Phase 4: `internal/app` view rendering

- [x] 4.1 RED `view_test.go`: `validationBody` renders `file:line[:col]` per component failure; `problemType=Warning` entries labeled via `mark("!!")`, distinct from `Error` (`mark("XX")`).
- [x] 4.2 RED `view_test.go`: `validationBody` renders one stack-trace frame per failing test in `styleDim`.
- [x] 4.3 RED `view_test.go`: `validationBody` renders per-class coverage `%` for entries below 75%, worst-first, capped at 10 rows with a `"+K más"` overflow line; `flowCoverageWarnings` render inline under the coverage heading; when both a per-class `%` line and an existing org-wide (empty-`Name`) `CodeCoverageWarning` are present, the org-wide entry stays visually distinguished (its own "cobertura global" label) from the new per-class lines.
- [x] 4.4 RED `view_test.go`: `validationBody`/failure screens render the persisted report path (`m.reportPath`) in `styleDim`, replacing the `"revisa el JSON crudo"` dead-end at `view.go:986`.
- [x] 4.5 RED `view_test.go` or `transitions_test.go`: `viewValidationResult` on `StateSucceeded` with `m.report.Status == "SucceededPartial"` renders a distinct amber header + callout, never the plain-success text (extends `delta_validation_test.go:674`'s success-header assertion).
- [x] 4.6 GREEN `internal/app/view.go`: implement 4.1-4.4 in `validationBody` (`view.go:932-988`); implement 4.5 in `viewValidationResult` (`view.go:758`), reading `m.report.Status` (`terminalState` at `app.go:769` stays unchanged per D4).
- [x] 4.7 RED `queue_review_test.go`: mirrored `deployQueueSOQL` const updated to the +3-column query; `viewQueueReview` row for an `InProgress` entry with `ErrorMessage`/`ErrorStatusCode` set shows that detail in red; `StateDetail` renders as context only when `ErrorMessage`/`ErrorStatusCode` is non-empty; a non-errored entry with only `StateDetail` renders nothing extra.
- [x] 4.8 GREEN `internal/app/view.go`: `viewQueueReview` (`view.go:593`) renders errored-entry detail per 4.7.
- [x] 4.9 RED `standalone_validate_test.go` or `delta_validation_test.go`: `viewValidationStart` (`view.go:659`) on launch failure shows the message + the persisted `validate.json` path from Phase 3.7/3.8.
- [x] 4.10 GREEN `internal/app/view.go`: `viewValidationStart` renders the path.
- [x] 4.11 Verify: `go test ./internal/app/... -race -run "View|Queue|Transitions"` GREEN under Ascii `TestMain`.

## Phase 5: Exact-string assertion churn (pre-existing tests)

- [x] 5.1 Inventory every pre-existing `strings.Contains`/exact-match assertion touched by Phases 1-4's render/output changes across `delta_validation_test.go`, `cancel_confirm_test.go`, `quick_deploy_test.go`, `quick_deploy_e2e_test.go`, `queue_review_test.go`, `standalone_validate_test.go`.
- [x] 5.2 Update each located assertion to the new output (component/test/coverage blocks, `"revisa el JSON crudo"` hint removal, cancel raw-error companion path, `SucceededPartial` header, queue row format, mirrored `deployQueueSOQL` const).
- [x] 5.3 Verify: `go test ./internal/app/... ./internal/salesforce/... ./internal/runs/... -race` — all pre-existing byte-asserted success/cancel/quick behaviors stay GREEN.

## Phase 6: Final Gates

- [x] 6.1 `PATH=/usr/local/go/bin:$PATH go build ./...` — clean, exit 0.
- [x] 6.2 `PATH=/usr/local/go/bin:$PATH go vet ./...` — clean, exit 0.
- [x] 6.3 `gofmt -l internal cmd` — empty output.
- [x] 6.4 `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` — full suite GREEN, all ~29 delta-spec scenarios covered by RED tests above.
