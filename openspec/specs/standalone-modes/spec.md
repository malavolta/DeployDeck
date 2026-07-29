# Standalone Modes

## Purpose

HU-018 (`docs/HISTORIAS.md:1116-1149`). Lets a developer generate a delta or validate an existing `package.xml` against a sandbox without the full promotion flow, for hand-prepared branches. `StateMainMenu` is the post-prereq landing; both modes reuse existing delta (HU-007/HU-008) and validation (HU-010/HU-011) machinery as-is, adding no new behavior.

## Requirements

### Requirement: Main Menu As Post-Prereq Landing

After prerequisites pass with no resumable run, the system SHALL show a menu: promote a ticket, generate a delta package, validate a package. Selecting "promote a ticket" SHALL enter the existing full flow unchanged.

#### Scenario: No resumable run shows the menu
- GIVEN prerequisites pass and no resumable run exists
- WHEN startup completes
- THEN the menu is shown with the three entries

#### Scenario: Promote entry enters the unchanged full flow
- GIVEN the menu is shown
- WHEN the user selects "promote a ticket"
- THEN the full promotion flow starts exactly as before this change

### Requirement: Resume-Detection Preserved After Menu Landing

Resume-detection SHALL still run before the menu is shown; a detected resumable run SHALL still be offered for resume as before (HU-013), and the menu SHALL appear only when nothing is resumable.

#### Scenario: Resumable run offers resume instead of the menu
- GIVEN a resumable run exists (in-progress cherry-pick or non-terminal jobId)
- WHEN DeployDeck starts and prerequisites pass
- THEN resume is offered, not the menu (HU-013 regression guard)

### Requirement: Unimplemented Modes Hidden From The Menu

The system SHALL build the menu's entries from only implemented modes; an unimplemented mode SHALL NOT appear (HU-018 AC3, `docs/HISTORIAS.md:1141`).

#### Scenario: Implemented modes appear and work
- GIVEN delta and validation are both implemented
- WHEN the menu is shown
- THEN both entries appear and are operational when selected

#### Scenario: An unimplemented mode is hidden
- GIVEN a mode is not yet implemented
- WHEN the menu is shown
- THEN that entry does not appear

### Requirement: Standalone Delta Generates A Package Without Cherry-Picks

Delta mode SHALL let the user choose a base branch, using current `HEAD` as the ref (display-only), and SHALL generate the package and per-type summary as the full flow's delta step does (HU-007/HU-008), WITHOUT any cherry-pick.

#### Scenario: Delta mode generates and summarizes the package
- GIVEN a repo with a hand-prepared branch
- WHEN the user picks delta mode and a base branch
- THEN `package.xml` is generated under `.deploydeck/manifest/` with a per-type summary, no cherry-pick performed (HU-018 Test E2E)

#### Scenario: Empty delta reuses the existing warning
- GIVEN the base branch produces no changes against the current ref
- WHEN the delta is generated
- THEN the existing empty-delta warning (HU-007/HU-008) is shown

### Requirement: Standalone Validation Launches And Polls Like The Full Flow

Validation mode SHALL let the user choose an existing `package.xml` and a sandbox, and SHALL launch async validation and poll to a terminal state as the full flow's validation step does (HU-010/HU-011).

#### Scenario: Validation mode launches and polls to terminal
- GIVEN an existing `package.xml` and a sandbox
- WHEN the user picks validation mode and confirms
- THEN validation launches, `jobId` is captured, and polling reaches a terminal state like the full flow (HU-018 Test E2E)

### Requirement: Invalid Or Nonexistent Package Rejected Before Launch

Before launching standalone validation, the system SHALL check the selected `package.xml` exists and parses; otherwise it SHALL show an actionable error and SHALL NOT launch.

#### Scenario: Nonexistent package.xml blocks the launch
- GIVEN the selected `package.xml` path does not exist
- WHEN validation mode is confirmed
- THEN an actionable error is shown and validation is not launched

#### Scenario: Invalid package.xml blocks the launch
- GIVEN the selected `package.xml` fails to parse
- WHEN validation mode is confirmed
- THEN an actionable error is shown and validation is not launched

### Requirement: Standalone Modes Create A Local Run

Both standalone modes SHALL create a local run under `.deploydeck/runs/`, tagged with the mode that created it (see `run-persistence`'s `Mode` field).

#### Scenario: Delta mode creates a run
- GIVEN delta mode finishes generating a package
- WHEN the run is persisted
- THEN a local run is created, tagged as a delta-mode run

#### Scenario: Validation mode creates a run
- GIVEN validation mode captures a `jobId`
- WHEN the run is persisted
- THEN a local run is created, tagged as a validation-mode run

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
