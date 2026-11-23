# Apply Report: run-failure-detail

Status: done (23/23 tasks). Not committed (orchestrator instruction). Final gate: `go build ./... && go vet ./... && go test ./...` all ok (cmd/deploydeck, ai, app, config, delta, exec, gate, git, github, prereq, provenance, runs, salesforce, update, version); `gofmt -l internal cmd` empty.

## Size
`git diff --shortstat -- . ':!openspec'` with new files: 22 files, +1483/-47 tracked-incl-intent-to-add. Code ~576 lines, tests + fixtures ~954 lines (budget ~2,500; used ~1,530).

## Task 1.0.1 pin audit (recorded here)
Pins touched by the change:
- (a)/(b) `validationBody` / result screen: `view_test.go` TestValidationBody_* (all unchanged and green), new golden `view_golden_test.go`.
- (c) history Up/Down nil cmd: no existing test asserted nil with a non-nil `Runs`; with nil `Runs` the cmd stays nil, so all stay green unchanged.
- (d) two `runDetail` tests rewritten (see 2.2.1): `run_history_test.go` "d toggles the expanded detail" and `quick_deploy_test.go` unaffected-by-x check.
- Footer/esc-q: `footer_pins_test.go`, `esc_q_matrix_test.go`, `footer_parity_test.go` extended (rows added, nothing deleted).

## TDD Cycle Evidence

| Task | RED observed | GREEN | Notes |
|---|---|---|---|
| 1.0.1 audit | n/a (no code) | n/a | list above |
| 1.0.2 validationBody golden | n/a (characterization; passes on pre-extraction code by design) | pass before AND after extraction | 5 models: failed-full, succeeded, failed-fallback, canceled, polling; taken BEFORE touching view.go |
| 1.1.1 / 1.1.2 ParseDeployReport | build failed: `undefined: salesforce.ParseDeployReport` | `go test ./internal/salesforce` ok | trailing text, leading noise, empty status error, mapping parity, real 66% fixture (trimmed copy in testdata) |
| 1.2.1 / 1.2.2 LatestReport | build failed: `w.LatestReport undefined`, `runs.ErrNoReport undefined` | `go test ./internal/runs` ok | newest wins, fallback+rejected, all rejected, no files, non-report files ignored, path-like runID rejected (extra) |
| 1.3.1 / 1.3.2 classifier | vet failed: `undefined: failureKind` | `-run Classify` ok | priority table, non-failure statuses, coverage regex+fallback, 0% list cap 5 + "+K más", sanitization, real fixture headline "Falló por cobertura: 66% global < 75%" |
| 1.4.1 / 1.4.2 cache, cmd, tail | vet failed: `ensureReportCmd undefined` (+ later compile errors) | `go test ./internal/app` ok | nil cases, stale gen dropped, lands under own RunID, tail Motivo only after load, nil Runs byte-identical, history entry clears map + gen++, Up/Down return load cmd |
| 1.5.1 / 1.5.2 body extraction + result headline | 3 test failures: headline missing (h=-1) on Failed/Canceled/timeout | `-run 'ResultScreen\|Golden\|ValidationBody'` ok | extraction `reportSections` kept golden green; headline via `failureBlock(classify(m.report))` before body |
| 1.6.1 slice 1 gate | n/a | full suite ok at slice 1 | commit NOT made (instruction) |
| 2.1.1 / 2.1.2 StateRunDetail + scroll | vet failed: `undefined: StateRunDetail` | `-run RunDetail\|ScrollWindow\|KeyRunHistory` ok | first run hit a stack overflow (detailChrome rendered the footer which depends on detailScrollable); fixed by counting the footer as one line |
| 2.2.1 / 2.2.2 toggle removal | the two old tests failed (`runDetail` toggle no longer driven by `d`) after 2.1 GREEN; rewritten + new "Inline toggle is gone" test | ok; `runDetail` field/uses/inline Detalle block removed | RED/GREEN ordering of 2.1 and 2.2 interleaved: `d` was repurposed in 2.1, so the old tests went red there |
| 2.3.1 / 2.3.2 keymap/parity/help | fixtures + pin rows + help test added; RED proven afterwards by temporarily making the StateRunDetail binding case return nil: 10 failing parity/pin/help subtests; restored | parity, footer pins, esc/q matrix, help overlay ok | the keymap line itself landed in 2.1 (the footer needs it to render); RED demonstrated by temporary revert, not before first write |
| 2.4.1 audit closure | n/a | full suite | no test deleted; only the two runDetail tests rewritten; all other pins unchanged-green |
| F.1 final gate | n/a | build, vet, test all ok; gofmt empty | boundary_test, footer_parity, esc/q matrix, window tests included |
| F.2 diff size | n/a | ~1,530 changed lines, under 2,500 | |
| F.3 success criteria spot-check | n/a | coverage headline in tail, detail, result screen (tests above); each edge-case row has a test; Esc restores cursor; footer parity holds | |

