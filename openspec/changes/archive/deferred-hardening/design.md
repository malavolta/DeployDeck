# Design: Deferred Hardening — L-1 + D1 + D2

## Technical Approach

Extend three existing seams, never rewrite. **L-1**: add a record-only `isProtectedFromPrune` guard in front of the pure count/age selection in `internal/runs`. **D1**: read the already-fetched `m.repoState` at the standalone menu-entry site and block on `InProgress`. **D2**: fold the base into the standalone-delta run identity and add a `Mode`-aware render. Strict TDD: L-1 pure-unit (`selectPruneCandidates` table), D1/D2 app-level (`Model.Update` + render). No org. Maps to proposal capabilities `run-retention` (L-1) and `standalone-modes` (D1/D2).

## Architecture Decisions

### L-1 — protected predicate + import discipline

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Dup terminal set + phase check in `internal/runs` | 4 status strings + 2 phase strings duplicated; needs a source-of-truth comment | **Chosen** |
| `internal/runs` imports `internal/salesforce` (`IsTerminal`) | Wrong layering — record store would depend on the SF adapter; app→runs→salesforce coupling | Rejected |
| Inject `terminal`/`phase` predicates into `Prune(...)` | Cleanest on paper, but changes HU-017's frozen `Prune(keepLast, keepDays, now)` signature + its cleanup call site for two closed, stable sets | Rejected (heavier) |

`internal/runs` must NOT import `internal/salesforce` **or** `internal/app` (app already imports runs — an import back would cycle). Both value sets are stable, closed, and already hardcoded in `app.isResumable`. Predicate (record-only):

```go
// terminalStatuses mirrors salesforce.terminalStatuses (source of truth:
// internal/salesforce/report.go) — duped, never imported, to keep runs below
// the SF adapter. unfinishedPhases mirrors app.isCherryPickPhase for the same
// layering reason.
func isProtectedFromPrune(rec Record) bool {
    if rec.JobID != "" && !terminalStatuses[rec.Status] { return true } // validation in flight
    return rec.Phase == "cherry-pick" || rec.Phase == "git-conflict"    // unfinished pick
}
```

Wire as the FIRST `continue` in `selectPruneCandidates`'s loop, before `keptByCount`/`keptByAge`. Because the count is ranked by slice index `i` (unchanged by a `continue`), every non-protected record's count/age math stays byte-identical; the protected record is simply never appended to `pruned`.

### D1 — guard site + fail-open

| Aspect | Decision |
|--------|----------|
| Site | `keyMainMenu` enter branch, `StateDeltaSourceSelect` **and** `StatePackageSelect` cases only |
| Action on `m.repoState.InProgress==true` | Set actionable `m.notice` ("resolve the in-progress operation first"); `return m, nil` — stay on `StateMainMenu`, do NOT set `standaloneMode`, do NOT fire `standaloneBranchesCmd`/enter |
| Fail-open | `m.repoState` is populated by `onResumeDetect` (`update.go:310`), batched async from `onPrereqDone`. If the user enters before that msg lands, or detection errored, `repoState` is zero → `InProgress==false` → allow. Degrades safely (sgd/validate already error cleanly per the D1 finding) |
| Promote entry | Unaffected — an in-progress tree is already handled by the resume-offer flow |

### D2 — identity change + render

| Aspect | Decision |
|--------|----------|
| Identity | `confirmDeltaSourceSelect`: `Ticket = "standalone-" + sanitize(base)` (`/`→`-`); `TargetBranch = base` unchanged |
| Dir consistency | `deltaCmd` builds `<deltaBaseDir>/<Ticket>-to-<target>` from the SAME `m.plan`; `runPackagePath` reads `rec.Ticket`/`rec.Target`. Both become `standalone-<base>-to-<base>` — they match by construction. `TargetBranch` untouched, so `deltaCmd`'s `origin/<target>` diff is unchanged |
| Render | In `view.go` history row + detail: `Mode=="delta"` → `[delta] <Target>`; `Mode=="validate"` → `[validate] <basename(ManifestPath)>`; else keep existing columns. Detail panel shows `rec.ManifestPath` for validate runs instead of the meaningless `runPackagePath` |

## Data Flow

    menu Enter ──InProgress?──► block (notice, stay)   [D1]
         └─delta─► confirmDeltaSourceSelect (Ticket=standalone-<base>) ─► deltaCmd ─► onDeltaDone(Save Mode="delta")
    runsList ──► render: [delta]<base> / [validate]<pkg>              [D2]
    Prune ─► selectPruneCandidates ─► isProtectedFromPrune? skip : count/age  [L-1]

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/runs/retention.go` | Modify | Add `isProtectedFromPrune` + runs-local terminal/phase sets; first `continue` in `selectPruneCandidates` |
| `internal/app/keys.go` | Modify | D1 guard in `keyMainMenu`; D2 `Ticket="standalone-"+sanitize(base)` in `confirmDeltaSourceSelect` |
| `internal/app/view.go` | Modify | D2 `Mode`-aware history row + detail; validate detail uses `ManifestPath` |

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `selectPruneCandidates`/`isProtectedFromPrune` | Table: non-terminal-in-flight KEPT, unfinished-pick KEPT, terminal-old PRUNED, recent/count KEPT, `Mode==""` terminal prunes as before |
| App | D1 | `InProgress==true` → stays `StateMainMenu`, `standaloneMode` unset, `cmd==nil` (no launch); `false` → enters |
| App | D2 | Two bases → distinct `runPackagePath` segment; `Mode` render asserts `[delta]`/`[validate]`, never blank `-to-` |

`internal/app/boundary_test.go` unaffected — no new exec/subprocess.

## Threat Matrix

N/A — no new routing, shell, subprocess, VCS/PR automation, or executable-file classification. D1 REMOVES an exec path (blocks `deltaCmd`/`validateCmd`); L-1 only narrows `Prune`'s existing `os.RemoveAll` set (already path-bounded to `runDir`).

## Migration / Rollout

No migration. L-1/D2 add no schema field (base rides the existing `Ticket`/`Target`). Revert = single-PR revert; each item independent + additive.

## Deviations from Exploration

1. **"Different bases share the dir" is inaccurate.** `deltaCmd`/`runPackagePath` already embed `target(=base)`, so distinct bases already yield distinct dirs. The real defect is the SAME-base re-run output overwrite plus the blank `-to-` render. The chosen `Ticket` change makes the identity self-describing and distinguishes standalone from a literal-"standalone" promotion; per-run OUTPUT isolation for same-base re-runs stays out of scope (`deltaCmd` runs before the timestamped runID exists) — logged as residual risk.
2. **Standalone-delta e2e likely needs NO update.** `TestE2E_StandaloneDelta_*` assert only the `.deploydeck/manifest` prefix + `Mode`/`ManifestPath`/`TargetBranch` — none change under `Ticket="standalone-"+base`. Recommend ADDING a per-base-distinctness assertion rather than fixing broken ones.

## Open Questions

- [ ] Confirm sanitize scope for `/` in base branch names (e.g. `release/1.0`) — folding to `-` is assumed.
