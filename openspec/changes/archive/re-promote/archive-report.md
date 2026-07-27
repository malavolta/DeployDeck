# Archive Report: HU-016 — Re-Promote Ticket Between Environments (`re-promote`)

**Date Archived**: 2026-07-28  
**Status**: Complete and Verified  
**Total Tasks**: All 90 tasks (7 phases + 4 remediations) checked  
**Final Verification**: READY TO ARCHIVE (as reported by sdd-verify)  
**Test Result**: Full regression suite green; `go test ./... -race` clean, `go vet` clean, `gofmt` clean

## Executive Summary

HU-016 transforms run history from a passive log into the source of the next promotion, enabling users to efficiently re-promote a ticket from one environment to the next. When a prior run succeeds in environment A, pressing `r` on its history row pre-seeds a new promotion toward environment B: the source is the prior run's environment branch (not a temporary branch), commits are patch-id-mapped across the new environment range, missing commits are warned explicitly, and the new run records a `SourceRunID` link back to the prior run. The implementation introduces a new `re-promotion` capability, extends `run-history` with the `r` key action, extends `run-persistence` with the `SourceRunID` field, and extends `commit-discovery` with next-environment suggestion. All 90 implementation tasks (plus 4 post-review remediations) completed; no PR can be re-promoted without a successful prior run, and all state transitions remain within the existing promotion machinery.

## What Shipped

### 1. Re-Promotion Eligibility & Pre-Seed

**Acceptance Criteria Addressed**: AC1, AC2, AC6

- **Eligibility Gate**: Re-promotion is offered only for prior runs with `Succeeded` or `SucceededPartial` status; `Failed`, `Canceled`, and `Aborted` runs offer no pre-load
- **Entry Point**: Pressing `r` on an eligible row in `StateRunHistory` calls `startRePromoteInto(rec)`, mirroring the structural pattern of `resumeInto`
- **Synchronous Seeding**: Fields `ticket`, `sourceRunID`, and `prelim` (next-environment suggestion) are pre-populated
- **State Transition**: INTERIM `StateCommitDiscovery` is entered while patch-id remap runs asynchronously
- **Implementation**: New `isRePromoteEligible()` gate in `keyRunHistory`, `startRePromoteInto()` and `onRePromoteSeeded()` in `update.go`, new `Model` fields `sourceRunID` and `rePromoteMissing`

### 2. Discovery Source: Prior Run's Environment Branch

**Acceptance Criteria Addressed**: AC1, AC6

