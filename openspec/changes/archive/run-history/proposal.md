# Proposal: HU-013 — Run History, Retention & Resume (`run-history`)

## Intent

Crash/quit safety and operational continuity. Today a half-done promotion (cherry-pick mid-conflict) or a running Salesforce validation is lost when the TUI closes — the user must restart from scratch and cannot audit past runs. This slice makes runs durable, browsable, and resumable. It CLOSES two deferred items: **HU-006** reopen/resume (cherry-pick, `cherry-pick/spec.md:142`) and **HU-011** resume-by-jobId (`validation-progress/spec.md:75`).

## Scope

### In Scope
- Run history list/browse screen (`Historial De Runs`, `MOCKUPS_TUI.md:369-388`).
- Retention `keepLast`/`keepDays` + `deploydeck runs prune` CLI + config-bounds validation.
- Resume-by-jobId: re-attach `ReportDeploy` polling into `StateValidationPolling`.
- Resume open cherry-pick conflict on startup into `StateCherryPickConflict` with rehydrated context (ticket, pick N of M).
- Resync when persisted state no longer matches the repo (external resolve/abort).

### Out of Scope / Deferred
- HU-014 push/PR, HU-016 re-promote (`SourceRunID`), HU-017 branch cleanup, HU-015/018.
- Deferrals **closed** by this change: HU-006 reopen/resume, HU-011 resume.

## Resolved Decisions (stated, not reopened)
1. Schema is **additive, NO `SchemaVersion` bump** (Go zero-defaults missing fields; forward-readable contract). Backward-compat test: prior-slice `run.json` still `Load`s.
2. Retention: a run is KEPT if within the most-recent `keepLast` (by `CreatedAt`) OR age ≤ `keepDays`; pruned only if outside BOTH. Pure `selectPruneCandidates(records, keepLast, keepDays, now)`.
3. HU-013 OWNS `Prune` + `runs prune`; HU-017 owns only branch cleanup and reuses the same config.
4. Startup resume runs after prereqs: in-progress cherry-pick OR non-terminal jobId → OFFER resume (accept → route; decline → normal). No blocking interstitial when nothing is resumable.
5. **No literal `StateSuspended`** — resume routes DIRECTLY into `StateCherryPickConflict`/`StateValidationPolling` with rehydrated `Model` fields.

## Capabilities

### New Capabilities
- `run-resume`: startup resume-detection at `onPrereqDone` (offer/decline), resync of diverged repo state (`HISTORIAS.md:847`), direct routing into existing states.
- `run-history`: browsable history screen (list + detail, `Enter` resume).
- `run-retention`: `keepLast`/`keepDays` policy + `runs prune` CLI + `config/validate.go` bounds.

### Modified Capabilities
- `run-persistence`: `Record` += `PickIndex`,`PickTotal`,`CurrentCommit`,`Phase`,`Commits[]`; new `List()`/`Load()`/`Prune()`; forward-readable, backward-compat.
- `cherry-pick`: closes HU-006 deferral — accept resumed entry with rehydrated conflict context.
- `validation-progress`: closes HU-011 deferral — accept resumed `ReportDeploy` re-attach.

## Approach

Additive `internal/runs` methods (`List`/`Load`/`Prune` + record fields) → startup resume-detection `tea.Cmd` wired at `onPrereqDone`, **reusing `git.Service.RepoState` + `salesforce.Client.ReportDeploy` (ZERO new exec seams; `internal/app` never execs directly)** → history screen → `runs prune` Cobra subcommand → config validation.

**Build order**: runs `List`/`Load`/`Prune` + fields → resume-detection wiring → history screen → prune CLI + config bounds.

**Testing** (strict TDD): pure-unit `selectPruneCandidates`/`List` sort/`Load` round-trip/backward-compat; integration on temp git repo + temp `.deploydeck/runs/` with a real in-progress cherry-pick and `FakeRunner`. Full E2E is CI-safe `fs + temp + sf-fake`, no org (`HISTORIAS.md:888-894`). Real-org `ReportDeploy` re-attach is optional/best-effort (timing-hard), never mandated.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/runs/` | Modified | `Record` fields + `List`/`Load`/`Prune` + pure `selectPruneCandidates` |
| `internal/app/update.go` | Modified | resume-detection at `onPrereqDone`; new `StateRunHistory` |
| `internal/app/` (model/views) | Modified | resumed-run context fields; history list/detail views |
| `cmd/deploydeck/` | New | `deploydeck runs prune` subcommand |
| `internal/config/validate.go` | Modified | `Runs.KeepLast`/`KeepDays` bounds |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| ARQUITECTURA vs HISTORIAS `RunRecord` shapes disagree | High | Canonical = extended on-disk `Record`; don't copy either doc literally |
| "pick N of M" has no direct field | Med | Derive `PickIndex = PickTotal - SequencerRemaining`; fix formula in design |
| HU-013/HU-017 retention boundary | Med | Decision #3: HU-013 owns `Prune`; HU-017 reuses config |
| Cold-path perf (detection every launch) | Low | `RepoState` + `List` are cheap local I/O only |
| `KeepLast`/`KeepDays` unvalidated (negatives pass) | Med | Add bounds to `validate.go` |

## Rollback Plan

Additive-only. Revert the `run-history` branch: new `runs` methods, resume wiring, history screen, and `runs prune` disappear; the minimal writer and existing `run.json` files remain valid (no schema migration to undo).

## Dependencies

- Archived `foundation-mvp-git`, `delta-validation`, `deploy-queue`.
- Existing `git.Service.RepoState`, `salesforce.Client.ReportDeploy`, `RunsConfig`.

## Success Criteria

- [x] Prior-slice `run.json` loads cleanly (backward-compat test). — `TestRecord_BackwardCompat_OldShapeRunJSONStillLoads` (Batch 1)
- [x] History lists runs; run w/ jobId resumes via `deploy report`; run w/o job shows last step reached. — `TestViewRunHistory_ListsRunsNewestFirst`, `TestResume_ByJobId_ReattachesPolling`, `TestViewRunHistory_RowShowsProgressReached`
- [x] In-progress cherry-pick offers resume to conflict screen with ticket + pick N/M. — `TestResume_RealInProgressCherryPick_RoutesToConflict`
- [x] Diverged repo (external resolve/abort) resyncs before continuing. — `TestResume_Resync_ExternallyResolved`, `TestOnResumeDetect_ResyncsStaleConflictRecord`
- [x] `runs prune` applies `keepLast`/`keepDays`; invalid config bounds rejected. — `TestRunsPrune_RemovesOnlyOutsideWindow`, `TestRunsPruneCmd_EndToEnd`; bounds via `TestConfig_Validate` (Batch 1)
- [x] CI-safe E2E (`fs + temp + sf-fake`, no org) passes. — `TestE2E_RunHistory_SeedsOfferDeclineResume` + the three required integration tests, all no-org
