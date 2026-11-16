# Archive Report: run-failure-detail

**Date**: 2026-11-16
**Artifact store**: openspec
**Status**: partial (specs synced and report written; folder move deliberately left to the user)

## Gates

| Gate | Result | Evidence |
|---|---|---|
| Tasks complete | PASS | `tasks.md`: all 23 tasks `[x]`, no unchecked implementation task |
| Verify verdict | PASS | `verify-report.md` round 1: 0 CRITICAL, 3 WARNING, 6 SUGGESTION; round 2 resolves W1-W3, S1, S3, S4, S5 |
| Review receipt gate | Waived | User override (2026-10-14): receipt gates disabled. No `gentle-ai review` commands were run |
| Build / vet / test / gofmt | Green (recorded) | apply-report fix round. Not re-run by the archive phase |

## Verify Round 2 (fix round)

Recorded in `verify-report.md` under "Round 2 (fix round)".

- Fixed: W1, W2, W3, S1, S3, S4, S5.
- Open follow-ups: S2 (in-progress record without report shows "Sin reporte persistido"), S6 (Job Id / Phase from run.json not sanitized, pre-existing).

## Specs Synced (openspec/specs)

| Domain | Action | Details |
|---|---|---|
| run-failure-detail | Created | New main spec copied from the delta. 4 requirements (listed below) |
| run-history | Updated | 1 ADDED, 2 MODIFIED, `d` wording fixed |
| run-persistence | Updated | 1 ADDED |
| validation-progress | Updated | 1 ADDED |

### run-failure-detail (new, `openspec/specs/run-failure-detail/spec.md`)

- Failure Classification Priority
- Coverage Headline And Hint
- Report Read-Back Is Corruption-Tolerant
- Full-Screen Run Detail

### run-history (`openspec/specs/run-history/spec.md`)

- ADDED: Selected Run Tail Shows The Failure Motivo (appended at end of Requirements).
- MODIFIED, replaced in place: Detail View On Selection. `d` is the only key that opens the full-screen detail; the inline toggle is removed; `Enter` never opens it.
- MODIFIED, replaced in place: Per-Row Quick Deploy Action On An Eligible Run. `d` is now "open run detail", not "delete". This is the stale `d` (delete) wording fix. The "delete" wording is gone from the main spec, including the scenario "Enter, d, and r remain unaffected".

### run-persistence (`openspec/specs/run-persistence/spec.md`)

- ADDED: Latest Report Read-Back With Fallback (inserted before "Extended by HU-013").

### validation-progress (`openspec/specs/validation-progress/spec.md`)

- ADDED: Terminal Result Screen Shows The Failure Headline (inserted before "Design Notes").

## Archive Contents (pre-move)

- proposal.md, explore.md, design.md, tasks.md (23/23 complete)
- specs/ (run-failure-detail, run-history, run-persistence, validation-progress)
- verify-report.md (round 1 and round 2)
- apply-report.md

## Archive Move

Not performed by this phase, per user instruction. The user will move the folder to:

`openspec/changes/archive/2026-11-16-run-failure-detail/`

Pending check after the move: the active `openspec/changes/run-failure-detail/` directory no longer exists.

## Engram Traceability

Not applicable. The store mode is openspec, so no observation IDs exist. The archive executor has no Engram tool in this session, so `sdd/run-failure-detail/archive-report` was not saved.

## Risks and Follow-ups

- Spec wording vs code: the coverage headline in the `run-failure-detail` spec still shows the literal `✗`. The code uses the `mark()` token, per the reconciliation note in `tasks.md`. Tests assert on the text and the token, not the literal `✗`. Left as-is to keep the delta verbatim.
- Not committed: the change has no commit yet (apply-report). Commit is the user's call.
- Follow-ups: S2 and S6 (see above).
- Test runtime: `internal/app` ~107s, `internal/git` ~310s (pre-existing).
