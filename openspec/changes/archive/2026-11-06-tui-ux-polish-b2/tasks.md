# Tasks: TUI UX Polish (change B2)

Test command: `go test ./...` (Go lives at `/usr/local/go/bin`, not on PATH; use `export PATH=$PATH:/usr/local/go/bin`). Strict TDD: each behavior task is a RED test written and seen failing, then its GREEN implementation. Table tests, `t.TempDir()`, fakes and stubs per the go-testing skill. No test touches the network, `brew`, or a real Salesforce org. Commits follow the work-unit-commits skill: one reviewable unit per commit, tests and implementation together, conventional commit messages, no AI attribution. Work in the main checkout on `feat/tui-self-update` (single PR, PR #4). Existing resume, esc/q matrix, `quitCmd`, ctrl+c and `boundary_test.go` tests stay green and unmodified, except the edits the design lists (`footer_parity_test.go`, `subphase_test.go`, `footer_pins_test.go`, `menu_cleanup_test.go`) and the U4 entry-command adaptations (task 5.1).

Notation: spec scenarios are referenced as `[capability: scenario]`. Reuse the existing builders (`richModel`, `fixtureModel`, `cleanupListModel`, `st`), `acts()` and `press()`. Everything is under the Ascii profile with a fake `Deps.Now`.

## Spec amendment (recorded by this phase, 2026-11-06)

`tui-presentation` requirement `[U3] Footers Advertise ? ayuda...` listed the pin changes categorically, which left q-only screens and y/n prompts ambiguous. Design ADR-8 is authoritative: `?` is enabled in every state except text-input states and in-flight states, where in-flight means a screen whose only binding is `ctrl+c interrumpe`. The spec delta was amended to say this and to carry the exact pin list below. The "AI input" state had already been removed from the delta.

Exact footers from `footer_pins_test.go` (`TestFooterPins_BytesAreUnchanged`) that gain the trailing `   ? ayuda` (50 cases): error, failed, succeeded no branch, succeeded, aborted, prereq, main menu, main menu update, delta source, sandbox, source confirm, selection empty, selection, target, plan preview, conflict continue on, verification, delta failed, delta running, package review, package review empty, package review delta, queue, queue failed, validation start failed, validation start, polling, history empty, history, history re-promote, gate blocked, collision, push confirm, pushing, ready authed, ready authed ai, ready authed accept, ready compare ai, ready compare, pr confirm, pr confirm ai ready, pr confirm ai pending, pr creating, pr created, cleanup empty, cleanup list, cleanup counting, cleanup confirm, cleanup prune, delete confirm.

Exact footers that stay byte-identical (10 cases): `in-flight` and `discovery` and `cleanup loading` (`ctrl+c interrumpe`, in-flight); `ticket` (`Enter buscar   Esc volver`), `package path` (`Enter continuar   q/Esc volver`), `cancel confirm` (`Enter confirmar   Esc volver`), `quick deploy` (`Enter confirmar   q/Esc volver`), `quick deploy none` (`q/Esc volver`), `cleanup strong` and `delete strong` (`Enter confirmar   Esc cancelar`), all text-input.

Example: `main menu` becomes `Enter seleccionar   ↑/↓ navegar   q/Esc salir   ? ayuda`; `polling` becomes `r refrescar   c cancelar mi job   q salir (el job sigue activo)   ? ayuda`.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | About 1,325 raw forecast with multipliers (adds plus deletes, excluding openspec, against e8d783a): U1 195, U2 360, U3 305, U4 240, U5 105, U6 45, U7 75 |
| Applied budget (session) | 1,300 lines (size exception approved up front, excludes openspec) |
| 400-line budget risk | High (exceeds 400, user-approved size exception) |
| Chained PRs recommended | No (single PR #4 at the user's request) |
| Suggested split | Single PR, 7 commits (one per work unit) |
| Delivery strategy | single-pr |
| Chain strategy | n/a |

Decision needed before apply: No.
Chained PRs recommended: No
400-line budget risk: High

Measure with: `git diff --numstat e8d783a -- . ':!openspec'` (sum adds plus deletes) plus `wc -l` of untracked non-openspec files (`git ls-files --others --exclude-standard -- . ':!openspec' | xargs wc -l`).

Cut plan (applied by the size-check tasks): cut U7 first, then U6. U5 is user-visible and is NEVER cut silently. If the projection after U4 or after U5 passes 1,300 (running measured total plus the forecast of the remaining units, U5 105 included), apply STOPS and reports to the orchestrator; it does not continue and does not cut U5. Cutting a unit means marking its tasks `[~] CUT`, recording the delta in the verify report and `state.yaml`. U6 and U7 have no spec requirements, so a cut changes no spec.

### Suggested Work Units

| Unit | Goal | Commit | Focused test command | Rollback boundary |
|------|------|--------|----------------------|-------------------|
| U1 | Strict parity, sentinel, W2 guards, ticket `b` | `test(app): make footer parity strict and guard terminal-row keys` | `go test ./internal/app/ -run 'Parity\|Sentinel\|Pin\|Resume\|AIAction\|TicketB'` | Revert commit |
| U2 | Window size, windowing, truncation | `feat(app): window and truncate the history and commit lists` | `go test ./internal/app/ -run 'Window\|Truncate\|Overflow\|History\|Selection'` | Revert commit, go.mod |
| U3 | `?` help overlay | `feat(app): add a help overlay on ?` | `go test ./internal/app/ -run 'Help\|Overlay\|Pin\|Parity'` | Revert commit |
| U4 | Phase-aware spinner | `feat(app): animate the spinner in push, PR, queue, cleanup and gate phases` | `go test ./internal/app/ -run 'Spinner\|Tick'` | Revert commit |
| U5 | Polling elapsed and last-poll age | `feat(app): show elapsed time and last poll age while polling` | `go test ./internal/app/ -run 'Polling\|Clock\|Elapsed'` | Revert commit |
| U6 (cut candidate 2) | Fast quit detector | `test(app): detect quit commands by pointer instead of running them` | `go test ./internal/app/` | Revert commit |
| U7 (cut candidate 1) | Shared enablement predicates | `refactor(app): share quick-deploy and re-promote predicates between footer and handlers` | `go test ./internal/app/ -run 'Parity\|Predicate\|History'` | Revert commit |

Order is strictly sequential (single writer): U1, size check, U2, size check, U3, size check, U4, size check (STOP gate), U5, size check (STOP gate), U6, size check, U7, size check, final verification.

## Phase 1: Parity hardening (U1)

- [x] 1.1 RED (`internal/app/footer_parity_test.go`): add a trailing `stateCount` to the `State` iota in `app.go` (comment: must stay last) only as a compile stub, and add `TestStateCount_Sentinel`: `viewBody()` is non-empty for every `s < stateCount` and empty for `stateCount` and `stateCount+1`. Both coverage loops range `s < stateCount`. A state with no fixture fails naming the state. Satisfies [tui-presentation: New state without a fixture fails], [Sentinel disagrees with state count].
- [x] 1.2 RED (`footer_parity_test.go`, `subphase_test.go`): add the four named fixtures to the fixture list: `history terminal row`, `ready authed ai after PR` (prURL set), `cleanup empty prune` (`cleanupBranches=nil`, `cleanupPruneConfirm`), `queue failed`. A test asserts the fixture list names all four. Add a cross-check that the test's `isTyped` equals the production `m.textInput()` over all fixtures (the production method arrives in 3.x; until then the check is written against a local stand-in and switched in U3, or the assertion is added in task 3.7). Satisfies [tui-presentation: The four named sub-phases are fixtures].
- [x] 1.3 RED: remove `pressed(..., lenient)` so every enabled binding must `acts()`. `fixtureModel` gives typed states valid buffers (ticket `DD-1`, valid package path, `CANCELAR`, `DESPLEGAR`, `BORRAR`). Add `var blockedNotice = map[State][]string{}` (empty to start) and a test that fails on any allowlist entry no fixture uses. Run it; record every failing (state, key) pair. Satisfies [tui-presentation: A key that only sets a notice fails unless listed], [Text fixtures use valid buffers].
- [x] 1.4 RED: for each (state, key) failure from 1.3 that is a real design-enabled notice-only key (expected: `StatePackageReview: {"enter"}` for the empty package), add it to `blockedNotice` and assert the Spanish notice renders. Satisfies [tui-presentation: Listed blocked-notice key passes with a notice].
- [x] 1.5 RED (`internal/app/history_resume_guard_test.go`): history with a resumable row followed by a terminal row: on the terminal row the footer has no `Enter reanudar`, `Enter` leaves the state unchanged, returns a nil cmd and the detail view remains; moving the cursor back and forth makes `Enter reanudar` disappear and return. Satisfies [run-history: Terminal row neither advertises nor acts on Enter], [Footer follows the cursor].
- [x] 1.6 RED: Enter on a conflicted resumable run still routes to the conflict screen, and Enter on a run with a non-terminal `jobId` re-attaches polling. Existing resume tests cover it; run them unmodified as the pin. Satisfies [run-history: Enter on a conflicted run resumes to the conflict screen], [Enter on a run with a jobId resumes to polling].
- [x] 1.7 RED (`internal/app/ai_action_guard_test.go`): push-ready with AI and no PR: footer has `a` and `a` starts generation; with `prURL` set: footer has no `a` (shows only `q salir`) and `a` returns nil cmd with the model unchanged; push-ready without AI: `a` does nothing. Satisfies [ai-pr-summary: `a` is offered before the PR exists], [`a` is inert after the PR exists], [Without AI `a` stays inert regardless of the PR].
- [x] 1.8 RED (`internal/app/ticket_b_test.go`): ticket state with an empty buffer, `b` opens branch cleanup and leaves the buffer unchanged; with a non-empty buffer `b` is appended with no other effect; the footer for both equals `Enter buscar   Esc volver` with no `b` entry. Mostly passes today; keep as pin. Satisfies [tui-presentation: `b` on an empty ticket buffer opens cleanup], [`b` on a non-empty buffer is typed], [`b` is not advertised], [Ticket `b` is the only other guarded key].
- [x] 1.9 RED: parity over every fixture asserts the printable-state rule still holds with `c`, `d`, `x`, `p`: appended to the buffer, no other effect (existing 3.8/3.9 tests from B stay green). Satisfies [tui-presentation: Printable characters reach the buffer] (letters part), [`q` keeps its guarded behavior].
- [x] 1.10 RED: structural parity requirements stay satisfied over the extended fixture set: each fixture has a non-empty `bindings()`, every footer token acts, every enabled help-bearing binding is listed. Satisfies [tui-presentation: Every state is covered], [No listed key is inert and no acting key is hidden].
- [x] 1.11 GREEN (`keymap.go`, `keys.go`, `app.go`): add `m.canResume()` (`selectedRun` + `isResumable(rec, m.repoState)`) and `m.aiActionable()` (`pushPhase == pushReady && prURL == "" && aiAvailable()`). History Enter hint and `a` hint use these predicates; the handlers return `m, nil` early when the predicate is false; the `a` case drops its redundant pending/accepted branch. Keep `stateCount` last. Populate `blockedNotice` per 1.4. Run 1.1-1.10 green and the whole suite.
- [x] 1.12 Commit U1 (`f481f3a`).

## Phase 2: Size check after U1

- [x] 2.1 Size check: measure per the Review Workload Forecast. Expected about 195. Record the running total in `state.yaml`. Projection = measured + 360 + 305 + 240 + 105 + 45 + 75. If it passes 1,300: cut U7, then U6 (mark Phase 11 then Phase 10 `[~] CUT`). If it still passes 1,300 after both cuts, STOP and report (U5 is not cut).

## Phase 3: Windowing and truncation (U2)

- [x] 3.1 RED (`internal/app/window_test.go`): table for `window(cursor,total,rows)`: rows 0, total <= rows (whole range), cursor at top, middle, bottom (total 50, rows 10, cursor 49 gives 40..49), clamping of an out-of-range cursor; length <= rows and within `[0,total)`; scrolling down from window 0..9 with the cursor moving 9 to 10 gives 1..10. Satisfies [tui-presentation: Short list is not windowed], [Cursor at the bottom stays visible], [Scrolling down moves one row].
- [x] 3.2 RED: `overflow(n, up)` returns `"  ↑ N más\n"` / `"  ↓ N más\n"` and `""` for n <= 0; a view showing rows 20..29 of 50 renders `↑ 20 más` and `↓ 20 más`, and no indicator on a zero side. Satisfies [tui-presentation: Overflow counts].
- [x] 3.3 RED: `truncate(s,width)`: width 0 returns `s`; width 40 on a 100-char string gives display width <= 40 ending `…`; accented text is valid UTF-8; a `mark()` row keeps ANSI sequences intact (use a forced non-Ascii profile for this test only, restored by `t.Cleanup`). `listRows(height, chrome)`: 0 for height <= 0, else `max(3, height-lines(chrome)-2)`. Satisfies [tui-presentation: Truncate respects width].
- [x] 3.4 RED (`internal/app/window_size_test.go`): `tea.WindowSizeMsg{Width:80,Height:24}` in several states sets `width`/`height`, leaves state unchanged and returns a nil cmd; a new model has both 0 and `View()` is byte-identical to the pre-change output for history and selection. Satisfies [tui-presentation: Size message is stored], [Zero size means unchanged output].
- [x] 3.5 RED (`internal/app/history_window_test.go`): 50 runs, 80x24, cursor on the last row: `View()` has at most 24 lines, the selected row is visible, `↑ N más` equals the hidden count above; moving down from the last visible row keeps the selection visible and moves the window by one. Satisfies [run-history: Long history fits the window], [Moving down scrolls minimally].
- [x] 3.6 RED: history with 50 runs and width/height 0 renders all rows with no `más`; a run row wider than 60 at width 60 renders a line of at most 60 cells; resumable-row actions (`Enter`, `x`, `r`, `d`) still apply to the selected row outside the first window. Satisfies [run-history: Unsized history is unchanged], [Long rows are truncated].
- [x] 3.7 RED (`internal/app/selection_window_test.go`): 60 commits, 100x24, cursor on the last: at most 24 lines, cursor row visible, `↑ N más` reports the hidden count; moving up from the top of a scrolled window keeps the cursor visible and updates both counts. Satisfies [commit-selection: Long list fits the window], [Cursor moving up follows].
- [x] 3.8 RED: a commit marked with Space that scrolls out of the window is included in the plan the stub builder receives on confirm. Satisfies [commit-selection: Marks persist outside the window].
- [x] 3.9 RED: 60 commits at width/height 0 render every row, byte-equal to the pre-change output, no `más`; a 120-char message at width 60 gives a line of at most 60 cells ending `…`, including a row with a status mark. Satisfies [commit-selection: Unsized list is unchanged], [Rows are truncated to the width].
- [x] 3.10 GREEN (`window.go` new, `app.go`, `update.go`, `view.go`, `go.mod`): `window`, `overflow`, `truncate` (`ansi.Truncate(s,width,"…")`, `x/ansi` becomes a direct require), `listRows`, `lines`; `width`/`height` fields and a `WindowSizeMsg` case returning `m, nil`. History and selection views build `top`, rows and `bottom`, compute `rows := listRows(m.height, m.updateBanner()+top+bottom)`, window by cursor, add overflow lines and `truncate(row, m.width)` on data rows only. Run focused tests green.
- [x] 3.11 Commit U2 (`9e5274b`).

## Phase 4: Size check after U2

- [x] 4.1 Size check. Expected cumulative about 555. If measured + 305 + 240 + 105 + 45 + 75 passes 1,300, cut U7 then U6; if still over, STOP and report.

## Phase 5: Help overlay (U3)

- [x] 5.1 RED (`internal/app/help_overlay_test.go`): `?` on run history with rows opens an overlay whose view contains `Atajos` and one entry per binding, including a binding with no footer help text (shown by its key list), and a disabled binding shows ` (no disponible)`. Satisfies [tui-presentation: Overlay lists every binding].
- [x] 5.2 RED: `?` closes it with state and cursor unchanged; Esc closes it from a state whose Esc goes back, leaving the state unchanged (not going back). Satisfies [tui-presentation: `?` closes it], [Esc closes it without going back].
- [x] 5.3 RED: overlay open over commit selection, Space leaves the selection unchanged (all other keys inert, async messages still update underneath); `ctrl+c` returns the interrupt command. Satisfies [tui-presentation: Other keys do not act through the overlay], [Ctrl+C still interrupts].
- [x] 5.4 RED: each text-input state (ticket, package path, CANCELAR, DESPLEGAR, BORRAR from both entry points) with a non-empty buffer and with an empty buffer: `?` is appended and no overlay opens; footers advertise no `?`. Satisfies [tui-presentation: `?` is typed], [`?` on an empty buffer is also typed], and the `?` part of [Printable characters reach the buffer].
- [x] 5.5 RED (`footer_pins_test.go`): update the 50 pins listed in the Spec amendment section above with the `   ? ayuda` suffix; assert the 10 listed pins unchanged. Add an overlay pin (title `Atajos` plus footer `? ayuda` replaced by `?/Esc cerrar`). Satisfies [tui-presentation: Enabled state shows the hint] (main menu pin), [Text-input and in-flight footers are unchanged].
- [x] 5.6 RED (`footer_parity_test.go`): `?` is an ordinary enabled binding in the structural parity (listed and acting) for every fixture where `helpEnabled`; add the fixture `help open` (bindings = `[HelpClose]`); typed fixtures list no `?`; the `isTyped` equals `textInput()` cross-check from 1.2 is switched to the production method. Satisfies [tui-presentation: Parity treats `?` as acting].
- [x] 5.7 GREEN (`keymap.go`, `keys.go`, `app.go`, `view.go`): `keys.Help` and `keys.HelpClose`; `screenBindings()` split from `bindings()`; `textInput()` (ticket, package path, CANCELAR, DESPLEGAR, `cleanupStrongConfirm` incl. terminal entry); `interruptOnly`; `helpEnabled = !textInput && !interruptOnly(screenBindings())`; `bindings()` = `screenBindings()` plus `? ayuda` appended last when enabled (or `[HelpClose]` while open); `helpOpen` field; intercept in `handleKey` after the notice is cleared; `View()` returns only the overlay while open (`screenHeader("Atajos")` + `help.Model{}.FullHelpView(...)` with `overlayEntries` forcing every binding enabled, fallback label `strings.Join(Keys(), "/")`, closing footer `?/Esc cerrar`). Run the whole suite green.
- [x] 5.8 Commit U3 (`00f0933`).

## Phase 6: Size check after U3

- [x] 6.1 Size check. Expected cumulative about 860. If measured + 240 + 105 + 45 + 75 passes 1,300, cut U7 then U6; if still over, STOP and report.

## Phase 7: Phase-aware spinner (U4)

- [x] 7.1 Update existing tests that pin the exact entry command for phases that become animated, using the batch-flattening helper that skips ticks (introduce `isTick(cmd)`, the `tea.Tick` closure pointer, in `menu_cleanup_test.go`): push `p` entry, PR create `y`, cleanup `b` entry, queue load (entering the queue and `r`), quick-deploy gate entry, resume into polling. Each test keeps asserting its real command; only the extra spinner tick is tolerated. Run the suite after the edit to see exactly which tests fail once GREEN lands (they must already pass with the helper). Satisfies the design note "tests that assert the exact cmd on entry into a new spinner phase are adapted".
- [x] 7.2 RED (`internal/app/spinner_phase_test.go`): per-phase table: entering push (`p`), PR create, cleanup `b`, queue load and `r`, gate check, resume into polling returns a cmd batch including a tick and `spinning == true`. Satisfies [tui-presentation: Each new phase animates], [Progress Feedback: delta generation still animates] via the existing test kept green.
- [x] 7.3 RED: with a spinner phase active a tick returns the next tick and the view shows a spinner frame, the following tick shows a different frame (push phase). Satisfies [tui-presentation: Spinner animates during push].
- [x] 7.4 RED: spinner glyph line present in the pushing, PR creating, cleanup loading, queue loading and quick-deploy in-flight views, absent after the phase ends. Satisfies [tui-presentation: Each new phase animates].
- [x] 7.5 RED: second spinner phase entry while `spinning == true` returns no extra tick; a key on a fixture already spinning gets no seed. Satisfies [tui-presentation: One tick loop only].
- [x] 7.6 RED: a pending tick delivered after the phase completed returns a nil cmd and `spinning == false`; polling keeps the tick alive but renders no glyph; `queueLoading` is cleared on every `onQueueDone` path. Satisfies [tui-presentation: No tick after the phase ends].
- [x] 7.7 GREEN (`app.go`, `update.go`, `keys.go`, `view.go`): `queueLoading` field set by `confirmPackageReview` and queue `r`, cleared on every `onQueueDone` path; `spinnerActive()` per ADR-9; `onSpinnerTick` uses it; `Update` becomes a wrapper seeding the tick on the `!was && active && !spinning` transition (ADR-10); spinner lines in the new in-flight views. Run the whole suite green.
- [x] 7.8 Commit U4 (`75ee431`).

## Phase 8: Size check after U4 (STOP gate)

- [x] 8.1 Size check. Projection = measured + 105 (U5) + 45 (U6) + 75 (U7). If it passes 1,300, cut U7 first, then U6. If the projection still passes 1,300 with U5 alone (measured + 105 > 1,300), STOP and report to the orchestrator without starting U5; never cut U5 silently.

## Phase 9: Polling elapsed and last-poll age (U5)

- [x] 9.1 RED (`internal/app/polling_clock_test.go`): fake `Deps.Now`; `clock(d)` table `65s -> 01:05`, `61m5s -> 61:05`, `0 -> 00:00`. Polling armed at T and now T+65s renders `01:05`; at 61 min 5 s renders `61:05`. Satisfies [validation-progress: Elapsed is formatted mm:ss], [Long jobs keep counting].
- [x] 9.2 RED: a successful report at T and now T+7s renders `hace 7s`; a new report at the current instant renders `hace 0s`; an errored report leaves the age unchanged. Satisfies [validation-progress: Last poll is relative], [Last poll refreshes on a new report].
- [x] 9.3 RED: polling just started with no report shows `00:00` and `pendiente` (no `hace Ns` number). Satisfies [validation-progress: No report yet].
- [x] 9.4 RED: terminal states (succeeded, failed, canceled results) show neither `Transcurrido` nor `hace`; resume into polling restarts elapsed from the re-attach time. Satisfies [validation-progress: Terminal state drops the live lines] and the re-attach clause.
- [x] 9.5 GREEN (`app.go`, `update.go`, `view.go`): `pollStart` set where `pollDeadline` is armed (`onValidateDone`, `resumeInto` jobId branch) and both fields reset then; `lastReportAt = m.now()` in `onReportDone` on the success path only; `clock(d)`; the view adds `"  Transcurrido: %s · Último sondeo: %s\n"` after `validationBody` (`hace Ns` or `pendiente`), computed from `m.now()` at render. No new poll requests. Run the whole suite green (the existing polling pins stay green).
- [x] 9.6 Commit U5 (`ff3438a`).

## Phase 10: Size check after U5 (STOP gate)

- [x] 10.1 Size check. Projection = measured + 45 (U6) + 75 (U7). If it passes 1,300, cut U7, then U6. If the measured total itself already passes 1,300, STOP and report; never cut U5 or delete its tasks.

## Phase 11: Fast quit detector (U6, cut candidate 2)

- [x] 11.1 RED (`internal/app/menu_cleanup_test.go`): `isQuit(tea.Quit)` and the `quitCmd` closure return true; a blocking cmd whose body calls `t.Fatal` returns false without being called; helpers flattening `tea.BatchMsg` skip ticks via `isTick`.
- [x] 11.2 GREEN (test code only): `isQuit(cmd)` compares `reflect.ValueOf(cmd).Pointer()` with `tea.Quit` and the `quitCmd` closure code pointer (taken once from a Model with Git set); it never runs the cmd. Run the suite and note the wall time versus the previous run.
- [x] 11.3 Commit U6 (`ce296e3`; no spec requirements; acceptance is a green suite).

## Phase 12: Size check after U6

- [x] 12.1 Size check. If measured + 75 (U7) passes 1,300, cut U7.

## Phase 13: Shared enablement predicates (U7, cut candidate 1)

- [~] CUT 13.1 RED (`internal/app/predicate_test.go`): for each predicate `canQuickDeploy() (bool, string)`, `canRePromote()`, `rowsPresent()`, `deltaFailed()`: the footer binding is enabled exactly when the handler guard passes, over the relevant fixtures. History `x`/`r` keep their blocked-notice branches.
- [~] CUT 13.2 GREEN (`keymap.go`, `keys.go`): extract the predicates; `historyBindings`/`bindings` and the `keys.go` guards use them. Run the suite green with parity unchanged.
- [~] CUT 13.3 Commit U7. No spec requirements; if cut, record W7 as a known gap in the verify report.

## Phase 14: Final verification

- [x] 14.1 Run `go test ./... && go build ./... && go vet ./...`; all green, with no pre-existing test edited beyond the design's listed changes and the 7.1 adaptations.
- [x] 14.2 Final size check against the 1,300 ceiling (excluding openspec, against e8d783a); cross-check the Scenario Coverage Index below; update `state.yaml` (tasks and apply done, verify next); record cut units in the verify report.

## Scenario Coverage Index

- tui-presentation U1 parity fixtures (3) -> 1.1, 1.2; strict parity (3) -> 1.3, 1.4, 1.2 (valid buffers in 1.3); ticket `b` (3) -> 1.8; MODIFIED printable (3) -> 1.8, 1.9, 5.4; MODIFIED structural parity (2) -> 1.10.
- tui-presentation U2: window size (2) -> 3.4; helpers (5) -> 3.1, 3.2, 3.3.
- tui-presentation U3: overlay (5) -> 5.1, 5.2, 5.3; `?` unbound in text states (2) -> 5.4; footers (3) -> 5.5, 5.6.
- tui-presentation U4: spinner (4) -> 7.2-7.6; MODIFIED progress feedback (1) -> 7.2 (existing test kept).
- run-history: windowing (4) -> 3.5, 3.6; MODIFIED Enter (4) -> 1.5, 1.6.
- commit-selection: (5) -> 3.7, 3.8, 3.9.
- validation-progress: (6) -> 9.1-9.4.
- ai-pr-summary: (3) -> 1.7.
- U6 and U7: no spec scenarios; acceptance by suite green and parity (11.x, 13.x).
