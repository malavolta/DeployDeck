# Deploy Queue Specification

## Purpose

Give developers operational visibility of the active `DeployRequest` queue (pending/in-progress validations and deploys) in the shared sandbox before starting their own validation (HU-009).

## Requirements

### Requirement: List Active Deploy Queue Via Tooling API

The system SHALL query Salesforce `DeployRequest` records via Tooling API using `sf data query --use-tooling-api --json`, filtered to `Status IN ('Pending','InProgress')` ordered by `CreatedDate ASC`, and SHALL parse the envelope's `result.records[]` shape with fields `Id, Status, CheckOnly, CreatedDate, StartDate, CompletedDate, CreatedBy.Name` plus component/test progress counters.

#### Scenario: Queue is fetched and ordered by creation date
- GIVEN active `DeployRequest` jobs exist in the sandbox
- WHEN `ListDeployQueue` queries the queue
- THEN records are returned ordered by `CreatedDate` ascending (HU-009)

#### Scenario: CheckOnly distinguishes validation from deploy
- GIVEN a queue record has `CheckOnly = true`
- WHEN the queue is displayed
- THEN the record is shown as a validation, not a real deploy (HU-009)

### Requirement: QueueReview Is A Real Stop Between Package Confirm And Validation Start

The system SHALL present a `QueueReview` screen after package confirmation and before validation start, showing each queued job's user, status, creation date, progress, and elapsed time.

#### Scenario: QueueReview shows queue details per job
- GIVEN the queue query succeeds with jobs present
- WHEN `QueueReview` renders
- THEN each job shows user, status, date, progress, and elapsed time (HU-009)

#### Scenario: Own job is highlighted with approximate position
- GIVEN the current run's job is present in the queue
- WHEN `QueueReview` renders
- THEN the own job is highlighted and shows its approximate queue position (HU-009)

#### Scenario: Own job absent lists the rest without highlight
- GIVEN the current run's job is NOT present in the queue
- WHEN `QueueReview` renders
- THEN the rest of the queue is listed without an own-job highlight (HU-009)

#### Scenario: Empty queue shows a clear empty state
- GIVEN no `Pending` or `InProgress` jobs exist
- WHEN `QueueReview` renders
- THEN a clear empty-state message is shown (HU-009)

### Requirement: Non-Blocking Degrade On Query Failure

The system SHALL distinguish a Tooling-API-permission failure from a generic query failure. On a permission failure, it SHALL warn the user and continue the flow directly to `ValidationStart` without the queue view. On a generic failure, it SHALL show an actionable error and SHALL NOT necessarily abort the flow.

#### Scenario: Permission failure skips the queue non-blockingly
- GIVEN the query fails because the profile lacks Tooling API permission
- WHEN `ListDeployQueue` is called
- THEN a warning is shown and the flow continues to `ValidationStart` without the queue (HU-009)

#### Scenario: Generic query failure shows an actionable error without aborting
- GIVEN the query fails for a reason unrelated to permissions
- WHEN `ListDeployQueue` is called
- THEN an actionable error is shown and the flow is not necessarily aborted (HU-009)
