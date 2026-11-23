# Delta for Quick Deploy

## ADDED Requirements

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
