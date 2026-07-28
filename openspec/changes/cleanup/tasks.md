# Tasks: HU-017 — Branch & Run Cleanup (`cleanup`)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2300 (5 files modified, 1 new prod file, 9 new test files) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | Group 1 → Group 2 → Group 3 → Group 4 → Group 5 |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending (user decides: stacked-to-main vs feature-branch-chain vs single-PR size:exception; proposal.md notes "single stacked branch" — reconcile before apply) |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `git.Service` cleanup methods + pure parser | PR 1 | `go test ./internal/git/... -run 'CurrentBranch\|Checkout\|DeleteLocalBranch\|DeleteRemoteBranch\|UnpushedCommitCount\|IsMergedInto\|ListDeployBranches\|ParseDeployBranches' -v` | real temp git (`newTempRepo`/`newTempRepoWithRemote`), `-short`-skippable, no org | revert `internal/git/service_cleanup.go` + its tests; nothing else depends on it yet |
| 2 | Original-branch restore + quit-swap | PR 2 | `go test ./internal/app/... -run 'ShouldRestore\|OriginalBranch\|QuitCmd' -v` | FakeRunner (fast) + real temp git for the 2 mandatory guard cases, `-short`-skippable | revert `originalBranch`/`quitCmd`/batch change + the 13-site swap; app falls back to bare `tea.Quit` |
| 3 | Inline current-branch delete | PR 3 | `go test ./internal/app/... -run 'OnPushDone\|KeySucceeded_D\|InlineDelete\|StrongConfirm\|ConfirmDelete' -v` | FakeRunner + real temp git+bare remote for the round-trip case, `-short`-skippable | revert inline-delete fields/keys/quitCmd extension; terminals revert to quit-only |
| 4 | `StateBranchCleanup` screen + retention wiring | PR 4 | `go test ./internal/app/... -run 'SelectOrphans\|KeyTicket_B\|KeyBranchCleanup\|ListDeployBranchesCmd\|DeleteOrphanCmd\|MergedLabel\|ViewBranchCleanup' -v` | FakeRunner only (reuses Unit 1's proven git.Service) | revert `StateBranchCleanup` const/keys/view/cmds; `b` becomes inert |
| 5 | Consolidated HU-017 e2e | PR 5 | `go test ./internal/app/... -run 'TestHU017_E2E' -v` | real temp git + bare remote + `.deploydeck/runs/` fs, `-short`-skippable, no org | revert only the new e2e test file; no prod code |

### Corrections Baked In (traceability)

1. Delete/quit branch source = `m.plan.PromotionBranch` (never `m.branchName`) → 3.6, 3.12
2. `onPrereqDone` → `tea.Batch(resumeDetectCmd(), originalBranchCmd())` → 2.3, 2.4
3. `InProgress` guard = git refuses checkout w/ unmerged paths (not a resume-detection need) → 2.1, 2.8 (comment must say this)
4. `m.currentPushed` set on push success → 3.1, 3.2
5. `BORRAR` + dedicated `deleteConfirm` field, reuse `keyCancelConfirm` structure → 3.7, 3.8, 4.11, 4.12
6. ctrl+c stays hard interrupt (no restore); only the 13 `keys.go` sites swap → 2.14

## Group 1 — `git.Service` Cleanup Methods

Files: `internal/git/service_cleanup.go` (new), `internal/git/deploy_branches_test.go` (new, pure), `internal/git/service_cleanup_e2e_test.go` (new, real-git, `-short`-skip).

- [x] 1.1 RED `service_cleanup_e2e_test.go`: `TestService_CurrentBranch_ReturnsCheckedOutBranch` + `_DetachedReturnsHEAD` (checkout by SHA). AC: restore no-op on detached.
- [x] 1.2 GREEN: implement `CurrentBranch` (`rev-parse --abbrev-ref HEAD`).
- [x] 1.3 RED: `TestService_Checkout_SwitchesToExistingBranch` + `_UnknownBranchErrors` (plain checkout, no `-b`).
- [x] 1.4 GREEN: implement `Checkout`.
- [x] 1.5 RED: `TestService_DeleteLocalBranch_RemovesBranch` (`branch -D`; verify gone via `rev-parse --verify`).
- [x] 1.6 GREEN: implement `DeleteLocalBranch`.
- [x] 1.7 RED (bare remote): `TestService_DeleteRemoteBranch_RemovesOriginRef` (`push origin --delete`; verify via `ls-remote`).
- [x] 1.8 GREEN: implement `DeleteRemoteBranch`.
- [x] 1.9 RED: `TestService_UnpushedCommitCount_WithUpstream` (`rev-list origin/<b>..<b> --count`) + `_NoUpstream` (`rev-list --count <b> --not --remotes=origin`).
- [x] 1.10 GREEN: implement `UnpushedCommitCount` two-form selection, exact rev-list forms per design.
- [x] 1.11 RED: `TestService_IsMergedInto_TrueForAncestor` / `_FalseForDivergent` (`merge-base --is-ancestor`, exit-code-as-data mirroring `revParseVerify`).
- [x] 1.12 GREEN: implement `IsMergedInto`.
- [x] 1.13 RED (pure, `deploy_branches_test.go`): `TestParseDeployBranches_StripsOriginAndSetsPushed` table-driven — strips `origin/`, `Pushed` via head/remote correlation, `LastCommit` from `committerdate:iso-strict`.
- [x] 1.14 GREEN: pure `parseDeployBranches([]byte) []DeployBranch` + `DeployBranch{Name,LastCommit,Pushed}` struct.
- [x] 1.15 RED: `TestService_ListDeployBranches_SinglePassWithAgeAndPushStatus` (seed local-only + pushed `deploy/*`, differing dates).
- [x] 1.16 GREEN: implement `ListDeployBranches` — ONE `for-each-ref --format='%(refname:short)|%(committerdate:iso-strict)' refs/heads/deploy/* refs/remotes/origin/deploy/*` + `parseDeployBranches`.

Group gate: `go build ./... && go vet ./... && go test ./internal/git/... -short`; confirm `TestApp_NeverImportsExecSeam` unaffected (no `internal/app` change yet).

## Group 2 — Original-Branch Restore

Files: `internal/app/app.go`, `internal/app/commands.go`, `internal/app/update.go`, `internal/app/keys.go`; tests: `internal/app/original_branch_test.go` (pure+FakeRunner), `internal/app/original_branch_e2e_test.go` (real-git, `-short`-skip).

- [x] 2.1 RED (pure): `TestShouldRestore_GuardMatrix` — skip iff `InProgress`, `original==""`, `=="HEAD"`, `==current`, or `!exists`; restore otherwise. Comment/task wording: guard exists because git REFUSES checkout with unmerged paths, not a resume-detection need.
- [x] 2.2 GREEN: `commands.go` pure `shouldRestore(inProgress bool, original, current string, exists bool) bool`.
- [x] 2.3 RED: `TestOnPrereqDone_BatchesResumeDetectAndOriginalBranch` — asserts both cmds fire; `deps.Git==nil` → `originalBranchCmd` nil, `tea.Batch` drops it.
- [x] 2.4 GREEN: `update.go:105` → `return m, tea.Batch(m.resumeDetectCmd(), m.originalBranchCmd())`.
- [x] 2.5 RED: `TestOriginalBranchCmd_CapturesCurrentBranch` (FakeRunner cans `rev-parse --abbrev-ref HEAD`).
- [x] 2.6 GREEN: `commands.go` `originalBranchCmd`/`originalBranchMsg`; `update.go` sets `m.originalBranch`.
- [x] 2.7 RED: `TestQuitCmd_RestoresOriginalBranch_OnTerminalQuit` (FakeRunner: CurrentBranch≠original, BranchExists true → `Checkout(original)` called).
- [x] 2.8 RED (MANDATORY negative): `TestQuitCmd_AbortMidConflict_NoRestore` — `repoState.InProgress=true` → zero `Checkout` calls.
- [x] 2.9 RED: `TestQuitCmd_NoopWhenOriginalGoneDetachedOrCurrent` — table: `""`, `"HEAD"`, `==current`, `!exists` → zero `Checkout` calls.
- [x] 2.10 GREEN: `commands.go` implement `quitCmd()` per design's closure (sync `CurrentBranch`→`BranchExists`→`Checkout`, gated by `shouldRestore`), returns `tea.Quit()`'s `QuitMsg`.
- [x] 2.11 RED (real-git): `TestHU017_RestoreOnQuit_RealRepo` — drive to `StateSucceeded`, `q`, assert real `symbolic-ref --short HEAD` back on original.
- [x] 2.12 RED (MANDATORY, real-git): `TestHU017_AbortMidConflict_NoRestore_RealRepo` — real unresolved conflict, `q` from `StateCherryPickConflict`; HEAD stays on deploy branch, `CHERRY_PICK_HEAD` intact (resume still possible).
- [x] 2.13 GREEN: none — 2.10 already satisfies 2.11/2.12; rerun to confirm.
- [x] 2.14 GREEN: `keys.go` swap `return m, tea.Quit` → `return m, m.quitCmd()` at lines 48, 69, 91, 106, 118, 124, 145, 306, 334, 359, 384, 435, 473 (13 sites). `update.go:26` ctrl+c stays a hard `tea.Quit` — no restore, by design (user is left on the deploy branch on interrupt).

Group gate: `go build ./... && go vet ./... && go test ./internal/app/... -short`.

## Group 3 — Inline Current-Branch Delete

Files: `internal/app/app.go`, `internal/app/commands.go`, `internal/app/keys.go`, `internal/app/update.go`; tests: `internal/app/inline_delete_test.go`, `internal/app/inline_delete_e2e_test.go` (real-git+bare remote, `-short`-skip).

- [x] 3.1 RED: `TestOnPushDone_SetsCurrentPushedTrueOnSuccess` / `_LeavesFalseOnFailure`.
- [x] 3.2 GREEN: `update.go` `onPushDone` — set `m.currentPushed = true` on the success branch (before `preparePRCmd`).
- [x] 3.3 RED: `TestKeySucceeded_D_UnpushedRequiresStrongConfirm` — `d` fires `unpushedCountCmd(m.plan.PromotionBranch)`, count>0 → `strongConfirm`.
- [x] 3.4 RED: `TestKeySucceeded_D_PushedUsesNormalConfirm` — count==0 → normal confirm.
- [x] 3.5 GREEN: `keys.go` `keySucceeded` (+ `StateAborted` terminal block) gains `d`; `handleKey`'s terminal switch routes it.
- [x] 3.6 RED (MANDATORY, BLOCKER FIX): `TestInlineDelete_ResumedRun_TargetsPlanPromotionBranch_NotBranchName` — build Model via `resumeInto`'s path (`m.branchName==""`, only `m.plan.PromotionBranch` set), confirm delete targets `plan.PromotionBranch`.
- [x] 3.7 RED: `TestStrongConfirm_BORRAR_TypedGate` — wrong text refused+notice, exact `"BORRAR"` (case-sensitive) deletes; buffer built via `KeyRunes`/backspace on dedicated `m.deleteConfirm` (never `m.cancelInput`).
- [x] 3.8 GREEN: `app.go` add `currentPushed`, `pendingDeleteCurrent`, `deleteConfirm string`, `cleanupPhase` type+consts (`cleanupIdle`, `cleanupConfirm`, `cleanupStrongConfirm`); `commands.go` `unpushedCountCmd`/`unpushedMsg`; `keys.go` confirm/strongConfirm handler mirroring `keyCancelConfirm`'s structure, gated on `deleteConfirm == "BORRAR"`.
- [x] 3.9 RED: `TestConfirmDelete_SetsPendingDeleteCurrent_ThenQuits`.
- [x] 3.10 GREEN: confirm success → `m.pendingDeleteCurrent = true; return m, m.quitCmd()`.
- [x] 3.11 RED: `TestQuitCmd_DeletesCurrentBranch_LocalAndRemoteIfPushed` / `_DeletesLocalOnly_WhenUnpushed`.
- [x] 3.12 GREEN: extend `quitCmd()` — after restore-checkout succeeds, `if del { DeleteLocalBranch(name); if pushed { DeleteRemoteBranch(name) } }`, `name := m.plan.PromotionBranch` (never `m.branchName`).
- [x] 3.13 RED (real-git+bare remote): `TestHU017_InlineDelete_LocalAndRemoteRoundTrip`.
- [x] 3.14 GREEN: none — proven by 3.12; rerun to confirm.

Group gate: `go build ./... && go vet ./... && go test ./internal/app/... -short`.

## Group 4 — `StateBranchCleanup` Screen

Files: `internal/app/app.go`, `internal/app/keys.go`, `internal/app/commands.go`, `internal/app/update.go`, `internal/app/view.go`; tests: `internal/app/branch_cleanup_test.go`.

- [ ] 4.1 RED (pure): `TestSelectOrphans_ExcludesInProgressRun` — a `deploy/*` tied to a live in-progress run (via `runs.List()` correlation) excluded; others included.
- [ ] 4.2 GREEN: `commands.go` pure `selectOrphans(branches []git.DeployBranch, runs []runs.Record, inProgress bool) []git.DeployBranch`.
- [ ] 4.3 RED: `TestKeyTicket_B_EntersBranchCleanupAndLoads`.
- [ ] 4.4 GREEN: `app.go` add `StateBranchCleanup`; `keys.go` `keyTicket` adds `case "b"`.
- [ ] 4.5 RED: `TestKeyBranchCleanup_QEsc_ReturnsToTicketInput`.
- [ ] 4.6 GREEN: `keys.go` new `keyBranchCleanup` wired in `handleKey`; `app.go` extends `cleanupPhase` (loading|browsing|confirm|strongConfirm|pruneConfirm) + `cleanupBranches`, `cleanupCursor`, `cleanupNotice`.
- [ ] 4.7 RED: `TestListDeployBranchesCmd_LoadsAndSetsBrowsing`.
- [ ] 4.8 GREEN: `commands.go` `listDeployBranchesCmd`/`deployBranchesMsg`; `update.go` handler.
- [ ] 4.9 RED: `TestKeyBranchCleanup_Nav_MovesCursor`.
- [ ] 4.10 GREEN: nav case in `keyBranchCleanup`.
- [ ] 4.11 RED: `TestKeyBranchCleanup_D_UnpushedGatesStrongConfirm` / `_PushedGatesNormalConfirm` (per-row `unpushedCountCmd`).
- [ ] 4.12 GREEN: `d` case reusing Group 3's strong/normal-confirm machinery, generalized to the selected row.
- [ ] 4.13 RED: `TestDeleteOrphanCmd_DeletesLocalAndRemoteIfPushed_ThenReloads`; declining leaves the list untouched.
- [ ] 4.14 GREEN: `commands.go` `deleteOrphanCmd`/`deleteDoneMsg`; `update.go` handler reloads via `listDeployBranchesCmd`.
- [ ] 4.15 RED: `TestMergedLabel_NeverBypassesConfirm` — "likely merged" row still needs the same gate; a false "not merged" only adds an extra confirm, never force-deletes.
- [ ] 4.16 GREEN: `IsMergedInto` wired for row labeling only (view display) — delete path untouched (proves 4.12's gate is unconditional).
- [ ] 4.17 RED: `TestKeyBranchCleanup_P_FiresPruneRunsCmd_ThenNotice` — asserts `deps.Runs.Prune(cfg.Runs.KeepLast, cfg.Runs.KeepDays, m.now())` called UNCHANGED.
- [ ] 4.18 GREEN: `commands.go` `pruneRunsCmd`/`pruneDoneMsg`; `keys.go` `p` + confirm; `update.go` handler.
- [ ] 4.19 RED: `TestViewBranchCleanup_RendersAgeAndPushStatus` — string-contains per row (age, pushed/local-only, merged-label).
- [ ] 4.20 GREEN: `view.go` `viewBranchCleanup` + `View()`'s `case StateBranchCleanup`.

Group gate: `go build ./... && go vet ./... && go test ./internal/app/... -short`.

## Group 5 — Consolidated HU-017 E2E

File: `internal/app/hu017_cleanup_e2e_test.go` (real temp git + bare remote + `.deploydeck/runs/` fs, `-short`-skip, no org). Mirrors `docs/HISTORIAS.md:1108-1114`.

- [ ] 5.1 RED: `TestHU017_E2E_FullCleanupFlow` — seed original branch, pushed+local-only orphans (differing ages), old+recent runs; finish→restore; inline confirm deletes current branch (local+remote-if-pushed); `b` lists orphans w/ age+push-status; unpushed delete refused without `BORRAR`, deletes with it; `p` prunes outside `keepLast`/`keepDays`, keeps recent.
- [ ] 5.2 RED (MANDATORY variant): `TestHU017_E2E_AbortMidSequence_NoRestore` — abort mid-conflict, quit; no restore, `CHERRY_PICK_HEAD` intact, resume still offered.
- [ ] 5.3 GREEN: none — both prove Groups 1–4; rerun to confirm; fix any cross-group wiring gap in the owning group's files, not here.

Group gate: `go build ./... && go vet ./... && go test ./internal/app/... ./internal/git/... -short`; full run (no `-short`) before merge.
