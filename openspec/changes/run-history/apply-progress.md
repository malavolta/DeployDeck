# Apply Progress: HU-013 — Run History, Retention & Resume (`run-history`)

**Batch 1** (Phases 1-3) and **Batch 2** (Phases 4-7) — ALL phases complete. Batch 1 scope: `internal/runs` extension, `internal/config` bounds, run-creation-at-branch + pick-index + `validateCmd` compat. Batch 2 scope: startup resume-detection + resync (Phase 4), `StateRunHistory` browse screen (Phase 5), `deploydeck runs prune` CLI (Phase 6), final verification + CI-safe E2E (Phase 7).

Strict TDD Mode: enabled. RED confirmed (compile failure or failing assertion) before every GREEN implementation, per task, across both batches.

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

## Risks / Notes (carried forward from Batch 1)

- `onPickDone`'s EMPTY-pick branch (auto-skip) does not persist progress (only the conflict branch does, per task 3.6's explicit scope). Batch 2 resume recomputes `PickIndex` LIVE from `RepoState` via `derivePickIndex` — never the persisted value — so the empty-pick/auto-skip branch not persisting `PickIndex` is harmless: `resumeInto` reads the live sequencer, not the stale record. Confirmed by `TestResume_RealInProgressCherryPick_RoutesToConflict` (3-commit, conflict-on-second → live `pickIndex==2`).

---

# Batch 2 — Phases 4-7 (resume-detection, run-history screen, `runs prune`, verification)

Strict TDD Mode: enabled. RED (compile failure or failing assertion) confirmed before each GREEN, per task.

## TDD Cycle Evidence (Batch 2)

| Task | RED (test written first) | GREEN (implementation passes) | REFACTOR |
|---|---|---|---|
| 4.1/4.2 `resumeDetectCmd` | `TestResumeDetectCmd_RealInProgressCherryPick` + `TestResumeDetectCmd_NilDepsIsNoOp` fail to compile (`resumeDetectCmd`/`resumeDetectMsg` undefined) | `resumeDetectMsg` + `resumeDetectCmd` (composes `Git.RepoState`+`Runs.List`, nil-dep guard) in commands.go; `onPrereqDone` dispatches it; both green | None |
| 4.3/4.4 `onResumeDetect` routing | `TestOnResumeDetect_RoutesToRunHistoryWhenResumable` (4 sub-cases) fails to compile (`resumeDetectMsg`/`StateRunHistory` undefined) | `onResumeDetect` + `firstResumable`/`isResumable`/`containsSHA` helpers; resumable → `StateRunHistory` pre-selected, none → `StateTicketInput`, error → normal flow; green | None |
| 4.5/4.6 resync | `TestOnResumeDetect_ResyncsStaleConflictRecord` fails (record still `git-conflict`) | `reconcileStaleRuns` (repo clean + cherry-pick phase → `Save` Phase=aborted) before the resumable set; green | None |
| 4.7/4.8 `resumeInto` | `TestResumeInto_RoutesConflictAndPolling` (3 sub-cases) fails to compile (`resumeInto` undefined) | `resumeInto` — conflict-phase+live-HEAD → `StateCherryPickConflict` (pickIndex recomputed LIVE, not persisted 99) + `repoStateCmd`/`tick`; non-terminal jobId → `StateValidationPolling` re-armed (jobID/runID/pollCtx/reportCmd, mirrors `onValidateDone`); terminal → no-op; green | None |
| 4.x integration (real git + fs + sf-fake) | `TestResume_RealInProgressCherryPick_RoutesToConflict`, `TestResume_Resync_ExternallyResolved`, `TestResume_ByJobId_ReattachesPolling` fail to compile/assert | All three green against real `git cherry-pick` sequencer + `Runs.List` + FakeRunner `deploy report` | None |
| 5.1/5.2 `viewRunHistory` list | `TestViewRunHistory_ListsRunsNewestFirst` fails (compile after Phase-4 minimal view trimmed) | Enriched `viewRunHistory` — rows (date/ticket/target/progress/jobId) newest-first, empty renders no rows; green | Phase-4 committed a minimal offer-surface view; Phase 5 enriched it tests-first (trim→RED→enrich→GREEN) to keep strict TDD honest per phase |
| 5.3/5.4 progress cell | `TestViewRunHistory_RowShowsProgressReached` fails to compile (`runProgressLabel` undefined) | `runProgressLabel` — jobId→Status, jobless→Phase; green | None |
| 5.5/5.6 detail panel | `TestViewRunHistory_DetailOnSelection` fails (no branch/commits/package rendered) | Selected-run detail (branch via `RenderBranchName`, commit count, `runPackagePath`); green | None |
| 5.7/5.8 `keyRunHistory` nav | `TestKeyRunHistory_Navigation` (5 sub-cases) fails (↑/↓/`d` not wired) | `keyRunHistory` up/down/`d` added alongside Enter-resume/`q`-decline; green | None |
| 6.1/6.2 `runs prune` core | `TestRunsPrune_RemovesOnlyOutsideWindow` + `TestRunsCmd_Registered` fail to compile (`runPrune`/`newRunsCmd` undefined) | `newRunsCmd`→`runs prune` (composes `config.Load`+`runs.NewWriter().Prune`), registered via `root.AddCommand`; `runPrune` testable core; green | Extracted `runPrune(w, dir)` so `RunE` only supplies `os.Getwd()` — testable end-to-end without process-level cwd tricks |
| 6.3/6.4 end-to-end + non-zero exit | `TestRunsPruneCmd_EndToEnd` (via `t.Chdir`+cobra `Execute`) + `TestRunsPrune_ConfigError_NonZeroExit` fail | `RunE` resolves cwd → `runPrune`; a load/prune failure returns a non-zero exit mirroring `newDoctorCmd`; green | None |
| 7.1/7.2 consolidated E2E | `TestE2E_RunHistory_SeedsOfferDeclineResume` fails to compile until wiring exists | Seeds done/stale-conflict/jobId runs on a real clean repo + fs + sf-fake; drives startup→offer→resync→list→decline→resume-by-jobId; green with NO gaps beyond Phase 4-6 wiring | None |

