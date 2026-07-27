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

### Requirement: Additive Record Growth With Backward Compatibility

The `Record` SHALL grow additively with `PickIndex`, `PickTotal`, `CurrentCommit`, `Phase`, and `Commits []string`, with NO `SchemaVersion` bump. Older `run.json` files written before these fields existed SHALL still `Load` cleanly, with the new fields zero-valued.

#### Scenario: Prior-slice run.json still loads
- GIVEN a `run.json` written before `PickIndex`/`PickTotal`/`CurrentCommit`/`Phase`/`Commits` existed
- WHEN it is loaded
- THEN it loads without error and the new fields are zero-valued

#### Scenario: New fields persist through a full round trip
- GIVEN a run whose record has `PickIndex`, `PickTotal`, `CurrentCommit`, `Phase`, and `Commits` set
- WHEN the record is written and reloaded
- THEN all five fields round-trip unchanged

### Requirement: List Runs Newest First

`Writer.List()` SHALL scan `.deploydeck/runs/*/run.json` and return all records ordered newest first by `CreatedAt`.

#### Scenario: List returns runs sorted newest first
- GIVEN three persisted runs with different `CreatedAt` values
- WHEN `List()` is called
- THEN the returned records are ordered newest first

### Requirement: Load A Single Run By ID

`Writer.Load(runID)` SHALL return the persisted `Record` for a given run ID.

#### Scenario: Load returns the persisted record
- GIVEN a persisted run directory for `runID`
- WHEN `Load(runID)` is called
- THEN the matching `Record` is returned

#### Scenario: Load a non-existent run ID errors
- GIVEN no run directory exists for `runID`
- WHEN `Load(runID)` is called
- THEN an error is returned

### Requirement: Prune Removes Runs Outside The Retention Window

`Writer.Prune(keepLast, keepDays, now)` SHALL delete the on-disk directory for every run selected as prunable (per the `run-retention` selection rule) and SHALL return the list of removed run IDs, leaving all other runs untouched.

#### Scenario: Prune removes only the selected run directories
- GIVEN a set of runs where some are outside both `keepLast` and `keepDays`
- WHEN `Prune(keepLast, keepDays, now)` runs
- THEN only those runs' directories are deleted and their IDs are returned

## Extended by HU-013

Full run history browsing/listing, retention/cleanup policy, and resume-by-jobId are now owned by HU-013, which extends this writer. See `run-history`, `run-retention`, and `run-resume` capabilities.
