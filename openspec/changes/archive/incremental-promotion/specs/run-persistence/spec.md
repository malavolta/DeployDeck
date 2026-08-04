# Delta for Run Persistence

## ADDED Requirements

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
