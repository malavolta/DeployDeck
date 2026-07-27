# Archive Report: HU-013 — Run History, Retention & Resume

**Change**: `run-history`  
**Archived**: 2026-07-27  
**Status**: COMPLETE — 49/49 tasks passed, all success criteria verified, review findings remediated

---

## What Shipped

HU-013 closes the crashability and operational-continuity gaps introduced by HU-010/HU-011 (minimal run writer only):

### Delivered Capabilities
1. **Run history browsing** (`StateRunHistory` screen): displays past runs newest-first with ticket, target org, status, and date; detail panel shows branch, commit count, and delta package path.
2. **Run retention** (`deploydeck runs prune`): configurable policy (`keepLast` count OR `keepDays` age, union) with a CLI command to apply it and config validation for bounds.
3. **Startup resume-detection**: after prerequisites pass, detects resumable runs (in-progress cherry-pick OR non-terminal `jobId`) and offers to continue; declining proceeds to normal flow; nothing resumable skips the prompt.
4. **Resume-by-jobId re-attach**: resumed runs with non-terminal `jobId` re-attach `deploy report` polling at the configured interval into `StateValidationPolling`.
5. **Resume of open cherry-pick conflict**: resumed runs in `CherryPickConflict` phase route directly to `StateCherryPickConflict` with rehydrated ticket, pick N/M, and current commit.
6. **Resync on divergence**: when a persisted conflict record no longer matches the real repo (externally resolved/aborted), the record is resynced before offering resume.

---

## Specs Changed

### New Specs (Created)
| Spec | Purpose |
|------|---------|
| `openspec/specs/run-history/spec.md` | History browse screen (list, detail, Enter-resume, decline routing) |
| `openspec/specs/run-retention/spec.md` | Retention policy (`keepLast` OR `keepDays`), `runs prune` CLI, config bounds |
| `openspec/specs/run-resume/spec.md` | Startup detection, offer/decline, resync, direct routing to existing states (no `StateSuspended`) |

### Modified Specs (Merged Deltas)
| Spec | Changes |
|------|---------|
| `openspec/specs/run-persistence/spec.md` | Added: Additive `Record` growth (`PickIndex`, `PickTotal`, `CurrentCommit`, `Phase`, `Commits[]`, backward-compat), `List()`/`Load()`/`Prune()` methods; replaced deferred-to-HU-013 note with cross-reference to new capabilities |
| `openspec/specs/cherry-pick/spec.md` | Added: Resumed entry accepts rehydrated conflict context (ticket, pick N/M); replaced deferral note with `run-resume` pointer |
| `openspec/specs/validation-progress/spec.md` | Added: Re-attach polling to non-terminal `jobId` on later launch; kept existing "exit leaves job active/resumable" invariant intact |

---

## Architecture Decisions

1. **Additive `Record` with NO schema bump**: new fields (`PickIndex`, `PickTotal`, `CurrentCommit`, `Phase`, `Commits[]`) use Go zero-defaults (backward-compatible). Older `run.json` files load cleanly, new fields zero-valued. Forward-readable contract. Verified by `TestRecord_BackwardCompat_OldShapeRunJSONStillLoads`.

2. **Pick-index formula locked and empirically verified**: `PickIndex = PickTotal - SequencerRemaining + 1` (sequencer includes current commit). Formula pinned against REAL `git cherry-pick` sequencer state in 2-commit and 3-commit conflict scenarios (`TestOnPickDone_PersistsPickIndex_RealCherryPickSequencer`).

3. **Retention semantics (union, not intersection)**: a run is KEPT if `CreatedAt` is within most-recent `keepLast` runs OR age ≤ `keepDays`; pruned only if OUTSIDE BOTH. Implemented as pure `selectPruneCandidates(records, keepLast, keepDays, now)` function. Covered by 6 table-test sub-cases.

4. **NO literal `StateSuspended`**: resume routes DIRECTLY into `StateCherryPickConflict` or `StateValidationPolling` with rehydrated `Model` fields. Reuses existing conflict and polling logic; no new intermediate state or termination path.

5. **Resume rehydrates directly into existing screens**: `StateRunHistory` offers resume; accepting calls `resumeInto(rec)` which computes/rehydrates:
   - **Conflict branch**: live `PickIndex` via `derivePickIndex(pickTotal, RepoState)`, selected commits from `rec.Commits`, then routes to `StateCherryPickConflict`.
   - **Polling branch**: `jobId`/`runID`/`pollCtx`/`reportCmd` from `rec`, then routes to `StateValidationPolling`.

6. **Resync guards conflict stalenesss**: before computing the resumable set, runs with `Phase="git-conflict"` but no real `CHERRY_PICK_HEAD` are resynced (`Phase` set to `"aborted"` and saved), eliminating the stale-screen risk.

7. **Zero new execution seams**: resume-detection uses `git.Service.RepoState` + `salesforce.Client.ReportDeploy` (existing). Resume routing uses existing state/view logic. `internal/app` never execs directly (`boundary_test.go` stays green).

---

## Key Review Findings (Remediated)

Two bugs found by adversarial review and fixed under strict TDD:

### HOLE B (MEDIUM) — Resume rehydration gap
**Issue**: resuming a cherry-pick did not rehydrate `m.plan.SelectedCommits`, causing:
- Pick N/M counter to reset (after continue, showed `1 of 3` instead of `2 of 3`)
- Post-pick verification to be skipped (no selected commits means no verification)

**Fix**: added pure `rehydrateSelectedCommits(rec.Commits) []git.DiscoveredCommit` in `resumeInto`'s conflict branch. Reconstructs minimal SHA-only commits from persisted record. `len()` feeds `onPickDone`/`derivePickIndex`; each SHA feeds `VerifyPromotedContent`.

