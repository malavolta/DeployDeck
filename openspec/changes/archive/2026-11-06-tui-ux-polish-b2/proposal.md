# Proposal: TUI UX Polish (change B2)

**Delivery phase**: Fase 5 Productizacion. Follows `tui-bubbles-adoption` (B, archived 2026-11-05). Single PR #4, branch `feat/tui-self-update`.

## Intent

B left four gaps. Long lists ignore the terminal size, so run history and commit selection scroll off screen. There is no way to see every key in a state. Push, PR creation, queue load, cleanup load and the gate check show static text while they work. Validation polling gives no sense of time. B's verify also left parity holes (W2, W4, W5, W6/S4, S1) where the footer and the handler can still disagree without a test failing.

## Scope

### In Scope (ordered work units, each one a valid stopping point)
- **U1 parity hardening** (lands first so it guards U2-U5):
  - **W2**: history `Enter reanudar` is disabled on non-resumable rows. Push-ready `a` is inert once `prURL` is set, matching the `q salir` footer (`keymap.go:226`, `keys.go:451`).
  - **W4**: amend the spec (see Decision).
  - **W5**: parity is strict. Text-state fixtures get valid buffers. Leniency is kept only for an explicit list of blocked-notice keys.
  - **W6/S4**: add fixtures for a history terminal row, push-ready after the PR with AI wired, the empty-list prune confirm, and the queue error.
  - **S1**: a `stateCount` sentinel so a new state cannot be skipped.
- **U2 windowing and truncation**: `tea.WindowSizeMsg` sets width and height. Pure `window()`, `overflow()` and `truncate()` helpers (B's ADR-8). Applied to run history and commit selection. Height or width 0 leaves the output unchanged.
- **U3 `?` overlay**: `help.FullHelpView` over the same `bindings()`. `?` opens it, and `?` or Esc closes it. Never bound in text-input states. Footers advertise `? ayuda` where it is enabled.
- **U4 phase-aware shared spinner**: the predicate covers push phases, `cleanupLoading`, queue load and the gate check. Keep the single-flight `m.spinning` guard. No tick runs after the phase ends.
- **U5 elapsed and last-poll time** during validation polling, via `Deps.Now`.
- **U6 S3**: a faster quit detector for the slow tests (about 10s each).
- **U7 W7 (optional)**: shared enablement predicates for `keymap.go` and `keys.go`. Otherwise it is recorded as a known gap.

### Out of Scope (B3 and later)
- An error screen redesign (title, cause, next step).
- Windowing for cleanup and the queue. Status badges in history.
- The English continue-gate reasons in `internal/git` (a separate change).
- `bubbles/list`/`viewport`, and bubbles or bubbletea v2.

## W4 Decision

**Recommended: amend the spec.** `Printable Bindings Stay Inactive In Text-Input States` names ticket `b` (empty buffer only) as an allowed guarded key, next to `q`. The footer stays as it is.
- Advertising `b` would break the same MUST ("the footer MUST NOT advertise a printable shortcut there"). It would also change the `Enter buscar   Esc volver` pin depending on the buffer.
- The main menu already reaches branch cleanup, so `b` is a legacy shortcut and does not need to be discoverable.
- No code change, and it follows the `q` precedent.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `tui-presentation`: windowed lists and truncation, the `?` overlay, `Progress Feedback` extended to the new phases, the W4 amendment, and stricter structural parity (sentinel, explicit notice list).
- `run-history`: the list is windowed. Enter is not offered on non-resumable rows.
- `commit-selection`: the list is windowed and rows are truncated.
- `validation-progress`: polling shows elapsed and last-poll time.
- `ai-pr-summary`: `a` is inert once the PR exists.

## Approach

Strict TDD. U1 first, so its strict parity catches drift from the `? ayuda` footers and spinner screens. `window()`/`truncate()` follow B's ADR-8, including the amended rule: bottom-anchored, cursor visible, minimal scroll down. Each list view measures its own chrome. The overlay is a model flag checked before `viewBody`.

## Affected Areas

| Area | Impact |
|---|---|
| `internal/app/window.go` | New |
| `internal/app/keymap.go`, `keys.go`, `view.go`, `update.go`, `app.go` | Modified |
| `internal/app/footer_parity_test.go`, `subphase_test.go` | Modified |
| `go.mod` | `charmbracelet/x/ansi` becomes direct |

## Size Forecast and Cut Plan

User-approved ceiling: **1,300** changed lines, excluding openspec, measured against `e8d783a`. The multiplier is **1.5×**, between B's 1.33× and A's 1.9×.

| Unit | Raw (prod+test) | Adjusted ×1.5 | Cumulative |
|---|---|---|---|
| U1 | 170 (25+145) | 255 | 255 |
| U2 | 305 (105+200) | 460 | 715 |
| U3 | 200 (80+120) | 300 | 1,015 |
| U4 | 150 (50+100) | 225 | 1,240 |
| U5 | 90 (30+60) | 135 | 1,375 |
| U6 | 40 | 60 | 1,435 |
| U7 | 60 | 90 | 1,525 |

Raw total is about 1,015 and adjusted about 1,525, which is over the ceiling. **Cut plan**: measure after each unit. If the projection passes 1,300, cut U7, then U6 (internal-only). U5 is user-visible and the user asked for the full scope, so it is NOT cut silently: apply stops and the orchestrator asks the user for a larger exception (amended 2026-11-06). U3's footer-pin churn is the largest unknown.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Size overrun (mostly tests) | High | Measure per unit, then follow the cut order |
| `?` swallows typed input | Med | Not bound in text states, with a parity fixture per state |
| Spinner tick leak | Med | A test that the loop stops when the phase ends |
| Windowing changes tall-list output | Med | 0 means unchanged, plus byte pins |
| `? ayuda` breaks the byte pins | High | Update the pins deliberately and list them in the spec |

## Rollback Plan

UI-only. No Git, Salesforce or persisted-data changes. Revert the unit commits (one per unit), then run `go mod tidy`.

## Success Criteria

- [ ] With the height set, history and commit selection fit the window and keep the cursor visible. With 0, they are unchanged.
- [ ] `?` shows every binding of the current state, and is inert in text-input states.
- [ ] Every long-running phase animates, and no tick outlives its phase.
- [ ] Polling shows elapsed and last-poll time.
- [ ] Parity is strict, the sentinel is present, and the W2/W6 fixtures are green.
- [ ] The full suite and `boundary_test.go` pass. Size is at most 1,300, or the cuts are applied.

## Proposal question round

Auto mode, so scope was fixed by the user. Assumptions for review:
- W2: disable Enter on terminal rows rather than repurposing it.
- Elapsed format `mm:ss`. Last poll shown as `hace Ns`.
- Overlay copy in Spanish, title `Atajos`.
