# Validation Progress

Source: HU-011 (`docs/HISTORIAS.md:698-761`). Polls `sf project deploy report` to a terminal state and shows live progress.

## Requirements

### Requirement: Periodic Report Polling

The system SHALL poll `sf project deploy report --job-id <id> --target-org <alias> --json` at the configured `pollIntervalSeconds` interval while a validation job is open.

#### Scenario: Report is polled periodically
- GIVEN a valid `jobId`
- WHEN the progress screen is open
- THEN `deploy report` is queried at the configured interval

### Requirement: Live Component And Test Progress Updates

The system SHALL update displayed components and tests as the job advances.

#### Scenario: Progress updates as the job advances
- GIVEN the job advances
- WHEN a new report is polled
- THEN the TUI updates the components and tests progress

### Requirement: Metadata Errors Shown With Component Detail

The system SHALL show metadata errors with component, type, message, and — when the Salesforce CLI report provides them — file location (`fileName:lineNumber[:columnNumber]`) and `problemType`. Entries where `problemType` is `Warning` SHALL be visually labeled as warnings, distinct from `Error` entries.

#### Scenario: Metadata error shows component, type, message, and location
- GIVEN the report includes a metadata error with `fileName` and `lineNumber` (optionally `columnNumber`)
- WHEN it is displayed
- THEN it shows its component, type, message, and `fileName:lineNumber[:columnNumber]`

#### Scenario: Warning-typed component failure is labeled distinctly
- GIVEN a componentFailures entry has `problemType` = `Warning`
- WHEN it is displayed
- THEN it is visually labeled as a warning, distinguishable from `Error`-typed entries

### Requirement: Failed Tests Shown With Class And Method Detail

The system SHALL show failed tests with class, method, message, and — when the report provides a `stackTrace` — a compact, bounded stack-trace excerpt (first frame(s) only); the full trace remains reachable via the latest persisted report path (see "Latest Persisted Report Path Shown On Failure Screens").

#### Scenario: Failed tests show class, method, and message
- GIVEN the report includes failed tests
- WHEN they are displayed
- THEN each failed test shows its class, method, and message

#### Scenario: Failed test shows a compact stack-trace excerpt
- GIVEN a failed test's report entry includes a `stackTrace`
- WHEN it is displayed
- THEN a compact, bounded excerpt of the stack trace is shown alongside the class, method, and message

### Requirement: Terminal-State Detection Stops Polling

The system SHALL stop polling once the report reaches a terminal state in `{Succeeded, SucceededPartial, Failed, Canceled}`.

#### Scenario: Terminal state stops polling
- GIVEN a report reaches `Succeeded`, `SucceededPartial`, `Failed`, or `Canceled`
- WHEN that report is processed
- THEN polling stops

### Requirement: Hard Timeout And Context Cancel Bound Polling

The system SHALL bound polling with a hard timeout and a cancelable `context`, and SHALL retry transient report errors within that timeout rather than aborting immediately.

#### Scenario: Polling stops at the hard timeout
- GIVEN polling has not reached a terminal state
- WHEN the configured hard timeout elapses
- THEN polling stops via context cancellation

#### Scenario: Transient error retries within the timeout
- GIVEN a transient error from `deploy report`
- WHEN polling is still within the hard timeout
- THEN the poll retries instead of terminating the run

### Requirement: Distinct Cancel Entry Key From Polling

The system SHALL provide a distinct key (`c`) from the `ValidationPolling` screen that enters the cancel-confirmation flow (`StateCancelConfirm`), separate from the exit key (`q`).

#### Scenario: Cancel key enters cancel confirmation
- GIVEN the user is on `ValidationPolling` viewing the current run's in-progress job
- WHEN the user presses `c`
- THEN the TUI transitions to `StateCancelConfirm` (HU-012)

### Requirement: User Exit Leaves The Job Active And Resumable

The system SHALL leave the Salesforce job active when the user exits the progress screen, and SHALL keep the run resumable. This invariant applies to the exit key (`q`) only and is UNCHANGED by the separate, explicit, typed-confirmation cancel action (`c` → `StateCancelConfirm` → `CancelDeploy`): cancelling requires its own confirmed action and does not alter what `q` does.

