# Delta for Deploy Queue

## MODIFIED Requirements

### Requirement: List Active Deploy Queue Via Tooling API

The system SHALL query Salesforce `DeployRequest` records via Tooling API using `sf data query --use-tooling-api --json`, filtered to `Status IN ('Pending','InProgress')` ordered by `CreatedDate ASC`, and SHALL parse the envelope's `result.records[]` shape with fields `Id, Status, CheckOnly, CreatedDate, StartDate, CompletedDate, CreatedBy.Name, StateDetail, ErrorMessage, ErrorStatusCode` plus component/test progress counters.
(Previously: SOQL and parsing omitted `StateDetail`, `ErrorMessage`, and `ErrorStatusCode`.)

#### Scenario: Queue is fetched and ordered by creation date
- GIVEN active `DeployRequest` jobs exist in the sandbox
- WHEN `ListDeployQueue` queries the queue
- THEN records are returned ordered by `CreatedDate` ascending (HU-009)

#### Scenario: CheckOnly distinguishes validation from deploy
- GIVEN a queue record has `CheckOnly = true`
- WHEN the queue is displayed
- THEN the record is shown as a validation, not a real deploy (HU-009)

### Requirement: QueueReview Is A Real Stop Between Package Confirm And Validation Start

The system SHALL present a `QueueReview` screen after package confirmation and before validation start, showing each queued job's user, status, creation date, progress, and elapsed time. For a queued entry that reports an error — `ErrorMessage` or `ErrorStatusCode` non-empty, e.g. an `InProgress` deploy that is failing while still occupying the active queue — it SHALL additionally show that error detail, with `StateDetail` rendered as context when present. The queue query remains scoped to the ACTIVE queue (`Pending`/`InProgress`): terminal `Failed` jobs are structurally not part of this view, and progress-only `StateDetail` display for non-errored entries is out of scope (live-progress UX, not an error gap).
(Previously: rendered user, status, date, progress, and elapsed time only; errored entries showed no error detail and `StateDetail` was neither queried nor rendered.)

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

#### Scenario: Erroring queue entry shows its error detail
- GIVEN a queue record has `Status` = `InProgress` with `ErrorMessage` (or `ErrorStatusCode`) populated — a deploy failing while still in the active queue
- WHEN `QueueReview` renders that row
- THEN the row shows the job's error detail (D8)

#### Scenario: Erroring entry renders StateDetail as context
- GIVEN a queue record has `ErrorMessage` populated and a non-empty `StateDetail`
- WHEN `QueueReview` renders that row
- THEN the row shows the error detail with the `StateDetail` value as context

#### Scenario: Non-errored entry shows no StateDetail line
- GIVEN a queue record has `Status` = `InProgress`, a non-empty `StateDetail`, and empty `ErrorMessage`/`ErrorStatusCode`
- WHEN `QueueReview` renders that row
- THEN no error-detail or StateDetail line is rendered (progress-only display is out of scope)
