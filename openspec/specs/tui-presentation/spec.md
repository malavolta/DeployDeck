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

Every screen's footer SHALL follow one consistent grammar and SHALL list only keys that act in that state (see `Footer Lists Exactly The Keys That Act`). A screen SHALL offer a quit or exit affordance according to the esc/q rule by category (see `Esc And Q Rule Per Screen Category`); screens where quitting is unsafe (in-flight states) SHALL instead show `ctrl+c interrumpe`. `q` SHALL keep its two meanings and stay typeable on the free-text screens (ticket, `CANCELAR`, `DESPLEGAR`, `BORRAR`, package path) via guarded handling: append when the buffer is non-empty, otherwise act as `esc` and never quit.

(Previously: every footer always exposed an exit affordance, and `q` on an empty free-text buffer could quit the app.)

#### Scenario: `q` is appended into a non-empty free-text buffer
- GIVEN the ticket screen with non-empty typed text
- WHEN the user presses `q`
- THEN `q` is appended to the buffer and the screen does not quit

#### Scenario: `q` on an empty free-text buffer goes back, not out
- GIVEN the ticket screen with an empty buffer
- WHEN the user presses `q`
- THEN the screen returns to the main menu and no quit command is returned

#### Scenario: In-flight footer replaces the quit affordance
- GIVEN an in-flight state
- WHEN the view renders
- THEN the footer shows `ctrl+c interrumpe` and offers no `q` or `esc`

### Requirement: Actionable Empty States

An empty result SHALL explain why it is empty and what to do next, and
SHALL NOT advertise keys that only apply to populated rows.

#### Scenario: Zero commits found
- GIVEN a ticket search yields no discovered commits
- WHEN the commit-selection screen renders
- THEN it shows why and the available next action
- AND the footer omits the "Space marcar" hint

### Requirement: Correct Spanish Copy

All user-facing strings SHALL be in Spanish with correct tildes/accents and `¿`/`¡` punctuation, matching `viewSourceConfirm`'s reference phrasing. No English user-facing copy SHALL remain on screen, including notices, status words and field labels. Internal identifiers, log lines and wrapped error text that are not shown as UI copy MAY stay in English. Known exception (follow-up): the conflict continue-gate pending reasons produced by `internal/git` remain English in this change (ADR-9 cut).

(Previously: only tildes and inverted punctuation were required; English notices and labels remained.)

#### Scenario: Validation-result title is accented
- GIVEN a run reaches a terminal validation state
- WHEN the result screen renders
- THEN its title reads "Resultado De Validación"

#### Scenario: Yes/No prompts use inverted question marks
- GIVEN a destructive confirmation rendered as a question
- WHEN it renders
- THEN it opens with `¿` and closes with `?`

#### Scenario: English labels are replaced
- GIVEN the plan preview with destructive changes, the cleanup list with a pushed and a local-only branch, and a validating job
- WHEN each view renders
- THEN none contains the English fragments `Destructive changes`, `pushed`, `local`, `validating` or `title:`
- AND each renders a Spanish equivalent

#### Scenario: Former English notices are Spanish
- GIVEN each state that previously set an English notice (the sites listed in the exploration)
- WHEN the triggering key is pressed
- THEN the rendered notice is in Spanish

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

### Requirement: Esc And Q Rule Per Screen Category

Every state MUST follow the esc/q rule of its category:

| Category | esc | q |
|---|---|---|
| Main menu | quit | quit |
| Pickers and sub-screens (delta source, sandbox, run history, branch cleanup, gate-blocked, branch collision, push prep before push) | back to the parent state | quit command |
| Main-flow screens (source confirm through push) | back only where nothing has happened yet (source confirm, commit selection, target, plan, queue); on every later screen esc does nothing | quit command |
| In-flight screens (commit discovery, branch creation, cherry-picking) | none | none |
| Terminal results (Succeeded, Aborted, Failed, Canceled, Error) | quit | quit (Enter also quits) |
| Typed confirmations and text input (ticket, package path, CANCELAR, DESPLEGAR, BORRAR) | cancel and go back (ticket goes to the main menu) | typed into the buffer when it has text; when empty, same as esc. Never quits. |
| y/n confirmations | cancel and go back | cancel and go back (same as esc; never quits from inside a pending decision) |

"Back to the parent" means: branch cleanup goes to the main menu, push prep goes to Succeeded, history and the other pickers go to the state that opened them. Quit via `q` MUST use `quitCmd()`, so restore is skipped while a cherry-pick is in progress and a Salesforce job is left active.

#### Scenario: Main menu esc and q both quit
- GIVEN `StateMainMenu`
- WHEN the user presses `esc`, and separately `q`
- THEN each returns the quit command

#### Scenario: q in a picker quits, esc goes back
- GIVEN `StateRunHistory` opened from the main menu
- WHEN the user presses `q`
- THEN the quit command is returned and the state is unchanged
- AND WHEN instead the user presses `esc`
- THEN the state becomes `StateMainMenu` and no quit command is returned

#### Scenario: Esc is a no-op after a main-flow point of no return
- GIVEN a main-flow state past plan and queue (for example package review or push)
- WHEN the user presses `esc`
- THEN the state is unchanged, no command is returned, and the footer does not mention `esc`

#### Scenario: Esc goes back where nothing has happened yet
- GIVEN commit selection
- WHEN the user presses `esc`
- THEN the model returns to the previous screen and no quit command is returned

