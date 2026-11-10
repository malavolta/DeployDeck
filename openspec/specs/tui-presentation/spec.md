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

The system SHALL show an animated spinner during commit discovery, branch creation, cherry-picking, delta generation, validation start, push, PR creation, queue loading, branch-cleanup loading and the quick-deploy gate check.
(Previously: only the first five phases.)

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

The footer SHALL keep the existing grammar: `[ ` + entries + ` ]` where each entry is `<key> <description>` and entries are separated by exactly three spaces (`[ k desc   k desc ]`). Rendering the footer from bindings MUST NOT alter the bytes of any footer produced before this change, except where a binding is disabled because it cannot act (the conflict `c` case, and history `Enter reanudar` on non-resumable rows) and except for the `   ? ayuda` suffix defined by `Footers Advertise `? ayuda` Where It Acts, With Deliberate Pin Changes` (tui-ux-polish-b2, 2026-11-06). A state with no enabled help-bearing binding SHALL render no footer line, as before.

#### Scenario: Footer bytes are unchanged
- GIVEN the main menu, commit selection with commits, and run history with rows
- WHEN each view renders
- THEN each footer line matches its pre-change bytes, apart from the trailing `   ? ayuda` entry, including the `[ ` prefix, the ` ]` suffix and the three-space separator

#### Scenario: Entries are separated by three spaces
- GIVEN a state whose footer has two or more entries
- WHEN the view renders
- THEN adjacent entries are separated by exactly three spaces and no other separator appears

### Requirement: Structural Footer Parity Over Every State

A single structural test MUST range over every state the model can render, including every reachable sub-phase (see `Parity Fixtures Cover Every Reachable Sub-Phase And A Sentinel Guards New States`), and MUST verify, strictly (see `Parity Is Strict Except For An Explicit Blocked-Notice List`): every key token in the rendered footer corresponds to an enabled binding that acts, and every enabled binding appears in the footer when it has help text. A state with no registered bindings MUST fail the test.
(Previously: a notice counted as acting everywhere, and sub-phase coverage had no sentinel.)

#### Scenario: Every state is covered
- GIVEN the list of all model states and their sub-phases
- WHEN the parity test runs
- THEN each has a fixture and the sentinel matches

#### Scenario: No listed key is inert and no acting key is hidden
- GIVEN each fixture
- WHEN each footer token and each enabled binding is exercised
- THEN no listed key is inert and no enabled, help-bearing binding is missing from the footer

### Requirement: Printable Bindings Stay Inactive In Text-Input States

In states that consume typed text (ticket input, package path, and the typed confirmations `CANCELAR`, `DESPLEGAR` and `BORRAR`), the keymap MUST NOT contain bindings on printable keys other than the existing guarded handling: `q` (all text states) and `b` (ticket input, empty buffer only). Typing any other printable character, including `?`, MUST append it to the buffer exactly as before, and the footer MUST NOT advertise a printable shortcut there.
(Previously: only `q` was named as an allowed guarded key, which left ticket `b` outside the stated rule.)

#### Scenario: Printable characters reach the buffer
- GIVEN each text-input state with a non-empty buffer
- WHEN the user types a letter that is a binding in other states (for example `c`, `d`, `x`, `p`) or `?`
- THEN the character is appended to the buffer and no other effect occurs

#### Scenario: `q` keeps its guarded behavior
- GIVEN a text-input state
- WHEN the user presses `q` with a non-empty buffer, and separately with an empty buffer
- THEN `q` is appended in the first case and acts as `esc` in the second, never quitting

#### Scenario: Ticket `b` is the only other guarded key
- GIVEN the ticket state
- WHEN `b` is pressed with an empty buffer, and separately with a non-empty buffer
- THEN it opens cleanup in the first case and is appended in the second

### Requirement: Parity Fixtures Cover Every Reachable Sub-Phase And A Sentinel Guards New States

The structural parity test MUST include at least one fixture for every reachable sub-phase of every state, including at minimum: a run-history row that is terminal (non-resumable), push-ready after the PR exists with AI wired, the branch-cleanup empty-list prune confirmation, and the queue error. The test MUST declare a `stateCount` sentinel equal to the number of model states; if a state exists that has no fixture, or the sentinel disagrees with the real state count, the test MUST fail. Adding a state without a fixture and without updating the sentinel MUST NOT pass silently.

#### Scenario: New state without a fixture fails
- GIVEN the parity test and a model state value not covered by any fixture
- WHEN the parity test runs
- THEN it fails and names the uncovered state

#### Scenario: Sentinel disagrees with state count
- GIVEN `stateCount` differs from the number of declared states
- WHEN the parity test runs
- THEN it fails

#### Scenario: The four named sub-phases are fixtures
- GIVEN the fixture list
- WHEN it is inspected by the parity test
- THEN it contains a history terminal row, push-ready after the PR with AI wired, the empty-list prune confirm and the queue error

### Requirement: Parity Is Strict Except For An Explicit Blocked-Notice List

