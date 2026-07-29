# Quick Deploy Specification

## Purpose

HU-015 (`docs/HISTORIAS.md:987-1037`) lets a release manager reuse a
successful validation's `job-id` for a Salesforce quick deploy — within
Salesforce's 10-day window — without turning DeployDeck into a default
deploy tool (Objetivo, `docs/HISTORIAS.md:997-999`). Detection and the
suggested command are always on; execution is strictly opt-in, gated by
configuration and a typed strong confirmation, and blocked for production
targets unless explicitly allowed.

## Requirements

### Requirement: Eligibility Detection For Quick Deploy

The system SHALL treat a run as quick-deploy-eligible if and only if its
persisted status is `Succeeded` or `SucceededPartial`, its persisted
`TestLevel` indicates the required tests ran, and its age is less than 10
days (Salesforce's quick-deploy window) (HU-015 Tareas `docs/HISTORIAS.md:1003`,
AC `docs/HISTORIAS.md:1012`).

#### Scenario: Eligible run is offered
- GIVEN a run with status `Succeeded`, required tests ran, and age under 10 days
- WHEN eligibility is evaluated
- THEN the run is quick-deploy-eligible

#### Scenario: Run older than 10 days is not offered
- GIVEN a run with status `Succeeded`, required tests ran, and age over 10 days
- WHEN eligibility is evaluated
- THEN the run is NOT quick-deploy-eligible (age variant, HU-015 AC
  `docs/HISTORIAS.md:1012`, Test E2E `:1035`)

#### Scenario: Run without required tests is not offered
- GIVEN a run with status `Succeeded`, age under 10 days, but required tests
  did not run
- WHEN eligibility is evaluated
- THEN the run is NOT quick-deploy-eligible (test-level variant, HU-015 AC
  `docs/HISTORIAS.md:1012`, Test E2E `:1035`)

#### Scenario: Old run.json without TestLevel fails safe
- GIVEN a run persisted before `TestLevel` existed, loaded with `TestLevel`
  zero-valued
- WHEN eligibility is evaluated
- THEN the run is NOT quick-deploy-eligible, regardless of status or age

### Requirement: Suggested Command Displayed, Not Executed By Default

For an eligible run, the system SHALL display the suggested
`sf project deploy quick --job-id <run's JobID> --target-org <alias>`
command. The system SHALL NOT execute this command by default (HU-015 AC
`docs/HISTORIAS.md:1011`, Tareas `:1004`).

#### Scenario: Eligible run shows its own job-id in the command
- GIVEN an eligible run with a persisted `JobID`
- WHEN its quick-deploy view is opened
- THEN the suggested command is displayed with that run's `JobID`
- AND no command is executed

### Requirement: Production Target Blocked Without Explicit Configuration

The system SHALL NOT execute quick deploy against a production target
unless `AllowProduction` is explicitly configured true (HU-015 AC
`docs/HISTORIAS.md:1013`).

#### Scenario: Production target without AllowProduction is not executed
- GIVEN the target org is production and `AllowProduction` is false
- WHEN execution is requested, even with `AllowExecution` true and strong
  confirmation given
- THEN quick deploy is NOT executed

### Requirement: Strong Confirmation Required For Execution

The system SHALL NOT execute quick deploy without a typed strong
confirmation, independent of any other configuration (HU-015 AC
`docs/HISTORIAS.md:1014`).

#### Scenario: Missing confirmation blocks execution
- GIVEN `AllowExecution` is true and the target is permitted
- WHEN the user does not provide the typed strong confirmation
- THEN no deploy is executed

### Requirement: Opt-In Execution Runs Quick Deploy And Registers The Action

When `AllowExecution` is true, the target is permitted (non-production, or
`AllowProduction` is true), and the user provides the typed strong
confirmation, the system SHALL run `sf project deploy quick` with the
eligible run's `JobID` and target org alias, and SHALL record the action on
the run (HU-015 AC `docs/HISTORIAS.md:1011`, Test E2E
`docs/HISTORIAS.md:1034`).

#### Scenario: Fully authorized execution runs and registers
- GIVEN `AllowExecution` is true, the target is permitted, and the user
  provides the typed strong confirmation
- WHEN quick deploy is invoked for an eligible run
- THEN `sf project deploy quick` runs with that run's `JobID` and target org
  alias
- AND the action is recorded on the run

### Requirement: Suggest-Only By Default

With `AllowExecution` false (the default), the system SHALL only ever
display the suggested command for eligible runs and SHALL NOT execute quick
deploy (HU-015 Tareas `docs/HISTORIAS.md:1004`).

#### Scenario: Default configuration never executes
- GIVEN `AllowExecution` is false (default) and an eligible run
- WHEN the quick-deploy view is opened
- THEN only the suggested command is shown
- AND no execution occurs, regardless of any confirmation input
