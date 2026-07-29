# Proposal: Deferred Hardening — L-1 + D1 + D2

## Intent

Close three documented, LOW-severity follow-ups deferred by prior slices' adversarial reviews, now that the 19-HU epic is complete. One change / one PR (user request): a **safety** fix (never prune a resumable run), a **robustness** guard (no standalone mode over an in-progress tree), and a **polish** fix (collision-free identity + distinct render). Success: each deferred finding is closed with regression coverage and no behavior regression elsewhere.

## Scope

### In Scope
- **L-1 (safety)**: `runs.Prune`/`selectPruneCandidates` (`internal/runs/retention.go`) SKIPS a non-terminal/resumable run — jobId in flight with non-terminal `Status`, OR an unfinished cherry-pick `Phase` — even when outside `keepLast`/`keepDays`. Count/age rule otherwise unchanged.
- **D1 (robustness)**: guard standalone delta/validate entry (`keyMainMenu`, `keys.go:437,441`) on `m.repoState.InProgress` → blocked with an actionable notice; `deltaCmd`/`validateCmd` NOT fired. Best-effort (fail-open when InProgress unknown).
- **D2 (polish)**: standalone-delta output identity includes the base branch (no same-dir collision across bases); Mode-tagged runs render `[delta] <base>` / `[validate] <package>` instead of a blank `"-to-"`.

### Out of Scope
- Changing the retention count/age math (only ADD the protection skip).
- Any NEW delta/validation/menu behavior.
- Origin/PR/HU-019-activation logistics.

## Capabilities

> Contract for sdd-spec.

### New Capabilities
None.

### Modified Capabilities
- `run-retention`: L-1 — non-terminal/resumable runs are never pruned (additive protection clause; count/age rule intact).
- `standalone-modes`: D1 — in-progress guard on standalone entry; D2 — collision-free per-base identity + distinct Mode-tagged render.

## Approach

Extend, don't rewrite. L-1 adds a record-only `isProtectedFromPrune` predicate in front of the pure count/age selection. D1 checks the already-fetched `m.repoState` before entering the mode. D2 folds the base into the standalone-delta run identity and tags history/detail render by `Mode`. Strict TDD: pure-unit for L-1 (`selectPruneCandidates` table), app-level for D1/D2. No org.

**Design must pin**: L-1 protected predicate + import strategy (runs-local terminal-status set vs Phase-only check — AVOID `internal/runs → internal/salesforce`); D1 guard site + fail-open-when-unknown; D2 identity change (base-in-`Ticket` vs output-dir segment) + render format.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/runs/retention.go` | Modified | L-1 protection skip |
| `internal/app/keys.go` | Modified | D1 guard; D2 identity/render |
| `internal/app/view.go` | Modified | D2 `runPackagePath` / history render |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| L-1 extends the retention rule HU-017 froze | Med | Legitimate follow-up; keep count/age intact, ADD skip only |
| `runs` gains awkward `salesforce` dep | Med | Prefer runs-local terminal set / Phase-only check (design pins) |
| D2 breaks standalone-delta e2e | Med | Update `TestE2E_StandaloneDelta_*` assertions |
| D1 best-effort race (InProgress unknown) | Low | Fail-open — degrades safely per original finding |

## Rollback Plan

Revert the single PR. Each item is independent and additive; no data migration, no schema change to run records beyond an optional base field.

## Dependencies

None — all three items resolved without external decisions. Post-epic follow-up (out of the enumerated Fase 1–5 list).

## Success Criteria

- [ ] L-1: a non-terminal/resumable run outside keepLast/keepDays is NOT pruned; terminal old runs still prune (backward-compat preserved).
- [ ] D1: standalone entry with `InProgress==true` is blocked (no cmd fired); `false` enters normally.
- [ ] D2: two deltas to different bases yield distinct dirs; Mode-tagged runs render `[delta]`/`[validate]`, never `"-to-"`.
- [ ] Strict TDD, no org; `run-retention` + `standalone-modes` deltas authored.
