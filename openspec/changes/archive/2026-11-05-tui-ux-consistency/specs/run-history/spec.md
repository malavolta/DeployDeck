# Run History Delta (tui-ux-consistency)

## ADDED Requirements

### Requirement: History Esc Goes Back And Q Quits

On `StateRunHistory`, `esc` MUST go back to the parent state (the main menu when opened from it) and `q` MUST return the quit command. When history is shown as the resume offer, `esc` declines the offer. A run declined or left with `q` stays resumable and MUST be offered again on the next launch. The footer MUST contain `Esc volver` and `q salir`.

#### Scenario: Esc goes back
- GIVEN `StateRunHistory` opened from the main menu
- WHEN the user presses `esc`
- THEN the state is `StateMainMenu`

#### Scenario: q quits
- GIVEN `StateRunHistory`
- WHEN the user presses `q`
- THEN the quit command is returned

#### Scenario: Declining with esc, then q, keeps the run resumable
- GIVEN a resumable run offered through the history screen
- WHEN the user presses `esc`, and in another session presses `q`
- THEN the run record is not deleted or marked terminal
- AND the next startup offers resume for that run again

#### Scenario: Footer pins both keys
- GIVEN `StateRunHistory` with at least one row
- WHEN it renders
- THEN the footer contains `Esc volver` and `q salir`

### Requirement: Reasons For Ineligible x And r

When `x` (quick deploy) or `r` (re-promote) is pressed on a history row that is not eligible, the system MUST NOT act and MUST show a Spanish notice stating why. The `x` notice MUST convey the reason from `runs.QuickDeployEligible`, translated to Spanish. The `r` notice MUST state that only runs that finished successfully (`Succeeded` or `SucceededPartial`) can be re-promoted. The state, selection and history rows MUST remain unchanged.

#### Scenario: x on an ineligible run explains the reason
- GIVEN the selected row's run is not quick-deploy-eligible, with a known reason
- WHEN the user presses `x`
- THEN the quick-deploy view does not open
- AND the view contains a Spanish notice carrying that reason (the notice is not empty and contains no English reason text)

#### Scenario: Each QuickDeployEligible reason maps to Spanish
- GIVEN every ineligibility reason `runs.QuickDeployEligible` can return
- WHEN the corresponding notice text is produced
- THEN each reason has a Spanish text, and an unknown reason falls back to a generic Spanish notice

#### Scenario: r on a Failed run explains the restriction
- GIVEN the selected row's status is `Failed`, `Canceled` or `Aborted`
- WHEN the user presses `r`
- THEN no re-promotion starts
- AND the view contains a Spanish notice mentioning that only successful runs can be re-promoted (fragment `exitos`)

#### Scenario: Notice clears on movement
- GIVEN an ineligibility notice is displayed
- WHEN the user moves the selection or leaves the screen
- THEN the notice is no longer rendered

## MODIFIED Requirements

### Requirement: r On An Eligible Terminal-Success Run Initiates Re-Promote

Pressing `r` on a `StateRunHistory` row whose prior run status is `Succeeded` or `SucceededPartial` SHALL initiate re-promotion for that run's ticket, pre-seeded per the `re-promotion` capability. On a row that is not eligible (`Failed`, `Canceled`, `Aborted`) `r` SHALL NOT start re-promotion and SHALL show a Spanish notice explaining why (see `Reasons For Ineligible x And r`). This action is distinct from, and SHALL NOT alter, the existing `Enter`→resume action or list navigation (HU-016).
(Previously: `r` on an ineligible row had no effect and no feedback.)

#### Scenario: r on a Succeeded run starts re-promote
- GIVEN the selected row's prior run status is `Succeeded`
- WHEN the user presses `r`
- THEN re-promotion starts for that run's ticket (HU-016 AC `docs/HISTORIAS.md:1062`)

#### Scenario: r on a SucceededPartial run starts re-promote
- GIVEN the selected row's prior run status is `SucceededPartial`
- WHEN the user presses `r`
- THEN re-promotion starts for that run's ticket

#### Scenario: r on a Failed run does not start and explains
- GIVEN the selected row's prior run status is `Failed`
- WHEN the user presses `r`
- THEN no re-promotion starts and the state is unchanged (HU-016 `docs/HISTORIAS.md:1076`)
- AND a Spanish notice is shown

#### Scenario: Enter still resumes independently of r
- GIVEN the selected row is resumable per the existing Enter requirement
- WHEN the user presses `Enter`
- THEN resume proceeds exactly as before, unaffected by the new `r` action

### Requirement: Per-Row Quick Deploy Action On An Eligible Run

On a `StateRunHistory` row whose underlying run is quick-deploy-eligible (per the `quick-deploy` capability), the system SHALL provide a dedicated per-row action (`x`) that opens the quick-deploy view for that run, showing its suggested command. On a non-eligible row the action SHALL NOT open the view and SHALL show a Spanish notice with the translated `runs.QuickDeployEligible` reason. This action is distinct from, and SHALL NOT alter, the existing `Enter` (resume), `d` (delete), and `r` (re-promote) actions or list navigation (HU-015 proposal, Modified Capabilities: `run-history`).
(Previously: the action on a non-eligible row did nothing, with no feedback.)

#### Scenario: Quick-deploy action on an eligible row opens the view
- GIVEN the selected row's underlying run is quick-deploy-eligible
- WHEN the user triggers the quick-deploy action
- THEN the quick-deploy view opens showing that run's suggested command

#### Scenario: Quick-deploy action on a non-eligible row explains
- GIVEN the selected row's underlying run is NOT quick-deploy-eligible
- WHEN the user triggers the quick-deploy action
- THEN the history view remains and a Spanish notice with the reason is shown

#### Scenario: Enter, d, and r remain unaffected
- GIVEN the selected row supports resume, delete, and/or re-promote per their existing requirements
- WHEN the user presses `Enter`, `d`, or `r`
- THEN each proceeds exactly as before, unaffected by the new quick-deploy action

## REMOVED Requirements

None.