For every fixture, every enabled binding's key MUST act (state or phase changes, a command is returned, or the view changes once the notice is cleared) with no leniency for "a notice appeared". The only exception is an explicit, named list of blocked-notice keys (keys that by design set a Spanish notice and nothing else, per `Spanish Notice For Blocked Keys`). A key on that list MAY satisfy parity by producing its notice. Text-input fixtures MUST be populated with buffers that make their enabled bindings valid (for example a non-empty ticket for Enter), so no text-state key relies on leniency.

#### Scenario: A key that only sets a notice fails unless listed
- GIVEN a fixture where an enabled binding's key only sets a notice and its key is not on the blocked-notice list
- WHEN the parity test runs
- THEN it fails

#### Scenario: Listed blocked-notice key passes with a notice
- GIVEN a blocked-notice key on the explicit list that is pressed in a state where it is blocked
- WHEN the key is pressed
- THEN the Spanish notice renders and parity accepts it

#### Scenario: Text fixtures use valid buffers
- GIVEN the ticket, package path, CANCELAR, DESPLEGAR and BORRAR fixtures
- WHEN the parity test exercises Enter
- THEN each fixture buffer is valid and Enter acts without relying on a notice

### Requirement: Ticket `b` Is An Allowed Guarded Printable Key

This amends `Printable Bindings Stay Inactive In Text-Input States`. In the ticket input state, `b` on an EMPTY buffer is an allowed guarded printable key, next to `q`: it opens branch cleanup as before. `b` with a non-empty buffer MUST be appended to the buffer. The footer MUST NOT advertise `b` (the footer stays `Enter buscar   Esc volver`), consistent with the `q` precedent. No other text-input state gains a printable binding.

#### Scenario: `b` on an empty ticket buffer opens cleanup
- GIVEN the ticket state with an empty buffer
- WHEN the user presses `b`
- THEN the branch-cleanup flow opens and the buffer is unchanged

#### Scenario: `b` on a non-empty buffer is typed
- GIVEN the ticket state with a non-empty buffer
- WHEN the user presses `b`
- THEN `b` is appended and no other effect occurs

#### Scenario: `b` is not advertised
- GIVEN the ticket state, empty or non-empty buffer
- WHEN the view renders
- THEN the footer contains no `b` entry and equals the existing `Enter buscar   Esc volver` pin

### Requirement: Window Size Is Tracked

`tea.WindowSizeMsg` MUST set the model's width and height and return no command. A width or height of 0 (the initial value, or a zero-sized message) MUST leave all output byte-identical to output without windowing or truncation.

#### Scenario: Size message is stored
- GIVEN any state
- WHEN `tea.WindowSizeMsg{Width: 80, Height: 24}` is sent
- THEN no command is returned and later list views use width 80 and height 24

#### Scenario: Zero size means unchanged output
- GIVEN a list longer than any terminal and a model with width 0 and height 0
- WHEN the view renders
- THEN the output is byte-identical to the output before this change and contains no `más` indicator

### Requirement: Window, Overflow And Truncate Helpers

The system MUST provide pure helpers with these rules.
- `window(cursor, total, rows)`: when `rows <= 0` or `total <= rows`, it returns the whole range `(0, total)`. Otherwise the window is bottom-anchored and the cursor MUST be inside it; moving the cursor down scrolls by the minimum (one row at a time), and moving up the window follows the cursor.
- `overflow(n, up)`: returns a line with `↑ N más` (up) or `↓ N más` (down) when `n > 0`, otherwise nothing.
- `truncate(s, width)`: returns `s` unchanged when `width <= 0`; otherwise cuts to at most `width` display cells with a trailing `…`, counting runes by display width and not breaking ANSI sequences.

#### Scenario: Short list is not windowed
- GIVEN total 5 and rows 10
- WHEN the window is computed
- THEN the range is the whole list

#### Scenario: Cursor at the bottom stays visible
- GIVEN total 50, rows 10 and the cursor at row 49
- WHEN the window is computed
- THEN the range is rows 40 to 49 and the cursor is inside it

#### Scenario: Scrolling down moves one row
- GIVEN total 50, rows 10, window rows 0 to 9 and the cursor at 9
- WHEN the cursor moves to 10
- THEN the window shows rows 1 to 10

#### Scenario: Overflow counts
- GIVEN a window showing rows 20 to 29 of 50
- WHEN the view renders
- THEN it shows `↑ 20 más` above and `↓ 20 más` below, and no indicator on a side with zero hidden rows

#### Scenario: Truncate respects width
- GIVEN a row wider than the terminal width
- WHEN it renders with width 40
- THEN every rendered line is at most 40 cells wide and a cut line ends with `…`
- AND with width 0 the row is unchanged

### Requirement: `?` Opens A Help Overlay With Every Binding Of The State

In every state where `?` is enabled, pressing `?` MUST open an overlay titled `Atajos` that lists ALL bindings of the current state (including bindings with no footer help text), rendered from the same per-state bindings that drive the handler and footer. While the overlay is open, it MUST replace the state body, and keys other than `?`, Esc and `ctrl+c` MUST NOT act on the underlying state. `?` or Esc MUST close it and restore the state unchanged, and Esc MUST NOT also trigger the state's own Esc action. `ctrl+c` keeps its hard-interrupt behavior. `?` is enabled in every state except text-input states and in-flight states.

