# Delta for Quick Deploy

## MODIFIED Requirements

### Requirement: Opt-In Execution Runs Quick Deploy And Registers The Action

When `AllowExecution` is true, the target is permitted (non-production, or
`AllowProduction` is true), the user provides the typed strong
confirmation, and — if the target environment has an enabled deploy gate —
that gate's conditions all pass, the system SHALL run
`sf project deploy quick` with the eligible run's `JobID` and target org
alias, and SHALL record the action on the run (HU-015 AC
`docs/HISTORIAS.md:1011`, Test E2E `docs/HISTORIAS.md:1034`).
(Previously: execution ran on the typed confirmation alone, with no
dependency on the target environment's deploy-gate state.)

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
