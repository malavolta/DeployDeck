# Archive Report — gh-optional-doctor

**Date**: 2026-11-10
**Mode**: openspec
**Status**: Specs merged. Folder move pending (performed manually by the user).
**Archive target**: `openspec/changes/archive/2026-11-10-gh-optional-doctor/`

## Gates

| Gate | Result | Notes |
|---|---|---|
| Verification verdict | PASS WITH WARNINGS | 0 CRITICAL, 2 WARNING, 4 SUGGESTION (`verify-report.md`). Archive allowed. |
| Task completion | PASS | `tasks.md`: all 14 tasks (1.1–7.2) marked `[x]`. No stale checkboxes, no reconciliation needed. |
| Review receipt gate | Not checked | Disabled by user decision (2026-10-14). No `gh review` status or validate run. |
| Delta destructiveness | PASS | No REMOVED or RENAMED sections in any delta. `openspec/config.yaml` `rules.archive` (warn before destructive deltas) not triggered. |

## Carried Warnings (from verify-report.md)

- W1: `apply-report.md` lacks a per-task TDD cycle evidence table. Behavior verified at runtime.
- W2: `internal/app/view.go:186-188` loading line depends on `len(m.checks)==0`. Static only when `NewChecker==nil`, which production wiring does not reach.
- S1–S4: four suggestions, recorded in `verify-report.md`. Not blocking.

## Accepted Deviation (from tasks.md)

The "Doctor Screen Loading Indicator" requirement is satisfied on first load only. On retry (`r`), previous results are kept and no loading line is shown. The deviation was accepted in the design. It is not a regression.

## Specs Synced

| Domain | Requirement | Action | Detail |
|---|---|---|---|
| `prereq-check` | `gh` CLI Availability And Authentication Check (Informative) | MODIFIED (replaced) | Host-scoped probe, 5s timeout, timed-out state, consequence text, never blocking. |
| `prereq-check` | Doctor Screen Loading Indicator | ADDED (appended) | Placed after "AI Endpoint And Model Availability Check", before Design Notes. 3 scenarios. |
| `push-pr-preparation` | `gh` Detection Via A Single `gh auth status` Call | MODIFIED (replaced) | Host-scoped call, timeout, timed-out state. 4 scenarios. |
| `push-pr-preparation` | Compare URL Fallback Derived From Origin | MODIFIED (replaced) | Covers timed-out, no gh-create option, timeout note. 4 scenarios. |
| `push-pr-preparation` | PR Creation Requires Explicit Confirmation And Records The URL | MODIFIED (one scenario replaced) | Only "PR lookup itself unavailable degrades to manual flow" changed to include timed-out. All other scenarios and the requirement text are untouched. |
| `deploy-gate` | Gate-Block Screen | MODIFIED (replaced) | Adds gh-unavailable exception. Scenarios kept, with the first one's wording clarified. |
| `deploy-gate` | gh-Unavailable Gate Block Names gh As The Cause | ADDED (appended) | Placed after Gate-Block Screen, before Design Notes. 5 scenarios. |

Totals: 3 ADDED requirements, 4 MODIFIED requirements (one of them scenario-level only), 0 REMOVED, 0 RENAMED.

Main spec files updated:
- `openspec/specs/prereq-check/spec.md`
- `openspec/specs/push-pr-preparation/spec.md`
- `openspec/specs/deploy-gate/spec.md`

Each MODIFIED block keeps its `(Previously: ...)` note, matching the existing convention in the main specs.

## Archive Contents (to move with the folder)

- proposal.md
- design.md
- tasks.md (14/14 complete)
- specs/prereq-check/spec.md
- specs/push-pr-preparation/spec.md
- specs/deploy-gate/spec.md
- apply-report.md
- verify-report.md
- archive-report.md (this file)

## Traceability

- Engram observation IDs: N/A. Artifact store is `openspec`. No Engram artifacts were read or saved for this change.
- Source artifacts: the filesystem paths above.
- No Engram `mem_save` was performed. The `openspec` artifact store does not use it, and no Engram save tool was available to this executor.

## Pending Manual Step

The change folder has not been moved. The user will run:

```
mv openspec/changes/gh-optional-doctor openspec/changes/archive/2026-11-10-gh-optional-doctor
```

After the move, `openspec/changes/gh-optional-doctor/` should no longer exist, and the archived folder should contain the files listed above.

## Risks

- None blocking. The folder move is pending, and the user performs it.
- Diff size was 525 changed lines against a ~550 budget, within the forecast.
