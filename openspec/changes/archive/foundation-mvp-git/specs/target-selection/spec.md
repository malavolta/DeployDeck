# Delta for Target Selection

## ADDED Requirements

### Requirement: Config-Driven Destination Branch Listing
The system SHALL read target branches and their sandbox mapping from YAML configuration and display each destination with its associated sandbox. (HU-004)

#### Scenario: Configured branches show their sandbox
- GIVEN branches configured with sandbox mappings
- WHEN the user opens destination selection
- THEN each destination is shown with its associated sandbox

### Requirement: Nonexistent Destination Branch Block
The system SHALL block continuing when the selected destination branch does not exist locally or remotely.

#### Scenario: Missing destination branch blocks the flow
- GIVEN a destination branch that does not exist locally or remotely
- WHEN the user attempts to continue
- THEN the flow is blocked

### Requirement: Release Branch Sandbox Pattern Resolution
The system SHALL resolve the sandbox for `Release/*` branches via a configured pattern, and SHALL block with an actionable message when no mapping resolves.

#### Scenario: Release branch resolves sandbox by pattern
- GIVEN a `Release/*` branch and a matching sandbox pattern in configuration
- WHEN the user selects that branch
- THEN the sandbox resolves via the pattern

#### Scenario: Unmapped Release branch blocks with actionable message
- GIVEN a `Release/*` branch with no matching sandbox pattern
- WHEN the user selects that branch
- THEN the flow is blocked with an actionable message

### Requirement: Remote HEAD Display
The system SHALL show the destination branch's remote HEAD via `git rev-parse origin/<target>`.

#### Scenario: Remote HEAD is shown for the destination
- GIVEN a destination branch with a known `origin` HEAD
- WHEN it is selected
- THEN the remote HEAD is displayed

### Requirement: Unauthenticated Sandbox Warning
The system SHALL warn, without blocking, when the sandbox alias associated with the selected destination is not authenticated per `sf org list --json`.

#### Scenario: Unauthenticated sandbox shows a warning
- GIVEN the sandbox alias for the selected destination is not authenticated
- WHEN the destination is selected
- THEN a warning is shown before validation

### Requirement: Production Branch Warning
The system SHALL show a productive-environment warning when the user selects `main`.

#### Scenario: Selecting main warns about production
- GIVEN the user selects `main` as destination
- WHEN the selection is made
- THEN a production-environment warning is shown

### Requirement: Custom Branch Validation
The system SHALL allow a custom destination branch only if it exists locally or remotely.

#### Scenario: Custom branch must exist
- GIVEN the user enters a custom branch name
- WHEN it does not exist locally or remotely
- THEN the selection is rejected

### Requirement: Selection Persistence To Deployment Plan
The system SHALL save the selected branch, sandbox alias, and test level into the `DeploymentPlan`.

#### Scenario: Selection is saved to the plan
- GIVEN a valid destination and sandbox selection
- WHEN the user confirms
- THEN branch, alias, and test level are saved to the `DeploymentPlan`
