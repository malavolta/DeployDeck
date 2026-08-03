# Apply Progress: tui-ux-polish

Mode: **Strict TDD**. Artifact store: OpenSpec (file-based). Delivery: single-pr,
`size:exception` accepted (5 sequential-commit phases inside one PR; apply did
NOT git commit — orchestrator handles git).

## Status: DONE — all 5 phases complete, 40/40 tasks

Test runner: `PATH=/usr/local/go/bin:$PATH go test ./... -race`
Final full run (uncached, `-count=1`): **GREEN** — all 13 packages pass.
`go build ./...`: clean. `gofmt -l .`: no output (all formatted). `go vet ./...`:
clean. `TestApp_NeverImportsExecSeam` (boundary invariant): PASS.

## Phase 1: Color Infrastructure + Ascii Safety Net — DONE (7/7)

- `internal/app/color_test.go` (new): package's sole `TestMain` → `lipgloss.SetColorProfile(termenv.Ascii)`.
- `internal/app/style.go` (new): `styleOK/Warn/Err/Bold/Dim`, `mark(tok)`.
- `internal/app/style_test.go` (new): `TestMark_AsciiProfileIsPlain`, `TestMark_NonAsciiProfileAppliesEscape`.
- `internal/app/view.go`: routed `statusMark`/`selectionMark`/`conflictMark` call sites + 20 literal `[OK]/[!!]/[XX]/[i]` `WriteString` sites through `mark()`.
- `go.mod`/`go.sum`: `lipgloss` + `termenv` promoted to direct via `go mod tidy` (`colorprofile` stayed indirect — no direct import anywhere in the package, which is correct `go mod tidy` behavior, not a deviation).

Deviation note: design said "~10" literal sites; tasks.md's own exploration corrected this to "~24"; grep-verified actual count was 20 `WriteString` sites (`viewBranchCleanup` has none). All found sites are routed.

## Phase 2: Footer Standardization + Context Bar — DONE (10/10)

- `internal/app/view.go`: `contextBar()`, `screenHeader()`; 28 header call sites routed (24 `b.WriteString(header(...))` + 4 inline `viewBody` branches); 2 `footer()` copy fixes in `viewBranchCleanup` (`"y confirmar   n/Esc cancelar"`).
- `internal/app/keys.go`: guarded `"q"` added to `keyTicket`, `keyCancelConfirm`, `keyQuickDeploy` (real bugs fixed — empty-buffer `"q"` no longer silently swallowed/misbehaves).
- Tests: `view_test.go` (+3), `keys_test.go` (+3). Updated 1 pre-existing test (`quick_deploy_test.go`) that captured the old buggy unconditional-`"q"` behavior — split into esc-only + covered by the new guarded-q test.
- No existing multi-line `Contains` assertion was split by the inserted context-bar line — zero adjustment needed for that part.

## Phase 3: Empty States + Column Headers + Menu Descriptions — DONE (10/10)

- `internal/app/view.go`: `viewSelection`'s footer switches on `len(m.items)==0` (empty → `"Esc volver"` only); column header rows added to `viewRunHistory`/`viewQueueReview`/`viewBranchCleanup` (reusing the same `%-Ns` width specifiers as their data rows); `viewMainMenu` renders `menuEntry.description`; unified source-branch terminology (`"Rama sugerida"` → `"Rama origen"` in `viewSelection`, matching `viewSourceConfirm`'s reference title).
- `internal/app/app.go`: `menuEntry.description` field, populated for all 3 entries.
- Tests: `view_test.go` (+4), `main_menu_test.go` (+1).

## Phase 4: Spanish Accent / `¿¡` Copy Sweep — DONE (4/4, copy-only, no new tests)

