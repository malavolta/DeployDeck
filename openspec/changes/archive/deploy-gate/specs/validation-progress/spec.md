# Delta for Validation Progress

## ADDED Requirements

### Requirement: Terminal Successful CheckOnly Triggers The Deploy-Gate Validation Comment

When a validation job reaches a terminal successful state (`Succeeded` or
`SucceededPartial`) on an environment whose deploy gate is enabled, the
system SHALL trigger the deploy-gate capability's validation-comment
post/upsert on the run's pull request (see the `deploy-gate` capability for
the marker format, content, and idempotent upsert semantics — not restated
here). On an environment with no enabled deploy gate, reaching a terminal
successful state SHALL NOT trigger any comment.

#### Scenario: Successful CheckOnly on a gate-enabled environment triggers the comment
- GIVEN a validation job's target environment has an enabled deploy gate
- WHEN the job reaches a terminal successful state (`Succeeded` or
  `SucceededPartial`)
- THEN the deploy-gate validation-comment post/upsert is triggered for the
  run's pull request

#### Scenario: Successful CheckOnly on an ungated environment posts nothing
- GIVEN a validation job's target environment has no enabled deploy gate
- WHEN the job reaches a terminal successful state
- THEN no validation comment is posted or updated

#### Scenario: A non-successful terminal state does not trigger the comment
- GIVEN a validation job's target environment has an enabled deploy gate
- WHEN the job reaches a terminal state that is not `Succeeded` or
  `SucceededPartial` (e.g. `Failed` or `Canceled`)
- THEN the deploy-gate validation comment is NOT triggered
