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
`AllowProduction` is true), the user provides the typed strong
confirmation, and — if the target environment has an enabled deploy gate —
that gate's conditions all pass, the system SHALL run
`sf project deploy quick` with the eligible run's `JobID` and target org
alias, and SHALL record the action on the run (HU-015 AC
`docs/HISTORIAS.md:1011`, Test E2E `docs/HISTORIAS.md:1034`).

#### Scenario: Fully authorized execution runs and registers
- GIVEN `AllowExecution` is true, the target is permitted, and the user
  provides the typed strong confirmation
- WHEN quick deploy is invoked for an eligible run
- THEN `sf project deploy quick` runs with that run's `JobID` and target org
  alias
- AND the action is recorded on the run

#### Scenario: An enabled deploy gate with an unmet condition blocks dispatch
- GIVEN `AllowExecution` is true, the target is permitted, the user provides
  the typed strong confirmation, and the target environment has an enabled
  deploy gate with at least one unmet condition
- WHEN quick deploy is invoked
- THEN `sf project deploy quick` is NOT run and no action is recorded
- AND the gate-block screen is shown listing the unmet condition(s)

#### Scenario: A target with no enabled deploy gate deploys unchanged
- GIVEN `AllowExecution` is true, the target is permitted, the user provides
  the typed strong confirmation, and the target environment has no enabled
  deploy gate
- WHEN quick deploy is invoked for an eligible run
- THEN `sf project deploy quick` runs and the action is recorded on the run,
  exactly as before the deploy-gate capability existed

### Requirement: Suggest-Only By Default

With `AllowExecution` false (the default), the system SHALL only ever
display the suggested command for eligible runs and SHALL NOT execute quick
deploy (HU-015 Tareas `docs/HISTORIAS.md:1004`).

#### Scenario: Default configuration never executes
- GIVEN `AllowExecution` is false (default) and an eligible run
- WHEN the quick-deploy view is opened
- THEN only the suggested command is shown
- AND no execution occurs, regardless of any confirmation input

### Requirement: Failed Quick Deploy Persists Raw Response And Shows An Actionable Message

When `sf project deploy quick` fails during opt-in execution, the system SHALL show an actionable error message and SHALL persist the raw failure response as a companion file for the run, mirroring the cancel-failure persistence pattern.

#### Scenario: Quick deploy failure shows an actionable message
- GIVEN opt-in execution is authorized (`AllowExecution` true, target permitted, typed strong confirmation given)
- WHEN `sf project deploy quick` fails
- THEN an actionable error message is shown to the user

#### Scenario: Failed quick deploy persists the raw response
- GIVEN the quick-deploy failure above
- WHEN it is processed
- THEN the raw failure response is persisted as a companion file for the run

### Requirement: Quick Deploy Offered From The Validation Result Screen

On `StateSucceeded`, the system SHALL offer quick deploy via `x` ("desplegar") if and only if the target is ungated (no enabled gate), the target is not production, and the just-finished run is quick-deploy-eligible. Gated or production targets SHALL see no offer, hint or binding. The offer SHALL be subject to the same configuration as run history: when `AllowExecution` is false it SHALL be shown as unavailable with its reason and SHALL never execute.

#### Scenario: Eligible ungated non-production run shows the offer
- GIVEN `StateSucceeded` for an ungated, non-production target and an eligible run
- WHEN the result screen renders
- THEN an `x` "desplegar" hint is shown in the footer and an offer line is shown

#### Scenario: Production target shows no offer
- GIVEN `StateSucceeded` for a production target (including a branch not listed under `branches`), even with `AllowProduction` true
- WHEN the result screen renders and `x` is pressed
- THEN no offer is shown and `x` does nothing

#### Scenario: Ineligible run shows the reason
- GIVEN `StateSucceeded` for an ungated non-production target whose run is ineligible (no required tests, expired, or already deployed)
- WHEN the result screen renders
- THEN the offer is shown as unavailable with the existing Spanish reason from `quickDeployReasonES`
- AND `x` does not open the confirmation

#### Scenario: Execution disabled shows unavailable and never executes
- GIVEN an eligible ungated non-production run and `AllowExecution` false
- WHEN the result screen renders and `x` is pressed
- THEN the offer is shown as unavailable with the reason
- AND no deploy is executed, regardless of any input

### Requirement: Result-Screen Deploy Reuses The Typed Confirmation

Pressing `x` on an available offer SHALL load the just-finished run record and open the existing `StateQuickDeploy` with the `DESPLEGAR` typed confirmation; no new state SHALL be introduced. Execution SHALL require the typed confirmation and the same in-flight guards as the run-history path. If the run record cannot be loaded or lacks its persisted status, `JobID`, alias or `TestLevel`, the offer SHALL be hidden or unavailable rather than executed.

#### Scenario: x opens the DESPLEGAR confirmation
- GIVEN an available offer on `StateSucceeded`
- WHEN the user presses `x`
- THEN `StateQuickDeploy` opens for that run, requiring `DESPLEGAR`
- AND nothing executes until the correct text is confirmed

#### Scenario: Confirmed deploy runs and records the action
- GIVEN the confirmation screen opened from the result screen
- WHEN `DESPLEGAR` is typed and confirmed
- THEN `sf project deploy quick` runs with that run's `JobID` and alias and the action is recorded on the run

#### Scenario: Incomplete run record hides the offer
- GIVEN the finished run's record cannot be loaded or is missing required fields
- WHEN the result screen renders
- THEN no executable offer is shown

### Requirement: Quick Deploy Returns To Its Origin Screen

`StateQuickDeploy` SHALL remember whether it was opened from the result screen or from run history. `esc` and `q` (empty buffer) SHALL return to `StateSucceeded` for the former and to `StateRunHistory` for the latter. The run-history `x` path SHALL be unchanged.

#### Scenario: Esc returns to the result screen
- GIVEN `StateQuickDeploy` opened from `StateSucceeded`
- WHEN the user presses `esc`, or `q` with an empty buffer
- THEN the state is `StateSucceeded` and no quit command is returned

#### Scenario: Run-history origin is unchanged
- GIVEN `StateQuickDeploy` opened from run history
- WHEN the user presses `esc`
- THEN the state is `StateRunHistory`
