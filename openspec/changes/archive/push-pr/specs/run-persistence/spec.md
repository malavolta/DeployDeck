# Delta for Run Persistence

## ADDED Requirements

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
