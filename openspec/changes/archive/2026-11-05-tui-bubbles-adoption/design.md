# Design: TUI Bubbles Adoption (change B, core)

> **Amendment 2026-11-05**: U3 and U4 (windowing, truncation, `window.go`, `WindowSizeMsg` handling, `x/ansi` as a direct dependency) were CUT to B2 by user decision. Sections that describe them are kept as the original plan and marked CUT or AMENDED. Shipped: U1, U2, U5. Size is about 1,428 lines (user-approved; the 1,200 ceiling is superseded). See `archive-report.md`.

## Technical Approach

All changes stay in `internal/app`, plus `go.mod`/`go.sum`. Key strings are defined once, in a `keys` vocabulary of `key.Binding` values (`keymap.go`). Handlers match against that vocabulary with `key.Matches`. Each state's footer is the ordered `[]key.Binding` that `bindings()` returns, with per-state labels, and with enablement computed by the same Model predicates the handlers use as guards. `footerOf(bs...)` renders the bindings through `help.ShortHelpView` inside the existing `[ ]` wrapper. A structural test derives its probe universe from the keymap, so it covers every state and every sub-phase. Windowing and truncation are pure helpers in `window.go`. Handler behavior does not change. Only the matching mechanism changes. *AMENDED: windowing and truncation helpers were CUT to B2.*

## Architecture Decisions

