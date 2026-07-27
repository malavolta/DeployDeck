# Delta for Validation Progress

Source: HU-011 (`docs/HISTORIAS.md:698-761`). Polls `sf project deploy report` to a terminal state and shows live progress.

## ADDED Requirements

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

The system SHALL show metadata errors with component, type, and message when present in a report.

#### Scenario: Metadata errors show component, type, and message
- GIVEN the report includes metadata errors
- WHEN they are displayed
- THEN each error shows its component, type, and message

### Requirement: Failed Tests Shown With Class And Method Detail

The system SHALL show failed tests with class, method, and message when present in a report.

#### Scenario: Failed tests show class, method, and message
- GIVEN the report includes failed tests
- WHEN they are displayed
- THEN each failed test shows its class, method, and message

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

### Requirement: User Exit Leaves The Job Active And Resumable

The system SHALL leave the Salesforce job active when the user exits the progress screen, and SHALL keep the run resumable.

#### Scenario: Exiting leaves the job active
- GIVEN the user exits the progress screen before a terminal state
- WHEN the exit happens
- THEN the Salesforce job keeps running and the run remains resumable

### Requirement: Each Raw Report Persisted

The system SHALL save each polled raw `deploy report` response relevant to the run.

#### Scenario: Raw report is saved on each poll
- GIVEN a `deploy report` poll returns a response
- WHEN the response is processed
- THEN the raw response is saved for the run
