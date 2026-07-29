# Delta for Run Persistence

## ADDED Requirements

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
