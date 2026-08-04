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

### Requirement: PR URL Recorded On The Run

The `Record` SHALL grow additively with an optional `PRUrl` field
(`json:"prUrl,omitempty"`), with NO `SchemaVersion` bump, and the system
SHALL provide `Writer.MarkPRCreated(runID, prURL)`, which persists the PR URL
to the run's `run.json`, mirroring the method shape of `MarkCanceled`. Older
`run.json` files written before `PRUrl` existed SHALL still `Load` cleanly
with `PRUrl` zero-valued.

#### Scenario: MarkPRCreated persists the PR URL
- GIVEN an existing run directory for a `runID`
- WHEN `MarkPRCreated(runID, prURL)` is called after a successful
  `gh pr create` (HU-014 AC5)
- THEN the run's persisted `PRUrl` in `run.json` is updated to the given URL

#### Scenario: Prior-slice run.json still loads without PRUrl
- GIVEN a `run.json` written before `PRUrl` existed
- WHEN it is loaded
- THEN it loads without error and `PRUrl` is zero-valued

#### Scenario: PRUrl round-trips through a full write/reload
- GIVEN a run whose record has `PRUrl` set
- WHEN the record is written and reloaded
- THEN `PRUrl` round-trips unchanged

### Requirement: SourceRunID Recorded On The Run

The `Record` SHALL grow additively with an optional `SourceRunID` field
(`json:"sourceRunId,omitempty"`), with NO `SchemaVersion` bump, set at
NEW-run creation time when the run is a re-promotion of a prior run
(HU-016). Older `run.json` files written before `SourceRunID` existed SHALL
still `Load` cleanly, with `SourceRunID` zero-valued.

#### Scenario: SourceRunID persisted at creation for a re-promotion
- GIVEN a new run is created as a re-promotion of a prior run
- WHEN the run record is written
- THEN `SourceRunID` holds the prior run's ID (HU-016 AC `docs/HISTORIAS.md:1065`)

#### Scenario: Prior-slice run.json still loads without SourceRunID
- GIVEN a `run.json` written before `SourceRunID` existed
- WHEN it is loaded
- THEN it loads without error and `SourceRunID` is zero-valued

#### Scenario: SourceRunID round-trips through a full write/reload
- GIVEN a run whose record has `SourceRunID` set
- WHEN the record is written and reloaded
- THEN `SourceRunID` round-trips unchanged

### Requirement: TestLevel Recorded On The Run

The `Record` SHALL grow additively with an optional `TestLevel` field
(`json:"testLevel,omitempty"`), with NO `SchemaVersion` bump, set from the
deployment plan's `TestLevel` at run-creation time. Older `run.json` files
written before `TestLevel` existed SHALL still `Load` cleanly, with
`TestLevel` zero-valued (HU-015 proposal, In Scope). This preserves the
existing "Additive Record Growth With Backward Compatibility" contract:
new fields are additive, unversioned, and zero-value-safe on load.

#### Scenario: TestLevel persisted at run creation
- GIVEN a new run is created from a deployment plan carrying a `TestLevel`
- WHEN the run record is written
- THEN `TestLevel` holds the plan's test level value

#### Scenario: Prior-slice run.json still loads without TestLevel
- GIVEN a `run.json` written before `TestLevel` existed
- WHEN it is loaded
- THEN it loads without error and `TestLevel` is zero-valued (empty string)

#### Scenario: TestLevel round-trips through a full write/reload
- GIVEN a run whose record has `TestLevel` set
- WHEN the record is written and reloaded
- THEN `TestLevel` round-trips unchanged

### Requirement: Mode Recorded On The Run

The `Record` SHALL grow additively with an optional `Mode` field (`json:"mode,omitempty"`), holding `""` (promotion), `"delta"`, or `"validate"`, with NO `SchemaVersion` bump, set at run-creation time to distinguish standalone-mode runs (HU-018) from full-promotion runs. This preserves the existing "Additive Record Growth With Backward Compatibility" contract: new fields are additive, unversioned, and zero-value-safe on load. Older `run.json` files written before `Mode` existed SHALL still `Load` cleanly, with `Mode` zero-valued (empty string, meaning promotion).

#### Scenario: Mode set to delta for standalone delta runs
- GIVEN a new run is created from standalone delta mode
- WHEN the run record is written
- THEN `Mode` holds `"delta"`

#### Scenario: Mode set to validate for standalone validation runs
- GIVEN a new run is created from standalone validation mode
- WHEN the run record is written
- THEN `Mode` holds `"validate"`

#### Scenario: Prior-slice run.json still loads without Mode
- GIVEN a `run.json` written before `Mode` existed
- WHEN it is loaded
- THEN it loads without error and `Mode` is zero-valued (empty string)

#### Scenario: Mode round-trips through a full write/reload
- GIVEN a run whose record has `Mode` set
- WHEN the record is written and reloaded
- THEN `Mode` round-trips unchanged

### Requirement: Incremental Append Mutates The Same Run Record In Place

When commits are appended to an existing promotion branch via reuse-on-collision, the system SHALL locate the prior run record for that branch and SHALL mutate it in place — extending `Commits`, bumping `PickTotal` and `UpdatedAt` — rather than creating a new run record. `CreatedAt` and the run's identity (`RunID`) SHALL be preserved unchanged, and `SourceRunID` SHALL NOT be set by this mutation.

#### Scenario: Reuse appends to the existing run record
- GIVEN a prior run record exists for the deploy branch being reused
- WHEN new commits are cherry-picked onto the reused branch
- THEN the same `RunID`'s record is updated with the extended `Commits`, a bumped `PickTotal`, and a bumped `UpdatedAt`

#### Scenario: CreatedAt and RunID survive the increment
- GIVEN a run record is incrementally appended
- WHEN the update is persisted
- THEN `CreatedAt` and `RunID` remain unchanged from the original run

#### Scenario: Increment does not set SourceRunID
- GIVEN a run record is incrementally appended after branch reuse
- WHEN the update is persisted
- THEN `SourceRunID` is not set, since the increment targets the same branch rather than a cross-environment re-promotion

#### Scenario: Prior-slice run.json still loads after an in-place mutation
- GIVEN a run record written before an increment occurred
- WHEN it is reloaded after being mutated in place
- THEN it loads without error and reflects the extended commit set

## Extended by HU-013

Full run history browsing/listing, retention/cleanup policy, and resume-by-jobId are now owned by HU-013, which extends this writer. See `run-history`, `run-retention`, and `run-resume` capabilities.
