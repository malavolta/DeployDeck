# Tasks: Run Failure Detail

Strict TDD: in every phase the RED task (failing test, confirmed failing for the right reason) precedes the GREEN task. Budget: ~2,000 changed lines (design forecast ~1,280 raw, ~1,700 at 1.6x). Final phase stops the work if the diff exceeds ~2,000.

Reconciliation notes (apply throughout):
- Headline marker: the headline uses the existing `mark()` tokens (`[XX]` failure, `[OK]`, `[!!]`, `[i]`), NOT a literal `✗`. The spec wording `✗ Falló por cobertura: ...` is satisfied by the mark token; tests assert on `Falló por cobertura: 66% global < 75%` plus the `mark(...)` token, never the literal `✗`.
- Up/Down in history now returns a load cmd (`ensureReportCmd`) when `Deps.Runs` is non-nil and the row is uncached. Tests that expected a nil cmd must be updated (with nil `Runs` the cmd stays nil).
- The stale `d` "(delete)" wording in the `run-history` spec is already fixed in the delta (`specs/run-history/spec.md`, MODIFIED "Per-Row Quick Deploy Action On An Eligible Run").

---

## Slice 1: seams, classifier, tail line, result headline (~800 lines)

### Phase 1.0: Pin audit and goldens (~60 lines, sequential, first)

- [x] 1.0.1 Audit pinned tests before touching code. List every assertion that pins (a) the terminal result screen / `validationBody` output, (b) the delta-validation screens, (c) the history tail and Up/Down returning nil, (d) the two `runDetail` tests. Files: `internal/app/view_test.go`, delta-validation tests (`internal/app/*validation*_test.go`), `internal/app/run_history_test.go` (206-212), `internal/app/quick_deploy_test.go` (184), `internal/app/footer_pins_test.go`, `internal/app/esc_q_matrix_test.go`. Output: the list recorded as a comment block in the PR description or commit body; no code yet. Spec: all deltas.
- [x] 1.0.2 Capture a golden of `validationBody` for a representative set of reports (Failed with sections, Succeeded, fallback) BEFORE extraction, as a test in `internal/app/view_test.go`. It must pass on current code. Spec: design "Body reuse" (byte-identical).

### Phase 1.1: Tolerant parse in salesforce (~140 lines; parallel with 1.2)

- [x] 1.1.1 RED: table tests in `internal/salesforce/report_test.go`: trailing text after envelope, leading noise before first `{`, empty `Status` is an error, mapping parity with `ReportDeploy` for the same fixture. Spec: run-persistence "Trailing CLI text tolerated".
- [x] 1.1.2 GREEN: in `internal/salesforce/report.go` extract lines ~286-339 into `toDeployReport(decoded, raw)`; add `ParseDeployReport(raw)` (first JSON value via `json.Decoder`, retry from first `{`, empty Status errors). `ReportDeploy` uses `toDeployReport`; behavior unchanged; existing tests stay green.

### Phase 1.2: Read-back in runs (~150 lines; parallel with 1.1)

- [x] 1.2.1 RED: tests in `internal/runs/reports_test.go` with `t.TempDir()`: newest wins (001+003 gives 003); corrupt newest falls back and returns the rejected path; all rejected; no files gives `ErrNoReport`; non-report files ignored. Spec: run-persistence scenarios (Latest wins, Unparseable falls back, No reports).
- [x] 1.2.2 GREEN: create `internal/runs/reports.go`: `ReportFile`, `ErrNoReport`, `(*Writer).LatestReport(runID, accept func([]byte) bool) (ReportFile, []string, error)`, lazy newest-first scan. No import of `salesforce`.

### Phase 1.3: Classifier (~230 lines; depends on 1.1 types only, can start after 1.1.2)

- [x] 1.3.1 RED: pure table tests in `internal/app/failure_test.go`: every priority row (canceled > metadata > tests > coverage > org error > generic); Succeeded `[OK] Validación exitosa`, SucceededPartial `[!!] Resultado parcial`, non-terminal `[i] En curso` with no failure headline; coverage regex `66% global < 75%` plus fallback `Falló por cobertura de código`; hint states org-average covers all org Apex classes; 0% list capped at 5 then `+K más`; ESC/newline sanitized via `sanitizeTermLine`. Assert with `mark()` tokens, not `✗`. Spec: run-failure-detail "Failure Classification Priority", "Coverage Headline And Hint".
- [x] 1.3.2 GREEN: create `internal/app/failure.go`: `classify(report) verdict{kind, headline, hint}` and rendering helpers. No new imports beyond existing app boundary.

### Phase 1.4: Cache, load cmd, history tail (~170 lines; depends on 1.2, 1.3)

- [x] 1.4.1 RED: tests in `internal/app/run_history_test.go` / new `rundetail_test.go`: entering history clears the map and increments `reportGen`; `ensureReportCmd` is nil with nil `Deps.Runs` or when cached/loading; `runReportMsg` with stale gen is dropped; result lands under its RunID only; tail shows `Motivo: <mark> Falló por cobertura: 66% global < 75%` only after load; tail byte-identical with nil `Runs`/no load; Succeeded/in-progress/no-report show no line. Update existing history tests that expect a nil cmd on Up/Down so they set nil `Runs` or assert the load cmd (reconcile item c). Spec: run-history "Selected Run Tail", run-failure-detail "Stale load dropped".
- [x] 1.4.2 GREEN: `internal/app/app.go`: add `runReports map[string]runReportEntry`, `reportGen`; `internal/app/rundetail.go` (create, partial): `ensureReportCmd`, `onRunReport`; `internal/app/update.go`: route `runReportMsg`; `internal/app/keys.go`: reset on history entry, Up/Down return `ensureReportCmd`; `internal/app/view.go`: tail `Motivo` line.