#### Scenario: Exiting leaves the job active
- GIVEN the user exits the progress screen before a terminal state
- WHEN the exit happens
- THEN the Salesforce job keeps running and the run remains resumable

#### Scenario: Exit and cancel remain distinct actions
- GIVEN the user is on `ValidationPolling`
- WHEN the user presses `q` to exit rather than `c` to cancel
- THEN the job is left active/resumable and no cancellation is executed (HU-012)

### Requirement: Re-Attach Polling To A Non-Terminal jobId On A Later Launch

When `run-resume` offers and the user accepts resume for a run whose persisted `jobId` has not reached a terminal state, the system SHALL re-attach `deploy report` polling for that `jobId` at the configured interval, reusing the same polling, terminal-state-detection, and timeout requirements as a freshly started validation.

#### Scenario: Resumed run re-attaches to deploy report polling
- GIVEN a persisted run with a `jobId` not in `{Succeeded, SucceededPartial, Failed, Canceled}`
- WHEN the user accepts the resume offer
- THEN `deploy report` polling re-attaches for that `jobId` at the configured interval (HU-013 AC `docs/HISTORIAS.md:844`)

#### Scenario: Resumed job already terminal is not offered for polling resume
- GIVEN a persisted run whose `jobId` already reached a terminal state
- WHEN startup resume-detection runs
- THEN that run is not offered for polling re-attach (it remains browsable in history but is not resumable via polling)

### Requirement: Each Raw Report Persisted

The system SHALL save each polled raw `deploy report` response relevant to the run.

#### Scenario: Raw report is saved on each poll
- GIVEN a `deploy report` poll returns a response
- WHEN the response is processed
- THEN the raw response is saved for the run

### Requirement: Coverage Failures Show Per-Class Percentage Below Gate

The system SHALL render the coverage percentage for each class below the configured coverage gate (75%) when the report's per-class `codeCoverage[]` is available, bounded to the failing classes (or the N worst). The org-wide gate warning (identified by its empty `name`) SHALL remain visually distinguished from per-class warnings.

#### Scenario: Below-gate class shows its coverage percentage
- GIVEN the report's `codeCoverage[]` includes a class below the 75% gate
- WHEN the coverage failure is displayed
- THEN that class's coverage percentage is shown

#### Scenario: Org-wide gate warning stays distinguished from per-class warnings
- GIVEN the report includes both an org-wide coverage warning (empty `name`) and a per-class warning
- WHEN they are displayed
- THEN the org-wide warning is visually distinguished from the per-class ones

### Requirement: Latest Persisted Report Path Shown On Failure Screens

The system SHALL show the file-system path of the latest persisted `report-NNN.json` for the run on failure and result screens, replacing the prior "revisa el JSON crudo" hint that pointed to no reachable raw JSON.

#### Scenario: Failure screen shows the latest report path
- GIVEN a run reaches a failure or terminal result state with at least one persisted `report-NNN.json`
- WHEN the failure/result screen renders
- THEN it shows the file-system path of the latest persisted report, in place of the prior raw-JSON hint

### Requirement: SucceededPartial Renders An Explicit Partial-Success Callout

The system SHALL render `SucceededPartial` with a distinct header and an explicit partial-success callout on the result screen, and SHALL NOT render it as plain success.

#### Scenario: SucceededPartial shows a distinct partial-success callout
- GIVEN a run's terminal status is `SucceededPartial`
- WHEN the result screen renders
- THEN it shows a distinct header and an explicit partial-success callout, not the plain-success rendering

## Design Notes

- Visual severity follows the existing `tui-presentation` "Semantic Color And Visual Hierarchy" requirement (bright-red for Error, bright-amber/yellow for Warning and partial success, bright-green for success) — not duplicated here.
- Stack-trace excerpts and file/JSON paths render in a muted/secondary style so the error message itself stays visually prominent; org-sourced strings are sanitized of control/ANSI characters before rendering.
- Rendering stays bounded by design (compact stack excerpt, only below-gate coverage classes, capped rows with an overflow count) so the TUI does not flood on large failures.
- Tests keep asserting plain text via the existing Ascii `TestMain` convention (forced Ascii color profile).