- `internal/app/view.go`: fixed missing accents across `-ción` words, verb past tenses (`falló`/`alcanzó`/`devolvió`), `vacío`/`también`/`está`/`código`/`título`/`descripción`/`retención`/`Posición`/`selección`/`promoción`/`Verificación`; added 2 more missing `¿` prefixes on Yes/No prompts in `viewPRData` (`viewBranchCleanup`'s 2 were fixed in 4.1); validation-result title confirmed exact `"Resultado De Validación"`.
- `internal/app/keys.go` / `update.go`: fixed the handful of actually-Spanish `m.notice`/`m.cleanupNotice` strings missing accents (`cancelación`, `configuración`, `retención` x2); most `m.notice` strings in these two files are English and needed no change.
- Updated 4 pre-existing verbatim-asserting test files in lockstep: `push_preparation_ai_test.go` (creación x4, título x1), `standalone_modes_e2e_test.go` + `standalone_delta_test.go` (`"Package vacio"` → `"Package vacío"`).

## Phase 5: Progress Spinner — DONE (9/9)

- `internal/app/commands.go`: `spinnerTickMsg`, `spinnerInterval` (120ms), `spinnerCmd()` (mirrors `tickCmd`).
- `internal/app/app.go`: `Model.spinnerFrame int`.
- `internal/app/view.go`: `spinnerFrames`/`spinnerView(f)`; appended (never replacing) to the 5 spinner-state views — 2 inline `viewBody` branches, `viewCherryPicking`, `viewDeltaGeneration` (non-error branch only), `viewValidationStart` (non-error branch only).
- `internal/app/update.go`: `isSpinnerState`, `onSpinnerTick` (mirrors `onTick`'s state-scoped reschedule guard); `spinnerTickMsg` wired into `Update`.
- Batched `spinnerCmd()` via `tea.Batch(existingCmd, spinnerCmd())` at all 5 state-entry sites (`keyTicket`, `keyPlanPreview`, `onBranchCreated`, `confirmDeltaSourceSelect`+`keyVerification`, and the 3 `StateValidationStart` entry sites).
- Fixed 1 pre-existing test (`standalone_validate_test.go`) that assumed a single non-batched `validateCmd` return — updated to unwrap `tea.BatchMsg` and run every sub-command (bubbletea's `compactCmds` only collapses to a direct `Cmd` with exactly one non-nil command; with 2 it always returns a real `BatchMsg`), mirroring the established unwrap pattern already used in `original_branch_test.go`.

## TDD Cycle Evidence (all phases)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `color_test.go` | Unit (infra) | N/A (new) | N/A (TestMain) | ✅ 115 pre-existing suites stay green | N/A | ➖ None needed |
| 1.3/1.4 | `style_test.go` | Unit | N/A (new) | ✅ Written (compile fail) | ✅ Passed | ✅ 2 cases | ➖ None needed |
| 1.6 | `view.go` (routing) | Unit | ✅ full suite pre-check | N/A (approval-style: 115 pre-existing asserts = safety net) | ✅ byte-identical | N/A | ➖ None needed |
| 2.1/2.2 | `view_test.go` | Unit | N/A (new) | ✅ Written (compile fail) | ✅ Passed | ✅ 2 cases | ➖ None needed |
| 2.4 | `view.go` (routing) | Unit | ✅ full suite pre-check | N/A | ✅ 0 adjustments | N/A | ➖ None needed |
| 2.5-2.7 | `keys_test.go` | Unit | ✅ full suite pre-check | ✅ Written, ran, failed as predicted | ✅ Passed (1 pre-existing test split) | ✅ 2 cases each | ➖ None needed |
| 2.9 | `view.go` (copy) | N/A | ✅ full suite pre-check | N/A (behavior unchanged) | ✅ full suite green | N/A | ➖ None needed |
| 3.1 | `view_test.go` | Unit | ✅ full suite pre-check | ✅ Written, failed as predicted | ✅ Passed | ➖ Single (empty-state scenario) | ➖ None needed |
| 3.3-3.5 | `view_test.go` | Unit | ✅ full suite pre-check | ✅ Written, failed (no header row) | ✅ Passed | ✅ 3 tables | ➖ None needed |
| 3.7 | `main_menu_test.go` | Unit | ✅ full suite pre-check | ✅ Written (compile fail) | ✅ Passed | ➖ Single (all 3 entries in one loop) | ➖ None needed |
| 3.9 | `view.go` (copy) | N/A | ✅ full suite pre-check | N/A | ✅ full suite green | N/A | ➖ None needed |
| 4.1-4.3 | `view.go`/`keys.go`/`update.go` (copy) | N/A | ✅ full suite pre-check | N/A (pure copy, per strict-tdd copy-task convention) | ✅ full suite green, 4 verbatim tests updated in lockstep | N/A | ➖ None needed |
| 5.1 | `commands_test.go` | Unit | N/A (new) | ✅ Written (compile fail) | ✅ Passed | ➖ Single (mirrors tickCmd, no branching) | ➖ None needed |
| 5.2 | `update_test.go` | Unit | N/A (new) | ✅ Written (compile fail) | ✅ Passed | ✅ 2 cases (in-state / out-of-state) | ➖ None needed |
| 5.3 | `view_test.go` | Unit | N/A (new) | ✅ Written (compile fail) | ✅ Passed | ✅ 2 cases (frame 0 / frame 1) | ➖ None needed |
| 5.8 | `keys.go`/`update.go` (wiring) | Unit | ✅ full suite pre-check | N/A (wiring, no new test) | ✅ Passed (1 pre-existing test fixed: BatchMsg unwrap) | N/A | ➖ None needed |

### Test Summary
- Total new tests written: 16 across the 5 phases.
- Pre-existing tests updated (behavior/copy changed by design, fixed in lockstep): 6
  (`TestKeyQuickDeploy_QOrEsc_...` split; `push_preparation_ai_test.go` x4 assertions;
  `standalone_modes_e2e_test.go` + `standalone_delta_test.go` "Package vacio"→"vacío";
  `standalone_validate_test.go` BatchMsg unwrap).
- All passing. Full suite green (uncached, `-race`) after every phase boundary and at the end.

## Files Changed (all phases, cumulative)

| File | Action |
|------|--------|
| `internal/app/color_test.go` | Created |
| `internal/app/style.go` | Created |
| `internal/app/style_test.go` | Created |
| `internal/app/view.go` | Modified (all 5 phases) |
| `internal/app/keys.go` | Modified (phases 2, 4, 5) |
| `internal/app/update.go` | Modified (phases 4, 5) |
| `internal/app/app.go` | Modified (phases 3, 5) |
| `internal/app/commands.go` | Modified (phase 5) |
| `go.mod` / `go.sum` | Modified (phase 1) |
| `internal/app/view_test.go` | Modified (phases 2, 3, 5) |
| `internal/app/keys_test.go` | Modified (phase 2) |
| `internal/app/main_menu_test.go` | Modified (phase 3) |
| `internal/app/quick_deploy_test.go` | Modified (phase 2) |
| `internal/app/commands_test.go` | Modified (phase 5) |
| `internal/app/update_test.go` | Modified (phase 5) |
| `internal/app/push_preparation_ai_test.go` | Modified (phase 4) |
| `internal/app/standalone_modes_e2e_test.go` | Modified (phase 4) |
| `internal/app/standalone_delta_test.go` | Modified (phase 4) |
| `internal/app/standalone_validate_test.go` | Modified (phase 5) |

## Deviations from Design

1. Phase 1: `colorprofile` stayed `// indirect` (design said "lipgloss+colorprofile" promoted to direct) — only `lipgloss` and `termenv` are ever directly imported, so `go mod tidy` correctly promotes only those two. Not a functional deviation.
2. Phase 2: tasks 2.1/2.2's RED tests call `m.contextBar()`/`m.screenHeader()` directly rather than through `m.View()` — the design's own task 2.3 GREEN gate ("Run tests — 2.1/2.2 GREEN") only implements the two functions, not call-site routing (that's task 2.4), so a `View()`-based assertion could not have gone GREEN at that checkpoint.
3. Phase 1: literal-site count was 20 (grep-verified), not the "~24" the tasks forecast carried over from exploration — no site was left un-routed; the discrepancy is in the estimate, not the coverage.

None of these deviations change behavior or scope — all are covered above with rationale.

## Rollback Boundaries (per phase, mirrors tasks.md's Suggested Work Units table)

1. Revert `style.go`, `color_test.go`, `style_test.go`, `go.mod`/`go.sum`, and the `mark()` call-site edits in `view.go`.
2. Revert `contextBar`/`screenHeader` + their `view.go` call-site routing, and the 3 guarded-`q` cases in `keys.go` (+ the 1 updated `quick_deploy_test.go` test).
3. Revert the 3 header-row insertions, the `viewSelection` footer switch, `menuEntry.description`, and the terminology unification.
4. Revert the string literal accent/¿¡ edits in `view.go`/`keys.go`/`update.go` and their 4 matching test-file updates.
5. Revert `spinnerCmd`/`spinnerTickMsg`/`onSpinnerTick`/`spinnerView`/`isSpinnerState`, the 5 `tea.Batch` entry-site edits, and the 1 updated `standalone_validate_test.go` test.
