# Run Failure Detail Specification

## Purpose

Explain WHY a validation run failed, live and later from history: pure classification, headline/hint copy, report read-back, and a full-screen scrollable run-detail screen.

## Requirements

### Requirement: Failure Classification Priority

A pure classifier MUST map a parsed report to one cause with priority: canceled > metadata > tests > coverage > org error > generic. It MUST NOT emit a failure headline for `Succeeded`/`SucceededPartial`, nor while the status is non-terminal (`Queued`, `InProgress`, `Pending`).

#### Scenario: Priority resolves overlapping causes
- GIVEN a Failed report with a metadata error and a failed test
- WHEN it is classified
- THEN the cause is metadata

#### Scenario: Canceled outranks everything
- GIVEN a report with status `Canceled` and failed tests
- WHEN it is classified
- THEN the cause is canceled

#### Scenario: Polling shows no headline
- GIVEN the latest status is `Queued`, `InProgress` or `Pending`
- WHEN the headline is requested
- THEN none is produced

### Requirement: Coverage Headline And Hint

For a coverage cause the headline MUST read `✗ Falló por cobertura: 66% global < 75%` (values parsed from the org-wide warning); if unparseable it MUST fall back to a generic coverage headline. The hint MUST state that the org-wide average includes all org Apex classes, not just those in this run, and list the 0% classes. Org text MUST be sanitized of control/ANSI characters.

#### Scenario: Org-wide coverage failure
- GIVEN a Failed report whose org-wide warning is 66% against a 75% gate
- WHEN the headline renders
- THEN it contains `Falló por cobertura: 66% global < 75%` and the all-org-classes hint

#### Scenario: Warning wording unrecognized
- GIVEN a coverage warning whose text cannot be parsed
- WHEN the headline renders
- THEN a generic coverage headline is shown without error

### Requirement: Report Read-Back Is Corruption-Tolerant

The detail MUST be built from the latest persisted report, loaded asynchronously and cached by RunID; a late load result MUST be cached under its own RunID and MUST NOT alter the currently selected run's view, and a load result from a previous history session (older generation) MUST be dropped. Nothing external MAY be executed.

#### Scenario: No report
- GIVEN a run without a jobId or persisted report
- WHEN the detail is shown
- THEN it shows `Sin reporte persistido`

#### Scenario: Run in progress
- GIVEN a run whose status is non-terminal
- WHEN the detail is shown
- THEN it shows `En curso` and no failure headline

#### Scenario: Corrupt latest report
- GIVEN the newest report is unreadable and an older one parses
- WHEN the detail loads
- THEN it uses the older report and shows `Reporte ilegible: <path>` without blocking

#### Scenario: Late load cached under its own run
- GIVEN a load for run A is pending and the cursor moves to run B
- WHEN A's result arrives
- THEN it is cached under A's RunID and B's view is unchanged

#### Scenario: Previous-session load dropped
- GIVEN a load was fired in an earlier history session
- WHEN its result arrives after history was re-entered
- THEN it is discarded

### Requirement: Full-Screen Run Detail

`d` on history MUST open a full-screen detail showing headline, hint, Estado, Job Id, Fase, Pick, counters, failure sections (shared renderer) and the report path. It MUST scroll with `↑/↓`, clamped at both ends. Footer MUST be `↑/↓ desplazar`, `Esc volver`, `q salir` plus `? ayuda`, with parity between footer and acting keys. `Esc` MUST return to history with the cursor unchanged; `q` MUST quit.

#### Scenario: Scroll is clamped
- GIVEN content taller than the window, scrolled to the top
- WHEN the user presses `↑`, then `↓` past the end
- THEN the offset never goes below 0 or beyond the last page

#### Scenario: Esc preserves the cursor
- GIVEN history with the cursor on row 5, detail open
- WHEN the user presses `Esc`
- THEN history shows with the cursor on row 5

#### Scenario: Succeeded and partial
- GIVEN a `Succeeded` run, and separately a `SucceededPartial` run
- WHEN each detail is shown
- THEN the first shows a success summary with no failure headline, the second an amber partial callout
