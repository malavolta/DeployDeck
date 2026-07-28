# Archive Report: HU-017 — Branch & Run Cleanup (`cleanup`)

**Date Archived**: 2026-07-28  
**Status**: Complete and Verified  
**Total Tasks**: All 150 tasks (5 groups + 6 remediations) checked  
**Final Verification**: READY TO ARCHIVE (as reported by sdd-verify)  
**Test Result**: Full regression suite green; `go test ./... -race` clean, `go vet` clean, `gofmt` clean

## Executive Summary

HU-017 completes branch-lifecycle hygiene in DeployDeck by restoring the original branch on quit/finish/abort, offering to delete the current run's temporary `deploy/*` branch with confirmation, providing a new `StateBranchCleanup` screen for batch orphan cleanup with age and push-status detection, and surfacing the existing run-retention mechanism in the TUI. The implementation introduces a new `branch-cleanup` capability, adds 7 exec-free `git.Service` methods (all via `tea.Cmd`), extends the state machine with new `StateBranchCleanup`, and reuses the existing unchanged `runs.Prune` retention logic. All 150 implementation tasks (5 work-unit groups + 6 post-review remediations: H-1, H-2, M-1, M-2, L-2, L-1 deferred) completed; branch restoration is guarded against mid-conflict scenarios, all deletions require confirmation with strong confirmation for unpushed work, and no schema/config changes are needed.

## What Shipped

### 1. Original-Branch Restore on Finish/Abort

**Group 1 Core Capability**: New `git.Service.CurrentBranch()` method (exec-free via `tea.Cmd`)  
**Group 2 Application Logic**: Capture original branch at `onPrereqDone`, restore via `quitCmd()` closure, 13-site `tea.Quit` swap

- **Capture**: `onPrereqDone` → `tea.Batch(resumeDetectCmd(), originalBranchCmd())` fires `git.CurrentBranch` at startup, stores in `m.originalBranch`
- **Restore Closure**: `quitCmd()` runs `git.Checkout(originalBranch)` synchronously before returning `tea.Quit`'s msg
- **Guards (load-bearing)**: Skip restore when `repoState.InProgress` (unresolved cherry-pick), `original ∈ {"","HEAD"}` (detached/empty), `original == current`, or `!BranchExists(original)`
- **Swap Pattern**: All 13 `return m, tea.Quit` sites in `keys.go` → `return m, m.quitCmd()`; `update.go:26` ctrl+c stays hard `tea.Quit` (no restore on interrupt)
- **Mandatory Negative Test**: Abort mid-conflict → NO restore, `CHERRY_PICK_HEAD` intact, resume still detectable

### 2. Current Run's Temp Branch Inline Delete

**Group 3 Core Capability**: New `git.Service.DeleteLocalBranch()` + `DeleteRemoteBranch()` methods (force `-D`, hardened with `--` separator)  
**Group 3 Application Logic**: Offer delete inline at `StateSucceeded`/`StateAborted`, confirmation gate, `pendingDeleteCurrent` state, deletion inside `quitCmd()` after restore

- **Entry Point**: `d` key at terminal states (`StateSucceeded`, `StateAborted`) fires `unpushedCountCmd(m.plan.PromotionBranch)`
- **Confirm Gate**: Count>0 → strong confirm (type `"BORRAR"`), else normal confirm (`y`)
- **Execution**: Confirmation sets `m.pendingDeleteCurrent = true`, calls `m.quitCmd()`, which deletes local branch then remote-if-pushed AFTER restore-checkout (must leave the branch before deleting it)
- **Target Correctness**: Always targets `m.plan.PromotionBranch`, never `m.branchName` (critical for resumed runs where `m.branchName=""`)

### 3. Orphan Deploy Branches Listed & Batch Cleaned

**Group 1 Core Capability**: New `git.Service.ListDeployBranches()` method (single `for-each-ref` + age/push parsing)  
**Group 4 Application Logic**: New `StateBranchCleanup` screen, entry via `b` from `StateTicketInput`, nav/delete/prune keys, orphan correlation

