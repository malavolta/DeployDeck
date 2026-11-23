# Proposal: Run Failure Detail

Delivery phase: **Fase 3 Validacion** (EPICA.md Plan De Entrega).

## Intent

A validation failed on org-wide Apex coverage (66% < 75%) and the TUI never said why. Failure sections render only on the live screen, below counters. History `d` shows Estado/Job Id/Fase/Pick. Nothing reads persisted `report-NNN.json` back. Users must see **why** a run failed, both live and later from history.

## Scope

### In Scope (UX option C, user-approved)
- One-line motivo in the history "Run seleccionado" tail (e.g. `✗ Falló por cobertura: 66% global < 75%`).
- `d` on history opens a full-screen scrollable run-detail screen: headline, hint, Job/Estado/counters, failure sections reused from `validationBody`, report path. Footer `↑/↓ desplazar  Esc volver  q salir`. Esc returns to history with the cursor kept.
- Same headline on top of the terminal validation result screen.
- Pure failure classifier, priority: **canceled > metadata > tests > coverage > org error > generic**.
- Coverage hint: the org-wide average includes **all org Apex classes, not just the ones in this run**; list 0% classes. Org text sanitized.
- Read-back seams: `runs.(*Writer).LatestReport`, `salesforce.ParseDeployReport` (first JSON value, mapping shared with `ReportDeploy`), async load cached by RunID.

### Out of Scope
- Changing report persistence format or retention.
- Re-fetching reports from Salesforce for old runs.
- Coverage remediation suggestions beyond the hint.

## Edge Cases

| Case | Behavior |
|------|----------|
| No report (dry run / no jobId) | `Sin reporte persistido` |
| Succeeded | Success summary, no failure headline |
| SucceededPartial | Amber partial callout |
| In progress | `En curso` |
| Corrupt report | `Reporte ilegible: <path>`, never blocks; fall back to the previous parseable report-NNN |
| Trailing non-JSON text | Tolerated (first JSON value parsed) |
| Canceled | Canceled headline (highest priority) |
| Timeout | Headline from the last known report; status shown as-is |
| Stale async load | Dropped by RunID |

## Capabilities

### New Capabilities
- `run-failure-detail`: failure classification, headline/hint copy, full-screen run-detail screen, report read-back.

### Modified Capabilities
- `run-history`: `d` opens the full-screen detail (replaces the inline toggle); tail shows motivo.
- `validation-progress`: terminal result screen gains the headline.
- `run-persistence`: latest-report read-back with fallback.

## Approach

One pure renderer, shared by history tail, detail screen, and result screen. Loading happens through a `tea.Cmd` plus `runReportMsg`. No external commands from `internal/app`.

## Affected Areas

| Area | Impact |
|------|--------|
| `internal/app/view.go`, `keys.go`, `keymap.go`, `app.go`, `window.go` | Modified |
| `internal/runs/writer.go` | Modified (reader) |
| `internal/salesforce/client.go` | Modified (parser extraction) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Coverage warning wording changes | Med | Fallback to generic coverage headline |
| Footer parity breaks | Med | New state in `screenBindings` + parity test |
| Size overrun | Med | Budget approved ~2,000 lines |

## Rollback Plan

Read-only feature: it never touches Git state or sandboxes. To roll back, revert the commits. Persisted reports are unchanged.

## Success Criteria

- [ ] Failed coverage run shows `Falló por cobertura: 66% global < 75%` in history tail, detail, and result screen.
- [ ] Every edge-case row is covered by a test.
- [ ] Esc from detail restores history cursor; footer parity holds.
