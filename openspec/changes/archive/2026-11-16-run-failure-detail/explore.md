# Explore: run-failure-detail

## Problem
A validation failed in Salesforce (org-wide Apex coverage 66% < 75%) but the reason was never visible in the TUI.
- `validationBody()` (internal/app/view.go:1197) renders failure sections, but only on the live/terminal validation screen, below the counters, unwindowed.
- Run history `d` (view.go:1385) shows only Estado/Job Id/Fase/Pick.
- Reports are persisted as `.deploydeck/runs/<runId>/report-NNN.json` (raw stdout+stderr, may carry trailing CLI warning text) but nothing reads them back; `m.reportPath` is in-memory only.

## Current-state map
- Renderer helpers: belowGateCoverage (1116), hasCoverageDetail (1145), hasStructuredFailureDetail (1161), coverageMaxRows=10, coverageGatePercent=75.
- History: viewRunHistory 1320-1405, windowedRows/listRows (window.go:49, min 3 rows), keyRunHistory keys.go:1378, historyBindings keymap.go:250, runDetail app.go:477 reset keys.go:614.
- Persistence: runs.Writer.AppendReport (writer.go:225), no report reader; Deps.Runs is concrete *runs.Writer, may be nil.
- Parsing: salesforce decodeEnvelope (client.go:133) uses json.Unmarshal on the whole buffer and fails on trailing text; envelope->DeployReport mapping inline in ReportDeploy (286-339).

## Failure classification (new, pure)
Priority: cancelled > metadata > tests > coverage > org error > generic. Coverage headline "Falló por cobertura: 66% global < 75%" parsed from the warning with graceful fallback; hint notes the average spans all org Apex classes and lists the 0% classes. All org text sanitized.

## UX options
- A: one-line motivo + inline report under `d` in history. Cheapest; tail growth squeezes the list to 3 rows; no scroll.
- B: dedicated full-screen run detail (new state, scrollable, d/Enter opens, Esc returns with cursor kept).
- C (recommended): one-line motivo in history + full-screen detail (B) + headline on top of the live/terminal Failed screen; one pure renderer shared.

## Edge cases
No report (dry run / no jobId) -> "Sin reporte persistido"; Succeeded -> success summary; Partial -> amber callout; in-progress -> "En curso"; corrupt -> "Reporte ilegible: <path>" without blocking; trailing non-JSON tolerated; latest report-NNN wins, fallback to previous if unparseable; stale async load messages dropped by RunID.

## Seams
- `runs.(*Writer).LatestReport(runID) (raw, path, err)`
- `salesforce.ParseDeployReport(raw) (DeployReport, error)` (first JSON value, shared mapping with ReportDeploy)
- Load via tea.Cmd + runReportMsg, cached by RunID.

## Forecast (raw, x1.6 historical undercount)
Slice 1 (motivo + headline + inline d): ~840 (~1350). Slice 2 (full-screen detail): ~400 more. Both: ~1240 (~2000).