| # | Decision | Choice | Rejected | Rationale |
|---|---|---|---|---|
| ADR-1 | Keymap shape | A single `keyMap` struct holds the named vocabulary (`Nav`, `Up`, `Down`, `Enter`, `Esc`, `Quit`, `Exit` = enter/q/esc, `Continue`, `Skip`, `Abort`, `Edit`, `Retry`, `Refresh`, `Delete`, `Push`, `GH`, `AI`, `Yes`, `No`, `Override`, `Detail`, `Quick`, `RePromote`, `Mark`, `Update`, `Prune`, `Interrupt`, ...) as package var `keys`. Each state has a `(m Model) <state>Bindings() []key.Binding`. `bindings()` switches on `m.state` and mirrors `viewBody`. Each sub-phase function switches on `cleanupPhase`, `pushPhase`, AI wiring and branch presence, mirroring `deleteConfirmView`/`viewPushPreparation`/`aiFooterPart` | Per-state structs implementing `help.KeyMap` | About 25 struct types would cost about 300 lines for no extra safety. `FullHelp` has no use until B2's `?` overlay. A slice keeps the footer order explicit. |
| ADR-2 | Labels and enablement | `hint(b, key, desc string, on bool) key.Binding` copies `b` (value receiver), then calls `SetHelp` and `SetEnabled`. Enablement uses shared predicates: `m.continueEnabled`, `m.hasBranch()`, `m.aiAvailable()` (replaces `aiFooterPart`), `quickDeployOK(rec)`, `isRePromoteEligible`, and `len(list)>0` | Duplicating guard conditions in the footers | A predicate that is used by both the guard and the footer cannot drift. S1 is fixed because `Continue` is `hint(..., m.continueEnabled)`. |
| ADR-3 | Disabled keys in handlers | Most cases use `key.Matches(msg, keys.X)`. Keys whose disabled press shows an explanatory notice (conflict `c`, Succeeded/Aborted `p`/`d` with no branch, history `x`/`r`) keep their existing guard branch, so they still match while hidden | Matching against the state's footer copy, which would hide those notices because `key.Matches` returns false for disabled bindings | Under ADR-2 of change A, a notice-only press does not act, so hiding these keys is consistent with the parity test. |
| ADR-4 | Footer adapter | `var helpView = help.Model{ShortSeparator: "   ", Styles: help.Styles{}}` and `footerOf(bs ...key.Binding) string`. It returns `""` when `ShortHelpView` is empty (the spec's "no footer line" rule) and otherwise `footer(helpView.ShortHelpView(bs))`. `Width` stays 0, so nothing is truncated | `help.New()` default styles | The default styles color footers on a real TTY (a visual change that belongs to B2). Zero styles give byte-identical output on every profile. `ShortHelpView` skips disabled bindings and inserts the separator only between rendered items. |
| ADR-5 | Literal footers | None remain. All 61 call sites become `footerOf(...)`, including in-flight `ctrl+c interrumpe` (`keys.Interrupt`, display-only, handled by `Update`) and the Error/Failed/Canceled `Exit`. `footer(string)` stays a private wrapper that only `footerOf` calls. A source test asserts that `view.go` contains no `footer("` | Keeping in-flight screens as literals | Every state is derived from bindings, which is a proposal success criterion. |
| ADR-6 | Text-input states | Ticket, PackagePath, CANCELAR, DESPLEGAR and both BORRAR states bind only `Enter`/`Esc` (and `Backspace`, which is not shown). The handler's `default:` branch appends `msg.Runes` when `msg.Type == tea.KeyRunes`. A guarded `q` with an empty buffer goes back, as before, and is not a binding | Binding `q` or `?` in typed states | A printable binding would swallow typed input. The guarded `q` stays unadvertised, as in change A. |
| ADR-7 | What "acts" means | Reuse change A's ADR-2: a key acts when the state, `pushPhase` or `cleanupPhase` changes, when the returned cmd is non-nil, or when `View()` changes once `notice` is cleared on both models. This is the existing `acts()` helper | `reflect.DeepEqual` | The Model holds funcs and contexts. |
| ADR-8 (CUT to B2) | Window state | `Model.width` and `Model.height` are set by `tea.WindowSizeMsg` in `Update` (returns `m, nil`). `window(cursor, total, rows int) (start, end int)`: when `rows <= 0` or `total <= rows`, it returns `(0, total)`. Otherwise it anchors to the bottom, with `start = clamp(cursor-rows+1, 0, total-rows)`. This is pure and needs no offset field. It is minimal scroll moving down. Moving up, the window follows the cursor (see Open Questions). `overflow(n int, up bool) string` returns `"  ↑ N más\n"` or `"  ↓ N más\n"` when n > 0. `truncate(s string, width int) string` returns `s` when width <= 0, otherwise `ansi.Truncate(s, width, "…")` | Fixed chrome constants per screen | The detail block, warnings, notices and banner make chrome variable, so constants drift. Each list view renders its top and bottom first, then sets `rows = height - lines(banner+top+bottom) - 2`, with a minimum of 3. `charmbracelet/x/ansi` is already in the module graph and is ANSI-aware, which matters because rows contain `mark()` escapes. |
| ADR-9 | Adaptive colors | OK `{Light:"2", Dark:"10"}`, Warn `{Light:"3", Dark:"11"}`, Err `{Light:"1", Dark:"9"}`. `TestMain` also calls `lipgloss.SetHasDarkBackground(true)` | Fixed bright colors | Light terminals get the base palette. Pinning a dark background keeps `TestMark_NonAsciiUsesBrightColors` deterministic and avoids an OSC 11 query in tests. A new test covers the light shades. |
| ADR-10 | U2 notice | In the `cleanupStrongConfirm` branch of `deleteConfirmView`: prompt, then `"  " + buffer + "_\n"`, then `m.noticeLine()`, then the footer | Rendering the notice in `abortedTail` or `viewValidationResult` | Both callers return early on `deleteConfirmView`. That early return is the bug. |

## Byte-Identical Footer Pins (must not change)

These are all current strings, rendered as `[ <text> ]`:

- `ctrl+c interrumpe`
- `Enter/q/Esc salir`
- `d borrar rama   Enter/q/Esc salir`
- `p preparar push   d borrar rama   Enter/q/Esc salir`
- `r reintentar   c continuar con warnings   q/Esc salir`
- `Enter seleccionar   ↑/↓ navegar   [u actualizar   ]q/Esc salir`
- `Enter seleccionar   ↑/↓ navegar   Esc volver   q salir`
- `Enter continuar   q/Esc volver`
- `Enter buscar   Esc volver`
- `s confirmar   n/N/Enter declinar   Esc volver   q salir`
- `Esc volver   q salir`
- `↑/↓ navegar   Space/x marcar   Enter continuar   Esc volver   q salir`
- `Enter ejecutar   Esc volver   q salir`
- Conflict: `↑/↓ elegir   e editor   c continuar   s saltar   a abortar   q salir (el run queda reanudable)`. The only new behavior is that `c` is dropped while continue is disabled.
- `r reintentar   e editar selección   q salir`
- `q salir`
- `[o override   ]Enter validar   e editar selección   q salir`
- `r reintentar|refrescar   Enter continuar validación   Esc volver   q salir`
- `r reintentar   q salir`
- `r refrescar   c cancelar mi job   q salir (el job sigue activo)`
- `Enter confirmar   Esc volver`
- `q/Esc volver`
- `Enter confirmar   q/Esc volver`
- `Esc volver al historial   q salir`
- `p ejecutar push   Esc volver   q salir`
- `n/Esc/q volver`
- `y con IA   d título default   n/Esc/q cancelar`
- `y confirmar crear PR   n/Esc/q cancelar`
- `g crear PR con gh[   a sugerir título|aceptar sugerencia]   q salir`
- `y confirmar   n/Esc/q cancelar`
- `Enter confirmar   Esc cancelar`
- `Esc cancelar`
- `p retención de runs   Esc volver   q salir`
- `d borrar   p retención de runs   ↑/↓ navegar   Esc volver   q salir`
- History: `Enter reanudar   d detalle[   x quick deploy][   r re-promocionar]   ↑/↓ navegar   Esc volver   q salir`
- `r reusar   d borrar y recrear   c/Esc cancelar   q salir`
- `Enter continuar igualmente   e editar selección   q salir`

## Data Flow

    tea.KeyMsg ─→ Update ─→ handleKey ─→ key<State>: key.Matches(msg, keys.X) ─→ state/cmd
    tea.WindowSizeMsg ─→ m.width, m.height
    View ─→ view<State> ─→ footerOf(m.<state>Bindings()...) ─→ "[ " + ShortHelpView + " ]"
                       └→ window(cursor,total,rows) + overflow + truncate(row, m.width)

## File Changes

| File | Action | Description |
|---|---|---|
| `go.mod`, `go.sum` | Modify | Adds `bubbles v0.21.0`. *AMENDED: `x/ansi` stays indirect. Only the CUT U3 truncate helper needed it as a direct dependency.* |
| `internal/app/keymap.go` | Create | `keyMap` vocabulary, `hint`, `bindings()`, per-state binding funcs, `footerOf` |
| ~~`internal/app/window.go`~~ | ~~Create~~ | ~~`window`, `overflow`, `truncate`~~ *CUT to B2* |
| `internal/app/keys.go` | Modify | `switch msg.String()` becomes `switch { case key.Matches(...) }` |
| `internal/app/view.go` | Modify | 61 footers use `footerOf`. U2 notice. `aiFooterPart` and `runHistoryFooter` are removed. *History and selection windowing: CUT to B2.* |
| ~~`internal/app/update.go`, `app.go`~~ | ~~Modify~~ | ~~`WindowSizeMsg` handling. `width` and `height` fields~~ *CUT to B2* |
| `internal/app/style.go` | Modify | AdaptiveColor |
| `internal/app/footer_parity_test.go` | Replace | Structural test (`PinnedFragments` is kept) |
| `internal/app/subphase_test.go` | Modify | `TestFooterParity_SubPhaseKeys` is removed. Its fixtures move into the structural test |
| `internal/app/color_test.go` | Modify | `SetHasDarkBackground(true)` |

## Testing Strategy (strict TDD, RED first)

| Seam | Test |
|---|---|
| Structural parity | Coverage: loop `State(0)…` until `viewBody()` returns `""`. Every state must have at least one fixture and a non-empty `bindings()`, otherwise the test fails. Fixtures are `escQFixtures` + `subPhaseFixtures`, plus conflict with continue on and off, history with and without eligible rows, and lists empty and non-empty. Universe U = the union of `Keys()` over `bindings()` for every fixture. For each fixture: (1) the last footer equals `footer(helpView.ShortHelpView(m.bindings()))`; (2) every enabled binding has at least one key that acts; (3) no key of a disabled binding acts; (4) every key in U that acts belongs to an enabled binding (rune keys are exempt in typed states) |
| Byte pins | A table of the strings above, checked against fixture `View()` |
| No literals | `view.go` source contains no `footer("` |
| ~~window/truncate~~ | ~~Table: rows 0, total ≤ rows, cursor at the top, middle and bottom, and clamping. Truncate is rune- and ANSI-aware. width 0 is a no-op~~ *CUT to B2* |
| ~~History/selection~~ | ~~With height 0, the output equals the current output. With height set and 50 rows, `View()` has at most `height` lines, the cursor row is visible, and the `↑`/`↓ N más` counts are correct~~ *CUT to B2* |
| U2 / U5 | Wrong word + Enter shows the notice. Light background gives SGR 32/33/31 and dark gives 92/93/91 |

The existing resume, esc/q matrix and `boundary_test.go` tests stay green without modification.

## Threat Matrix

N/A. There is no routing, shell, subprocess, VCS/PR automation, executable-file classification or process-integration boundary. The only new boundary is the third-party dependency, which is covered under Rollout.

## Migration / Rollout

No migration required. **Network blocker (RESOLVED 2026-11-05):** the earlier blocker (`github.com` DNS failure, bubbles v0.21.0 missing from the module cache) is resolved. bubbles v0.21.0 was resolved from the module cache with `GOPROXY=off GOFLAGS=-mod=mod go get github.com/charmbracelet/bubbles@v0.21.0`, followed by `go mod tidy`. go.mod gained one line and go.sum two lines. The non-test imports are only `key` and `help`. The fallback path (U3 → U4 before U1) was not used. **Apply order (as executed):** U2 → U5 → U1. U3 and U4 are CUT to B2.

## Size Forecast (honest)

*AMENDED 2026-11-05: actual is about 1,428 lines for U1+U2+U5 (adds plus deletes, excluding openspec), plus a few lines from `360d0d6`. U3 and U4 were CUT, so the original 1,320 forecast (and its 1,200 ceiling) is superseded. The user approved 1,428. The table below is the original forecast.*

Change A forecast 790 and landed at 1,505, with tests about 2× production. Changed lines here are counted as adds plus deletes:

| Unit | Prod | Test | Total |
|---|---|---|---|
| U1 | keymap and bindings 270, handler matching about 300, footer sites about 120 | structural 180, removed tests about 80 | **≈950** |
| U2 | 5 | 25 | 30 |
| U3 | 70 | 140 | 210 |
| U4 | 35 | 60 | 95 |
| U5 | 8 | 30 | 38 |

The total is about 1,320, which is over the 1,200 ceiling. **Expected cut:** after U1, U2 and U5 (about 1,020), U4 goes to B2. U3 lands only if the measured total plus 210 stays at or under 1,200. Otherwise both move to B2. Asking the user for another exception is the alternative.

## Open Questions

- [x] **Spec conflict (tui-presentation, Lists Are Windowed) — CUT to B2 2026-11-05 (windowing not shipped). The recommendation below stays on record for B2:** "Moving the cursor MUST scroll the window the minimum amount" cannot be met by a pure 3-argument `window()` when moving up. That would need a per-list top-offset field, updated by the handlers from a row count that only the view knows. Recommendation: amend the spec to "keep the cursor visible; scrolling down is minimal" and keep the pure bottom-anchored helper. The alternative is `runsTop`/`itemsTop` fields plus a stored `lastRows`, at about 40 more lines.

- [ ] The overflow copy `↑ N más`/`↓ N más` (Spanish assumed).
- [ ] The light shades 2/3/1. Base yellow (3) is hard to read on white, and `"136"` is an alternative.
- [ ] Below `rows` minimum 3, a tiny terminal still overflows. This is accepted.
