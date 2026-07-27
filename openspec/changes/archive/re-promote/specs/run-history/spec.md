# Delta for Run History

## ADDED Requirements

### Requirement: r On An Eligible Terminal-Success Run Initiates Re-Promote

Pressing `r` on a `StateRunHistory` row whose prior run status is
`Succeeded` or `SucceededPartial` SHALL initiate re-promotion for that run's
ticket, pre-seeded per the `re-promotion` capability. `r` SHALL have no
effect on a row that is not eligible (`Failed`, `Canceled`, `Aborted`). This
action is distinct from, and SHALL NOT alter, the existing `Enter`→resume
action or list navigation (HU-016).

#### Scenario: r on a Succeeded run starts re-promote
- GIVEN the selected row's prior run status is `Succeeded`
- WHEN the user presses `r`
- THEN re-promotion starts for that run's ticket (HU-016 AC `docs/HISTORIAS.md:1062`)

#### Scenario: r on a SucceededPartial run starts re-promote
- GIVEN the selected row's prior run status is `SucceededPartial`
- WHEN the user presses `r`
- THEN re-promotion starts for that run's ticket

#### Scenario: r on a Failed run has no effect
- GIVEN the selected row's prior run status is `Failed`
- WHEN the user presses `r`
- THEN no re-promotion starts and the history view remains unchanged (HU-016 `docs/HISTORIAS.md:1076`)

#### Scenario: Enter still resumes independently of r
- GIVEN the selected row is resumable per the existing Enter requirement
- WHEN the user presses `Enter`
- THEN resume proceeds exactly as before, unaffected by the new `r` action
