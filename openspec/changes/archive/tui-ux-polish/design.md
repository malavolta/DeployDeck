# Design: TUI UX/UI Polish

## Technical Approach

All 8 fixes are pure presentation over `internal/app` (Bubble Tea). No `internal/exec`/net/boundary change. lipgloss renders escapes only under a real TTY; a test-global `Ascii` profile forces byte-identical plain text so the **115 `strings.Contains` assertions never change**. Color lands first as the foundation; the rest reuse existing seams (`header`/`footer`, the `viewPackageReview` footer switch, the `keyPackageSelect` guarded-`q`, the `tickCmd` idiom).

## Architecture Decisions

| # | Choice | Rejected | Rationale |
|---|--------|----------|-----------|
| D1 | `go mod tidy` promotes lipgloss+colorprofile to **direct**; net-new `color_test.go` holds the only `TestMain` → `SetColorProfile(termenv.Ascii)`; `style.go` holds named styles applied at seams | rewrite 115 asserts / golden migration; implicit non-TTY detect (lipgloss #439 flaky) | Ascii = plain bytes under test; prod auto-detects TTY/`NO_COLOR`; pure-render keeps boundary intact |
| D2 | Context bar **omit-until-resolved**: `repo · branch`, append `· org` only when `m.plan.SandboxAlias != ""` | dangling `—` placeholder | no source for org on pre-target screens; never shows stale org |
| D3 | Preserve **two `q` meanings** (quit-app vs go-back); guarded-`q` on the 3 free-text buffers | global flatten | `q` must stay typeable; muscle memory |
| D4 | Correct Spanish (tildes+`¿¡`) everywhere; `viewSourceConfirm` is the reference | — | some notices asserted verbatim → per-string verify |
| D5 | Spinner via `tea.Tick` + `spinnerFrame`+`spinnerTickMsg`, guarded like `onTick` | add `bubbles` dep | reuses proven pattern |

## Data Flow

    View() = updateBanner() + m.screenHeader(title)+body+footer(keys)
                                     │
              screenHeader = header(title) + m.contextBar()   [D2]
    KeyMsg 'q' → key<Screen> → guarded: buf!="" ? buf+="q" : goBack   [D3]
    enter spinner state → tea.Batch(work, spinnerCmd()) → spinnerTickMsg
                          → onSpinnerTick: frame++ ; reschedule iff still in-state

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/app/style.go` | Create | `styleOK/Warn/Err/Bold/Dim` via `lipgloss.NewStyle()`; `mark(tok)` colorizes `[OK]`→green `[!!]`→amber `[XX]`→red `[i]`/`[--]`→dim |
| `internal/app/color_test.go` | Create | sole `TestMain`: `lipgloss.SetColorProfile(termenv.Ascii); os.Exit(m.Run())` |
| `internal/app/view.go` | Modify | `screenHeader`+`contextBar`; route header/footer + ~10 literal-token sites through `mark`; empty-state footer switch; column headers; copy |
| `internal/app/keys.go` | Modify | guarded-`q` on ticket/cancel/quick; notice copy (40) |
| `internal/app/update.go` | Modify | `spinnerTickMsg` case → `onSpinnerTick`; notice copy (6) |
| `internal/app/app.go` | Modify | `menuEntry.description` field + glossary render |
| `internal/app/commands.go` | Modify | `spinnerTickMsg`, `spinnerCmd()` |
| `go.mod`/`go.sum` | Modify | lipgloss/colorprofile → direct |

## Interfaces / Contracts

- `mark(tok string) string` — returns styled `[tok]`; **invariant: Ascii → plain `[tok]`**.
- `m.contextBar() string` → `"  " + filepath.Base(m.deps.Dir) + " · " + branch [+ " · " + m.plan.SandboxAlias] + "\n"`; branch = `m.originalBranch` (async → `""`/`…` until `onOriginalBranch`).
- `m.screenHeader(title)` = `header(title) + m.contextBar()`; routes the ~25 header sites (incl. 4 inline `viewBody` branches) so every screen gets the bar; org self-omits.
- Guarded-`q` (mirror `keyPackageSelect:638`): `case "q": if buf!=""{buf+="q";return m,nil}; <esc-action>` — ticket→`m.ticket`, cancel→`m.cancelInput` (esc-action re-arms poll), quick→`m.quickConfirm`.
- `spinnerCmd()` = `tea.Tick(120*ms, …spinnerTickMsg{})`; `spinnerFrame int` on Model; `spinnerView(f)` cycles `{|,/,-,\}` **appended after** existing static text (never replaces asserted substrings, frame 0 deterministic). Start = `tea.Batch(work, spinnerCmd())` on entry to CommitDiscovery/BranchCreation/CherryPicking/DeltaGeneration/ValidationStart; `onSpinnerTick` reschedules only while `m.state` ∈ those 5 (else nil).

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | color | assert **plain** string (Ascii); never assert escapes |
| Unit | context bar | `View()` Contains `base · branch`; `· org` only after `SandboxAlias` set — RED first |
| Unit | guarded-`q` | non-empty buf → buf gains `q`; empty → state change — RED first |
| Unit | empty state | 0 items → footer Contains `Esc volver`, NOT `Space marcar` — RED first (precedent `viewPackageReview:517`) |
| Unit | spinner | enter state schedules `spinnerCmd`; `onSpinnerTick` bumps frame; frame-0 char present — RED first |
| Copy | tildes/`¿¡` | pure copy: **no new test**; update verbatim-asserted strings + source in the SAME commit |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. `tea.Tick` is an in-process timer.

## Migration / Rollout

No migration. Rollback = revert the single PR; `go mod tidy` re-demotes lipgloss to indirect.

## Open Questions

- [ ] **Size** ~630–960 lines vs 800 budget → tasks must forecast `size:exception` vs slice.
- [ ] Context-bar line insertion must not split any multi-line `Contains` assertion — verify per screen before wiring.
- [ ] Spinner char must not be captured by any in-progress-screen `Contains` — keep it strictly appended.
