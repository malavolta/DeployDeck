# Delta for Deploy Validation

Source: HU-010 (`docs/HISTORIAS.md:628-696`). Launches async `sf project deploy validate` against the reviewed package.

## ADDED Requirements

### Requirement: Async Validate Command Construction

The system SHALL run `sf project deploy validate --manifest <package.xml> --target-org <alias> --test-level <level> --async --json` to validate a package, using the `TargetBranch`/`SandboxAlias`/`TestLevel` from the `DeploymentPlan`.

#### Scenario: Valid package returns a jobId
- GIVEN a valid `package.xml`
- WHEN validation is launched
- THEN `sf project deploy validate --async --json` runs and a `jobId` is obtained

### Requirement: Destructive Flag Included Only When Applicable

The system SHALL include `--post-destructive-changes <destructiveChanges.xml>` only when the package has destructive changes, and SHALL omit it otherwise.

#### Scenario: Destructive changes add the post-destructive flag
- GIVEN a package with destructive changes
- WHEN validation is launched
- THEN the command includes `--post-destructive-changes` pointing at `destructiveChanges.xml`

#### Scenario: No destructive changes omits the flag
- GIVEN a package with no destructive changes
- WHEN validation is launched
- THEN the command does not include `--post-destructive-changes`

### Requirement: RunSpecifiedTests Includes The Tests Flag

The system SHALL include `--tests <classes>` only when the user selects `RunSpecifiedTests` as the test level, listing the chosen classes; other test levels SHALL NOT include `--tests`.

#### Scenario: RunSpecifiedTests includes the tests flag
- GIVEN the user selects `RunSpecifiedTests` with a list of classes
- WHEN validation is launched
- THEN the command includes `--tests` with the indicated classes

### Requirement: CLI Error Surfaces Message And Raw JSON

The system SHALL show an error message and, when available, the raw JSON returned by the Salesforce CLI when `sf project deploy validate` fails, without breaking the flow.

#### Scenario: CLI error shows message and raw JSON
- GIVEN the Salesforce CLI returns an error
- WHEN validation is launched
- THEN an error message is shown together with the raw JSON, and the flow continues rather than crashing

### Requirement: Run Persisted Immediately On jobId Receipt

The system SHALL persist the run to `.deploydeck/runs/` immediately once a `jobId` is obtained (see `run-persistence` capability for the minimal writer contract).

#### Scenario: jobId receipt triggers immediate persistence
- GIVEN validation returns a `jobId`
- WHEN the `jobId` is parsed
- THEN the run is persisted to `.deploydeck/runs/` before any further step
