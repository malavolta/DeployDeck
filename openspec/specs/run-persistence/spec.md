# Run Persistence

Source: HU-010 (`docs/HISTORIAS.md:659`) and HU-011 (`docs/HISTORIAS.md:718`) acceptance criteria that require `.deploydeck/runs/`. MINIMAL slice scoped to this change; full lifecycle belongs to HU-013.

## Requirements

### Requirement: Minimal Run Record On jobId Receipt

The system SHALL create `.deploydeck/runs/<run-id>/` as soon as a validation `jobId` is received, and SHALL hold, at minimum, the `jobId`, the current status, and the raw validate/report JSON for that run.

#### Scenario: jobId receipt creates the run directory
- GIVEN a validation call returns a `jobId`
- WHEN the `jobId` is received
- THEN `.deploydeck/runs/<run-id>/` is created holding the `jobId`, current status, and raw validate JSON

#### Scenario: Report polling updates the persisted status and raw JSON
- GIVEN an existing run directory for a `jobId`
- WHEN a new `deploy report` is polled
- THEN the run's status and raw report JSON are updated on disk

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

This slice is a minimal writer only. Full run history browsing/listing, retention/cleanup policy, and resume-by-jobId are explicitly OUT of this change and are owned by HU-013, which extends this writer rather than rewriting it.
