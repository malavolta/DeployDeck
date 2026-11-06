# Design: TUI UX Consistency (change A)

## Technical Approach

The change touches only behavior and copy in `internal/app`, plus display strings in `internal/git`. It adds no dependency and no new abstraction. Each view keeps its `footer("...")` call, with one format: `<keys> <label>` segments separated by three spaces, keys joined with `/`. One table-driven test parses the rendered footer for every state and checks it against `handleKey`. Change B replaces this with `key.Binding`. The esc/q rule from the proposal is applied handler by handler. `quitCmd()`, `resumeInto` and the typed-word gates are unchanged.

## Architecture Decisions

| # | Decision | Choice | Rejected | Rationale |
|---|---|---|---|---|
| ADR-1 | Footer source | Inline footer strings, normalized format. The test parses `[ ... ]` from `View()` | A per-state `[]keyHint` table | A table means about 40 refactored sites that B throws away. Parsing `View()` checks what the user actually sees. |
| ADR-2 | What "acts" means | A key acts when the state changes, the returned cmd is non-nil, or `View()` changes once `notice` is cleared on both models | `reflect.DeepEqual(Model)` | Model holds funcs and contexts, so DeepEqual does not work. A key whose only effect is a notice explains why it is unavailable. It is not an action, so the conditional footers (history `x`/`r`, push `p`) stay honest. This matches the spec's "recognized but cannot act". |
| ADR-3 | esc/q | Edit each handler in place, following the proposal table | A central esc/q dispatcher in `handleKey` | Sub-phases (push, cleanup, delete confirm) and guarded-q text screens need local context. A dispatcher would duplicate those phase switches. |
| ADR-4 | Conflict cursor | New field `conflictIdx`, reset to 0 at both entry sites (`update.go:452` resume, `:815` pick). `conflictCursor()` clamps it | Reusing `m.cursor`; resetting in the view | The bug is the shared field. Clamping covers `Unmerged` shrinking when `repoStateCmd` refreshes. |
| ADR-5 | Cleanup entry | Extract `enterBranchCleanup()`, used by the menu entry and by ticket `b`. esc returns to `StateMainMenu` from both | Keeping the ticket return for `b` | One back target. The proposal says cleanup goes back to the menu. |
| ADR-6 | Eligibility reasons | App-side map `quickDeployReasonES` keyed by the English reason from `runs`. Unknown reasons fall back to a generic Spanish notice (never raw English text) | Spanish strings in `runs`; a typed reason enum | The reasons are pinned in `runs/quick_deploy_test.go` and `app/quick_deploy_test.go:865`. An enum changes the `runs` API and its 6 callers. A drift test covers the map. |
| ADR-7 | Notice styling | `noticeLine()` = `"\n  " + styleWarn.Render(notice) + "\n"`, used only at new sites (history, validation result, Aborted) | Migrating all 14 inline sites | Under the Ascii test baseline the output is identical. Migrating every site is B's restyle, and it would add about 40 diff lines. |
| ADR-8 | Column width | REPLACED by a targeted fix: fixed widths raised to fit header and values (cleanup Antigüedad 10, Estado 8; run-history Destino 10; queue Tipo 10). The computed `%-*s` helper was dropped | A rune-width helper over every table | fmt pads by rune count, so the fixed widths line up. A helper adds code for no extra coverage. Tested in `subphase_test.go`. |
| ADR-9 | Conflict labels | CUT. `ConflictKind.Label()` is not implemented; conflict kinds stay as is and are out of this change (follow-up) | n/a | Cut under the size budget. Its scenarios were removed from the specs. |
| ADR-10 | Delta `e` during generation | `e` acts only when `deltaErr != nil` | Advertising `e` while the spinner runs | The handler comment already scopes `e` to the failure screen. Editing mid-generation races `deltaCmd`. |

## esc/q Per State (implements the proposal rule)

| State / phase | esc | q | Footer delta |
|---|---|---|---|
| Prereq, MainMenu | quit | quit | `q/Esc salir` |
| DeltaSource, Sandbox, RunHistory, GateBlocked, Cleanup browse | back (menu / PackageSelect / menu / history / menu) | `quitCmd()` (was back) | `Esc volver   q salir` |
| SourceConfirm, Selection, Target, PlanPreview, QueueReview, BranchCollision | back (unchanged) | `quitCmd()` (new) | `+ q salir`. Selection: `↑/↓ navegar   Space/x marcar` |
| Conflict, Verification, Delta, PackageReview, ValidationStart, pushReady | none | `quitCmd()` | Conflict: `↑/↓ elegir   e editor ... q salir (el run queda reanudable)` |
| ValidationPolling | none | `quitCmd()` | `r refrescar   c cancelar mi job   q salir (el job sigue activo)` |
| pushConfirm | `StateSucceeded` (was quit) | `quitCmd()` | `Esc volver` |
| Discovery, BranchCreation, CherryPicking, cleanup loading | none | none | `ctrl+c interrumpe` |
| Succeeded, Aborted, Failed, Canceled, Error | quit | quit (Enter too) | `d borrar rama` on Succeeded/Aborted only when a branch exists (Aborted gains the same empty-branch gate and notice). `Enter/q/Esc salir` |
| Ticket, PackagePath, CANCELAR, DESPLEGAR, BORRAR (both) | back (ticket → menu) | typed when the buffer has text, else same as esc | BORRAR gains guarded q |
| y/n (cleanup confirm/prune, PR confirm, delete confirm) | back | back (cancel; orchestrator amendment 2026-11-05) | `y confirmar   n/Esc/q cancelar` |

