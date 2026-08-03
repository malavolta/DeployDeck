# Capability: TUI Presentation

## Overview

Cross-cutting presentation rules for the DeployDeck TUI: color, a
context bar, footer grammar, empty states, Spanish copy, progress
feedback, table headers, and menu descriptions. Domain capabilities
keep their behavior; this capability governs only how it renders.

## Requirements

### Requirement: Semantic Color And Visual Hierarchy

The system SHALL render status tokens (`OK`/`!!`/`XX`/`i`) with
semantic color in a real TTY using the BRIGHT ANSI palette (bright-green
`10` for OK, bright-yellow `11` for warning, bright-red `9` for
error/blocking) rather than the base ANSI palette, for visibility on
dark-theme terminals, and SHALL render byte-identical plain text under
`go test` (forced Ascii color profile), so existing assertions hold
unchanged.
(Previously: status colors used the base ANSI palette — green `2`, amber
`3`, red `1` — which rendered muted and hard to distinguish on dark-theme
terminals.)

#### Scenario: Failed status is colored at runtime, plain under test
- GIVEN a `Failed`/blocking status rendered in a real TTY
- WHEN the row renders
- THEN its `XX` marker uses the bright semantic error color (ANSI `9`)
- AND the identical view under `go test` contains the same plain `XX`
  substring, with no ANSI escape codes

#### Scenario: OK status uses the bright green variant at runtime
- GIVEN an `OK` status rendered in a real TTY
- WHEN the row renders
- THEN its marker uses the bright semantic success color (ANSI `10`), not the base-palette green
- AND the identical view under `go test` contains the same plain `OK`
  substring, with no ANSI escape codes

### Requirement: Persistent Context Bar

The system SHALL show `repo · branch` on every screen, appending `· org`
only once a target org is resolved (omit-until-resolved). It MUST NOT
show a placeholder for an unresolved org.

#### Scenario: Main menu shows repo and branch only
- GIVEN the main menu, before any org is resolved
- WHEN it renders
- THEN it shows `repo · branch` with no org segment

#### Scenario: Org appears once resolved
- GIVEN a screen reached after org resolution (e.g. plan preview or
  validation)
- WHEN it renders
- THEN it shows `repo · branch · org`

### Requirement: Standardized Footer Grammar And Always-Offer-Quit

Every screen's footer SHALL follow one consistent grammar and always
expose an exit affordance. `q` SHALL keep its two meanings and stay
typeable on the three free-text screens (ticket, `CANCELAR`,
`DESPLEGAR`) via guarded handling: append when the buffer is
non-empty, act otherwise.

#### Scenario: `q` is appended into a non-empty free-text buffer
- GIVEN the ticket screen with non-empty typed text
- WHEN the user presses `q`
- THEN `q` is appended to the buffer and the screen does not quit

#### Scenario: `q` acts on an empty free-text buffer
- GIVEN the ticket screen with an empty buffer
- WHEN the user presses `q`
- THEN the app quits (or goes back to the previous screen)

### Requirement: Actionable Empty States

An empty result SHALL explain why it is empty and what to do next, and
SHALL NOT advertise keys that only apply to populated rows.

#### Scenario: Zero commits found
- GIVEN a ticket search yields no discovered commits
- WHEN the commit-selection screen renders
- THEN it shows why and the available next action
- AND the footer omits the "Space marcar" hint

### Requirement: Correct Spanish Copy

User-facing Spanish strings SHALL use correct tildes/accents and
`¿`/`¡` punctuation, matching `viewSourceConfirm`'s reference phrasing.

#### Scenario: Validation-result title is accented
- GIVEN a run reaches a terminal validation state
- WHEN the result screen renders
- THEN its title reads "Resultado De Validación"

#### Scenario: Yes/No prompts use inverted question marks
- GIVEN a destructive confirmation rendered as a question
- WHEN it renders
- THEN it opens with `¿` and closes with `?`

### Requirement: Progress Feedback For Long-Running Operations

The system SHALL show an animated spinner during commit discovery,
branch creation, cherry-picking, delta generation, and validation start.

#### Scenario: Spinner animates during delta generation
- GIVEN the delta-generation screen
- WHEN a spinner tick fires
- THEN the view renders a spinner frame
- AND the next tick renders a different frame

### Requirement: Labeled Table Column Headers

The queue, run-history, and branch-cleanup tables SHALL render a header
row labeling each column, aligned to the row values below it.

#### Scenario: Run history shows column labels
- GIVEN at least one recorded run
- WHEN the run-history screen renders
- THEN a header row labels each column, aligned with the data rows

### Requirement: Menu Descriptions And Unified Terminology

Each main-menu entry SHALL show a one-line description alongside its
label, and source-branch terminology SHALL be unified to one term used
consistently across all screens.

#### Scenario: Main menu entries show descriptions
- GIVEN the main menu renders its entries
- WHEN each entry line is shown
- THEN it includes a one-line description of that entry

#### Scenario: Source-branch terminology is consistent
- GIVEN two screens both reference the discovered source branch
- WHEN each renders that reference
- THEN both use the same glossary term for it

### Requirement: Localized Empty-Selection Guard Notice Without Cross-Screen Bleed

WHEN the user attempts to continue from commit selection with zero commits selected, the system SHALL show a guard notice in Spanish, and SHALL clear that notice on any back/navigation transition that leaves the commit-selection screen, so the notice MUST NOT render on a subsequent or unrelated screen.

#### Scenario: Guard notice is shown in Spanish
- GIVEN the user attempts to continue from commit selection with zero commits selected
- WHEN the guard notice renders
- THEN it reads in Spanish (e.g. "selecciona al menos un commit para continuar")

#### Scenario: Notice does not persist after going back
- GIVEN the empty-selection guard notice is currently shown on the commit-selection screen
- WHEN the user declines or goes back to a previous screen (e.g. ticket entry)
- THEN the subsequent screen renders with no stale notice

#### Scenario: Notice does not bleed onto an unrelated screen reached via navigation
- GIVEN the guard notice was shown before the user navigated away
- WHEN the user navigates to a different, unrelated screen (e.g. Doctor)
- THEN that screen does not display the guard notice
