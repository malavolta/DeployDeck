# Tasks: TUI UX Consistency (change A)

Test command: `go test ./...` (Go lives at `/usr/local/go/bin`, not on PATH). Strict TDD: each behavior task is a RED test written and seen failing, then its GREEN implementation. Table tests, `t.TempDir()`, fakes and stubs per the go-testing skill. No test touches the network, `brew`, or a real Salesforce org. Commits follow the work-unit-commits skill: one reviewable unit per commit, tests and implementation together, conventional commit messages, no AI attribution. Work in the main checkout on `feat/tui-self-update` (single PR with tui-self-update, PR #4). Existing resume, `quitCmd`, ctrl+c and `boundary_test.go` tests stay green and unmodified, except the pins the design lists (run_history_menu_test last entry, history/cleanup/ticket back targets).

Notation: spec scenarios are referenced as `[capability: scenario]`. Test helpers: reuse the existing app model builders; the quit command is detected by running the cmd and expecting `tea.QuitMsg` (`Deps.Git` nil).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | Original ~790. Actual after the corrective round: ~1505 (against fd7dac1, excluding openspec). Size exceptions approved: 970, then 1150 |
| Applied budget (session) | 800 lines (this change's own diff, excluding `openspec/`) |
| 400-line budget risk | High (exceeds 400, final 1505 lines under a user-approved size exception (800 -> 970 -> 1150 -> 1505)) |
| Chained PRs recommended | No (same PR as tui-self-update at the user's request) |
| Suggested split | Single PR, 7 commits (work units below) |
| Delivery strategy | single-pr |
| Chain strategy | n/a |

Decision needed before apply: No
Chained PRs recommended: No
400-line budget risk: High

Status: ADR-9 (Unit 6 conflict labels, 6.5-6.8) is CUT. ADR-8 (Unit 7) is replaced by a targeted fixed-width fix (7.2, 7.3 satisfied; 7.1, 7.4, 7.5 not done as written). Original cut order if the running total passes 800 (measure at task 6.1): first ADR-9 conflict labels (Unit 6, about -35 lines; also drops the scenarios `Conflict kinds are shown in Spanish` and `Displayed type labels are Spanish`, and the kind half of the English-labels copy; record the delta in the verify report), then ADR-8 column widths (Unit 7, about -30 lines; also drops the three `Labeled Table Column Headers` width scenarios except `Run history shows column labels`, which stays as a pin of existing behavior if it already passes).

### Suggested Work Units

| Unit | Goal | Commit | Focused test command | Rollback boundary |
|------|------|--------|----------------------|-------------------|
| 1 | Conflict cursor, `e` target, nav, marker | `fix(app): give the conflict screen its own cursor` | `go test ./internal/app/ -run Conflict` | Revert commit; one new field |
| 2 | Menu cleanup entry, ticket esc, cleanup back targets, BORRAR prompt | `feat(app): add branch cleanup to the main menu and fix back targets` | `go test ./internal/app/ -run 'Menu\|Cleanup\|Ticket\|Borrar'` | Revert commit |
| 3 | esc/q rule matrix and handler edits | `feat(app): apply one esc and q rule across every screen` | `go test ./internal/app/ -run EscQ` | Revert commit |
| 4 | Blocked-key notices, history reasons, Spanish notices | `feat(app): explain blocked keys with Spanish notices` | `go test ./internal/app/ -run 'Notice\|Reason\|History'` | Revert commit |
| 5 | Footer normalization and parity table | `feat(app): list exactly the keys that act in every footer` | `go test ./internal/app/ -run FooterParity` | Revert commit |
| 6 | Remaining Spanish copy and conflict kind labels (ADR-9, cut lever 1) | `feat(app): translate remaining English copy and conflict kinds` | `go test ./internal/app/ ./internal/git/...` | Revert commit |
| 7 | Rune-sized table columns (ADR-8, cut lever 2) | `fix(app): size table columns to their headers` | `go test ./internal/app/ -run Column` | Revert commit |

Order is strictly sequential (single writer). Unit 3 needs Unit 2 (`enterBranchCleanup`, cleanup back targets). Unit 5 needs Units 1-4 so its parity fixtures are written against final handlers and footers. Units 6 and 7 are independent of each other but run after Unit 5, which already forces footer copy to its final form.

## Phase 1: Conflict cursor (Unit 1)

- [x] 1.1 RED (`internal/app/conflict_cursor_test.go`): 3 files, commit-selection `m.cursor`=3, enter conflict via a pick entry; view has exactly one `>` marker on file 1. Satisfies [cherry-pick: Cursor starts on the first file regardless of commit selection], [Marker rendering].
- [x] 1.2 RED: resume entry (`update.go:452` path) renders marker on file 1. Satisfies [Resumed entry resets the cursor].
- [x] 1.3 RED: move to file 2, drive a second conflict entry, marker on file 1. Satisfies [Cursor resets on every entry].
- [x] 1.4 RED: table of `down`,`j`,`down`(clamp),`up`,`k`,`up`(clamp) with the expected marker row at each step. Satisfies [Navigation keys move the marker].
- [x] 1.5 RED: cursor on file 3, `repoStateCmd` refresh reports 1 unmerged file, view marks file 1, no panic. Satisfies [Cursor is clamped when the list shrinks].
- [x] 1.6 RED: files a,b,c, `down`, `e`, stub `Edit` receives `b`; with stale `m.cursor`=2 and conflict cursor 0, `e` gets file 1. Satisfies [Edit Key: e opens the highlighted file], [e does not follow a stale commit-selection cursor].
- [x] 1.7 RED: conflict footer with 2 files contains `↑/↓` and `e `. Satisfies [Conflict Footer: Footer names navigation and edit].
- [x] 1.8 GREEN (`app.go`, `keys.go`, `view.go`, `update.go`): add `conflictIdx`, `conflictCursor()` clamp, reset to 0 at both entry sites, `up/k/down/j` in the conflict handler, `e` uses `conflictCursor()`, `>` marker column, footer `↑/↓ elegir   e editor ...   q salir (el run queda reanudable)`. Run focused tests green.
- [x] 1.9 Commit Unit 1 (landed in squashed commit 4d2c81e; units interleave in keys.go/view.go).

## Phase 2: Menu entry, ticket esc, cleanup navigation (Unit 2)

- [x] 2.1 RED (`internal/app/menu_cleanup_test.go`): menu labels in order have "Limpieza de ramas" immediately after "Historial de runs" and last; the line carries a non-empty Spanish description. Satisfies [standalone-modes: Entry order], [Entry has a description], [Main Menu: No resumable run shows the menu]. Update the `run_history_menu_test` last-entry pin here.
- [x] 2.2 RED: ticket pre-filled, cursor on the entry, Enter gives the cleanup state (cleanup load cmd returned). Satisfies [Entry opens cleanup with a pre-filled ticket].
- [x] 2.3 RED: empty ticket, `b` opens cleanup. Satisfies [b shortcut remains].
- [x] 2.4 RED: ticket `esc` with empty buffer and with typed text gives `StateMainMenu`, no quit cmd. Satisfies [Ticket Input Esc: Esc on ticket goes to the menu], [Esc with text cancels and returns to the menu].
- [x] 2.5 RED: cleanup list from menu, `esc` gives `StateMainMenu`; same after entering via ticket `b`; result view `esc` gives menu. Satisfies [branch-cleanup: Esc returns to the menu].
- [x] 2.6 RED: cleanup list `q` returns the quit cmd. Satisfies [branch-cleanup: q quits].
- [x] 2.7 RED: on the y/n confirm, prune confirm and the BORRAR prompt, `esc` returns to the cleanup list (not the menu); on y/n and prune, `q` also returns to the list with no quit cmd. Satisfies the confirm-view clause of the requirement and [Esc and q inside the prompt] (esc half).
- [x] 2.8 RED: BORRAR prompt for `deploy/DD-7` contains `deploy/DD-7` and `BORRAR`. Satisfies [BORRAR Prompt Names The Branch: Prompt shows the branch name].
- [x] 2.9 RED: BORRAR gate table (wrong word + Enter keeps the branch; exact `BORRAR` deletes); non-empty buffer `q` appends and does not quit. Satisfies [Gate unchanged], [Esc and q inside the prompt] (q half).
- [x] 2.10 GREEN (`keys.go`, `app.go`, `view.go`): extract `enterBranchCleanup()` used by the menu entry and ticket `b`; add the menu entry; ticket esc goes to `StateMainMenu`; cleanup list/result esc to menu, `q` to `quitCmd()`; confirm views esc/q to list; BORRAR prompt names the branch and gets guarded `q`.
- [x] 2.11 Commit Unit 2 (landed in squashed commit 4d2c81e; units interleave in keys.go/view.go).

## Phase 3: Esc/q rule matrix (Unit 3)

- [x] 3.1 RED (`internal/app/esc_q_matrix_test.go`): fixture table `state -> {esc, q}` expecting `Quit`, `Back(State)` or `None`, per the design's esc/q table (every row, including sub-phases pushReady, pushConfirm, delete confirm, gate-blocked, branch collision, sandbox, delta source, run history). Cases that pass today are pinned and must stay green. Satisfies [tui-presentation: Main menu esc and q both quit], [q in a picker quits, esc goes back], [Esc goes back where nothing has happened yet], [Esc is a no-op after a main-flow point of no return] (esc half), [Esc and q do nothing in flight], [standalone-modes: Esc and q quit from the menu], [Picker esc and q], [cherry-pick/run-history picker rows].
- [x] 3.2 RED: pushConfirm `esc` goes to `StateSucceeded` with no push cmd; `q` returns quit cmd. Satisfies design row pushConfirm.
- [x] 3.3 RED: terminal states (Succeeded, Aborted, Failed, Canceled, Error) each quit on `esc`, `q` and Enter. Satisfies [Terminal screens quit on esc, q and Enter].
- [x] 3.4 RED: typed states (CANCELAR, DESPLEGAR, BORRAR, ticket, package path): non-empty buffer + `q` appends, no quit; empty buffer + `q` equals `esc` (back, no quit). Table covers all five. Satisfies [Typed confirmation never quits on q], [Main-Menu: q on an empty free-text buffer goes back, not out], [`q` is appended into a non-empty free-text buffer].
- [x] 3.5 RED: y/n states (cleanup confirm, prune, PR confirm, delete confirm): `esc` and `q` both cancel, no quit cmd. Satisfies [y/n confirmation].
- [x] 3.6 RED: `q` from a main-flow state with a persisted run returns quit cmd and the run record still exists and is offered for resume. Use a fake run store. Satisfies [q quits from a main-flow screen without destroying work].
- [x] 3.7 RED: `ctrl+c` returns the interrupt cmd in `StateCherryPicking` and in a typed state with text (buffer unchanged). Pins existing behavior. Satisfies [Ctrl+C Remains A Hard Interrupt: both scenarios].
- [x] 3.8 RED (`internal/app/subphase_test.go`, `TestSubPhase_DeclinedResumeIsOfferedAgain`): resume offer shown through history: `esc` declines, `q` quits, the run record is neither deleted nor terminal, and the next startup offers resume again. Satisfies [run-history: Declining with esc, then q, keeps the run resumable].
- [x] 3.9 GREEN (`keys.go`): edit each handler in place per the design table. `quitCmd()` is reused and unchanged. Run the matrix green.
- [x] 3.10 Commit Unit 3 (landed in squashed commit 4d2c81e; units interleave in keys.go/view.go).

## Phase 4: Notices, reasons, history (Unit 4)

- [x] 4.1 RED (`internal/app/notices_test.go`): `p` with no branch leaves the state unchanged and `View()` contains `rama`. Satisfies [Spanish Notice: p with no branch shows a notice].
- [x] 4.2 RED: `d` with no branch on Succeeded and on Aborted: no delete flow, view contains `rama`. Satisfies [d with no branch shows a notice] and the Aborted empty-branch gate.
- [x] 4.3 RED: standalone delta Enter that cannot proceed leaves the state unchanged and shows a Spanish notice; delta `e` acts only when `deltaErr != nil` (ADR-10). Satisfies [Enter in standalone delta shows a notice], [standalone-modes: Enter that cannot proceed explains why].
- [x] 4.4 RED: show a notice, navigate to another screen, new view lacks the notice. Satisfies [Notice clears on navigation].
- [x] 4.5 RED (`internal/app/notices_test.go`): `x` on an ineligible run with a known reason keeps `StateRunHistory`, notice is non-empty and has no English reason text. Satisfies [x on an ineligible run explains the reason], [Quick-deploy action on a non-eligible row explains].
- [x] 4.6 RED: for every reason string `runs.QuickDeployEligible` can return (enumerated from `runs/quick_deploy_test.go`), `quickDeployReasonES` yields distinct Spanish text; an unknown reason falls back to a generic Spanish notice. Satisfies [Each QuickDeployEligible reason maps to Spanish] and the ADR-6 drift test.
- [x] 4.7 RED: `r` on Failed, Canceled and Aborted rows: no re-promotion, state/selection/rows unchanged, notice contains `exitos`. Satisfies [r on a Failed run explains the restriction], [r on a Failed run does not start and explains].
- [x] 4.8 (satisfied by existing tests: quick_deploy_test.go:186-199, re_promote_test.go:119,173) RED: `r` on Succeeded and SucceededPartial still starts re-promotion; Enter still resumes; `d` unchanged. Pins existing behavior. Satisfies [r on a Succeeded run starts re-promote], [r on a SucceededPartial run starts re-promote], [Enter still resumes independently of r], [Enter, d, and r remain unaffected], [Quick-deploy action on an eligible row opens the view].
- [x] 4.9 RED: history notice cleared by moving the selection and by leaving the screen. Satisfies [Notice clears on movement].
- [x] 4.10 RED: table over every former English notice site (keys.go:550, 562, 868, 908, 1053, 1088, 1124, 1208; update.go:570, 970), each trigger yields a Spanish notice. Satisfies [Correct Spanish Copy: Former English notices are Spanish].
- [x] 4.11 GREEN: add `noticeLine()` (ADR-7, new sites only), `quickDeployReasonES`, the new notices in `keys.go`/`update.go`, and Spanish text for the former sites. Delta `e` gate. Run focused tests green.
- [x] 4.12 Commit Unit 4 (landed in squashed commit 4d2c81e; units interleave in keys.go/view.go).

## Phase 5: Footer normalization and parity (Unit 5)

- [x] 5.1 RED (`internal/app/footer_parity_test.go`): fixture table covering every State plus push, cleanup and delete sub-phases, Succeeded with and without a branch, and push-ready with and without AI configured. Parser extracts tokens from the `[ ... ]` footer of `View()`; helper `acts()` per ADR-2 (state change, non-nil cmd, or `View()` change with `notice` cleared on both models). Asserts every advertised token acts (multi-key tokens: at least one key) and every acting probe key (a-z, enter, esc, up, down, space; aliases k/j, N/S/O/E; runes exempt on text states) is advertised. Satisfies [Footer Lists Exactly The Keys That Act: Footer parity matrix].
- [x] 5.2 RED: fragment pins: polling `c cancelar`, `r refrescar`, `q salir (el job sigue activo)`; conflict `el run queda reanudable`; commit selection `↑/↓` and `x`; Succeeded with branch contains `d`, without it does not; push-ready lists `a` only with AI and `a` is inert without. Satisfies [Polling footer lists cancel], [Terminal footers list delete only with a branch], [Commit selection footer lists navigation and x], [Push-ready footer follows AI configuration].
- [x] 5.3 RED: in-flight footers (discovery, branch creation, cherry-picking, cleanup loading) contain `ctrl+c interrumpe` and neither `q salir` nor `esc`; main-flow states past the point of no return have no `esc`. Satisfies [In-flight footer states the exit affordance], [In-flight footer replaces the quit affordance].
- [x] 5.4 RED: history, cleanup list, confirm and result footers contain `Esc volver` and `q salir` (confirms: `n/Esc/q cancelar`); none mentions q returning to ticket input. Satisfies [run-history: Footer pins both keys], [branch-cleanup: Footer matches handlers].
- [x] 5.5 GREEN (`view.go`): normalize every `footer("...")` to `<keys> <label>` segments with three-space separators, keys joined by `/`, per the design's footer-delta column. Run `FooterParity` green and fix any handler/footer mismatch found.
- [x] 5.6 Commit Unit 5 (landed in squashed commit 4d2c81e; units interleave in keys.go/view.go).

## Phase 6: Size check, then Spanish copy and conflict labels (Unit 6)

- [x] 6.1 Size check: `git diff --stat main...HEAD -- . ':!openspec'` minus the tui-self-update commits (diff against the commit before Unit 1). Record the running total of this change. If the total plus the remaining forecast of Units 6 and 7 (about 65) exceeds 800, apply the cut order (drop ADR-9 first, then ADR-8) and note it in the verify report.
- [x] 6.2 RED (`internal/app/copy_test.go`): plan preview with destructive changes, cleanup list with a pushed and a local-only branch, validating job: none contains `Destructive changes`, `pushed`, `local`, `validating`, `title:`; each shows a Spanish equivalent. Satisfies [English labels are replaced].
- [x] 6.3 RED: validation-result screen title is `Resultado De Validación`. Satisfies [Validation-result title is accented].
- [x] 6.4 RED: destructive confirmation renders opening with `¿` and closing with `?`. Satisfies [Yes/No prompts use inverted question marks].
- [~] CUT 6.5 RED (cut lever 1, `internal/git/conflict_test.go`): `ConflictKind.Label()` returns a Spanish label for text, binary, modify, delete; `String()` values unchanged. Continue-gate Pending text uses labels. Satisfies [cherry-pick: Displayed type labels are Spanish], the git classification-unchanged clause.
- [~] CUT 6.6 RED (cut lever 1, `internal/app`): conflict screen with kinds text, binary, modify, delete shows no English kind word as label; `git.Conflict` kind values unchanged. Satisfies [Conflict kinds are shown in Spanish], [Conflict stops the flow with classified files].
- [~] CUT 6.7 (the copy half landed with 6.2-6.4; only `Label()` is cut) GREEN: copy changes in `view.go` ("Cambios destructivos", "título:", "pusheada/local", "validando", `¿...?`, accented title); `Label()` in `internal/git/conflict.go` and `continue_gate.go`, used by the conflict view.
- [~] CUT 6.8 Commit Unit 6.

## Phase 7: Column widths (Unit 7)

- [~] CUT 7.1 RED (`internal/app/columns_test.go`): history, queue and cleanup screens with at least one row each: header and first data row column offsets (by rune index) are equal for every column. Satisfies [Labeled Table Column Headers: Run history shows column labels].
- [x] 7.2 (satisfied by `TestSubPhase_CleanupColumnsFitHeaders`) RED: cleanup header contains `Antigüedad` in full and the next column's first character matches its header offset. Satisfies [Cleanup age column fits its header].
- [x] 7.3 (satisfied by `TestSubPhase_HistoryDestinoFitsHeader`) RED: the screen whose header holds `Destino` shows it in full and the next column stays aligned. Satisfies [Target column fits its header].
- [~] REPLACED 7.4 (targeted fixed-width fix in view.go instead of a rune-width helper; ADR-8 partially restored) GREEN (`view.go`): `%-*s` with `w = max(dataMin, utf8.RuneCountInString(header))` at the three tables.
- [~] CUT 7.5 Commit Unit 7.

## Phase 8: Final verification

- [x] 8.1 Run `go test ./... && go build ./...` and `go vet ./...`; all green, with no unmodified pre-existing test edited beyond the design's listed pins.
- [x] 8.2 (size 1505 = user-approved exception; verify round 2 PASS WITH WARNINGS) Final size check against the 800-line budget; confirm no scenario in the five specs lacks a test (cross-check the references above); update `state.yaml` (tasks done, apply next).

## Scenario Coverage Index

- tui-presentation: all 9 esc/q scenarios -> 3.1-3.8; ctrl+c (2) -> 3.7; footer (6) -> 5.1-5.3; notices (4) -> 4.1-4.4; MODIFIED footer grammar (3) -> 3.4, 5.3; copy (5) -> 4.10, 6.2-6.6; columns (3) -> 7.1-7.3.
- cherry-pick: conflict cursor (6) -> 1.1-1.5, 1.2; edit (2) -> 1.6; footer (1) -> 1.7; modified (2) -> 6.5, 6.6.
- standalone-modes: entry (4) -> 2.1-2.3; ticket esc (2) -> 2.4; menu (3) -> 2.1, 3.1; delta (4) -> 4.3, 3.1.
- branch-cleanup: nav (3) -> 2.5-2.7, 5.4; BORRAR (3) -> 2.8, 2.9, 2.7; listed (2) -> 6.2 and existing tests.
- run-history: esc/q (4) -> 3.1, 3.8, 5.4; reasons (4) -> 4.5-4.9; r modified (4) -> 4.7, 4.8; x modified (3) -> 4.5, 4.8.
