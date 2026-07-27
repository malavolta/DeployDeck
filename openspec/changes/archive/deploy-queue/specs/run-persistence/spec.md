# Delta for Run Persistence

## ADDED Requirements

### Requirement: Cancel Result Persisted As A Companion File

The system SHALL provide `Writer.MarkCanceled(runID, cancelRaw)`, which persists the cancel result as a `cancel.json` companion file under `.deploydeck/runs/<run-id>/`, mirroring how `Create` persists `validate.json`. `MarkCanceled` SHALL NOT write the cancel result using the `report-<NNN>.json` poll-numbering scheme.

#### Scenario: Successful cancel writes a cancel.json companion
- GIVEN an existing run directory for a `jobId`
- WHEN `MarkCanceled(runID, cancelRaw)` is called after a successful `CancelDeploy`
- THEN a `cancel.json` file is written under the run directory holding the raw cancel response (HU-012)

#### Scenario: Cancel record does not consume report numbering
- GIVEN an existing run directory with prior `report-<NNN>.json` files from polling
- WHEN `MarkCanceled` persists the cancel result
- THEN the cancel record is written as `cancel.json`, not as a new `report-<NNN>.json` entry (HU-012)

### Requirement: Canceled Status Updates The Run Record

The system SHALL update the run's persisted status in `run.json` to `Canceled` when `MarkCanceled` is called for a successful cancellation.

#### Scenario: MarkCanceled updates run status to Canceled
- GIVEN an existing run directory holding a status other than `Canceled`
- WHEN `MarkCanceled(runID, cancelRaw)` is called
- THEN the run's persisted status becomes `Canceled` (HU-012)

## Out of Scope (Deferred to HU-013)

Cancel persistence in this slice remains minimal, matching the writer's existing scope: it records only the cancel result and the status transition. Full run history browsing/listing (including canceled runs), retention/cleanup policy, and resume-by-jobId remain OUT of this change and are owned by HU-013, which extends this writer rather than rewriting it.