#### Scenario: Overlay lists every binding
- GIVEN run history with rows
- WHEN the user presses `?`
- THEN the view contains `Atajos` and one entry per binding of that state, including ones not shown in the footer

#### Scenario: `?` closes it
- GIVEN the overlay is open
- WHEN the user presses `?`
- THEN the overlay is gone and the state and cursor are unchanged

#### Scenario: Esc closes it without going back
- GIVEN the overlay is open in a state whose Esc goes back
- WHEN the user presses Esc
- THEN the overlay closes and the state is unchanged

#### Scenario: Other keys do not act through the overlay
- GIVEN the overlay is open over commit selection
- WHEN the user presses Space
- THEN the selection is unchanged

#### Scenario: Ctrl+C still interrupts
- GIVEN the overlay is open
- WHEN the user presses `ctrl+c`
- THEN the interrupt command is returned

### Requirement: `?` Is Unbound In Every Text-Input State

In text-input states (ticket, package path, `CANCELAR`, `DESPLEGAR`, `BORRAR`), `?` MUST NOT be a binding, MUST be appended to the buffer like any printable character, and MUST NOT open the overlay or be advertised. This tightens `Printable Bindings Stay Inactive In Text-Input States`: the only guarded printable keys remain `q` and ticket `b`.

#### Scenario: `?` is typed
- GIVEN each text-input state with a non-empty buffer
- WHEN the user presses `?`
- THEN `?` is appended to the buffer and no overlay opens

#### Scenario: `?` on an empty buffer is also typed
- GIVEN a text-input state with an empty buffer
- WHEN the user presses `?`
- THEN `?` is appended and no overlay opens

### Requirement: Footers Advertise `? ayuda` Where It Acts, With Deliberate Pin Changes

Every footer of a state where `?` is enabled MUST end with the entry `? ayuda`, preserving the footer grammar (`[ k desc   k desc ]`, three-space separator). Footers of text-input states and in-flight states MUST NOT contain it. The structural parity test MUST treat `?` as an ordinary enabled binding (listed and acting). "In-flight" means a screen whose only binding is `ctrl+c interrumpe` (commit discovery, branch creation, cherry-picking, branch-cleanup loading). Screens with `q salir` or `Esc` only (delta running, validation start, pushing, PR creating, cleanup counting) and y/n prompts are NOT in-flight and DO gain `? ayuda`. The footer byte pins (`PinnedFragments` and the byte-pin table) that change are exactly those of the states where `?` is enabled: each gains a trailing `   ? ayuda` before the closing ` ]`. The pins that stay byte-identical are exactly the text-input footers (ticket, package path, CANCELAR, quick-deploy DESPLEGAR, and both strong BORRAR confirmations) and the `ctrl+c interrumpe` footers. The exact lists live in `tasks.md`. (Amended 2026-11-06 to match design ADR-8.) The spec records this list so that the pin edits are deliberate and not drift.

#### Scenario: Enabled state shows the hint
- GIVEN the main menu
- WHEN the view renders
- THEN the footer contains `? ayuda` as its last entry and the entries are separated by exactly three spaces

#### Scenario: Text-input and in-flight footers are unchanged
- GIVEN the ticket input, the package path input, the CANCELAR prompt and an in-flight state
- WHEN each renders
- THEN no footer contains `? ayuda` and each footer is byte-identical to its pre-change pin

#### Scenario: Parity treats `?` as acting
- GIVEN every fixture where `?` is enabled
- WHEN the parity test runs
- THEN `?` is both listed in the footer and acts (opens the overlay)

### Requirement: Spinner Covers Push, PR, Queue, Cleanup And Gate Check

The system SHALL show an animated spinner frame while any of these run: the push phase, PR creation, queue loading, branch-cleanup loading and the quick-deploy gate check, in addition to the phases that already animate. The spinner predicate MUST depend on the active phase (push phases, `cleanupLoading`, queue load, gate check), not only on the top-level state. Exactly one tick loop MUST exist at a time (single-flight), and when the phase ends no further tick MUST be scheduled: a tick received after the phase ended returns no command and stops the loop.

#### Scenario: Spinner animates during push
- GIVEN the model in the push phase
- WHEN a spinner tick fires, and then another
- THEN the view shows a spinner frame and the next tick shows a different frame

#### Scenario: Each new phase animates
- GIVEN PR creation, queue load, cleanup load and the gate check
- WHEN a spinner tick fires in each
- THEN each view renders a spinner frame

#### Scenario: One tick loop only
- GIVEN a spinner already running
- WHEN a second phase that needs a spinner starts
- THEN no second tick loop is started

#### Scenario: No tick after the phase ends
- GIVEN a running spinner whose phase has just completed
- WHEN the pending tick is delivered
- THEN no command is returned and no further tick is scheduled