#### Scenario: q quits from a main-flow screen without destroying work
- GIVEN a main-flow state with a persisted run
- WHEN the user presses `q`
- THEN the quit command is returned
- AND the run record is not deleted, so the run is still offered for resume on the next launch

#### Scenario: Esc and q do nothing in flight
- GIVEN each of `StateCommitDiscovery`, `StateBranchCreation` and `StateCherryPicking`
- WHEN the user presses `esc`, and separately `q`
- THEN the state is unchanged and no quit command is returned

#### Scenario: Terminal screens quit on esc, q and Enter
- GIVEN each of Succeeded, Aborted, Failed, Canceled and Error
- WHEN the user presses `esc`, `q` or Enter
- THEN each returns the quit command

#### Scenario: Typed confirmation never quits on q
- GIVEN the CANCELAR, DESPLEGAR or BORRAR prompt, or ticket input, with a non-empty buffer
- WHEN the user presses `q`
- THEN `q` is appended to the buffer and no quit command is returned
- AND WHEN the buffer is empty and the user presses `q`
- THEN the screen behaves as on `esc` (cancel and go back) and no quit command is returned

#### Scenario: y/n confirmation
- GIVEN a y/n confirmation state
- WHEN the user presses `esc`, and separately `q`
- THEN both cancel the confirmation and go back, and no quit command is returned

### Requirement: Ctrl+C Remains A Hard Interrupt

`ctrl+c` MUST keep its current behavior in every state, including in-flight states: it interrupts the program without the restore step, regardless of the esc/q rule.

#### Scenario: Ctrl+C interrupts in flight
- GIVEN `StateCherryPicking`
- WHEN the user presses `ctrl+c`
- THEN the interrupt command is returned, as before this change

#### Scenario: Ctrl+C is unaffected on typed inputs
- GIVEN a typed-confirmation state with a non-empty buffer
- WHEN the user presses `ctrl+c`
- THEN the interrupt command is returned and the buffer is not appended to

### Requirement: Footer Lists Exactly The Keys That Act

The footer of every state MUST list every key that has an effect in that state and MUST NOT list a key that has no effect there. A table-driven test MUST pin each state's footer against its key handler: for each state and each key token in the footer, pressing the key changes the model or returns a command; and for each key the handler acts on, the footer names it. Footer key tokens SHOULD be pinned as fragments (for example `c cancelar`), not as the full line. Known required fixes:

- validation polling: lists `c` (cancel own job);
- Succeeded and Aborted: list `d` (delete the branch) only when a branch exists;
- commit selection: lists `↑/↓` and `x`;
- push-ready: lists `a` when AI is configured and does not list it when it is not;
- in-flight screens: show "ctrl+c interrumpe" and do not show `q` or `esc`;
- main-flow screens past the point of no return: do not show `esc`, and show that quitting keeps the run resumable or the job active (fragments `el run queda reanudable` and `el job sigue activo` where they apply).

#### Scenario: Polling footer lists cancel
- GIVEN the validation polling state
- WHEN the view renders
- THEN the footer contains the fragment `c cancelar`
- AND pressing `c` has an effect

#### Scenario: Terminal footers list delete only with a branch
- GIVEN Succeeded with a promotion branch
- WHEN the view renders
- THEN the footer contains `d` for deleting the branch
- AND with no branch the footer does not contain it

#### Scenario: Commit selection footer lists navigation and x
- GIVEN commit selection with at least one commit
- WHEN the view renders
- THEN the footer contains `↑/↓` and an `x` entry, and each key acts

#### Scenario: Push-ready footer follows AI configuration
- GIVEN the push-ready state with AI configured
- WHEN the view renders
- THEN the footer lists `a`
- AND with AI not configured the footer does not list `a` and `a` has no effect

#### Scenario: In-flight footer states the exit affordance
- GIVEN `StateCherryPicking`
- WHEN the view renders
- THEN the footer contains `ctrl+c interrumpe` and does not contain `q salir`

#### Scenario: Footer parity matrix
- GIVEN the table of every state with its expected key tokens
- WHEN each state is rendered and each listed token and each handled key is exercised
- THEN no listed key is inert and no acting key is missing from the footer

### Requirement: Spanish Notice For Blocked Keys

When a key is recognized in a state but cannot act, the system MUST set a short Spanish notice rendered on the screen instead of doing nothing silently. The notice MUST clear on the next navigation, consistent with the existing guard-notice behavior. This applies at least to:

- `p` and `d` when there is no branch;
- Enter in standalone delta when it cannot proceed;
- `x` and `r` on ineligible history rows (see `run-history`).

#### Scenario: p with no branch shows a notice
- GIVEN a state that handles `p` and has no promotion branch
- WHEN the user presses `p`
- THEN the state is unchanged and the view contains a Spanish notice naming the missing branch (fragment `rama`)

#### Scenario: d with no branch shows a notice
- GIVEN a terminal state with no promotion branch
- WHEN the user presses `d`
- THEN no delete flow starts and the view contains a Spanish notice (fragment `rama`)

#### Scenario: Enter in standalone delta shows a notice
- GIVEN standalone delta where Enter cannot proceed
- WHEN the user presses Enter
- THEN the state is unchanged and the view contains a Spanish notice stating why

#### Scenario: Notice clears on navigation
- GIVEN a blocked-key notice is displayed
- WHEN the user navigates to another screen
- THEN the new screen does not render the notice
