# Apply Progress: tui-bubbles-adoption (batch 1 + batch 2 + batch 3)

Mode: Strict TDD. Scope: Phase 1 (U2), Phase 2 (U5). Task 3.1 gate: FAILED at the time (bubbles not a known dependency), stopped.
*Amendment 2026-11-05: commit tasks 1.6 and 2.5 are DONE as `d0ebf0b` (U2) and `16f577f` (U5). The 3.1 gate was later resolved (see batch 2).*

## Tasks
- [x] 1.1-1.5 U2 wrong-word notice
- [x] 1.6 commit U2 (`d0ebf0b`)
- [x] 2.1-2.4 U5 AdaptiveColor
- [x] 2.5 commit U5 (`16f577f`)
- [x] 3.1 GATE: *resolved 2026-11-05.* Was "not a known dependency"; bubbles v0.21.0 was then resolved from the module cache (see batch 2).

## TDD Cycle Evidence

| Task | RED (test, failure) | GREEN | Triangulation | Safety net |
|------|---------------------|-------|---------------|------------|
| 1.1 wrong word notice | `TestBorrar_WrongWordRendersNotice`: view lacked notice for Succeeded/Aborted (4 subtests failed) | `view.go`: `m.noticeLine()` in the `cleanupStrongConfirm` branch of `deleteConfirmView` | 3 entry points x 2 words (borrar, BORR) | full `internal/app` suite green before and after |
| 1.2 exact word | `TestBorrar_ExactWordDeletesWithoutNotice` (pin, passed at once) | unchanged | asserts `branch -D` ran | same |
| 1.3 notice clears | `TestBorrar_NoticeClears/cleanup_list/rune`: typing a rune left `cleanupNotice` set | `keys.go`: clear `notice`/`cleanupNotice` on rune, `q` append and backspace in both handlers | rune and esc, 3 entry points | same |
| 1.4 both entry points | covered by 1.1 table (Succeeded, Aborted, cleanup list); list entry already rendered `cleanupNotice` | n/a | n/a | same |
| 2.1 dark bright palette | `TestMark_AdaptiveColor` dark rows (pin, passed); `TestMain` pins dark | n/a | TrueColor and ANSI256 | `TestMark_NonAsciiUsesBrightColors` unmodified |
| 2.2 light darker | `TestMark_AdaptiveColor` light rows: 6 subtests failed (still 92/93/91) | `style.go`: AdaptiveColor Light 2/3/1, Dark 10/11/9 | 3 tokens x 2 profiles | same |
| 2.3 ascii plain | `TestMark_AsciiPlainOnBothBackgrounds` (pin, passed) | n/a | both backgrounds, 4 tokens | same |

## Files changed
- internal/app/view.go, keys.go, style.go (prod); color_test.go (modified); borrar_notice_test.go (new)

## Size (vs 4cc5ea6, excl. openspec)
Tracked numstat adds+deletes: 67; untracked borrar_notice_test.go: 93 lines; total 160 of 1,200.

## Verification
gofmt -l clean; go vet clean; go test ./... -short -race all ok; boundary_test.go unchanged.

---

# Batch 2: U1 (Phase 3) and size check 4.1

Mode: Strict TDD. Scope: 3.1-3.13 done, 3.14 commit left open (no commits by instruction). 4.1 measured; STOPPED per the "U1 alone over 1,200" rule. U3/U4 untouched (no spec edits, no CUT marks) pending the user's decision.

## Tasks
- [x] 3.1 gate: `GOPROXY=off GOFLAGS=-mod=mod go get github.com/charmbracelet/bubbles@v0.21.0` then `go mod tidy` resolved from the cache. go.mod +1 line (bubbles v0.21.0), go.sum +2 lines. bubbletea v1.3.10 and lipgloss v1.1.0 unchanged. `x/ansi` stays indirect (only U3 needs it direct).
- [x] 3.2-3.10 RED tests; [x] 3.11-3.13 GREEN (keymap.go, view.go, keys.go).
- [x] 3.14 commit U1 (`14da889`, go.mod and go.sum included)
- [x] 4.1 size check: measured at 1,428. *DECIDED 2026-11-05: the user accepted 1,428. U3 and U4 are CUT to B2.*
- [~] U3 (phase 5) and U4 (phase 6): CUT to B2 (user decision 2026-11-05). Not implemented.

