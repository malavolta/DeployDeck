# Run History

## Purpose

HU-013 (`docs/HISTORIAS.md:814-894`) makes past runs browsable after the TUI closes and reopens: a list of prior runs, a detail view, and a shortcut to resume a resumable one. Built on `Runs.List()` from `run-persistence`. Mockup: `docs/MOCKUPS_TUI.md:369-388` (`Historial De Runs`).

## Requirements

### Requirement: History Screen Lists Runs Newest First

The system SHALL show a run history screen listing past runs newest first, each row showing ticket, target org, status, and date.

#### Scenario: Prior run appears after reopening the TUI
- GIVEN the TUI was closed after a run was recorded
- WHEN the history screen is opened
- THEN the prior run appears in the list (HU-013 AC `docs/HISTORIAS.md:843`)

#### Scenario: Empty history shows no rows
- GIVEN no runs have been recorded yet
- WHEN the history screen is opened
- THEN the list is empty and no error is shown

### Requirement: Row Shows Progress Reached

Each history row SHALL show enough progress detail to distinguish runs: for a run with a `jobId`, its validation status; for a run without a job, the last step reached.

#### Scenario: Run without a job shows last step reached
- GIVEN a persisted run with no `jobId`
- WHEN it is shown in the history list
- THEN the row shows the last step the flow reached (HU-013 AC `docs/HISTORIAS.md:845`)

### Requirement: Detail View On Selection

Selecting a run SHALL show a detail panel with its branch, commit count, and delta package path.

#### Scenario: Selecting a run shows its detail
- GIVEN the history list has a selected run
- WHEN the user navigates to it
- THEN the detail panel shows its branch, commit count, and package path (`docs/MOCKUPS_TUI.md:380-383`)

### Requirement: Enter On A Resumable Run Initiates Resume

Pressing `Enter` on a resumable run (one with an in-progress cherry-pick or a non-terminal `jobId`) SHALL initiate the same resume routing as startup resume-detection.

#### Scenario: Enter on a conflicted run resumes to the conflict screen
- GIVEN the selected run is in `CherryPickConflict` with a matching in-progress cherry-pick
- WHEN the user presses `Enter`
- THEN the TUI routes to the conflict screen with the run's rehydrated context (HU-013 AC `docs/HISTORIAS.md:844`)

#### Scenario: Enter on a run with a jobId resumes to polling
- GIVEN the selected run has a non-terminal `jobId`
- WHEN the user presses `Enter`
- THEN the TUI re-attaches `deploy report` polling for that `jobId`

#### Scenario: Enter on a terminal run does not attempt resume
- GIVEN the selected run reached a terminal status (e.g. `Succeeded`, `Canceled`)
- WHEN the user presses `Enter`
- THEN no resume is attempted and the detail view remains shown

### Requirement: r On An Eligible Terminal-Success Run Initiates Re-Promote

Pressing `r` on a `StateRunHistory` row whose prior run status is
`Succeeded` or `SucceededPartial` SHALL initiate re-promotion for that run's
ticket, pre-seeded per the `re-promotion` capability. On a row that is not eligible (`Failed`, `Canceled`, `Aborted`) `r` SHALL NOT start re-promotion and SHALL show a Spanish notice explaining why (see `Reasons For Ineligible x And r`). This action is distinct from, and SHALL NOT alter, the existing `Enter`→resume
action or list navigation (HU-016).

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

On a `StateRunHistory` row whose underlying run is quick-deploy-eligible
(per the `quick-deploy` capability), the system SHALL provide a dedicated
per-row action that opens the quick-deploy view for that run, showing its
suggested command. This action is distinct from, and SHALL NOT alter, the
existing `Enter` (resume), `d` (delete), and `r` (re-promote) actions or list
navigation (HU-015 proposal, Modified Capabilities: `run-history`).

#### Scenario: Quick-deploy action on an eligible row opens the view
- GIVEN the selected row's underlying run is quick-deploy-eligible
- WHEN the user triggers the quick-deploy action
- THEN the quick-deploy view opens showing that run's suggested command

#### Scenario: Quick-deploy action on a non-eligible row explains
- GIVEN the selected row's underlying run is NOT quick-deploy-eligible
- WHEN the user triggers the quick-deploy action
- THEN the history view remains and a Spanish notice with the reason is shown

#### Scenario: Enter, d, and r remain unaffected
- GIVEN the selected row supports resume, delete, and/or re-promote per
  their existing requirements
- WHEN the user presses `Enter`, `d`, or `r`
- THEN each proceeds exactly as before, unaffected by the new quick-deploy
  action

(Previously: the action on a non-eligible row did nothing, with no feedback.)

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