## Work Unit Evidence (Batch 2)

### Unit 4 — Startup resume-detection + resync + `resumeInto` (commit `feat(app): startup resume-detection and resync`)
- Focused test command and exact result: `go test ./internal/app/ -run 'Resume|OnResumeDetect|ResumeInto|ResumeDetect'` → PASS (routing units + 3 real-git/fs/sf-fake integration tests)
- Runtime harness command/scenario and exact result: `go test ./internal/app/ -run 'TestResume_RealInProgressCherryPick_RoutesToConflict|TestResume_Resync_ExternallyResolved|TestResume_ByJobId_ReattachesPolling' -v` → PASS; REAL `git cherry-pick` sequencer (live `PickIndex` recompute), external `git cherry-pick --abort` resync, and FakeRunner `deploy report` re-attach — no org
- Rollback boundary: revert `resumeDetectMsg`/`resumeDetectCmd` (commands.go), `onResumeDetect`/`resumeInto`/`reconcileStaleRuns`/`isResumable`/`firstResumable`/`containsSHA`/`isCherryPickPhase` + `onPrereqDone` dispatch + `onPickDone` pickIndex (update.go), `StateRunHistory` const + Model fields (app.go), `keyRunHistory` + routing (keys.go), `viewConflict` pick-N-of-M + minimal `viewRunHistory` (view.go), `resume_test.go`; `onPrereqDone` reverts to unconditional `StateTicketInput`

### Unit 5 — `StateRunHistory` browse screen (commit `feat(app): run-history browse screen`)
- Focused test command and exact result: `go test ./internal/app/ -run 'RunHistory'` → PASS (list/progress/detail/navigation)
- Runtime harness: `Model.Update()` direct transitions + `View()` rendering — N/A for a real external-process harness (pure TUI state/render layer)
- Rollback boundary: revert the enriched `viewRunHistory` + `runProgressLabel`/`runPackagePath` (view.go) and `keyRunHistory` up/down/`d` (keys.go) + `run_history_test.go`; the minimal Phase-4 offer surface remains functional

