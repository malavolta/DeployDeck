# Design: TUI UX Polish (change B2)

## Technical Approach

All changes stay in `internal/app`, plus `go.mod`/`go.sum` (`charmbracelet/x/ansi` becomes direct). U1 makes the structural parity test strict before anything else changes, so U2-U5 land under it. Windowing reuses B's ADR-8 unchanged, including the amended rule (bottom-anchored, cursor visible, minimal scroll down). The `?` overlay is a modal Model flag that is intercepted in `handleKey` and rendered before `viewBody`. The spinner becomes phase-aware through one predicate and one transition hook in `Update`, so no entry point can be missed. Polling time is derived at render time from `Deps.Now`. Every predicate a footer uses is also the guard its handler uses.

## Architecture Decisions

| # | Unit | Choice | Rejected | Rationale |
|---|---|---|---|---|
| ADR-1 | U1 W2 | Add `m.canResume()` (`selectedRun` + `isResumable(rec, m.repoState)`) and `m.aiActionable()` (`pushPhase == pushReady && prURL == "" && aiAvailable()`) to `keymap.go`. Footers use `hint(..., pred)`. Handlers return `m, nil` early when `!pred`. History Enter keeps calling `resumeInto`. The `a` case drops its redundant pending/accepted branch | Repurposing Enter on terminal rows. A separate guard in `keys.go` | `isResumable` already mirrors `resumeInto`'s switch exactly, so the footer can no longer advertise a no-op. With one predicate per key, the footer and the guard cannot drift |
| ADR-2 | U1 W5 | Remove `pressed(..., lenient)`. Enabled bindings must `acts()`. `fixtureModel` gives typed states a valid buffer (ticket `DD-1`, valid package path, `CANCELAR`, `DESPLEGAR`, `BORRAR`). Leniency is kept only for `var blockedNotice = map[State][]string{...}`, starting empty. Apply adds an entry only for a key that is enabled by design yet notice-only (expected: `StatePackageReview: {"enter"}` for the empty package). A test fails on any allowlist entry that no fixture uses | A per-fixture allowlist. Keeping the lenient rule | An explicit, self-pruning list makes the exception visible and keeps it from growing silently |
| ADR-3 | U1 S1 | Add a trailing `stateCount` to the `State` iota (comment: must stay last). Both coverage loops become `s < stateCount`. `TestStateCount_Sentinel` asserts that `viewBody()` is non-empty for every `s < stateCount`, and empty for `stateCount` and `stateCount+1` | Looping until `viewBody()==""`. A go/ast source check | The `+1` probe catches a state appended after the sentinel with a view. It is cheap, with no AST code |
| ADR-4 | U1 W6 | New fixtures: `history terminal row`, `ready authed ai after PR` (prURL set), `cleanup empty prune` (`cleanupBranches=nil`, `cleanupPruneConfirm`), `queue failed`. A test also asserts that the test's `isTyped` equals the production `m.textInput()` over all fixtures | — | These close the W2/W6 holes. The cross-check stops the U3 predicate from becoming a tautology |
| ADR-5 | U2 | B's ADR-8 verbatim: `Model.width/height` are set by `tea.WindowSizeMsg` (returns `m, nil`). The pure helpers live in `window.go`. Each list view builds `top`, the rows and `bottom` as strings, then calls `rows := listRows(m.height, m.updateBanner()+top+bottom)`. `truncate(row, m.width)` applies only to history and selection data rows. The header, detail block and footer are never truncated | Fixed chrome constants. Per-list offset fields | Chrome is variable (detail, warnings, notice, banner). A height or width of 0 makes every helper a no-op, so output stays byte-identical |
| ADR-6 | U3 render | `help.Model{}.FullHelpView([][]key.Binding{overlayEntries(screenBindings())})` with zero styles. `overlayEntries` lists ALL bindings, as the spec requires: each one is copied with `SetEnabled(true)`. An empty help key falls back to `strings.Join(Keys(), "/")`. A disabled binding gets the suffix ` (no disponible)` on its description. Layout is `screenHeader("Atajos")` + table + `footerOf(keys.HelpClose "?/Esc cerrar")` | A custom column layout. Listing only enabled bindings | Same source and same labels as the footer. `FullHelpView` skips disabled bindings, so forcing them enabled is the only way to list "ones not shown in the footer". Zero styles keep the output deterministic |
| ADR-7 | U3 modality | `m.helpOpen bool`. In `handleKey`, after the notice is cleared: if open, `?`/Esc close it and every other key is inert. If closed and `m.helpEnabled()`, `?` opens it. `View()` returns ONLY the overlay while it is open (no banner, no notice). Async messages keep updating state underneath, and the overlay lists the live state's bindings. `bindings()` returns `[HelpClose]` while it is open | Showing the banner over the overlay. Passing keys through | The modal overlay passes parity naturally (only `?`/Esc act). The main-menu banner says "pulsa u", which would advertise an inert key while the overlay is open |
| ADR-8 | U3 enablement | `helpEnabled = !m.textInput() && !interruptOnly(screenBindings())`. "In-flight" is defined precisely as a screen whose only binding is `ctrl+c interrumpe` (discovery, branch creation, cherry-picking, cleanup loading). `textInput` covers ticket, package path, CANCELAR, DESPLEGAR, and cleanup/terminal `cleanupStrongConfirm`. There is NO AI text-input state in the code (the AI flow is `a`/`y`/`d` keys only), so the spec's "AI input" scenarios have no target. See Open Questions. `bindings()` = `screenBindings()` + `on(keys.Help, "?", "ayuda")`, appended LAST when it is enabled | Binding `?` only on list screens. Hiding `? ayuda` | One rule that is easy to state in the spec. Appending last means every affected pin changes only by the suffix `   ? ayuda` |
| ADR-9 | U4 predicate | `m.spinnerActive()` = `isSpinnerState(state)` OR push `pushPushing`/`pushPRCreating` OR `StateBranchCleanup && cleanupLoading` OR `StateQueueReview && queueLoading` (new field, set by `confirmPackageReview` and queue `r`, cleared on every `onQueueDone` path) OR `StateQuickDeploy && (gateCheckingRunID != "" \|\| quickDeployingRunID != "")` OR `StateValidationPolling`. `onSpinnerTick` uses it in place of `isSpinnerState`. Spinner glyph lines are added to the push/PR, cleanup-loading, queue-loading and quick-deploy in-flight views. Polling is refresh-only: it keeps the tick alive but renders the U5 clock, not a glyph | Per-site `startSpinner` calls only. A separate 1 s clock loop for polling | The polling tick refreshes the U5 clock every 120 ms with no second loop and no new poll requests. Quick-deploy execution is the same in-flight screen as the gate check |
| ADR-10 | U4 seeding | `Update` becomes a wrapper: `was := m.spinnerActive(); nm, cmd := m.update(msg)`. If `!was && nm.spinnerActive() && !nm.spinning`, set `spinning = true` and return `tea.Batch(cmd, spinnerCmd())`. The existing `startSpinner` calls stay, and the hook sees `spinning == true` | Adding `startSpinner` at about 10 new sites. A hook that seeds whenever it is active | Seeding on the transition alone means it cannot miss an entry. A fixture already in a spinner phase never gets a seed cmd on an unrelated key, which keeps parity sound. Termination is unchanged: the tick clears `spinning` once the predicate is false, and the single-flight guard holds |
| ADR-11 | U5 | `m.pollStart` is set where `pollDeadline` is armed (`onValidateDone`, `resumeInto` jobId branch). `m.lastReportAt = m.now()` in `onReportDone` on the SUCCESS path only (after the `msg.err` check), as the spec requires ("last successful report"). Both fields are reset when polling is armed. The view adds `"  Transcurrido: %s · Último sondeo: %s\n"` after `validationBody`. `clock(d)` gives `%02d:%02d` (total minutes, so it can go past 59). The last poll shows `hace Ns`, or `pendiente` when it is zero. Values come from `m.now()` at render time (precedent: `historyBindings`) | A persisted job start time. A separate 1 s clock loop | Elapsed shares the deadline's origin. On resume it counts from re-attach (documented limitation; no persistence change) |
| ADR-12 | U6 | `isQuit(cmd)` compares `reflect.ValueOf(cmd).Pointer()` with `tea.Quit` and with the `quitCmd` closure code pointer (taken once from a Model with Git set). It NEVER runs `cmd`. A shared `isTick(cmd)` (the `tea.Tick` closure pointer) is introduced in U4 and reused here. Test helpers that flatten `tea.BatchMsg` skip ticks | Running the cmd with a timeout | Running a 10 s `pollTickCmd` is the slowdown. Pointer identity is instant and deterministic. Closure pointers identify code, which is what we need here |
| ADR-13 | U7 (optional) | Extract `m.canQuickDeploy() (bool, string)` and `m.canRePromote() bool` (selection + eligibility), plus `m.rowsPresent()`/`m.deltaFailed()`, used by `historyBindings`/`bindings` and by the `keys.go` guards. History x/r keep their blocked-notice branches | A full predicate table | Only worth doing if the budget allows. Otherwise W7 is recorded as a known gap |

