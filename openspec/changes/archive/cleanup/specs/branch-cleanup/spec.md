# Delta for Branch Cleanup

## ADDED Requirements

### Requirement: Original Branch Restored On Finish Or Abort

When a flow reaches a terminal state or is abandoned by quitting, the system MUST return the user to the branch active at flow startup (docs/HISTORIAS.md:1095,1103). Restore MUST be skipped (no-op) when the repository is `RepoState.InProgress` (unresolved cherry-pick conflict), when the original branch is empty/detached/no longer exists, or when it already equals the current branch.

#### Scenario: Finish or abort restores the original branch
- GIVEN the user started on `main` and DeployDeck checked out `deploy/DD-1`
- WHEN the flow reaches `StateSucceeded` or `StateAborted` and the user quits
- THEN the working tree is checked out back onto `main`

#### Scenario: Unresolved conflict blocks restore (mandatory negative)
- GIVEN a cherry-pick left `RepoState.InProgress` (unmerged paths) on `deploy/DD-1`
- WHEN the user quits mid-conflict without resolving
- THEN no checkout is attempted; the working tree stays on `deploy/DD-1`
- AND HU-013 conflict resume-detection remains possible on the next run

#### Scenario: No-op restore when already current, detached, or gone
- GIVEN the current branch already equals the original, or the original is detached/no longer exists locally
- WHEN the flow terminates
- THEN no checkout is attempted and quitting proceeds without error

### Requirement: Current Run's Temp Branch Deleted With Confirmation

After a terminal or abandoned flow, the system MUST offer to delete the current run's `deploy/*` branch and delete it ONLY on explicit confirmation (docs/HISTORIAS.md:1096,1104). On confirmation, the branch MUST be deleted locally, and its remote ref MUST also be deleted if it had been pushed.

#### Scenario: Confirmed delete removes local and remote refs
- GIVEN `deploy/DD-1` was pushed to the bare `origin`
- WHEN the user confirms deletion after the flow ends
- THEN the local branch and `origin/deploy/DD-1` are both deleted

#### Scenario: Confirmed delete of a local-only branch skips remote
- GIVEN `deploy/DD-1` was never pushed
- WHEN the user confirms deletion
- THEN only the local branch is deleted; no remote delete is attempted

#### Scenario: Declining confirmation deletes nothing
- GIVEN `deploy/DD-1` exists locally and remotely
- WHEN the user does not confirm
- THEN both the local branch and its remote ref still exist

### Requirement: Orphan Deploy Branches Listed For Batch Cleanup

The system MUST provide a cleanup screen listing every orphan `deploy/*` branch — one not tied to a live in-progress run — with its age and push status (pushed vs local-only) (docs/HISTORIAS.md:1097,1105).

#### Scenario: Orphans listed with age and push status
- GIVEN `deploy/DD-2` (pushed, 10 days old) and `deploy/DD-3` (local-only, 2 days old) are orphans
- WHEN the user opens the branch cleanup screen
- THEN both are listed with their age and push status

#### Scenario: In-progress run's branch is excluded
- GIVEN `deploy/DD-4` belongs to a currently in-progress run
- WHEN the cleanup screen is opened
- THEN `deploy/DD-4` is NOT listed as an orphan

### Requirement: Merged-Vs-Abandoned Is A Best-Effort Label Only

The cleanup screen MAY label an orphan branch "likely merged" (git-native ancestor check against `origin/<target>`) vs "abandoned". This label is advisory ONLY and MUST NEVER bypass or weaken the deletion confirmation required elsewhere in this spec. A false "not merged" result (e.g. produced by a squash-merge) MUST only cause an extra confirmation prompt, never a wrong deletion.

#### Scenario: Label never bypasses confirmation
- GIVEN `deploy/DD-5` is labeled "likely merged"
- WHEN the user selects it for deletion
- THEN the same confirmation rules apply as for any branch (strong confirmation if unpushed work exists)

#### Scenario: Mislabeled squash-merge stays safely gated
- GIVEN `deploy/DD-6` was squash-merged, so the ancestor check falsely reports "not merged"
- WHEN the user attempts to delete it
- THEN only the applicable confirmation is required and no wrong deletion occurs

### Requirement: Strong Confirmation For Unpushed Work

A branch with commits not present on its remote ref, or with no remote ref at all, MUST NOT be deleted without a strong (typed) confirmation. A normal confirmation MUST be refused for such a branch (docs/HISTORIAS.md:1099,1106).

#### Scenario: Normal confirm refused for unpushed work
- GIVEN `deploy/DD-7` has commits not present on `origin/deploy/DD-7`, or `deploy/DD-8` has no remote ref at all
- WHEN the user attempts a normal confirmation to delete it
- THEN deletion is refused and the branch still exists

#### Scenario: Strong confirmation deletes an unpushed branch
- GIVEN `deploy/DD-7` has unpushed commits
- WHEN the user completes the strong (typed) confirmation
- THEN the branch is deleted

#### Scenario: Fully pushed branch accepts normal confirmation
- GIVEN `deploy/DD-9` has zero commits ahead of `origin/deploy/DD-9`
- WHEN the user confirms deletion normally
- THEN the branch is deleted without requiring strong confirmation

### Requirement: Run Retention Applied From The Cleanup Surface

The branch cleanup screen MUST provide a way to invoke the existing run-retention mechanism (`runs.keepLast`/`runs.keepDays`, see `openspec/specs/run-retention/spec.md`) so that runs outside the retention window are pruned and recent runs are kept (docs/HISTORIAS.md:1098,1112). This requirement REUSES `run-retention`'s existing pure selection rule and MUST NOT redefine, reimplement, or change it.

#### Scenario: Invoking retention from the cleanup screen prunes old runs
- GIVEN `.deploydeck/runs/` has runs outside both `keepLast` and `keepDays`, plus recent runs within the window
- WHEN the user invokes retention from the branch cleanup screen
- THEN the outside-window runs are removed and the recent runs are kept, per `run-retention`'s existing selection rule
