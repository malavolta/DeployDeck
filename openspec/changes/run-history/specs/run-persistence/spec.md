# Delta for Run Persistence

HU-013 (`docs/HISTORIAS.md:814-894`) extends the minimal writer from HU-010/HU-011. This closes the `## Out of Scope (Deferred to HU-013)` carve-out at the bottom of the main spec: full history browsing, retention, and resume-by-jobId are now implemented by the `run-history`, `run-retention`, and `run-resume` capabilities, which build on the additions below. At archive time, replace that carve-out note with a pointer to those three capabilities. No existing requirement's behavior changes — all three prior requirements (`Minimal Run Record On jobId Receipt`, `Cancel Result Persisted As A Companion File`, `Canceled Status Updates The Run Record`) are kept unchanged.

## ADDED Requirements

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
