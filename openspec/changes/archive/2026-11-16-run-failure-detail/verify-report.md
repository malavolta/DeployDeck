# Verification Report — run-failure-detail (round 1)

**Verdict**: PASS WITH WARNINGS (0 CRITICAL, 3 WARNING, 6 SUGGESTION)

## Evidence
- build, vet, `go test ./...` (fresh run of app/runs/salesforce) green; gofmt clean; boundary test green.
- 23/23 tasks done; every spec scenario has a passing covering test.
- UX rendered with real agenforce-demo data at 80x24 and 120x40: reason above the fold on history (Motivo line), detail and result screens; Esc keeps cursor; footers correct.

## Warnings
- W1 (UX): coverage hint is one ~140-char line; truncated on the detail screen (`rundetail.go:187`) and cut by the renderer on the result screen (`failure.go:142-148`), hiding the 0% class list at 80 cols.
- W2: timeout-with-last-report headline cannot occur at runtime (timeout only while non-terminal, classify returns none); the scenario passes only via an impossible fixture (`view_golden_test.go:101-107`). Spec contradicts itself.
- W3: spec says stale loads for other RunIDs are dropped; code caches under the result's own RunID (design intent). Wording fix.

## Suggestions
- S1: report path truncated at 80 cols; show it relative to the project dir.
- S2: in-progress record without report shows "Sin reporte persistido"; could fall back to record status.
- S3: Canceled result shows headline and the older "Cancelado por:" line (latter unsanitized, pre-existing).
- S4: document the shared-map contract in ensureReportCmd.
- S5: stale "toggles the expanded detail" comments in run_history_test.go.
- S6: Job Id/Phase from run.json not sanitized (pre-existing behavior).

## Round 2 (fix round)

**Resolutions** (details in apply-report.md, "Verify fix round"):
- W1 (coverage hint truncated on detail and result screens): fixed. The hint wraps at 80 columns with "Clases al 0%" on its own line; detail scroll counts wrapped lines.
- W2 (timeout-with-report headline impossible at runtime): fixed. The spec scenario is rewritten to the non-terminal timeout case; the impossible fixture was replaced.
- W3 (spec said loads for other RunIDs are dropped; code caches under the result's own RunID): fixed in the spec wording. A late load is cached under its own RunID; only older-generation loads are dropped.
- S1 (report path truncated at 80 cols): fixed. The path is shown relative to the artifacts root or project dir, absolute otherwise.
- S3 (duplicated Canceled name line): fixed. The "Cancelado por:" line is dropped only where the headline is shown.
- S4 (shared-map contract undocumented): fixed with a comment on `ensureReportCmd` and `onRunReport`.
- S5 (stale "toggles the expanded detail" comments): fixed.
- S2 (in-progress record without report shows "Sin reporte persistido"): follow-up, not fixed.
- S6 (Job Id / Phase from run.json not sanitized, pre-existing): follow-up, not fixed.

**Gate**: `go build ./...`, `go vet ./...`, `go test ./...` and `gofmt -l internal cmd` are green per the apply-report fix-round evidence. The archive phase did not re-run them.

**Verdict after round 2**: PASS (0 CRITICAL, 0 open WARNING; S2 and S6 open as follow-ups).
