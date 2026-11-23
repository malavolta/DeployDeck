# Delta for Run History

## ADDED Requirements

### Requirement: Selected Run Tail Shows The Failure Motivo

The "Run seleccionado" tail MUST add one line with the failure headline when the selected run's cached report classifies as failed (e.g. `✗ Falló por cobertura: 66% global < 75%`). It MUST show nothing for succeeded, in-progress or report-less runs and MUST NOT block rendering while the report loads.

#### Scenario: Failed run shows motivo
- GIVEN the selected run failed on coverage and its report is loaded
- WHEN history renders
- THEN the tail contains `Falló por cobertura: 66% global < 75%`

#### Scenario: Succeeded run shows no motivo
- GIVEN the selected run is `Succeeded`
- WHEN history renders
- THEN no failure line appears

## MODIFIED Requirements

### Requirement: Detail View On Selection

Selecting a run SHALL show a panel with its branch, commit count, and delta package path. `d` MUST be the only key that opens the full-screen run detail (see `run-failure-detail`); `Enter` keeps its resume behavior and MUST NOT open it. The former inline toggle is removed; Estado, Job Id, Fase and Pick move into the full-screen detail.
(Previously: `d` toggled an inline Detalle block with Estado/Job Id/Fase/Pick under the list.)

#### Scenario: Selecting a run shows its detail
- GIVEN the history list has a selected run
- WHEN the user navigates to it
- THEN the panel shows its branch, commit count, and package path (`docs/MOCKUPS_TUI.md:380-383`)

#### Scenario: d opens the full-screen detail
- GIVEN history with a selected run
- WHEN the user presses `d`
- THEN the full-screen detail opens and the history list is not rendered inline-expanded

#### Scenario: Inline toggle is gone
- GIVEN history after pressing `d` and `Esc`
- WHEN history renders
- THEN no inline `Detalle:` block with Estado/Job Id/Fase/Pick appears

#### Scenario: Enter does not open the detail
- GIVEN a resumable selected run
- WHEN the user presses `Enter`
- THEN resume proceeds and the detail does not open; on a terminal row nothing happens

### Requirement: Per-Row Quick Deploy Action On An Eligible Run

On a `StateRunHistory` row whose underlying run is quick-deploy-eligible
(per the `quick-deploy` capability), the system SHALL provide a dedicated
per-row action that opens the quick-deploy view for that run, showing its
suggested command. This action is distinct from, and SHALL NOT alter, the
existing `Enter` (resume), `d` (open run detail), and `r` (re-promote)
actions or list navigation (HU-015 proposal, Modified Capabilities: `run-history`).
(Previously: `d` was described as "(delete)"; it never deleted, and now opens the detail.)

#### Scenario: Quick-deploy action on an eligible row opens the view
- GIVEN the selected row's underlying run is quick-deploy-eligible
- WHEN the user triggers the quick-deploy action
- THEN the quick-deploy view opens showing that run's suggested command

#### Scenario: Quick-deploy action on a non-eligible row explains
- GIVEN the selected row's underlying run is NOT quick-deploy-eligible
- WHEN the user triggers the quick-deploy action
- THEN the history view remains and a Spanish notice with the reason is shown

#### Scenario: Enter, d, and r remain unaffected
- GIVEN the selected row supports resume, run detail, and/or re-promote per
  their existing requirements
- WHEN the user presses `Enter`, `d`, or `r`
- THEN each proceeds as specified, unaffected by the quick-deploy action
