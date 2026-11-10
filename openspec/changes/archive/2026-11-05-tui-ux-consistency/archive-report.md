# Archive Report: TUI UX Consistency (change A)

**Change**: `tui-ux-consistency`
**Status**: `pass with warnings`. Verify round 2 is PASS WITH WARNINGS with 0 CRITICAL (6 WARNING, 3 SUGGESTION). Round-1 CRITICAL C1 through C7 are resolved. W1 and W2 (stale bookkeeping) were fixed by the orchestrator before archive.
**Branch**: `feat/tui-self-update` (single PR with tui-self-update, PR #4)
**Archived**: 2026-11-05
**Archived to**: `openspec/changes/archive/2026-11-05-tui-ux-consistency/`

---

## What Shipped

| Commit | Scope |
|--------|-------|
| `bbb60e6` | openspec: plan and verify |
| `4d2c81e` | code: `feat(app): make TUI navigation, footers and copy consistent` (single squashed commit for Units 1-5 per tasks.md) |

Work units (see `tasks.md`):

| Unit | Status |
|------|--------|
| 1 Conflict cursor, `e` target | done |
| 2 Menu entry, ticket esc, cleanup back targets, BORRAR prompt | done |
| 3 esc/q rule matrix | done |
| 4 Blocked-key notices, history reasons | done |
| 5 Footer normalization and parity | done (parity covers 12 sub-phases only, see W4) |
| 6 Spanish copy and conflict kind labels | copy half done (6.2-6.4, 6.7 copy part); ADR-9 `Label()` CUT (6.5, 6.6, 6.8) |
| 7 Column widths | ADR-8 replaced by a targeted `%-*s` fix (7.2, 7.3 done); 7.1 and 7.5 CUT, 7.4 REPLACED |

---

## What Was Merged

Each delta under `openspec/changes/tui-ux-consistency/specs/<cap>/spec.md` was applied to `openspec/specs/<cap>/spec.md`. Untouched requirements were not modified. No REMOVED or RENAMED entries exist in any delta.

| Capability | ADDED | MODIFIED | Requirements after merge |
|------------|-------|----------|--------------------------|
| `tui-presentation` | Esc And Q Rule Per Screen Category; Ctrl+C Remains A Hard Interrupt; Footer Lists Exactly The Keys That Act; Spanish Notice For Blocked Keys | Standardized Footer Grammar And Always-Offer-Quit (replaced in place); Correct Spanish Copy (replaced in place); Labeled Table Column Headers (delta text identical to living text, no edit) | 9 to 13 |
| `cherry-pick` | Conflict Screen Has Its Own Selection Cursor; Edit Key Opens The Selected Conflict File; Conflict Footer Lists Navigation | none (delta MODIFIED is "None") | 15 to 18 |
| `standalone-modes` | Branch Cleanup Entry In The Main Menu; Ticket Input Esc Returns To The Main Menu | Main Menu As Post-Prereq Landing (replaced); Standalone Delta Generates A Package Without Cherry-Picks (replaced) | 9 to 11 |
| `branch-cleanup` | Cleanup Reachable From The Menu And Returns To It; BORRAR Prompt Names The Branch | Orphan Deploy Branches Listed For Batch Cleanup (replaced) | 6 to 8 |
| `run-history` | History Esc Goes Back And Q Quits; Reasons For Ineligible x And r | r On An Eligible Terminal-Success Run Initiates Re-Promote (replaced); Per-Row Quick Deploy Action On An Eligible Run (replaced) | 6 to 8 |

Notes on the merge:

- **Heading levels**: `branch-cleanup/spec.md` uses `## Requirement:` and `### Scenario:`. The delta headings were converted to that level for this file only. The other four living specs use `###`/`####` and were kept as they are.
- **`(Previously: …)` notes**: kept in the living text. The house format keeps them, as seen in `openspec/specs/update-notification/spec.md` and the living `tui-presentation` spec.
- **Placement**: MODIFIED blocks replaced the matching requirement in place. ADDED blocks were appended to the Requirements section, before `## Design Notes` in `cherry-pick`.

---

## Size History

The review budget was 800 changed lines (excluding `openspec/`). The user approved three size exceptions: 800, then 970, then 1150, then 1505 (2026-11-05). The final diff is 1505 lines: 514 in tracked code and 991 in new test files. This equals the approved exception. `state.yaml` records `size_exception_lines: 1505`. The 400-line budget risk is High, and the single-PR delivery strategy was kept at the user's request.

---

## Follow-Ups (recorded, not in this change)

1. **English conflict continue-gate reasons**: `internal/git/continue_gate.go:26,29` still produce English pending text. ADR-9 was cut. `ConflictKind.Label()` was also cut, so conflict kind labels stay English. Recorded as a known exception in the merged `Correct Spanish Copy` requirement. Follow-up for a later change.
2. **Strong-confirm wrong-word notice not rendered**: on Succeeded and Aborted, the notice set at `keys.go:177` is never rendered by `deleteConfirmView` (`view.go:1078`). This predates the change. Fold into `tui-bubbles-adoption` (change B). Also W5.
3. **Full-key footer parity**: the probe covers only the 12 sub-phases. Change B replaces footers with `key.Binding`, which removes the need for the hand-maintained token table. Also W4. Scenario `Footer parity matrix` is PARTIAL.
4. **Column widths**: the rune-width helper (ADR-8) was not built. Headers are protected by the targeted `%-*s` fix in `view.go`. The `columns_test.go` alignment test (7.1) was CUT.
5. **User review items** (state.yaml `open_issues`): `q` in pickers and history now quits, where it used to go back; `esc` declines the resume offer; `esc` on pushConfirm returns to Succeeded; `esc` is unbound on pushReady. These follow the proposal rule unless the user objects.
6. **Next change**: `tui-bubbles-adoption` (change B).

---

## Remaining Warnings (verify round 2)

- **W3**: the TDD evidence table in `apply-progress.md` covers only the corrective round and has no Safety Net column.
- **W4**: the footer parity scenario is only partly met. The runtime probe found no real violation across 43 fixtures, but sub-phases outside the 12 are not probed. See follow-up 3.
- **W5**: on Succeeded and Aborted, a wrong word in the strong delete confirmation sets a notice that is never rendered (`keys.go:177`, `view.go:1078`). See follow-up 2.
- **W6**: conflict continue-gate pending reasons are still English (`internal/git/continue_gate.go:26,29`). The requirement records this as a known exception. See follow-up 1.

## Suggestions (verify round 2)

- **S1**: the conflict footer lists `c` while continue is disabled. There `c` only shows a notice.
- **S2**: the doc comment at `keys.go:229` still describes the old back target for q and esc.
- **S3**: a few tests are weak. The "none" outcome does not assert `cmd == nil`, and one notice test sets its state by hand. The "Warning:" text at `view.go:484` could become "Aviso:".

---

## Verification Evidence

From `verify-report.md` (round 2):

- `go vet ./...`, `gofmt`, `go build ./...`: pass.
- `go test ./... -short -race -count=1`: 15/15 packages ok.
- `go test ./internal/app/... -count=1`: 439 PASS, 0 FAIL.
- Spec coverage: 74 scenarios. 73 COMPLIANT, 1 PARTIAL (W4).
- Regressions: none in resume, the `quitCmd()` restore, DESPLEGAR/CANCELAR/BORRAR gates, or ctrl+c.
- `boundary_test.go`, `go.mod` and `go.sum` are unchanged. No bubbles dependency.

Task completion gate: `tasks.md` has no `- [ ]` entries. Completed tasks are `[x]`. Cut tasks (6.5-6.8, 7.1, 7.5) are marked `[~] CUT`, and 7.4 is `[~] REPLACED`. No stale-checkbox reconciliation was used.

---

## Artifact Traceability

Store mode is `openspec`. No Engram observation IDs apply to this change. Artifacts are referenced by file path:

- Proposal: `openspec/changes/archive/2026-11-05-tui-ux-consistency/proposal.md`
- Delta specs: `openspec/changes/archive/2026-11-05-tui-ux-consistency/specs/{tui-presentation,cherry-pick,standalone-modes,branch-cleanup,run-history}/spec.md`
- Design: `openspec/changes/archive/2026-11-05-tui-ux-consistency/design.md`
- Tasks: `openspec/changes/archive/2026-11-05-tui-ux-consistency/tasks.md`
- Apply progress: `openspec/changes/archive/2026-11-05-tui-ux-consistency/apply-progress.md`
- Verify report: `openspec/changes/archive/2026-11-05-tui-ux-consistency/verify-report.md`
- Exploration: `openspec/changes/archive/2026-11-05-tui-ux-consistency/exploration.md`
- State: `openspec/changes/archive/2026-11-05-tui-ux-consistency/state.yaml`
- Merged main specs: `openspec/specs/{tui-presentation,cherry-pick,standalone-modes,branch-cleanup,run-history}/spec.md`

---

## SDD Cycle Complete

Planned, specified, designed, implemented, verified with warnings (0 CRITICAL), and archived. The living specs now reflect the new behavior. Follow-ups are carried into change B (`tui-bubbles-adoption`) and a later conflict-copy change.

**Archived to**: `openspec/changes/archive/2026-11-05-tui-ux-consistency/`