**Verified**: `TestResume_ContinueToNextConflict_PreservesPickProgress` (real double-conflict cherry-pick; resume → continue → next conflict, pick N/M survives) and `TestResume_Completion_RunsPostPickVerification` (resume → wrong resolution → post-pick verify flags partial promotion).

### HOLE A (LOW) — Startup resume-detection race
**Issue**: a resumable run detected while `m.state != StateTicketInput` hijacked the state machine, discarding in-progress user selections (e.g., ticket input mid-type).

**Fix**: (1) guard `onResumeDetect` with `if m.state != StateTicketInput { return m, nil }`; (2) bound `resumeDetectCmd` with 5-second context timeout, degrading to normal flow on slow git.

**Verified**: `TestOnResumeDetect_IgnoredOffTicketInput` (resumable AND nothing-resumable cases both no-op when not on ticket-input screen).

---

## Test Posture

### Unit Testing
- **Run persistence layer** (`internal/runs/`): 25 tests covering backward-compat, `List()`/`Load()`/`Save()`/`Prune()`, `selectPruneCandidates` with 6 retention sub-cases (count, age, boundary, both-false, zero bounds, empty).
- **Config bounds** (`internal/config/`): negative `keepLast`/`keepDays` rejection with field identification.
- **Pick-index derivation** (`derivePickIndex`): 6 sub-cases (pin N of M at conflict-on-first/second, clamp edges, single-pick, sequence-complete).

### Integration Testing (Real Git)
- `TestOnPickDone_PersistsPickIndex_RealCherryPickSequencer`: 2-commit conflict-on-first (verifies `PickIndex==1`), 3-commit conflict-on-second (verifies `PickIndex==2`).
- `TestOnBranchCreated_PersistsInitialRunRecord`: run record created at branch creation with ticket/pickTotal/commits/phase.
- `TestResume_RealInProgressCherryPick_RoutesToConflict`: resume flow end-to-end (in-progress repo → startup → resume offered → Enter → conflict screen with rehydrated context).
- `TestResume_Resync_ExternallyResolved`: repo state divergence (persisted conflict, clean repo after external abort → resynced, not offered).
- `TestResume_ByJobId_ReattachesPolling`: resume with non-terminal `jobId` → polling re-attaches and continues from prior state.

### E2E (CI-Safe: `fs+temp+sf-fake`, No Org)
- `TestE2E_RunHistory_SeedsOfferDeclineResume`: full user journey — seeds conflicted run, resume-eligible run, terminal run; drives startup → offer → decline → history list → Enter → resume → polling/conflict → exit.
- `TestRunsPruneCmd_EndToEnd`: real `.deploydeck/runs/` fixture, `deploydeck runs prune` invoked via cobra end-to-end, exact dirs removed/kept.
- Optional real-org E2E (`[~]` — not mandated): covered by CI-safe path `TestResume_ByJobId_ReattachesPolling`.

### Coverage Summary
- **49/49 tasks** all `[x]` (7.3 opt-in E2E marked `[~]`, not mandated; its code path covered by CI-safe `sf-fake` polling).
- **All 6 success criteria** checked off with their proving tests.
- **`go test -race ./...`** green across all packages.
- **`go vet`, `gofmt`** clean.
- **`boundary_test.go`** (`TestApp_NeverImportsExecSeam`) green — no new exec seam introduced.

---

## Out of Scope / Future

The following are explicitly OUT of this change and owned by future histories:

| Historia | Description |
|----------|-------------|
| HU-014 | Push/PR (`--pr` flag), PR auto-comment with delta summary. |
| HU-016 | Re-promote (`SourceRunID` field, re-attempt with a prior run's delta). |
| HU-017 | Branch cleanup (scheduled or manual) — WILL REUSE the `keepLast`/`keepDays` config and `Prune` mechanism. |
| HU-015/018 | Future org/flow enhancements (unfiled). |

---

## Deliverables

### Filesystem Changes
- **Living specs merged** (open-spec):
  - Created: `openspec/specs/run-history/spec.md`, `openspec/specs/run-retention/spec.md`, `openspec/specs/run-resume/spec.md`
  - Modified: `openspec/specs/run-persistence/spec.md`, `openspec/specs/cherry-pick/spec.md`, `openspec/specs/validation-progress/spec.md`
- **Change folder** archived to: `openspec/changes/archive/2026-07-27-run-history/`

### SDD Artifact Traceability
- **Proposal**: `openspec/changes/run-history/proposal.md`
- **Specs**: `openspec/changes/run-history/specs/` (3 new + 3 delta)
- **Design**: `openspec/changes/run-history/design.md`
- **Tasks**: `openspec/changes/run-history/tasks.md` (49/49 complete)
- **Apply Progress**: `openspec/changes/run-history/apply-progress.md` (3 batches: implementation, history/prune/verification, review remediation)

---

## Merge Integrity

All delta specs merged faithfully:
- **run-persistence**: kept all 3 existing requirements, added 4 new (Record growth, List, Load, Prune), replaced deferred note.
- **cherry-pick**: added 1 new (Resumed entry accepts context), kept all 11 existing, replaced deferral note.
- **validation-progress**: added 1 new (Re-attach polling), kept all 9 existing (including "exit leaves job active/resumable" invariant).

No requirements removed. All existing behavior preserved for forward compatibility.

---

## SDD Cycle Complete

The HU-013 change has been:
- **Planned** (proposal + spec + design + tasks)
- **Implemented** (7 work units, strict TDD, all tasks complete)
- **Verified** (49 integration/unit/E2E tests green, review findings remediated)
- **Archived** (specs merged, change folder moved to archive)

The change closes HU-006 reopen/resume and HU-011 resume-by-jobId deferrals. Run history, retention, and resume are now live and operational.
