# Exploration: HU-016 — Re-Promocionar Ticket Entre Ambientes (`re-promote`)

Builds on 5 archived slices (foundation-mvp-git, delta-validation, deploy-queue, run-history, push-pr). 17 living specs. HU-016 COMPOSES existing capabilities — no new machinery, only new pre-fill data feeding the existing flow.

## Current state (verified against REAL code — anchor on `internal/runs.Record`, NOT the aspirational `ARQUITECTURA.md` RunRecord)
- **Run history** (HU-013): `runs.Writer.List()`/`Load(runID)` (`writer.go:116,147`) back `StateRunHistory` (`app.go:110`, `keys.go:478-505`). Today that screen only does `↑/↓`/`d`/`Enter→resumeInto` (re-enter a LIVE run). No action for a terminal Succeeded run.
- **RF-002 source suggestion**: `git.SuggestDefaultSource(candidates,cfg,target)` (`source_suggestion.go:26-54`) resolves "previous env of X" via a fixed `environmentPipelineOrder = ["integration","uat","production"]` (`:12`) mapped through `config.Branches`. Wired via `flow.go:36 resolveSource` ← `discoverCmd` (`commands.go:221-249`). HU-016 needs the INVERSE — "next env after X" — which doesn't exist yet.
- **Equivalence-by-patch-id** (HU-002): `git.ClassifyEquivalence` (`equivalence.go:44-55`), `Service.Cherry`/`Service.PatchID` (`service_equivalence.go:40-91`) — directly reusable for "does this prior-run commit have a patch-id equivalent in the new source range".
- **Promotion machinery**: `CreatePromotionBranch`, cherry-pick engine, `delta.Generate`, validate — all unchanged; HU-016 only changes how `m.plan.SelectedCommits`/`m.ticket` get pre-seeded.
- **Record**: `internal/runs.Record` (`writer.go:29-66`) grows additively per HU (Commits, PickIndex/PickTotal/Phase from HU-013; PRUrl from HU-014), each `omitempty`, no SchemaVersion bump. `ARQUITECTURA.md:249` + `HISTORIAS.md:880` anticipate `SourceRunID string json:"sourceRunId,omitempty"`; `HISTORIAS.md:1288` names it in the HU-016 e2e.

## Scope IN/OUT (`docs/HISTORIAS.md:1038-1077`)
IN: re-promote a ticket already SUCCESSFULLY validated in env A onward to env B, reusing the prior run's context via a new `SourceRunID`; source = the prior run's own env (its `Target`), NOT the feature branch; offer to reuse the prior run's N commits, pre-loaded and EDITABLE; map prior SHAs to their equivalents in the new origin via patch-id; EXPLICIT warning (not silent) when a prior commit isn't found in the new origin; persist `SourceRunID` on the new run; a FAILED prior run does NOT offer reuse; a ticket with no prior run falls back to the normal manual flow.
OUT: HU-017 cleanup (do NOT clean the prior run's temp `deploy/*` branch — that's HU-017's job), HU-019 release pipeline, HU-015 quick deploy / HU-018 standalone (Futuro), the local-model PR idea.

## Resolved decisions (from the 6 open decisions)
1. **Entry point = `StateRunHistory`, new key `r`** on an eligible row → `startRePromoteInto(rec)` (mirrors `resumeInto` at `update.go:176-221`). `r` is free within `keyRunHistory`.
2. **Eligibility** = prior run `Succeeded` OR `SucceededPartial` (both terminal-success); `Failed`/`Canceled`/`Aborted` NOT eligible. No prior run for the ticket → normal manual flow.
3. **Source = `origin/<rec.Target>`** (the environment branch itself, post-merge) — KNOWN outright from the picked run, so BYPASS `SuggestDefaultSource`. Do NOT use `rec.PromotionBranch` (the temp `deploy/*` branch, which HU-017 may clean).
4. **Next target = HYBRID**: a NEW forward helper (`NextEnvironmentBranch`, mirror of `SuggestDefaultSource`, reusing `environmentPipelineOrder`) suggests the env after `rec.Target`'s env as the DEFAULT on the existing `StateTargetSelection`, still user-overridable. If `rec.Target` is outside the pipeline (e.g. `Release/*` glob) or has no next → no auto-default, user picks manually. Removes manual re-selection for the common INT→UAT→prod path while staying overridable and glob-safe.
5. **Reuse commits**: patch-id-map `rec.Commits` into the new `origin/<nextTarget>..origin/<rec.Target>` discovery range (`git.Discover` + `Service.PatchID`/`ClassifyEquivalence`); pre-check the equivalents in `StateCommitSelection`, still editable; a prior SHA with NO patch-id match → EXPLICIT warning (AC3), never silent omission.
6. **`SourceRunID`** = additive `Record` field (`omitempty`, no schema bump), set at NEW-run creation time (the source run is known BEFORE the flow starts — unlike `PRUrl` which is post-hoc; justify this minor deviation from the `MarkPRCreated` precedent in design).
7. **Flow after pre-seed** = the EXISTING `StateTargetSelection → StatePlanPreview → StateBranchCreation → cherry-pick → delta → validate` states, unmodified.

## Acceptance criteria + E2E (`docs/HISTORIAS.md:1060-1077`)
ACs: prior successful run → offer reuse (N commits precharged+editable); accepted → selection precharged with patch-id equivalents, editable; commit not found in new origin → explicit warning; completed → new run records `SourceRunID`; failed prior run → NO precarga; no prior run → manual flow.
**Harness = `temp + fs` (git repo + runs fixture, NO org), CI: si** (`:1073,1077,1288`). Pure git + runs.Writer; no Salesforce fake/alias needed for the reuse/mapping core.

## Testability (strict TDD)
Pure-unit: `SourceRunID` round-trip (mirror PRUrl); `NextEnvironmentBranch` table (INT→UAT, UAT→prod, prod→none, Release/*→none); `keyRunHistory` `r` gated on Succeeded vs Failed. Integration (temp git + runs fixture, `-short`-skippable): re-promote uses `origin/<rec.Target>` as the new source; patch-id equivalence maps two different SHAs for the same logical commit; the missing-commit warning; the consolidated e2e (`:1071-1077`). Real-org NOT needed (story says "sin org").

## Risks
- Fixed 3-stage `environmentPipelineOrder` excludes `Release/*` glob targets → handle via "no auto-next, user picks" (decision #4).
- Patch-id equivalence inherits the repo's known squash-merge fragility (`commit-discovery/spec.md:96-99`); AC3's explicit warning is the acknowledged mitigation.
- Anchor design on the REAL `internal/runs.Record` (missing SourceBranch/TargetOrg/DeployBranch/etc. from the ARQUITECTURA sketch), not the aspirational doc.
- `SourceRunID`-at-creation deviates from the `PRUrl`/`Mark*` precedent — justify in design.

## Ready for Proposal: yes (resolve the decisions above; anchor on real code).