## Deviations
- Headline uses `mark("XX")` token, per reconciliation notes (not a literal check mark).
- Result screen also shows the coverage hint (dim) under the headline, via `failureBlock`; spec only requires the headline.
- The detail shows only the newest rejected report path as `Reporte ilegible: <path>`.
- Detail shows `Cargando reporte...` while a load is in flight (not in spec).
- `LatestReport` rejects path-like runIDs (returns ErrNoReport) as a hardening extra.
- Fixture is a trimmed copy (71 lines, paths removed, trailing CLI warning kept) of the real report, stored in both `internal/salesforce/testdata` and `internal/app/testdata`.
- Slice 1 commit not made.

## Risks
- Test runtime: internal/app 107s and internal/git 310s (pre-existing).
- `ensureReportCmd` has a value receiver and marks "loading" in a map shared across Model copies; safe because the map is replaced (never mutated across sessions) on history entry, but it relies on `New` initialising the map (nil map just disables caching).
- Comments in `run_history_test.go` about "toggles the expanded detail" may be slightly stale (cosmetic).

## Verify fix round
Addresses verify-report W1-W3, S1, S3, S4, S5. Skipped (follow-ups): S2 (in-progress record without report could fall back to the record status instead of "Sin reporte persistido"), S6 (Job Id / Phase from run.json are not sanitized; pre-existing).

| Item | RED | GREEN | Notes |
|---|---|---|---|
| W1 hint wrap | `TestResultScreen_HintWrapsAt80AndKeepsClassNames`, `TestRunDetail_HintWrapsAt80AndScrollCountsWrappedLines` failed (hint one line, >80 cols, no own-line "Clases al 0%") | pass | `wrapIndented`/`hintLines` in failure.go; `failureBlock(v, width)`; "Clases al 0%:" split into its own paragraph; width 0 stays one line (`TestResultScreen_UnsizedHintStaysOneLine`); detail scroll uses `detailLines` so wrapped lines are counted |
| W2 timeout | n/a (pin already green; spec/test were the wrong side) | pass | `TestResultScreen_TimeoutWithNonTerminalReportHasNoHeadline`; impossible fixture replaced in view_golden_test.go; validation-progress spec scenario rewritten |
| W3 spec wording | n/a (docs) | n/a | run-failure-detail spec: late load cached under own RunID; older-generation loads dropped; scenario split in two |
| S1 relative path | `TestReportPath_RelativeUnderRootAbsoluteOtherwise` failed | pass | `Model.displayPath` (ArtifactsRoot, then ProjectDir); absolute otherwise. No pinned golden changed (goldens use `Deps{}`, no roots) |
| S3 canceled dup | `TestResultScreen_CanceledShowsNameOnce` failed | pass | `validationBodyFor(skipCancel)` / `reportSections(r, skipCancel)`; line dropped only where the headline is shown (result + detail); remaining render sanitized; `validationBody()` golden unchanged |
| S4 | n/a | n/a | shared-map contract comment on `ensureReportCmd` (and `onRunReport`) |
| S5 | n/a | n/a | stale comment in run_history_test.go fixed |

Gate: build, vet, `go test ./...` green; `gofmt -l internal cmd` empty.
Note: on the result screen with no job id the hint is followed directly by "Estado:" (no blank line); pre-existing body layout, left as is.