## TDD Cycle Evidence

| Task | RED | GREEN | Triangulation | Safety net |
|------|-----|-------|---------------|------------|
| 3.2 vocabulary | `keymap_test.go` (`hint`, `footerOf`): build failure, `undefined: keys` | `keymap.go` `keys`, `hint`, `footerOf` (zero-style `help.Model`, help-less bindings skipped) | 7 footerOf cases: none, all disabled, help-less, one, 3-space separator, leading/middle disabled | n/a (new) |
| 3.3 byte pins | `footer_pins_test.go`: 58 footers, passed against the old code (baseline), kept green after GREEN | `bindings()` mirrors `viewBody`; all literals removed | every design pin plus AI/auth/cleanup/push sub-phases | full app suite |
| 3.4 structural parity | `TestFooterParity_Structural`: build failure (`m.bindings` undefined), then 4 real failures: conflict footer kept `c` while disabled (2 fixtures), empty selection `Space/x` acted with no rows | `bindings()`, footers via `footerLine()`, selection Mark guarded by `len(m.items) > 0` | coverage loop over every State, 55 fixtures, universe of keys from all bindings | full app suite |
| 3.5 conflict c | `TestKeymap_ConflictContinueFollowsTheGate`: footer still listed `c` | `hint(keys.Continue, ..., m.continueEnabled)`; handler keeps its notice-only guard (ADR-3) | gate off and on | same |
| 3.6 push-ready a / Succeeded d | `TestKeymap_DisabledByModelStateIsNeitherShownNorActed` (passes already, pin) | n/a | AI off, Succeeded/Aborted without branch | same |
| 3.7 in-flight/polling | `TestKeymap_InFlightFooters` (pin, passed) | n/a | 3 in-flight states, polling c | same |
| 3.8/3.9 printable and guarded q | `TestKeymap_PrintableKeysReachTheBuffer`, `TestKeymap_QOnEmptyBufferActsAsEscNeverQuits` (pins, passed) | n/a | 7 text states (ticket, package path, CANCELAR, DESPLEGAR, BORRAR x3) | existing `TestEscQ_TypedStatesNeverQuitOnQ` unmodified |
| 3.10 no `footer("` | `TestFooterSource_NoLiteralFooters`: 61 literals found | all 61 call sites migrated to `m.footerLine()` | n/a | same |

Notes: `TestFooterParity_SubPhaseKeys` removed from `subphase_test.go`; its fixtures (`subPhaseFixtures`) and `acts()` stay in `subphase_test.go` and are reused by the structural test (not moved, to save lines). `aiFooterPart`, `runHistoryFooter` and `abortedFooter` removed. `PinnedFragments` and `EscAndQMatchHandlers` kept.

## Size measurement (vs 4cc5ea6, excl. openspec)
- Tracked `git diff --numstat`: 571 adds + 321 deletes = 892.
- Untracked: footer_pins_test 117, footer_source_test 18, keymap.go 232, keymap_states_test 125, keymap_test 44 = 536.
- Total 1,428 (batch 1 was 160, so U1 added about 1,268 against a 950 forecast). Over the 1,200 ceiling by 228 with U1 alone.
- Projection with U3 (210) + U4 (95): about 1,733. The cut rule would cut both, but the stop rule applies first: the user decides (raise the ceiling, or trim).

## Verification
gofmt -l clean; go vet clean; go build ok; `go test ./... -short -race` all ok; `go test ./internal/app/... -count=1` ok; boundary_test.go unchanged.

---

# Batch 3: verify fixes (commit `360d0d6`)

Scope: fixes from verify round 1 under TDD. Verdict is PASS WITH WARNINGS (see `verify-report.md`).

| Finding | Fix | Test |
|---------|-----|------|
| C1 (CRITICAL): queue-review error footer said `r refrescar` instead of `r reintentar` | Footer corrected and pinned as `queue failed` row in `footer_pins_test.go` | pin row `queue failed` |
| W3: on an empty cleanup list, `p` opened an invisible prune confirmation; `y` pruned run dirs | Prompt and its y/n footer now render | `TestCleanup_EmptyListPruneConfirmIsVisible` |

Full suite green (verifier report). Size: the fix adds a few lines on top of 1,428 (not re-measured).
