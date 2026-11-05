# Tasks: TUI Bubbles Adoption (change B, core)

> **Amendment 2026-11-05**: U3 (window size and history windowing) and U4 (commit-selection windowing and truncation) were CUT to B2 by user decision. Shipped scope is U1, U2 and U5 in commits `d0ebf0b`, `16f577f`, `14da889`, plus the verify fixes in `360d0d6`. Measured size is about 1,428 changed lines (excluding openspec), approved by the user as the size exception on 2026-11-05 (the original ceiling was 1,200). Phases 5 and 6 are `[~] CUT`. Their checkboxes are kept for traceability. See `verify-report.md` and `archive-report.md`.

Test command: `go test ./...` (Go lives at `/usr/local/go/bin`, not on PATH; use `export PATH=$PATH:/usr/local/go/bin`). Strict TDD: each behavior task is a RED test written and seen failing, then its GREEN implementation. Table tests, `t.TempDir()`, fakes and stubs per the go-testing skill. No test touches the network, `brew`, or a real Salesforce org. Commits follow the work-unit-commits skill: one reviewable unit per commit, tests and implementation together, conventional commit messages, no AI attribution. Work in the main checkout on `feat/tui-self-update` (single PR, PR #4). Existing resume, esc/q matrix, `quitCmd`, ctrl+c and `boundary_test.go` tests stay green and unmodified, except the changes the design lists (`footer_parity_test.go` replaced, `TestFooterParity_SubPhaseKeys` removed from `subphase_test.go` with its fixtures moved, `color_test.go` gets `SetHasDarkBackground(true)`).

Notation: spec scenarios are referenced as `[capability: scenario]`. Reuse the existing app model builders and the `acts()` helper (change A ADR-2). The quit command is detected by running the cmd and expecting `tea.QuitMsg` (`Deps.Git` nil).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | About 1,320 forecast (adds plus deletes, excluding openspec, measured against 4cc5ea6): U1 950, U2 30, U3 210, U4 95, U5 38. **Actual at 4.1: 1,428** for U1+U2+U5 (U3 and U4 CUT), plus a few lines from `360d0d6` (not re-measured) |
| Applied budget (session) | 1,428 lines (user-approved size exception on 2026-11-05; the original 1,200 ceiling is superseded; review target was 1,000) |
| 400-line budget risk | High (exceeds 400, user-approved size exception) |
| Chained PRs recommended | No (same PR #4 at the user's request) |
| Suggested split | Single PR, up to 5 commits (work units below) |
| Delivery strategy | single-pr |
| Chain strategy | n/a |

Decision needed before apply: Yes, the user must run `go get github.com/charmbracelet/bubbles@v0.21.0 && go mod tidy` and review the go.sum diff before U1 (tasks 3.1+). U2 and U5 do not need it.
Chained PRs recommended: No
400-line budget risk: High

*Amendment 2026-11-05: the bubbles dependency blocker is RESOLVED (resolved from the module cache, see 3.1). The size decision was made by the user (1,428 accepted). The forecast lines above are the original plan and are kept for traceability.*

Cut plan (measured at 4.1 and 5.1): U2+U5+U1 project to about 1,018. Adding U3 (210) gives about 1,228, which is over 1,200, so the expected outcome is that U3 and U4 both move to B2. *Amendment 2026-11-05: the measured total at 4.1 was 1,428 for U1+U2+U5. The user accepted 1,428 and CUT U3 and U4 to B2, so the cut rule below was applied.* Rule: if projected total (running measured total + forecast of the remaining units) exceeds 1,200, cut U4 first, then U3. Cutting U3 or U4 means: skip its phase, mark its tasks `[~] CUT`, and remove its requirements from the spec deltas (`tui-presentation`: `[U3] Window Size Is Tracked`, `[U3] Lists Are Windowed...`, `[U3] Long Text Is Truncated...`; `run-history` delta whole file for U3; `commit-selection` delta whole file for U4), recording the delta in the verify report. U3 and U4 share `window.go`: cutting U3 forces cutting U4.

### Suggested Work Units

| Unit | Goal | Commit | Focused test command | Rollback boundary |
|------|------|--------|----------------------|-------------------|
| U2 | Wrong-word notice in the BORRAR prompt | `fix(app): show the wrong-word notice in the strong delete prompt` | `go test ./internal/app/ -run 'Borrar\|WrongWord'` | Revert commit |
| U5 | AdaptiveColor palette | `feat(app): use darker status colors on light terminals` | `go test ./internal/app/ -run 'Color\|Mark'` | Revert commit |
| U1 | Keymap, footers from bindings, handlers, structural parity | `refactor(app): derive footers and handlers from one keymap` | `go test ./internal/app/ -run 'Keymap\|Footer\|Parity\|Pin'` | Revert commit, go.mod/go.sum |
| U3 (CUT to B2) | Window size and history windowing | `feat(app): window the run history to the terminal size` | `go test ./internal/app/ -run 'Window\|Truncate\|History'` | Not applicable (not shipped) |
| U4 (CUT to B2) | Commit-selection windowing and truncation | `feat(app): window and truncate the commit list` | `go test ./internal/app/ -run 'Selection\|Window'` | Not applicable (not shipped) |

Order is strictly sequential (single writer): U2, U5, U1 (gated), size check, U3, size check, U4. U3 needs U1 only for the footer, not for code; it can run before U1 only if the dependency is still missing (design Rollout fallback), in which case the cut is U4 then U3.

## Phase 1: Wrong-word notice (U2)

- [x] 1.1 RED (`internal/app/borrar_notice_test.go`): strong confirmation for `deploy/DD-7` with `borrar` and with `BORR` typed, Enter; `View()` contains `escribe BORRAR exactamente para confirmar el borrado`, the stub delete is not called, state and buffer unchanged. Satisfies [branch-cleanup: Wrong word renders the notice].
- [x] 1.2 RED: same prompt with `BORRAR`, Enter; the delete runs and the view has no wrong-word notice. Pins existing behavior. Satisfies [branch-cleanup: Exact word still deletes].
- [x] 1.3 RED: with the notice shown, a typed rune clears it; separately `esc` leaves the prompt and the next view lacks it. Satisfies [branch-cleanup: Notice clears].
- [x] 1.4 RED: table of both entry points (current run branch deletion via `deleteConfirmView` from Succeeded/Aborted, and the cleanup list `cleanupStrongConfirm`): wrong word renders the same notice in each. Satisfies [branch-cleanup: Notice is visible in both entry points].
- [x] 1.5 GREEN (`view.go`): in the `cleanupStrongConfirm` branch of `deleteConfirmView` render prompt, `"  " + buffer + "_\n"`, `m.noticeLine()`, footer (ADR-10). Run focused tests green.
- [x] 1.6 Commit U2.

## Phase 2: AdaptiveColor (U5)

- [x] 2.1 RED (`internal/app/color_test.go`): `TestMain` also calls `lipgloss.SetHasDarkBackground(true)`. With dark background and a forced TrueColor/ANSI256 profile, OK/warn/err tokens emit SGR 92/93/91. Satisfies [tui-presentation: Dark background keeps the bright palette]. May already pass; keep as pin.
- [x] 2.2 RED: with `lipgloss.SetHasDarkBackground(false)` (restored by `t.Cleanup`), tokens emit SGR 32/33/31. Satisfies [tui-presentation: Light background uses darker shades].
- [x] 2.3 RED: under the Ascii profile and each background setting, `OK`/`!!`/`XX`/`i` render as plain text with no ESC. Pin. Satisfies [tui-presentation: Ascii profile output is unchanged].
- [x] 2.4 GREEN (`style.go`): OK `AdaptiveColor{Light:"2",Dark:"10"}`, Warn `{Light:"3",Dark:"11"}`, Err `{Light:"1",Dark:"9"}`. Confirm `TestMark_NonAsciiUsesBrightColors` stays green and unmodified.
- [x] 2.5 Commit U5.

## Phase 3: Keymap, footers, handlers, structural parity (U1)

- [x] 3.1 GATE: verify the dependency is present: `go list -m github.com/charmbracelet/bubbles` must print `github.com/charmbracelet/bubbles v0.21.0`, and `go build ./...` must resolve `bubbles/key` and `bubbles/help`. If the module is missing, STOP and report to the orchestrator and the user (`go get github.com/charmbracelet/bubbles@v0.21.0 && go mod tidy`, then review go.sum). Do not start any U1 task. Fallback: do Phase 5 and 6 before U1 and cut per the design Rollout. *Resolved 2026-11-05: bubbles v0.21.0 was resolved from the module cache (`GOPROXY=off GOFLAGS=-mod=mod go get github.com/charmbracelet/bubbles@v0.21.0`, then `go mod tidy`). The fallback was not needed.*
- [x] 3.2 RED (`internal/app/keymap_test.go`): `window`-free unit tests of the vocabulary: `hint()` returns a copy with new help text and enablement without mutating the shared `keys.X` binding; `footerOf()` returns `""` when all bindings are disabled or help-less, and `[ k desc   k desc ]` otherwise; disabled bindings are skipped with no stray separator. Satisfies [tui-presentation: Entries are separated by three spaces], [Footer Grammar: no footer line for no enabled binding].
- [x] 3.3 RED (`internal/app/footer_pins_test.go`): table of every byte-identical footer string from the design's pin list checked against fixture `View()` (reuse `escQFixtures` + `subPhaseFixtures`), including main menu, commit selection with commits and run history with rows. Mostly passes today and must stay green after GREEN. Satisfies [tui-presentation: Footer bytes are unchanged].
- [x] 3.4 RED (`internal/app/footer_parity_test.go`, replaces the file; keep `PinnedFragments`): structural parity per design Testing Strategy. Coverage loop over `State(0)...` until `viewBody()` is `""`; every state needs at least one fixture and non-empty `bindings()`. Fixtures: `escQFixtures` + `subPhaseFixtures`, conflict with continue on and off, history with and without eligible rows, lists empty and non-empty. Asserts (1) last footer equals `footer(helpView.ShortHelpView(m.bindings()))`, (2) each enabled binding has at least one acting key, (3) no key of a disabled binding acts, (4) every acting key in the universe U belongs to an enabled binding (runes exempt in typed states). *Amendment 2026-11-05: the fixtures stayed in `subphase_test.go` (`subPhaseFixtures` and `acts()`) and are reused by the structural test, not moved, to save lines. `TestFooterParity_SubPhaseKeys` was removed from `subphase_test.go`.* Original plan: move the `TestFooterParity_SubPhaseKeys` fixtures into it and delete that test from `subphase_test.go`. Satisfies [tui-presentation: Every state is covered], [No listed key is inert and no acting key is hidden], [Keys an enabled binding names do act], [Footer parity matrix is structural].
- [x] 3.5 RED: conflict state with the continue gate reporting pending reasons: footer has no `c` entry and `c` returns no cmd and leaves the state unchanged (the existing blocked-notice branch may still set a notice, per ADR-3); with the gate satisfied the footer contains `c` and `c` continues. Satisfies [Conflict footer hides `c` while continue is disabled], [Conflict footer shows `c` once continue is enabled].
- [x] 3.6 RED: push-ready without AI has no `a` entry and `a` does not start the action; Succeeded with no promotion branch has no `d` entry and `d` does not start delete (notice only). Satisfies [A key disabled by model state is neither shown nor acted on], [tui-presentation: Push-ready footer follows AI configuration] pin.
- [x] 3.7 RED: polling footer contains `c cancelar` and `c` acts; in-flight footers contain `ctrl+c interrumpe` and neither `q salir` nor `esc`. Pins from change A. Satisfies [tui-presentation: Polling footer lists cancel].
- [x] 3.8 RED: text-input states (ticket, package path, CANCELAR, DESPLEGAR, BORRAR from both entry points) with a non-empty buffer: typing `c`, `d`, `x`, `p` appends to the buffer with no other effect; footers advertise no printable key. Satisfies [tui-presentation: Printable characters reach the buffer].
- [x] 3.9 RED: in the same states `q` with a non-empty buffer appends and with an empty buffer acts as `esc`, never quitting. Satisfies [tui-presentation: `q` keeps its guarded behavior].
- [x] 3.10 RED (`internal/app/footer_source_test.go`): the source of `view.go` contains no `footer("` (ADR-5). Fails until every call site is migrated.
- [x] 3.11 GREEN (`keymap.go`, new): `keyMap` vocabulary `keys`, `hint()`, `helpView` (`ShortSeparator: "   "`, zero Styles), `footerOf()`, `bindings()` mirroring `viewBody`, per-state `<state>Bindings()` with sub-phase switches, predicates `continueEnabled`, `hasBranch`, `aiAvailable` (replaces `aiFooterPart`), `quickDeployOK`, `isRePromoteEligible`. Add `bubbles` to `go.mod`/`go.sum`, `x/ansi` as direct. Non-test imports are `key` and `help` only.
- [x] 3.12 GREEN (`view.go`): migrate all 61 footers to `footerOf(m.<state>Bindings()...)`, including `ctrl+c interrumpe` (`keys.Interrupt`, display-only) and Error/Failed/Canceled `Exit`. Remove `aiFooterPart` and `runHistoryFooter`. `footer(string)` stays private. Run 3.3, 3.10 green.
- [x] 3.13 GREEN (`keys.go`, `update.go`): convert every `switch msg.String()` to `switch { case key.Matches(msg, keys.X): ... }`; keys with blocked-key notices (conflict `c`, Succeeded/Aborted `p`/`d`, history `x`/`r`) keep their guard branch (ADR-3); text states bind only Enter/Esc/Backspace and append `msg.Runes` on `tea.KeyRunes` (ADR-6). Run the whole suite green, including unmodified resume, esc/q matrix and boundary tests.
- [x] 3.14 Commit U1 (go.mod and go.sum included). Committed as `14da889`. The size decision was made by the user on 2026-11-05 (1,428 accepted, see apply-progress batch 2).

## Phase 4: Size check after U1

- [x] 4.1 Size check (DECIDED 2026-11-05: user accepted 1,428 for U1+U2+U5; U3 and U4 CUT to B2. U1 alone puts the total at 1,428, over the 1,200 ceiling; see apply-progress batch 2): `git diff --numstat 4cc5ea6 -- . ':!openspec'` (adds plus deletes). Record the running total (U2+U5+U1). Compute projected total = measured + 210 (U3) + 95 (U4). If it exceeds 1,200, cut U4 first (mark Phase 6 `[~] CUT`, remove the `commit-selection` delta requirement from specs); if measured + 210 still exceeds 1,200, cut U3 too (mark Phase 5 `[~] CUT`, remove the U3 requirements from `tui-presentation` and `run-history` deltas). Note the decision in the verify report and `state.yaml`. Expected outcome per forecast: both cut (about 1,018 + 210 = 1,228).

## Phase 5: Window size and history windowing (U3) (cut candidate 2)

- [~] (CUT to B2) 5.1 RED (`internal/app/window_test.go`): table for `window(cursor,total,rows)`: rows 0, total <= rows (full range), cursor at top, middle (cursor 15 of 30, rows 10 contains 15), bottom, clamping of out-of-range cursor; range length <= rows and within `[0,total)`. Bottom-anchored: `start = clamp(cursor-rows+1, 0, total-rows)`; moving down advances the window by exactly 1 once past the bottom; moving up may re-anchor. Satisfies [tui-presentation: Window keeps the cursor visible], [Short list is not windowed].
- [~] (CUT to B2) 5.2 RED: `overflow(n, up)` returns `"  ↑ N más\n"` / `"  ↓ N más\n"` and `""` for n = 0. Satisfies [Indicators show hidden counts] (helper half), [No indicator at the edges] (helper half).
- [~] (CUT to B2) 5.3 RED: `truncate(s,width)`: 100 chars width 40 gives display width <= 40 ending `…`; accented Spanish string cut at a boundary is valid UTF-8 with no partial rune; string with `mark()` ANSI escapes keeps escapes intact; width 0 and short rows unchanged. Satisfies [Long row is truncated with an ellipsis], [Multi-byte text is not corrupted], [Width 0 and short rows are unchanged].
- [~] (CUT to B2) 5.4 RED (`internal/app/window_size_test.go`): `Update(tea.WindowSizeMsg{Width:80,Height:24})` in several states sets `width`/`height`, leaves state unchanged, returns nil cmd; a new model has both 0 and `View()` equals the pre-change output. Satisfies [Size is stored], [Default size is unknown].
- [~] (CUT to B2) 5.5 RED (`internal/app/history_window_test.go`): 40 runs, height set, selection 0: at most 10 run rows (column header and chrome excluded), `↓ 30 más`, no `↑`. Satisfies [run-history: Long history fits a 10-row window], [tui-presentation: No indicator at the edges].
- [~] (CUT to B2) 5.6 RED: selection moved down to run 25: run 25 visible, `↑ N más` and `↓ N más` with N matching hidden rows (totals check: visible + N + N = 40). Satisfies [run-history: Window follows the selection], [tui-presentation: Indicators show hidden counts].
- [~] (CUT to B2) 5.7 RED: windowed history with the selection on a resumable run outside the first window: `Enter` starts resume for that run; `x`, `r` and `d` act on the selected row. Satisfies [run-history: Actions apply to the selected row].
- [~] (CUT to B2) 5.8 RED: height 0 with 40 runs: all 40 rows and no indicator; long rows with width set are truncated and the column header, `>` cursor and row format are unchanged for rows that fit. Satisfies [run-history: Height 0 renders every run], [tui-presentation: Height 0 means no windowing].
- [~] (CUT to B2) 5.9 GREEN (`window.go`, `app.go`, `update.go`, `view.go`): `window`, `overflow`, `truncate` (`ansi.Truncate(s,width,"…")`); `width`/`height` fields and `WindowSizeMsg` case returning `m, nil`; history view renders top and bottom first, then `rows = height - lines(banner+top+bottom) - 2`, minimum 3, windows the rows and applies `truncate(row, m.width)`. Run focused tests green.
- [~] (CUT to B2) 5.10 Commit U3.
- [~] (CUT to B2) 5.11 Size check: re-measure `git diff --numstat 4cc5ea6 -- . ':!openspec'`. If measured + 95 (U4) exceeds 1,200, cut Phase 6 (mark `[~] CUT`, remove the `commit-selection` delta).

## Phase 6: Commit-selection windowing and truncation (U4) (cut candidate 1; requires Phase 5)

- [~] (CUT to B2) 6.1 RED (`internal/app/selection_window_test.go`): 50 commits, height set for 10 rows, cursor 0: at most 10 commit rows, `↓ 40 más`, no `↑`. Satisfies [commit-selection: Long commit list fits a 10-row window].
- [~] (CUT to B2) 6.2 RED: cursor moved to commit 30: commit 30 visible with `↑ N más` and `↓ N más` matching hidden counts. Satisfies [commit-selection: Window follows the cursor].
- [~] (CUT to B2) 6.3 RED: `Space` selects a commit, cursor moves so it is outside the window; the selected-count line counts it, and continuing builds the plan including it (stub plan builder receives it). Satisfies [commit-selection: Selection outside the window still counts].
- [~] (CUT to B2) 6.4 RED: commit with a 120-char message and width 60: that row is at most 60 columns and ends with `…`; rows that fit are unchanged (byte-compare with height/width 0 output for short rows). Satisfies [commit-selection: Long row is truncated to the width].
- [~] (CUT to B2) 6.5 RED: 50 commits with long messages and height 0, width 0: every row in full, no indicator. Satisfies [commit-selection: Height and width 0 leave rendering unchanged].
- [~] (CUT to B2) 6.6 GREEN (`view.go`): window the selection rows by `m.cursor` with `overflow` and `truncate`, computing `rows` from the rendered chrome (minimum 3); selection and cursor operate on the full list.
- [~] (CUT to B2) 6.7 Commit U4.

## Phase 7: Final verification

- [x] 7.1 Run `go test ./... && go build ./... && go vet ./...`; all green, with no pre-existing test edited beyond the design's listed changes. Confirm `go list -m github.com/charmbracelet/bubbles` is v0.21.0 and the non-test imports of bubbles are only `key` and `help`. *Done 2026-11-05: verifier reports all commands green and the dependency check passes (see `verify-report.md`). Fixes from `360d0d6` are included. Ticked by the archive step on orchestrator instruction (exceptional reconciliation, recorded in `archive-report.md`).*
- [x] 7.2 Final size check against the ceiling (excluding openspec, measured against 4cc5ea6); cross-check the Scenario Coverage Index below against surviving tests and skipped (cut) units; update `state.yaml` (tasks done, verify next) and record any cut scenarios in the verify report. *Done 2026-11-05: size is 1,428 (approved by the user as the size exception; the 1,200 ceiling is superseded). Cut scenarios (U3 and U4) are recorded in `verify-report.md` and `archive-report.md`.*

## Scenario Coverage Index

- tui-presentation U1: footers/handlers one keymap (4) -> 3.5, 3.6, 3.4; footer grammar (2) -> 3.2, 3.3; structural parity (2) -> 3.4; printable states (2) -> 3.8, 3.9.
- tui-presentation U3 (CUT to B2): window size (2) -> 5.4; windowed lists (5) -> 5.1, 5.2, 5.5, 5.6, 5.8; truncation (3) -> 5.3.
- tui-presentation U5: color (3) -> 2.1-2.3.
- tui-presentation MODIFIED footer parity (2) -> 3.4, 3.7.
- run-history U3 (CUT to B2): (4) -> 5.5-5.8.
- commit-selection U4 (CUT to B2): (5) -> 6.1-6.5.
- branch-cleanup U2: (4) -> 1.1-1.4.
