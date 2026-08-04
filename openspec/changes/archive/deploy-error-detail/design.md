# Design: Deploy Error Detail (surface the failure detail the CLI already emits)

## Technical Approach

All eight gaps are ADDITIVE. `internal/salesforce` grows struct fields + one sentinel and parses
data the CLI already emits on stdout (exit-code-independent parsing at `report.go:176-201` stays
verbatim). `internal/app/{view,app}` render the newly-parsed fields through the existing semantic
palette (`style.go`); `commands.go`/`update.go` persist previously-dropped raw envelopes via the
established Writer companion pattern. NO new exec surface, NO new app State (D1/D4), NO schema bump.
Maps to proposal D1–D8.

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| D1 parse home | New fields on existing `salesforce` structs; app renders only | parse in app | preserves exec boundary; old data loads zero-valued |
| D2 stack traces | render `stackTraceFrames = 1` (first frame) per failing test, muted | full trace; 3 frames | first frame = throw site (file:line), the actionable datum; N tests × 3 frames overflows the TUI; full trace via report path (D5) |
| D3 coverage cut | `CodeCoverageResult.Percent() < coverageGatePercent(75)`, worst-first, cap `coverageMaxRows = 10`, `+K más` overflow line | full `codeCoverage[]`; N-worst regardless of % | `codeCoverage[]` holds hundreds of classes; below-gate classes ARE the culprits; org-wide warning (empty name) stays distinct (already handled) |
| D4 partial | `SucceededPartial` = amber header + callout in `viewValidationResult`; `terminalState` UNCHANGED (still folds to `StateSucceeded`) | new app State | a new state ripples through resume/persistence/quick-deploy eligibility; callout is byte-local |
| D5 report path | `AppendReport` returns the written `report-NNN.json` path → `m.reportPath`; replaces the `view.go:986` "revisa el JSON crudo" dead-end | raw-JSON viewer screen; model rescans dir | writer already computes the path (`writer.go:237`); returning it is a 1-line change; viewer is out of scope |
| D6 cancel sentinel | `ErrCancelAlreadyTerminal` when the error-envelope `name` ∈ {`CannotCancelDeployPre`, `CannotCancelDeploy`}; app branches via `errors.Is` | only `Pre`; raw-string match | BOTH mean "job already terminal, nothing to cancel" — identical friendly outcome from the app's view; mirrors `ErrQueuePermission`'s heuristic idiom (+ same unverified caveat) |
| D7 launch-failure raw | pre-created run → `SaveRawCompanion(runID,"validate.json",raw)`; no-runID → `Create(rec{Status:"Failed"},raw)` (mirrors the success fallback); path surfaced on `viewValidationStart` | new state; drop raw | reuses the `validate.json` companion; `validate.go` ALREADY returns `Raw` on launch failure (`validate.go:90`) — refinement of proposal: `validate.go` needs NO change, the fix is app-side persistence |
| gap7 cancel/quick fail | `SaveRawCompanion(runID,"cancel-error.json"/"quick-error.json",raw)`; status UNTOUCHED | reuse `cancel.json`/`quick.json` | success companions carry Status semantics (`MarkCanceled` sets `Status=Canceled`); a failure must not masquerade as success |
| D8 queue detail | SOQL +3 columns; render `StateDetail`/`ErrorMessage`/`ErrorStatusCode` ONLY for errored entries (`ErrorMessage!=""||ErrorStatusCode!=""`) | render for every row (progress UX) | keeps `StateDetail` progress-display out of scope (Q2); errored-only stays in the error-surfacing scope |

