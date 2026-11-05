# Capability: TUI Presentation

## Overview

Cross-cutting presentation rules for the DeployDeck TUI: color, a
context bar, footer grammar, empty states, Spanish copy, progress
feedback, table headers, and menu descriptions. Domain capabilities
keep their behavior; this capability governs only how it renders.

## Requirements

### Requirement: Semantic Color And Visual Hierarchy

The system SHALL render status tokens (`OK`/`!!`/`XX`/`i`) with semantic color in a real TTY, choosing the color by terminal background. On a dark background it SHALL use the BRIGHT ANSI palette (bright-green `10` for OK, bright-yellow `11` for warning, bright-red `9` for error/blocking). On a light background it SHALL use the darker base ANSI shades (green `2` for OK, yellow `3` for warning, red `1` for error/blocking) for contrast. The system SHALL render byte-identical plain text under `go test` (forced Ascii color profile), so existing assertions hold unchanged.
(Previously: the bright palette was used on every background, which is hard to read on light terminals.)

#### Scenario: Dark background keeps the bright palette
- GIVEN a dark terminal background
- WHEN OK, warning and error status tokens render in a real TTY
- THEN they use ANSI `10`, `11` and `9` respectively

#### Scenario: Light background uses darker shades
- GIVEN a light terminal background
- WHEN OK, warning and error status tokens render in a real TTY
- THEN they use ANSI `2`, `3` and `1` respectively

#### Scenario: Ascii profile output is unchanged
- GIVEN the Ascii color profile used by `go test`
- WHEN any status token renders, on either background setting
- THEN the view contains the same plain `OK`/`!!`/`XX`/`i` text with no ANSI escape codes

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

The footer of every state MUST list every key that has an effect in that state and MUST NOT list a key that has no effect there. The footer MUST be derived from the same enabled bindings the handler matches (see `Footers And Handlers Share One Per-State Keymap`), and a structural test over every state MUST pin that parity (see `Structural Footer Parity Over Every State`). Footer key tokens SHOULD still be pinned in tests as fragments (for example `c cancelar`), not as the full line. The fixes required by `tui-ux-consistency` remain in force: validation polling lists `c`; Succeeded and Aborted list `d` only when a branch exists; commit selection lists `↑/↓` and `x`; push-ready lists `a` only when AI is configured; in-flight screens show `ctrl+c interrumpe` and no `q` or `esc`; main-flow screens past the point of no return show no `esc`; the conflict state lists `c` only when continue is enabled.
(Previously: parity was pinned by a table-driven probe over a subset of sub-phases, and the conflict footer listed `c` while continue was disabled.)

#### Scenario: Footer parity matrix is structural
- GIVEN every state with its enabled bindings
- WHEN each state is rendered and each key is exercised
- THEN no listed key is inert and no acting key is missing from the footer, without relying on a hand-written token table

#### Scenario: Polling footer lists cancel
- GIVEN the validation polling state
- WHEN the view renders
- THEN the footer contains the fragment `c cancelar` and pressing `c` has an effect

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

### Requirement: Footers And Handlers Share One Per-State Keymap

For every state, the system MUST define one set of key bindings, and both the key handler and the footer of that state MUST be derived from that same set. A key that the handler acts on MUST be one of the state's enabled bindings, and the footer MUST list exactly the enabled bindings that carry help text. A binding that cannot act in the current model state (for example no branch, no commits, AI not configured, continue disabled) MUST be disabled: the footer MUST NOT list it and the handler MUST NOT act on it. Pressing a disabled key MUST NOT change the state; where a Spanish blocked-key notice is already required (see `Spanish Notice For Blocked Keys`), that notice remains the only effect.

#### Scenario: Conflict footer hides `c` while continue is disabled
- GIVEN the conflict state with the continue gate reporting pending reasons (continue disabled)
- WHEN the view renders
- THEN the footer does not contain a `c` entry
- AND WHEN the user presses `c`
- THEN the state is unchanged and no command is returned

#### Scenario: Conflict footer shows `c` once continue is enabled
- GIVEN the conflict state with the continue gate satisfied
- WHEN the view renders
- THEN the footer contains a `c` entry
- AND pressing `c` acts (continues the cherry-pick)

#### Scenario: A key disabled by model state is neither shown nor acted on
- GIVEN push-ready without AI configured, and a terminal state with no promotion branch
- WHEN each view renders
- THEN the push-ready footer has no `a` entry and the terminal footer has no `d` entry
- AND pressing the corresponding key does not start the action

#### Scenario: Keys an enabled binding names do act
- GIVEN any state with its enabled bindings
- WHEN each enabled binding's key is pressed
- THEN the model changes or a command is returned

### Requirement: Footer Grammar Stays Byte-Identical

The footer SHALL keep the existing grammar: `[ ` + entries + ` ]` where each entry is `<key> <description>` and entries are separated by exactly three spaces (`[ k desc   k desc ]`). Rendering the footer from bindings MUST NOT alter the bytes of any footer produced before this change, except where a binding is newly disabled by the first requirement of this delta (the conflict `c` case). A state with no enabled help-bearing binding SHALL render no footer line, as before.

#### Scenario: Footer bytes are unchanged
- GIVEN the main menu, commit selection with commits, and run history with rows
- WHEN each view renders
- THEN each footer line matches its pre-change bytes, including the `[ ` prefix, the ` ]` suffix and the three-space separator

#### Scenario: Entries are separated by three spaces
- GIVEN a state whose footer has two or more entries
- WHEN the view renders
- THEN adjacent entries are separated by exactly three spaces and no other separator appears

### Requirement: Structural Footer Parity Over Every State

A single structural test MUST range over every state the model can render (every state handled by `viewBody` and `handleKey`, including sub-phases of states with several phases) and MUST verify, for each: every key token in the rendered footer corresponds to an enabled binding that acts, and every enabled binding appears in the footer when it has help text. A state that has no registered bindings MUST fail the test, so adding a state without a keymap is detected. This test replaces the 12-sub-phase parity probe from `tui-ux-consistency`.

#### Scenario: Every state is covered
- GIVEN the list of all model states and their sub-phases
- WHEN the parity test runs
- THEN each has a bindings entry, and a missing entry fails the test

#### Scenario: No listed key is inert and no acting key is hidden
- GIVEN each state populated so its bindings are representative
- WHEN each footer token and each enabled binding is exercised
- THEN no listed key is inert and no enabled, help-bearing binding is missing from the footer

### Requirement: Printable Bindings Stay Inactive In Text-Input States

In states that consume typed text (ticket input, package path, and the typed confirmations `CANCELAR`, `DESPLEGAR` and `BORRAR`), the keymap MUST NOT contain bindings on printable keys other than the existing guarded `q` handling. Typing any printable character MUST append it to the buffer exactly as before this change, and the footer MUST NOT advertise a printable shortcut there.

#### Scenario: Printable characters reach the buffer
- GIVEN each text-input state with a non-empty buffer
- WHEN the user types a letter that is a binding in other states (for example `c`, `d`, `x`, `p`)
- THEN the letter is appended to the buffer and no other effect occurs

#### Scenario: `q` keeps its guarded behavior
- GIVEN a text-input state
- WHEN the user presses `q` with a non-empty buffer, and separately with an empty buffer
- THEN `q` is appended in the first case and acts as `esc` in the second, never quitting
