# Delta for Promotion Branch

## MODIFIED Requirements

### Requirement: Existing Temp Branch Collision Handling

The system SHALL prompt the user to choose among reuse & append, delete & recreate, or cancel when the temp branch name already exists locally or remotely, and SHALL NOT dead-end into an unrecoverable error.
(Previously: prompted for "an action" with no defined choice set; in practice the collision fell through to an unconditional error dead end with no reuse path.)

#### Scenario: Existing branch offers reuse, recreate, or cancel
- GIVEN a branch with the target name already exists locally or in `origin`
- WHEN creation is attempted
- THEN the user is prompted to choose reuse & append, delete & recreate, or cancel

#### Scenario: Collision never dead-ends silently
- GIVEN a branch collision is detected
- WHEN the collision prompt is shown
- THEN the flow always presents an actionable choice, never a terminal error with no path forward
