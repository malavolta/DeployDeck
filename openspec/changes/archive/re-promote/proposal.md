# Proposal: Re-Promote Ticket Between Environments (HU-016)

## Intent

After a run succeeds in env A, moving the same ticket to env B is manual re-work today: re-find the commits, re-select the source by hand. As a ticket travels `INT → UAT → Release → main`, each manual re-selection risks dropping a commit. HU-016 turns run history from a passive log into the source of the next promotion: pick a prior successful run, pre-load its commits (patch-id-mapped, editable), and link the new run back via `SourceRunID`.

## Scope

### In Scope
- New `r` key on an ELIGIBLE `StateRunHistory` row → `startRePromoteInto(rec)` (mirrors `resumeInto`). Eligible = prior run `Succeeded`/`SucceededPartial`.
- Source = `origin/<rec.Target>` (env branch, post-merge), bypassing `SuggestDefaultSource`; NOT `rec.PromotionBranch`.
- `NextEnvironmentBranch` forward helper (mirror of `SuggestDefaultSource`, reuses `environmentPipelineOrder`) suggests the next env as the DEFAULT on `StateTargetSelection`, still overridable.
- Patch-id remap of `rec.Commits` into `origin/<nextTarget>..origin/<rec.Target>`; equivalents pre-checked in `StateCommitSelection`, editable.
- EXPLICIT warning when a prior SHA has no patch-id match (never silent).
- Additive `SourceRunID` `Record` field (`json:"sourceRunId,omitempty"`), set at new-run creation.

### Out of Scope
- HU-017 cleanup of the prior temp `deploy/*` branch; HU-019 release; HU-015/HU-018 (Futuro); local-model PR idea.
- Any change to `StateTargetSelection → PlanPreview → BranchCreation → cherry-pick → delta → validate`.

## Capabilities

### New Capabilities
- `re-promotion`: pre-seed a new promotion from a prior successful run — eligibility gate, `origin/<Target>` source, patch-id commit remap, missing-commit warning, `SourceRunID` linkage.

### Modified Capabilities
- `run-history`: `r` on an eligible terminal-success row initiates re-promote.
- `run-persistence`: additive `SourceRunID` field, set at new-run creation.
- `commit-discovery`: `NextEnvironmentBranch` next-environment suggestion (forward mirror).

## Approach

Compose existing capabilities — no new promotion machinery. `internal/app` never execs directly; the unchanged states run the promotion. Build order:
1. `SourceRunID` field + `NextEnvironmentBranch` helper (pure).
2. Patch-id remap / pre-seed command (reuse `git.Discover` + `Service.PatchID`/`ClassifyEquivalence`).
3. `StateRunHistory` `r` entry + wiring.
4. Persist `SourceRunID` at creation.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/runs/writer.go` | Modified | Add `SourceRunID` field; set at Create |
| `internal/git/source_suggestion.go` | Modified | Add `NextEnvironmentBranch` forward helper |
| re-promote pre-seed command | New | Patch-id remap of prior commits + warning |
| `internal/app` (`StateRunHistory` keys/update) | Modified | `r` → `startRePromoteInto` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Fixed pipeline order excludes `Release/*` | Med | No auto-next → user picks manually |
| Patch-id squash-merge fragility | Med | Explicit missing-commit warning (AC3) |
| `SourceRunID`-at-creation deviates from `MarkPRCreated` post-hoc precedent | Low | Source run known upfront; justify in design |

## Rollback Plan

Additive-only: `SourceRunID` is `omitempty` (old `run.json` still loads); removing the `r` handler + `NextEnvironmentBranch` reverts behavior. No schema bump or state-machine change to revert.

## Dependencies

- 5 archived slices (run-history, commit-discovery, run-persistence). No external deps.

## Success Criteria

- [ ] Prior successful run offers reuse of N pre-loaded, editable commits
- [ ] Accepted selection pre-checked with patch-id equivalents
- [ ] Missing commit warned explicitly, never omitted silently
- [ ] Completed run persists `SourceRunID`
- [ ] Failed prior run offers no pre-load; no prior run → manual flow