## Data Flow

    tea.KeyMsg ─→ Update (ctrl+c: tea.Quit, unchanged)
                └→ handleKey ─→ key<State> ─→ state / notice / cmd
    View() ─→ screen + footer("<k> <label>   ...")
    footer_parity_test: fixture(State) ─ parse footer ─→ send each key ─→ assert acts
                                       └ probe set (a-z, enter, esc, up, down, space) ─→ acts ⇒ advertised

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/app/keys.go` | Modify | esc/q per table, conflict ↑/↓/k/j, notices, `enterBranchCleanup`, guarded BORRAR q, delta `e` gate |
| `internal/app/app.go` | Modify | `conflictIdx`, "Limpieza de ramas" menu entry (last), `quickDeployReasonES` |
| `internal/app/view.go` | Modify | Footers, `>` conflict marker, `noticeLine`, Spanish copy, rune-sized columns, BORRAR names the branch |
| `internal/app/update.go` | Modify | Conflict cursor reset at 2 sites. Notices at :570, :970 in Spanish |
| `internal/git/conflict.go`, `continue_gate.go` | Modify | `Label()`, Spanish Pending text |
| `internal/app/footer_parity_test.go` | Create | Fixture table: parity and esc/q matrix |
| `internal/app/*_test.go` | Modify | Pins (`run_history_menu_test` last entry becomes cleanup, history/cleanup/ticket back targets) |

## Copy Inventory (Spanish)

Notices: keys.go:550, 562, 868, 908, 1053, 1088, 1124, 1208; update.go:570, 970. New: `p`/`d` with no branch ("este run no tiene rama de promoción"), `x` ("quick deploy no disponible: <reason>"), `r` ("re-promoción disponible solo para runs exitosos con commits"), standalone delta Enter ("modo delta: no hay validación; pulsa q para salir"). Views: "Cambios destructivos", "título:", "pusheada/local", "validando", BORRAR prompt "La rama %s tiene commits sin pushear...". Git: Pending text and kind labels. The spec phase owns the exact wording.

## Testing Strategy (strict TDD, RED first)

| Seam | Test |
|---|---|
| Footer parity | Each fixture (every State plus push, cleanup and delete sub-phases, Succeeded with and without a branch, AI configured): every advertised token acts (for multi-key tokens, at least one key); every acting probe key is advertised (aliases k/j, N/S/O/E; runes exempt on text states) |
| esc/q matrix | Same fixtures with `esc`/`q` columns: Quit (`Deps.Git` nil, so the cmd yields `tea.QuitMsg`), Back(State), None |
| Conflict | 3 files, ↓↓ then `e`: the stub `Edit` gets file 3. The cursor is 0 after a resume entry and after a pick entry |
| Menu | Cleanup is last and history is right before it. Enter runs `enterBranchCleanup` even with a pre-filled ticket. esc returns to the menu |
| Notices/reasons | Every `runs` reason maps to a different Spanish string. `x`/`r`/`p`/`d`/Enter notices appear in `View()` |
| Columns/BORRAR | Header and row columns line up by rune offset. The prompt contains the branch name. Guarded q |

Existing resume, `quitCmd`, ctrl+c and `boundary_test.go` tests stay green, unmodified.

## Threat Matrix

N/A. No new shell, subprocess, Git or PR call. `quitCmd()` is unchanged and now reachable from more pre-branch states, where its `current != original` and `InProgress` guards make it a no-op. pushConfirm esc no longer quits and never pushes.

## Migration / Rollout

No migration required. UI only. Revert the PR.

## Budget

Original forecast was about 790 lines. Actual after the corrective round is about 1505 (production about 500 including the delete prompt and column fixes, tests about 980), against the user-approved exceptions of 970 and then 1150. ADR-9 stays cut; ADR-8 is replaced by the targeted width fix above.

## Notices (W5)

A notice clears on the next key press and on navigation (`notice` is reset at the top of key handling and by state changes). Decision: this also applies to async notices (for example the queue-permission notice in `update.go`), which are shown once and vanish on the next key. They are informational, the user can always trigger the action again, and persisting them would need a per-notice lifetime that this change does not add. The inline delete confirmation (Succeeded/Aborted) now renders its own prompt and footer, so the notice line is not used there.

## Footer casing (W4)

One form: `Esc` (capitalised) in every footer and in the spec fragments; `ctrl+c` stays lowercase. Footer parity lowercases tokens when comparing.

## Open Questions

- [ ] User review (from the proposal): q in pickers/history now quits; push esc only before the push.
- [x] **Spec conflict (resolved by the orchestrator, 2026-11-05: branch-cleanup spec amended; esc/q on a confirm returns to the list).** The first `branch-cleanup` requirement says esc on the "confirm" views returns to the menu, but its BORRAR scenario returns to the list. The design keeps esc in any confirm (y/n, BORRAR, counting) going back to the list, and only esc on the list going to the menu. The spec should be amended to say this.
- [ ] Ticket `b` stays unadvertised (text-input exemption). The menu entry is the discoverable path.


> **Size note (2026-11-05, final):** 1505 changed lines (excl. openspec) under a user-approved size exception (800 → 970 → 1150 → 1505). Verify round 2: PASS WITH WARNINGS.
