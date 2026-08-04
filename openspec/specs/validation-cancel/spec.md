# Validation Cancel Specification

## Purpose

Let a developer cancel their own in-progress validation to free the shared sandbox queue, without ever touching another user's job (HU-012).

## Requirements

### Requirement: Cancel Deploy Via CLI

The system SHALL cancel a Salesforce deploy/validation job by running `sf project deploy cancel --job-id <id> --target-org <alias>` through `CancelDeploy`.

#### Scenario: Cancel invokes the CLI with the run's jobId
- GIVEN the current run's `jobId`
- WHEN cancellation is confirmed
- THEN `CancelDeploy` runs `sf project deploy cancel` with that `jobId` and the target org (HU-012)

### Requirement: Cancel Is Restricted To The Current Run's Own Job

The system SHALL enable the cancel action only for the job belonging to the current run, and SHALL NOT offer a cancel action for another user's job.

#### Scenario: A foreign job never offers a cancel action
- GIVEN a queue entry belongs to another user's job
- WHEN the queue or polling screen is displayed
- THEN no cancel action is offered for that job (HU-012)

### Requirement: Typed Confirmation Required Before Cancelling

The system SHALL require the user to type the exact literal `CANCELAR` in a `StateCancelConfirm` screen, reachable from `ValidationPolling`, before executing `CancelDeploy`.

#### Scenario: Correct typed confirmation triggers cancellation
- GIVEN the user is on `StateCancelConfirm`
- WHEN the user types the exact literal `CANCELAR` and confirms
- THEN `CancelDeploy` is executed for the current run's job (HU-012)

#### Scenario: Declining or mismatched input does not trigger cancellation
- GIVEN the user is on `StateCancelConfirm`
- WHEN the user does not confirm, or the typed text does not exactly match `CANCELAR`
- THEN cancellation is not executed and the run remains untouched (HU-012)

### Requirement: Successful Cancel Marks The Run Canceled

The system SHALL transition the run to `Canceled` and persist the cancel result when `CancelDeploy` succeeds.

#### Scenario: Confirmed cancel moves the run to Canceled
- GIVEN a confirmed cancellation for the current run's in-progress job
- WHEN `CancelDeploy` succeeds
- THEN the run moves to `Canceled` and the cancel result is persisted (HU-012)

### Requirement: Failed Cancel Leaves The Run Untouched

The system SHALL show an error and SHALL NOT mark the run as canceled when `CancelDeploy` fails, and SHALL persist the raw failure response as a companion file for the run (see `run-persistence`).

#### Scenario: Cancel failure shows an error and leaves the run unmarked
- GIVEN cancellation is confirmed
- WHEN `CancelDeploy` fails
- THEN an error is shown and the run is NOT marked as canceled (HU-012)

#### Scenario: Failed cancel persists the raw response
- GIVEN cancellation is confirmed
- WHEN `CancelDeploy` fails, other than the already-terminal outcome below
- THEN the raw failure response is persisted as a companion file for the run

### Requirement: Already-Terminal Cancel Classified As A Friendly Outcome

The system SHALL classify a cancel response shaped like `CannotCancelDeployPre` (the job already reached a terminal state on Salesforce) via a package-level sentinel, and SHALL present this to the user as a friendly informational outcome rather than a raw CLI error. The run's persisted state SHALL NOT be corrupted or altered by this outcome.

#### Scenario: Cancel on an already-finished job shows a friendly message
- GIVEN the current run's job has already reached a terminal state on Salesforce
- WHEN cancellation is confirmed and `CancelDeploy` returns a `CannotCancelDeployPre`-shaped response
- THEN the system classifies it via the already-terminal sentinel and shows a friendly informational message, not a raw CLI error

#### Scenario: Already-terminal outcome does not corrupt the run
- GIVEN the already-terminal cancel outcome above
- WHEN it is processed
- THEN the run's persisted state is left intact and unchanged