### Phase 1.5: Body extraction and result-screen headline (~120 lines; depends on 1.0.2, 1.3)

- [x] 1.5.1 RED: view tests for the result screen: headline above counters on Failed and Canceled; none while `Queued`/`InProgress`/`Pending`; timeout keeps `Timeout: la validación no alcanzó un estado terminal a tiempo.` and adds headline from last `m.report`; timeout with no report shows only the timeout line. Spec: validation-progress scenarios (all four).
- [x] 1.5.2 GREEN: `internal/app/view.go`: extract lines ~1204-1303 of `validationBody` into `reportSections(r)`; golden from 1.0.2 must stay green; add headline to result screen (Failed/Canceled only). Rewrite any result-screen / delta-validation pins found in 1.0.1 that legitimately change.

### Phase 1.6: Slice 1 gate (~0 lines)

- [x] 1.6.1 Run `go build ./... && go vet ./... && go test ./...` and `gofmt -l .`; `boundary_test.go` green. Commit slice 1 (conventional commit, no AI attribution). NOT committed per orchestrator instruction; commit deferred.

---

## Slice 2: StateRunDetail, toggle removal, keymap/parity/help (~900 lines)

### Phase 2.1: Detail screen and scroll (~330 lines; sequential after slice 1)

- [x] 2.1.1 RED: tests in `internal/app/rundetail_test.go`: `d` (runs>0) opens `StateRunDetail` and triggers `ensureReportCmd`; Enter never opens it; Esc returns to history with `runsCursor` unchanged; `q` quits; detail shows headline, hint, Estado, Job Id, Fase, Pick, counters, `reportSections`, report path; `Sin reporte persistido`; `En curso` with no headline; `Reporte ilegible: <path>` fallback; Succeeded success summary vs SucceededPartial amber callout; scroll clamps at 0 and at last page (`scrollWindow` table test in `window_test.go`). Spec: run-failure-detail "Full-Screen Run Detail", "Report Read-Back".
- [x] 2.1.2 GREEN: `internal/app/app.go`: add `StateRunDetail` (before `stateCount`), `detailScroll`; `internal/app/window.go`: `scrollWindow(offset, total, rows)`; `internal/app/rundetail.go`: `viewRunDetail`, `keyRunDetail`; `internal/app/keys.go`: dispatch case and `d` opens detail; `internal/app/view.go`: route state.

### Phase 2.2: Inline toggle removal and test rewrites (~120 lines; after 2.1)

- [x] 2.2.1 RED: rewrite the two `runDetail` tests (`internal/app/run_history_test.go:206-212`, `internal/app/quick_deploy_test.go:184`) to assert the new behavior (`d` opens `StateRunDetail`; after `d` then Esc no inline `Detalle:` block with Estado/Job Id/Fase/Pick). Add the "Inline toggle is gone" scenario. They fail against the current toggle.
- [x] 2.2.2 GREEN: remove `runDetail` from `internal/app/app.go` (field and comment at 466-477), `keys.go` (615, 1391), and the inline `Detalle` block in `view.go` (~1385). Remove dead code.

### Phase 2.3: Keymap, footer parity, help overlay (~180 lines; parallel-safe with 2.2 after 2.1)

- [x] 2.3.1 RED: new `StateRunDetail` fixtures (plain and sized/scrollable) in the parity harness; rows in `internal/app/footer_pins_test.go` and `internal/app/esc_q_matrix_test.go`; `?` overlay lists `↑/↓ desplazar`, `Esc volver`, `q salir`. Spec: run-failure-detail footer parity.
- [x] 2.3.2 GREEN: `internal/app/keymap.go`: `StateRunDetail`: `hint(Nav,"↑/↓","desplazar",scrollable)`, `bEsc("volver")`, `bQuit()`; ensure the help overlay picks it up.

### Phase 2.4: Pin audit closure (~40 lines; after 2.1-2.3)

- [x] 2.4.1 Re-run the 1.0.1 audit list: confirm each pinned test in `view_test.go`, delta-validation tests, history Up/Down nil-cmd tests, and the two runDetail tests is either unchanged-green or deliberately rewritten (note each rewrite in the commit body). No test is deleted merely to go green.

---

## Final Phase: verification (~0 lines)

- [x] F.1 `go build ./...`, `go vet ./...`, `go test ./...` (see MEMORY: Go PATH quirk), `gofmt -l .` must print nothing; `internal/app/boundary_test.go` green.
- [x] F.2 Diff size check: `git diff --stat main...HEAD` plus untracked files. If changed lines exceed ~2,000, STOP and report to the orchestrator before continuing; do not trim tests to fit.
- [x] F.3 Spot-check each Success Criteria item from `proposal.md`: coverage headline in tail, detail and result screen; each edge-case row has a test; Esc restores cursor; footer parity holds.

## Line estimate summary

| Phase | Est. lines |
|---|---|
| 1.0 pin audit/golden | 60 |
| 1.1 salesforce parse | 140 |
| 1.2 runs read-back | 150 |
| 1.3 classifier | 230 |
| 1.4 cache + tail | 170 |
| 1.5 body extract + result headline | 120 |
| Slice 1 subtotal | ~870 |
| 2.1 detail screen | 330 |
| 2.2 toggle removal | 120 |
| 2.3 keymap/parity | 180 |
| 2.4 audit closure | 40 |
| Slice 2 subtotal | ~670 |
| Total raw | ~1,540 |

Dependency order: 1.0 -> (1.1 || 1.2) -> 1.3 -> 1.4 -> 1.5 -> 1.6 -> 2.1 -> (2.2 || 2.3) -> 2.4 -> F. Single writer; "parallel" means order-independent, not concurrent agents.