- **Source Selection**: For a re-promotion, `origin/<rec.Target>` (the prior run's own environment branch, post-merge) is used directly, bypassing the `SuggestDefaultSource` path
- **Commit Range**: All commits in `origin/<rec.Target>..origin/<nextTarget>` are evaluated for patch-id equivalence against `rec.Commits`
- **Read-Only**: Discovery source is read-only; no temporary branches are created or modified
- **Implementation**: Reuses existing `git.Service.CommitsInRange()` and introduces `RemapCommitsByPatchID()` to compose per-commit `PatchID` lookups over the new range

### 3. Patch-ID Remap & Pre-Checked Selection

**Acceptance Criteria Addressed**: AC3, AC6

- **Commit Mapping**: Each prior-run commit SHA is mapped to its patch-id equivalent in the new environment range
- **Pre-Selection**: Equivalent commits are pre-checked in `StateCommitSelection` and remain fully editable
- **User Control**: Users can toggle any pre-checked item off before continuing, and the edited selection is honored through to plan and branch creation
- **Implementation**: New `git.Service.RemapCommitsByPatchID()` returns a `RemapResult` with `Matched` (pre-checkable) and `Unmatched` (warned) lists; `NewCommitSelectionItems()` pre-checks `Matched` commits

### 4. Explicit Missing-Commit Warning

**Acceptance Criteria Addressed**: AC4

- **Visibility**: When a prior-run commit has no patch-id equivalent in the new range, it is EXPLICITLY warned (never silently omitted)
- **Warning Display**: Missing SHAs are listed in `selectionWarnings` during commit selection, clearly stating which commits are absent from the new origin range
- **Non-Blocking**: The user may proceed with the available (matched) commits, or abort to investigate the gap
- **Robustness**: Malformed SHAs (e.g. leading `-`) and patch-id lookup failures degrade to `Unmatched`, never to a crash
- **Implementation**: `onRePromoteSeeded()` populates `m.rePromoteMissing` from `msg.Unmatched`; `view.go` prepends a warning line listing them

### 5. Next-Environment Suggestion

**Acceptance Criteria Addressed**: AC5, AC6

- **Forward Mirror**: New `git.NextEnvironmentBranch(cfg, currentTarget)` mirrors `SuggestDefaultSource`'s use of configured pipeline order but in the forward direction
- **Default Target**: On `StateTargetSelection`, the next environment in the pipeline is suggested as the default (via `m.prelim`), still user-overridable
- **Fallback**: When the prior environment is last in the pipeline (e.g. `main`/production) or falls outside the fixed order (e.g. `Release/*`), no default is suggested and the user picks manually
- **Implementation**: New `NextEnvironmentBranch()` in `git/source_suggestion.go` reuses `environmentPipelineOrder` and `environmentKeyForBranch` logic; returns `(branch string, ok bool)` with `ok=false` for out-of-range cases

### 6. SourceRunID Linkage On Completion

**Acceptance Criteria Addressed**: AC5

- **Record Field**: New optional `SourceRunID` field in `Record` (`json:"sourceRunId,omitempty"`)
- **Creation-Time Set**: When a re-promotion run is created in `onBranchCreated()`, `SourceRunID` is set to `m.sourceRunID` (the prior run's ID)
- **Backward Compatible**: No `SchemaVersion` bump; older `run.json` files load cleanly with `SourceRunID` zero-valued
- **Persisted**: The link is written to `run.json` at creation and survives reload/resume
- **Implementation**: Single-line addition to the `Record` literal in `onBranchCreated()` and the `runs.Record` struct in `writer.go`

### 7. Degradation: No Prior Run / No Next Environment

**Acceptance Criteria Addressed**: AC5, AC6

- **Manual Fallback**: When a ticket has no prior run, the normal `StateTicketInput` → `StateTargetSelection` flow proceeds with no pre-load
- **End-of-Pipeline**: When re-promoting from the last environment (production or `Release/*` glob), the system degrades to manual target selection (no auto-suggestion) but keeps the ticket pre-filled
- **Routing**: Both cases flow through the unchanged downstream states (`PlanPreview` → `BranchCreation` → cherry-pick → delta → validate)

## Capabilities: 1 New + 3 Modified

| Capability | Action | Details |
|---|---|---|
| `re-promotion` | **Created** | New spec at `openspec/specs/re-promotion/spec.md` — full lifecycle of eligibility, discovery source, commit remap, missing-commit warning, and `SourceRunID` linkage. 7 requirements, each with 1–3 scenarios. |
| `run-history` | **Modified** | Added 1 new requirement (+ 4 scenarios) for the `r` key action initiating re-promotion on eligible terminal-success rows. Existing `Enter`→resume behavior unaffected. |
| `run-persistence` | **Modified** | Added 1 new requirement (+ 3 scenarios) for `SourceRunID` field and its backward-compatible additive growth. Existing `PRUrl`, `PickIndex`, `Phase`, `Commits` fields remain. |
| `commit-discovery` | **Modified** | Added 2 new requirements (+ 7 scenarios) for `NextEnvironmentBranch` forward suggestion and re-promotion source bypass. Existing discovery, equivalence detection, and source-branch enforcement remain. |

## Key Design Decisions

### 1. **Re-Promotion Source = `origin/<rec.Target>`, Not `rec.PromotionBranch`**

- **Rationale**: The temporary `deploy/*` promotion branch is HU-017's to clean up. The durable post-merge home of promoted commits is the environment branch itself. Using `origin/<rec.Target>` ensures we discover the actual canonical state of the environment after the prior run completed.
- **Benefit**: Patch-id remapping occurs over the canonical env-to-next-env range, not a stale temporary branch.

### 2. **Patch-ID Remap in `git.Service`, Not `internal/app`**

- **Rationale**: `internal/app` must never execute shell commands directly (enforced by `boundary_test.go`). The remap is a git operation and belongs in `git.Service`.
- **Implementation**: New `RemapCommitsByPatchID()` method composes `CommitsInRange` + per-commit `PatchID` lookups, returning structured `RemapResult`.

### 3. **`SourceRunID` Set at Run Creation, Not Post-Hoc**

- **Rationale**: Unlike `PRUrl` (which is unknown until `gh pr create` returns), the source run ID is known the instant the user picks it in history. Threading it at creation time (like `Ticket` and `Commits`) avoids a second write operation.
- **Timing**: Written in `onBranchCreated()` Record literal; no separate `MarkSourceRun()` method needed.

### 4. **Pre-Checked Selection Is Fully Editable**

- **Rationale**: The reused commits are contextual suggestions, not mandatory. Users may need to toggle items off if the new environment branch has evolved (e.g. a commit was reverted, or a dependency was added).
- **Implementation**: `NewCommitSelectionItems()` marks matched commits as initially checked; the existing `ToggleSelection()` logic keeps them editable.

### 5. **Explicit Warning for Missing Commits**

- **Rationale**: Silent omission would hide gaps in the re-promotion (spec AC4). Users must know which prior commits could not be found in the new range, so they can investigate or consciously proceed with the available set.
- **Implementation**: `rePromoteMissing` list is populated during `onRePromoteSeeded()` and prepended to `selectionWarnings`.

### 6. **Eligibility Gate Requires Non-Empty Commits List**

- **Rationale**: A terminal-success run with zero commits has nothing to re-promote; the gate also checks `len(rec.Commits) > 0` to prevent redundant processing.
- **Edge Case Handling**: Covered by adversarial review remediation R4a.

### 7. **Fresh Discovery Object on Re-Promotion**

- **Rationale**: Prevents sibling fields (`Alternatives`, `CandidateBranches`) from an earlier in-session discovery from leaking into the re-promotion's commit selection view.
- **Implementation**: `onRePromoteSeeded()` assigns a fresh `DiscoverResult{OrderedCommits: msg.Matched}` instead of mutating a single field.

## Review Findings & Remediations

Adversarial review identified 4 findings (1 HIGH, 1 MED, 2 LOW), all remediations applied and verified:

### R1 [HIGH]: No Remap When Next Environment Does Not Exist

**Issue**: `startRePromoteInto` would fire `rePromoteRemapCmd` even when `git.NextEnvironmentBranch` returned `ok=false`, attempting to build a malformed git range (`origin/..origin/<Target>`) and failing to `StateError`.

**Fix Applied**: Degradation logic added — when `ok=false`, the flow falls back to the manual path (ticket pre-filled, `preliminaryTarget` set, `discoverCmd` fired), mirroring the normal `keyTicket` → `Enter` branch. `m.sourceRunID` is left unset (no well-defined provenance).

**Verification**: RED test `TestStartRePromoteInto_NoNextEnvironment_DegradesToManualFlowWithTicketPrefilled` (unit) and e2e variant confirm the git range error is caught before triggering.

### R2 [MED]: Stale Re-Promotion State Linkage

**Issue**: If a user pressed `r` to start re-promotion, then pressed `Esc` to cancel, then selected a different ticket and pressed `Enter`, the stale `m.sourceRunID` from the abandoned re-promotion would link the new run to the wrong prior run.

**Fix Applied**: `keyTicket` → `Enter` branch now resets `m.sourceRunID = ""` and `m.rePromoteMissing = nil`, preventing stale linkage.

**Verification**: RED test `TestKeyTicket_Enter_ResetsStaleRePromoteState` confirms the reset occurs.

### R3 [LOW]: Missing-Commits Warning Reset

**Issue**: Covered by R2's fix; no separate code needed.

### R4a [LOW]: Empty Commits List Edge Case

**Issue**: A terminal-success run with zero commits is not a valid reuse source and should be ineligible.

**Fix Applied**: `isRePromoteEligible` now also checks `len(rec.Commits) > 0`.

**Verification**: RED test `TestIsRePromoteEligible_ZeroCommits_IsIneligible`.

### R4b [LOW]: Stale Discovery Sibling Fields

**Issue**: `onRePromoteSeeded` was mutating a single field on a prior `m.discovery`, allowing `Alternatives` and `CandidateBranches` from an earlier discovery to leak into the re-promotion's view.

**Fix Applied**: Assign a fresh `DiscoverResult{OrderedCommits: msg.Matched}` instead of mutating.

**Verification**: RED test `TestOnRePromoteSeeded_ReplacesDiscoveryFresh_NoStaleSiblingFields`.

## Test Posture

### Unit Tests

- **`internal/runs/writer_test.go`**: `SourceRunID` round-trip, backward-compat with prior `run.json` (zero-valued on load)
- **`internal/git/source_suggestion_test.go`**: `NextEnvironmentBranch` table-driven (INT→UAT, UAT→main, main→none, Release/*→none, unconfigured→none)
- **`internal/git/re_promote_test.go`**: `RemapCommitsByPatchID` with temp git repos; same-SHA mapping, unmatched detection, malformed SHA handling
- **`internal/app/re_promote_test.go`**: `startRePromoteInto` seeding, `onRePromoteSeeded` application, eligibility gates, `r` key routing, fallback to manual flow
- **`internal/app/keys_test.go`**: `keyRunHistory` `r` branch (eligible vs ineligible rows), `Enter` reset of stale state

### Integration Tests

- **`internal/git/re_promote_test.go`**: Temp git repos with real branch ranges; patch-id equivalence across SHAs; unmatched warning; malformed input handling

### End-to-End Tests

- **`internal/app/re_promote_e2e_test.go`** (90 tasks + 4 remediations):
  - **FullACPath**: Temp git repo + runs fixture (NO org); drive `StateRunHistory` → `r` (interim discovery while remap runs) → `rePromoteSeededMsg` lands on `StateCommitSelection` → assert source is `origin/<rec.Target>`, equivalent commit is patch-id-mapped and pre-checked, missing commit is warned → confirm selection → target selection (next-env default pre-suggested, still overridable) → plan preview → branch creation → cherry-pick → post-pick verification; load the new run and assert `SourceRunID == priorRunID`
  - **Edited Selection**: Toggle a pre-checked item off; assert the edit is honored through to the plan
  - **Failed Prior Run**: A `Failed`-status prior run for the same ticket; `r` is a no-op (no pre-load)
  - **No Prior Run**: Ticket with no prior run; normal `StateTicketInput` flow proceeds with no pre-load

### Code Quality

- **`go test -race ./...`**: All green (no race conditions)
- **`go vet ./...`**: Clean (no vet warnings)
- **`gofmt -l .`**: No formatting issues
- **`TestApp_NeverImportsExecSeam`**: Still passes (no new exec seams in `internal/app`)

### Summary

- **90 planned tasks** (7 phases + 4 remediations): All checked
- **E2E scenarios**: 4 (full AC path, edited selection, failed prior run, no prior run)
- **Adversarial review**: 4 findings identified and fixed (R1–R4b); full regression suite green

## Merged Artifacts

### Files Synced to Living Specs

1. **`openspec/specs/re-promotion/spec.md`** (NEW)
   - Created from change's delta (full spec, not delta format)
   - 7 requirements (eligibility gate, discovery source, pre-loaded/editable commits, patch-id remap, missing-commit warning, SourceRunID linkage, fallback)
   - 17 scenarios total

2. **`openspec/specs/run-persistence/spec.md`** (MODIFIED)
   - Added 1 new requirement: `SourceRunID` recorded on the run (3 scenarios)
   - `Record.SourceRunID` field, backward-compatible additive growth, `SourceRunID` set at new-run creation
   - Existing `PRUrl`, `PickIndex`, `Phase`, `Commits`, cancellation, retention all preserved

3. **`openspec/specs/run-history/spec.md`** (MODIFIED)
   - Added 1 new requirement: `r` on an eligible terminal-success run initiates re-promote (4 scenarios)
   - `r` key action, eligibility gate, ineligible no-op, existing `Enter`→resume behavior preserved
   - Existing history list, detail view, resume routing all preserved

4. **`openspec/specs/commit-discovery/spec.md`** (MODIFIED)
   - Added 2 new requirements: re-promotion source bypass (1 scenario) + next-environment suggestion (3 scenarios)
   - `NextEnvironmentBranch` function, forward pipeline direction, manual fallback for last/out-of-order environments
   - Existing ticket-based search, branch search, source-branch enforcement, topological ordering, equivalence detection, diagnostics all preserved

### Change Artifacts Archived

- **Proposal**: `openspec/changes/re-promote/proposal.md` (moved to archive)
- **Design**: `openspec/changes/re-promote/design.md` (moved to archive)
- **Exploration**: `openspec/changes/re-promote/exploration.md` (moved to archive)
- **Specs**: `openspec/changes/re-promote/specs/re-promotion/spec.md`, `run-persistence/spec.md`, `run-history/spec.md`, `commit-discovery/spec.md` (merged to living; folder archived)
- **Tasks**: `openspec/changes/re-promote/tasks.md` (archived; all 90 tasks complete + 4 remediations)

## What Remains OUT (Future Work)

### HU-017: Cleanup (Explicit Post-Deploy Cleanup)

- Scope: Delete the temporary `deploy/*` promotion branch and `.deploydeck/runs/<run-id>` after successful deployment
- Current State: HU-016 leaves the branch in place
- Status: Deferred to HU-017

### HU-018: Multiple Sandbox Deployments in One Run

- Scope: Deploy to multiple sandboxes in sequence, each with its own validation, push, and PR
- Current State: HU-016 handles single re-promotion per run
- Status: Deferred to HU-018

### HU-019: Release Pipeline Automation

- Scope: Extend PR-to-release workflow (e.g., auto-merge on CI pass, auto-tag, auto-deploy to prod)
- Current State: PR creation is manual, merge is manual
- Status: Deferred to HU-019

### HU-015: Quick Deploy

- Scope: Add `--quick` flag to skip summary/confirmation when deploying to dev sandboxes
- Status: Independent, not included in HU-016

## Specification Conformance

All acceptance criteria from `docs/HISTORIAS.md:HU-016` met:

| AC | Title | Status | Evidence |
|---|---|---|---|
| AC1 | Prior successful run offers reuse (Succeeded/SucceededPartial) | ✅ | Eligibility gate + e2e full AC path |
| AC2 | Failed/Canceled/Aborted prior run offers no pre-load | ✅ | `TestKeyRunHistory_R_OnIneligibleRow_IsNoOp` + e2e ineligible branch |
| AC3 | Accepted reuse pre-loads N commits, patch-id-mapped & pre-checked, edited selection honored | ✅ | `TestOnRePromoteSeeded_AppliesMatchedAndUnmatched_LandsOnCommitSelection` + e2e edited-selection scenario |
| AC4 | Missing commit warned explicitly, never omitted silently | ✅ | `selectionWarnings` prepends unmatched SHAs; e2e scenario confirms warning visible |
| AC5 | Completed re-promotion records `SourceRunID` + next-env default suggested | ✅ | `TestOnBranchCreated_RePromoteRun_PersistsSourceRunID` + e2e target-selection default confirmed |
| AC6 | Failed run → no pre-load; no prior run → manual flow; no next env → manual target pick | ✅ | R1 remediation + e2e no-prior/no-next scenarios |

## Transition to Production

### Pre-Commit Checklist

- [x] All 90 tasks (7 phases + 4 remediations) checked in `tasks.md`
- [x] `go test -race ./...` green
- [x] `go vet ./...` clean
- [x] `gofmt -l .` clean
- [x] `boundary_test.go` passes (no new exec seams)
- [x] Proposal.md success criteria verified
- [x] E2E tests pass (no real org, temp git fixture only)
- [x] Adversarial review: 4 findings identified and fixed; full regression suite green

### Deployment Notes

1. **No Database Migrations**: `SourceRunID` is `omitempty` — every existing `run.json` still loads
2. **No Config Changes Required**: Pipeline order is already configured (reused from existing discovery)
3. **Backward Compatible**: Older runs resume correctly; re-promotion is opt-in (user-initiated via TUI `r` key)
4. **Rollback**: Delete `internal/git/re_promote.go`, revert `internal/app/keys.go` `keyRunHistory` `r` branch, revert `internal/app/update.go` additions, revert `internal/git/source_suggestion.go` `NextEnvironmentBranch`, revert `internal/runs/writer.go` `SourceRunID` field

## Handoff Summary

- **Source of Truth Updated**: `openspec/specs/{re-promotion,run-history,run-persistence,commit-discovery}/spec.md` now authoritative
- **Change Folder**: Archived to `openspec/changes/archive/re-promote/`
- **Code Ready**: All tests pass, all 90 tasks + 4 remediations complete, adversarial review findings fixed
- **No Blockers**: Clean bill of health for merge

---

**Archived By**: SDD Archive Phase (re-promote)  
**Archive Date**: 2026-07-28  
**Next Steps**: Merge to `main`, deploy, and proceed with HU-015, HU-017, or HU-018
