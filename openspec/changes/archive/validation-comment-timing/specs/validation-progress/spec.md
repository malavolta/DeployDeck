# Delta for Validation Progress

## MODIFIED Requirements

### Requirement: Terminal Successful CheckOnly Triggers The Deploy-Gate Validation Comment

When a validation job reaches a terminal successful state (`Succeeded` or
`SucceededPartial`) on an environment whose deploy gate is enabled, the
system SHALL trigger the deploy-gate capability's validation-comment
post/upsert on the run's pull request. The trigger SHALL ALSO be
(re)attempted when the run's PR is created, if a terminal-successful
validation already occurred this session on a gate-enabled environment with
`requireValidationComment` on — this covers the normal flow where
validation completes before the PR exists. Both trigger points invoke the
same post/upsert; see the `deploy-gate` capability for the marker format,
content, and the exactly-once posting semantics across the two trigger
points — not restated here. On an environment with no enabled deploy gate,
reaching a terminal successful state SHALL NOT trigger any comment, and PR
creation with no terminal-successful validation this session SHALL NOT
trigger any comment either.
(Previously: the trigger fired only at terminal-successful validation,
which never reached the run's PR in the normal flow because the PR is
created afterward, via a separate explicit action, once validation is
already done.)

#### Scenario: Successful CheckOnly on a gate-enabled environment triggers the comment
- GIVEN a validation job's target environment has an enabled deploy gate
- WHEN the job reaches a terminal successful state (`Succeeded` or `SucceededPartial`)
- THEN the deploy-gate validation-comment post/upsert is triggered for the run's pull request

#### Scenario: Successful CheckOnly on an ungated environment posts nothing
- GIVEN a validation job's target environment has no enabled deploy gate
- WHEN the job reaches a terminal successful state
- THEN no validation comment is posted or updated

#### Scenario: A non-successful terminal state does not trigger the comment
- GIVEN a validation job's target environment has an enabled deploy gate
- WHEN the job reaches a terminal state that is not `Succeeded` or `SucceededPartial` (e.g. `Failed` or `Canceled`)
- THEN the deploy-gate validation comment is NOT triggered

#### Scenario: Validation completes before the PR exists; PR creation (re)triggers the comment
- GIVEN a validation job reached a terminal successful state this session on a gate-enabled environment with `requireValidationComment` on, before the run's PR existed
- WHEN the run's PR is subsequently created
- THEN the deploy-gate validation-comment post/upsert is (re)triggered at PR creation, reaching the PR that did not exist at validation time (see `deploy-gate` for how the dedup marker keeps this exactly-once)
