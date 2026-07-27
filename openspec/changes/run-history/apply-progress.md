# Apply Progress: HU-013 — Run History, Retention & Resume (`run-history`)

Batch 1 of N. Scope: **Phases 1-3 only** (`internal/runs` extension, `internal/config` bounds, run-creation-at-branch + pick-index + `validateCmd` compat). Phase 4 (resume-detection) and later are explicitly OUT of this batch.

Strict TDD Mode: enabled. RED confirmed (compile failure or failing assertion) before every GREEN implementation, per task.

## TDD Cycle Evidence

| Task | RED (test written first) | GREEN (implementation passes) | REFACTOR |
|---|---|---|---|
| 1.1/1.2 Record additive fields | `TestRecord_BackwardCompat_OldShapeRunJSONStillLoads` + `TestRecord_NewFields_RoundTripUnchanged` fail to compile (`PickIndex` etc. undefined) | Fields added to `Record`, `go test` green | None needed |
| 1.3/1.4 `List()` | `TestWriter_List_ReturnsNewestFirstAndSkipsMalformed` + `TestWriter_List_NoRunsDirReturnsEmpty` fail to compile (`w.List` undefined) | `Writer.List()` implemented, green | None needed |
| 1.5/1.6 `Load()` | `TestWriter_Load_ReturnsPersistedRecord` + `TestWriter_Load_MissingRunIDErrors` fail to compile | `Writer.Load(runID)` implemented (delegates to `readRecord`), green | None needed |
| 1.7/1.8 `Save()` | `TestWriter_Save_UpsertsRunJSONAndForcesSchemaVersion` fails to compile | `Writer.Save(rec) error` implemented, green | None needed |
| 1.9/1.10 `selectPruneCandidates` | `TestSelectPruneCandidates` (6 sub-cases) fails to compile (`selectPruneCandidates` undefined) | Pure function in `retention.go`, all 6 sub-cases green | Simplified guard clauses (`i < keepLast` / `age <= keepDays*24h` need no explicit `>0` guards — `keepLast=0`/`keepDays=0` fall out naturally) |
| 1.11/1.12 `Prune()` | `TestWriter_Prune_RemovesOnlyRunsOutsideRetentionWindow` + `TestWriter_Prune_NeverTouchesArbitraryPaths` fail to compile | `Writer.Prune` implemented (delegates to `List` + `selectPruneCandidates` + `os.RemoveAll` scoped to `runDir(runID)`), green | None needed |
| 2.1/2.2 config bounds | `TestConfig_Validate` negative-keepLast/keepDays sub-cases + `TestConfig_Validate_RunsBoundsIdentifyField` fail (assertion: `err == nil`) | Bounds checks added to `Validate()`, green | None needed |
| 3.1/3.2 `derivePickIndex` | `TestDerivePickIndex` (6 sub-cases) fails to compile | Pure function in `update.go`, all 6 sub-cases green, **empirically pinned against real git in 3.5/3.6** | None needed |
| 3.3/3.4 `onBranchCreated` persistence | `TestOnBranchCreated_PersistsInitialRunRecord` + `TestOnBranchCreated_NilRunsWriterIsANoOp` fail to compile | `onBranchCreated` generates `m.runID` + best-effort `Runs.Save`, green | None needed |
| 3.5/3.6 pick-index persistence (load-bearing, real git) | `TestOnPickDone_PersistsPickIndex_RealCherryPickSequencer` (2 sub-cases, real temp-repo cherry-picks) fails to compile, then fails assertion (`PickIndex` always 0) before wiring | `onPickDone`'s conflict branch computes `derivePickIndex` + `saveRunProgress`, both real-git sub-cases green (`PickIndex==1` for 2-commit conflict-on-first; `PickIndex==2` for 3-commit conflict-on-second) | None needed |
| 3.7 `validateCmd` fallback | `TestValidateCmd_FallbackRunIDWhenEmpty` — passed immediately (pre-existing behavior, made explicit/locked) | N/A (documents existing behavior before refactor) | None needed |
| 3.8 `validateCmd` reuse/merge | `TestValidateCmd_ReusesExistingRunID_MergesJobIDAndPhase` fails (`runID` mismatch — old code always re-derived) | `validateCmd` branches on `m.runID`: reuse via `Load`+mutate+`Save` when set, fallback via `Create` when empty; `TestModel_ValidationStart_PersistsRunOnJobId` (HU-010) + full `cancel_confirm_test.go` (HU-012) suite rerun green | None needed |
| 3.9/3.10 `onVerifyDone`/`onAborted` Phase persistence | `TestOnVerifyDone_PersistsPhaseDone` + `TestOnAborted_PersistsPhaseAborted` fail to compile | Both handlers call `saveRunProgress` setting `Phase="done"`/`"aborted"`, green | Extracted shared `saveRunProgress(mutate func(*runs.Record))` helper (Load-then-Save merge) reused by `onPickDone`/`onVerifyDone`/`onAborted` |

## Work Unit Evidence