## Data Flow

    tea.WindowSizeMsg ─→ m.width, m.height
    tea.KeyMsg ─→ Update(wrapper) ─→ update ─→ handleKey
                    │                   ├─ helpOpen? ?/Esc close, else inert
                    │                   ├─ helpEnabled && "?" → helpOpen
                    │                   └─ key<State> (guards = footer predicates)
                    └─ !was && spinnerActive && !spinning → Batch(cmd, spinnerCmd)
    View ─→ helpOpen ? overlay(FullHelpView(screenBindings)) : banner + viewBody
            list views: top/bottom → listRows → window → overflow + truncate(row)
            polling: clock(now-pollStart), hace(now-lastReportAt)

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/app/window.go` | Create | `window`, `overflow`, `truncate`, `listRows`, `lines` |
| `internal/app/app.go` | Modify | `stateCount`. Fields `width`, `height`, `helpOpen`, `queueLoading`, `pollStart`, `lastReportAt` |
| `internal/app/keymap.go` | Modify | `Help`/`HelpClose` bindings, `screenBindings`/`bindings` split, `canResume`, `aiActionable`, `textInput`, `helpEnabled`, (U7 predicates) |
| `internal/app/keys.go` | Modify | Help intercept, W2 guards, `queueLoading` set |
| `internal/app/update.go` | Modify | `Update` wrapper + hook, `WindowSizeMsg`, `spinnerActive`, `pollStart`/`lastReportAt`, `queueLoading` clear |
| `internal/app/view.go` | Modify | Overlay, windowed history/selection, spinner lines in the new phases, polling clock |
| `internal/app/footer_parity_test.go`, `subphase_test.go` | Modify | Strict parity, allowlist, sentinel, fixtures |
| `internal/app/footer_pins_test.go` | Modify | `   ? ayuda` suffix pins, W2 history pins, overlay pin |
| `internal/app/menu_cleanup_test.go` | Modify | Pointer-based `isQuit`, `isTick` |
| `go.mod`, `go.sum` | Modify | `x/ansi` direct |

## Interfaces / Contracts

```go
func window(cursor, total, rows int) (start, end int) // rows<=0||total<=rows → (0,total); else start=clamp(cursor-rows+1,0,total-rows)
func overflow(n int, up bool) string                  // "" if n<=0; "  ↑ N más\n" / "  ↓ N más\n"
func truncate(s string, width int) string             // width<=0 → s; else ansi.Truncate(s, width, "…")
func listRows(height int, chrome string) int          // 0 if height<=0; else max(3, height-lines(chrome)-2)
func clock(d time.Duration) string                    // "mm:ss"
func (m Model) spinnerActive() bool
func (m Model) canResume() bool
func (m Model) aiActionable() bool
func (m Model) textInput() bool
func (m Model) helpEnabled() bool
```

## Testing Strategy (strict TDD, RED first, per unit)

| Unit | RED tests |
|---|---|
| U1 | Strict parity fails first on the new W6 fixtures and the empty allowlist. `TestStateCount_Sentinel`. History terminal row: Enter is not in the footer and does not act. Ready after PR with AI: `a` is inert. Unused-allowlist check. `isTyped` == `textInput` cross-check |
| U2 | Table tests for `window` (rows 0, total ≤ rows, cursor at top, middle and bottom, clamping) and `truncate` (ANSI `mark()` row, runes, width 0). History and selection: height 0 is byte-equal to the pre-change output. With height H and 50 rows: at most H lines, cursor row present, correct `↑`/`↓ N más` counts. Moving down scrolls by 1 |
| U3 | `?` opens, `?`/Esc close, other keys inert. `?` types into every text-input buffer. Overlay lists every enabled screen binding. Parity fixture `help open`. Pins updated deliberately |
| U4 | Per-phase entry (`p` push, PR create, `b` cleanup, queue load/`r`, gate check, resume into polling) returns `spinning==true`. A tick after the phase ends returns `nil` and `spinning==false`. A key on a fixture already spinning gets no seed. A second entry while spinning does not double-seed |
| U5 | Fake `Deps.Now`: 65 s after validate shows `01:05`. 61 min 5 s shows `61:05`. Before the first report shows `pendiente`. 7 s after a report shows `hace 7s`. An errored report leaves the age unchanged. Terminal states show no live lines. Resume restarts elapsed |
| U6 | `isQuit(tea.Quit)` and the quitCmd closure give true. A blocking cmd gives false without being called (the test cmd calls `t.Fatal`). Package wall time drops (measured in verify) |
| U7 | Existing parity covers it. Add a guard/footer agreement test per extracted predicate |

The existing resume, esc/q matrix and `boundary_test.go` tests stay green. Tests that assert the exact cmd on entry into a new spinner phase are adapted with the batch-flattening helper.

## Threat Matrix

N/A. There is no routing, shell, subprocess, VCS/PR automation, executable-file classification or process-integration boundary. `x/ansi` is already in the module graph and only becomes direct.

## Migration / Rollout

No migration required. One commit per unit, in order U1 → U2 → U3 → U4 → U5 → U6 → U7. Measure after each unit against `e8d783a`, excluding openspec.

## Size Forecast

| Unit | Raw (prod+test) | Adjusted | Cumulative |
|---|---|---|---|
| U1 | 130 (20+110) | 195 (×1.5) | 195 |
| U2 | 240 (95+145) | 360 (×1.5) | 555 |
| U3 | 235 (60+175, ~100 is mechanical pin churn) | 305 (×1.3) | 860 |
| U4 | 160 (45+115, incl. adapting existing tests) | 240 (×1.5) | 1,100 |
| U5 | 70 | 105 | 1,205 |
| U6 | 30 | 45 | 1,250 |
| U7 | 50 | 75 | 1,325 |

The projection lands U1-U6 at about 1,250, under 1,300. U7 is the expected cut. If U1-U4 measure above 1,100, cut U6 too. If U5 would push past 1,300 after both cuts, apply STOPS and asks the user. It is never cut silently.

## Open Questions

- [x] On resume, elapsed restarts from re-attach. The spec settles this ("restarts elapsed from the time of re-attach").
- [ ] **Spec/code mismatch**: `tui-presentation` names an "AI input" text-input state. None exists, because AI is key-driven. Tasks should drop the AI-input scenarios, or the spec should be amended to remove them. Apply MUST NOT invent a state.
- [ ] `? ayuda` also appears on y/n prompts and on `q salir`-only screens such as pushing, PR creating and validation start (ADR-8 rule: only `ctrl+c` screens are in-flight). The spec's pin list must match this set exactly.
- [ ] Carried over: the overflow copy `N más`, `mm:ss`/`hace Ns`, and the title `Atajos` (Spanish assumed).
