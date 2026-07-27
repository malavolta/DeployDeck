# Delta for Run Persistence

## ADDED Requirements

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
