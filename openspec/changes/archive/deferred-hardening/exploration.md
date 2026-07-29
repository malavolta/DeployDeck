# Exploration: Deferred Hardening — L-1 + D1 + D2 (`deferred-hardening`)

Post-epic follow-up (all 19 HUs done). Bundles THREE documented deferred follow-ups from prior slices' adversarial reviews into ONE change/PR (user: "1 pero todo en esa misma pr"). All LOW-severity, all now-actionable without external decisions.

## The three items (code-anchored)
### L-1 (from HU-017 review) — `runs.Prune` can delete a non-terminal/resumable run
`selectPruneCandidates` (`internal/runs/retention.go`) is PURELY count/age: `keptByCount = i < keepLast`, `keptByAge = age <= keepDays`, else pruned. It has no notion of "still resumable", so an old run that is mid-validation (jobId in flight, non-terminal `Status`) or mid-cherry-pick (unfinished `Phase`) — falling outside `keepLast`/`keepDays` — gets its dir `os.RemoveAll`'d, destroying the resume trail. With defaults (keepLast=30/keepDays=90) it effectively never fires, but the cleanup screen (HU-017) newly exposes `Prune`. HU-017's spec froze the retention RULE in-place, so this is a NEW change that legitimately EXTENDS `run-retention` with a protection clause.
- Signals available on the `Record` (record-only, no live git state): `Status` (terminal set `{Succeeded,SucceededPartial,Failed,Canceled}` via `salesforce.IsTerminal` at `report.go:104`), `JobID`, `Phase` (`isCherryPickPhase` in `internal/app/update.go` treats `"cherry-pick"`/`"git-conflict"` as unfinished).

### D1 (from HU-018 review) — no in-progress guard on standalone modes
`keyMainMenu` (`keys.go:437,441`) sets `standaloneMode="delta"/"validate"` and enters the standalone screens with NO check of `m.repoState.InProgress`. So a standalone delta can run `deltaCmd` (`origin/<base>..HEAD`) over a mid-cherry-pick/conflicted tree. Degrades safely today (sgd errors → `deltaErr`, no crash), but a guard is cleaner. `m.repoState` is populated by resume-detect before the menu is normally reached.

### D2 (from HU-018 review) — cosmetic blank render + standalone-delta output-dir collision
`confirmDeltaSourceSelect` hardcodes `Ticket: "standalone"` (`keys.go:500`), so every standalone delta writes `<deltaBaseDir>/standalone-to-<target>/...` (`runPackagePath` at `view.go:901-902` = `<deltaBaseDir>/<ticket>-to-<target>/package/package.xml`) → same-base re-runs overwrite/collide, and different bases share the dir. Standalone-validate runs carry empty `Ticket`/`Target` → history rows + `runPackagePath` render `"-to-"`. No functional break; purely presentation + a latent overwrite.

## Resolved decisions
1. **L-1 — protect non-terminal/resumable runs from pruning.** `selectPruneCandidates` must SKIP a record that is "protected" regardless of count/age. Protected predicate (record-only): `(rec.JobID != "" && !<terminal>(rec.Status))` (validation in flight) OR the run is mid-cherry-pick (an unfinished-`Phase` check). **Import strategy** (design must pin): AVOID an awkward `internal/runs → internal/salesforce` import — prefer a small runs-local terminal-status set (the 4 strings are stable and already referenced in HU-016 eligibility) OR a `runs`-level `isProtectedFromPrune(rec)` helper. Update the `run-retention` living spec with the new "non-terminal/resumable runs are never pruned" requirement. Keep the count/age rule otherwise unchanged.
2. **D1 — in-progress guard on standalone entry.** In `keyMainMenu` (or the delta/validate confirm), when `m.repoState.InProgress`, do NOT enter the standalone mode — set an actionable notice ("resolve the in-progress operation first") and stay on the menu. Best-effort (uses the already-fetched `m.repoState`); if InProgress is true, block. Applies to BOTH delta and validate entries.
3. **D2 — distinct render + collision-free identity.** (a) Standalone-delta output identity includes the BASE so different bases don't collide — e.g. `Ticket = "standalone-" + base` (or fold the base into the output-dir segment) so `runPackagePath` yields a per-base dir; also record the base on the run. (b) Render Mode-tagged runs distinctly in run-history + the detail panel — show `[delta] <base>` / `[validate] <package>` instead of a blank `"-to-"`. Keep it minimal (presentation only).

## Capabilities
- Modified (additive delta): `run-retention` (L-1: non-terminal/resumable runs excluded from pruning). `run-history` or `standalone-modes` (D1 guard + D2 render). Design/spec to finalize which living spec each scenario lands in (L-1 → `run-retention`; D1/D2 → `standalone-modes`). No NEW capability.

## ACs → testability (strict TDD; pure-unit + app-level; NO org)
- L-1: `selectPruneCandidates` table — a non-terminal (jobId + non-terminal Status) run outside keepLast/keepDays is NOT pruned; an unfinished-cherry-pick-Phase run is NOT pruned; a terminal old run IS pruned (existing behavior preserved); backward-compat (a Mode="" terminal run prunes as before).
- D1: entering standalone delta/validate with `m.repoState.InProgress==true` → blocked with a notice, `deltaCmd`/`validateCmd` NOT fired; with InProgress==false → enters normally (regression).
- D2: two standalone deltas to DIFFERENT bases → distinct output dirs (no collision); a Mode-tagged run renders `[delta]`/`[validate]` (not `"-to-"`).

## Risks
- L-1 touches the retention rule HU-017 froze — this is a legitimate FOLLOW-UP change extending `run-retention`, not an in-place HU-017 edit; keep the count/age math intact, only ADD the protection skip. Watch the import direction (`runs` must not gain an awkward `salesforce` dep — prefer a local terminal set).
- D1: the `m.repoState` may be unknown if the user reaches the menu before resume-detect lands (race) — the guard is best-effort; document that an unknown/false InProgress does not block (fail-open is acceptable since it degrades safely per the original D1 finding).
- D2: changing the standalone-delta `Ticket`/output identity must not break the existing standalone-delta e2e (`TestE2E_StandaloneDelta_*`) — update those assertions.
- Small slice; single stacked branch, no origin.

## Ready for Proposal: yes
Design must pin: the L-1 protected-predicate + import strategy (runs-local terminal check vs a Phase-only check); the exact D1 guard site; the D2 identity change (base-in-Ticket vs output-dir segment) + the render format.
