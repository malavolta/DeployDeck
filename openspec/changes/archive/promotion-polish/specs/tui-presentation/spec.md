# Delta for TUI Presentation

## MODIFIED Requirements

### Requirement: Semantic Color And Visual Hierarchy

The system SHALL render status tokens (`OK`/`!!`/`XX`/`i`) with semantic color in a real TTY using the BRIGHT ANSI palette (bright-green `10` for OK, bright-yellow `11` for warning, bright-red `9` for error/blocking) rather than the base ANSI palette, for visibility on dark-theme terminals, and SHALL render byte-identical plain text under `go test` (forced Ascii color profile), so existing assertions hold unchanged.
(Previously: status colors used the base ANSI palette — green `2`, amber `3`, red `1` — which rendered muted and hard to distinguish on dark-theme terminals.)

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

## ADDED Requirements

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
