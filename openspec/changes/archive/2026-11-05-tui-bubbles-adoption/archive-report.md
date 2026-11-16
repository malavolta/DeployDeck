# Archive Report: TUI Bubbles Adoption (change B, core)

**Change**: `tui-bubbles-adoption`
**Status**: `PASS WITH WARNINGS`. Archived as an intentional partial scope, with U3 and U4 CUT to B2 by user decision. 0 CRITICAL open.
**Branch**: `feat/tui-self-update` (single PR #4)
**Archived**: 2026-11-05
**Archive path**: `openspec/changes/archive/2026-11-05-tui-bubbles-adoption/` (the folder move is done by the orchestrator, because this executor has no shell)

---

## What Shipped

| Commit | Unit | Scope |
|--------|------|-------|
| `d0ebf0b` | U2 | `fix(app): show the wrong-word notice in the strong delete prompt` |
| `16f577f` | U5 | `feat(app): use darker status colors on light terminals` |
| `14da889` | U1 | `refactor(app): derive footers and handlers from one keymap` (go.mod and go.sum included) |
| `360d0d6` | verify fixes | `fix(app): keep the queue retry label and show the empty-list prune prompt` (C1 and W3) |

## What Was Cut

| Unit | Scope | Decision | Destination |
|------|-------|----------|-------------|
| U3 | Window size (`WindowSizeMsg`), history windowing, `truncate` (`window.go`), tasks 5.1 to 5.11 | CUT by user, 2026-11-05 | B2 |
| U4 | Commit-selection windowing and row truncation, tasks 6.1 to 6.7 | CUT by user, 2026-11-05 | B2 |

Reason: the measured size (1,428 for U1+U2+U5) exceeded the 1,200 ceiling. The user chose to approve 1,428 and cut U3 and U4, rather than cut U1 or raise the ceiling further. U3 and U4 were never implemented, so no spec delta for windowing was shipped. The `run-history` and `commit-selection` deltas listed in the original plan were removed from `state.yaml`. No such delta files exist in the change folder.

Scenario coverage that was cut: tui-presentation windowing (window size, windowed lists, truncation), run-history U3 (4 scenarios), commit-selection U4 (5 scenarios). Recorded in `tasks.md` (Scenario Coverage Index) and `verify-report.md`.

---

## What Was Merged

Each delta under `openspec/changes/tui-bubbles-adoption/specs/<cap>/spec.md` was applied to `openspec/specs/<cap>/spec.md`. Untouched requirements were not modified.

Work-unit tags (`[U1]`, `[U2]`, `[U5]`) were dropped from the requirement headings when merged, because the living specs do not carry them. This is needed so that MODIFIED blocks match their target by name. The tags remain in the archived delta files.

| Capability | ADDED | MODIFIED | Requirements after merge |
|------------|-------|----------|--------------------------|
| `tui-presentation` | Footers And Handlers Share One Per-State Keymap; Footer Grammar Stays Byte-Identical; Structural Footer Parity Over Every State; Printable Bindings Stay Inactive In Text-Input States | Semantic Color And Visual Hierarchy (replaced in place: dark keeps bright 10/11/9, light uses 2/3/1); Footer Lists Exactly The Keys That Act (replaced in place: structural parity, the 12-sub-phase probe is gone) | 13 to 17 |
| `branch-cleanup` | Strong Delete Confirmation Shows The Wrong-Word Notice | none | 8 to 9 |

Notes on the merge:

- **Heading levels**: `branch-cleanup/spec.md` uses `## Requirement:` and `### Scenario:`. The new requirement follows that level. `tui-presentation/spec.md` uses `###`/`####` and was kept.
- **`(Previously: ...)` notes**: kept in the living text, as in the other merged specs.
- **Placement**: MODIFIED blocks replaced the matching requirement in place. ADDED blocks were appended at the end of each file.
- **No REMOVED or RENAMED entries** exist in either delta.

---

## Size History

| Point | Figure | Source |
|-------|--------|--------|
| Original proposal forecast | about 1,000 | `proposal.md` |
| Original design and tasks forecast | about 1,320 (U1 950, U2 30, U3 210, U4 95, U5 38) | `design.md`, `tasks.md` |
| Batch 1 measured (U2+U5) | 160 | `apply-progress.md` |
| Measured at 4.1 (U1+U2+U5, adds plus deletes, excluding openspec) | 1,428 (tracked 892, untracked 536). U1 alone came in at about 1,268 against a 950 forecast | `apply-progress.md` batch 2 |
| `360d0d6` fix | a few lines, not re-measured (no shell in this executor) | `apply-progress.md` batch 3 |
| Ceiling | 1,200 (superseded) | `state.yaml` |
| User approval | 1,428 accepted on 2026-11-05 | `state.yaml` `size_exception_lines` |

Final shipped size: 1,428 plus the small `360d0d6` delta.

---

## Follow-Ups (recorded, not in this change)

- **B2**: windowing and truncation (cut U3/U4: `window()`, `overflow()`, `truncate()`, history and commit-selection windowing, `WindowSizeMsg`); the `?` help overlay; a phase-aware shared spinner; elapsed and last-poll time; verify warnings W2 and W4; suggestions S1 to S4.
- **B3**: the error screen redesign; cleanup and queue windowing; status badges.
- **Unassigned**: English conflict continue-gate pending reasons (`internal/git/continue_gate.go:26,29`). Carried from change A.
- **x/ansi**: stays an indirect dependency. Only the cut U3 truncate helper needed it as a direct one, so it is not made direct in this change.

---

## Remaining Warnings (verify, carried)

- **W2**: the parity fixtures do not catch two keys. History `Enter reanudar` on terminal runs (`keymap.go:180`). Push-ready `a` acts after the PR exists while the footer shows only `q salir` (`keymap.go:222-223`, `keys.go:451-466`). Carried to B2.
- **W4**: the ticket `b` is a guarded printable binding. This conflicts with the "no printable bindings in text-input states" MUST. Carried to B2.
- **W5**: the lenient rule "a notice counts as acting" is load-bearing (9 fixtures). Carried to B2.
- **W6**: sub-phase coverage relies on a hand-picked fixture list. Carried to B2.
- **W7**: enablement predicates are duplicated between `keymap.go` and `keys.go`. Carried to B2.

Resolved in this change: C1 (CRITICAL, `360d0d6`), W3 (`360d0d6`), W1 (archive step), W8 (tasks 7.1 and 7.2 ticked, see reconciliation below).

Suggestions S1 to S4 are carried to B2.

User review items still open: the overflow copy `N más` (moot until B2 windowing), and the light-background shades 2/3/1 (shipped, to be confirmed on a real light terminal).

---

## Gate Checks

- **Task Completion Gate**: passed after exceptional reconciliation. `tasks.md` has no unchecked implementation tasks. Phases 5 and 6 are `[~] CUT`. Tasks 1.6, 2.5 and 3.14 were already `[x]` in `tasks.md`, but `apply-progress.md` still showed them as pending, and that is now corrected.
  - **Reconciliation (exceptional)**: tasks 7.1 and 7.2 were `[ ]`. The orchestrator instructed this archive step to tick them. The basis is the final verification in `verify-report.md`, with the fixes in `360d0d6`. The verifier reports the commands green. This executor did not re-run them (no shell).
- **CRITICAL check**: no CRITICAL issues remain. C1 is fixed.
- **Native Review Receipt Gate**: NOT EVALUATED. The user's global instructions (CLAUDE.md, "User Overrides") disable review receipt gates as of 2026-10-14. This executor received no structured status with `reviewGate.result`, and it did not run any `gentle-ai review` command. Risk noted below.
- **Action context**: no `workspace-planning` mode. Operations stayed inside `openspec/`.
- **Destructive merge check** (`openspec/config.yaml` `rules.archive`): the merge contains two MODIFIED replacements, and no large section was removed. No warning was needed.

---

## Artifact Traceability

Store mode is `openspec`. No Engram observation IDs apply.

- Proposal: `openspec/changes/tui-bubbles-adoption/proposal.md` (amended)
- Delta specs: `openspec/changes/tui-bubbles-adoption/specs/{tui-presentation,branch-cleanup}/spec.md`
- Design: `openspec/changes/tui-bubbles-adoption/design.md` (amended)
- Tasks: `openspec/changes/tui-bubbles-adoption/tasks.md` (amended, 7.1 and 7.2 ticked)
- Apply progress: `openspec/changes/tui-bubbles-adoption/apply-progress.md` (amended, batch 3 added)
- Verify report: `openspec/changes/tui-bubbles-adoption/verify-report.md` (new)
- State: `openspec/changes/tui-bubbles-adoption/state.yaml` (archive done, next none)
- Exploration: `openspec/changes/tui-bubbles-adoption/exploration.md`
- Merged main specs: `openspec/specs/tui-presentation/spec.md`, `openspec/specs/branch-cleanup/spec.md`

Engram: the `mem_save` for `sdd/tui-bubbles-adoption/archive-report` was not performed. This executor's toolset has no `mem_save` or `mem_search`. The store mode is `openspec`, so the file above is the persisted record.

---

## Orchestrator Step Remaining

Move the folder (no shell in this executor):

```
mv openspec/changes/tui-bubbles-adoption openspec/changes/archive/2026-11-05-tui-bubbles-adoption
```

After the move, confirm `openspec/changes/tui-bubbles-adoption/` no longer exists.

---

## SDD Cycle Complete

Planned, specified, designed, implemented (U1, U2, U5), verified with warnings (0 CRITICAL), and archived. U3 and U4 are carried to B2. The living specs `tui-presentation` and `branch-cleanup` now reflect the new behavior.
