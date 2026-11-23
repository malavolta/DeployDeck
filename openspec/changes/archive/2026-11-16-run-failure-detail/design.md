# Design: Run Failure Detail

## Technical Approach

A pure classifier turns a `salesforce.DeployReport` into a verdict (kind, headline, hint). Three places render it: the history tail, a new `StateRunDetail` screen, and the terminal result screen. Persisted reports are read back through `runs` (file I/O) and `salesforce` (parsing). `internal/app` gets them through a `tea.Cmd`, so it never touches the filesystem or a subprocess.

## Architecture Decisions

| Decision | Option chosen | Rejected | Rationale |
|---|---|---|---|
| Read-back seam | `(*Writer).LatestReport(runID, accept func([]byte) bool) (ReportFile, []string, error)`. It scans `report-NNN.json` newest-first and reads lazily. It returns the first file `accept` takes, plus the paths it rejected, or `ErrNoReport`. | Read every report into a slice; make `runs` import `salesforce` | Polling appends one report per poll, so a run can have hundreds. Lazy scanning stops at the first good file. The callback keeps `runs` free of `salesforce`. |
| Tolerant parse | `salesforce.ParseDeployReport(raw)`: a `json.Decoder` reads the first value. If that fails, it retries from the first `{`. An empty `Status` is an error. | Change `decodeEnvelope` | `decodeEnvelope` has 6 callers. Persisted raw is `stdout + "\n" + stderr`, so trailing text is normal. An error envelope with no status gets rejected, which triggers the fallback to an older file. |
| Shared mapping | Pull the mapping at report.go:286-339 out into `toDeployReport(decoded, raw)`, used by both `ReportDeploy` and `ParseDeployReport` | Copy the mapping | One mapping only. `ReportDeploy` behavior does not change. |
| Classifier home | `internal/app/failure.go`, pure, Spanish copy | `salesforce` package | The copy is UI text. `salesforce` stays copy-free. |
| Body reuse | Pull lines 1204-1303 of `validationBody` out into `reportSections(r) string` (counters, sections, fallback). `validationBody` becomes Job + Estado + `reportSections` + path. | Rewrite the renderer | The output stays byte-identical. The detail screen reuses the same sections. |
| Cache/staleness | `m.runReports map[string]runReportEntry` keyed by RunID, plus `m.reportGen`. Entering history clears the map and does `gen++`. A `runReportMsg{runID, gen}` whose gen does not match is dropped. A loading entry blocks duplicate loads. | Track only the detail RunID | Keying by RunID means a result can never land on the wrong row. The gen check drops loads from an earlier history session. |
| Scrolling | `m.detailScroll` clamped against `listRows(m.height, chrome)`. The view uses `overflow`/`truncate` from window.go. A new helper `scrollWindow(offset, total, rows)` sits next to `window`. | Reuse the cursor-anchored `window` | Free scrolling has no cursor. The existing helpers still do the sizing and truncation. |
| Inline toggle | Remove `runDetail`. `d` opens `StateRunDetail`. Estado/Job Id/Fase/Pick move to the detail screen. | Keep both | This is the approved UX (option C). |
| Tail line | One line `  Motivo: <mark> <headline>`, shown only once an entry has loaded for the selected run | Always reserve the line | With nil `Runs` or no load, the tail stays byte-identical, so existing pins hold. |

## Data Flow

    history entry / ↑↓ / d ──→ ensureReportCmd(runID)   (nil if Deps.Runs nil or cached)
        └─ tea.Cmd: Runs.LatestReport(runID, accept=ParseDeployReport ok)
              └─→ runReportMsg{runID, gen, report, path, rejected, err}
                    └─ Update: gen match? → runReports[runID] = entry
    view: entry → classify(report) → verdict → tail | detail | (result screen uses m.report)

    StateRunHistory ──d (runs>0)──→ StateRunDetail ──Esc──→ StateRunHistory (runsCursor kept)
                                          └──q──→ quit

