# Delta for Deploy Gate

## ADDED Requirements

### Requirement: Ungated Environment Deploys Without PR, Approvals Or Signature

An environment with no matching `gates` entry, or whose gate is `enabled: false`, SHALL be deployable through quick deploy without a pull request, approvals, resolved threads, validation comment or signature. No gate evaluation SHALL run for it, and `gh` SHALL NOT be required. This is the documented default: governance is opt-in per environment. A target that is not listed under `branches` is classified as production (fail-closed) and keeps the production rules.

#### Scenario: Ungated target deploys with no PR
- GIVEN a target with no matching `gates` entry, and an eligible run that has no PR
- WHEN the typed `DESPLEGAR` confirmation is given with execution allowed
- THEN the quick deploy is dispatched without any gate check and without `gh`

#### Scenario: Disabled gate behaves as ungated
- GIVEN a target whose `gates` entry has `enabled: false`
- WHEN the same deploy is requested
- THEN it is dispatched without a gate check

### Requirement: Result-Screen Deploy Offer Never Applies To Gated Targets

The validation result-screen deploy offer (see `quick-deploy`) SHALL NOT be shown for a target whose gate is enabled. Gated targets SHALL deploy only through the existing run-history path, where the gate check runs unchanged after the typed confirmation.

#### Scenario: Gated target sees no offer
- GIVEN a successful validation on a target with an enabled gate
- WHEN the result screen renders
- THEN no deploy offer, hint or `x` binding is shown

#### Scenario: Gated run-history deploy still checks the gate
- GIVEN a run on a gated target opened for quick deploy from run history
- WHEN the typed confirmation is given
- THEN the gate check runs before dispatch, exactly as before
