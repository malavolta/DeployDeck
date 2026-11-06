# Exploration: tui-ux-consistency (change A)

Source: a read-only UX audit of the TUI on branch `feat/tui-self-update` (2026-11-05). The orchestrator verified the conflict-cursor, window-size and bubbles claims below. This change is the quick, behavior-level half (A). The structural adoption of `bubbles` is a separate follow-up change (B: `tui-bubbles-adoption`). A must NOT introduce bubbles.

## Findings in scope for A

### 1. Conflict screen opens an arbitrary file (BUG)
- `e` opens `Unmerged[conflictCursor()]` (`internal/app/keys.go:1097-1100`).
- `conflictCursor()` (`keys.go:1110-1115`) returns `m.cursor`, which still holds its value from commit selection.
- `keyConflict` (`keys.go:~1086-1105`) has no up/down/k/j handling, and the view draws no selection marker (`view.go:517-519`).
- Result: with more than one conflicted file, `e` can open a file the user never chose.
- Fix: give the screen its own cursor, reset when the screen is entered, with ↑/↓/k/j and a `>` marker.

### 2. Footers disagree with the key handlers
| Screen | Footer | Reality |
|---|---|---|
| Validation polling (`view.go:742`) | no `c` | `c` cancels own job (`keys.go:1271`); the mockup shows "c cancelar mi job" (`docs/MOCKUPS_TUI.md:317`) |
| Succeeded / Aborted (`view.go:896,898,77`) | no `d` | `d` deletes the branch (`keys.go:107,136`) |
| Branch cleanup (`view.go:1410,1422,1461`) | "q salir" | q/esc return to ticket input (`keys.go:320`) |
| Cherry-picking (`view.go:486`) | "q salir cuando termine" | `handleKey` (`keys.go:17-75`) has no case for CherryPicking, BranchCreation or CommitDiscovery, so q does nothing |
| Commit selection (`view.go:374`) | missing ↑/↓ and `x` | both work |

Footers must list exactly the keys that act in that state. The structural fix (a `key.Binding` single source of truth) belongs to B; A fixes the strings and adds tests that pin each footer against its handler.

### 3. Inconsistent q/esc semantics
- Esc on ticket input goes to Doctor, not the main menu (`keys.go:923-938`).
- In pickers and history, q goes back. In main-flow screens q quits the app: verification `keys.go:1132`, package review `:1197`, delta `:1157`, push `:405`.
- Target convention: esc = back one screen, q = quit. Quitting mid-flow while a run is persisted should confirm first, or at least be reflected honestly in the footer. Proposal/design must decide the exact rule per state without breaking resume semantics or `quitCmd()` branch restore.

### 4. Hidden features and silent no-ops
- Branch cleanup is reachable only by `b` on an EMPTY ticket (`keys.go:888`), and is not shown in the menu or footer. When the ticket is pre-filled from the branch, it is unreachable. Fix: add a main-menu entry "Limpieza de ramas", placed after "Historial de runs".
- The AI `a` key appears only in body text (`view.go:~1049`).
- Silent no-ops:
  - `p`/`d` without a branch (`keys.go:98,112`);
  - `x`/`r` on ineligible history rows (`:1333,1347`);
  - Enter in standalone delta (`:1175`).
- Fix: show a short Spanish notice explaining why the key is unavailable. For example, x on an ineligible run should say the reason from `runs.QuickDeployEligible`.

### 5. English copy and misaligned columns
- English notices: `keys.go:550,562,868,908,1053,1088,1124,1208` and `update.go:570,970`.
- Other English UI strings: "Destructive changes" (`view.go:~566`), "title:", "pushed/local" (`:1436`), "validating" (`:1381`), and conflict kinds "text/binary/modify/delete" (`internal/git/conflict.go:30`, display only).
- Misaligned headers: "Antigüedad" (10 characters) sits in a 6-character column (`view.go:1428`), and "Destino" (7) in a 6-character column (`:1287`).
- Fix: translate the UI copy to Spanish and size columns from the header width. Keep internal identifiers and log/error wrapping in English when they are not user-facing.

### 6. Typed confirmations (minor)
DESPLEGAR, CANCELAR and BORRAR are consistent. The BORRAR prompt does not name the branch (`view.go:1453`); add it.

## Out of scope for A (goes to B)
- Adopting bubbles (list, viewport, help, spinner, key), the `?` help overlay, `tea.WindowSizeMsg` and scrolling.
- Spinners on push, PR, queue and gate, plus elapsed time during polling.
- Adaptive colors and status badges.
- Error screen redesign.
- Box borders and other mockup drift.

## Constraints
- `internal/app/boundary_test.go` must stay green.
- Strict TDD.
- Existing tests pin many footer and copy strings; they must be updated deliberately, not deleted.
- Resume behavior (`resumeInto`, `onResumeDetect`), `quitCmd()` branch restore, and the typed-confirmation guards must not regress.

## Rough size
About 400-600 changed lines, mostly copy, footers and tests, plus a small conflict-cursor change and the menu entry.
