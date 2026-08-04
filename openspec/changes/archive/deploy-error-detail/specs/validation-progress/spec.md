# Delta for Validation Progress

## MODIFIED Requirements

### Requirement: Metadata Errors Shown With Component Detail

The system SHALL show metadata errors with component, type, message, and — when the Salesforce CLI report provides them — file location (`fileName:lineNumber[:columnNumber]`) and `problemType`. Entries where `problemType` is `Warning` SHALL be visually labeled as warnings, distinct from `Error` entries.
(Previously: showed only component, type, and message; `lineNumber`/`columnNumber`/`fileName`/`problemType` were parsed by the CLI but discarded and never rendered.)

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
(Previously: showed only class, method, and message; `stackTrace` was parsed by the CLI's JSON but never captured or rendered.)

#### Scenario: Failed tests show class, method, and message
- GIVEN the report includes failed tests
- WHEN they are displayed
- THEN each failed test shows its class, method, and message

#### Scenario: Failed test shows a compact stack-trace excerpt
- GIVEN a failed test's report entry includes a `stackTrace`
- WHEN it is displayed
- THEN a compact, bounded excerpt of the stack trace is shown alongside the class, method, and message

## ADDED Requirements

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

- Visual severity follows the existing `tui-presentation` "Semantic Color And Visual Hierarchy" requirement (bright-red for Error, bright-amber/yellow for Warning and partial success, bright-green for success) — not duplicated here; new content added by this delta must be colored per that existing rule.
- Stack-trace excerpts and file/JSON paths render in a muted/secondary style so the error message itself stays visually prominent.
- Rendering stays bounded by design (compact stack excerpt, only below-gate coverage classes) so the TUI does not flood on large failures.
- Tests keep asserting plain text via the existing Ascii `TestMain` convention (forced Ascii color profile) — a testing convention, not a new requirement.
