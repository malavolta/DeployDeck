# Archive Report: TUI UX Polish (change B2)

**Change**: `tui-ux-polish-b2`
**Status**: `PASS WITH WARNINGS`. Archived as an intentional scope with U7 CUT, and an approved size overrun of 79 lines. 0 CRITICAL open.
**Branch**: `feat/tui-self-update` (single PR #4, `delivery_strategy: single-pr`)
**Archived**: 2026-11-06
**Archive path**: `openspec/changes/archive/2026-11-06-tui-ux-polish-b2/` (the folder move is done by the orchestrator, because this executor has no shell)

---

## What Shipped

| Commit | Unit | Scope |
|--------|------|-------|
| `f481f3a` | U1 | `test(app): make footer parity strict and guard terminal-row keys` |
| `9e5274b` | U2 | `feat(app): window and truncate the history and commit lists` |
| `00f0933` | U3 | `feat(app): add a help overlay on ?` |
| `75ee431` | U4 | `feat(app): animate the spinner in push, PR, queue, cleanup and gate phases` |
| `ff3438a` | U5 | `feat(app): show elapsed time and last poll age while polling` |
| `ce296e3` | U6 | `test(app): detect quit commands by pointer instead of running them` |
| `5c9d708` | corrective | `fix(app): enforce strict footer parity and keep windowed lists inside the terminal` (verify round 1 fixes) |

## What Was Cut

| Unit | Scope | Decision | Destination |
|------|-------|----------|-------------|
| U7 | Shared enablement predicates between `keymap.go` and `keys.go` (tasks 13.1 to 13.3, marked `[~] CUT`) | CUT by the size plan, recorded in `state.yaml` `cut_units: [U7]` | Known gap W5, carried below |

U7 has no spec requirements, so the cut changes no spec. U6 was not cut: it was shipped as `ce296e3`.

Reason: the projection after U6 with U7 still in plan was above the 1,300 ceiling. U7 was cut first, as the plan requires. U5 (user-visible elapsed and last-poll time) was never cut.

---

## What Was Merged

Each delta under `openspec/changes/tui-ux-polish-b2/specs/<cap>/spec.md` was applied to `openspec/specs/<cap>/spec.md`. Untouched requirements were not modified. The `[Ux]` work-unit tags were dropped from every merged heading, so that MODIFIED blocks match their target by name. The tags remain in the archived delta files.

| Capability | ADDED | MODIFIED | REMOVED |
|------------|-------|----------|---------|
| `tui-presentation` | 9: Parity Fixtures Cover Every Reachable Sub-Phase And A Sentinel Guards New States; Parity Is Strict Except For An Explicit Blocked-Notice List; Ticket `b` Is An Allowed Guarded Printable Key; Window Size Is Tracked; Window, Overflow And Truncate Helpers; `?` Opens A Help Overlay With Every Binding Of The State; `?` Is Unbound In Every Text-Input State; Footers Advertise `? ayuda` Where It Acts, With Deliberate Pin Changes; Spinner Covers Push, PR, Queue, Cleanup And Gate Check | 3: Printable Bindings Stay Inactive In Text-Input States; Structural Footer Parity Over Every State; Progress Feedback For Long-Running Operations | none |
| `run-history` | 1: History List Is Windowed To The Terminal | 1: Enter On A Resumable Run Initiates Resume | none |
| `commit-selection` | 1: Commit List Is Windowed And Rows Are Truncated | none | none |
| `validation-progress` | 1: Polling Shows Elapsed Time And Last-Poll Age | none | none |
| `ai-pr-summary` | 1: The Suggestion Key Is Inert Once The PR Exists | none | none |

Notes on the merge:

- **Placement**: MODIFIED blocks replaced the matching requirement in place. ADDED blocks were appended to the end of `tui-presentation`, `run-history` and `ai-pr-summary`. In `commit-selection` and `validation-progress`, which end with a `## Design Notes` section, the ADDED block was inserted before that section, so it stays inside the Requirements part.
- **Heading levels**: every target uses `###` for requirements, the same as the deltas. No level change was needed.
- **`(Previously: ...)` notes**: kept in the living text, as in the B archive.
- **Delta text used as-is**: the MODIFIED `Enter On A Resumable Run Initiates Resume` block in `run-history` is taken verbatim from the delta. It drops the `(HU-013 AC docs/HISTORIAS.md:844)` traceability suffix that the old scenarios carried. See Risks.
- **REMOVED**: none in any delta.

---

## Size History

Excludes `openspec/`, measured against `e8d783a`.

| Point | Figure | Source |
|-------|--------|--------|
| Forecast, U1 to U6 | about 1,250 | `state.yaml` comment (`size_forecast_lines` / `design_forecast_lines`) |
| Forecast, all seven units (raw) | 1,325 (`design_forecast_lines`); 1,525 raw with multiplier (`size_forecast_lines`) | `tasks.md` Review Workload Forecast, `state.yaml` |
| Measured after U1 | 247 | `state.yaml` `apply_measured_lines` |
| Measured after U2 | 592 | `state.yaml` `apply_measured_lines` |
| Measured after U3 | 860 | `state.yaml` `apply_measured_lines` |
| Measured after U4 | 1,146 | `state.yaml` `apply_measured_lines` |
| Measured after U5 | 1,242 | `state.yaml` `apply_measured_lines` |
| Measured after U6 (before corrective) | 1,265 | `state.yaml` `apply_measured_lines`, under the 1,300 ceiling |
| Final, after corrective `5c9d708` and the U7 cut | 1,379 | `verify-report.md` round 2 |
| Ceiling | 1,300 | `state.yaml` `cut_plan` |
| User approval | 1,379 accepted on 2026-11-06 | `state.yaml` `size_exception_lines` |

The overrun of 79 lines is attributable to the corrective fixes in `5c9d708`, which the verify round-1 CRITICAL and warnings required. The user approved the 1,379 figure. The `state.yaml` 1,265 figure was measured before the corrective commit.

---

## Verification

- **Round 1**: FAIL, 1 CRITICAL (C1: `footer_parity_test.go` hard-coded `listed=true`, so the strict-parity requirement was not enforced), 5 WARNING, 4 SUGGESTION.
- **Round 2**: PASS WITH WARNINGS, 0 CRITICAL, 3 WARNING, 5 SUGGESTION. All 58 of 58 scenarios are compliant. Commands: `vet`, `gofmt`, `build`, `go test -short -race` and `go test ./internal/app -count=1` (976 PASS), plus the `-gcflags=all=-l` run. There are no regressions, and the footer pins are unchanged against `ce296e3`.
- **Round-1 findings resolved in `5c9d708`**: C1 (mutation probes M1 to M6), W1 (12,760-render sweep), W2 (`isTyped` / `textInput()` cross-check), W3 (suite passes under `-gcflags=all=-l`), S1 (temp dirs cleaned up), S3 (Esc clears `queueLoading`).

This executor did not re-run the test suite (no shell). Verification evidence is the round-2 report.

---

## Remaining Warnings (verify round 2)

These labels are the round-2 labels in `verify-report.md`. They are not the B-era W4 to W7 labels.

- **W4**: TDD evidence ordering in U2, U4 and U5 is a recorded deviation. The RED step landed together with stubs in those units. Recorded, not fixed.
- **W5**: U7 was cut, so the W7 shared-predicate gap remains. Enablement predicates are still duplicated between `keymap.go` and `keys.go`.
- **W6**: the size overrun described above (1,379 against the 1,300 ceiling), approved by the user.

## Suggestions (follow-ups, not in this change)

- **S2**: add a golden pin for zero-size output (byte identity at width and height 0).
- **S4**: lower the polling tick from 120 ms to 1 s. The 1 s figure is from the verifier.
- **S5**: tighten the `rawBatch` prefilter so it excludes `tea.Every` and test closures whose names contain `Batch`.
- **S6**: decide how to handle the `max(3,…)` floor overflow on very short terminals.
- **S7**: clean up minor weaknesses in the parity test: a stale-entry check for keys that act and also show a notice, the unused-entry check sitting outside its subtest, and checking that the notice actually appears in `View()`.

## Follow-Ups (recorded, not in this change)

- **B3**: the error screen redesign; cleanup and queue windowing (U2 windowing covers history and commit selection only); status badges in run history.
- **English continue-gate reasons**: the conflict continue-gate pending reasons in `internal/git` (`continue_gate.go:26,29`) remain English. This is the known exception recorded in the `Correct Spanish Copy` requirement. It is a separate change.
- **Overflow copy `N más`**: Spanish assumed; user review item carried from B.
- **Open user-review items** (from `state.yaml` `open_issues`): W2 disable of Enter on terminal rows (shipped in U1, user to confirm the approach); elapsed format `mm:ss` and last poll `hace Ns` (shipped, user to confirm); overlay copy in Spanish with title `Atajos` (shipped, user to confirm).

---

## Risks and Notes for the Orchestrator

1. **Footer grammar contradiction (not resolved here)**: `Footer Grammar Stays Byte-Identical` (`tui-presentation`) still has a scenario "Footer bytes are unchanged" that lists the main-menu footer as byte-identical. The ADDED `Footers Advertise ? ayuda` requirement adds `   ? ayuda` to that same footer. The delta did not MODIFY the older requirement. The merge applied the deltas exactly, so the old requirement was left as written. A follow-up amendment should scope `Footer Grammar Stays Byte-Identical` to the pre-B2 baseline, or MODIFY it.
2. **Traceability dropped**: the MODIFIED `Enter On A Resumable Run Initiates Resume` block removed the `(HU-013 AC docs/HISTORIAS.md:844)` suffix from the conflicted-run and jobId scenarios. The delta is authoritative for that block. Restore the suffix in a follow-up if the HU link is still wanted.
3. **Reference to a change-local file**: the `Footers Advertise ? ayuda` requirement says "The exact lists live in `tasks.md`". After the folder move, `tasks.md` is under `openspec/changes/archive/2026-11-06-tui-ux-polish-b2/`. This is a historical pointer, and it is kept verbatim as the delta says.
4. **Verification not re-run here**: no shell in this executor. The test suite and the 1,379 figure come from `verify-report.md` round 2.
5. **Folder move pending**: see Orchestrator Step below.

---

## Gate Checks

- **Task Completion Gate**: passed without any reconciliation. `tasks.md` has no `- [ ]` left. The only non-`[x]` entries are the U7 items (13.1 to 13.3), which are `[~] CUT`. Commit tasks 1.12, 3.11, 5.8, 7.8, 9.6 and 11.3 are `[x]`, and each now carries its SHA. Phases 1 to 12 and 14 are `[x]`.
- **CRITICAL check**: no CRITICAL issues remain. Round 2 reports 0 CRITICAL, and C1 is fixed in `5c9d708`.
- **Native Review Receipt Gate**: NOT EVALUATED. The user's global instructions (CLAUDE.md, "User Overrides") disable review receipt gates as of 2026-10-14. This executor received no structured status with `reviewGate.result`, and it did not run any `gentle-ai review` command.
- **Action context**: no `workspace-planning` mode. Operations stayed inside `openspec/`.
- **Destructive merge check** (`openspec/config.yaml` `rules.archive`): no REMOVED requirements in any delta. The MODIFIED blocks are replacements of the same requirement. No large section was removed, so no warning was needed.

---

## Artifact Traceability

Store mode is `openspec`. No Engram observation IDs apply, and this executor's toolset has no `mem_save` or `mem_search`.

- Proposal: `openspec/changes/tui-ux-polish-b2/proposal.md`
- Exploration: `openspec/changes/tui-ux-polish-b2/exploration.md`
- Delta specs: `openspec/changes/tui-ux-polish-b2/specs/{tui-presentation,run-history,commit-selection,validation-progress,ai-pr-summary}/spec.md`
- Design: `openspec/changes/tui-ux-polish-b2/design.md`
- Tasks: `openspec/changes/tui-ux-polish-b2/tasks.md` (commit SHAs added to the six commit tasks)
- Apply progress: `openspec/changes/tui-ux-polish-b2/apply-progress.md`
- Verify report: `openspec/changes/tui-ux-polish-b2/verify-report.md` (round 1 and round 2)
- State: `openspec/changes/tui-ux-polish-b2/state.yaml` (verify done, archive done, next none)
- Archive report: `openspec/changes/tui-ux-polish-b2/archive-report.md` (this file)
- Merged main specs: `openspec/specs/tui-presentation/spec.md`, `openspec/specs/run-history/spec.md`, `openspec/specs/commit-selection/spec.md`, `openspec/specs/validation-progress/spec.md`, `openspec/specs/ai-pr-summary/spec.md`

---

## Orchestrator Step Remaining

Move the folder (no shell in this executor):

```
mv openspec/changes/tui-ux-polish-b2 openspec/changes/archive/2026-11-06-tui-ux-polish-b2
```

After the move, confirm `openspec/changes/tui-ux-polish-b2/` no longer exists.

---

## SDD Cycle Complete

Planned, specified, designed, implemented (U1 to U6, with U7 CUT), verified (round 2 PASS WITH WARNINGS, 0 CRITICAL), and archived. The living specs `tui-presentation`, `run-history`, `commit-selection`, `validation-progress` and `ai-pr-summary` now reflect the new behavior. Follow-ups are S2, S4, S5, S6, S7, B3 and the English continue-gate reasons.
