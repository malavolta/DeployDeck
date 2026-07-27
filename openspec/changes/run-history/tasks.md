# Tasks: HU-013 — Run History, Retention & Resume (`run-history`)

Strict TDD. `[U]` unit, `[I]` integration (temp git repo / temp `.deploydeck/runs/`), `[T]` TUI (`Model.Update()`), `[E2E]` CI-safe `fs+temp+sf-fake`, `[E2E-ORG]` opt-in/best-effort. Capability noted per task.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~1,200–1,650 (prod ~500–650: `runs`, `config`, `app`, `cmd`; tests ~700–1,000) |
| 400-line budget risk | Low (session `review_budget_lines=40000` override per preflight; would read High vs. the generic 400-line default) |
| Chained PRs recommended | No |
| Suggested split | Single PR — 7 work-unit commits, one per phase below |
| Delivery strategy | single-pr |
| Chain strategy | pending |

```text
Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low
```

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `internal/runs`: additive `Record` + `List`/`Load`/`Save`/`Prune` | PR 1 | `go test ./internal/runs/...` | `t.TempDir()` run.json/prune fixtures, no git/org | Revert `writer.go`+`writer_test.go`; no dependents yet |
| 2 | `internal/config`: `KeepLast`/`KeepDays` bounds | PR 1 | `go test ./internal/config/...` | N/A — pure struct validation | Revert `validate.go`+test; independent |
| 3 | Run-creation-at-branch, pick-index formula, `validateCmd` compat | PR 1 | `go test ./internal/app/... -run 'PickIndex\|BranchCreated\|ValidateCmd\|ValidationStart'` | Real temp git repo, 2-/3-commit cherry-picks | Revert `onBranchCreated`/`onPickDone`/`validateCmd` diffs; HU-010/011 tests prove no regression |
| 4 | Resume detection, resync, resume-accept routing | PR 1 | `go test ./internal/app/... -run 'Resume\|Resync'` | Real temp git repo (in-progress + externally-resolved) + FakeRunner | Revert `resumeDetectCmd`/`onResumeDetect`/`resumeInto`; `onPrereqDone` reverts to unconditional `StateTicketInput` |
| 5 | `StateRunHistory` screen | PR 1 | `go test ./internal/app/... -run 'RunHistory'` | `Model.Update()` direct transitions | Revert state const + view/keys; screen unreachable |
| 6 | `deploydeck runs prune` CLI | PR 1 | `go test ./cmd/deploydeck/...` | Real temp `.deploydeck/runs/` fixture via `RunE` | Revert `newRunsCmd` + its `AddCommand` registration |
| 7 | Final verification + CI-safe E2E | PR 1 | `go test -race ./...` | `fs+temp+sf-fake` E2E, no org | N/A — verification only |

## Constraints

- `internal/app` MUST NOT exec directly — `boundary_test.go` holds throughout.
- NO literal `StateSuspended` — resume routes directly into existing states.
- OUT of scope, no tasks: HU-014 push/PR, HU-016 re-promote, HU-017 branch cleanup.

## Phase 1: `internal/runs` extension (run-persistence, run-retention)

