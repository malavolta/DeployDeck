# Apply Progress: tui-ux-polish-b2

Status: U1-U6 done, U7 CUT (size). Branch feat/tui-self-update, base e8d783a. No push.

## Commits and size (cumulative changed lines, excl. openspec, vs e8d783a)

| Unit | Commit | Cumulative |
|---|---|---|
| U1 | f481f3a test(app): make footer parity strict and guard terminal-row keys | 247 |
| U2 | 9e5274b feat(app): window and truncate the history and commit lists | 592 |
| U3 | 00f0933 feat(app): add a help overlay on ? | 860 |
| U4 | 75ee431 feat(app): animate the spinner in push, PR, queue, cleanup and gate phases | 1146 |
| U5 | ff3438a feat(app): show elapsed time and last poll age while polling | 1242 |
| U6 | ce296e3 test(app): detect quit commands by pointer instead of running them | 1265 |
| U7 | CUT (1265 + 75 = 1340 > 1300) | n/a |

## TDD Cycle Evidence

| Task | RED test + failure | GREEN | Triangulation | Safety net |
|---|---|---|---|---|
| U1 1.1 sentinel | TestStateCount_Sentinel, loops over stateCount (stub const added first) | stateCount last in iota | probes stateCount and stateCount+1 | full suite green before |
| U1 1.2/1.3/1.4 fixtures, strict parity | parity failed on `history terminal row` (Enter inert) and `ready authed ai after PR` (a acts, not listed) | canResume, aiActionable | 4 named fixtures asserted; blockedNotice stays empty (no notice-only enabled key found, including empty package Enter) with a self-pruning check | existing parity and pins |
| U1 1.5-1.8 | history guard: Enter advertised on terminal row; AI guard: a acts after PR | footer and handler share predicates | resumable/terminal/cursor round trip; with/without AI and PR; ticket b empty vs non-empty (pin, passes already) | resume tests unmodified |
| U2 | window_test (undefined window), window_size_test and views: height/width not stored, no windowing | window.go, WindowSizeMsg, history and selection views | table over cursor/total/rows; 50 and 60 rows; width 60 truncation; marks outside window; zero size byte-equal | history/selection/view tests |
| U3 | help_overlay_test (helpOpen undefined) | keys.Help/HelpClose, screenBindings split, textInput, helpEnabled, overlay | Esc and ? close; modal; 6 text states x empty/non-empty; 51 pins updated, 10 unchanged; `help open` fixture; isTyped == textInput | footer pins and parity |
| U4 | spinner_phase_test (spinnerActive undefined) | spinnerActive, Update wrapper seeding on transition, queueLoading, glyph lines | 7 entry points; no seed while spinning; stale tick stops; polling keeps tick, no glyph; all 3 onQueueDone paths | whole suite; entry-command tests adapted with withoutTicks |
| U5 | polling_clock_test (clock undefined) | pollStart, lastReportAt, clock, pollingClock line | 65s, 61:05, pendiente, hace 0s/7s, error leaves age, terminal, resume restarts | existing polling tests |
| U6 | TestIsQuit_IdentifiesWithoutRunning | isQuit by code identity | tea.Quit, quitCmd closure, tick, blocking cmd with t.Fatal | whole suite |

## Deliberate edits to existing tests
- footer_pins_test.go: 50 pins plus the new U1 `history terminal row` pin gain `   ? ayuda` (51), 10 stay identical; `history` pin now uses an InProgress run, `history re-promote` loses `Enter reanudar` (terminal row), `help open` pin added.
- branch_cleanup_test.go: empty-list prune footer gains `? ayuda`.
- `run()` in flow_e2e_test.go and a few direct `cmd()` calls strip spinner ticks via withoutTicks (7.1); standalone_validate_test.go no longer unwraps a batch.
- boundary_test.go untouched.

## Deviations
- U7 cut: W7 stays a known gap (record in verify).
- U6 does not reduce wall time: the slow tests are real-git e2e tests (5s each), not isQuit. Suite is about 63s before and after.
- isTick/isQuit use the closure symbol name (runtime.FuncForPC); isBatch now runs prefiltered commands and checks for tea.BatchMsg (corrective round).
- In U2 and U4 some implementation stubs landed in the same step as tests (RED confirmed by compile failure / failing view tests, not a separate stub run).
- Task 1.2 isTyped cross-check landed with U3 (task 5.6) as designed.

## Corrective round (verify FAIL, commit 5c9d708)
All fixes RED first, then GREEN; `go test ./internal/app -gcflags=all=-l` passes (60s).

| Finding | RED | GREEN |
|---|---|---|
| C1 (tasks 1.3/1.4) | `pressed(..., isBlockedNotice(state,k), usedBlocked)` replaces hard-coded `listed=true`; Structural failed for quick deploy, package review empty, state 3, 4, 10, 24 (Enter notice-only) | Fixtures made realistic (below); `blockedNotice` used and the unused-entry check now works (`used` is set only when a listed key produces a non-empty notice) |
| W1 | `TestWindow_MidListFitsLikeBubbletea`: 25 lines vs 24 for history and selection | `listRows` reserves 3 rows (2 indicators + trailing newline); `TestListRows` expectation 19 -> 18; height 0 untouched |
| W2 (task 1.2) | `TestIsTyped_MatchesProduction` over every fixture (passes: predicates already equal; guards drift) | n/a |
| W3 | `-gcflags=all=-l` failed at HEAD | `isBatch` runs only prefiltered non-tick candidates and checks `tea.BatchMsg`; `checkDetection` canary (sync.Once, panics) runs from isTick/isBatch so it applies under any `-run` filter |
| S1 | n/a | `pkgDir` removed in TestMain |
| S3 | `TestQueueReview_EscClearsLoading` failed | `keyQueueReview` Esc sets `queueLoading=false` |

Six notice-only Enter keys:
- Quick deploy (fixture "quick deploy" and state 24): (a) `quickReady` gives an eligible Succeeded run (Target UAT) and `QuickDeploy.AllowExecution`; DESPLEGAR is already typed.
- CommitSelection (state 3): (a) first item selected.
- TargetSelection (state 4): (a) `targetCursor=1` (UAT has a sandbox, INT has none).
- PickVerification (state 10): (a) `repoState.InProgress=false`.
- Package review empty: (b) legitimate: Enter needs the `o` override first, so `blockedNotice[StatePackageReview]={"enter"}` (a notice is asserted non-empty to count).

Size: 1379 changed lines (1221 added + 158 deleted) vs e8d783a, over the 1300 ceiling by 79; the corrective commit adds 136 (119+/17-).
