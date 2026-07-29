# Proposal: Standalone Modes — Delta & Validation Without Full Promotion (HU-018)

## Intent

Let a developer generate a delta OR validate an existing `package.xml` against a sandbox WITHOUT the full promotion flow (no cherry-picks), reached from a minimal main menu — for hand-prepared branches. Pure REUSE of HU-007/008 (delta) and HU-010/011 (validate); the new `StateMainMenu` is the only genuine architectural piece.

Phase: **Futuro** (out of the enumerated Fase 1–5 list). Stacked on `quick-deploy`. Biggest architectural change of the epic (new entry point) — batch the apply.

## Scope

### In Scope
- **`StateMainMenu`** — post-prereq no-resume landing. Entries: Promocionar ticket → `StateTicketInput` (full flow UNCHANGED past that point); Generar delta package → standalone delta; Validar package → standalone validation. Cursor+Enter; `q`/`esc` quits. Hide-unimplemented from a compile-time implemented-modes list (AC3). **LOAD-BEARING: change `onResumeDetect` guard `!= StateTicketInput` → `!= StateMainMenu` (`update.go:269`)** or HU-013 resume-detection silently regresses — explicit regression test required.
- **Delta standalone** — base-branch picker (reuse `git.ListBranches`); current ref = HEAD (`deltaCmd` hardcode, display-only); minimal `m.plan` → reuse `deltaCmd` AS-IS → `StatePackageReview` summary → STOP after summary (mode-aware branch; no cherry-picks/validation). Empty-delta reuses HU-007/008 warning.
- **Validation standalone** — existing `package.xml` (path input) + sandbox; pre-check exists/parses → actionable error, do NOT launch; minimal `m.plan` → reuse `validateCmd` AS-IS → `StateValidationPolling` to terminal.
- **Run creation (both modes)** — minimal Record via `Writer.Create`/`Save` under `.deploydeck/runs/`; additive `Record.Mode string` (`json:"mode,omitempty"`; ""/"delta"/"validate"); no SchemaVersion bump.

### Out of Scope
- New delta/validation behavior (pure reuse). A productive deploy tool. Any change to the full-promotion flow past `StateTicketInput`. Arbitrary current-ref picking (HEAD only).

## Capabilities

### New Capabilities
- `standalone-modes`: main menu (hide-unimplemented) + standalone delta + standalone validation + run creation; plus the resume-detection-still-works invariant on the new landing.

### Modified Capabilities
- `run-persistence`: additive `Record.Mode` discriminator (fits the existing additive-growth, no-SchemaVersion-bump contract).

Confirmed: no other living spec changes.

## Approach

Reuse-first. Both modes populate a MINIMAL `m.plan` with EXACTLY the fields the reused command reads, then call `deltaCmd`/`validateCmd` unchanged. The menu is the single new entry point; startup shifts from landing on `StateTicketInput` to `StateMainMenu`.

**Must pin (flag for sdd-design):** exact `m.plan` fields `deltaCmd`/`validateCmd` read (for minimal-plan population); menu placement + `onResumeDetect` guard fix + test-migration plan for the ~148 `Update` callers / post-prereq==`StateTicketInput` assertions; the `StatePackageReview` mode-aware stop-after-summary branch.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `onResumeDetect` guard regresses HU-013 resume | High | Guard → `!= StateMainMenu`; explicit regression test |
| Test churn from new post-prereq landing (~148 `Update` callers) | High | Migration plan in design; direct `state:`-set tests unaffected |
| `StatePackageReview` not mode-aware | Med | Careful branch; keep full-flow continue-to-validation intact |
| Plan-coupling: reused cmds read `m.plan.*` | Med | Enumerate exact read fields in design; set precisely |
| `deltaCmd` `to:="HEAD"` hardcode | Low | Documented limitation (display-only), not a bug |

## Rollback Plan

Additive and isolated: revert the landing + guard to `StateTicketInput`, drop the menu/standalone states and `Record.Mode` (omitempty → old `run.json` still loads). No shared Git state mutated (delta read-only; validation check-only) — nothing to unwind on branches or sandboxes.

## Dependencies

`quick-deploy` (stacked on). Existing `git.ListBranches`, `deltaCmd`, `validateCmd`, `Writer.Create`/`Save`. `sfdx-git-delta` (delta tests) and `sf`/local alias (validation tests).

## Success Criteria

- [ ] Delta mode on a hand-prepared branch → `package.xml` under `.deploydeck/manifest/` + per-type summary, no cherry-picks; run created (Mode="delta").
- [ ] Validation mode on an existing `package.xml` → launches + polls to terminal like the full flow; run created (Mode="validate").
- [ ] Menu entries appear + operational when implemented; hide-unimplemented works (AC3).
- [ ] Invalid/nonexistent `package.xml` → actionable error, no launch; empty delta reuses HU-007/008 warning.
- [ ] Resume-detection still offered after prereq when a resumable run exists (regression guard).