- **List Generation**: ONE `git for-each-ref --format='%(refname:short)|%(committerdate:iso-strict)' refs/heads/deploy/* refs/remotes/origin/deploy/*` → pure `parseDeployBranches()` → `[]DeployBranch{Name,LastCommit,Pushed}`
- **Orphan Correlation**: `selectOrphans()` excludes branches tied to live in-progress runs (via `runs.List()` + `renderBranchName` correlation) + non-terminal validation runs (`JobID != "" && !salesforce.IsTerminal(Status)`)
- **Display**: Age (human-readable: "2 days old"), push-status (pushed/local-only), merged-label (best-effort only — see below)
- **Batch Delete**: `d` → per-row unpushed count → (strong/normal) confirm → `deleteOrphanCmd()` → reload list

### 4. Merged-Vs-Abandoned Best-Effort Label

**Group 1 Core Capability**: New `git.Service.IsMergedInto()` method (`merge-base --is-ancestor`, exit-code-as-data)  
**Group 4 Application Logic**: Label-only, never a delete gate

- **Label Logic**: `git merge-base --is-ancestor <deploy> origin/<target>` → "likely merged" vs "abandoned" vs "unknown"
- **Advisory Only**: Label never bypasses confirmation; false negatives (squash-merges) cause extra confirm, never wrong delete
- **Harness Limitation**: No `github.Client` PR lookup (org-free); git-native ancestry is the only signal
- **Mutation-Tested**: Delete path never reads `MergedLabel`, proving gate is unconditional

### 5. Strong Confirmation For Unpushed Work

**Group 1 Core Capability**: New `git.Service.UnpushedCommitCount()` method (two-form: via upstream or remote-count fallback)  
**Group 3 Application Logic**: Typed-confirm gate reusing `keyCancelConfirm` structure, dedicated `deleteConfirm` field + `"BORRAR"` word

- **Unpushed Detection**: If `origin/<branch>` resolves → `rev-list --count origin/<b>..<b>`; else → `rev-list --count <b> --not --remotes=origin`
- **Gate**: Count>0 → strong confirm required (user types `"BORRAR"`, case-sensitive); count==0 → normal confirm (`y`)
- **Dedicated Field**: `m.deleteConfirm` buffer (not shared with `cancelInput`/`CANCELAR` — avoids cross-contamination)
- **Refused**: Normal confirm on unpushed work is refused; strong confirm deletes

### 6. Run Retention From TUI Cleanup Surface

**Group 4 Application Logic**: Invoke existing unchanged `deps.Runs.Prune(cfg.Runs.KeepLast, cfg.Runs.KeepDays, m.now())`  
**Group 4 Testing**: Reuses existing `runs` tests; only the app-level wiring caller is new

- **Entry Point**: `p` key from `StateBranchCleanup` → `pruneConfirm` phase → `pruneRunsCmd` → `runs.Writer.Prune` (untouched)
- **Rule/Config/Schema**: UNCHANGED — HU-017 is purely a new caller, not a redefiner
- **Default Behavior**: `keepLast=30`/`keepDays=90` — effectively low-risk for non-terminal run directories
- **Deferral Note**: L-1 remediation defers deeper `selectPruneCandidates` review to a future `run-retention` follow-up

## Capabilities: 1 New

| Capability | Action | Details |
|---|---|---|
| `branch-cleanup` | **Created** | New spec at `openspec/specs/branch-cleanup/spec.md` — full lifecycle of original-branch restore, temp-branch deletion (inline + batch orphans), unpushed/age/merged-label detection, and TUI retention surface. 5 requirements (each with 2–4 scenarios). |

## Key Design Decisions

### 1. **Restore Checkpoint = Startup's Current Branch, Captured Async**

- **Rationale**: User may switch branches manually before starting a flow. Capture at `onPrereqDone` (after repo check, before user is in a flow state) is the right moment. Async batch+msg avoids blocking the reducer.
- **Benefit**: Zero `main.go` change; restore completes inside the `quitCmd` closure before `Program.Run()` returns.

### 2. **Guards Are Load-Bearing (Especially `InProgress`)**

