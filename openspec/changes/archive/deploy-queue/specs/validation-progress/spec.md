# Delta for Validation Progress

## ADDED Requirements

### Requirement: Distinct Cancel Entry Key From Polling

The system SHALL provide a distinct key (`c`) from the `ValidationPolling` screen that enters the cancel-confirmation flow (`StateCancelConfirm`), separate from the exit key (`q`).

#### Scenario: Cancel key enters cancel confirmation
- GIVEN the user is on `ValidationPolling` viewing the current run's in-progress job
- WHEN the user presses `c`
- THEN the TUI transitions to `StateCancelConfirm` (HU-012)

## MODIFIED Requirements

### Requirement: User Exit Leaves The Job Active And Resumable

The system SHALL leave the Salesforce job active when the user exits the progress screen, and SHALL keep the run resumable. This invariant applies to the exit key (`q`) only and is UNCHANGED by the separate, explicit, typed-confirmation cancel action (`c` → `StateCancelConfirm` → `CancelDeploy`): cancelling requires its own confirmed action and does not alter what `q` does.
(Previously: stated the exit-leaves-active-and-resumable invariant without reference to the new cancel action; the invariant's behavior is unchanged, this wording clarifies it is unaffected by cancel.)

#### Scenario: Exiting leaves the job active
- GIVEN the user exits the progress screen before a terminal state
- WHEN the exit happens
- THEN the Salesforce job keeps running and the run remains resumable

#### Scenario: Exit and cancel remain distinct actions
- GIVEN the user is on `ValidationPolling`
- WHEN the user presses `q` to exit rather than `c` to cancel
- THEN the job is left active/resumable and no cancellation is executed (HU-012)
