# Delta for Validation Cancel

## MODIFIED Requirements

### Requirement: Failed Cancel Leaves The Run Untouched

The system SHALL show an error and SHALL NOT mark the run as canceled when `CancelDeploy` fails, and SHALL persist the raw failure response as a companion file for the run (see `run-persistence`).
(Previously: showed an error and left the run unmarked, but the raw failure response was discarded rather than persisted.)

#### Scenario: Cancel failure shows an error and leaves the run unmarked
- GIVEN cancellation is confirmed
- WHEN `CancelDeploy` fails
- THEN an error is shown and the run is NOT marked as canceled (HU-012)

#### Scenario: Failed cancel persists the raw response
- GIVEN cancellation is confirmed
- WHEN `CancelDeploy` fails, other than the already-terminal outcome below
- THEN the raw failure response is persisted as a companion file for the run

## ADDED Requirements

### Requirement: Already-Terminal Cancel Classified As A Friendly Outcome

The system SHALL classify a cancel response shaped like `CannotCancelDeployPre` (the job already reached a terminal state on Salesforce) via a package-level sentinel, and SHALL present this to the user as a friendly informational outcome rather than a raw CLI error. The run's persisted state SHALL NOT be corrupted or altered by this outcome.

#### Scenario: Cancel on an already-finished job shows a friendly message
- GIVEN the current run's job has already reached a terminal state on Salesforce
- WHEN cancellation is confirmed and `CancelDeploy` returns a `CannotCancelDeployPre`-shaped response
- THEN the system classifies it via the already-terminal sentinel and shows a friendly informational message, not a raw CLI error (D6)

#### Scenario: Already-terminal outcome does not corrupt the run
- GIVEN the already-terminal cancel outcome above
- WHEN it is processed
- THEN the run's persisted state is left intact and unchanged
