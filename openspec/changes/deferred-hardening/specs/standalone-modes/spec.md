# Delta for Standalone Modes

## ADDED Requirements

### Requirement: Standalone Entry Blocked While A Git Operation Is In Progress

Deferred follow-up from HU-018's adversarial review (`docs/HISTORIAS.md:1116-1149`).

Entering a standalone mode (delta or validation) SHALL be blocked with an actionable message when a git operation is in progress (an unresolved cherry-pick). Neither delta nor validation SHALL be launched while blocked. When no operation is in progress, entry SHALL proceed normally.

#### Scenario: Entry blocked with an in-progress cherry-pick
- GIVEN an unresolved cherry-pick is in progress
- WHEN the user selects standalone delta or standalone validation
- THEN entry is blocked with an actionable message
- AND neither delta nor validation is launched

#### Scenario: Entry proceeds normally with no operation in progress
- GIVEN no git operation is in progress
- WHEN the user selects standalone delta or standalone validation
- THEN the mode is entered normally

### Requirement: Standalone Runs Are Distinguishable And Collision-Free

Deferred follow-up from HU-018's adversarial review (`docs/HISTORIAS.md:1116-1149`).

Two standalone delta generations from DIFFERENT base branches SHALL produce distinct outputs; neither SHALL share or overwrite the other's package directory. Standalone runs SHALL render with a mode-distinct label rather than an empty ticket/target placeholder.

#### Scenario: Different base branches produce distinct outputs
- GIVEN a standalone delta was generated from base branch A
- WHEN a second standalone delta is generated from a different base branch B
- THEN the two runs produce distinct package outputs, neither overwriting the other

#### Scenario: Standalone runs render with a mode-distinct label
- GIVEN a standalone delta or validation run exists
- WHEN the run is rendered in run-history or the detail panel
- THEN it shows a mode-distinct label (e.g. `[delta]` or `[validate]`) instead of an empty ticket/target placeholder