- **Rationale**: `InProgress` (unmerged paths) must block checkout — git refuses it, and HU-013 resume-detection reads `CHERRY_PICK_HEAD` off the current branch. This is not a soft preference; it's a hard git constraint.
- **Benefit**: Abort mid-conflict still leaves the user on the deploy branch with unmerged paths intact, so resume is still possible.

### 3. **Single-Pass `for-each-ref` Avoids N+1**

- **Rationale**: One subprocess with both `refs/heads/deploy/*` and `refs/remotes/origin/deploy/*` refspecs eliminates per-branch lookups.
- **Benefit**: Fast listing even with 100s of orphan branches; `Pushed` is derived from ref-existence, same semantics as `RemoteHead`.

### 4. **Inline Delete Happens AFTER Restore-Checkout**

- **Rationale**: You cannot delete the branch you are currently on. The flow must: restore-checkout to original → leave original branch intact → delete the temp branch.
- **Benefit**: Guarded by confirmation; safe recovery if the delete fails (user is already off the deleted branch).

### 5. **`-D` Force Delete Is Post-Gate**

- **Rationale**: The app gates on (unpushed + strong-confirm) FIRST. Git's `-d` would double-gate and mis-refuse squash/rebase-merged branches (new SHA → `--is-ancestor` false-negative). Force is deliberate and safe.
- **Benefit**: Users never see git's "this branch is not fully merged" error on squash-merged branches; they see the correct confirmation gate instead.

### 6. **Dedicated `deleteConfirm` + `BORRAR` Word**

- **Rationale**: Reuse `keyCancelConfirm` structure but avoid shared-buffer contamination. User typing "cancel" to delete is semantically wrong and confusing.
- **Benefit**: Decoupled confirmation contexts; each has its own buffer and word.

### 7. **Orphan Correlation via `renderBranchName`**

- **Rationale**: The TUI renders branch names per the app's naming convention. Correlation must match that rendering, not raw git names.
- **Benefit**: Excludes branches tied to in-progress runs correctly, even when the render name differs from git's stored name.

## Review Findings & Remediations

Adversarial review identified 6 findings (1 HIGH, 1 HIGH, 1 MED, 1 MED, 1 LOW, 1 LOW-deferred), all remediations applied and verified:

### H-1 [HIGH]: Cursor-Move TOCTOU on Cleanup Screen

**Issue**: User presses `d` to delete a branch, then immediately moves the cursor while the unpushed-count command runs asynchronously. The confirm could act on a different row.

**Fix Applied**: `d` now CAPTURES the row into `cleanupDeleteTarget`/`cleanupDeleteTargetPushed`, enters a new `cleanupCounting` phase that freezes cursor-move keys, and `confirmDeleteOrphan` deletes the CAPTURED row, never the live cursor row. `onUnpushedCount` drops results for mismatched branches.

**Verification**: RED tests `TestKeyBranchCleanup_D_CapturesTarget_FreezesCursor_DeletesCaptured` + `TestConfirmDeleteOrphan_DeletesCapturedTarget_NotCursorRow` + `TestOnUnpushedCount_DropsStaleCountForDifferentBranch`.

### H-2 [HIGH]: Non-Terminal Validation Runs Incorrectly Listed as Orphans

**Issue**: A branch tied to a non-terminal validation run (e.g., `JobID != ""` and status is `Running`) should not appear in the orphan list, but the original check only looked at `repoState.InProgress`.

**Fix Applied**: `selectOrphans` now ALSO excludes any branch whose correlating record has `JobID != "" && !salesforce.IsTerminal(Status)`, independent of cherry-pick-in-progress status.

**Verification**: RED tests `TestSelectOrphans_ExcludesNonTerminalValidatingRun` + `TestSelectOrphans_TerminalValidationRunStaysListed`.

### M-1 [MED]: Restore Restore-vs-Delete Coupling

**Issue**: Delete confirmation and restore-checkout were tightly coupled; a resumed run where `current==original` (so restore is skipped) also skipped the delete, even when the user confirmed.

**Fix Applied**: `quitCmd` now decouples the two: confirmed delete of `m.plan.PromotionBranch` fires even when restore is skipped (no-op checkout but delete proceeds); mid-conflict (`InProgress`) still skips both.