## Data Flow

    sf ... --json (stdout, exit-code-independent)  ──►  internal/salesforce  (PARSE ONLY, additive)
      report.go  → ComponentFailure{+FileName,LineNumber,ColumnNumber,ProblemType}, TestFailure{+StackTrace},
                   CodeCoverageResult[], FlowCoverageWarning[], DeployReport{+StateDetail}
      queue.go   → DeployQueueEntry{+StateDetail,ErrorMessage,ErrorStatusCode}   (SOQL +3 cols)
      cancel.go  → CancelResult{+Status,CanceledByName};  ErrCancelAlreadyTerminal  (name∈{Pre,raced})
      quick.go   → QuickDeployResult{+Status}      validate.go → unchanged (already returns Raw)
                                    │
      internal/app (commands.go/update.go — wire + persist, NO exec)
        onReportDone       → m.reportPath = Runs.AppendReport(...)                      [D5]
        onCancelDone       → errors.Is(ErrCancelAlreadyTerminal)? friendly (NO companion write)
                             : error + SaveRawCompanion("cancel-error.json")             [D6, gap7]
        onQuickDeployDone  → SaveRawCompanion("quick-error.json")                        [gap7]
        validateCmd(err!=nil) → SaveRawCompanion("validate.json") | Create(Failed)       [D7]
                                    │
      internal/runs/writer.go (companion pattern):  AppendReport→(path,err) ·
        SaveRawCompanion(runID,file,raw)→(path,err) · Create(rec{Failed},raw)
                                    │
      internal/app/view.go (render via style.go palette):
        validationBody      → file:line[:col] + [!!]Warning · 1 stack frame (dim) · per-class %<75 (amber)
                              · flow warnings inline · report-path line (dim, replaces view.go:986)
        viewValidationResult→ SucceededPartial: amber header + callout (D4, no new state)
        viewQueueReview     → errored jobs: StateDetail/ErrorMessage (red)
        viewValidationStart → launch-failure: validate.json path (dim)

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/salesforce/report.go` | Modify | grow `ComponentFailure`/`TestFailure`; add `CodeCoverageResult`+`Percent()`, `FlowCoverageWarning`; `DeployReport{+StateDetail,CodeCoverage,FlowCoverageWarnings}`; matching envelope tags + map loops |
| `internal/salesforce/queue.go` | Modify | `deployQueueSOQL` +`StateDetail,ErrorMessage,ErrorStatusCode`; grow record + `DeployQueueEntry` + `toEntry` |
| `internal/salesforce/cancel.go` | Modify | decode `Status`/`CanceledByName`; add `ErrCancelAlreadyTerminal` + `name`-envelope classifier |
| `internal/salesforce/quick.go` | Modify | decode `Status` |
| `internal/salesforce/validate.go` | None | already returns `Raw` on launch failure (D7 refinement) |
| `internal/runs/writer.go` | Modify | `AppendReport`→`(string,error)`; new `SaveRawCompanion(runID,filename,raw)(string,error)` |
| `internal/app/view.go` | Modify | component detail, first stack frame, per-class %, flow warnings, report path, partial callout, queue failure detail — all through `style.go` |
| `internal/app/app.go` | Modify | `viewValidationResult` partial header/callout (`terminalState` unchanged) |
| `internal/app/update.go` | Modify | capture `reportPath`; cancel already-terminal branch; persist cancel/quick/validate failure raw |
| `internal/app/commands.go` | Modify | `validateCmd` error branch persists launch raw + returns `runDir` |

## Interfaces / Contracts

```go
// internal/salesforce/report.go — additive fields (JSON tags per docs ref, exploration.md)
type ComponentFailure struct { Component, Type, Message,
    FileName /*fileName*/ string; LineNumber /*lineNumber*/, ColumnNumber /*columnNumber*/ int;
    ProblemType /*problemType: "Error"|"Warning"*/ string }
type TestFailure struct { Class, Method, Message, StackTrace /*stackTrace*/ string } // Time deferred
type CodeCoverageResult struct { Name, Namespace string; NumLocations, NumLocationsNotCovered int }
func (c CodeCoverageResult) Percent() int // (num-notCovered)/num*100; 0 when num==0
type FlowCoverageWarning struct { FlowName /*flowName*/, Message /*message*/ string }
// DeployReport +StateDetail string, +CodeCoverage []CodeCoverageResult, +FlowCoverageWarnings []FlowCoverageWarning
// envelope: details.runTestResult.codeCoverage[], .flowCoverageWarnings[]; top-level stateDetail

// internal/salesforce/cancel.go
type CancelResult struct { Status /*status*/, CanceledByName /*canceledByName*/, Raw string }
var ErrCancelAlreadyTerminal = errors.New("salesforce: deploy job already terminal; nothing to cancel")
// classify: unmarshal stdout {name}; name ∈ {"CannotCancelDeployPre","CannotCancelDeploy"} ⇒ wrap sentinel
// (unverified against a real terminal-job cancel — deliberately exact-name; any other name stays generic)

// internal/salesforce/quick.go
type QuickDeployResult struct { Status /*status*/, Raw string }

