# TUI Presentation Delta (tui-ux-polish-b2)

Work-unit tags: each requirement carries `[U1]`..`[U5]`. A cut removes whole requirements by tag. U6 (faster quit detector in slow tests) and U7 (shared enablement predicates between `keymap.go` and `keys.go`) are test-only or internal refactors with no observable behavior, so they have NO spec requirements. Their acceptance is "the full suite stays green and parity (U1) still passes". Tags are provenance only and are dropped on merge into `openspec/specs/tui-presentation/spec.md`.

All scenarios run through `Model.Update` and `Model.View` under the Ascii color profile (package `TestMain`), with the injected clock (`Deps.Now`) faked where time matters. Footer fragments are asserted as substrings of the rendered view. Size in scenarios is set by sending `tea.WindowSizeMsg`.

## ADDED Requirements

### Requirement: [U1] Parity Fixtures Cover Every Reachable Sub-Phase And A Sentinel Guards New States

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

### Requirement: [U1] Parity Is Strict Except For An Explicit Blocked-Notice List

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

### Requirement: [U1] Ticket `b` Is An Allowed Guarded Printable Key

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

### Requirement: [U2] Window Size Is Tracked

`tea.WindowSizeMsg` MUST set the model's width and height and return no command. A width or height of 0 (the initial value, or a zero-sized message) MUST leave all output byte-identical to output without windowing or truncation.

#### Scenario: Size message is stored
- GIVEN any state
- WHEN `tea.WindowSizeMsg{Width: 80, Height: 24}` is sent
- THEN no command is returned and later list views use width 80 and height 24

#### Scenario: Zero size means unchanged output
- GIVEN a list longer than any terminal and a model with width 0 and height 0
- WHEN the view renders
- THEN the output is byte-identical to the output before this change and contains no `más` indicator

### Requirement: [U2] Window, Overflow And Truncate Helpers

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

### Requirement: [U3] `?` Opens A Help Overlay With Every Binding Of The State

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

### Requirement: [U3] `?` Is Unbound In Every Text-Input State

In text-input states (ticket, package path, `CANCELAR`, `DESPLEGAR`, `BORRAR`), `?` MUST NOT be a binding, MUST be appended to the buffer like any printable character, and MUST NOT open the overlay or be advertised. This tightens `Printable Bindings Stay Inactive In Text-Input States`: the only guarded printable keys remain `q` and ticket `b`.

#### Scenario: `?` is typed
- GIVEN each text-input state with a non-empty buffer
- WHEN the user presses `?`
- THEN `?` is appended to the buffer and no overlay opens

#### Scenario: `?` on an empty buffer is also typed
- GIVEN a text-input state with an empty buffer
- WHEN the user presses `?`
- THEN `?` is appended and no overlay opens

### Requirement: [U3] Footers Advertise `? ayuda` Where It Acts, With Deliberate Pin Changes

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

### Requirement: [U4] Spinner Covers Push, PR, Queue, Cleanup And Gate Check

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

## MODIFIED Requirements

### Requirement: [U1] Printable Bindings Stay Inactive In Text-Input States

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

### Requirement: [U1] Structural Footer Parity Over Every State

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

### Requirement: [U4] Progress Feedback For Long-Running Operations

The system SHALL show an animated spinner during commit discovery, branch creation, cherry-picking, delta generation, validation start, push, PR creation, queue loading, branch-cleanup loading and the quick-deploy gate check.
(Previously: only the first five phases.)

#### Scenario: Spinner animates during delta generation
- GIVEN the delta-generation screen
- WHEN a spinner tick fires
- THEN the view renders a spinner frame
- AND the next tick renders a different frame

## REMOVED Requirements

None.
