# TUI Presentation Delta (tui-bubbles-adoption)

Work-unit tags: each requirement carries its work-unit tag (`[U1]` or `[U5]`). U1, U2 and U5 shipped. U3 and U4 were CUT to B2 (user decision 2026-11-05), so this delta contains no U3 or U4 requirements. The tags are provenance only and are dropped when the requirements are merged into `openspec/specs/tui-presentation/spec.md`.

All scenarios are exercised through `Model.Update` and `Model.View` under the Ascii color profile (package `TestMain`). Footer fragments are asserted as substrings of the rendered view.

## ADDED Requirements

### Requirement: [U1] Footers And Handlers Share One Per-State Keymap

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

### Requirement: [U1] Footer Grammar Stays Byte-Identical

The footer SHALL keep the existing grammar: `[ ` + entries + ` ]` where each entry is `<key> <description>` and entries are separated by exactly three spaces (`[ k desc   k desc ]`). Rendering the footer from bindings MUST NOT alter the bytes of any footer produced before this change, except where a binding is newly disabled by the first requirement of this delta (the conflict `c` case). A state with no enabled help-bearing binding SHALL render no footer line, as before.

#### Scenario: Footer bytes are unchanged
- GIVEN the main menu, commit selection with commits, and run history with rows
- WHEN each view renders
- THEN each footer line matches its pre-change bytes, including the `[ ` prefix, the ` ]` suffix and the three-space separator

#### Scenario: Entries are separated by three spaces
- GIVEN a state whose footer has two or more entries
- WHEN the view renders
- THEN adjacent entries are separated by exactly three spaces and no other separator appears

### Requirement: [U1] Structural Footer Parity Over Every State

A single structural test MUST range over every state the model can render (every state handled by `viewBody` and `handleKey`, including sub-phases of states with several phases) and MUST verify, for each: every key token in the rendered footer corresponds to an enabled binding that acts, and every enabled binding appears in the footer when it has help text. A state that has no registered bindings MUST fail the test, so adding a state without a keymap is detected. This test replaces the 12-sub-phase parity probe from `tui-ux-consistency`.

#### Scenario: Every state is covered
- GIVEN the list of all model states and their sub-phases
- WHEN the parity test runs
- THEN each has a bindings entry, and a missing entry fails the test

#### Scenario: No listed key is inert and no acting key is hidden
- GIVEN each state populated so its bindings are representative
- WHEN each footer token and each enabled binding is exercised
- THEN no listed key is inert and no enabled, help-bearing binding is missing from the footer

### Requirement: [U1] Printable Bindings Stay Inactive In Text-Input States

In states that consume typed text (ticket input, package path, and the typed confirmations `CANCELAR`, `DESPLEGAR` and `BORRAR`), the keymap MUST NOT contain bindings on printable keys other than the existing guarded `q` handling. Typing any printable character MUST append it to the buffer exactly as before this change, and the footer MUST NOT advertise a printable shortcut there.

#### Scenario: Printable characters reach the buffer
- GIVEN each text-input state with a non-empty buffer
- WHEN the user types a letter that is a binding in other states (for example `c`, `d`, `x`, `p`)
- THEN the letter is appended to the buffer and no other effect occurs

#### Scenario: `q` keeps its guarded behavior
- GIVEN a text-input state
- WHEN the user presses `q` with a non-empty buffer, and separately with an empty buffer
- THEN `q` is appended in the first case and acts as `esc` in the second, never quitting

## MODIFIED Requirements

### Requirement: [U5] Semantic Color And Visual Hierarchy

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

### Requirement: [U1] Footer Lists Exactly The Keys That Act

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

## REMOVED Requirements

None.
