# Proposal: TUI UX Consistency (change A)

**Delivery phase**: Fase 5 Productizacion. Change A of two; `tui-bubbles-adoption` (B) follows separately.

## Intent

The TUI says one thing and does another. With more than one conflicted file, `e` opens a file the user never picked: `conflictCursor()` reuses `m.cursor` from commit selection (`internal/app/keys.go:1097-1115`). Several footers do not match their handlers. `q` quits on some screens and goes back on others. Esc on ticket input goes to the prereq screen. Branch cleanup is hidden behind `b`. Some keys do nothing and give no reason. Some copy is in English and some columns are misaligned. Goal: every screen tells the truth about its keys, in Spanish, under one esc/q rule.

## Scope

### In Scope
1. Conflict screen: its own cursor, reset when the screen is entered, with ↑/↓/k/j and a `>` marker. `e` opens the selected file.
2. Footers list exactly the keys that work in that state. A table-driven test pins each footer against its handler. Known gaps: `c` on polling, `d` on Succeeded/Aborted, ↑/↓ and `x` on selection, `a` on push-ready when AI is configured, cleanup's "q salir", and cherry-picking's "q salir cuando termine".
3. The esc/q rule below, applied to every state.
4. A "Limpieza de ramas" main-menu entry after "Historial de runs". The `b` shortcut stays.
5. Spanish notices instead of silent no-ops: `p`/`d` with no branch, `x`/`r` on ineligible rows (the `x` notice gives the `runs.QuickDeployEligible` reason in Spanish), and Enter in standalone delta.
6. Spanish copy for the strings listed in the exploration. Column widths are fixed to fit their headers and values ("Antigüedad", "Destino"); no rune-width helper (ADR-8 replaced by a targeted fix).
7. The BORRAR prompt names the branch.

### Out of Scope
- Everything listed for B: bubbles (no new dependency), `?` help, `WindowSizeMsg`/scrolling, new spinners and elapsed time, adaptive colors, the error-screen redesign, box borders.
- A quit-confirmation dialog.
- Returning to the menu from a terminal screen.
- Changing ctrl+c. It stays a hard interrupt with no restore (`update.go:34`).

## Approach: esc/q rule (decision)

| Category | esc | q | Rationale |
|---|---|---|---|
| Main menu | quit | quit | Top level. There is nothing to go back to. |
| Pickers and sub-screens (delta source, sandbox, history, cleanup, gate-blocked, branch collision, push prep before push) | back to the parent (cleanup goes to the menu, push prep to Succeeded) | `quitCmd()` | One meaning per key. Esc declines the history resume offer. A run declined with q is still offered on the next launch. |
| Main-flow screens (source confirm through push) | back only where nothing has happened yet (source confirm, selection, target, plan, queue); after that, esc does nothing and is not shown | `quitCmd()` | `quitCmd()` already skips restore while a cherry-pick is in progress and leaves the Salesforce job active, so quitting is never destructive. Footers say so ("el run queda reanudable", "el job sigue activo"). No dialog needed. |
| In-flight screens (discovery, branch creation, cherry-picking) | none | none. The footer shows "ctrl+c interrumpe". | Quitting during a git subprocess is unsafe. An honest footer is better than a deferred quit. |
| Terminal results (Succeeded, Aborted, Failed, Canceled, Error) | quit | quit (Enter also quits) | No back target. Keeps the restore and delete behavior of `quitCmd()`. |
| Typed confirmations and text input (ticket, package path, CANCELAR, DESPLEGAR, BORRAR) | cancel and go back (ticket goes to the menu) | typed when the buffer has text, same as esc when it is empty. Never quits. | Keeps the D3 guards and the exact-word gates. |
| y/n confirmations | back | back (cancel, amended 2026-11-05) | Not free text; q never quits from a pending decision |

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `tui-presentation`: footer/handler parity, the esc/q rule, the in-flight exit affordance, Spanish copy, header-sized columns.
- `cherry-pick`: choosing a file explicitly on the conflict screen.
- `standalone-modes`: menu entries (history, cleanup) and ticket esc returning to the menu.
- `branch-cleanup`: entry from the menu, back to the menu, BORRAR names the branch.
- `run-history`: reasons for ineligible `x`/`r`, q quits and esc goes back.

## Affected Areas

| Area | Impact |
|---|---|
| `internal/app/keys.go`, `view.go`, `app.go`, `update.go` | Modified |
| `internal/git/conflict.go` (display labels only) | Not modified (ADR-9 cut) |
| `internal/app/*_test.go` | Modified pins plus new footer and esc/q matrix tests |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| q in history now quits; users used to it going back | Med | The footer says "Esc volver · q salir". The run stays resumable. |
| Updated string pins hide a regression | Med | Update pins deliberately and never delete them. The matrix test checks the behavior. |
| Over budget | Med | Conflict-kind labels were cut (ADR-9); column sizing became a targeted width fix. The user approved size exceptions of 970 and then 1150 lines; the actual size is above both (see the apply-progress report). |
| Resume or restore regresses | Low | Existing resume, `quitCmd`, ctrl+c and `boundary_test.go` tests stay green |

## Rollback Plan

UI-only. No persisted schema, Git or Salesforce behavior changes. Revert the PR.

## Dependencies

- Stacked on `feat/tui-self-update` (PR #4).

## Success Criteria

- [ ] `e` opens the highlighted conflict file. Tested with 2 or more files.
- [ ] Every state's footer matches its handler, pinned by a test.
- [ ] An esc/q matrix test covers every category above.
- [ ] Cleanup is reachable from the menu even when the ticket is pre-filled.
- [ ] No silent no-ops in the listed cases. No English copy on screen.
- [ ] One PR of about 720 lines (about 260 production, 460 test), within the 800-line budget.

## Proposal question round

Auto mode. The orchestrator fixed the scope and asked for the rule above. For user review:
- Is it acceptable that q in history and the pickers now quits instead of going back?
- Should esc on the push screens return to Succeeded, or stay unbound?


> **Size note (2026-11-05, final):** 1505 changed lines (excl. openspec) under a user-approved size exception (800 → 970 → 1150 → 1505). Verify round 2: PASS WITH WARNINGS.
