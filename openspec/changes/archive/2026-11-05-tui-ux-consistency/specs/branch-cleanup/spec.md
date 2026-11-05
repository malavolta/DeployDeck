# Branch Cleanup Delta (tui-ux-consistency)

## ADDED Requirements

### Requirement: Cleanup Reachable From The Menu And Returns To It

The branch cleanup screen MUST be reachable from the main-menu entry "Limpieza de ramas" (see `standalone-modes`) in addition to the `b` shortcut. `esc` on the cleanup list and result views MUST return to the main menu; `esc` (and `q`) on a confirmation view (y/n, prune, or the typed BORRAR prompt) MUST cancel it and return to the cleanup list. `q` MUST return the quit command. Cleanup footers MUST list `Esc volver` and `q salir` in line with those handlers, and MUST NOT describe `q` as returning to ticket input.

#### Scenario: Esc returns to the menu
- GIVEN the cleanup list opened from the main menu
- WHEN the user presses `esc`
- THEN the state is `StateMainMenu`

#### Scenario: q quits
- GIVEN the cleanup list
- WHEN the user presses `q`
- THEN the quit command is returned

#### Scenario: Footer matches handlers
- GIVEN the cleanup list, the confirm view and the result view
- WHEN each renders
- THEN each footer contains `Esc` for going back and, where `q` acts, `q salir`
- AND each key listed acts

### Requirement: BORRAR Prompt Names The Branch

The typed strong-confirmation prompt for deleting an unpushed branch MUST name the branch being deleted, together with the exact word to type (`BORRAR`). The exact-word gate is unchanged.

#### Scenario: Prompt shows the branch name
- GIVEN the strong confirmation for `deploy/DD-7`
- WHEN the prompt renders
- THEN it contains `deploy/DD-7` and `BORRAR`

#### Scenario: Gate unchanged
- GIVEN the BORRAR prompt for `deploy/DD-7`
- WHEN the user types anything other than `BORRAR` and presses Enter
- THEN the branch is not deleted
- AND WHEN the user types `BORRAR` exactly and presses Enter
- THEN the branch is deleted

#### Scenario: Esc and q inside the prompt
- GIVEN the BORRAR prompt with a non-empty buffer
- WHEN the user presses `q`
- THEN `q` is appended and nothing quits
- AND WHEN the user presses `esc`
- THEN the prompt is cancelled and the cleanup list is shown

## MODIFIED Requirements

### Requirement: Orphan Deploy Branches Listed For Batch Cleanup

The system MUST provide a cleanup screen listing every orphan `deploy/*` branch — one not tied to a live in-progress run — with its age and push status (pushed vs local-only), in Spanish, with column widths at least as wide as their headers (for example `Antigüedad`) (docs/HISTORIAS.md:1097,1105).
(Previously: no language or column-width constraint; the age header overflowed its column, and push status was shown as the English `pushed`/`local`.)

#### Scenario: Orphans listed with age and push status
- GIVEN `deploy/DD-2` (pushed, 10 days old) and `deploy/DD-3` (local-only, 2 days old) are orphans
- WHEN the user opens the branch cleanup screen
- THEN both are listed with their age and a Spanish push status

#### Scenario: In-progress run's branch is excluded
- GIVEN `deploy/DD-4` belongs to a currently in-progress run
- WHEN the cleanup screen is opened
- THEN `deploy/DD-4` is NOT listed as an orphan

## REMOVED Requirements

None.