- [x] 1.1 `[U]` RED: `writer_test.go` — old-shape `run.json` still `Load`s with zero-valued `PickIndex/PickTotal/CurrentCommit/Phase/Commits`; new-field round trip (write→reload unchanged). (run-persistence: Additive Record Growth)
- [x] 1.2 `[U]` GREEN: extend `Record` in `internal/runs/writer.go` with `Commits []string`, `PickIndex`, `PickTotal int`, `CurrentCommit`, `Phase string`, all `omitempty`, no `SchemaVersion` bump.
- [x] 1.3 `[U]` RED: `List()` — 3 runs w/ different `CreatedAt` return newest-first; a malformed run dir is skipped. (run-persistence: List Runs Newest First)
- [x] 1.4 `[U]` GREEN: implement `Writer.List()` scanning `.deploydeck/runs/*/run.json`, sorted desc by `CreatedAt`.
- [x] 1.5 `[U]` RED: `Load(runID)` — existing run returns `Record`; missing runID errors. (run-persistence: Load A Single Run By ID)
- [x] 1.6 `[U]` GREEN: export `readRecord` as `Writer.Load(runID)`.
- [x] 1.7 `[U]` RED: `Save(rec)` — upserts `run.json`, forces `SchemaVersion1`. (run-persistence, supports 3.x/4.x progress writes)
- [x] 1.8 `[U]` GREEN: implement `Writer.Save(rec Record) error` (upsert; simplified from design's `(string, error)` — no caller needs the dir path from a progress upsert). See apply-progress deviation note.
- [x] 1.9 `[U]` RED: `selectPruneCandidates` table test — kept-by-count-despite-old, kept-by-age-despite-outside-keepLast, pruned-outside-both, boundary `age==keepDays`, `keepLast=0`, empty. (run-retention: all 3 scenarios)
- [x] 1.10 `[U]` GREEN: implement pure `selectPruneCandidates(records, keepLast, keepDays, now) []string`.
- [x] 1.11 `[U]` RED: `Prune` — mixed fixture dir, only outside-both removed, returns removed IDs, others untouched. (run-persistence: Prune Removes Runs Outside The Retention Window)
- [x] 1.12 `[U]` GREEN: implement `Writer.Prune` deleting only `<baseDir>/.deploydeck/runs/<runID>` dirs enumerated by `List()` — never arbitrary paths (threat matrix).

## Phase 2: `internal/config` bounds (run-retention)

- [x] 2.1 `[U]` RED: negative `Runs.KeepLast` rejected; negative `Runs.KeepDays` rejected, each error identifies the field. (run-retention: KeepLast/KeepDays Config Bounds Are Validated)
- [x] 2.2 `[U]` GREEN: add bounds to `internal/config/validate.go`.

## Phase 3: Run-creation-at-branch, pick-index, `validateCmd` compat (run-persistence, cherry-pick)

- [x] 3.1 `[U]` RED: `derivePickIndex` table test — pins `idx==1` (pickTotal=2, SeqRem=2) and `idx==2` (pickTotal=3, SeqRem=2), plus clamp edges (conflict-on-last, no-sequencer single pick, sequence-complete). (design locked formula)
- [x] 3.2 `[U]` GREEN: implement `derivePickIndex(pickTotal int, st git.RepoState) int` = `pickTotal - st.SequencerRemaining + 1`, clamped `[1,pickTotal]`.
- [x] 3.3 `[I]` RED: `onBranchCreated` integration test — asserts `Runs.Save` creates `run.json` immediately at branch creation with `Ticket`/`PickTotal`/`Commits`/`Phase="cherry-pick"`, before any pick runs.
- [x] 3.4 `[I]` GREEN: wire run creation into `onBranchCreated` — generate `m.runID`, `Runs.Save` the initial record.
- [x] 3.5 `[I]` RED (disambiguating): REAL temp-repo cherry-picks — 2-commit conflict-on-first asserts persisted `PickIndex==1`; 3-commit conflict-on-second asserts `PickIndex==2`, via real `git cherry-pick` sequencer state.
- [x] 3.6 `[I]` GREEN: in `onPickDone`'s conflict branch, compute `PickIndex` via `derivePickIndex`, persist `PickIndex/PickTotal/CurrentCommit/Phase="git-conflict"` via `Runs.Save`.
- [x] 3.7 `[U]` RED: `TestValidateCmd_FallbackRunIDWhenEmpty` — `m.runID==""` still derives `ticket-to-target-timestamp` (locks the fallback the existing HU-010 test at `delta_validation_test.go:247-276` depends on).
- [x] 3.8 `[U]` GREEN: refactor `validateCmd` to reuse `m.runID` when set (merge `JobID`/`Phase="validating"` into the existing `Save`d record via `Runs.Save`), else fall back to legacy derivation + `Runs.Create`. Rerun `TestModel_ValidationStart_PersistsRunOnJobId` (HU-010) + `cancel_confirm_test.go` (HU-012) — must stay green.
- [x] 3.9 `[I]` RED: after `onVerifyDone`/`onAborted`, persisted `Phase` updates to `"done"`/`"aborted"`.
- [x] 3.10 `[I]` GREEN: add best-effort `Runs.Save` calls in `onVerifyDone`/`onAborted` (mirrors `onReportDone`'s best-effort `AppendReport`).

## Phase 4: Resume-detection at `onPrereqDone` (run-resume)

- [x] 4.1 `[I]` RED: real temp-repo in-progress cherry-pick seed — `resumeDetectCmd` returns `resumeDetectMsg{state, records}` reflecting real `RepoState`+`Runs.List()`.
- [x] 4.2 `[I]` GREEN: implement `resumeDetectMsg`/`resumeDetectCmd` (composes `Git.RepoState`+`Runs.List`); `onPrereqDone` dispatches it instead of unconditional `StateTicketInput`.
- [x] 4.3 `[T]` RED: matching in-progress pick + run (`Commits` contains `CurrentSHA`) → routes to `StateRunHistory` pre-selected. (run-resume: In-progress cherry-pick offers resume)
- [x] 4.4 `[T]` GREEN: implement `onResumeDetect` — resumable set (matching in-progress, or non-terminal `JobID`) → `StateRunHistory` pre-selected; none → `StateTicketInput`. (run-resume: No resumable run skips the prompt)
- [x] 4.5 `[I]` RED: real temp repo, `Phase="git-conflict"` record but no `CHERRY_PICK_HEAD` → corrected via `Save`, not offered as resumable. (run-resume: Resync When The Repo No Longer Matches...)
- [x] 4.6 `[I]` GREEN: in `onResumeDetect`, resync stale conflict records (`Runs.Save`) before computing the resumable set.
- [x] 4.7 `[T]` RED: resume-accept routing table test — conflict-phase run → `StateCherryPickConflict` rehydrated (ticket/pickIndex/pickTotal/runID, `repoStateCmd`+`tickCmd`); non-terminal-jobId run → `StateValidationPolling` re-armed (jobID/runID/pollCtx/reportCmd). (run-resume: both "Accepted..." scenarios; cherry-pick + validation-progress deltas)
- [x] 4.8 `[T]` GREEN: implement shared `resumeInto(rec)` routing both branches; terminal-jobId runs excluded upstream from the resumable set. (validation-progress: Resumed job already terminal is not offered)

## Phase 5: `StateRunHistory` browse screen (run-history)

- [x] 5.1 `[T]` RED: `viewRunHistory` — lists runs newest-first (ticket/target/status/date) from `m.runs`; empty list renders no rows without error. (run-history: History Screen Lists Runs Newest First)
- [x] 5.2 `[T]` GREEN: add `StateRunHistory` const, `Model.runs`/`runsCursor` fields, `viewRunHistory` per `MOCKUPS_TUI.md:369-388`.
- [x] 5.3 `[T]` RED: row w/ `jobId` shows validation status; row w/o `jobId` shows last step reached. (run-history: Row Shows Progress Reached)
- [x] 5.4 `[T]` GREEN: derive per-row progress label from `Record.JobID`/`Status`/`Phase`.
- [x] 5.5 `[T]` RED: selecting a run shows branch/commit-count/package-path detail. (run-history: Detail View On Selection)
- [x] 5.6 `[T]` GREEN: render selected-run detail panel from `Record` fields.
- [x] 5.7 `[T]` RED: `Enter` on a resumable run calls `resumeInto`; `Enter` on a terminal-status run is a no-op (detail stays shown); `d` toggles detail; `↑/↓` moves `runsCursor`; `q`/`esc` declines → `StateTicketInput`. (run-history: Enter On A Resumable Run...; run-resume: User declines the resume offer)
- [x] 5.8 `[T]` GREEN: implement `keyRunHistory` (`keys.go`) wiring Enter/d/↑↓/q per mockup line 386.

## Phase 6: `deploydeck runs prune` CLI (run-retention)

- [x] 6.1 `[U]` RED: fixture runs dir w/ >`keepLast` runs, some >`keepDays`, prune removes only outside-both. (run-retention: Running the command prunes only outside-window runs)
- [x] 6.2 `[U]` GREEN: implement `newRunsCmd` (`cmd/deploydeck/main.go`) → `runs prune` composing `config.Load`+`runs.NewWriter(dir).Prune`, registered via `root.AddCommand`.
- [x] 6.3 `[I]` RED: real temp `.deploydeck/runs/` fixture, invoke `RunE` end-to-end, assert exact dirs removed/kept.
- [x] 6.4 `[I]` GREEN: surface non-zero exit on `Prune` error, mirroring `newDoctorCmd`.

## Phase 7: Final verification

- [x] 7.1 `[E2E]` RED: CI-safe `fs+temp+sf-fake` (no org) — seed: conflicted run w/ `CHERRY_PICK_HEAD`; resync case (conflict record, clean repo); run w/ `jobId`; run w/o job; >`keepLast`/>`keepDays` runs. Drives startup→offer→resume→(conflict|polling)/decline + `runs prune`. (`TestE2E_RunHistory_SeedsOfferDeclineResume` for the TUI story; `TestResume_RealInProgressCherryPick_RoutesToConflict`/`TestResume_Resync_ExternallyResolved`/`TestResume_ByJobId_ReattachesPolling` for the individual seeds; `TestRunsPruneCmd_EndToEnd` for retention.)
- [x] 7.2 `[E2E]` GREEN: close any gaps surfaced by 7.1 (expect none beyond wiring already built). No gaps — the E2E passed against the existing Phase 4-6 wiring.
- [~] 7.3 `[E2E-ORG]` opt-in, best-effort: real-org `ReportDeploy` re-attach behind `DEPLOYDECK_E2E_ORG`, never mandated. Not mandated for CI; the CI-safe `sf-fake` re-attach (`TestResume_ByJobId_ReattachesPolling`) fully exercises the same code path without an org.
- [x] 7.4 Run `go test -race ./...`, `go vet ./...`, `gofmt -l .` — clean; rerun `boundary_test.go`. All clean under `-race -count=1`; `TestApp_NeverImportsExecSeam` reconfirmed green.
- [x] 7.5 Check off every `proposal.md` Success Criteria line against passing tests. All six checked off with their proving tests.