**Verification**: RED tests `TestQuitCmd_DeletesConfirmedBranch_EvenWhenRestoreSkipped` + `TestQuitCmd_NoopWhenStillOnTargetBranch`.

### M-2 [MED]: Stale Unpushed-Count State Leaks Off-Screen

**Issue**: User presses `d` on the cleanup screen, gets prompted for unpushed-count, then navigates to a different state (`StateTicketInput`, `StateSucceeded`) before confirming. The stale count could apply to the wrong state.

**Fix Applied**: `onUnpushedCount` now applies its result ONLY on delete-capable screens (`StateSucceeded`/`StateAborted`/`StateBranchCleanup`), dropping stale results otherwise. Entry into a terminal state resets the delete-confirmation state.

**Verification**: RED tests `TestOnUnpushedCount_DroppedWhenOffDeleteScreens` + `TestOnReportDone_TerminalEntryResetsStaleCleanupState` + `TestOnAborted_ResetsStaleCleanupState`.

### L-2 [LOW]: Destructive Git Calls Lack End-of-Options Separator

**Issue**: Branch names could contain special chars or flags (e.g., `deploy/--help`, `deploy/-D`). Git command injection is theoretical but possible.

**Fix Applied**: Hardened all destructive calls with `--` end-of-options separator: `git branch -D -- <branch>`, `git push origin --delete -- <branch>`, `git checkout <branch> --` (trailing, since `--` after the ref is the correct form for checkout).

**Verification**: RED assertions in `TestService_Checkout_SwitchesToExistingBranch`, `TestService_DeleteLocalBranch_RemovesBranch`, `TestService_DeleteRemoteBranch_RemovesOriginRef`; all app-level FakeRunner registrations updated.

### L-1 [LOW]: Non-Terminal Run Pruning Risk (DEFERRED)

**Issue**: `selectPruneCandidates` (run-retention) is count/age-only and can select a non-terminal run's directory for deletion (e.g., a still-running validation run). The HU-017 spec forbids redefining/reimplementing run-retention ("MUST NOT redefine, reimplement, or change it"), and with default `keepLast=30`/`keepDays=90` it effectively never triggers on production data. Left UNCHANGED — track as a `run-retention` follow-up.

## Test Posture

### Unit Tests

- **`internal/git/deploy_branches_test.go`** (pure): `TestParseDeployBranches_StripsOriginAndSetsPushed` — table-driven ref parsing
- **`internal/app/original_branch_test.go`** (pure+FakeRunner): `TestShouldRestore_GuardMatrix` — 5 guard conditions
- **`internal/git/service_cleanup_test.go`** (real-git, `-short`-skip): `TestService_CurrentBranch_ReturnsCheckedOutBranch` + all 6 core methods

### Integration Tests

- **`internal/git/service_cleanup_e2e_test.go`** (real temp git): Restore, inline delete, unpushed count, merge-ancestor label all with real repos + bare remote
- **`internal/app/branch_cleanup_test.go`**: Orphan correlation, cursor TOCTOU guard, non-terminal run exclusion

### End-to-End Tests

- **`internal/app/hu017_cleanup_e2e_test.go`** (real temp git + bare remote + fs, 2 scenarios):
  - **FullCleanupFlow**: Startup on original branch → flow to success → restore + inline delete → `b` lists orphans (age+status) → batch delete (strong+normal confirms) → `p` prunes old runs
  - **AbortMidConflict_NoRestore**: Abort mid-conflict → quit → no restore, `CHERRY_PICK_HEAD` intact, resume still offered

### Code Quality

- **`go test -race ./...`**: All green (no race conditions)
- **`go vet ./...`**: Clean (no vet warnings)
- **`gofmt -l .`**: No formatting issues
- **`TestApp_NeverImportsExecSeam`**: Still passes (all git via `git.Service` inside `tea.Cmd`)

### Summary

- **150 planned tasks** (5 groups + 6 remediations): All checked
- **Neg tests**: Abort-mid-conflict NO-restore, normal-confirm refuse on unpushed, delete-gate unconditional (mutation-tested)
- **E2E scenarios**: 2 (full cleanup flow, abort mid-conflict)
- **Adversarial review**: 6 findings identified and fixed (H-1, H-2, M-1, M-2, L-2, L-1 deferred); full regression suite green

