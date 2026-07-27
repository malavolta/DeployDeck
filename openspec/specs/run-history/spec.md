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
