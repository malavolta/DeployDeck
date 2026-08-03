# Tasks: TUI UX/UI Polish

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~630-960 (design estimate, incl. RED tests) — likely upper-half/above: exploration found 24 literal `[OK]/[!!]/[XX]/[i]` mark sites vs design's "~10" estimate, plus 2 extra un-accented Yes/No prompts and 2 extra footer-exit-affordance gaps not enumerated in design |
| 400-line budget risk | High |
| 800-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR, 5 sequential internal commits (design's own framing: "5-slice internal commit order") |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High
800-line budget risk: High

Rationale: the design explicitly organizes the change as ordered *commits within one PR*, not independent PRs — slice 1 (color infra) is a hard prerequisite for slices 2-5 (mark() call sites), and slices 2-5 all touch overlapping regions of `internal/app/view.go` (the ~25 `header()`/footer sites), so splitting into separately-reviewable PRs would create constant rebase churn without reducing the reviewer's real cognitive surface (same file, same screens). Given the fixed `single-pr` delivery strategy, the correct guard resolution is an explicit `size:exception` sign-off before `sdd-apply`, not a chain. The 5 commits below stay the rollback/verification unit even inside one PR.

### Suggested Work Units (sequential commits inside the single PR)

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|------|------|----------------------|-----------------|-------------------|
| 1 | Color infra + Ascii `TestMain` + `go mod tidy` | `go test ./internal/app/... -run TestMark` | N/A (pure render, no exec/network boundary per Threat Matrix); full-suite `go test ./internal/app/... -race` is the closest harness | Revert `style.go`, `color_test.go`, `go.mod`/`go.sum`, and the `mark()` call-site edits in `view.go` |
| 2 | Footer standardization + context bar (`screenHeader`/`contextBar`) + guarded-`q` | `go test ./internal/app/... -run "ContextBar\|GuardedQ"` | `go test ./internal/app/... -run TestFlow` (existing e2e suites double as the render-regression harness) | Revert `contextBar`/`screenHeader` + their `view.go` call-site routing, and the 3 guarded-`q` cases in `keys.go` |
| 3 | Empty states + column headers + menu descriptions | `go test ./internal/app/... -run "ColumnHeaders\|EmptyState\|MenuEntries"` | Same existing e2e suites (`internal/app/*_e2e_test.go`) | Revert the 3 header-row insertions, the `viewSelection` footer switch, and `menuEntry.description` |
| 4 | Spanish accent/`¿¡` copy sweep | `go test ./internal/app/...` (verbatim assertions are the test) | N/A — copy-only, no behavior | Revert the string literal edits; no structural change |
| 5 | Progress spinner | `go test ./internal/app/... -run Spinner` | `go test ./internal/app/... -run TestFlow` | Revert `spinnerCmd`/`spinnerTickMsg`/`onSpinnerTick`/`spinnerView` and the 5 `tea.Batch` entry-site edits |

## Phase 1: Color Infrastructure + Ascii Safety Net (Slice 1)

- [x] 1.1 GREEN (infra): create `internal/app/color_test.go` with the package's sole `func TestMain(m *testing.M)` → `lipgloss.SetColorProfile(termenv.Ascii); os.Exit(m.Run())`. Run `go test ./internal/app/... -race`: all pre-existing suites (115 `strings.Contains` assertions) stay green, byte-identical.
- [x] 1.2 GREEN (infra): run `go mod tidy` — promotes `github.com/charmbracelet/lipgloss` and `github.com/muesli/termenv` from `// indirect` to direct in `go.mod`/`go.sum` (`colorprofile` stays indirect — nothing in the package imports it directly; only lipgloss (style.go) and termenv (color_test.go/style_test.go) are direct imports, so `go mod tidy` correctly promotes only those two).
- [x] 1.3 RED: add `internal/app/style_test.go` with `TestMark_AsciiProfileIsPlain` asserting `mark("OK")=="[OK]"`, `mark("!!")=="[!!]"`, `mark("XX")=="[XX]"`, `mark("i")=="[i]"`, none containing `"\x1b"`. Compile fails (`mark` undefined) — confirms RED.
- [x] 1.4 RED: same file, `TestMark_NonAsciiProfileAppliesEscape` — set `lipgloss.SetColorProfile(termenv.TrueColor)`, call `mark("XX")`, assert `strings.Contains(got, "\x1b[")`, `defer lipgloss.SetColorProfile(termenv.Ascii)` to restore the TestMain baseline. Confirms RED.
- [x] 1.5 GREEN: create `internal/app/style.go` — `styleOK`/`styleWarn`/`styleErr`/`styleBold`/`styleDim` via `lipgloss.NewStyle()`; `mark(tok string) string` returns `"["+tok+"]"` through `styleOK`(`OK`)/`styleWarn`(`!!`)/`styleErr`(`XX`)/`styleDim`(`i`,`--`)/plain default. Run `go test ./internal/app/... -run TestMark` — 1.3/1.4 GREEN.
- [x] 1.6 GREEN: route `internal/app/view.go`'s status-token sites through `mark()` — `statusMark`/`selectionMark`/`conflictMark` call sites (`viewPrereq`, `viewSelection`, `viewConflict`) plus the 20 literal `[OK]`/`[!!]`/`[XX]`/`[i]` `WriteString` sites across `viewDeltaGeneration`, `viewPackageReview`, `viewQueueReview`, `viewValidationStart`, `viewCancelConfirm`, `viewQuickDeploy`, `viewValidationResult`, `viewRunHistory`, `viewVerification` (no literal bracket-token site exists in `viewBranchCleanup` — grep-verified). Every `strings.Contains("[XX]"/"[OK]"/...)` assertion stays byte-identical under the TestMain Ascii profile.
- [x] 1.7 Verification: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN, all packages pass.

## Phase 2: Footer Standardization + Context Bar (Slice 2)

- [x] 2.1 RED: add `internal/app/view_test.go` `TestContextBar_RepoAndBranchNoOrg` — Model with `deps.Dir` set, `m.originalBranch="feature/X"`, `m.plan.SandboxAlias=""`; assert `m.contextBar()` (direct unit test — call-site routing into `View()` is task 2.4) equals `"  "+filepath.Base(dir)+" · feature/X\n"` with no trailing org segment. Confirms RED (`contextBar` undefined, compile failure).
- [x] 2.2 RED: same file, `TestContextBar_AppendsOrgOnceResolved` — `m.plan.SandboxAlias="myorg"`; assert `m.contextBar()` Contains `"· feature/X · myorg"`. Also added `TestScreenHeader_ComposesHeaderAndContextBar` (companion, `screenHeader` undefined too).
- [x] 2.3 GREEN: implement `func (m Model) contextBar() string` (`"  "+filepath.Base(m.deps.Dir)+" · "+m.originalBranch[+" · "+m.plan.SandboxAlias if set]+"\n"`) and `func (m Model) screenHeader(title string) string` = `header(title)+m.contextBar()` in `view.go`. Run tests — 2.1/2.2 GREEN.
- [x] 2.4 GREEN: routed all 24 `b.WriteString(header("..."))` sites plus the 4 inline `viewBody` branches (`StateCommitDiscovery`, `StateBranchCreation`, `StateAborted`, `StateError`) — 28 total — through `m.screenHeader("...")`. Full suite stayed green with zero test adjustments: no existing multi-line `Contains` assertion was split by the inserted context-bar line.
- [x] 2.5 RED: extend `internal/app/keys_test.go` — `TestKeyTicket_GuardedQ`: non-empty `m.ticket`+`"q"` appends `"q"` (already true today, passed); empty `m.ticket`+`"q"` currently appends `"q"` into the buffer (bug) — asserted it instead transitions to `StatePrereqCheck` like `"esc"` (failed as expected: RED confirmed, state stayed `StateTicketInput`).
- [x] 2.6 RED: `TestKeyCancelConfirm_GuardedQ` — non-empty `m.cancelInput`+`"q"` appends (passed); empty `m.cancelInput`+`"q"` currently appends too (bug) — asserted it instead mirrors `"esc"` (clears buffer, `state=StateValidationPolling`, returns the re-arming `pollTickCmd(...)`; RED confirmed, state stayed `StateCancelConfirm`).
- [x] 2.7 RED: `TestKeyQuickDeploy_GuardedQ` — non-empty `m.quickConfirm`+`"q"` currently backs out immediately (bug: `case "q", "esc":` was unconditional) — asserted it instead appends `"q"` and stays on `StateQuickDeploy` (RED confirmed: buffer stayed empty, back-out fired); empty buffer+`"q"` still backs out to `StateRunHistory` (passed).
- [x] 2.8 GREEN: in `keys.go`, gave `keyTicket` a dedicated guarded `case "q":`, split `keyCancelConfirm`'s missing `"q"` case the same way (mirroring its `"esc"` branch incl. `pollTickCmd` re-arm), and split `keyQuickDeploy`'s combined `case "q", "esc":` into a guarded `"q"` plus the existing unconditional `"esc"` — mirrors `keyPackageSelect:638`'s established pattern. `go test ./internal/app/... -run GuardedQ` — 2.5-2.7 GREEN. Updated the one pre-existing test that captured the old buggy behavior (`TestKeyQuickDeploy_QOrEsc_ReturnsToRunHistoryAndClearsBuffer` typed "DESP" then pressed "q" expecting back-out) — split into `TestKeyQuickDeploy_Esc_ReturnsToRunHistoryAndClearsBuffer` (esc-only, unconditional) since "q"'s new guarded behavior is covered by `TestKeyQuickDeploy_GuardedQ`.
- [x] 2.9 GREEN: audited every `footer(...)` call in `view.go` (48 sites); the two confirmed gaps — `viewBranchCleanup`'s `cleanupConfirm`/`cleanupPruneConfirm` y/n footers (`"y confirmar   n cancelar"`) — now read `"y confirmar   n/Esc cancelar"` (no test asserted the old literal text; `esc` already behaved identically to `n` in `keyBranchCleanup`, so this is copy-only, no behavior change).
- [x] 2.10 Verification: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN, all packages pass.

## Phase 3: Empty States + Column Headers + Menu Descriptions (Slice 3)

- [x] 3.1 RED: `internal/app/view_test.go` `TestViewSelection_EmptyState_OmitsSpaceMarcar` — `m.items=nil`, `m.discovery.Alternatives` populated; assert `m.viewSelection()` Contains `"Esc volver"` and does NOT Contain `"Space marcar"`. Failed as predicted against the unconditional footer — RED confirmed.
- [x] 3.2 GREEN: switched `viewSelection`'s footer on `len(m.items)==0` (empty → `"Esc volver"` only; non-empty → unchanged), mirroring the existing `viewRunHistory` empty/populated footer-switch precedent already in the codebase.
- [x] 3.3 RED: `TestViewRunHistory_ColumnHeaders` — 1 `m.runs` entry; asserted a header row (`Fecha`/`Ticket`/`Destino`/`Estado`/`Job Id`) precedes the data row, aligned to the existing `%-16s %-16s %-6s %-12s` row format. RED confirmed (no header row existed).
- [x] 3.4 RED: `TestViewQueueReview_ColumnHeaders` — header row (`#`/`Job Id`/`Estado`/`Tipo`/`Creado por`/`Tiempo`/`Componentes`/`Tests`) aligned to the existing `%-10s %-12s %-8s %-16s %4s` row format. RED confirmed.
- [x] 3.5 RED: `TestViewBranchCleanup_ColumnHeaders` — header row (`Rama`/`Antiguedad`/`Estado`/`Merge`) aligned to the existing `%-30s %-6s %-7s` row format. RED confirmed.
- [x] 3.6 GREEN: added one header `WriteString` immediately before each of the 3 loops (`viewRunHistory`, `viewQueueReview`, `viewBranchCleanup`), reusing the SAME `%-Ns` width specifiers as the row format directly below. `go test ./internal/app/... -run ColumnHeaders` — 3.3-3.5 GREEN.
- [x] 3.7 RED: `TestViewMainMenu_EntriesShowDescriptions` — asserted `m.viewMainMenu()` Contains a one-line description for each of the 3 `menuEntries`, alongside its existing label. RED confirmed (`e.description` undefined, compile failure).
- [x] 3.8 GREEN: added `description string` to `menuEntry` (`app.go`), populated the 3 `menuEntries`, rendered it in `viewMainMenu` under/after each label. Test — 3.7 GREEN.
- [x] 3.9 GREEN (copy, no new test): unified the divergent source-branch term — `viewSelection`'s `"Rama sugerida: %s"` renamed to `"Rama origen: %s"` to match `viewSourceConfirm`'s reference title/phrasing (`"Confirmar Rama Origen"`, design D4) — the two screens that both reference the discovered `m.source.Name`. No existing test asserted the old literal string (grep-verified), so no test updates were needed.
- [x] 3.10 Verification: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN, all packages pass.

## Phase 4: Spanish Accent / `¿¡` Copy Sweep (Slice 4)

- [x] 4.1 GREEN (copy, no new test): fixed `viewBranchCleanup`'s two un-accented Yes/No prompts — `"Borrar la rama %s?\n"` → `"¿Borrar la rama %s?\n"`; `"Aplicar retencion de runs (...)?\n"` → `"¿Aplicar retención de runs (...)?\n"` — matching `viewSourceConfirm`'s reference phrasing. No test asserted the old literal text (grep-verified), so no assertion updates were needed here.
- [x] 4.2 GREEN (copy, no new test): swept `view.go`, `keys.go`, and `update.go`. Fixed missing accents (`-ción` words, `falló`/`alcanzó`/`vacío`/`también`/`está`/`código`/`título`/`descripción`/`retención`/`Posición`/`selección`/`promoción`, etc.), 2 more missing `¿` on Yes/No prompts (`"¿Crear la PR con la sugerencia IA?"`, `"¿Confirmar creación del PR con gh?"` in `viewPRData`), and the unified "Verificación De Promoción" title. Updated 4 pre-existing verbatim assertions in lockstep in the SAME commit: `push_preparation_ai_test.go` (`"Confirmar creacion..."` → `"...creación..."`, 4 occurrences; `"d titulo default"` → `"d título default"`), `standalone_modes_e2e_test.go` and `standalone_delta_test.go` (`"Package vacio"` → `"Package vacío"`). `keys.go`'s Spanish notices were mostly already correct except `cancelación`/`configuración` (fixed); most other `m.notice` strings in `keys.go` are English, not Spanish, so needed no change. `update.go`'s 2 `retención` notices fixed.
- [x] 4.3 GREEN (copy, no new test): the validation-result title now reads exactly `"Resultado De Validación"` at its one render site in `viewValidationResult` (fixed as part of 4.2's sweep; no test asserted the old string).
- [x] 4.4 Verification: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN, all packages pass.

## Phase 5: Progress Spinner (Slice 5)

- [x] 5.1 RED: `internal/app/commands_test.go` `TestSpinnerCmd_SchedulesTick` — `spinnerCmd()` returns a non-nil `tea.Cmd` whose invocation yields a `spinnerTickMsg` (mirrors the existing `tickCmd`/`tickMsg` idiom). RED confirmed (`spinnerCmd` undefined, compile failure).
- [x] 5.2 RED: `internal/app/update_test.go` `TestOnSpinnerTick_BumpsFrameWhileInSpinnerState` — Model in `StateDeltaGeneration`, `m.spinnerFrame=0` → after `onSpinnerTick()`, `m.spinnerFrame==1` and returned `tea.Cmd` non-nil; Model in `StateMainMenu` (not a spinner state) → `onSpinnerTick()` returns nil `tea.Cmd`. RED confirmed (`m.spinnerFrame`/`m.onSpinnerTick` undefined).
- [x] 5.3 RED: `internal/app/view_test.go` `TestViewDeltaGeneration_SpinnerFrameAppended` — `m.spinnerFrame=0` → view Contains the frame-0 char (`|`) appended AFTER the existing static text; frame `1` → Contains `/` instead, without breaking the existing static-text `Contains` assertion. RED confirmed.
- [x] 5.4 GREEN: in `commands.go`, added `type spinnerTickMsg struct{}`, `const spinnerInterval = 120 * time.Millisecond`, and `func spinnerCmd() tea.Cmd { return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return spinnerTickMsg{} }) }` (mirrors `tickCmd`/`tickMsg`).
- [x] 5.5 GREEN: added `spinnerFrame int` to `Model` (`app.go`) and `func spinnerView(f int) string` cycling `{'|','/','-','\\'}` (via `spinnerFrames [4]rune`) in `view.go`.
- [x] 5.6 GREEN: in `update.go`, added `case spinnerTickMsg: return m.onSpinnerTick()` to `Update`'s switch; implemented `isSpinnerState(state State) bool` + `onSpinnerTick()`: `m.spinnerFrame++`, return `spinnerCmd()` while `m.state` ∈ `{StateCommitDiscovery, StateBranchCreation, StateCherryPicking, StateDeltaGeneration, StateValidationStart}`, else `nil` (mirrors `onTick`). `go test ./internal/app/... -run Spinner` — 5.1/5.2 GREEN.
- [x] 5.7 GREEN: appended `spinnerView(m.spinnerFrame)` to the 5 spinner-state views (`viewBody`'s inline `StateCommitDiscovery`/`StateBranchCreation` branches, `viewCherryPicking`, `viewDeltaGeneration`, `viewValidationStart`) — strictly appended after existing static text, only on the non-error branch of `viewDeltaGeneration`/`viewValidationStart` (the error sub-screens show no live progress). Test — 5.3 GREEN.
- [x] 5.8 GREEN: batched `spinnerCmd()` with the existing work command at every state-entry site for the 5 states — `keyTicket`'s `"enter"`→`discoverCmd`; `keyPlanPreview`'s `"enter"`→`branchCreateCmd`; `onBranchCreated`'s `StateCherryPicking` launch; `confirmDeltaSourceSelect`/`keyVerification`'s `"enter"`→`deltaCmd`; the 3 `StateValidationStart` entry sites (`update.go` onQueueDone's permission-skip branch, `confirmSandboxSelect`, `keyQueueReview`'s `"enter"`)→`validateCmd` — via `tea.Batch(existingCmd, spinnerCmd())`. Fixed 1 pre-existing test (`TestModel_SandboxSelect_Confirm_ResetsStalePlanBeforeValidate`) that assumed a single (non-batched) `validateCmd` return — updated it to unwrap the `tea.BatchMsg` and run every sub-command, mirroring `original_branch_test.go`'s established unwrap pattern (`bubbletea`'s `compactCmds` only collapses to a single direct `Cmd` when exactly one of the batched commands is non-nil; with 2 non-nil commands it always returns a real `BatchMsg`).
- [x] 5.9 Verification: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN, all packages pass.
