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

The system SHALL show an error and SHALL NOT mark the run as canceled when `CancelDeploy` fails.

#### Scenario: Cancel failure shows an error and leaves the run unmarked
- GIVEN cancellation is confirmed
- WHEN `CancelDeploy` fails
- THEN an error is shown and the run is NOT marked as canceled (HU-012)
