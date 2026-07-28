# Delta for Run History

## ADDED Requirements

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

#### Scenario: Quick-deploy action on a non-eligible row has no effect
- GIVEN the selected row's underlying run is NOT quick-deploy-eligible
- WHEN the user triggers the quick-deploy action
- THEN nothing happens and the history view remains unchanged

#### Scenario: Enter, d, and r remain unaffected
- GIVEN the selected row supports resume, delete, and/or re-promote per
  their existing requirements
- WHEN the user presses `Enter`, `d`, or `r`
- THEN each proceeds exactly as before, unaffected by the new quick-deploy
  action
