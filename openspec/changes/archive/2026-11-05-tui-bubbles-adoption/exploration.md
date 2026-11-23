# Exploration: tui-bubbles-adoption (change B)

Follows the archived change A, `tui-ux-consistency`, at `openspec/changes/archive/2026-11-05-tui-ux-consistency/`.

## Current architecture

- **Rendering.** `View()` returns `updateBanner() + viewBody()` (`view.go:20`). `viewBody` is a switch over about 30 states (`view.go:46-110`).
- **Window size.** Nothing handles `tea.WindowSizeMsg`, and `Model` has no width or height fields (`update.go:31-116`).
- **Keys.** `handleKey` (`keys.go:16-77`) routes to about 27 handlers, each a hand-rolled `switch msg.String()`.
- **Footers.** `footer(keys string)` (`view.go:164`) has 61 call sites, all hardcoded Spanish strings. The key set is therefore defined twice, once in `view.go` and once in `keys.go`. `footer_parity_test.go` works around this by parsing the rendered `[ ... ]` text, and only for 12 sub-phases (A's W4). The conflict footer shows `c` even when continue is disabled (A's S1).
- **Spinner.** The 4-rune spinner is hand-rolled (`view.go:171-179`) and guarded by `startSpinner`/`isSpinnerState` (`commands.go:912`, `update.go:895-910`). `isSpinnerState` covers only 5 states. Push, PR creation, queue load, cleanup load, the gate check and polling have no spinner.
- **Lists.** Commit selection (`view.go:368`), run history (`:1357`), deploy queue (`:663`) and branch cleanup (`:1511`) render every row, with no height awareness and no truncation.
- **Palette.** Colors are fixed ANSI 9/10/11 (`style.go:61-66`). The package TestMain forces the Ascii profile.
- **Error screen.** The Error state only prints `%v` (`view.go:107`).
- **Carry-over from A.** The wrong-word notice in the strong delete confirm is set at `keys.go:177` but never rendered by `deleteConfirmView` (`view.go:1078`).

## Dependency

- **Version.** Use `github.com/charmbracelet/bubbles v0.21.0`. It requires bubbletea v1.3.4 and lipgloss v1.1.0, both at or below the versions already in go.mod, so there is no bubbletea v2 migration. Do NOT use `charm.land/bubbles/v2`, which requires bubbletea v2.
- **Packages.** Use only `key`, `help`, and optionally `spinner` and `viewport`. Avoid `list`, which is heavy and pulls in fuzzy.
- **Boundary.** None of these packages import os/exec, net/http or internal/*. `boundary_test.go` is unaffected.
- **Supply chain.** Review the go.sum diff and run `go mod tidy`.

## Approach

- **Per-state keymaps.** Define `key.Binding` sets per state, plus a `bindings()` dispatcher that mirrors `viewBody` and `handleKey`. Use `SetEnabled(false)` for keys that cannot act in the current state.
- **Handlers.** Handlers use `key.Matches`.
- **Footer.** The footer is `help.ShortHelpView` with `ShortSeparator = "   "`, wrapped in our own `[ ]` adapter so the existing grammar stays byte-identical.
- **Structural parity test.** One test ranges over every state's bindings. This fixes W4 and S1 by construction.
- **Windowing.** A pure helper `window(cursor, total, height)` adds "↑/↓ N más" indicators and keeps the existing cursors and row formats. A height of 0, which is the default and what all existing tests use, means no windowing.

## Honest size

| # | Item | Prod | Test |
|---|---|---|---|
| 1 | Dependency + `keymap.go` + footer adapter | 230 | 40 |
| 2 | Migrate 61 footers and about 25 handlers to `key.Matches` | 160 | 60 |
| 3 | Structural parity test | -20 | 140 |
| 4 | `?` help overlay | 60 | 80 |
| 5 | WindowSizeMsg + window/truncate helpers | 90 | 130 |
| 6 | Windowing for run history and commit selection | 50 | 90 |
| 7 | Windowing for cleanup and queue | 50 | 70 |
| 8 | Phase-aware shared spinner | 70 | 110 |
| 9 | Elapsed and last-poll time during polling | 40 | 70 |
| 10 | AdaptiveColor + status badges | 40 | 70 |
| 11 | Error screen (title, cause, next step) | 80 | 90 |
| 12 | Wrong-word notice (A carry-over) | 6 | 30 |

- **Full goal:** about 2,000 lines.
- **Core B** (items 1, 2, 3, 5, 6, 12, plus AdaptiveColor only from 10): about 1,050 lines. The realistic range is 1,000-1,200.
- **Follow-up B2** (items 4, 8, 9): about 500 lines.
- **Follow-up B3** (items 11 and 7, plus status badges): about 400 lines.

## Risks

- **Handler rewrite.** Rewriting about 25 handlers to use `key.Matches` is the biggest overrun risk.
- **`?` collides with text input.** Typed-input states consume printable keys, so `?` must not be bound there.
- **Phase-aware spinner.** Making the spinner phase-aware can leak tick loops or leave the single-flight guard stuck.
- **Footer grammar.** The footer must stay byte-identical to the spec grammar.
