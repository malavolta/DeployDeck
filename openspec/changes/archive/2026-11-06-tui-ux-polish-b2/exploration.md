# Exploration: tui-ux-polish-b2 (change B2)

Follows `openspec/changes/archive/2026-11-05-tui-bubbles-adoption/`. That change built `keymap.go`, the `key.Binding` footers, the structural parity test and AdaptiveColor. It cut U3 and U4 (windowing and truncation) to B2. Its full design for those units, which was planned but not implemented, is in its `design.md`: ADR-8, the `window(cursor,total,rows)` helper (bottom-anchored), `overflow`, `truncate` via `ansi.Truncate`, and per-view chrome measurement. The original exploration is at `openspec/changes/archive/2026-11-05-tui-bubbles-adoption/exploration.md`.

## Scope approved by the user (2026-11-06, "dale con todo")

The size exception is about 1,300 changed lines, approved up front. Measure it excluding openspec, against the commit before apply.

1. **Windowing and truncation (B's cut U3/U4).**
   - Handle `tea.WindowSizeMsg` and store width and height on Model.
   - Add a pure `window()` helper with "↑ N más"/"↓ N más" indicators, and a `truncate()` helper.
   - Apply both to run history and commit selection.
   - A height or width of 0 means no windowing or truncation, so existing tests are unaffected.
   - The amended spec rule from B applies: the window is bottom-anchored, the cursor stays visible, and scrolling down moves by the minimum.
2. **`?` help overlay.** Render all bindings of the current state with `help.FullHelpView`, from the same `bindings()`.
   - `?` toggles it, and Esc or `?` closes it.
   - It must NOT be bound in text-input states (ticket, package path, CANCELAR/DESPLEGAR/BORRAR, AI input), where `?` is a typed character.
   - The footer advertises `? ayuda` where it is enabled. Footer pins and the structural parity test must be updated deliberately.
3. **Phase-aware shared spinner.** Today `isSpinnerState` (`update.go` ~895-910) covers only 5 states, so push, PR creation, queue load, cleanup load and the quick-deploy gate check show static text.
   - Make the spinner predicate phase-aware (push phases, cleanupLoading, queue loading, gate check).
   - Keep the single-flight `m.spinning` guard.
   - Make sure tick loops stop when the phase ends; do not leak ticks.
4. **Elapsed time and last-poll time during validation polling.** Use the injected clock (`Deps.Now`).
5. **Verify warnings from B:**
   - **W2:** history `Enter reanudar` is advertised on terminal runs where `resumeInto` is a no-op (`keymap.go:180`, `update.go:430`). Disable it, or make Enter act on terminal rows. Push-ready `a` acts after the PR exists while the footer shows only `q salir` (`keymap.go:222-223`, `keys.go:451-466`). Guard it.
   - **W4:** the ticket `b` is a guarded printable binding. Either amend the spec to name it as an allowed guarded key, or advertise it on an empty buffer.
   - **W5:** the lenient "notice counts as acting" rule. Make parity strict by giving text-state fixtures valid buffers, and keep leniency only for an explicit list of blocked-notice keys.
   - **W6/S4:** add the missing fixtures: a history terminal row, push-ready after the PR with AI wired, empty-list prune confirm, and the queue error.
   - **S1:** add an explicit `stateCount` sentinel so a new state cannot be silently skipped.
   - **S3:** use a faster quit detector in the slow tests (about 10s each).
   - **W7:** the duplicated enablement predicates. Optional: extract shared predicates if that is cheap; otherwise record it as a known gap.

## Out of scope (B3)

- An error screen with a title, the cause and a suggested next step.
- Windowing for branch cleanup and the deploy queue.
- Status badges in run history.
- The English continue-gate reasons in `internal/git` (separate change).

## Environment

- The sandbox network is now allowed for github.com, proxy.golang.org and goreleaser.com, so `go get`/`go mod tidy` work.
- bubbles v0.21.0 is already a dependency (`key`, `help`). `spinner`/`viewport` are available in the same module if needed.
- `charmbracelet/x/ansi` is indirect today and becomes direct if `ansi.Truncate` is used.

## Risks

- Size: change A grew 1.9× over its forecast and B's U1 1.33×, mostly in tests. The 1,300 ceiling must be held with a cut plan. The suggested cut order is S3, then W7, then item 4 (elapsed time).
- `?` collides with text input.
- Spinner tick leaks.
- Windowing changes View output for tall lists. Height 0 must mean unchanged output.