### Unit 6 — `deploydeck runs prune` CLI (commit `feat(cmd): runs prune command`)
- Focused test command and exact result: `go test ./cmd/deploydeck/ -run 'RunsPrune|RunsCmd'` → PASS
- Runtime harness command/scenario and exact result: `TestRunsPruneCmd_EndToEnd` invokes the registered cobra command end-to-end (`t.Chdir` + real `deploydeck.yaml` + real `.deploydeck/runs/` fixture via `RunE`→`os.Getwd`→`config.Load`→`Prune`), asserting exact dirs removed/kept — no org
- Rollback boundary: revert `newRunsCmd`/`newRunsPruneCmd`/`runPrune` + the `root.AddCommand(newRunsCmd())` registration (cmd/deploydeck/main.go) + `runs_prune_test.go`

### Unit 7 — Final verification + consolidated CI-safe E2E
- Focused test command and exact result: `go test ./internal/app/ -run 'TestE2E_RunHistory_SeedsOfferDeclineResume' -v` → PASS
- Runtime harness: real clean temp git repo + temp `.deploydeck/runs/` + FakeRunner `deploy report` (fs+temp+sf-fake, no org)
- Rollback boundary: revert `run_history_e2e_test.go` (verification-only; removes no production behavior)

## Full-suite verification (end of Batch 2)

```
$ go build ./...                  # clean
$ go vet ./...                    # clean
$ gofmt -l .                      # clean (no output)
$ go test -race -count=1 ./...
ok  	deploydeck/cmd/deploydeck	5.816s
ok  	deploydeck/internal/app	9.450s
ok  	deploydeck/internal/config	1.640s
ok  	deploydeck/internal/delta	8.266s
ok  	deploydeck/internal/exec	3.328s
ok  	deploydeck/internal/git	50.454s
ok  	deploydeck/internal/prereq	4.301s
ok  	deploydeck/internal/runs	2.968s
ok  	deploydeck/internal/salesforce	2.670s
```

`TestApp_NeverImportsExecSeam` (boundary_test.go) reconfirmed green — the resume/history/prune paths route through `git.Service`/`salesforce.Client`/`runs.Writer` only; `internal/app` still never execs directly.

## Deviations from design.md (Batch 2)

1. **Resume offer surface is `StateRunHistory` (not direct-to-conflict)**: the orchestrator's Phase-4 bullet reads "route to `StateCherryPickConflict`", but design.md's data-flow diagram + tasks 4.3/4.4/4.7 route detection → `StateRunHistory` (offer) → Enter → `resumeInto` → `StateCherryPickConflict`/`StateValidationPolling`. Implemented per the design/tasks (the offer surface), with the integration test driving the full startup→offer→Enter→conflict path so the deliverable ("startup routes to the conflict screen with ticket + pick N of M") is satisfied end-to-end.
2. **`newRunsCmd()` takes no `Deps`** (unlike `newDoctorCmd(deps)`): `runs prune` composes `config.Load` + `runs.NewWriter` directly from the resolved cwd and needs nothing from `Deps`, so the constructor is parameterless. Registered as `root.AddCommand(newRunsCmd())`.

No other deviations — Phases 4-7 match design.md's data flow, locked pick-index formula (recomputed LIVE on resume), retention semantics, and file-changes table.

## Task Checklist (cumulative — ALL phases complete)

`openspec/changes/run-history/tasks.md` is the authoritative checklist: `[x]` through 7.5 (7.3 marked `[~]` — opt-in real-org E2E, not mandated; its code path is covered CI-safe by `TestResume_ByJobId_ReattachesPolling`). All six `proposal.md` Success Criteria checked off with their proving tests.
