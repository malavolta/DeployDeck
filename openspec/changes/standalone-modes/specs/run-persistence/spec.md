# Delta for Run Persistence

## ADDED Requirements

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