// internal/runs/writer.go
func (w *Writer) AppendReport(runID, status string, reportRaw []byte) (path string, err error) // path=report-NNN.json
func (w *Writer) SaveRawCompanion(runID, filename string, raw []byte) (path string, err error) // filename is a const, never user input
```

## Testing Strategy (strict TDD — RED first)

| Layer | What | File |
|---|---|---|
| Unit | report parse: line/col/file/problemType (Error+Warning), single-vs-multi `componentFailures`, empty `details`; `stackTrace`; `codeCoverage[]` incl `Percent()` (num==0→0); `flowCoverageWarnings`; `stateDetail` | `report_test.go` |
| Unit | queue SOQL contains the 3 new columns; record→entry maps `StateDetail`/`ErrorMessage`/`ErrorStatusCode`; empty stays zero | `queue_test.go` |
| Unit | cancel decode `Status`/`CanceledByName`; `name`=`CannotCancelDeployPre`→`errors.Is(ErrCancelAlreadyTerminal)`; `CannotCancelDeploy`→same; other name→generic; success path Raw byte-identical | `cancel_test.go` |
| Unit | quick decode `Status`; Raw preserved (`MarkQuickDeployed` bytes unchanged) | `quick_test.go` |
| Unit | `AppendReport` returns the exact `report-NNN.json` path; `SaveRawCompanion` writes verbatim bytes to the const filename + returns its path | `writer_test.go` |
| View | each new line: `file:line[:col]`, `[!!]`Warning label, first stack frame, per-class `%`<75 worst-first+cap+`+K más`, flow-warning inline, report-path line (replaces "revisa el JSON crudo"), queue errored-job detail, `SucceededPartial` amber header+callout, muted styles render plain under Ascii TestMain | `view_*_test.go` |
| Integ | validate launch-failure: pre-created run → `validate.json` companion + surfaced path; no-runID → `Create(Failed)`; cancel/quick failure → `cancel-error.json`/`quick-error.json`; already-terminal cancel leaves run UNMARKED + friendly notice | `delta_validation_test.go`, `cancel_confirm_test.go`, `quick_deploy_test.go` |
| Churn | UPDATE existing exact-string assertions: component/test/coverage render blocks (`delta_validation_test.go`), "revisa el JSON crudo" hint, cancel raw-error path (`cancel_confirm_test.go`), `SucceededPartial` success header (`transitions_test.go`, `delta_validation_test.go:674`), queue render row | inventory located during RED |

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| Argument composition (SOQL) | Applicable — `deployQueueSOQL` grows | still ONE `--query` slice element (`queue.go:180`), never shell-joined/interpolated | SOQL-string assertion |
| Companion file writes | Applicable — `SaveRawCompanion` | `filename` is a compile-time const (`validate.json`/`cancel-error.json`/`quick-error.json`); `runID` app-controlled; verbatim bytes, no traversal | companion-write test |
| Routing / subprocess / exec surface | N/A | additive parse+render only; no new exec, `os_runner` exit-code-as-data untouched (D1) | — |

## Migration / Rollout

No migration. Additive struct/field growth loads pre-existing `report-NNN.json`/`run.json` zero-valued;
NO `SchemaVersion` bump. Render-gated: revert the `view`/`app` additions + the `SaveRawCompanion`/
`AppendReport`-return calls to restore prior screens. Exec surface untouched. Single PR, `size:exception`
pre-approved (proposal Delivery Note).

## Open Questions

- [x] **Queue WHERE clause stays `Status IN ('Pending','InProgress')`** (design-validation
  finding 1): the queue view is the ACTIVE queue — terminal `Failed` jobs are structurally not
  part of it, and widening the query would be scope creep past gap 6/D8 and change the screen's
  semantics. The deploy-queue delta scenarios were reworded to the reachable condition (an
  `InProgress` entry with `ErrorMessage`/`ErrorStatusCode` populated, with `StateDetail` as
  context); a non-errored entry with only `StateDetail` renders nothing (progress-only display is
  out of scope per the proposal).
- [x] **Already-terminal cancel (sentinel) writes NO companion** (design-validation finding 3):
  it is a friendly outcome, not a failure — the deploy's own result is already persisted by
  report polling. `SaveRawCompanion("cancel-error.json")` fires ONLY on the generic-error branch.
- [x] **JSON tag convention** (design-validation finding 6): unmarshaling follows report.go's
  existing pattern — private envelope structs carry the real `json:"..."` tags; exported structs
  stay tag-free and hand-mapped. The new envelope fields mirror that convention; the exact
  `codeCoverage[]`/`flowCoverageWarnings[]` nesting settles in the RED parse test (item below).

- [x] `flowCoverageWarnings` layout → **inline** under the existing "Cobertura de código" heading, prefixed `flujo:`; coverage is one concern, a separate block adds noise.
- [x] Queue `StateDetail` scope → **errored jobs only** (`ErrorMessage`/`ErrorStatusCode` non-empty); progress-UX `StateDetail` stays out of scope (Q2 lean).
- [x] `TestFailure.Time` → **deferred**; not an error gap, lives in the surfaced report JSON.
- [ ] Verify against a real report fixture that `codeCoverage[]`/`flowCoverageWarnings[]` nest under `details.runTestResult` (mirrors the existing E2E shape-confirmation note) — settle exact tags in the RED report-parse test.
