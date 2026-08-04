# Delta for Run Persistence

## ADDED Requirements

### Requirement: Failed Cancel/Quick-Deploy Raw Response Persisted As A Companion File

When `CancelDeploy` or the opt-in quick-deploy execution fails (other than the already-terminal cancel outcome), the system SHALL persist the raw failure response as an additional companion file under the run's directory, following the same additive companion-file pattern as the existing cancel-success and per-poll report companions, with NO `SchemaVersion` bump.

#### Scenario: Failed cancel writes a raw-failure companion
- GIVEN an existing run directory for a `jobId`
- WHEN `CancelDeploy` fails, other than an already-terminal outcome
- THEN a companion file holding the raw failure response is written under the run directory

#### Scenario: Failed quick deploy writes a raw-failure companion
- GIVEN an existing run directory for a `jobId`
- WHEN the opt-in quick-deploy execution fails
- THEN a companion file holding the raw failure response is written under the run directory

### Requirement: Failed Validate-Launch Raw Envelope Persisted As A Companion File

When `sf project deploy validate` fails at launch, before a `jobId` is obtained, the system SHALL persist the raw failure envelope as a companion file associated with the attempted run, following the writer companion pattern, so its path can be surfaced on the error screen (see `deploy-validation`).

#### Scenario: Validate-launch failure writes a raw-envelope companion
- GIVEN `sf project deploy validate` fails before any `jobId` is obtained
- WHEN the failure is processed
- THEN the raw failure envelope is persisted as a companion file whose path can be surfaced to the user

### Requirement: Companion Growth Stays Additive And Backward Compatible

New companion files introduced for failed cancel/quick/validate-launch raw responses SHALL NOT require a `SchemaVersion` bump, and existing runs persisted before these companions existed SHALL still `List`/`Load` unchanged.

#### Scenario: Old runs load unchanged after the new companions are introduced
- GIVEN a run directory persisted before the failed-cancel/quick/validate-launch companions existed
- WHEN it is loaded via `List`/`Load`
- THEN it loads without error, unaffected by the new companion types