## Classifier

Priority: `Canceled` > metadata (ComponentFailures with non-Warning ProblemType, or NumberComponentErrors>0) > tests > coverage > `ErrorMessage` > generic. For non-Failed status: Succeeded gives `[OK] Validación exitosa`, SucceededPartial gives `[!!] Resultado parcial`, a non-terminal status gives `[i] En curso`. The coverage headline comes from a regex `(\d+)%.*?(\d+)%` on the org-wide warning (empty Name). If that does not match, it falls back to `Falló por cobertura de código`. The hint says the org average covers all org Apex classes, then lists the 0% classes (NumLocations>0): at most 5 names, then `+K más`. Every piece of org text goes through `sanitizeTermLine`. Tokens use `mark()`, which keeps output plain under the Ascii test profile.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/salesforce/report.go` | Modify | `ParseDeployReport`, `toDeployReport` |
| `internal/runs/reports.go` | Create | `LatestReport`, `ReportFile`, `ErrNoReport` |
| `internal/app/failure.go` | Create | `classify`, verdict, headline/hint rendering |
| `internal/app/rundetail.go` | Create | `viewRunDetail`, `keyRunDetail`, `ensureReportCmd`, `onRunReport`, `scrollWindow` use |
| `internal/app/app.go` | Modify | `StateRunDetail` (before `stateCount`), cache/gen/scroll fields, remove `runDetail` |
| `internal/app/view.go` | Modify | `reportSections` extraction, tail Motivo line, remove inline Detalle, result-screen headline (Failed/Canceled only, never while polling; timeout uses the last `m.report`) |
| `internal/app/keys.go` | Modify | dispatch case, `d`/↑↓ in history, reset on history entry |
| `internal/app/keymap.go` | Modify | `StateRunDetail`: `hint(Nav,"↑/↓","desplazar",scrollable)`, `bEsc("volver")`, `bQuit()` |
| `internal/app/update.go` | Modify | route `runReportMsg` |
| `internal/app/window.go` | Modify | `scrollWindow` |

## Testing Strategy (Strict TDD, RED first)

| Layer | What | Approach |
|---|---|---|
| salesforce | trailing text, leading noise, empty status, mapping parity with `ReportDeploy` | table tests on raw fixtures |
| runs | newest wins, corrupt newest falls back, all rejected, no files gives `ErrNoReport`, files that are not reports are ignored | `t.TempDir()` |
| app/failure | every priority row, coverage regex plus fallback, 0% list cap, sanitization of ESC/newline | pure table |
| app/view | `validationBody` pinned byte-identical (golden taken before the extraction), tail line, result headline on Failed/Canceled/timeout, none while polling | view tests |
| app/flow | `d` opens detail, Esc keeps cursor, stale gen dropped, nil Runs gives nil cmd, scroll clamps | Update-driven |
| parity | new state fixture plus a sized, scrollable fixture; rows in `footer_pins_test` and `esc_q_matrix_test`; `?` overlay lists the bindings | existing harnesses |
| boundary | `boundary_test.go` stays green | no new imports in app |

Existing `runDetail` tests (`run_history_test.go:206-212`, `quick_deploy_test.go:184`) get rewritten to cover the new screen.

## Threat Matrix

N/A: no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. The new I/O is read-only, on local files under `runDir`, and the RunIDs come from the records `List` already returns.

## Migration / Rollout

No migration required. The persisted format does not change. Two slices: (1) seams, classifier, tail, result headline; (2) the detail screen plus removal of the inline toggle.

## Revised Forecast

Production ~560 lines, tests ~720, so ~1,280 raw. Adjusted by 1.6x (the historical undercount) that is ~1,700 at worst. This is within the approved budget of ~2,000.

## Open Questions

- None blocking. Assumption: `mark()` tokens replace the literal `✗` from the UX sketch, to stay consistent with the existing screens.
