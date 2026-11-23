# Apply Progress: tui-ux-consistency (change A)

Status: Units 1-5 and tasks 6.1-6.4 implemented; ADR-8/ADR-9 cut. Size exception up to ~970 lines accepted by the user (recorded here and in state.yaml). Nothing committed.

## Size (measured against fd7dac1, excluding openspec, index untouched)
- tracked: 229 insertions + 116 deletions = 345; untracked non-openspec: 597 (5 new test files) => ~942 changed lines.
- Over the 800 budget by ~142 even with Units 6 (ADR-9 conflict labels) and 7 (ADR-8 widths) cut entirely.

## Done
- Unit 1 conflict cursor (conflictIdx, `>` marker, up/k/down/j, e uses it, footer): conflict_cursor_test.go
- Unit 2 menu "Limpieza de ramas", enterBranchCleanup, ticket esc/q -> menu, cleanup esc -> menu / q quit, confirm esc/q -> list, BORRAR prompt names branch, guarded q: menu_cleanup_test.go
- Unit 3 esc/q matrix (keys.go handlers): esc_q_matrix_test.go
- Unit 4 Spanish notices, noticeLine, quickDeployReasonES, notice clears on next key, delta e gate: notices_test.go
- Unit 5 footers normalized for esc/q parity + parity test over the esc/q fixtures: footer_parity_test.go

## Cuts and gaps
- ADR-9 (conflict kind labels, 6.5/6.6) and ADR-8 (column widths, 7.x) cut; scenarios removed from specs (tui-presentation: "Conflict kinds are shown in Spanish", "Cleanup age column fits its header", "Target column fits its header"; cherry-pick: MODIFIED "Displayed type labels are Spanish").
- NOT done and not cut by a lever: 6.2-6.4 (English labels, accented title already existed, inverted question marks), 3.6, 3.8 (history resume e2e; only a unit pin), 4.8 (r on Succeeded pin; existing tests cover it), 4.5 detail for Aborted p, 5.1 full ADR-2 acts() matrix (replaced by esc/q advertised-iff-acts parity over escQ fixtures plus pinned fragments).
- 4.10 implemented as a source scan for English notice literals, not per-trigger tests.

## Update: 6.2-6.4 done (copy_test.go)
- Destructive changes -> Cambios destructivos, pushed/local -> pusheada/sin push, validating -> validando, title: -> título:. Yes/no prompts and the accented result title already satisfied the spec; pinned by test. Pins updated: branch_cleanup_test, view_test.
- Not translated: cleanup MergedLabel values ("abandoned", "likely merged") and persisted Phase strings (data).

## Corrective round 1 (after FAILED verify)

Status: C1-C7, W2-W5, W7, W8 addressed. Size exception recorded as 1150 in state.yaml. Nothing committed; boundary_test.go unchanged.

### Size
- Measured against fd7dac1, excluding openspec: tracked 514 (changed lines) + untracked 991 = ~1505. This is ABOVE the approved 1150 by ~355. Unresolved: needs a user decision (trim tests, or raise the exception again). The corrective round added ~470 lines (about 310 in `subphase_test.go`, about 110 in view.go, the rest pin updates).

### TDD Cycle Evidence

| Task | RED (test and failure) | GREEN | Triangulation |
|------|------------------------|-------|---------------|
| C2.3 / S2 delete prompt on Succeeded/Aborted | `TestSubPhase_DeletePromptOnSucceededAndAborted`: no `¿Borrar la rama deploy/DD-9?`, normal p/d/Enter footer still shown; strong mode lacks BORRAR prompt | `deleteConfirmView()` rendered by `viewValidationResult` and `abortedTail()` | Succeeded and Aborted, confirm and strong, with typed buffer `BOR_` |
| C1 push-ready `a` | `TestSubPhase_PushReadyFooterFollowsAI`: footer `g crear PR con gh   q salir` has no `a` with AI (authed and compare) | `aiFooterPart()` in both footers (pending/accepted hide it) | authed/compare x AI/no AI, plus `a` requests a suggestion |
| C2.2 `n` in footers | `TestFooterParity_SubPhaseKeys/cleanup_confirm,cleanup_prune`: `n` acts but is not advertised | `n/Esc/q cancelar` | probes y,n,a,d,p,g,enter,esc,q |
| W7/S1 parity on sub-phases | same test: also showed `y`,`n` missing and `d`,`p`,`enter` advertised but inert on the delete confirmation | footers from deleteConfirmView | 12 fixtures: delete confirm/strong on both states, idle, cleanup confirm/prune/list, push-ready x4 |
| C3 / W8 widths | `TestSubPhase_CleanupColumnsFitHeaders` (Estado/Merge misaligned); `TestSubPhase_HistoryDestinoFitsHeader` (seen failing with the old `%-6s`) | cleanup `%-10s %-8s`, history `%-10s`, queue Tipo `%-10s` | header vs two data rows |
| C4 copy | `TestSubPhase_RemainingEnglishCopy` (mergedLabel, re-promote warning, queue Deploy/Validate/Components, timed out, title de arriba); `TestSubPhase_PersistedPhaseShownInSpanish` | Spanish strings, `phaseLabelES` display map (stored Phase unchanged) | unknown phase renders raw |
| C5 | `TestSubPhase_CommitSelectionFooter`, `_RNoticeOnCanceledAndAborted`, `_EscOnCleanupResultView`, `_DeclinedResumeIsOfferedAgain` (3.8), `_QFromMainFlowKeepsRunResumable` (3.6) | PIN tests: behavior already correct, written to pass at once (no RED expected) | decline then new Model on the same store |
| W4 casing | existing pin `esc/q cancelar` updated to `Esc/q cancelar` | all footers use `Esc` | specs aligned |

Pins updated deliberately: branch_cleanup_test (merged labels), view_test, delta_validation_test (timeout wording), run_history_test (conflicto), footer_parity_test.

### Decisions
- W5: notices clear on the next key, async ones included (documented in design.md).
- ADR-8 replaced by the targeted width fix, ADR-9 stays cut; design, proposal, tasks and state.yaml updated. ADR-6 states the Spanish fallback.
- Cleanup strong-confirm footer does not list `q` (q is text unless the buffer is empty); parity exempts q/runes on typed states.

### Still open
- Size over the 1150 exception; 4.8 pin not written (existing tests cover r on Succeeded); commit tasks unchecked by design (no commits).


> **Size note (2026-11-05, final):** 1505 changed lines (excl. openspec) under a user-approved size exception (800 → 970 → 1150 → 1505). Verify round 2: PASS WITH WARNINGS.
