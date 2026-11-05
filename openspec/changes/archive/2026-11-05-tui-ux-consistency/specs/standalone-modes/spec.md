# Standalone Modes Delta (tui-ux-consistency)

## ADDED Requirements

### Requirement: Branch Cleanup Entry In The Main Menu

The main menu MUST include an entry labeled "Limpieza de ramas", placed directly after "Historial de runs", with a one-line Spanish description. Selecting it MUST open the branch cleanup screen, regardless of the ticket input content (including a ticket pre-filled from the branch name). The `b` shortcut MUST keep working as before.

#### Scenario: Entry order
- GIVEN the main menu renders
- WHEN the entry labels are read in order
- THEN "Limpieza de ramas" appears immediately after "Historial de runs"

#### Scenario: Entry opens cleanup with a pre-filled ticket
- GIVEN the ticket input holds a pre-filled value
- AND the menu cursor is on "Limpieza de ramas"
- WHEN the user presses Enter
- THEN the state becomes the branch cleanup screen

#### Scenario: b shortcut remains
- GIVEN the ticket input is empty
- WHEN the user presses `b`
- THEN the branch cleanup screen opens as before

#### Scenario: Entry has a description
- GIVEN the main menu renders
- WHEN the "Limpieza de ramas" line is shown
- THEN it carries a one-line Spanish description, consistent with other entries

### Requirement: Ticket Input Esc Returns To The Main Menu

Esc on the ticket input MUST return to the main menu, not to the prerequisite (Doctor) screen. The typed buffer rules of `tui-presentation` apply.

#### Scenario: Esc on ticket goes to the menu
- GIVEN the ticket input reached from the main menu
- WHEN the user presses `esc`
- THEN the state is `StateMainMenu`

#### Scenario: Esc with text cancels and returns to the menu
- GIVEN the ticket input with typed text
- WHEN the user presses `esc`
- THEN the state is `StateMainMenu` and no quit command is returned

## MODIFIED Requirements

### Requirement: Main Menu As Post-Prereq Landing

After prerequisites pass with no resumable run, the system SHALL show a menu: promote a ticket, generate a delta package, validate a package, run history, and branch cleanup. Selecting "promote a ticket" SHALL enter the existing full flow unchanged. On the main menu both `esc` and `q` SHALL quit.
(Previously: the menu listed three entries.)

#### Scenario: No resumable run shows the menu
- GIVEN prerequisites pass and no resumable run exists
- WHEN startup completes
- THEN the menu is shown with the implemented entries, including "Historial de runs" and "Limpieza de ramas"

#### Scenario: Promote entry enters the unchanged full flow
- GIVEN the menu is shown
- WHEN the user selects "promote a ticket"
- THEN the full promotion flow starts exactly as before this change

#### Scenario: Esc and q quit from the menu
- GIVEN the main menu
- WHEN the user presses `esc`, and separately `q`
- THEN each returns the quit command

### Requirement: Standalone Delta Generates A Package Without Cherry-Picks

Delta mode SHALL let the user choose a base branch, using current `HEAD` as the ref (display-only), and SHALL generate the package and per-type summary as the full flow's delta step does (HU-007/HU-008), WITHOUT any cherry-pick. When Enter is pressed and cannot proceed, the system SHALL show a Spanish notice stating why instead of doing nothing. In the delta source picker `esc` SHALL go back to the menu and `q` SHALL return the quit command.
(Previously: Enter that could not proceed was a silent no-op, and `q` in the picker went back.)

#### Scenario: Delta mode generates and summarizes the package
- GIVEN a repo with a hand-prepared branch
- WHEN the user picks delta mode and a base branch
- THEN `package.xml` is generated under `.deploydeck/manifest/` with a per-type summary, no cherry-pick performed (HU-018 Test E2E)

#### Scenario: Empty delta reuses the existing warning
- GIVEN the base branch produces no changes against the current ref
- WHEN the delta is generated
- THEN the existing empty-delta warning (HU-007/HU-008) is shown

#### Scenario: Enter that cannot proceed explains why
- GIVEN standalone delta where Enter cannot start generation
- WHEN the user presses Enter
- THEN the view contains a Spanish notice and the state is unchanged

#### Scenario: Picker esc and q
- GIVEN the delta source picker
- WHEN the user presses `esc`
- THEN the state is `StateMainMenu`
- AND WHEN the user presses `q` instead
- THEN the quit command is returned

## REMOVED Requirements

None.