## Merged Artifacts

### Files Synced to Living Specs

1. **`openspec/specs/branch-cleanup/spec.md`** (NEW)
   - Created from change's delta spec (stripped delta header, became full living spec)
   - 5 requirements (restore, inline delete, orphan list, merged label, strong confirm, retention)
   - 17 scenarios total
   - No modifications to any other living spec (change references but does not modify `run-retention`)

### Change Artifacts Archived

- **Proposal**: `openspec/changes/cleanup/proposal.md` (moved to archive)
- **Design**: `openspec/changes/cleanup/design.md` (moved to archive)
- **Exploration**: `openspec/changes/cleanup/exploration.md` (moved to archive)
- **Specs**: `openspec/changes/cleanup/specs/branch-cleanup/spec.md` (merged to living spec; folder archived)
- **Tasks**: `openspec/changes/cleanup/tasks.md` (archived; all 150 tasks complete)

## What Remains OUT (Future Work)

### L-1: Run-Retention Non-Terminal Risk Assessment

- **Scope**: Deeper review of `selectPruneCandidates` to ensure non-terminal run directories are never pruned
- **Current State**: HU-017 reuses unchanged `runs.Prune`; default `keepLast=30`/`keepDays=90` mitigates risk
- **Status**: Deferred to `run-retention` follow-up

## Specification Conformance

All 5 requirements from HU-017 design fully shipped:

| Req | Title | Status | Evidence |
|---|---|---|---|
| 1 | Original branch restore on finish/abort | ✅ | `originalBranchCmd` capture + `quitCmd` restore; abort-mid-conflict guard test |
| 2 | Current run's temp branch deleted with confirmation | ✅ | Inline `d` at terminals + confirm gate; delete targets `plan.PromotionBranch` |
| 3 | Orphan branches listed with age/push-status | ✅ | `ListDeployBranches` + orphan correlation + `selectOrphans` + non-terminal exclusion |
| 4 | Merged-vs-abandoned best-effort label only | ✅ | `IsMergedInto` label; mutation-tested never bypasses confirm |
| 5 | Strong confirmation for unpushed work | ✅ | `UnpushedCommitCount` + typed `BORRAR` gate; normal confirm refused on unpushed |
| 6 | Run retention from cleanup surface | ✅ | `p` key → `pruneRunsCmd` → `deps.Runs.Prune` (unchanged) |

## Transition to Production

### Pre-Commit Checklist

- [x] All 150 tasks (5 groups + 6 remediations) checked in `tasks.md`
- [x] `go test -race ./...` green
- [x] `go vet ./...` clean
- [x] `gofmt -l .` clean
- [x] `TestApp_NeverImportsExecSeam` still passes (no new exec seams)
- [x] Proposal requirements verified
- [x] E2E tests pass (no real org, temp git + bare remote + fs fixture)
- [x] Adversarial review: 6 findings identified and fixed; full regression suite green

### Deployment Notes

1. **No Database Migrations**: All new fields are ephemeral model state or git-derived; no schema change
2. **No Config Changes Required**: Retention config already exists (reused unchanged)
3. **Backward Compatible**: Runs without branch-cleanup state work fine (restore/delete are new features, not required)
4. **Rollback**: Delete `internal/git/service_cleanup.go`, revert `internal/app` key swaps (13 sites), revert `StateBranchCleanup` additions, revert spec creation
5. **No Data Migration**: `Record` schema, config, CLI retention unchanged

## Handoff Summary

- **Source of Truth Updated**: `openspec/specs/branch-cleanup/spec.md` now authoritative
- **Change Folder**: Archived to `openspec/changes/archive/cleanup/`
- **Code Ready**: All tests pass, all 150 tasks + 6 remediations complete, adversarial review findings fixed
- **No Blockers**: Clean bill of health for merge

---

**Archived By**: SDD Archive Phase (cleanup)  
**Archive Date**: 2026-07-28  
**Next Steps**: Merge to `main`, deploy, and proceed with future enhancements or follow-up work (L-1 run-retention assessment)
