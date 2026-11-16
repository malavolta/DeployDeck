# Proposal: TUI Bubbles Adoption (change B, core)

**Delivery phase**: Fase 5 Productizacion. Change B of two. It follows `tui-ux-consistency` (A, archived 2026-11-05).

> **Amendment 2026-11-05**: U3 and U4 (windowing and truncation) were CUT to B2 by user decision. The shipped scope is U1, U2 and U5 (commits `d0ebf0b`, `16f577f`, `14da889`, `360d0d6`). The size is about 1,428 changed lines, approved by the user as the size exception on 2026-11-05. The original size forecast (about 1,000) and the 1,200 ceiling are superseded. Items below that describe windowing, the 1,200 ceiling or the forecast are kept as the original plan and are marked AMENDED or CUT. See `archive-report.md`.

## Intent

Each key set is written twice: once in the hardcoded footer strings (61 `footer()` calls in `view.go`) and once in the hand-rolled `switch msg.String()` handlers (`keys.go`). Change A could only pin parity with a probe over 12 sub-phases (W4), and the conflict footer still shows `c` while continue is disabled (S1). Long lists ignore the terminal height. Goal: one keymap per state is the source for both handling and footers. *AMENDED 2026-11-05: the "long lists fit the window" goal was CUT to B2 with U3 and U4.*

## Scope

### In Scope (ordered work units, each one a valid stopping point)
- **U1**: add `github.com/charmbracelet/bubbles v0.21.0` (only `key` and `help`). Add per-state `key.Binding` keymaps and a `bindings()` dispatcher. Handlers use `key.Matches`. Footers use `help.ShortHelpView` with `ShortSeparator="   "` inside our `[ ]` adapter, byte-identical to the current grammar. Keys that cannot act are disabled. A structural parity test over every state replaces the 12-sub-phase probe.
- **U2**: render the wrong-word notice in the strong delete confirm (`keys.go:177`, `deleteConfirmView` `view.go:1078`).
- ~~**U3**: store `tea.WindowSizeMsg` width and height. Add a pure `window(cursor,total,height)` helper that shows "↑/↓ N más", plus a truncate helper. A height of 0 means no windowing. Apply windowing to run history.~~ **CUT to B2 (2026-11-05).**
- ~~**U4**: windowing and row truncation for commit selection.~~ **CUT to B2 (2026-11-05).**
- **U5**: `lipgloss.AdaptiveColor` for `styleOK`, `styleWarn` and `styleErr`. Dark backgrounds keep 10/11/9. Output under the Ascii test profile is unchanged.

### Out of Scope (follow-ups)
- **B2**: windowing and truncation (cut U3/U4), the `?` overlay, a phase-aware shared spinner, elapsed and last-poll time, plus the warnings W2, W4 and suggestions S1-S4 from verify.
- **B3**: the error screen redesign, cleanup and queue windowing, status badges.
- Translating the English continue-gate reasons (`internal/git/continue_gate.go`).
- `bubbles/list` and bubbles v2. No bubbletea v2 migration.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `tui-presentation`: footers come from the enabled bindings. A disabled key is never shown. Status colors adapt to the terminal background. *AMENDED: the long-list windowing part was CUT to B2.*
- ~~`run-history`: the list is windowed.~~ *CUT to B2 (no delta shipped).*
- ~~`commit-selection`: the list is windowed and long rows are truncated.~~ *CUT to B2 (no delta shipped).*
- `branch-cleanup`: the strong-confirm wrong-word notice is rendered.

## Approach

Follow the exploration. `bindings()` mirrors `viewBody`/`handleKey`. A footer adapter wraps `ShortHelpView`. Handler switches change only their matching mechanism, not their behavior. `window()` and `truncate()` are pure helpers and are written test-first.

## Affected Areas

| Area | Impact |
|---|---|
| `go.mod`, `go.sum` | New dependency |
| `internal/app/keymap.go` | New |
| `internal/app/keys.go`, `view.go`, `update.go`, `style.go` | Modified |
| `internal/app/footer_parity_test.go` | Replaced with the structural test |

## Dependencies / Supply Chain / Boundaries

- bubbles v0.21.0 needs bubbletea >= v1.3.4 and lipgloss v1.1.0. We already have v1.3.10 and v1.1.0.
- Review the `go.sum` diff after `go mod tidy`.
- `key` and `help` import no os/exec, net/http or `internal/*`. `boundary_test.go` must stay green.

## Size Forecast and Cut Plan

- Forecast (original): about 1,000 lines, excluding openspec. Original ceiling: 1,200.
- **AMENDED 2026-11-05**: measured size is about 1,428 lines (U1+U2+U5, excluding openspec, measured against 4cc5ea6), plus a few lines from `360d0d6`. The user approved 1,428 as the size exception. The 1,200 ceiling is superseded and no longer a criterion.
- **Cut plan (applied)**: the measured size exceeded the ceiling. The user chose to CUT U3 and U4 to B2 and keep U1, U2 and U5 in this change.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| The handler rewrite overruns the budget | High | U1 first. Measure, then cut. |
| The footer grammar drifts | Med | Byte-identical footer pins plus the parity test |
| A binding swallows typed input | Med | No printable bindings in text-input states |
| AdaptiveColor changes test output | Low | TestMain keeps the Ascii profile |

## Rollback Plan

UI and dependency only. No persisted, Git or Salesforce changes. Revert the commits and `go mod tidy`.

## Success Criteria

- [x] Every state's footer is derived from its enabled bindings. The structural test covers all states.
- [x] Conflict `c` is hidden while continue is disabled.
- [x] The wrong-word notice is visible.
- ~~[ ] History and commit selection fit within a 10-row window when the height is set, and are unchanged when the height is 0.~~ *CUT to B2 (2026-11-05).*
- [x] The full test suite and `boundary_test.go` pass. *AMENDED: the size criterion is now the user-approved 1,428 lines (was 1,200).*

## Proposal question round

Auto mode, so scope was fixed by the user. For review:
- Should overflow indicators stay in Spanish ("N más")? Assumed yes.
- AdaptiveColor light-background shades: assumed darker variants (2/3/1).
