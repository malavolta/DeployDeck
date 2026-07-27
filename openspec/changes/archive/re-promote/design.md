# Design: Re-Promote Ticket Between Environments (HU-016)

## Technical Approach

Compose the existing promotion flow; add no new promotion machinery. A new `r` key on an
eligible `StateRunHistory` row calls `startRePromoteInto(rec)` (structural mirror of
`resumeInto`), which pre-seeds ticket + source + next-target and fires a patch-id remap
through a new `git.Service` method. The remap lands the user on the UNCHANGED
`StateCommitSelection` with the reused commits pre-checked and editable; from there the
existing `StateTargetSelection → PlanPreview → BranchCreation → cherry-pick → delta →
validate` states run verbatim. `internal/app` stays exec-free: every git subprocess goes
through `git.Service` (boundary_test.go holds). Specs: `run-history` (`r` entry),
`run-persistence` (`SourceRunID`), `commit-discovery` (`NextEnvironmentBranch`).

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| Remap location | New `git.Service.RemapCommitsByPatchID` composing `CommitsInRange`+`PatchID` | Remap inside `internal/app` | App must never exec; the boundary test forbids a new app-level seam. Reuses the existing runner. |
| `SourceRunID` timing | Written at run creation in `onBranchCreated` from `m.sourceRunID` | Post-hoc `MarkSourceRun` (à la `MarkPRCreated`) | `PRUrl` is unknown until `gh pr create` returns; the source run is known the instant the user picks it in history. Threading it like `Ticket`/`Commits` (also creation-time) avoids a redundant second write. |
| New source | `origin/<rec.Target>` known outright, bypass `SuggestDefaultSource` | `rec.PromotionBranch` (`deploy/*`) | The temp branch is HU-017's to clean; the env branch is the durable post-merge home of the promoted commits. |
| Next target | `NextEnvironmentBranch` default via `m.prelim`, overridable on `StateTargetSelection` | Hard-lock next env | Reuses the existing `prelim`→`destinationIndex` default-cursor path; keeps the user's override and stays glob-safe. |
| Pre-check set | Only patch-id equivalents of `rec.Commits`; unmatched → explicit warning | Pre-check whole range delta | The env-branch range holds other tickets; only the prior run's commits are the reuse target. AC3: warn, never drop. |
| No next env | `ok=false` → degrade to manual flow, ticket pre-filled, notice shown | Block `r` | prod/`Release/*` have no next stage (decision #4); manual is the documented fallback. |

## Data Flow

    StateRunHistory ─(r, eligible)→ startRePromoteInto(rec)
        │ seed: ticket, source=origin/<rec.Target>, prelim=NextEnvironmentBranch,
        │       sourceRunID=rec.RunID
        └→ rePromoteRemapCmd(rec.Commits, next, rec.Target)
              └ git.Service.RemapCommitsByPatchID  (rev-list + patch-id, in git.Service)
                 → rePromoteSeededMsg{Matched, Unmatched}
        onRePromoteSeeded: items=NewCommitSelectionItems(Matched); rePromoteMissing=Unmatched
        → StateCommitSelection (pre-checked, editable, warning shown)
        → [existing] TargetSelection → PlanPreview → BranchCreation
              └ Save Record{..., SourceRunID: m.sourceRunID}   ← link persisted
        → cherry-pick → delta → validate  (unchanged)

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/runs/writer.go` | Modify | Add `SourceRunID string json:"sourceRunId,omitempty"`; no SchemaVersion bump |
| `internal/git/source_suggestion.go` | Modify | Add `NextEnvironmentBranch` (forward mirror, reuses `environmentPipelineOrder`/`environmentKeyForBranch`) |
| `internal/git/re_promote.go` | Create | `RemapCommitsByPatchID` + `RemapResult` |
| `internal/app/update.go` | Modify | `startRePromoteInto`, `onRePromoteSeeded`, `isRePromoteEligible`; set `SourceRunID` in `onBranchCreated` |
| `internal/app/commands.go` | Modify | `rePromoteRemapCmd` + `rePromoteSeededMsg` |
| `internal/app/keys.go` | Modify | `keyRunHistory` `r` branch + eligibility gate |
| `internal/app/app.go` | Modify | Model fields `sourceRunID`, `rePromoteMissing` |
| `internal/app/view.go` | Modify | `selectionWarnings` prepends the missing-commit warning |

## Interfaces / Contracts

```go
// git/source_suggestion.go — env AFTER currentTarget; ok=false at prod/Release-*/unconfigured.
func NextEnvironmentBranch(cfg config.Config, currentTarget string) (branch string, ok bool)

// git/re_promote.go — bounded to origin/<target>..origin/<source>; per-priorSHA lookup
// failure (unresolvable/empty diff) → Unmatched, never a hard error.
type RemapResult struct { Matched []DiscoveredCommit; Unmatched []string }
func (s *Service) RemapCommitsByPatchID(ctx context.Context, dir string,
    priorSHAs []string, target, source string) (RemapResult, error)

// app — new-run linkage (in onBranchCreated Record literal): SourceRunID: m.sourceRunID
// eligibility: rec.Status == "Succeeded" || "SucceededPartial"
```

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | `SourceRunID` round-trip + omitempty back-compat | `runs` write/reload, prior-slice json loads zero-valued |
| Unit | `NextEnvironmentBranch` table: INT→UAT, UAT→prod, prod→none, `Release/*`→none, unconfigured→none | Table-driven |
| Unit | `isRePromoteEligible` gate | Succeeded/SucceededPartial vs Failed/Canceled |
| Unit | `keyRunHistory` `r` transition | Direct `Model.Update`: eligible→discovery, ineligible→notice |
| Integration | Remap over temp git: same logical commit under differing SHA maps by patch-id; absent commit → Unmatched | `t.TempDir()` git repo, `-short`-skippable |
| E2E (HU-016) | temp git + runs fixture, NO org: reuse N pre-checked editable; patch-id map; missing warned; new run persists `SourceRunID`; failed prior→no precarga; no prior→manual | Consolidated, `docs/HISTORIAS.md:1071-1077` |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED test |
|---|---|---|---|
| Git repository selection | N/A: reuses `Service.RepoRoot`; no new `-C`/cwd authority | — | — |
| Commit state | N/A: remap is read-only (`rev-list`/`show`/`patch-id`); no index writes | — | — |
| Push/PR | N/A: HU-014/017 out of scope; no push/PR added | — | — |
| Documentation paths / executable classification | N/A: no file classification | — | — |
| Subprocess args (`git show <priorSHA>` from persisted `rec.Commits`) | Applicable: a crafted leading-`-` SHA could be read as an option | Reuse the established positional-arg seam (as `PatchID`/`IsAncestor` do); a per-SHA `git show` failure degrades to `Unmatched`, never a crash | Remap with a malformed prior SHA → warned as Unmatched, no error/injection |

## Migration / Rollout

No migration. `SourceRunID` is `omitempty` — every existing `run.json` still loads. Removing the
`r` handler + `NextEnvironmentBranch` fully reverts behavior; no schema or state-machine change.

## Open Questions

- None blocking. (Non-blocking: remap computes one patch-id per range commit — acceptable for the
  MVP single-ticket delta; revisit only if adjacent-env ranges grow large.)
