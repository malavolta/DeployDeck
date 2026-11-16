# Branch Cleanup Delta (tui-bubbles-adoption)

## ADDED Requirements

### Requirement: [U2] Strong Delete Confirmation Shows The Wrong-Word Notice

In the strong delete confirmation (the typed `BORRAR` prompt shown for a branch with unpushed commits), when the user presses Enter with text other than the exact word, the system MUST set the Spanish notice `escribe BORRAR exactamente para confirmar el borrado` and the view MUST render it. The branch MUST NOT be deleted, the typed buffer and the state MUST be kept, and the notice MUST clear on the next navigation or when the buffer changes. The exact-word gate is unchanged.

#### Scenario: Wrong word renders the notice
- GIVEN the strong confirmation for `deploy/DD-7` with `borrar` or `BORR` typed
- WHEN the user presses Enter
- THEN the view contains `escribe BORRAR exactamente para confirmar el borrado`
- AND the branch is not deleted and the state is unchanged

#### Scenario: Exact word still deletes
- GIVEN the same prompt with `BORRAR` typed
- WHEN the user presses Enter
- THEN the branch is deleted and no wrong-word notice is shown

#### Scenario: Notice clears
- GIVEN the wrong-word notice is displayed
- WHEN the user edits the buffer or presses `esc`
- THEN the notice is no longer rendered

#### Scenario: Notice is visible in both entry points
- GIVEN the strong confirmation reached from the current run's branch deletion, and from the cleanup list
- WHEN a wrong word is submitted in each
- THEN each view renders the same wrong-word notice

## REMOVED Requirements

None.