### Unit 1 — `internal/runs`: additive `Record` + `List`/`Load`/`Save`/`Prune`
- Focused test command and exact result: `go test ./internal/runs/...` → `ok deploydeck/internal/runs 0.646s` (25 tests, all pass, includes backward-compat + threat-matrix guards)
- Runtime harness: `t.TempDir()` run.json/prune fixtures (real filesystem I/O), no git/org — N/A for a real external-process harness by design (pure file persistence layer)
- Rollback boundary: revert `internal/runs/writer.go`, `internal/runs/retention.go`, `internal/runs/writer_test.go`, `internal/runs/retention_test.go`; no dependents yet at this point in the batch (commit is self-contained)

### Unit 2 — `internal/config`: `KeepLast`/`KeepDays` bounds
- Focused test command and exact result: `go test ./internal/config/...` → `ok deploydeck/internal/config 0.313s`
- Runtime harness: N/A — pure struct validation, no I/O boundary
- Rollback boundary: revert `internal/config/validate.go` + `internal/config/validate_test.go`; independent of every other unit

### Unit 3 — Run-creation-at-branch, pick-index formula, `validateCmd` compat
- Focused test command and exact result: `go test ./internal/app/... -run 'TestDerivePickIndex|TestOnBranchCreated|TestOnPickDone|TestOnVerifyDone|TestOnAborted|TestValidateCmd|TestCommitSHAs'` → all PASS (see full log below)
- Runtime harness command/scenario and exact result: `go test ./internal/app/... -run TestOnPickDone_PersistsPickIndex_RealCherryPickSequencer -v` → PASS, 2.99s, REAL temp git repo + real `git cherry-pick` (both sub-cases green, empirically confirming design.md's locked "+1" formula)
- Rollback boundary: revert `internal/app/update.go`, `internal/app/commands.go`, `internal/app/flow.go` (the `commitSHAs` addition), `internal/app/run_persistence_test.go`, `internal/app/pick_index_e2e_test.go`, and the two new tests appended to `internal/app/delta_validation_test.go`; `TestModel_ValidationStart_PersistsRunOnJobId` (HU-010) and the full `cancel_confirm_test.go` suite (HU-012) prove no regression

## Full-suite verification (end of batch)

```
$ go build ./...                  # clean
$ go vet ./...                    # clean
$ gofmt -l .                      # clean (no output)
$ go test -race ./...
ok  	deploydeck/cmd/deploydeck	5.647s
ok  	deploydeck/internal/app	5.739s
ok  	deploydeck/internal/config	1.907s
ok  	deploydeck/internal/delta	(cached)
ok  	deploydeck/internal/exec	(cached)
ok  	deploydeck/internal/git	47.904s
ok  	deploydeck/internal/prereq	(cached)
ok  	deploydeck/internal/runs	2.179s
ok  	deploydeck/internal/salesforce	2.426s
```

`internal/app`'s `TestApp_NeverImportsExecSeam` (boundary_test.go) reconfirmed green — `internal/app` still never execs directly.

## Deviations from design.md

1. **`Writer.Save` signature**: design.md and tasks.md both specify `func (w *Writer) Save(rec Record) (string, error)`. The orchestrator's batch instructions explicitly simplified this to `Save(rec Record) error` (upsert only, no dir-path return) — no caller in this batch's scope needs the created/updated dir path from a progress upsert (unlike `Create`, whose returned dir feeds `validateDoneMsg.runDir` for potential display). Implemented as instructed; flagged here since it diverges from the authoritative design doc. If a later phase needs the dir path from `Save`, this is a compatible, additive signature change (add a second return value) at that time.
2. **`validateCmd` merge semantics**: the batch prompt described the merge as "JobID/Status=validating"; the actual `Phase` enum (per design.md's `Record` doc comment: `cherry-pick|git-conflict|validating|done|aborted`) uses `Phase`, not `Status`, for the value `"validating"`. Implemented as `rec.Phase = "validating"` while `rec.Status` keeps its original HU-010 meaning (`"Queued"`, `"InProgress"`, terminal SF statuses) — this preserves both fields' distinct, pre-existing semantics rather than conflating them.

No other deviations — Phase 1-3 implementation matches design.md's architecture decisions, locked pick-index formula, and file-changes table.

## Risks / Notes for next batch

- Phase 4 (resume-detection at `onPrereqDone`) is NOT started. `resumeDetectMsg`/`resumeDetectCmd`/`onResumeDetect`/`resumeInto` all remain to be implemented.
- The `Model` does not yet carry `runs []runs.Record` / `runsCursor` (Phase 5 `StateRunHistory` fields) — those arrive with Phase 5.
- `onPickDone`'s EMPTY-pick branch (auto-skip) does not persist progress in this batch (only the conflict branch does, per task 3.6's explicit scope). This is consistent with the empty-pick case transitioning immediately to the next pick without a user-visible stop, but should be revisited if Phase 4/5 resume needs pick-progress visibility mid-auto-skip.

## Task Checklist (cumulative — Phases 1-3 complete, Phase 4+ pending)

See `openspec/changes/run-history/tasks.md` for the authoritative, up-to-date checklist (`[x]` through task 3.10; `[ ]` from task 4.1 onward).
